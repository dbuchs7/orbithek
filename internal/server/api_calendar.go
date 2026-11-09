package server

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/dbuchs7/orbithek/internal/audit"
	"github.com/dbuchs7/orbithek/internal/auth"
	"github.com/dbuchs7/orbithek/internal/calendar"
	"github.com/dbuchs7/orbithek/internal/item"
)

// Feed window: what a subscribed calendar holds, around the day it reads
// the feed (ADR-0167).
const (
	feedDaysBack  = 90
	feedDaysAhead = 400
	// feedMaxItems bounds the items one feed reads, as the boards' limit
	// bounds a calendar in the browser.
	feedMaxItems = 2000
)

type calendarHandlers struct {
	itemHandlers
}

// calendarRoutes are a person's saved calendar views and the feed that
// subscribes to one (ADR-0167).
func calendarRoutes(d Deps) []Route {
	h := calendarHandlers{itemHandlers{d: d, log: d.Log}}
	if h.log == nil {
		h.log = slog.New(slog.DiscardHandler)
	}
	return []Route{
		{Method: http.MethodGet, Pattern: "/api/me/calendar-views", Summary: "List one's saved calendar views", Handler: http.HandlerFunc(h.list)},
		{Method: http.MethodPost, Pattern: "/api/me/calendar-views", Summary: "Save a calendar view under a name", Handler: http.HandlerFunc(h.create)},
		{Method: http.MethodPut, Pattern: "/api/me/calendar-views/{id}", Summary: "Rename a saved calendar view or change what it shows", Handler: http.HandlerFunc(h.update)},
		{Method: http.MethodDelete, Pattern: "/api/me/calendar-views/{id}", Summary: "Remove a saved calendar view and its feed", Handler: http.HandlerFunc(h.remove)},
		{Method: http.MethodPost, Pattern: "/api/me/calendar-views/{id}/feed", Summary: "Make a feed address for a saved calendar view, shown once; an earlier one stops working", Handler: http.HandlerFunc(h.newFeed)},
		{Method: http.MethodDelete, Pattern: "/api/me/calendar-views/{id}/feed", Summary: "End the feed of a saved calendar view", Handler: http.HandlerFunc(h.revokeFeed)},
		{Method: http.MethodGet, Pattern: "/calendar/{feed}", Summary: "A saved calendar view as iCalendar, opened by its feed's secret; it holds the key, title, time and a link of each entry its owner may read", Access: Public, Handler: http.HandlerFunc(h.feed)},
	}
}

func (h calendarHandlers) user(r *http.Request) auth.UserID {
	p, _ := PrincipalFrom(r.Context())
	return p.User
}

func (h calendarHandlers) viewError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, auth.ErrNotFound):
		writeError(w, http.StatusNotFound, "no such view")
	default:
		writeError(w, http.StatusInternalServerError, "the view was not changed")
	}
}

func (h calendarHandlers) list(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"views": nonNilSlice(h.d.DB.Accounts.CalendarViews(h.user(r)))})
}

type viewRequest struct {
	Name   string `json:"name"`
	Params string `json:"params"`
}

func (h calendarHandlers) create(w http.ResponseWriter, r *http.Request) {
	var req viewRequest
	if err := readJSON(w, r, &req); err != nil {
		badRequest(w, err)
		return
	}
	v, err := h.d.DB.Accounts.SaveCalendarView(h.user(r), auth.CalendarView{Name: req.Name, Params: req.Params})
	if err != nil {
		h.viewError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"view": v})
}

func (h calendarHandlers) update(w http.ResponseWriter, r *http.Request) {
	var req viewRequest
	if err := readJSON(w, r, &req); err != nil {
		badRequest(w, err)
		return
	}
	v, err := h.d.DB.Accounts.SaveCalendarView(h.user(r), auth.CalendarView{ID: r.PathValue("id"), Name: req.Name, Params: req.Params})
	if err != nil {
		h.viewError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"view": v})
}

func (h calendarHandlers) remove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	had := h.hasFeed(r, id)
	if err := h.d.DB.Accounts.RemoveCalendarView(h.user(r), id); err != nil {
		h.viewError(w, err)
		return
	}
	if had {
		h.d.DB.Audit.Record(audit.Entry{Event: audit.CalendarFeedRevoked, Actor: string(h.user(r)), Addr: clientAddr(r), Subject: id})
	}
	w.WriteHeader(http.StatusNoContent)
}

// hasFeed tells whether one of the caller's views has a feed.
func (h calendarHandlers) hasFeed(r *http.Request, id string) bool {
	for _, v := range h.d.DB.Accounts.CalendarViews(h.user(r)) {
		if v.ID == id {
			return v.Feed
		}
	}
	return false
}

func (h calendarHandlers) newFeed(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	secret, err := h.d.DB.Accounts.NewCalendarFeed(h.user(r), id)
	if err != nil {
		h.viewError(w, err)
		return
	}
	h.d.DB.Audit.Record(audit.Entry{Event: audit.CalendarFeedCreated, Actor: string(h.user(r)), Addr: clientAddr(r), Subject: id})
	var view auth.CalendarView
	for _, v := range h.d.DB.Accounts.CalendarViews(h.user(r)) {
		if v.ID == id {
			view = v
		}
	}
	writeJSON(w, http.StatusCreated, map[string]any{"view": view, "secret": secret, "url": origin(r) + "/calendar/" + secret + ".ics"})
}

func (h calendarHandlers) revokeFeed(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.d.DB.Accounts.RevokeCalendarFeed(h.user(r), id); err != nil {
		h.viewError(w, err)
		return
	}
	h.d.DB.Audit.Record(audit.Entry{Event: audit.CalendarFeedRevoked, Actor: string(h.user(r)), Addr: clientAddr(r), Subject: id})
	w.WriteHeader(http.StatusNoContent)
}

// feed answers a saved view as iCalendar to whoever holds its secret, as
// its owner reads the items: nothing the owner may not read is in it, and
// a feed whose owner is gone opens nothing (D10, ADR-0167).
func (h calendarHandlers) feed(w http.ResponseWriter, r *http.Request) {
	secret := strings.TrimSuffix(r.PathValue("feed"), ".ics")
	owner, view, ok := h.d.DB.Accounts.CalendarFeed(secret)
	if !ok {
		writeError(w, http.StatusNotFound, "no such calendar")
		return
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	from, to := today.AddDate(0, 0, -feedDaysBack), today.AddDate(0, 0, feedDaysAhead)
	v := calendar.ParseView(view.Params)
	var events []calendar.Event
	var err error
	h.d.DB.Read(func(s *item.Store) {
		fields := calendar.TypeFields(s.Types(), v)
		q := calendar.Query(v, fields, from, to)
		if q == "" {
			return
		}
		var items []item.Item
		items, _, _, err = s.As(string(owner)).SelectPage(item.Query{Space: v.Space}, q, h.queryEnv(), "", feedMaxItems)
		if err == nil {
			events = calendar.Events(items, fields, from, to.AddDate(0, 0, 1))
		}
	})
	if err != nil {
		// A saved query that no longer reads (a type renamed, a field
		// gone) is told, as the calendar tells it.
		writeError(w, http.StatusBadRequest, "the saved view cannot be read: "+err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.Header().Set("Content-Disposition", `inline; filename="calendar.ics"`)
	_, _ = w.Write(calendar.Feed(view.Name, origin(r), events, now))
}
