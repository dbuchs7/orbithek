package extcal

import (
	"net/url"
	"testing"
)

// Connecting the calendars of a Google account one's sign-in is linked to
// proposes that account to Google (ADR-0229); without a hint Google lets
// one choose as before.
func TestConnectingProposesTheLinkedGoogleAccount(t *testing.T) {
	g := NewGoogle("client", "secret", "https://wiki.example.org/api/calendar-accounts/google/callback")
	hinted, _, err := g.BeginFor("u1", "google-subject-1")
	if err != nil {
		t.Fatal(err)
	}
	q := mustQuery(t, hinted)
	if q.Get("login_hint") != "google-subject-1" || q.Get("scope") != googleScope || q.Get("access_type") != "offline" {
		t.Fatalf("hinted consent %s", hinted)
	}
	plain, _, err := g.Begin("u1")
	if err != nil {
		t.Fatal(err)
	}
	if mustQuery(t, plain).Has("login_hint") {
		t.Fatalf("a hint without a linked account: %s", plain)
	}
}

func mustQuery(t *testing.T, raw string) url.Values {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u.Query()
}
