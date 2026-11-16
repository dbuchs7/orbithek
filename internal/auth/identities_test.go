package auth

import (
	"errors"
	"testing"
	"time"
)

// An existing account is linked to identities at providers and unlinked
// again (ADR-0229).
func TestLinkingIdentitiesToAnExistingAccount(t *testing.T) {
	a, j, _ := newAccounts(t)
	anna, err := a.CreateUser("anna", pw)
	noErr(t, err)
	other, err := a.CreateExternalUser("other", "https://idp", "taken")
	noErr(t, err)

	noErr(t, a.LinkIdentity(anna.ID, "https://idp", "anna-at-idp"))
	if got, ok := a.UserByIdentity("https://idp", "anna-at-idp"); !ok || got.ID != anna.ID {
		t.Fatal("a linked identity does not find its account")
	}
	if ids := a.Identities(anna.ID); len(ids) != 1 || ids[0].Issuer != "https://idp" || ids[0].Subject != "anna-at-idp" {
		t.Fatalf("identities %+v", ids)
	}
	if err := a.LinkIdentity(anna.ID, "https://idp", "taken"); !errors.Is(err, ErrIdentityTaken) {
		t.Fatalf("an identity of another account: %v", err)
	}
	if err := a.LinkIdentity(anna.ID, "https://idp", "second"); !errors.Is(err, ErrProviderLinked) {
		t.Fatalf("a second identity at the same provider: %v", err)
	}
	noErr(t, a.LinkIdentity(anna.ID, "https://other-idp", "anna-elsewhere"))
	if err := a.LinkIdentity("ghost", "https://third", "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an unknown account: %v", err)
	}
	if err := a.LinkIdentity(anna.ID, "", "x"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("no issuer: %v", err)
	}

	// An account with a password may give up every identity.
	noErr(t, a.UnlinkIdentity(anna.ID, "https://other-idp", false))
	if _, ok := a.UserByIdentity("https://other-idp", "anna-elsewhere"); ok {
		t.Fatal("an unlinked identity still signs in")
	}
	if err := a.UnlinkIdentity(anna.ID, "https://other-idp", false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unlinking twice: %v", err)
	}

	// An account without a password keeps its last way in, unless an
	// administrator frees the identity on purpose.
	if a.HasPassword(other.ID) || !a.HasPassword(anna.ID) {
		t.Fatal("HasPassword")
	}
	if err := a.UnlinkIdentity(other.ID, "https://idp", false); !errors.Is(err, ErrLastSignIn) {
		t.Fatalf("the last way in: %v", err)
	}
	noErr(t, a.UnlinkIdentity(other.ID, "https://idp", true))
	noErr(t, a.LinkIdentity(anna.ID, "https://third", "taken-before"))

	b, err := New(j.records, j.append, WithIterations(1000))
	noErr(t, err)
	if got, ok := b.UserByIdentity("https://idp", "anna-at-idp"); !ok || got.ID != anna.ID {
		t.Fatal("a link is lost in a restart")
	}
	if _, ok := b.UserByIdentity("https://other-idp", "anna-elsewhere"); ok {
		t.Fatal("an unlinking is lost in a restart")
	}
	if _, ok := b.UserByIdentity("https://idp", "taken"); ok {
		t.Fatal("an identity freed by an administrator is back after a restart")
	}
	if _, err := New([][]byte{[]byte(`{"v":1,"kind":"IdentityUnlinked","user":"ghost","issuer":"i","subject":"s"}`)}, j.append); err == nil {
		t.Fatal("an unlinking of an unknown identity loaded")
	}
}

// Giving the password again before a link is throttled as a login is.
func TestConfirmingThePassword(t *testing.T) {
	a, _, _ := newAccounts(t)
	anna, err := a.CreateUser("anna", pw)
	noErr(t, err)
	ext, err := a.CreateExternalUser("ext", "https://idp", "s")
	noErr(t, err)
	noErr(t, a.ConfirmPassword(anna.ID, pw, "10.0.0.1"))
	if err := a.ConfirmPassword(anna.ID, "wrong", "10.0.0.1"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("wrong password: %v", err)
	}
	if err := a.ConfirmPassword(ext.ID, "", "10.0.0.1"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("an account without a password: %v", err)
	}
	for i := 0; i < 20; i++ {
		_ = a.ConfirmPassword(anna.ID, "wrong", "10.0.0.2")
	}
	if err := a.ConfirmPassword(anna.ID, pw, "10.0.0.2"); !errors.Is(err, ErrThrottled) {
		t.Fatalf("not throttled after many failures: %v", err)
	}
}

// A session knows when it began, so a fresh sign-in can stand in for the
// password of an account that has none.
func TestASessionKnowsWhenItBegan(t *testing.T) {
	a, _, c := newAccounts(t)
	u, err := a.CreateExternalUser("ext", "https://idp", "s")
	noErr(t, err)
	_, p, err := a.StartSession(u.ID)
	noErr(t, err)
	began := c.t
	c.t = c.t.Add(7 * time.Minute)
	if got, ok := a.SessionStarted(p.Session); !ok || !got.Equal(began) {
		t.Fatalf("started %v %v, want %v", got, ok, began)
	}
	if _, ok := a.SessionStarted("nope"); ok {
		t.Fatal("an unknown session has a start")
	}
}
