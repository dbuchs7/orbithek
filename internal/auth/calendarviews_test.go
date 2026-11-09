package auth

import (
	"errors"
	"strings"
	"testing"
)

// TestCalendarViewsAndTheirFeeds: a user saves, renames and removes views;
// a feed secret opens its view until it is replaced or revoked; nothing
// survives a reload but what was recorded, and the secret itself never is
// (ADR-0168).
func TestCalendarViewsAndTheirFeeds(t *testing.T) {
	a, j, _ := newAccounts(t)
	u, err := a.CreateUser("anna", "anna's password")
	noErr(t, err)
	b, err := a.CreateUser("ben", "ben's password")
	noErr(t, err)
	mine, err := a.SaveCalendarView(u.ID, CalendarView{Name: " My open tasks ", Params: "?who=me&status=open"})
	noErr(t, err)
	if mine.ID == "" || mine.Name != "My open tasks" || mine.Params != "who=me&status=open" || mine.Feed {
		t.Fatalf("saved %+v", mine)
	}
	team, err := a.SaveCalendarView(u.ID, CalendarView{Name: "Team", Params: "space=PLAT"})
	noErr(t, err)
	renamed, err := a.SaveCalendarView(u.ID, CalendarView{ID: team.ID, Name: "Platform team", Params: "space=PLAT&view=week"})
	noErr(t, err)
	if renamed.ID != team.ID || renamed.Name != "Platform team" {
		t.Fatalf("renamed %+v", renamed)
	}
	if got := a.CalendarViews(u.ID); len(got) != 2 || got[0].ID != mine.ID || got[1].Params != "space=PLAT&view=week" {
		t.Fatalf("views %+v", got)
	}
	if got := a.CalendarViews(b.ID); len(got) != 0 {
		t.Fatalf("ben sees anna's views: %+v", got)
	}

	secret, err := a.NewCalendarFeed(u.ID, mine.ID)
	noErr(t, err)
	if !strings.HasPrefix(secret, FeedPrefix) || len(secret) < 40 {
		t.Fatalf("secret %q", secret)
	}
	who, view, ok := a.CalendarFeed(secret)
	if !ok || who != u.ID || view.ID != mine.ID || !view.Feed || view.FeedCreated.IsZero() {
		t.Fatalf("feed resolves to %v %+v %v", who, view, ok)
	}
	for _, rec := range j.records {
		if strings.Contains(string(rec), secret) {
			t.Fatal("the journal holds the secret itself")
		}
	}
	// Saving the view again keeps its feed.
	_, err = a.SaveCalendarView(u.ID, CalendarView{ID: mine.ID, Name: "Mine", Params: "who=me"})
	noErr(t, err)
	if _, _, ok := a.CalendarFeed(secret); !ok {
		t.Fatal("renaming a view ended its feed")
	}
	// A new secret replaces the old one.
	next, err := a.NewCalendarFeed(u.ID, mine.ID)
	noErr(t, err)
	if _, _, ok := a.CalendarFeed(secret); ok {
		t.Fatal("a replaced secret still opens the feed")
	}
	if _, _, ok := a.CalendarFeed(next); !ok {
		t.Fatal("the new secret does not open the feed")
	}

	// A reload of the journal gives back views and feeds.
	r, err := New(j.records, j.append)
	noErr(t, err)
	if got := r.CalendarViews(u.ID); len(got) != 2 || !got[0].Feed {
		t.Fatalf("after reload %+v", got)
	}
	if who, _, ok := r.CalendarFeed(next); !ok || who != u.ID {
		t.Fatal("after reload the feed is gone")
	}

	noErr(t, a.RevokeCalendarFeed(u.ID, mine.ID))
	if _, _, ok := a.CalendarFeed(next); ok {
		t.Fatal("a revoked feed still opens")
	}
	feedOfTeam, err := a.NewCalendarFeed(u.ID, team.ID)
	noErr(t, err)
	noErr(t, a.RemoveCalendarView(u.ID, team.ID))
	if _, _, ok := a.CalendarFeed(feedOfTeam); ok {
		t.Fatal("the feed of a removed view still opens")
	}
	if got := a.CalendarViews(u.ID); len(got) != 1 {
		t.Fatalf("views after removal %+v", got)
	}

	for _, c := range []CalendarView{{Name: ""}, {Name: strings.Repeat("x", maxViewName+1)}, {Name: "a\nb"}, {Name: "ok", Params: strings.Repeat("p", maxViewParams+1)}} {
		if _, err := a.SaveCalendarView(u.ID, c); !errors.Is(err, ErrInvalid) {
			t.Errorf("%q: %v", c.Name, err)
		}
	}
	for _, f := range []func() error{
		func() error { _, err := a.SaveCalendarView(u.ID, CalendarView{ID: "nope", Name: "x"}); return err },
		func() error { return a.RemoveCalendarView(u.ID, "nope") },
		func() error { _, err := a.NewCalendarFeed(b.ID, mine.ID); return err },
		func() error { return a.RevokeCalendarFeed("nobody", mine.ID) },
		func() error { _, err := a.SaveCalendarView("nobody", CalendarView{Name: "x"}); return err },
	} {
		if err := f(); !errors.Is(err, ErrNotFound) {
			t.Errorf("want not found, got %v", err)
		}
	}
	if _, _, ok := a.CalendarFeed("ot_" + strings.Repeat("0", 64)); ok {
		t.Error("an API token is no feed secret")
	}
	if got := a.CalendarViews("nobody"); got != nil {
		t.Error("an unknown user has views")
	}
	for i := len(a.CalendarViews(u.ID)); i < maxCalendarViews; i++ {
		_, err := a.SaveCalendarView(u.ID, CalendarView{Name: "v"})
		noErr(t, err)
	}
	if _, err := a.SaveCalendarView(u.ID, CalendarView{Name: "one too many"}); !errors.Is(err, ErrInvalid) {
		t.Errorf("more than %d views: %v", maxCalendarViews, err)
	}
}
