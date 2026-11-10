package extcal

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const holidays = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nX-WR-CALNAME:Holidays\r\nBEGIN:VEVENT\r\nUID:h1\r\nSUMMARY:Christmas\r\nDTSTART;VALUE=DATE:20261225\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"

func readerFor(t *testing.T, srv *httptest.Server) (*Reader, *Store) {
	t.Helper()
	s, _, _ := newStore(t)
	return NewReader(s, srv.Client(), nil), s
}

// TestAnICalendarAddressIsReadAndKept: an address is read once while it is
// fresh, again when it has aged; a failure is kept for a shorter while,
// says why without the address, and hidden calendars are not read.
func TestAnICalendarAddressIsReadAndKept(t *testing.T) {
	var hits atomic.Int32
	status := atomic.Int32{}
	status.Store(200)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path == "/big.ics" {
			io.WriteString(w, "BEGIN:VCALENDAR\r\n"+strings.Repeat("X-PAD:"+strings.Repeat("x", 1000)+"\r\n", maxBody/1000+10))
			return
		}
		if r.URL.Path == "/html" {
			io.WriteString(w, "<html></html>")
			return
		}
		w.WriteHeader(int(status.Load()))
		io.WriteString(w, holidays)
	}))
	defer srv.Close()
	r, s := readerFor(t, srv)
	now := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return now }
	src, err := s.AddICS("anna", "Holidays", "#336699", srv.URL+"/secret-token/basic.ics")
	if err != nil {
		t.Fatal(err)
	}
	hidden, _ := s.AddICS("anna", "Hidden", "#336699", srv.URL+"/hidden.ics")
	s.Update("anna", hidden.ID, "Hidden", "#336699", true)
	from, to := day("2026-12-01"), day("2027-01-01")
	evs, problems := r.Events(context.Background(), "anna", from, to, time.UTC)
	if len(problems) != 0 || len(evs) != 1 || evs[0].Title != "Christmas" || evs[0].Source != src.ID || !evs[0].AllDay {
		t.Fatalf("%+v %+v", evs, problems)
	}
	r.Events(context.Background(), "anna", from, to, time.UTC)
	if hits.Load() != 1 {
		t.Fatalf("read %d times while fresh", hits.Load())
	}
	now = now.Add(keepFor)
	status.Store(http.StatusNotFound)
	_, problems = r.Events(context.Background(), "anna", from, to, time.UTC)
	if hits.Load() != 2 || len(problems) != 1 || !strings.Contains(problems[0].Message, "404") || strings.Contains(problems[0].Message, "secret-token") {
		t.Fatalf("aged: %d reads, %+v", hits.Load(), problems)
	}
	now = now.Add(keepFailure)
	status.Store(200)
	if evs, problems = r.Events(context.Background(), "anna", from, to, time.UTC); len(evs) != 1 || len(problems) != 0 {
		t.Fatalf("after the failure: %+v %+v", evs, problems)
	}
	r.Forget("anna", src.ID)
	r.Events(context.Background(), "anna", from, to, time.UTC)
	if hits.Load() != 4 {
		t.Fatalf("forgotten: %d reads", hits.Load())
	}

	// Trying an address before it is kept.
	if _, err := r.Fetch(context.Background(), srv.URL+"/html"); !errors.Is(err, ErrUnreadable) {
		t.Errorf("html: %v", err)
	}
	if _, err := r.Fetch(context.Background(), srv.URL+"/big.ics"); err == nil || !strings.Contains(err.Error(), "larger") {
		t.Errorf("too big: %v", err)
	}
	if _, err := r.Fetch(context.Background(), "https://127.0.0.1:1/x.ics"); err == nil || strings.Contains(err.Error(), "127.0.0.1:1/x.ics") {
		t.Errorf("unreachable: %v", err)
	}
	for code, word := range map[int]string{401: "refuses", 410: "knows no", 500: "answers 500"} {
		if msg := (&StatusError{Status: code}).Error(); !strings.Contains(msg, word) {
			t.Errorf("%d: %s", code, msg)
		}
	}
	if reason(context.DeadlineExceeded) != "the service did not answer in time" {
		t.Error("a timeout")
	}
	if _, ok := r.Store().Source("anna", src.ID); !ok || r.Google() != nil {
		t.Error("the reader's store and Google")
	}
}

// fakeGoogle is Google's OAuth and Calendar API, as far as they are used.
func fakeGoogle(t *testing.T, refreshes *atomic.Int32) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		switch r.Form.Get("grant_type") {
		case "authorization_code":
			if r.Form.Get("code") != "good-code" || r.Form.Get("code_verifier") == "" || r.Form.Get("client_secret") != "secret" {
				w.WriteHeader(400)
				io.WriteString(w, `{"error":"invalid_grant"}`)
				return
			}
			io.WriteString(w, `{"access_token":"access-1","refresh_token":"refresh-1","expires_in":3600,"token_type":"Bearer"}`)
		case "refresh_token":
			refreshes.Add(1)
			if r.Form.Get("refresh_token") == "withdrawn" {
				w.WriteHeader(400)
				io.WriteString(w, `{"error":"invalid_grant"}`)
				return
			}
			io.WriteString(w, `{"access_token":"access-2","refresh_token":"refresh-2","expires_in":3600,"token_type":"Bearer"}`)
		}
	})
	mux.HandleFunc("POST /revoke", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("GET /api/users/me/calendarList", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer access-") {
			w.WriteHeader(401)
			return
		}
		if r.URL.Query().Get("pageToken") == "" {
			io.WriteString(w, `{"items":[{"id":"anna@example.org","summary":"Anna","backgroundColor":"#9FC6E7","primary":true}],"nextPageToken":"p2"}`)
			return
		}
		io.WriteString(w, `{"items":[{"id":"team@group.calendar.google.com","summary":"Team","summaryOverride":"My team","backgroundColor":"bad"}]}`)
	})
	mux.HandleFunc("GET /api/calendars/{id}/events", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer expired" {
			w.WriteHeader(401)
			return
		}
		q := r.URL.Query()
		if q.Get("singleEvents") != "true" || q.Get("timeMin") == "" {
			w.WriteHeader(400)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{
			{"id": "e1", "status": "confirmed", "summary": "Review", "location": "Room 1", "start": map[string]string{"dateTime": "2026-12-03T10:00:00+01:00"}, "end": map[string]string{"dateTime": "2026-12-03T11:00:00+01:00"}},
			{"id": "e2", "status": "cancelled", "summary": "Gone", "start": map[string]string{"dateTime": "2026-12-04T10:00:00+01:00"}},
			{"id": "e3", "summary": "Holiday", "start": map[string]string{"date": "2026-12-08"}, "end": map[string]string{"date": "2026-12-09"}},
			{"id": "e4", "summary": "No end", "start": map[string]string{"date": "2026-12-10"}},
			{"id": "e5", "summary": "Broken", "start": map[string]string{"dateTime": "soon"}},
			{"id": "e6", "summary": "Broken day", "start": map[string]string{"date": "soon"}},
			{"id": "e7", "summary": "Backwards", "start": map[string]string{"dateTime": "2026-12-11T10:00:00Z"}, "end": map[string]string{"dateTime": "2026-12-11T09:00:00Z"}},
		}})
	})
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestAGoogleAccountIsConnectedAndRead(t *testing.T) {
	var refreshes atomic.Int32
	srv := fakeGoogle(t, &refreshes)
	if NewGoogle("", "secret", "https://wiki.example.org/cb") != nil {
		t.Fatal("Google without a client")
	}
	g := NewGoogle("client", "secret", "https://wiki.example.org/api/me/calendar-accounts/google/callback")
	g.AuthURL, g.TokenURL, g.RevokeURL, g.API = srv.URL+"/auth", srv.URL+"/token", srv.URL+"/revoke", srv.URL+"/api"
	s, _, _ := newStore(t)
	r := NewReader(s, srv.Client(), g)

	authURL, state, err := g.Begin("anna")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(authURL)
	q := u.Query()
	if q.Get("access_type") != "offline" || q.Get("prompt") != "consent" || q.Get("scope") != googleScope || q.Get("code_challenge") == "" || q.Get("state") != state {
		t.Fatalf("consent address %s", authURL)
	}
	if user, ok := g.Pending(state); !ok || user != "anna" {
		t.Fatal("pending")
	}
	if _, err := g.Finish(context.Background(), "other", "good-code"); !errors.Is(err, ErrNoFlow) {
		t.Fatalf("an unknown state: %v", err)
	}
	_, bad, _ := g.Begin("anna")
	if _, err := g.Finish(context.Background(), bad, "bad-code"); !errors.Is(err, ErrUnreadable) {
		t.Fatalf("a refused code: %v", err)
	}
	_, cancelled, _ := g.Begin("anna")
	g.Cancel(cancelled)
	if _, ok := g.Pending(cancelled); ok {
		t.Fatal("cancelled")
	}
	a, err := g.Finish(context.Background(), state, "good-code")
	if err != nil || a.Name != "anna@example.org" || a.Kind != KindGoogle {
		t.Fatalf("%+v %v", a, err)
	}
	cals, err := r.Calendars(context.Background(), "anna", a.ID)
	if err != nil || len(cals) != 2 || cals[0].Colour != "#9fc6e7" || !cals[0].Primary || cals[1].Name != "My team" || cals[1].Colour != "" {
		t.Fatalf("%+v %v", cals, err)
	}
	src, err := s.AddCalendar("anna", a.ID, cals[1].ID, "Team", "#9fc6e7")
	if err != nil {
		t.Fatal(err)
	}
	evs, problems := r.Events(context.Background(), "anna", day("2026-12-01"), day("2026-12-31"), time.UTC)
	if len(problems) != 0 {
		t.Fatal(problems)
	}
	want(t, events(evs),
		"Review 2026-12-03T09:00:00Z 2026-12-03T10:00:00Z",
		"Holiday 2026-12-08 2026-12-09",
		"No end 2026-12-10 2026-12-11",
		"Backwards 2026-12-11T10:00:00Z 2026-12-11T10:00:00Z")
	if refreshes.Load() != 0 {
		t.Fatal("the access token of the connection was not used")
	}

	// An expired access token is renewed, and a new refresh token kept.
	g.access["anna\x00"+a.ID].Expiry = time.Now().Add(-time.Hour)
	r.Forget("anna", src.ID)
	if _, problems := r.Events(context.Background(), "anna", day("2026-12-01"), day("2026-12-31"), time.UTC); len(problems) != 0 || refreshes.Load() != 1 {
		t.Fatalf("renewed: %v, %d", problems, refreshes.Load())
	}
	if tok, _ := s.Secret("anna", a.ID); tok != "refresh-2" {
		t.Fatalf("the new refresh token: %q", tok)
	}
	// Google refusing the access token drops it.
	g.access["anna\x00"+a.ID].AccessToken = "expired"
	r.Forget("anna", src.ID)
	if _, problems := r.Events(context.Background(), "anna", day("2026-12-01"), day("2026-12-31"), time.UTC); len(problems) != 1 {
		t.Fatal("a refused access token is a problem")
	}
	if _, held := g.access["anna\x00"+a.ID]; held {
		t.Fatal("a refused access token is kept")
	}
	// A withdrawn refresh token is told as such.
	s.Renew("anna", a.ID, "withdrawn")
	r.Forget("anna", src.ID)
	if _, problems := r.Events(context.Background(), "anna", day("2026-12-01"), day("2026-12-31"), time.UTC); len(problems) != 1 || !strings.Contains(problems[0].Message, "withdrawn") {
		t.Fatalf("withdrawn: %v", problems)
	}
	g.Revoke(context.Background(), "anna", a.ID)
	g.Revoke(context.Background(), "anna", "gone")

	// Without Google, a Google calendar cannot be read.
	plain := NewReader(s, srv.Client(), nil)
	if _, problems := plain.Events(context.Background(), "anna", day("2026-12-01"), day("2026-12-31"), time.UTC); len(problems) != 1 {
		t.Fatal("Google without its set-up")
	}
	if _, err := plain.Calendars(context.Background(), "anna", a.ID); err == nil {
		t.Fatal("listing without Google")
	}
	if _, err := plain.Calendars(context.Background(), "anna", "nothing"); !errors.Is(err, ErrNotFound) {
		t.Fatal("an unknown account")
	}
}

func events(shown []Shown) []Event {
	out := make([]Event, len(shown))
	for i, s := range shown {
		out[i] = s.Event
	}
	return out
}

// fakeCalDAV answers as iCloud does: the well-known address redirects,
// the principal and the home are on another path, and a calendar holds a
// weekly entry with a changed occurrence in a resource of its own.
func fakeCalDAV(t *testing.T) *httptest.Server {
	t.Helper()
	ms := func(body string) string {
		return `<?xml version="1.0"?><d:multistatus xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav" xmlns:a="http://apple.com/ns/ical/">` + body + `</d:multistatus>`
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		user, pass, _ := r.BasicAuth()
		if user != "anna@icloud.com" || pass != "app-password" {
			w.WriteHeader(401)
			return
		}
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/xml")
		switch {
		case r.URL.Path == "/":
			w.WriteHeader(404)
		case r.URL.Path == "/.well-known/caldav":
			http.Redirect(w, r, "/dav/", http.StatusMovedPermanently)
		case r.URL.Path == "/dav/" && strings.Contains(string(body), "current-user-principal"):
			w.WriteHeader(207)
			io.WriteString(w, ms(`<d:response><d:href>/dav/</d:href><d:propstat><d:prop><d:current-user-principal><d:href>/123/principal/</d:href></d:current-user-principal></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`))
		case r.URL.Path == "/123/principal/":
			w.WriteHeader(207)
			io.WriteString(w, ms(`<d:response><d:href>/123/principal/</d:href><d:propstat><d:prop><c:calendar-home-set><d:href>/123/calendars/</d:href></c:calendar-home-set></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`))
		case r.URL.Path == "/123/calendars/" && r.Method == "PROPFIND":
			if r.Header.Get("Depth") != "1" {
				w.WriteHeader(400)
				return
			}
			w.WriteHeader(207)
			io.WriteString(w, ms(`<d:response><d:href>/123/calendars/</d:href><d:propstat><d:prop><d:resourcetype><d:collection/></d:resourcetype></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`+
				`<d:response><d:href>/123/calendars/home/</d:href><d:propstat><d:prop><d:resourcetype><d:collection/><c:calendar/></d:resourcetype><d:displayname>Home</d:displayname><a:calendar-color>#FF2968FF</a:calendar-color><c:supported-calendar-component-set><c:comp name="VEVENT"/></c:supported-calendar-component-set></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`+
				`<d:response><d:href>/123/calendars/tasks/</d:href><d:propstat><d:prop><d:resourcetype><d:collection/><c:calendar/></d:resourcetype><d:displayname>Reminders</d:displayname><c:supported-calendar-component-set><c:comp name="VTODO"/></c:supported-calendar-component-set></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`+
				`<d:response><d:href>/123/calendars/work/</d:href><d:propstat><d:prop><d:resourcetype><d:collection/><c:calendar/></d:resourcetype></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat><d:propstat><d:prop><d:displayname/></d:prop><d:status>HTTP/1.1 404 Not Found</d:status></d:propstat></d:response>`))
		case r.URL.Path == "/123/calendars/home/" && r.Method == "REPORT":
			if !strings.Contains(string(body), `time-range start="2026`) {
				w.WriteHeader(400)
				return
			}
			w.WriteHeader(207)
			series := "BEGIN:VCALENDAR\nBEGIN:VEVENT\nUID:s\nSUMMARY:Choir\nDTSTART;TZID=Europe/Zurich:20261202T190000\nDTEND;TZID=Europe/Zurich:20261202T210000\nRRULE:FREQ=WEEKLY\nEND:VEVENT\nBEGIN:VEVENT\nUID:s\nRECURRENCE-ID;TZID=Europe/Zurich:20261209T190000\nSUMMARY:Choir concert\nDTSTART;TZID=Europe/Zurich:20261209T180000\nDTEND;TZID=Europe/Zurich:20261209T220000\nEND:VEVENT\nEND:VCALENDAR"
			io.WriteString(w, ms(`<d:response><d:href>/123/calendars/home/s.ics</d:href><d:propstat><d:prop><c:calendar-data>`+series+`</c:calendar-data></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`+
				`<d:response><d:href>/123/calendars/home/broken.ics</d:href><d:propstat><d:prop><c:calendar-data>nonsense</c:calendar-data></d:prop></d:propstat></d:response>`))
		default:
			w.WriteHeader(404)
		}
	})
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestACalDAVAccountIsConnectedAndRead(t *testing.T) {
	srv := fakeCalDAV(t)
	r, s := readerFor(t, srv)
	if _, _, err := r.ConnectCalDAV(context.Background(), "anna", srv.URL, "anna@icloud.com", "wrong"); !errors.Is(err, ErrRefused) || errors.Is(err, ErrUnreadable) {
		t.Fatalf("a wrong password: %v", err)
	}
	if len(s.Accounts("anna")) != 0 {
		t.Fatal("an account with a wrong password is kept")
	}
	for _, bad := range []string{"http://dav.example.org", "https://user:pw@dav.example.org", "nonsense"} {
		if _, _, err := r.ConnectCalDAV(context.Background(), "anna", bad, "anna", "pw"); !errors.Is(err, ErrInvalid) {
			t.Errorf("%q: %v", bad, err)
		}
	}
	if _, _, err := r.ConnectCalDAV(context.Background(), "anna", srv.URL, "", "pw"); !errors.Is(err, ErrInvalid) {
		t.Error("no user name")
	}
	// A user name copied with spaces around it is the same name.
	a, cals, err := r.ConnectCalDAV(context.Background(), "anna", srv.URL+"/#x", "  anna@icloud.com ", "app-password")
	if err != nil {
		t.Fatal(err)
	}
	if len(cals) != 2 || cals[0].Name != "Home" || cals[0].Colour != "#ff2968" || cals[0].ID != srv.URL+"/123/calendars/home/" || cals[1].Name != "work" {
		t.Fatalf("calendars %+v", cals)
	}
	again, err := r.Calendars(context.Background(), "anna", a.ID)
	if err != nil || len(again) != 2 {
		t.Fatalf("listed again: %v %v", again, err)
	}
	if _, err := s.AddCalendar("anna", a.ID, cals[0].ID, "Home", "#ff2968"); err != nil {
		t.Fatal(err)
	}
	evs, problems := r.Events(context.Background(), "anna", day("2026-12-01"), day("2026-12-17"), time.UTC)
	if len(problems) != 0 {
		t.Fatal(problems)
	}
	want(t, events(evs),
		"Choir 2026-12-02T18:00:00Z 2026-12-02T20:00:00Z",
		"Choir concert 2026-12-09T17:00:00Z 2026-12-09T21:00:00Z",
		"Choir 2026-12-16T18:00:00Z 2026-12-16T20:00:00Z")

	// A server that sends the password to plain http, or redirects for
	// ever, is not followed.
	loop := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/plain":
			http.Redirect(w, r, "http://dav.example.org/", http.StatusFound)
		case "/nowhere":
			w.WriteHeader(http.StatusFound)
		case "/xml":
			w.WriteHeader(207)
			io.WriteString(w, "not xml <")
		default:
			http.Redirect(w, r, r.URL.Path, http.StatusTemporaryRedirect)
		}
	}))
	defer loop.Close()
	for path, word := range map[string]string{"/plain": "https", "/loop": "too often", "/nowhere": "nowhere", "/xml": "no WebDAV"} {
		if _, err := r.Discover(context.Background(), loop.URL+path, "a", "b"); err == nil || !strings.Contains(err.Error(), word) {
			t.Errorf("%s: %v", path, err)
		}
	}
}

func TestSpansAndColours(t *testing.T) {
	f, e := span(utc("2026-11-30T23:00:00Z"), utc("2027-01-11T00:00:00Z"))
	if f.Format(time.DateOnly) != "2026-10-25" || e.Format(time.DateOnly) != "2027-02-08" {
		t.Errorf("span %v %v", f, e)
	}
	for in, out := range map[string]string{"#AABBCC": "#aabbcc", "#AABBCCFF": "#aabbcc", "red": "", "": ""} {
		if colourOf(in) != out {
			t.Errorf("%q: %q", in, colourOf(in))
		}
	}
	if _, err := CalDAVServer(AppleServer); err != nil {
		t.Error(err)
	}
	if href(&url.URL{}, "") != "" || href(&url.URL{Scheme: "https", Host: "x"}, "%zz") != "" {
		t.Error("an empty or broken href")
	}
	s, _, _ := newStore(t)
	r := NewReader(s, http.DefaultClient, nil)
	s.add("anna", Source{Kind: "other", Name: "x", Colour: "#000000"}, "")
	if _, problems := r.Events(context.Background(), "anna", day("2026-12-01"), day("2026-12-02"), nil); len(problems) != 1 {
		t.Error("an unknown kind")
	}
}

func TestRedirectsKeepToHTTPS(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "http://example.org/", nil)
	if Redirects(req, nil) == nil {
		t.Error("a redirect to http")
	}
	req, _ = http.NewRequest(http.MethodGet, "https://example.org/", nil)
	if Redirects(req, make([]*http.Request, maxRedirects+1)) == nil || Redirects(req, nil) != nil {
		t.Error("redirects")
	}
}

// Apple's app-specific passwords are letters and dashes: spaces a copy
// brings along are dropped. Other servers' passwords stay as typed.
func TestAnApplePasswordLosesItsSpaces(t *testing.T) {
	cases := []struct{ server, in, want string }{
		{AppleServer, " abcd-efgh-ijkl-mnop\n", "abcd-efgh-ijkl-mnop"},
		{AppleServer, "abcd efgh ijkl mnop", "abcdefghijklmnop"},
		{"https://p42-caldav.icloud.com/", " x ", "x"},
		{"https://dav.example.org/", " pass word ", " pass word "},
		{"https://icloud.com.example.org/", " x ", " x "},
		{"::", " x ", " x "},
	}
	for _, c := range cases {
		if got := cleanPassword(c.server, c.in); got != c.want {
			t.Errorf("%s %q: %q, want %q", c.server, c.in, got, c.want)
		}
	}
}

// A refusal of the credentials is told apart from other failures, also
// when an account's calendars are listed again later.
func TestARefusedPasswordIsToldApart(t *testing.T) {
	for status, want := range map[int]bool{401: true, 403: true, 404: false, 500: false} {
		err := refused(&StatusError{Status: status})
		if errors.Is(err, ErrRefused) != want {
			t.Errorf("%d: %v", status, err)
		}
	}
	if refused(nil) != nil {
		t.Error("no error became one")
	}
}
