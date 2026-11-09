package auth

import (
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Bounds of a user's saved calendar views (ADR-0167).
const (
	maxCalendarViews = 50
	maxViewName      = 80
	maxViewParams    = 2000
)

// FeedPrefix starts every calendar feed secret, so secret scanners can find
// a leaked feed address (ADR-0167).
const FeedPrefix = "otcal_"

// CalendarView is a calendar a user saved under a name (ADR-0167): the
// parameters of its address, as the calendar writes them, and whether it
// may be subscribed to as a feed. The feed's secret is shown once, when it
// is made, and kept only as its hash.
type CalendarView struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Params      string    `json:"params"`
	Feed        bool      `json:"feed"`
	FeedCreated time.Time `json:"feedCreated,omitzero"`
}

// viewRecord is a saved view as the journal keeps it, with its feed's hash.
type viewRecord struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Params      string    `json:"params"`
	FeedHash    string    `json:"feedHash,omitempty"`
	FeedCreated time.Time `json:"feedCreated,omitzero"`
}

func (v viewRecord) public() CalendarView {
	return CalendarView{ID: v.ID, Name: v.Name, Params: v.Params, Feed: v.FeedHash != "", FeedCreated: v.FeedCreated}
}

// feedRef names the view a feed secret opens.
type feedRef struct {
	user UserID
	view string
}

// CalendarViews lists a user's saved views in the order they were made.
func (a *Accounts) CalendarViews(id UserID) []CalendarView {
	a.mu.Lock()
	defer a.mu.Unlock()
	u, ok := a.users[id]
	if !ok {
		return nil
	}
	out := make([]CalendarView, len(u.views))
	for i, v := range u.views {
		out[i] = v.public()
	}
	return out
}

// validViewText reports whether a name or the parameters of a view are
// plain text of at most max runes, without control characters.
func validViewText(s string, max int) bool {
	if utf8.RuneCountInString(s) > max {
		return false
	}
	return !strings.ContainsFunc(s, unicode.IsControl)
}

// SaveCalendarView saves a view under its name: a new one when v has no ID,
// else the user's view of that ID, whose feed stays as it is.
func (a *Accounts) SaveCalendarView(id UserID, v CalendarView) (CalendarView, error) {
	v.Name = strings.TrimSpace(v.Name)
	v.Params = strings.TrimPrefix(strings.TrimSpace(v.Params), "?")
	if v.Name == "" || !validViewText(v.Name, maxViewName) {
		return CalendarView{}, fmt.Errorf("%w: a view needs a name of at most %d characters", ErrInvalid, maxViewName)
	}
	if !validViewText(v.Params, maxViewParams) {
		return CalendarView{}, fmt.Errorf("%w: a view's address is at most %d characters", ErrInvalid, maxViewParams)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	u, ok := a.users[id]
	if !ok {
		return CalendarView{}, ErrNotFound
	}
	next := slices.Clone(u.views)
	if v.ID == "" {
		if len(next) >= maxCalendarViews {
			return CalendarView{}, fmt.Errorf("%w: at most %d saved views", ErrInvalid, maxCalendarViews)
		}
		vid, err := a.newID(8)
		if err != nil {
			return CalendarView{}, err
		}
		v.ID = vid
		next = append(next, viewRecord{ID: vid, Name: v.Name, Params: v.Params})
	} else {
		i := slices.IndexFunc(next, func(r viewRecord) bool { return r.ID == v.ID })
		if i < 0 {
			return CalendarView{}, ErrNotFound
		}
		next[i].Name, next[i].Params = v.Name, v.Params
	}
	if err := a.record(event{Kind: calendarViewsSet, User: id, Views: next}); err != nil {
		return CalendarView{}, err
	}
	return a.viewOf(id, v.ID), nil
}

// viewOf is one of a user's views; the caller holds the lock.
func (a *Accounts) viewOf(id UserID, view string) CalendarView {
	for _, r := range a.users[id].views {
		if r.ID == view {
			return r.public()
		}
	}
	return CalendarView{}
}

// RemoveCalendarView deletes a user's view, and with it its feed.
func (a *Accounts) RemoveCalendarView(id UserID, view string) error {
	return a.changeView(id, view, func(next []viewRecord, i int) []viewRecord { return slices.Delete(next, i, i+1) })
}

// NewCalendarFeed makes a feed secret for a user's view and returns it,
// once; a feed the view had stops working (ADR-0167).
func (a *Accounts) NewCalendarFeed(id UserID, view string) (string, error) {
	raw, err := a.newID(32)
	if err != nil {
		return "", err
	}
	secret := FeedPrefix + raw
	now := a.now().UTC()
	err = a.changeView(id, view, func(next []viewRecord, i int) []viewRecord {
		next[i].FeedHash, next[i].FeedCreated = hashSecret(secret), now
		return next
	})
	if err != nil {
		return "", err
	}
	return secret, nil
}

// RevokeCalendarFeed ends the feed of a user's view.
func (a *Accounts) RevokeCalendarFeed(id UserID, view string) error {
	return a.changeView(id, view, func(next []viewRecord, i int) []viewRecord {
		next[i].FeedHash, next[i].FeedCreated = "", time.Time{}
		return next
	})
}

// changeView records the user's views as change leaves them; i is the
// index of the view named.
func (a *Accounts) changeView(id UserID, view string, change func([]viewRecord, int) []viewRecord) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	u, ok := a.users[id]
	if !ok {
		return ErrNotFound
	}
	next := slices.Clone(u.views)
	i := slices.IndexFunc(next, func(r viewRecord) bool { return r.ID == view })
	if i < 0 {
		return ErrNotFound
	}
	return a.record(event{Kind: calendarViewsSet, User: id, Views: change(next, i)})
}

// CalendarFeed resolves a feed secret to the user and the view it opens.
func (a *Accounts) CalendarFeed(secret string) (UserID, CalendarView, bool) {
	if !strings.HasPrefix(secret, FeedPrefix) {
		return "", CalendarView{}, false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	ref, ok := a.feeds[hashSecret(secret)]
	if !ok {
		return "", CalendarView{}, false
	}
	v := a.viewOf(ref.user, ref.view)
	return ref.user, v, v.ID != ""
}

// applyViews replaces a user's views and the feeds they open.
func (a *Accounts) applyViews(e event) error {
	u, ok := a.users[e.User]
	if !ok {
		return fmt.Errorf("calendar views of unknown user %s", e.User)
	}
	for _, v := range u.views {
		if v.FeedHash != "" {
			delete(a.feeds, v.FeedHash)
		}
	}
	u.views = slices.Clone(e.Views)
	for _, v := range u.views {
		if v.FeedHash != "" {
			a.feeds[v.FeedHash] = feedRef{user: e.User, view: v.ID}
		}
	}
	return nil
}
