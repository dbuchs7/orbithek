package server

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/dbuchs7/orbithek/internal/auth"
	"github.com/dbuchs7/orbithek/internal/extcal"
	"github.com/dbuchs7/orbithek/internal/sso"
	"github.com/dbuchs7/orbithek/internal/sso/ssotest"
)

// Once one's account is linked to a Google identity, the account page
// offers the calendars of that Google account, and connecting them
// proposes the same account to Google (ADR-0229).
func TestALinkedGoogleAccountLeadsToItsCalendars(t *testing.T) {
	idp := ssotest.New(t, "wiki")
	google, err := sso.New(sso.Config{ID: "google", Kind: "google", Name: "Google", Issuer: idp.URL, ClientID: "wiki", ClientSecret: "s",
		RedirectURL: "http://localhost/api/oidc/callback"}, idp.Client())
	if err != nil {
		t.Fatal(err)
	}
	w := newWorld(t, func(d *Deps) {
		d.SSO = []*sso.Provider{google}
		d.Calendars = extcal.NewReader(d.DB.Calendars, nil, extcal.NewGoogle("client", "secret", "https://wiki.example.org"+GoogleCalendarCallback))
	})
	bert := w.bert
	bertID := bert.expect("GET", "/api/me", "", http.StatusOK)["user"].(map[string]any)["id"].(string)
	if err := w.db.Accounts.LinkIdentity(auth.UserID(bertID), idp.URL, "bert-at-google"); err != nil {
		t.Fatal(err)
	}
	offered := func(c *client) any {
		t.Helper()
		return c.expect("GET", "/api/me/identities", "", http.StatusOK)["providers"].([]any)[0].(map[string]any)["calendar"]
	}
	// Calendars of other services are off until an administrator turns
	// them on, and so is the offer.
	if offered(bert) != false {
		t.Fatal("the calendars are offered while calendars of other services are off")
	}
	w.admin.expect("PUT", "/api/settings/calendars", `{"external":true}`, http.StatusOK)
	if offered(bert) != true {
		t.Fatal("a linked Google account's calendars are not offered")
	}
	if offered(w.vera) != false {
		t.Fatal("the calendars are offered without a linked account")
	}

	hint := func(c *client, target string) string {
		t.Helper()
		rec := c.do("GET", target, "")
		if rec.Code != http.StatusFound {
			t.Fatalf("%s: %d %s", target, rec.Code, rec.Body)
		}
		u, err := url.Parse(rec.Header().Get("Location"))
		if err != nil {
			t.Fatal(err)
		}
		return u.Query().Get("login_hint")
	}
	if got := hint(bert, "/api/me/calendar-accounts/google/connect?linked=1"); got != "bert-at-google" {
		t.Fatalf("login hint %q, want the linked subject", got)
	}
	if got := hint(bert, "/api/me/calendar-accounts/google/connect"); got != "" {
		t.Fatalf("a hint without asking for the linked account: %q", got)
	}
	if got := hint(w.vera, "/api/me/calendar-accounts/google/connect?linked=1"); got != "" {
		t.Fatalf("a hint for an account without a Google identity: %q", got)
	}
}
