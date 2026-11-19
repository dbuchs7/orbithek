package auth

import (
	"errors"
	"strings"
	"testing"
)

// TestTheDisplayNameIsKeptPerUserAndSurvivesReload: the name a person
// chooses to be called by is recorded once, trimmed, cleared by an empty
// name, and a reload of the journal gives it back (ADR-0246).
func TestTheDisplayNameIsKeptPerUserAndSurvivesReload(t *testing.T) {
	a, j, _ := newAccounts(t)
	u, err := a.CreateUser("anna", "anna's password")
	noErr(t, err)
	if got := a.DisplayName(u.ID); got != "" {
		t.Fatalf("a new user has the display name %q", got)
	}
	noErr(t, a.SetDisplayName(u.ID, "  Anna Müller  "))
	if got := a.DisplayName(u.ID); got != "Anna Müller" {
		t.Fatalf("display name %q", got)
	}
	n := len(j.records)
	noErr(t, a.SetDisplayName(u.ID, "Anna Müller")) // already: nothing recorded
	if len(j.records) != n {
		t.Error("an unchanged display name was recorded again")
	}
	if err := a.SetDisplayName("nobody", "Anna"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown user: %v", err)
	}
	if got := a.DisplayName("nobody"); got != "" {
		t.Errorf("unknown user's display name: %q", got)
	}
	b, err := New(j.records, j.append, WithIterations(1000))
	noErr(t, err)
	if got := b.DisplayName(u.ID); got != "Anna Müller" {
		t.Fatalf("after reload: %q", got)
	}
	noErr(t, b.SetDisplayName(u.ID, "   "))
	c, err := New(j.records, j.append, WithIterations(1000))
	noErr(t, err)
	if got := c.DisplayName(u.ID); got != "" {
		t.Fatalf("cleared, after reload: %q", got)
	}
}

// TestAnInvalidDisplayNameIsRefused: a display name is at most 64
// characters and holds no control or formatting characters, which could
// break a line or turn the text around where it is shown.
func TestAnInvalidDisplayNameIsRefused(t *testing.T) {
	a, j, _ := newAccounts(t)
	u, err := a.CreateUser("anna", "anna's password")
	noErr(t, err)
	n := len(j.records)
	for _, bad := range []string{
		strings.Repeat("a", MaxDisplayName+1),
		"Anna\nMüller",
		"Anna\tMüller",
		"Anna‮Müller", // right-to-left override
		"Anna​Müller", // zero-width space
		"Anna\x00",
		"Anna\xff",
	} {
		if err := a.SetDisplayName(u.ID, bad); !errors.Is(err, ErrInvalid) {
			t.Errorf("%q: %v", bad, err)
		}
	}
	if len(j.records) != n {
		t.Error("a refused display name was recorded")
	}
	noErr(t, a.SetDisplayName(u.ID, strings.Repeat("ä", MaxDisplayName)))
}

// TestTheDisplayNameOfAnUnknownUserIsRefusedOnLoad: a journal that names
// no such user does not load.
func TestTheDisplayNameOfAnUnknownUserIsRefusedOnLoad(t *testing.T) {
	if _, err := New([][]byte{[]byte(`{"v":1,"kind":"DisplayNameSet","user":"ghost","displayName":"Ghost"}`)}, func([]byte) error { return nil }, WithIterations(1000)); err == nil {
		t.Fatal("the display name of an unknown user was loaded")
	}
}
