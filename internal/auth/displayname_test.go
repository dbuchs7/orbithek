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

// TestAFirstDisplayNameIsTakenOnlyWhileNoneWasEverChosen: the name a
// sign-in provider asserts becomes the display name of an account that
// never had one, and never replaces a name, nor one cleared on purpose
// (ADR-draft-the-provider-names-a-first-display-name).
func TestAFirstDisplayNameIsTakenOnlyWhileNoneWasEverChosen(t *testing.T) {
	a, j, _ := newAccounts(t)
	u, err := a.CreateUser("anna", "anna's password")
	noErr(t, err)
	taken, err := a.SetFirstDisplayName(u.ID, "  Anna Muster ")
	noErr(t, err)
	if !taken || a.DisplayName(u.ID) != "Anna Muster" {
		t.Fatalf("taken %v, display name %q", taken, a.DisplayName(u.ID))
	}
	n := len(j.records)
	if taken, err := a.SetFirstDisplayName(u.ID, "Anna Changed"); err != nil || taken || a.DisplayName(u.ID) != "Anna Muster" {
		t.Fatalf("a second name replaced the first: %v %v %q", taken, err, a.DisplayName(u.ID))
	}
	if len(j.records) != n {
		t.Error("a name not taken was recorded")
	}

	// Cleared by its owner: the provider's name does not come back, also
	// not after a reload of the journal.
	noErr(t, a.SetDisplayName(u.ID, ""))
	b, err := New(j.records, j.append, WithIterations(1000))
	noErr(t, err)
	if taken, err := b.SetFirstDisplayName(u.ID, "Anna Muster"); err != nil || taken || b.DisplayName(u.ID) != "" {
		t.Fatalf("a cleared display name came back: %v %v %q", taken, err, b.DisplayName(u.ID))
	}

	// Empty and refused names are not taken, and leave the account free to
	// take a valid one later.
	v, err := b.CreateUser("vera", "vera's password")
	noErr(t, err)
	for _, bad := range []string{"", "   ", "Vera\nMuster", strings.Repeat("v", MaxDisplayName+1)} {
		taken, err := b.SetFirstDisplayName(v.ID, bad)
		if taken || (strings.TrimSpace(bad) != "" && !errors.Is(err, ErrInvalid)) {
			t.Errorf("%q: taken %v, %v", bad, taken, err)
		}
	}
	if taken, err := b.SetFirstDisplayName(v.ID, "Vera Muster"); err != nil || !taken {
		t.Fatalf("a valid name after refused ones: %v %v", taken, err)
	}
	if _, err := b.SetFirstDisplayName("nobody", "X"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown user: %v", err)
	}
}
