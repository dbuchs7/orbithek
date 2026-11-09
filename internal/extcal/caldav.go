package extcal

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// AppleServer is the CalDAV address of Apple iCloud; an iCloud account is
// reached with the Apple ID and an app-specific password.
const AppleServer = "https://caldav.icloud.com"

// maxRedirects is how many redirects a CalDAV request follows.
const maxRedirects = 5

// CalDAVServer checks the address of a CalDAV server. Only https is used,
// as the account's password goes with every request.
func CalDAVServer(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || !strings.EqualFold(u.Scheme, "https") || u.User != nil || len(raw) > maxSecret {
		return "", fmt.Errorf("%w: a CalDAV server's address is like https://caldav.icloud.com", ErrInvalid)
	}
	u.Scheme, u.Fragment, u.RawQuery = "https", "", ""
	return u.String(), nil
}

// multistatus is a WebDAV answer (RFC 4918) with the properties CalDAV
// discovery and queries ask for (RFC 4791, RFC 6764).
type multistatus struct {
	Responses []struct {
		Href      string `xml:"DAV: href"`
		Propstats []struct {
			Status string `xml:"DAV: status"`
			Prop   struct {
				Principal struct {
					Href string `xml:"DAV: href"`
				} `xml:"DAV: current-user-principal"`
				Home struct {
					Href string `xml:"DAV: href"`
				} `xml:"urn:ietf:params:xml:ns:caldav calendar-home-set"`
				Name string `xml:"DAV: displayname"`
				Type struct {
					Calendar *struct{} `xml:"urn:ietf:params:xml:ns:caldav calendar"`
				} `xml:"DAV: resourcetype"`
				Colour string `xml:"http://apple.com/ns/ical/ calendar-color"`
				Comps  struct {
					Comps []struct {
						Name string `xml:"name,attr"`
					} `xml:"urn:ietf:params:xml:ns:caldav comp"`
				} `xml:"urn:ietf:params:xml:ns:caldav supported-calendar-component-set"`
				Data string `xml:"urn:ietf:params:xml:ns:caldav calendar-data"`
			} `xml:"DAV: prop"`
		} `xml:"DAV: propstat"`
	} `xml:"DAV: response"`
}

const (
	propfindPrincipal = `<?xml version="1.0" encoding="utf-8"?><d:propfind xmlns:d="DAV:"><d:prop><d:current-user-principal/></d:prop></d:propfind>`
	propfindHome      = `<?xml version="1.0" encoding="utf-8"?><d:propfind xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav"><d:prop><c:calendar-home-set/></d:prop></d:propfind>`
	propfindCalendars = `<?xml version="1.0" encoding="utf-8"?><d:propfind xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav" xmlns:a="http://apple.com/ns/ical/">` +
		`<d:prop><d:resourcetype/><d:displayname/><a:calendar-color/><c:supported-calendar-component-set/></d:prop></d:propfind>`
	reportEvents = `<?xml version="1.0" encoding="utf-8"?><c:calendar-query xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav">` +
		`<d:prop><c:calendar-data/></d:prop><c:filter><c:comp-filter name="VCALENDAR"><c:comp-filter name="VEVENT">` +
		`<c:time-range start="%s" end="%s"/></c:comp-filter></c:comp-filter></c:filter></c:calendar-query>`
)

// dav sends a WebDAV request with the account's password and reads its
// multistatus answer, following redirects with the same method, but only
// to https, where the password may go. It answers the address that
// answered, against which the answer's addresses are resolved.
func (r *Reader) dav(ctx context.Context, method, target, username, password, depth, payload string) (*multistatus, *url.URL, error) {
	client := *r.client
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	u, err := url.Parse(target)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	for range maxRedirects + 1 {
		if u.Scheme != "https" {
			return nil, nil, fmt.Errorf("%w: the server sends the password elsewhere than over https", ErrUnreadable)
		}
		req, err := http.NewRequestWithContext(ctx, method, u.String(), strings.NewReader(payload))
		if err != nil {
			return nil, nil, fmt.Errorf("%w: %v", ErrUnreadable, err)
		}
		req.SetBasicAuth(username, password)
		req.Header.Set("Content-Type", "application/xml; charset=utf-8")
		req.Header.Set("Depth", depth)
		req.Header.Set("User-Agent", "orbithek-calendar")
		resp, err := client.Do(req)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: %s", ErrUnreadable, reason(err))
		}
		switch resp.StatusCode {
		case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
			resp.Body.Close()
			next, err := u.Parse(resp.Header.Get("Location"))
			if err != nil || resp.Header.Get("Location") == "" {
				return nil, nil, fmt.Errorf("%w: the server redirects nowhere", ErrUnreadable)
			}
			u = next
			continue
		}
		data, err := body(resp)
		if err != nil {
			return nil, nil, err
		}
		var ms multistatus
		if err := xml.Unmarshal(data, &ms); err != nil {
			return nil, nil, fmt.Errorf("%w: the server's answer is no WebDAV answer", ErrUnreadable)
		}
		return &ms, u, nil
	}
	return nil, nil, fmt.Errorf("%w: the server redirects too often", ErrUnreadable)
}

// ok reports whether a propstat's status is a success, as one without a
// status is taken to be.
func ok(status string) bool { return status == "" || strings.Contains(status, " 200 ") }

// href resolves an address of an answer against the one that answered.
func href(base *url.URL, h string) string {
	u, err := base.Parse(strings.TrimSpace(h))
	if err != nil || h == "" {
		return ""
	}
	return u.String()
}

// Discover finds the calendars of a CalDAV account (RFC 6764): the
// account's principal, at the server's address or its well-known one,
// the principal's calendar home, and the calendars in it that hold
// events. A wrong password is refused by the server here.
func (r *Reader) Discover(ctx context.Context, server, username, password string) ([]Remote, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	find := func(target, body string, pick func(base *url.URL, ms *multistatus) string) (string, error) {
		ms, base, err := r.dav(ctx, "PROPFIND", target, username, password, "0", body)
		if err != nil {
			return "", err
		}
		return pick(base, ms), nil
	}
	principalOf := func(base *url.URL, ms *multistatus) string {
		for _, resp := range ms.Responses {
			for _, ps := range resp.Propstats {
				if ok(ps.Status) && ps.Prop.Principal.Href != "" {
					return href(base, ps.Prop.Principal.Href)
				}
			}
		}
		return ""
	}
	principal, err := find(server, propfindPrincipal, principalOf)
	if err != nil || principal == "" {
		u, _ := url.Parse(server)
		wellKnown := u.Scheme + "://" + u.Host + "/.well-known/caldav"
		if p, err2 := find(wellKnown, propfindPrincipal, principalOf); err2 == nil && p != "" {
			principal, err = p, nil
		} else if err == nil {
			err = fmt.Errorf("%w: the server names no account (no current-user-principal)", ErrUnreadable)
		}
	}
	if err != nil {
		return nil, err
	}
	home, err := find(principal, propfindHome, func(base *url.URL, ms *multistatus) string {
		for _, resp := range ms.Responses {
			for _, ps := range resp.Propstats {
				if ok(ps.Status) && ps.Prop.Home.Href != "" {
					return href(base, ps.Prop.Home.Href)
				}
			}
		}
		return ""
	})
	if err != nil {
		return nil, err
	}
	if home == "" {
		return nil, fmt.Errorf("%w: the account has no calendar home", ErrUnreadable)
	}
	ms, base, err := r.dav(ctx, "PROPFIND", home, username, password, "1", propfindCalendars)
	if err != nil {
		return nil, err
	}
	var out []Remote
	for _, resp := range ms.Responses {
		for _, ps := range resp.Propstats {
			if !ok(ps.Status) || ps.Prop.Type.Calendar == nil {
				continue
			}
			events := len(ps.Prop.Comps.Comps) == 0
			for _, c := range ps.Prop.Comps.Comps {
				events = events || strings.EqualFold(c.Name, "VEVENT")
			}
			id := href(base, resp.Href)
			if !events || id == "" {
				continue
			}
			name := strings.TrimSpace(ps.Prop.Name)
			if name == "" {
				name = strings.Trim(resp.Href[strings.LastIndex(strings.TrimSuffix(resp.Href, "/"), "/")+1:], "/")
			}
			out = append(out, Remote{ID: id, Name: name, Colour: colourOf(strings.TrimSpace(ps.Prop.Colour))})
		}
	}
	return out, nil
}

// ConnectCalDAV finds a CalDAV account's calendars, which proves its
// password, and keeps the account.
func (r *Reader) ConnectCalDAV(ctx context.Context, user, server, username, password string) (Account, []Remote, error) {
	srv, err := CalDAVServer(server)
	if err != nil {
		return Account{}, nil, err
	}
	if strings.TrimSpace(username) == "" || password == "" {
		return Account{}, nil, fmt.Errorf("%w: a CalDAV account needs its user name and password", ErrInvalid)
	}
	cals, err := r.Discover(ctx, srv, username, password)
	if err != nil {
		return Account{}, nil, err
	}
	a, err := r.store.Connect(user, Account{Kind: KindCalDAV, Name: strings.TrimSpace(username), Server: srv}, password)
	return a, cals, err
}

// Calendars lists the calendars of one of a person's accounts.
func (r *Reader) Calendars(ctx context.Context, user, account string) ([]Remote, error) {
	a, found := r.store.Account(user, account)
	if !found {
		return nil, ErrNotFound
	}
	if a.Kind == KindGoogle {
		if r.google == nil {
			return nil, fmt.Errorf("%w: Google is not set up on this server", ErrUnreadable)
		}
		return r.google.Calendars(ctx, user, account)
	}
	password, err := r.store.Secret(user, account)
	if err != nil {
		return nil, err
	}
	return r.Discover(ctx, a.Server, a.Name, password)
}

// readCalDAV reads a calendar of a CalDAV account in [from, to): the
// server answers the entries that touch it, each with its changed
// occurrences, and they are expanded here.
func (r *Reader) readCalDAV(ctx context.Context, user string, s Source, from, to time.Time) (*Calendar, error) {
	a, found := r.store.Account(user, s.Account)
	if !found {
		return nil, fmt.Errorf("%w: the account is no longer connected", ErrUnreadable)
	}
	password, err := r.store.Secret(user, s.Account)
	if err != nil {
		return nil, fmt.Errorf("%w: the account is no longer connected", ErrUnreadable)
	}
	const f = "20060102T150405Z"
	ms, _, err := r.dav(ctx, "REPORT", s.Calendar, a.Name, password, "1", fmt.Sprintf(reportEvents, from.UTC().Format(f), to.UTC().Format(f)))
	if err != nil {
		return nil, err
	}
	all := &Calendar{}
	for _, resp := range ms.Responses {
		for _, ps := range resp.Propstats {
			if !ok(ps.Status) || ps.Prop.Data == "" {
				continue
			}
			c, err := Parse([]byte(ps.Prop.Data))
			if err != nil {
				continue
			}
			if all.Zone == nil {
				all.Zone = c.Zone
			}
			all.entries = append(all.entries, c.entries...)
		}
	}
	return all, nil
}
