package extcal

import (
	"cmp"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

// Google's endpoints and the one scope asked for: reading calendars, not
// changing them.
const (
	googleAuthURL   = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL  = "https://oauth2.googleapis.com/token"
	googleRevokeURL = "https://oauth2.googleapis.com/revoke"
	googleAPI       = "https://www.googleapis.com/calendar/v3"
	googleScope     = "https://www.googleapis.com/auth/calendar.readonly"
)

// FlowLifetime is how long a begun connection to Google waits for the
// person's consent; maxPending how many may wait at once.
const (
	FlowLifetime = 10 * time.Minute
	maxPending   = 200
	maxPages     = 10
)

// Errors of connecting a Google account.
var (
	ErrNoFlow  = errors.New("extcal: no such connection in progress")
	ErrRevoked = errors.New("extcal: access to the account was withdrawn; connect it again")
)

// Google connects people's Google accounts by OAuth and reads their
// calendars with the Calendar API. It uses the Google client of single
// sign-on (OT_GOOGLE_CLIENT_ID and OT_GOOGLE_CLIENT_SECRET, ADR-0066):
// the Calendar API must be enabled in its project, and RedirectURL added
// to its redirect addresses.
type Google struct {
	ClientID, ClientSecret, RedirectURL string
	// AuthURL, TokenURL, RevokeURL and API are Google's unless a test
	// sets them.
	AuthURL, TokenURL, RevokeURL, API string

	reader  *Reader
	now     func() time.Time
	mu      sync.Mutex
	pending map[string]pendingFlow
	access  map[string]*oauth2.Token
}

type pendingFlow struct {
	user, verifier string
	expires        time.Time
}

// NewGoogle sets Google up, or answers nil when the client or the
// redirect address is missing.
func NewGoogle(clientID, clientSecret, redirectURL string) *Google {
	if clientID == "" || clientSecret == "" || redirectURL == "" {
		return nil
	}
	return &Google{ClientID: clientID, ClientSecret: clientSecret, RedirectURL: redirectURL,
		AuthURL: googleAuthURL, TokenURL: googleTokenURL, RevokeURL: googleRevokeURL, API: googleAPI,
		now: time.Now, pending: map[string]pendingFlow{}, access: map[string]*oauth2.Token{}}
}

func (g *Google) config() *oauth2.Config {
	return &oauth2.Config{ClientID: g.ClientID, ClientSecret: g.ClientSecret, RedirectURL: g.RedirectURL,
		Scopes: []string{googleScope}, Endpoint: oauth2.Endpoint{AuthURL: g.AuthURL, TokenURL: g.TokenURL, AuthStyle: oauth2.AuthStyleInParams}}
}

func (g *Google) ctx(ctx context.Context) context.Context {
	return context.WithValue(ctx, oauth2.HTTPClient, g.reader.client)
}

func random() string {
	b := make([]byte, 24)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// Begin starts connecting a person's Google account and answers the
// address of Google's consent and the state the caller binds to the
// browser. Google is asked for a refresh token (offline access) and to
// ask the person again, so that it hands one out.
func (g *Google) Begin(user string) (authURL, state string, err error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	for k, f := range g.pending {
		if !now.Before(f.expires) {
			delete(g.pending, k)
		}
	}
	if len(g.pending) >= maxPending {
		return "", "", fmt.Errorf("%w: too many connections in progress", ErrUnreadable)
	}
	state, verifier := random(), oauth2.GenerateVerifier()
	g.pending[state] = pendingFlow{user: user, verifier: verifier, expires: now.Add(FlowLifetime)}
	return g.config().AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(verifier),
		oauth2.SetAuthURLParam("prompt", "consent")), state, nil
}

// Pending answers whose connection a state begun.
func (g *Google) Pending(state string) (string, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	f, ok := g.pending[state]
	return f.user, ok && g.now().Before(f.expires)
}

// Cancel forgets a begun connection the person refused at Google.
func (g *Google) Cancel(state string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.pending, state)
}

// Finish completes a connection with the code Google sent back: the
// refresh token is kept sealed as the account's credential, named by the
// account's address, which its primary calendar carries.
func (g *Google) Finish(ctx context.Context, state, code string) (Account, error) {
	g.mu.Lock()
	f, ok := g.pending[state]
	delete(g.pending, state)
	g.mu.Unlock()
	if !ok || !g.now().Before(f.expires) {
		return Account{}, ErrNoFlow
	}
	tok, err := g.config().Exchange(g.ctx(ctx), code, oauth2.VerifierOption(f.verifier))
	if err != nil {
		return Account{}, fmt.Errorf("%w: Google refused the code: %s", ErrUnreadable, reason(err))
	}
	if tok.RefreshToken == "" {
		return Account{}, fmt.Errorf("%w: Google handed out no refresh token", ErrUnreadable)
	}
	cals, err := g.list(ctx, tok.AccessToken)
	if err != nil {
		return Account{}, err
	}
	name := ""
	for _, c := range cals {
		if c.Primary {
			name = c.ID
		}
	}
	if name == "" {
		return Account{}, fmt.Errorf("%w: the account has no calendar of its own", ErrUnreadable)
	}
	a, err := g.reader.store.Connect(f.user, Account{Kind: KindGoogle, Name: name}, tok.RefreshToken)
	if err != nil {
		return Account{}, err
	}
	g.mu.Lock()
	g.access[f.user+"\x00"+a.ID] = tok
	g.mu.Unlock()
	return a, nil
}

// token is an access token for a person's account, renewed with its
// refresh token when the one held has run out. A refresh token Google no
// longer accepts — the person withdrew this server's access — is
// ErrRevoked.
func (g *Google) token(ctx context.Context, user, account string) (string, error) {
	k := user + "\x00" + account
	g.mu.Lock()
	tok := g.access[k]
	g.mu.Unlock()
	if tok.Valid() {
		return tok.AccessToken, nil
	}
	refresh, err := g.reader.store.Secret(user, account)
	if err != nil {
		return "", fmt.Errorf("%w: the account is no longer connected", ErrUnreadable)
	}
	tok, err = g.config().TokenSource(g.ctx(ctx), &oauth2.Token{RefreshToken: refresh}).Token()
	if err != nil {
		var re *oauth2.RetrieveError
		if errors.As(err, &re) && re.ErrorCode == "invalid_grant" {
			return "", ErrRevoked
		}
		return "", fmt.Errorf("%w: renewing access at Google: %s", ErrUnreadable, reason(err))
	}
	if tok.RefreshToken != "" && tok.RefreshToken != refresh {
		if err := g.reader.store.Renew(user, account, tok.RefreshToken); err != nil {
			return "", err
		}
	}
	g.mu.Lock()
	g.access[k] = tok
	g.mu.Unlock()
	return tok.AccessToken, nil
}

// forget drops the access token held for an account.
func (g *Google) forget(user, account string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.access, user+"\x00"+account)
}

// Revoke withdraws a refresh token at Google, as the person disconnects
// the account; it is tried once, and the account goes either way.
func (g *Google) Revoke(ctx context.Context, user, account string) {
	g.forget(user, account)
	refresh, err := g.reader.store.Secret(user, account)
	if err != nil {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.RevokeURL, strings.NewReader(url.Values{"token": {refresh}}.Encode()))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if resp, err := g.reader.client.Do(req); err == nil {
		resp.Body.Close()
	}
}

// Remote is a calendar an account holds, which its person may choose to
// show.
type Remote struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Colour  string `json:"colour,omitempty"`
	Primary bool   `json:"primary,omitempty"`
}

// get reads one of the Calendar API's answers.
func (g *Google) get(ctx context.Context, access, path string, q url.Values, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.API+path+"?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+access)
	data, err := g.reader.do(req)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, into); err != nil {
		return fmt.Errorf("%w: Google's answer cannot be read", ErrUnreadable)
	}
	return nil
}

func (g *Google) list(ctx context.Context, access string) ([]Remote, error) {
	var out []Remote
	q := url.Values{"minAccessRole": {"reader"}, "maxResults": {"250"}}
	for range maxPages {
		var page struct {
			Items []struct {
				ID, Summary, SummaryOverride, BackgroundColor string
				Primary                                       bool
			}
			NextPageToken string
		}
		if err := g.get(ctx, access, "/users/me/calendarList", q, &page); err != nil {
			return nil, err
		}
		for _, c := range page.Items {
			out = append(out, Remote{ID: c.ID, Name: cmp.Or(c.SummaryOverride, c.Summary, c.ID), Colour: colourOf(c.BackgroundColor), Primary: c.Primary})
		}
		if page.NextPageToken == "" {
			break
		}
		q.Set("pageToken", page.NextPageToken)
	}
	return out, nil
}

// Calendars lists the calendars of a person's Google account.
func (g *Google) Calendars(ctx context.Context, user, account string) ([]Remote, error) {
	access, err := g.token(ctx, user, account)
	if err != nil {
		return nil, err
	}
	cals, err := g.list(ctx, access)
	var se *StatusError
	if errors.As(err, &se) && se.Status == http.StatusUnauthorized {
		g.forget(user, account)
	}
	return cals, err
}

// googleTime is an event's start or end: a date for an all-day event, a
// time with its offset otherwise.
type googleTime struct {
	Date     string `json:"date"`
	DateTime string `json:"dateTime"`
}

// readGoogle reads a calendar of a Google account in [from, to), each
// occurrence of a repeating event apart (singleEvents), as Google expands
// them itself.
func (r *Reader) readGoogle(ctx context.Context, user string, s Source, from, to time.Time) ([]Event, error) {
	g := r.google
	if g == nil {
		return nil, fmt.Errorf("%w: Google is not set up on this server", ErrUnreadable)
	}
	access, err := g.token(ctx, user, s.Account)
	if err != nil {
		return nil, err
	}
	q := url.Values{"singleEvents": {"true"}, "orderBy": {"startTime"}, "maxResults": {"2500"},
		"timeMin": {from.UTC().Format(time.RFC3339)}, "timeMax": {to.UTC().Format(time.RFC3339)}}
	var out []Event
	for range maxPages {
		var page struct {
			Items []struct {
				ID, Status, Summary, Location string
				Start, End                    googleTime
			}
			NextPageToken string
		}
		if err := g.get(ctx, access, "/calendars/"+url.PathEscape(s.Calendar)+"/events", q, &page); err != nil {
			var se *StatusError
			if errors.As(err, &se) && se.Status == http.StatusUnauthorized {
				g.forget(user, s.Account)
			}
			return nil, err
		}
		for _, it := range page.Items {
			if it.Status == "cancelled" {
				continue
			}
			ev := Event{UID: it.ID, Title: it.Summary, Location: it.Location}
			if it.Start.Date != "" {
				start, err1 := time.Parse(time.DateOnly, it.Start.Date)
				end, err2 := time.Parse(time.DateOnly, it.End.Date)
				if err1 != nil {
					continue
				}
				if err2 != nil || !end.After(start) {
					end = start.AddDate(0, 0, 1)
				}
				ev.Start, ev.End, ev.AllDay = start, end, true
			} else {
				start, err1 := time.Parse(time.RFC3339, it.Start.DateTime)
				end, err2 := time.Parse(time.RFC3339, it.End.DateTime)
				if err1 != nil {
					continue
				}
				if err2 != nil || end.Before(start) {
					end = start
				}
				ev.Start, ev.End = start, end
			}
			out = append(out, ev)
		}
		if page.NextPageToken == "" {
			break
		}
		q.Set("pageToken", page.NextPageToken)
	}
	return out, nil
}

// colourOf writes a service's colour as #rrggbb, or nothing; Apple writes
// #rrggbbaa.
func colourOf(c string) string {
	if len(c) == 9 && c[0] == '#' {
		c = c[:7]
	}
	if colour.MatchString(c) {
		return strings.ToLower(c)
	}
	return ""
}
