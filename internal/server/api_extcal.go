package server

import (
	"crypto/subtle"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/dbuchs7/orbithek/internal/audit"
	"github.com/dbuchs7/orbithek/internal/connector"
	"github.com/dbuchs7/orbithek/internal/extcal"
	"github.com/dbuchs7/orbithek/internal/site"
)

// googleCalendarCookie binds a begun connection of a Google account to the
// browser that began it, as connectCookie does for engines.
const googleCalendarCookie = "ot_calendar_state"

// GoogleCalendarCallback is the path Google sends a person back to; the
// public address and this path must be among the redirect addresses of
// the Google client (ADR-0172).
const GoogleCalendarCallback = "/api/calendar-accounts/google/callback"

// maxEventDays is the longest window one request reads.
const maxEventDays = 400

// CalendarClient makes the client people's calendars are read with:
// through the environment's proxy, with timeouts, following redirects only
// to https, and kept to public addresses unless OT_CALENDAR_ALLOW_PRIVATE
// allows others (ADR-0172).
func CalendarClient(getenv func(string) string) *http.Client {
	var transport http.RoundTripper = &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: extcal.ReadTimeout,
		MaxIdleConnsPerHost:   4,
		IdleConnTimeout:       90 * time.Second,
	}
	if getenv(extcal.AllowPrivateVar) != "on" {
		transport = connector.PublicOnlyUnless(transport, extcal.AllowPrivateVar)
	}
	return &http.Client{Timeout: extcal.ReadTimeout, Transport: transport, CheckRedirect: extcal.Redirects}
}

type extcalHandlers struct {
	authHandlers
	log *slog.Logger
}

// extcalRoutes let people show their calendars of other services in the
// wiki's calendar, and administrators allow it (ADR-0172).
func extcalRoutes(d Deps) []Route {
	h := extcalHandlers{authHandlers: authHandlers{d: d}, log: d.Log}
	if h.log == nil {
		h.log = slog.New(slog.DiscardHandler)
	}
	return []Route{
		{Method: http.MethodGet, Pattern: "/api/settings/calendars", Summary: "Whether people may show calendars of other services, whether Google is set up and the redirect address to register with it, and whether calendars in the server's own network are read (administrators)", Handler: http.HandlerFunc(h.settings)},
		{Method: http.MethodPut, Pattern: "/api/settings/calendars", Summary: "Allow people to show calendars of other services, or not: external (administrators)", Handler: http.HandlerFunc(h.setSettings)},
		{Method: http.MethodGet, Pattern: "/api/me/calendars", Summary: "One's calendars of other services and accounts at Google or CalDAV servers, without their addresses or credentials; enabled says whether the server allows them", Handler: http.HandlerFunc(h.list)},
		{Method: http.MethodPost, Pattern: "/api/me/calendars", Summary: "Show a calendar of another service: an iCalendar address (kind ics, url, read once to try it), or a calendar of one of one's accounts (kind google or caldav, account, calendar); with name and colour", Handler: http.HandlerFunc(h.add)},
		{Method: http.MethodPut, Pattern: "/api/me/calendars/{id}", Summary: "Rename, recolour, hide or show one of one's calendars of other services", Handler: http.HandlerFunc(h.update)},
		{Method: http.MethodDelete, Pattern: "/api/me/calendars/{id}", Summary: "Remove one of one's calendars of other services; its address is destroyed", Handler: http.HandlerFunc(h.remove)},
		{Method: http.MethodGet, Pattern: "/api/me/calendars/events", Summary: "The entries of one's shown calendars of other services between from and to (at most 400 days), as their services answer them: title, place and time; zone is one's time zone, for times a calendar gives without one; problems names the calendars that could not be read", Handler: http.HandlerFunc(h.events)},
		{Method: http.MethodPost, Pattern: "/api/me/calendar-accounts", Summary: "Connect an account at a CalDAV server (kind caldav: server, username, password — for Apple iCloud https://caldav.icloud.com with the Apple ID and an app-specific password); answers its calendars", Handler: http.HandlerFunc(h.connectCalDAV)},
		{Method: http.MethodGet, Pattern: "/api/me/calendar-accounts/{id}/calendars", Summary: "The calendars one of one's accounts holds, to choose which are shown", Handler: http.HandlerFunc(h.accountCalendars)},
		{Method: http.MethodDelete, Pattern: "/api/me/calendar-accounts/{id}", Summary: "Disconnect one of one's accounts: its calendars go, its credential is destroyed, and Google's grant is revoked", Handler: http.HandlerFunc(h.disconnect)},
		{Method: http.MethodGet, Pattern: "/api/me/calendar-accounts/google/connect", Summary: "Begin connecting one's Google account; redirects to Google, where one signs in and allows this server to read one's calendars", Handler: http.HandlerFunc(h.connectGoogle)},
		{Method: http.MethodGet, Pattern: GoogleCalendarCallback, Summary: "Google's redirect back after one allowed or refused this server; keeps the account, bound to the browser that began it, and goes on to the calendar", Access: Public, Handler: http.HandlerFunc(h.googleCallback)},
	}
}

func (h extcalHandlers) user(r *http.Request) string {
	p, _ := PrincipalFrom(r.Context())
	return string(p.User)
}

func (h extcalHandlers) enabled() bool { return h.d.DB.Site.Calendars().External }

// refuseOff answers that the server does not allow calendars of other
// services, if it does not.
func (h extcalHandlers) refuseOff(w http.ResponseWriter) bool {
	if h.enabled() {
		return false
	}
	writeError(w, http.StatusForbidden, "calendars of other services are turned off on this server")
	return true
}

func (h extcalHandlers) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, extcal.ErrInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, extcal.ErrNotFound):
		writeError(w, http.StatusNotFound, "no such calendar or account")
	case errors.Is(err, extcal.ErrTooMany):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, extcal.ErrUnreadable), errors.Is(err, extcal.ErrRevoked):
		writeError(w, http.StatusBadGateway, err.Error())
	default:
		h.log.Warn("calendars of other services", "err", err)
		writeError(w, http.StatusInternalServerError, "the calendar was not changed")
	}
}

func (h extcalHandlers) settings(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.adminOnly(w, r); !ok {
		return
	}
	g := h.d.Calendars.Google()
	redirect := ""
	if g != nil {
		redirect = g.RedirectURL
	}
	writeJSON(w, http.StatusOK, map[string]any{"external": h.enabled(), "google": g != nil, "googleRedirect": redirect,
		"allowPrivate": h.d.CalendarAllowPrivate})
}

func (h extcalHandlers) setSettings(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.adminOnly(w, r)
	if !ok {
		return
	}
	var req site.Calendars
	if err := readJSON(w, r, &req); err != nil {
		badRequest(w, err)
		return
	}
	before := h.enabled()
	if err := h.d.DB.Site.SetCalendars(actor, req); err != nil {
		writeError(w, http.StatusInternalServerError, "setting not saved")
		return
	}
	if before != req.External {
		state := "off"
		if req.External {
			state = "on"
		}
		h.record(audit.Entry{Event: audit.CalendarsChanged, Actor: actor, Addr: clientAddr(r), Detail: map[string]string{"external": state}})
	}
	h.settings(w, r)
}

func (h extcalHandlers) list(w http.ResponseWriter, r *http.Request) {
	user := h.user(r)
	writeJSON(w, http.StatusOK, map[string]any{"enabled": h.enabled(), "google": h.d.Calendars.Google() != nil,
		"sources": nonNilSlice(h.d.Calendars.Store().Sources(user)), "accounts": nonNilSlice(h.d.Calendars.Store().Accounts(user))})
}

type sourceRequest struct {
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Colour   string `json:"colour"`
	URL      string `json:"url"`
	Account  string `json:"account"`
	Calendar string `json:"calendar"`
	Hidden   bool   `json:"hidden"`
}

func (h extcalHandlers) add(w http.ResponseWriter, r *http.Request) {
	if h.refuseOff(w) {
		return
	}
	var req sourceRequest
	if err := readJSON(w, r, &req); err != nil {
		badRequest(w, err)
		return
	}
	user := h.user(r)
	store := h.d.Calendars.Store()
	var (
		src    extcal.Source
		err    error
		detail = map[string]string{"kind": req.Kind}
	)
	switch req.Kind {
	case extcal.KindICS:
		addr, aerr := extcal.ICSAddress(req.URL)
		if aerr != nil {
			h.fail(w, aerr)
			return
		}
		// The address is read once, so a wrong one is told now, not
		// later in the calendar; a calendar that names itself names the
		// calendar when the person gave no name.
		cal, ferr := h.d.Calendars.Fetch(r.Context(), addr)
		if ferr != nil {
			h.fail(w, ferr)
			return
		}
		if strings.TrimSpace(req.Name) == "" {
			req.Name = cal.Name
		}
		src, err = store.AddICS(user, req.Name, req.Colour, addr)
		detail["host"] = src.Host
	case extcal.KindGoogle, extcal.KindCalDAV:
		a, found := store.Account(user, req.Account)
		if !found || a.Kind != req.Kind {
			h.fail(w, extcal.ErrNotFound)
			return
		}
		// Only a calendar the account holds is shown: its address is
		// where the account's credential goes.
		cals, cerr := h.d.Calendars.Calendars(r.Context(), user, a.ID)
		if cerr != nil {
			h.fail(w, cerr)
			return
		}
		known := false
		for _, c := range cals {
			if c.ID == req.Calendar {
				known = true
				if strings.TrimSpace(req.Name) == "" {
					req.Name = c.Name
				}
			}
		}
		if !known {
			h.fail(w, extcal.ErrNotFound)
			return
		}
		src, err = store.AddCalendar(user, a.ID, req.Calendar, req.Name, req.Colour)
		detail["account"] = a.Name
	default:
		writeError(w, http.StatusBadRequest, "kind is ics, google or caldav")
		return
	}
	if err != nil {
		h.fail(w, err)
		return
	}
	h.record(audit.Entry{Event: audit.CalendarSourceAdded, Actor: user, Addr: clientAddr(r), Subject: src.ID, Detail: detail})
	writeJSON(w, http.StatusCreated, map[string]any{"source": src})
}

func (h extcalHandlers) update(w http.ResponseWriter, r *http.Request) {
	var req sourceRequest
	if err := readJSON(w, r, &req); err != nil {
		badRequest(w, err)
		return
	}
	user, id := h.user(r), r.PathValue("id")
	src, err := h.d.Calendars.Store().Update(user, id, req.Name, req.Colour, req.Hidden)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"source": src})
}

func (h extcalHandlers) remove(w http.ResponseWriter, r *http.Request) {
	user, id := h.user(r), r.PathValue("id")
	src, found := h.d.Calendars.Store().Source(user, id)
	if err := h.d.Calendars.Store().Remove(user, id); err != nil {
		h.fail(w, err)
		return
	}
	h.d.Calendars.Forget(user, id)
	if found {
		h.record(audit.Entry{Event: audit.CalendarSourceRemoved, Actor: user, Addr: clientAddr(r), Subject: id, Detail: map[string]string{"kind": src.Kind}})
	}
	w.WriteHeader(http.StatusNoContent)
}

// zoneName is an IANA zone's name, as browsers report it.
var zoneName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_+\-/]{0,63}$`)

// moment reads a time of a window: RFC 3339, or a date at midnight UTC.
func moment(s string) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	t, err := time.Parse(time.DateOnly, s)
	return t, err == nil
}

// shownEvent is an occurrence as the calendar shows it: an all-day one by
// its dates, the end the day after its last.
type shownEvent struct {
	Source   string `json:"source"`
	Title    string `json:"title"`
	Location string `json:"location,omitempty"`
	Start    string `json:"start"`
	End      string `json:"end"`
	AllDay   bool   `json:"allDay"`
}

func (h extcalHandlers) events(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	from, ok1 := moment(q.Get("from"))
	to, ok2 := moment(q.Get("to"))
	if !ok1 || !ok2 || !to.After(from) || to.Sub(from) > maxEventDays*24*time.Hour {
		writeError(w, http.StatusBadRequest, "from and to are a window of at most 400 days, as dates or RFC 3339 times")
		return
	}
	var reader *time.Location
	if z := q.Get("zone"); z != "" && zoneName.MatchString(z) {
		reader, _ = time.LoadLocation(z)
	}
	if !h.enabled() {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": false, "events": []shownEvent{}, "problems": []any{}})
		return
	}
	evs, problems := h.d.Calendars.Events(r.Context(), h.user(r), from, to, reader)
	out := make([]shownEvent, 0, len(evs))
	for _, e := range evs {
		f := time.RFC3339
		if e.AllDay {
			f = time.DateOnly
		}
		out = append(out, shownEvent{Source: e.Source, Title: e.Title, Location: e.Location, Start: e.Start.UTC().Format(f), End: e.End.UTC().Format(f), AllDay: e.AllDay})
	}
	type problem struct {
		Source  string `json:"source"`
		Message string `json:"message"`
	}
	ps := make([]problem, 0, len(problems))
	for _, p := range problems {
		ps = append(ps, problem{p.Source, p.Message})
	}
	writeJSON(w, http.StatusOK, map[string]any{"enabled": true, "events": out, "problems": ps})
}

type accountRequest struct {
	Kind     string `json:"kind"`
	Server   string `json:"server"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h extcalHandlers) connectCalDAV(w http.ResponseWriter, r *http.Request) {
	if h.refuseOff(w) {
		return
	}
	var req accountRequest
	if err := readJSON(w, r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if req.Kind != extcal.KindCalDAV {
		writeError(w, http.StatusBadRequest, "kind is caldav; a Google account is connected through Google")
		return
	}
	user := h.user(r)
	a, cals, err := h.d.Calendars.ConnectCalDAV(r.Context(), user, req.Server, req.Username, req.Password)
	if err != nil {
		h.fail(w, err)
		return
	}
	h.record(audit.Entry{Event: audit.CalendarAccountConnected, Actor: user, Addr: clientAddr(r), Subject: a.ID, Detail: map[string]string{"kind": a.Kind, "account": a.Name, "server": a.Server}})
	writeJSON(w, http.StatusCreated, map[string]any{"account": a, "calendars": nonNilSlice(cals)})
}

func (h extcalHandlers) accountCalendars(w http.ResponseWriter, r *http.Request) {
	if h.refuseOff(w) {
		return
	}
	cals, err := h.d.Calendars.Calendars(r.Context(), h.user(r), r.PathValue("id"))
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"calendars": nonNilSlice(cals)})
}

func (h extcalHandlers) disconnect(w http.ResponseWriter, r *http.Request) {
	user, id := h.user(r), r.PathValue("id")
	store := h.d.Calendars.Store()
	a, found := store.Account(user, id)
	if !found {
		h.fail(w, extcal.ErrNotFound)
		return
	}
	if g := h.d.Calendars.Google(); g != nil && a.Kind == extcal.KindGoogle {
		g.Revoke(r.Context(), user, id)
	}
	for _, s := range store.Sources(user) {
		if s.Account == id {
			h.d.Calendars.Forget(user, s.ID)
		}
	}
	if err := store.Disconnect(user, id); err != nil {
		h.fail(w, err)
		return
	}
	h.record(audit.Entry{Event: audit.CalendarAccountDisconnected, Actor: user, Addr: clientAddr(r), Subject: id, Detail: map[string]string{"kind": a.Kind, "account": a.Name}})
	w.WriteHeader(http.StatusNoContent)
}

// toCalendar sends the browser to the calendar, saying how connecting an
// account went.
func toCalendar(w http.ResponseWriter, r *http.Request, param, value string) {
	http.Redirect(w, r, "/?"+url.Values{param: {value}}.Encode()+"#/calendar", http.StatusFound)
}

func (h extcalHandlers) connectGoogle(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFrom(r.Context())
	g := h.d.Calendars.Google()
	switch {
	case p.Via != "session":
		writeError(w, http.StatusForbidden, "connect a Google account from the web UI, signed in")
		return
	case !h.enabled():
		toCalendar(w, r, "calendar_error", "off")
		return
	case g == nil:
		toCalendar(w, r, "calendar_error", "config")
		return
	}
	authURL, state, err := g.Begin(string(p.User))
	if err != nil {
		toCalendar(w, r, "calendar_error", "unavailable")
		return
	}
	// Lax, so the cookie comes back with Google's cross-site redirect.
	http.SetCookie(w, &http.Cookie{Name: googleCalendarCookie, Value: state, Path: "/api/calendar-accounts/", MaxAge: int(extcal.FlowLifetime / time.Second),
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: isSecure(r)})
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (h extcalHandlers) googleCallback(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: googleCalendarCookie, Value: "", Path: "/api/calendar-accounts/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: isSecure(r)})
	q := r.URL.Query()
	state := q.Get("state")
	c, err := r.Cookie(googleCalendarCookie)
	g := h.d.Calendars.Google()
	if err != nil || state == "" || g == nil || subtle.ConstantTimeCompare([]byte(c.Value), []byte(state)) != 1 {
		toCalendar(w, r, "calendar_error", "state")
		return
	}
	user, ok := g.Pending(state)
	if !ok {
		toCalendar(w, r, "calendar_error", "state")
		return
	}
	if q.Get("error") != "" {
		g.Cancel(state)
		toCalendar(w, r, "calendar_error", "denied")
		return
	}
	if !h.enabled() {
		g.Cancel(state)
		toCalendar(w, r, "calendar_error", "off")
		return
	}
	a, err := g.Finish(r.Context(), state, q.Get("code"))
	if err != nil {
		h.log.Warn("connecting a Google account failed", "err", err)
		toCalendar(w, r, "calendar_error", "unavailable")
		return
	}
	h.record(audit.Entry{Event: audit.CalendarAccountConnected, Actor: user, Addr: clientAddr(r), Subject: a.ID, Detail: map[string]string{"kind": a.Kind, "account": a.Name}})
	toCalendar(w, r, "calendar_account", a.ID)
}
