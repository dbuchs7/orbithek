package photosource

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

// Google's endpoints and the one scope asked for: reading the media items
// a person picks, nothing else of their library.
const (
	googleAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL = "https://oauth2.googleapis.com/token"
	googlePicker   = "https://photospicker.googleapis.com/v1"
	pickerScope    = "https://www.googleapis.com/auth/photospicker.mediaitems.readonly"
)

// FlowLifetime is how long a begun connection to Google waits for the
// person's consent.
const FlowLifetime = 10 * time.Minute

// Bounds of the Picker: connections in progress at once, open sessions per
// person, the items one session may answer and the pages they come in,
// and how long one answer of the API may be.
const (
	maxPending   = 200
	maxSessions  = 3
	maxItems     = 2000
	maxItemPages = 30
	maxAnswer    = 4 << 20
)

// Lifetimes of what Google hands out: an access token without an expiry
// is taken to last an hour, Google's usual; a session without an expiry
// or timeout is kept an hour; a listed session is kept so long that its
// base addresses, valid for 60 minutes, can be downloaded; a token counts
// as valid only with a minute to spare.
const (
	defaultTokenLife   = time.Hour
	defaultSessionLife = time.Hour
	baseURLLife        = 55 * time.Minute
	tokenMargin        = time.Minute
)

// googleContent and googlePickerHosts are the hosts Google hands out
// photos on and sends people to for picking.
var (
	googleContent     = []string{".googleusercontent.com"}
	googlePickerHosts = []string{".google.com"}
)

// Google brings photos from Google Photos through the Picker API, in
// which a person chooses photos at Google and the server reads exactly
// those. Since 31 March 2025 the Library API reaches only what an app
// created itself, so the Picker is the way to a person's own photos.
//
// A person connects by OAuth 2.0 with PKCE and online access only: no
// refresh token is asked for or kept, and the access token is held in
// memory until it expires, about an hour. Nothing that outlives it is
// stored; a person connects again for a later import.
//
// Google is safe for concurrent use.
type Google struct {
	ClientID, ClientSecret, RedirectURL string
	// AuthURL, TokenURL and API are Google's unless a test sets them.
	AuthURL, TokenURL, API string

	client   *http.Client
	now      func() time.Time
	mu       sync.Mutex
	pending  map[string]pendingFlow
	access   map[string]accessToken
	sessions map[string]*session
}

type pendingFlow struct {
	user, verifier string
	expires        time.Time
}

type accessToken struct {
	token   string
	expires time.Time
}

// session is a picking session the server opened for a person, with the
// items it listed.
type session struct {
	user    string
	expires time.Time
	picked  bool
	items   map[string]pickedItem
}

type pickedItem struct {
	baseURL string
	media   Media
}

// NewGoogle sets Google up with the client every request goes through, or
// answers nil when the client ID, its secret or the redirect address is
// missing. A nil client is http.DefaultClient.
func NewGoogle(clientID, clientSecret, redirectURL string, client *http.Client) *Google {
	if clientID == "" || clientSecret == "" || redirectURL == "" {
		return nil
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &Google{ClientID: clientID, ClientSecret: clientSecret, RedirectURL: redirectURL,
		AuthURL: googleAuthURL, TokenURL: googleTokenURL, API: googlePicker,
		client: client, now: time.Now, pending: map[string]pendingFlow{},
		access: map[string]accessToken{}, sessions: map[string]*session{}}
}

func (g *Google) config() *oauth2.Config {
	return &oauth2.Config{ClientID: g.ClientID, ClientSecret: g.ClientSecret, RedirectURL: g.RedirectURL,
		Scopes: []string{pickerScope}, Endpoint: oauth2.Endpoint{AuthURL: g.AuthURL, TokenURL: g.TokenURL, AuthStyle: oauth2.AuthStyleInParams}}
}

func random() string {
	b := make([]byte, 24)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// Begin starts connecting a person to Google Photos and answers the
// address of Google's consent and the state the caller binds to the
// browser. Access is online only, so Google hands out no refresh token.
func (g *Google) Begin(user string) (authURL, state string, err error) {
	if user == "" {
		return "", "", fmt.Errorf("%w: no person", ErrInvalid)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	for k, f := range g.pending {
		if !now.Before(f.expires) {
			delete(g.pending, k)
		}
	}
	if len(g.pending) >= maxPending {
		return "", "", fmt.Errorf("%w: connections to Google in progress", ErrTooMany)
	}
	state, verifier := random(), oauth2.GenerateVerifier()
	g.pending[state] = pendingFlow{user: user, verifier: verifier, expires: now.Add(FlowLifetime)}
	return g.config().AuthCodeURL(state, oauth2.AccessTypeOnline, oauth2.S256ChallengeOption(verifier)), state, nil
}

// Pending answers whose connection a state began, while it waits.
func (g *Google) Pending(state string) (user string, ok bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	f, ok := g.pending[state]
	if !ok || !g.now().Before(f.expires) {
		return "", false
	}
	return f.user, true
}

// Cancel forgets a begun connection, as when the person refused at
// Google.
func (g *Google) Cancel(state string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.pending, state)
}

// Finish completes a connection with the code Google sent back, exchanged
// with the flow's verifier, and keeps the access token in memory for the
// person who began it. A refresh token, should Google hand one out, is
// dropped. A grant without the Picker's scope — the person unticked it at
// Google — is ErrNotConnected.
func (g *Google) Finish(ctx context.Context, state, code string) (user string, err error) {
	g.mu.Lock()
	f, ok := g.pending[state]
	delete(g.pending, state)
	g.mu.Unlock()
	if !ok || !g.now().Before(f.expires) {
		return "", fmt.Errorf("%w: no such connection in progress", ErrNotFound)
	}
	tok, err := g.config().Exchange(context.WithValue(ctx, oauth2.HTTPClient, g.client), code, oauth2.VerifierOption(f.verifier))
	if err != nil {
		var re *oauth2.RetrieveError
		if errors.As(err, &re) && errorCode(re.ErrorCode) {
			return "", fmt.Errorf("%w: Google refused the code (%s)", ErrUnreadable, re.ErrorCode)
		}
		return "", fmt.Errorf("%w: Google refused the code", ErrUnreadable)
	}
	if tok.AccessToken == "" {
		return "", fmt.Errorf("%w: Google handed out no access token", ErrUnreadable)
	}
	if scope, _ := tok.Extra("scope").(string); scope != "" && !slices.Contains(strings.Fields(scope), pickerScope) {
		return "", fmt.Errorf("%w: access to Google Photos was not granted", ErrNotConnected)
	}
	now := g.now()
	life := defaultTokenLife
	switch {
	case tok.ExpiresIn > 0:
		life = time.Duration(tok.ExpiresIn) * time.Second
	case !tok.Expiry.IsZero():
		life = time.Until(tok.Expiry)
	}
	g.mu.Lock()
	g.access[f.user] = accessToken{token: tok.AccessToken, expires: now.Add(life)}
	g.mu.Unlock()
	return f.user, nil
}

// errorCode reports whether an OAuth error code is a plain word such as
// invalid_grant, which may be named in a message.
func errorCode(s string) bool {
	if s == "" || len(s) > 40 {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && r != '_' {
			return false
		}
	}
	return true
}

// Connected reports whether the person holds an access token valid for at
// least another minute.
func (g *Google) Connected(user string) bool {
	_, ok := g.token(user)
	return ok
}

// token is the person's access token while it is valid; one that ran out
// is dropped.
func (g *Google) token(user string) (string, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	t, ok := g.access[user]
	if !ok {
		return "", false
	}
	if !g.now().Add(tokenMargin).Before(t.expires) {
		delete(g.access, user)
		return "", false
	}
	return t.token, true
}

// Forget drops the person's access token and sessions, as when they
// disconnect or sign out. Nothing is sent to Google.
func (g *Google) Forget(user string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.access, user)
	for id, s := range g.sessions {
		if s.user == user {
			delete(g.sessions, id)
		}
	}
}

// PickerSession is a picking session: the address the person opens to
// choose photos at Google, how often and how long to ask whether they are
// done, and whether they are.
type PickerSession struct {
	ID, PickerURI string
	PollInterval  time.Duration
	Timeout       time.Duration
	Picked        bool
}

// wireSession is a PickingSession as the Picker API writes it; durations
// are strings such as "5s", times RFC 3339.
type wireSession struct {
	ID            string `json:"id"`
	PickerURI     string `json:"pickerUri"`
	PollingConfig struct {
		PollInterval string `json:"pollInterval"`
		TimeoutIn    string `json:"timeoutIn"`
	} `json:"pollingConfig"`
	ExpireTime    string `json:"expireTime"`
	MediaItemsSet bool   `json:"mediaItemsSet"`
}

// duration reads a duration of the Picker API, between bottom and top.
func duration(s string, bottom, top time.Duration) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil || d < bottom {
		return bottom
	}
	return min(d, top)
}

// picker checks and converts a session Google answered.
func (w wireSession) picker() (PickerSession, error) {
	if w.ID == "" {
		return PickerSession{}, fmt.Errorf("%w: Google answered a session without an ID", ErrUnreadable)
	}
	if w.PickerURI != "" {
		u, err := url.Parse(w.PickerURI)
		if err != nil || !allowed(u, googlePickerHosts...) {
			return PickerSession{}, fmt.Errorf("%w: Google answered a picking address that is not Google's", ErrUnreadable)
		}
	}
	return PickerSession{ID: w.ID, PickerURI: w.PickerURI,
		PollInterval: duration(w.PollingConfig.PollInterval, time.Second, time.Minute),
		Timeout:      duration(w.PollingConfig.TimeoutIn, 0, 24*time.Hour),
		Picked:       w.MediaItemsSet}, nil
}

// expiry is when a session is forgotten: its expiry as Google says, else
// at the end of its timeout, else after an hour.
func (w wireSession) expiry(now time.Time, timeout time.Duration) time.Time {
	if t, err := time.Parse(time.RFC3339, w.ExpireTime); err == nil {
		return t
	}
	if timeout > 0 {
		return now.Add(timeout)
	}
	return now.Add(defaultSessionLife)
}

// sweep forgets sessions that expired; g.mu is held.
func (g *Google) sweep(now time.Time) {
	for id, s := range g.sessions {
		if !now.Before(s.expires) {
			delete(g.sessions, id)
		}
	}
}

// own answers the person's session while it lasts; g.mu is held.
func (g *Google) own(user, id string) (*session, error) {
	g.sweep(g.now())
	s, ok := g.sessions[id]
	if !ok || s.user != user {
		return nil, fmt.Errorf("%w: no such picking session", ErrNotFound)
	}
	return s, nil
}

// call sends one request to the Picker API with the person's access token
// and reads its answer into into, if not nil. 401 drops the token and is
// ErrNotConnected, 404 is ErrNotFound, any other refusal ErrUnreadable
// with its status.
func (g *Google) call(ctx context.Context, user, method, path string, q url.Values, body, into any) error {
	access, ok := g.token(user)
	if !ok {
		return ErrNotConnected
	}
	addr := g.API + path
	if len(q) > 0 {
		addr += "?" + q.Encode()
	}
	var rd io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalid, err)
		}
		rd = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, addr, rd)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrUnreadable, reason(err))
	}
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrUnreadable, reason(err))
	}
	if err := g.refusal(user, resp); err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := readBounded(resp.Body, maxAnswer)
	if err != nil {
		return err
	}
	if into != nil {
		if err := json.Unmarshal(data, into); err != nil {
			return fmt.Errorf("%w: Google's answer cannot be read", ErrUnreadable)
		}
	}
	return nil
}

// refusal answers the error an answer that is no success stands for, and
// drops it; 401 also drops the person's token.
func (g *Google) refusal(user string, resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		return nil
	}
	drain(resp)
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		g.mu.Lock()
		delete(g.access, user)
		g.mu.Unlock()
		return fmt.Errorf("%w: Google no longer accepts the access", ErrNotConnected)
	case http.StatusNotFound:
		return fmt.Errorf("%w: Google knows no such session or item", ErrNotFound)
	}
	return fmt.Errorf("%w: Google answers %d", ErrUnreadable, resp.StatusCode)
}

// NewSession opens a picking session for the person: POST /sessions. A
// person has at most three open at once (ErrTooMany).
func (g *Google) NewSession(ctx context.Context, user string) (PickerSession, error) {
	if _, ok := g.token(user); !ok {
		return PickerSession{}, ErrNotConnected
	}
	g.mu.Lock()
	g.sweep(g.now())
	open := 0
	for _, s := range g.sessions {
		if s.user == user {
			open++
		}
	}
	g.mu.Unlock()
	if open >= maxSessions {
		return PickerSession{}, fmt.Errorf("%w: picking sessions open at once", ErrTooMany)
	}
	var w wireSession
	if err := g.call(ctx, user, http.MethodPost, "/sessions", nil, struct{}{}, &w); err != nil {
		return PickerSession{}, err
	}
	p, err := w.picker()
	if err != nil {
		return PickerSession{}, err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	if _, taken := g.sessions[p.ID]; taken {
		return PickerSession{}, fmt.Errorf("%w: Google answered a session ID already in use", ErrUnreadable)
	}
	g.sessions[p.ID] = &session{user: user, expires: w.expiry(now, p.Timeout), picked: p.Picked}
	return p, nil
}

// Poll asks Google about one of the person's sessions: GET /sessions/{id}.
// Someone else's session, or one forgotten, is ErrNotFound.
func (g *Google) Poll(ctx context.Context, user, id string) (PickerSession, error) {
	g.mu.Lock()
	_, err := g.own(user, id)
	g.mu.Unlock()
	if err != nil {
		return PickerSession{}, err
	}
	var w wireSession
	if err := g.call(ctx, user, http.MethodGet, "/sessions/"+url.PathEscape(id), nil, nil, &w); err != nil {
		return PickerSession{}, err
	}
	p, err := w.picker()
	if err != nil {
		return PickerSession{}, err
	}
	if p.ID != id {
		return PickerSession{}, fmt.Errorf("%w: Google answered another session", ErrUnreadable)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if s, ok := g.sessions[id]; ok && s.user == user {
		s.picked = p.Picked
		if t, err := time.Parse(time.RFC3339, w.ExpireTime); err == nil {
			s.expires = t
		}
	}
	return p, nil
}

// Media is a picked item as the album needs it: its name and type, when it
// was taken, its size and camera, and whether it is a video.
type Media struct {
	ID, Name, Type string
	Taken          time.Time
	Width, Height  int
	Make, Model    string
	Video          bool
}

// wireItem is a PickedMediaItem as the Picker API writes it.
type wireItem struct {
	ID         string `json:"id"`
	CreateTime string `json:"createTime"`
	Type       string `json:"type"`
	MediaFile  struct {
		BaseURL           string `json:"baseUrl"`
		MimeType          string `json:"mimeType"`
		Filename          string `json:"filename"`
		MediaFileMetadata struct {
			Width       number `json:"width"`
			Height      number `json:"height"`
			CameraMake  string `json:"cameraMake"`
			CameraModel string `json:"cameraModel"`
		} `json:"mediaFileMetadata"`
	} `json:"mediaFile"`
}

func (w wireItem) media() Media {
	m := w.MediaFile.MediaFileMetadata
	taken, _ := time.Parse(time.RFC3339, w.CreateTime)
	return Media{ID: w.ID, Name: w.MediaFile.Filename, Type: w.MediaFile.MimeType, Taken: taken,
		Width: m.Width.int(), Height: m.Height.int(), Make: m.CameraMake, Model: m.CameraModel,
		Video: w.Type == "VIDEO"}
}

// Items lists what the person picked in one of their sessions, every page
// of GET /mediaItems, at most 2000 items (more is ErrTooMany). Each item's
// base address is kept with the session for Download, and the session is
// kept long enough to use them.
func (g *Google) Items(ctx context.Context, user, id string) ([]Media, error) {
	g.mu.Lock()
	_, err := g.own(user, id)
	g.mu.Unlock()
	if err != nil {
		return nil, err
	}
	q := url.Values{"sessionId": {id}, "pageSize": {"100"}}
	var out []Media
	items := map[string]pickedItem{}
	for page := 0; ; page++ {
		if page == maxItemPages {
			return nil, fmt.Errorf("%w: Google names more than %d pages of picked items", ErrTooMany, maxItemPages)
		}
		var answer struct {
			MediaItems    []wireItem `json:"mediaItems"`
			NextPageToken string     `json:"nextPageToken"`
		}
		if err := g.call(ctx, user, http.MethodGet, "/mediaItems", q, nil, &answer); err != nil {
			return nil, err
		}
		for _, w := range answer.MediaItems {
			if w.ID == "" {
				continue
			}
			if _, dup := items[w.ID]; dup {
				continue
			}
			if len(items) == maxItems {
				return nil, fmt.Errorf("%w: more than %d items were picked", ErrTooMany, maxItems)
			}
			m := w.media()
			items[w.ID] = pickedItem{baseURL: w.MediaFile.BaseURL, media: m}
			out = append(out, m)
		}
		if answer.NextPageToken == "" {
			break
		}
		q.Set("pageToken", answer.NextPageToken)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	s, ok := g.sessions[id]
	if !ok || s.user != user {
		return nil, fmt.Errorf("%w: no such picking session", ErrNotFound)
	}
	s.items, s.picked = items, true
	if keep := g.now().Add(baseURLLife); s.expires.Before(keep) {
		s.expires = keep
	}
	return out, nil
}

// Picked answers one picked item as Items listed it, without asking
// Google, so the caller can choose how many bytes it may have: a video
// may have more than a photo (ADR-0189, ADR-0192).
func (g *Google) Picked(user, id, media string) (Media, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	s, err := g.own(user, id)
	if err != nil {
		return Media{}, err
	}
	it, ok := s.items[media]
	if !ok {
		return Media{}, fmt.Errorf("%w: no such picked item", ErrNotFound)
	}
	return it.media, nil
}

// Download reads one picked item from Google with the person's access
// token: a photo by its base address with "=d", which keeps its metadata
// but its place, a video with "=dv" (ADR-0192). The item must have been
// listed by Items. The base address must be https on a host of
// googleusercontent.com, and redirects are followed only on the same
// host, so the token goes nowhere else. Reading more than limit bytes
// fails with ErrTooLarge.
func (g *Google) Download(ctx context.Context, user, id, media string, limit int64) (io.ReadCloser, Media, error) {
	return g.fetch(ctx, user, id, media, func(m Media) string {
		if m.Video {
			return "=dv"
		}
		return "=d"
	}, limit)
}

// Poster reads the picture Google makes of a picked video, at most 1600
// pixels on its longer edge, as the video's poster (ADR-0192). A photo
// has none: ErrInvalid.
func (g *Google) Poster(ctx context.Context, user, id, media string, limit int64) (io.ReadCloser, error) {
	rc, m, err := g.fetch(ctx, user, id, media, func(m Media) string {
		if !m.Video {
			return ""
		}
		return "=w1600-h1600"
	}, limit)
	if err == nil && !m.Video {
		rc.Close()
		return nil, fmt.Errorf("%w: a photo has no poster", ErrInvalid)
	}
	return rc, err
}

// fetch reads one picked item at its base address with the parameter
// param chooses for it; an empty parameter reads nothing.
func (g *Google) fetch(ctx context.Context, user, id, media string, param func(Media) string, limit int64) (io.ReadCloser, Media, error) {
	if limit <= 0 {
		return nil, Media{}, fmt.Errorf("%w: no limit", ErrInvalid)
	}
	g.mu.Lock()
	s, err := g.own(user, id)
	var it pickedItem
	ok := false
	if err == nil {
		it, ok = s.items[media]
	}
	g.mu.Unlock()
	if err != nil {
		return nil, Media{}, err
	}
	if !ok {
		return nil, Media{}, fmt.Errorf("%w: no such picked item", ErrNotFound)
	}
	suffix := param(it.media)
	if suffix == "" {
		return nil, it.media, fmt.Errorf("%w: nothing to read of this item", ErrInvalid)
	}
	u, err := url.Parse(it.baseURL)
	if err != nil || !allowed(u, googleContent...) || u.RawQuery != "" || u.Fragment != "" {
		return nil, it.media, fmt.Errorf("%w: Google handed out an address that is not one of its photos", ErrUnreadable)
	}
	access, ok := g.token(user)
	if !ok {
		return nil, it.media, ErrNotConnected
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, it.baseURL+suffix, nil)
	if err != nil {
		return nil, it.media, fmt.Errorf("%w: %s", ErrUnreadable, reason(err))
	}
	req.Header.Set("Authorization", "Bearer "+access)
	client := guarded(g.client, func(next, first *url.URL) bool {
		return next.Scheme == "https" && strings.EqualFold(next.Host, first.Host)
	})
	resp, err := client.Do(req)
	if err != nil {
		return nil, it.media, fmt.Errorf("%w: %s", ErrUnreadable, reason(err))
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, it.media, g.refusal(user, resp)
	}
	rc, err := download(resp, limit)
	return rc, it.media, err
}

// Close ends one of the person's sessions at Google, DELETE
// /sessions/{id}, tried once, and forgets it either way.
func (g *Google) Close(ctx context.Context, user, id string) {
	g.mu.Lock()
	_, err := g.own(user, id)
	if err == nil {
		delete(g.sessions, id)
	}
	g.mu.Unlock()
	if err != nil {
		return
	}
	g.call(ctx, user, http.MethodDelete, "/sessions/"+url.PathEscape(id), nil, nil, nil)
}
