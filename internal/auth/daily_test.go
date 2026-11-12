package auth

import (
	"errors"
	"testing"
)

// TestTheDailyRoomIsKeptPerUserAndSurvivesReload: choosing the room of
// one's daily notes is recorded once, cleared by an empty key, and a
// reload of the journal gives it back (ADR-0203).
func TestTheDailyRoomIsKeptPerUserAndSurvivesReload(t *testing.T) {
	a, j, _ := newAccounts(t)
	u, err := a.CreateUser("anna", "anna's password")
	noErr(t, err)
	if got := a.DailyRoom(u.ID); got != "" {
		t.Fatalf("a new user has the daily room %q", got)
	}
	noErr(t, a.SetDailyRoom(u.ID, "PLAT"))
	n := len(j.records)
	noErr(t, a.SetDailyRoom(u.ID, "PLAT")) // already: nothing recorded
	if len(j.records) != n {
		t.Error("an unchanged room was recorded again")
	}
	if got := a.DailyRoom(u.ID); got != "PLAT" {
		t.Fatalf("room %q", got)
	}
	for _, bad := range []string{"plat", "1A", "A-B", " "} {
		if err := a.SetDailyRoom(u.ID, bad); !errors.Is(err, ErrInvalid) {
			t.Errorf("%q: %v", bad, err)
		}
	}
	if err := a.SetDailyRoom("nobody", "PLAT"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown user: %v", err)
	}
	if got := a.DailyRoom("nobody"); got != "" {
		t.Errorf("unknown user's room: %q", got)
	}
	b, err := New(j.records, j.append, WithIterations(1000))
	noErr(t, err)
	if got := b.DailyRoom(u.ID); got != "PLAT" {
		t.Fatalf("after reload: %q", got)
	}
	noErr(t, b.SetDailyRoom(u.ID, ""))
	c, err := New(j.records, j.append, WithIterations(1000))
	noErr(t, err)
	if got := c.DailyRoom(u.ID); got != "" {
		t.Fatalf("cleared, after reload: %q", got)
	}
}

// TestTheDailyRoomOfAnUnknownUserIsRefusedOnLoad: a journal that names no
// such user does not load.
func TestTheDailyRoomOfAnUnknownUserIsRefusedOnLoad(t *testing.T) {
	if _, err := New([][]byte{[]byte(`{"v":1,"kind":"DailyRoomSet","user":"ghost","room":"PLAT"}`)}, func([]byte) error { return nil }, WithIterations(1000)); err == nil {
		t.Fatal("the daily room of an unknown user was loaded")
	}
}
