package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/dbuchs7/orbithek/internal/extcal"
)

// fakeServices answers as an iCalendar address, Google and a CalDAV
// server do, as far as the calendar asks them.
func fakeServices(t *testing.T) *httptest.Server {
	t.Helper()
	day := time.Now().UTC().AddDate(0, 0, 2).Format("20060102")
	ics := "BEGIN:VCALENDAR\r\nX-WR-CALNAME:Holidays\r\nBEGIN:VEVENT\r\nUID:h\r\nSUMMARY:Holiday\r\nDTSTART;VALUE=DATE:" + day + "\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	ms := func(body string) string {
		return `<?xml version="1.0"?><d:multistatus xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav">` + body + `</d:multistatus>`
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /secret/holidays.ics", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, ics) })
	mux.HandleFunc("GET /not-a-calendar", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "<html></html>") })
	mux.HandleFunc("POST /google/token", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		if r.Form.Get("grant_type") == "authorization_code" && r.Form.Get("code") != "good" {
			w.WriteHeader(400)
			io.WriteString(w, `{"error":"invalid_grant"}`)
			return
		}
		io.WriteString(w, `{"access_token":"access","refresh_token":"refresh","expires_in":3600,"token_type":"Bearer"}`)
	})
	mux.HandleFunc("POST /google/revoke", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("GET /google/api/users/me/calendarList", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"items":[{"id":"bert@example.org","summary":"Bert","primary":true,"backgroundColor":"#7986cb"}]}`)
	})
	mux.HandleFunc("GET /google/api/calendars/{id}/events", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now().UTC().AddDate(0, 0, 1).Truncate(time.Hour)
		json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{{"id": "g1", "summary": "Lunch", "location": "Canteen",
			"start": map[string]string{"dateTime": start.Format(time.RFC3339)}, "end": map[string]string{"dateTime": start.Add(time.Hour).Format(time.RFC3339)}}}})
	})
	mux.HandleFunc("/dav/", func(w http.ResponseWriter, r *http.Request) {
		if u, p, _ := r.BasicAuth(); u != "bert" || p != "app-password" {
			w.WriteHeader(401)
			return
		}
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(207)
		switch {
		case strings.Contains(string(body), "current-user-principal"):
			io.WriteString(w, ms(`<d:response><d:href>/dav/</d:href><d:propstat><d:prop><d:current-user-principal><d:href>/dav/p/</d:href></d:current-user-principal></d:prop></d:propstat></d:response>`))
		case strings.Contains(string(body), "calendar-home-set"):
			io.WriteString(w, ms(`<d:response><d:href>/dav/p/</d:href><d:propstat><d:prop><c:calendar-home-set><d:href>/dav/home/</d:href></c:calendar-home-set></d:prop></d:propstat></d:response>`))
		case r.Method == "PROPFIND":
			io.WriteString(w, ms(`<d:response><d:href>/dav/home/choir/</d:href><d:propstat><d:prop><d:resourcetype><c:calendar/></d:resourcetype><d:displayname>Choir</d:displayname></d:prop></d:propstat></d:response>`))
		default:
			io.WriteString(w, ms(`<d:response><d:href>/dav/home/choir/a.ics</d:href><d:propstat><d:prop><c:calendar-data>BEGIN:VCALENDAR
BEGIN:VEVENT
UID:c
SUMMARY:Rehearsal
DTSTART:`+day+`T180000Z
DTEND:`+day+`T200000Z
END:VEVENT
END:VCALENDAR</c:calendar-data></d:prop></d:propstat></d:response>`))
		}
	})
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// TestCalendarsOfOtherServices: off until an administrator turns them on;
// then a person shows an iCalendar address, a CalDAV account's calendar
// and a Google account's calendar beside the wiki's, nobody else sees or
// changes them, their credentials are never answered, and adding,
// removing, connecting and disconnecting is in the audit trail (ADR-0174).
func TestCalendarsOfOtherServices(t *testing.T) {
	srv := fakeServices(t)
	var reader *extcal.Reader
	w := newWorld(t, func(d *Deps) {
		g := extcal.NewGoogle("client", "secret", "https://wiki.example.org"+GoogleCalendarCallback)
		g.AuthURL, g.TokenURL, g.RevokeURL, g.API = srv.URL+"/google/auth", srv.URL+"/google/token", srv.URL+"/google/revoke", srv.URL+"/google/api"
		reader = extcal.NewReader(d.DB.Calendars, srv.Client(), g)
		d.Calendars = reader
	})
	b := w.bert
	now := time.Now().UTC()
	window := "/api/me/calendars/events?from=" + now.Format(time.DateOnly) + "&to=" + now.AddDate(0, 0, 7).Format(time.DateOnly) + "&zone=Europe/Zurich"
	ics := `{"kind":"ics","name":"","colour":"#336699","url":"` + srv.URL + `/secret/holidays.ics"}`

	// Off by default: nothing is added or read.
	if got := b.expect("GET", "/api/me/calendars", "", http.StatusOK); got["enabled"] != false || got["google"] != true {
		t.Fatalf("list: %v", got)
	}
	b.expect("POST", "/api/me/calendars", ics, http.StatusForbidden)
	b.expect("POST", "/api/me/calendar-accounts", `{"kind":"caldav"}`, http.StatusForbidden)
	if got := b.expect("GET", window, "", http.StatusOK); got["enabled"] != false || len(got["events"].([]any)) != 0 {
		t.Fatalf("events while off: %v", got)
	}
	b.expect("PUT", "/api/settings/calendars", `{"external":true}`, http.StatusForbidden)
	b.expect("GET", "/api/settings/calendars", "", http.StatusForbidden)
	set := w.admin.expect("PUT", "/api/settings/calendars", `{"external":true}`, http.StatusOK)
	if set["external"] != true || set["google"] != true || set["googleRedirect"] != "https://wiki.example.org"+GoogleCalendarCallback || set["allowPrivate"] != false {
		t.Fatalf("settings: %v", set)
	}
	w.admin.expect("PUT", "/api/settings/calendars", `{"external":true}`, http.StatusOK)
	w.admin.expect("PUT", "/api/settings/calendars", `{"external":`, http.StatusBadRequest)

	// An iCalendar address: tried, named by its calendar, kept sealed.
	b.expect("POST", "/api/me/calendars", `{"kind":"ics","name":"x","colour":"#336699","url":"http://example.org/a.ics"}`, http.StatusBadRequest)
	b.expect("POST", "/api/me/calendars", `{"kind":"ics","name":"x","colour":"#336699","url":"`+srv.URL+`/not-a-calendar"}`, http.StatusBadGateway)
	b.expect("POST", "/api/me/calendars", `{"kind":"other"}`, http.StatusBadRequest)
	b.expect("POST", "/api/me/calendars", `{"kind":`, http.StatusBadRequest)
	created := b.expect("POST", "/api/me/calendars", ics, http.StatusCreated)
	src := created["source"].(map[string]any)
	if src["name"] != "Holidays" || src["host"] != "127.0.0.1" {
		t.Fatalf("source %v", src)
	}
	id := src["id"].(string)
	listed := b.do("GET", "/api/me/calendars", "").Body.String()
	if strings.Contains(listed, "/secret/") {
		t.Fatalf("the list holds the address: %s", listed)
	}
	evs := b.expect("GET", window, "", http.StatusOK)
	if e := evs["events"].([]any); len(e) != 1 || e[0].(map[string]any)["title"] != "Holiday" || e[0].(map[string]any)["allDay"] != true || e[0].(map[string]any)["source"] != id {
		t.Fatalf("events %v", evs)
	}
	for _, bad := range []string{"?from=x&to=y", "?from=2026-01-01&to=2025-01-01", "?from=2026-01-01&to=2028-01-01"} {
		b.expect("GET", "/api/me/calendars/events"+bad, "", http.StatusBadRequest)
	}

	// Nobody else sees, changes or removes it.
	if got := w.vera.expect("GET", "/api/me/calendars", "", http.StatusOK); len(got["sources"].([]any)) != 0 {
		t.Fatalf("vera sees bert's calendars: %v", got)
	}
	w.vera.expect("PUT", "/api/me/calendars/"+id, `{"name":"x","colour":"#000000"}`, http.StatusNotFound)
	w.vera.expect("DELETE", "/api/me/calendars/"+id, "", http.StatusNotFound)
	if got := w.vera.expect("GET", window, "", http.StatusOK); len(got["events"].([]any)) != 0 {
		t.Fatal("vera reads bert's calendar")
	}

	// Hidden, it is not read.
	b.expect("PUT", "/api/me/calendars/"+id, `{"name":"Holidays","colour":"#336699","hidden":true}`, http.StatusOK)
	if got := b.expect("GET", window, "", http.StatusOK); len(got["events"].([]any)) != 0 {
		t.Fatal("a hidden calendar is shown")
	}
	b.expect("PUT", "/api/me/calendars/"+id, `{"name":"","colour":"#336699"}`, http.StatusBadRequest)
	b.expect("PUT", "/api/me/calendars/"+id, `{"name":`, http.StatusBadRequest)

	// A CalDAV account: its calendars, one of them shown.
	b.expect("POST", "/api/me/calendar-accounts", `{"kind":"caldav","server":"`+srv.URL+`/dav/","username":"bert","password":"wrong"}`, http.StatusBadGateway)
	b.expect("POST", "/api/me/calendar-accounts", `{"kind":"google"}`, http.StatusBadRequest)
	b.expect("POST", "/api/me/calendar-accounts", `{"kind":`, http.StatusBadRequest)
	acc := b.expect("POST", "/api/me/calendar-accounts", `{"kind":"caldav","server":"`+srv.URL+`/dav/","username":"bert","password":"app-password"}`, http.StatusCreated)
	accID := acc["account"].(map[string]any)["id"].(string)
	cals := acc["calendars"].([]any)
	if len(cals) != 1 || cals[0].(map[string]any)["name"] != "Choir" {
		t.Fatalf("calendars %v", acc)
	}
	calID := cals[0].(map[string]any)["id"].(string)
	if strings.Contains(b.do("GET", "/api/me/calendars", "").Body.String(), "app-password") {
		t.Fatal("the list holds the password")
	}
	b.expect("GET", "/api/me/calendar-accounts/"+accID+"/calendars", "", http.StatusOK)
	w.vera.expect("GET", "/api/me/calendar-accounts/"+accID+"/calendars", "", http.StatusNotFound)
	b.expect("POST", "/api/me/calendars", `{"kind":"caldav","account":"`+accID+`","calendar":"https://elsewhere.example/cal/","colour":"#00aa00"}`, http.StatusNotFound)
	b.expect("POST", "/api/me/calendars", `{"kind":"google","account":"`+accID+`","calendar":"`+calID+`","colour":"#00aa00"}`, http.StatusNotFound)
	choir := b.expect("POST", "/api/me/calendars", `{"kind":"caldav","account":"`+accID+`","calendar":"`+calID+`","colour":"#00aa00"}`, http.StatusCreated)
	if choir["source"].(map[string]any)["name"] != "Choir" {
		t.Fatalf("choir %v", choir)
	}

	// A Google account, connected through Google.
	anonGoogle := &client{t: t, h: b.h}
	if rec := anonGoogle.do("GET", "/api/me/calendar-accounts/google/connect", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("connect anonymously: %d", rec.Code)
	}
	rec := b.do("GET", "/api/me/calendar-accounts/google/connect", "")
	if rec.Code != http.StatusFound {
		t.Fatalf("connect: %d %s", rec.Code, rec.Body)
	}
	consent, _ := url.Parse(rec.Header().Get("Location"))
	state := consent.Query().Get("state")
	var stateCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == googleCalendarCookie {
			stateCookie = c
		}
	}
	if !strings.HasPrefix(consent.String(), srv.URL+"/google/auth") || state == "" || stateCookie == nil || stateCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("consent %s, cookie %v", consent, stateCookie)
	}
	callback := func(query string, cookie *http.Cookie) string {
		req := httptest.NewRequest("GET", GoogleCalendarCallback+"?"+query, nil)
		if cookie != nil {
			req.AddCookie(cookie)
		}
		rec := httptest.NewRecorder()
		b.h.ServeHTTP(rec, req)
		if rec.Code != http.StatusFound {
			t.Fatalf("callback: %d %s", rec.Code, rec.Body)
		}
		return rec.Header().Get("Location")
	}
	if loc := callback("state="+state+"&code=good", nil); !strings.Contains(loc, "calendar_error=state") {
		t.Fatalf("without the cookie: %s", loc)
	}
	if loc := callback("state=other&code=good", &http.Cookie{Name: googleCalendarCookie, Value: "other"}); !strings.Contains(loc, "calendar_error=state") {
		t.Fatalf("an unknown state: %s", loc)
	}
	if loc := callback("state="+state+"&code=good", stateCookie); !strings.Contains(loc, "calendar_account=") || !strings.HasSuffix(loc, "#/calendar") {
		t.Fatalf("connected: %s", loc)
	}
	var google map[string]any
	for _, a := range b.expect("GET", "/api/me/calendars", "", http.StatusOK)["accounts"].([]any) {
		if a.(map[string]any)["kind"] == "google" {
			google = a.(map[string]any)
		}
	}
	if google == nil || google["name"] != "bert@example.org" {
		t.Fatalf("the Google account: %v", google)
	}
	gid := google["id"].(string)
	b.expect("POST", "/api/me/calendars", `{"kind":"google","account":"`+gid+`","calendar":"bert@example.org","name":"Mine","colour":"#7986cb"}`, http.StatusCreated)
	// Refused at Google, or a code Google does not take.
	rec = b.do("GET", "/api/me/calendar-accounts/google/connect", "")
	consent, _ = url.Parse(rec.Header().Get("Location"))
	if loc := callback("state="+consent.Query().Get("state")+"&error=access_denied", &http.Cookie{Name: googleCalendarCookie, Value: consent.Query().Get("state")}); !strings.Contains(loc, "calendar_error=denied") {
		t.Fatalf("denied: %s", loc)
	}
	rec = b.do("GET", "/api/me/calendar-accounts/google/connect", "")
	consent, _ = url.Parse(rec.Header().Get("Location"))
	if loc := callback("state="+consent.Query().Get("state")+"&code=bad", &http.Cookie{Name: googleCalendarCookie, Value: consent.Query().Get("state")}); !strings.Contains(loc, "calendar_error=unavailable") {
		t.Fatalf("a bad code: %s", loc)
	}

	// All three beside each other.
	b.expect("PUT", "/api/me/calendars/"+id, `{"name":"Holidays","colour":"#336699","hidden":false}`, http.StatusOK)
	all := b.expect("GET", window, "", http.StatusOK)
	titles := map[string]bool{}
	for _, e := range all["events"].([]any) {
		titles[e.(map[string]any)["title"].(string)] = true
	}
	if !titles["Holiday"] || !titles["Rehearsal"] || !titles["Lunch"] || len(all["problems"].([]any)) != 0 {
		t.Fatalf("all: %v", all)
	}

	// Disconnecting removes the account's calendars; removing a calendar
	// works also while the server has them off.
	b.expect("DELETE", "/api/me/calendar-accounts/"+gid, "", http.StatusNoContent)
	b.expect("DELETE", "/api/me/calendar-accounts/"+gid, "", http.StatusNotFound)
	b.expect("DELETE", "/api/me/calendar-accounts/"+accID, "", http.StatusNoContent)
	if got := b.expect("GET", "/api/me/calendars", "", http.StatusOK); len(got["sources"].([]any)) != 1 || len(got["accounts"].([]any)) != 0 {
		t.Fatalf("after disconnecting: %v", got)
	}
	w.admin.expect("PUT", "/api/settings/calendars", `{"external":false}`, http.StatusOK)
	b.expect("GET", "/api/me/calendar-accounts/"+accID+"/calendars", "", http.StatusForbidden)
	if loc := b.do("GET", "/api/me/calendar-accounts/google/connect", "").Header().Get("Location"); !strings.Contains(loc, "calendar_error=off") {
		t.Fatalf("connect while off: %s", loc)
	}
	b.expect("DELETE", "/api/me/calendars/"+id, "", http.StatusNoContent)
	b.expect("DELETE", "/api/me/calendars/"+id, "", http.StatusNotFound)

	trail := w.admin.expect("GET", "/api/audit?limit=200", "", http.StatusOK)
	counts := map[string]int{}
	for _, e := range trail["entries"].([]any) {
		counts[e.(map[string]any)["event"].(string)]++
	}
	if counts["calendars.change"] != 2 || counts["calendar-source.add"] != 3 || counts["calendar-source.remove"] != 1 ||
		counts["calendar-account.connect"] != 2 || counts["calendar-account.disconnect"] != 2 {
		t.Errorf("audit: %v", counts)
	}
}

// TestGoogleNotSetUp: without Google's client, connecting a Google account
// says so, and the callback opens nothing.
func TestGoogleNotSetUp(t *testing.T) {
	w := newWorld(t)
	w.admin.expect("PUT", "/api/settings/calendars", `{"external":true}`, http.StatusOK)
	if loc := w.bert.do("GET", "/api/me/calendar-accounts/google/connect", "").Header().Get("Location"); !strings.Contains(loc, "calendar_error=config") {
		t.Fatalf("connect: %s", loc)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", GoogleCalendarCallback+"?state=x", nil)
	req.AddCookie(&http.Cookie{Name: googleCalendarCookie, Value: "x"})
	w.bert.h.ServeHTTP(rec, req)
	if !strings.Contains(rec.Header().Get("Location"), "calendar_error=state") {
		t.Fatalf("callback: %s", rec.Header().Get("Location"))
	}
	if got := w.admin.expect("GET", "/api/settings/calendars", "", http.StatusOK); got["google"] != false || got["googleRedirect"] != "" {
		t.Fatalf("settings: %v", got)
	}
	tok := w.bert.expect("POST", "/api/tokens", `{"name":"t","scope":"read","days":1}`, http.StatusCreated)
	api := &client{t: t, h: w.bert.h, bearer: tok["secret"].(string)}
	api.expect("GET", "/api/me/calendar-accounts/google/connect", "", http.StatusForbidden)
	c := CalendarClient(func(string) string { return "" })
	if c.CheckRedirect == nil || c.Timeout != extcal.ReadTimeout {
		t.Fatal("the calendar client")
	}
	open := CalendarClient(func(string) string { return "on" })
	if _, guarded := open.Transport.(*http.Transport); !guarded {
		t.Fatal("OT_CALENDAR_ALLOW_PRIVATE=on leaves the transport unguarded")
	}
}
