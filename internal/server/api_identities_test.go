package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dbuchs7/orbithek/internal/auth"
	"github.com/dbuchs7/orbithek/internal/sso/ssotest"
)

// beginIdentityLink asks to link the signed-in account to the provider "sso" and
// returns the answer, the provider's address and the link cookie.
func beginIdentityLink(t *testing.T, c *client, body string, status int) (map[string]any, string, *http.Cookie) {
	t.Helper()
	rec := c.do("POST", "/api/me/identities/sso", body)
	if rec.Code != status {
		t.Fatalf("start link: %d, want %d; %s", rec.Code, status, rec.Body)
	}
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	url, _ := out["url"].(string)
	return out, url, cookie(rec, linkCookie)
}

// completeIdentityLink plays the provider and the browser's way back.
func completeIdentityLink(t *testing.T, h http.Handler, idp *ssotest.IdP, url string, ck *http.Cookie, claims ssotest.Claims) *httptest.ResponseRecorder {
	t.Helper()
	cb := idp.Authorize(t, url, claims)
	if ck == nil {
		return getWith(h, "/api/oidc/callback?"+cb.Encode())
	}
	return getWith(h, "/api/oidc/callback?"+cb.Encode(), ck)
}

func TestLinkingAnExistingAccountToASignIn(t *testing.T) {
	h, d, idp := ssoSetup(t)
	admin := &client{t: t, h: h}
	admin.login("admin", adminPassword)
	me := admin.expect("GET", "/api/me", "", http.StatusOK)["user"].(map[string]any)["id"]

	list := admin.expect("GET", "/api/me/identities", "", http.StatusOK)
	providers := list["providers"].([]any)
	if len(providers) != 1 || providers[0].(map[string]any)["id"] != "sso" || providers[0].(map[string]any)["linked"] != false || list["password"] != true {
		t.Fatalf("identities %v", list)
	}

	// The password is given again first.
	if got, _, _ := beginIdentityLink(t, admin, `{"password":"wrong"}`, http.StatusForbidden); got["code"] != "password" {
		t.Fatalf("wrong password: %v", got)
	}
	beginIdentityLink(t, admin, `{}`, http.StatusForbidden)
	admin.expect("POST", "/api/me/identities/nope", `{"password":"`+adminPassword+`"}`, http.StatusNotFound)

	_, url, ck := beginIdentityLink(t, admin, `{"password":"`+adminPassword+`"}`, http.StatusOK)
	if ck == nil || !ck.HttpOnly || ck.SameSite != http.SameSiteLaxMode || ck.Path != "/api/oidc/" {
		t.Fatalf("link cookie %+v", ck)
	}
	rec := completeIdentityLink(t, h, idp, url, ck, ssotest.Claims{Subject: "admin-at-idp", PreferredUsername: "someone"})
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/#/account?linked=sso" || cookie(rec, SessionCookie) != nil {
		t.Fatalf("link callback: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if got := admin.expect("GET", "/api/me/identities", "", http.StatusOK)["providers"].([]any)[0].(map[string]any); got["linked"] != true {
		t.Fatalf("not listed as linked: %v", got)
	}

	// The identity now signs in to the existing account, also while new
	// accounts at sign-in are off.
	admin.expect("PUT", "/api/settings/sign-in", `{"newAccounts":false}`, http.StatusOK)
	in := signOn(t, h, idp, ssotest.Claims{Subject: "admin-at-idp"})
	again := (&client{t: t, h: h, cookie: cookie(in, SessionCookie)}).expect("GET", "/api/me", "", http.StatusOK)
	if again["user"].(map[string]any)["id"] != me || again["admin"] != true {
		t.Fatalf("the linked identity signs in elsewhere: %v", again)
	}

	// One identity per provider: a second link is refused before it starts.
	if got, _, _ := beginIdentityLink(t, admin, `{"password":"`+adminPassword+`"}`, http.StatusConflict); got["code"] != "linked" {
		t.Fatalf("second identity: %v", got)
	}

	// Unlinking; the account keeps its password.
	admin.expect("DELETE", "/api/me/identities/sso", "", http.StatusOK)
	admin.expect("DELETE", "/api/me/identities/sso", "", http.StatusNotFound)
	if rec := signOn(t, h, idp, ssotest.Claims{Subject: "admin-at-idp"}); rec.Header().Get("Location") != "/?login_error=unlinked" {
		t.Fatalf("an unlinked identity still signs in: %s", rec.Header().Get("Location"))
	}

	events := map[string]int{}
	for _, e := range d.DB.Audit.List(0, 200) {
		if e.Detail["provider"] == "sso" {
			events[e.Event]++
		}
	}
	if events["identity.link"] != 1 || events["identity.unlink"] != 1 {
		t.Fatalf("audit %v", events)
	}
}

func TestLinkRefusals(t *testing.T) {
	h, d, idp := ssoSetup(t)
	admin := &client{t: t, h: h}
	admin.login("admin", adminPassword)
	pass := `{"password":"` + adminPassword + `"}`
	refused := func(rec *httptest.ResponseRecorder, code, name string) {
		t.Helper()
		if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/#/account?link_error="+code || cookie(rec, SessionCookie) != nil {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Header().Get("Location"))
		}
	}

	// An identity that holds an account of its own already.
	if rec := signOn(t, h, idp, ssotest.Claims{Subject: "taken", PreferredUsername: "dup"}); cookie(rec, SessionCookie) == nil {
		t.Fatal("first sign-in")
	}
	_, url, ck := beginIdentityLink(t, admin, pass, http.StatusOK)
	refused(completeIdentityLink(t, h, idp, url, ck, ssotest.Claims{Subject: "taken"}), "taken", "identity of another account")

	// Without the link cookie the answer is no link, and not a sign-in of
	// someone else's account either.
	_, url, _ = beginIdentityLink(t, admin, pass, http.StatusOK)
	if rec := completeIdentityLink(t, h, idp, url, nil, ssotest.Claims{Subject: "fresh"}); rec.Header().Get("Location") != "/?login_error=sso" || cookie(rec, SessionCookie) != nil {
		t.Errorf("no link cookie: %s", rec.Header().Get("Location"))
	}

	// A link callback is not replayed.
	_, url, ck = beginIdentityLink(t, admin, pass, http.StatusOK)
	cb := idp.Authorize(t, url, ssotest.Claims{Subject: "once"})
	if rec := getWith(h, "/api/oidc/callback?"+cb.Encode(), ck); rec.Header().Get("Location") != "/#/account?linked=sso" {
		t.Fatalf("genuine link: %s", rec.Header().Get("Location"))
	}
	admin.expect("DELETE", "/api/me/identities/sso", "", http.StatusOK)
	refused(getWith(h, "/api/oidc/callback?"+cb.Encode(), ck), "expired", "replayed link")

	// A provider's refusal.
	_, _, ck = beginIdentityLink(t, admin, pass, http.StatusOK)
	state := ck.Value[len("sso."):]
	refused(getWith(h, "/api/oidc/callback?error=access_denied&state="+state, ck), "provider", "provider refusal")

	// An API token does not link: linking needs a signed-in browser.
	tok := admin.expect("POST", "/api/tokens", `{"name":"agent","scope":"full","days":1}`, http.StatusCreated)["secret"].(string)
	(&client{t: t, h: h, bearer: tok}).expect("POST", "/api/me/identities/sso", pass, http.StatusForbidden)

	n := 0
	for _, e := range d.DB.Audit.List(0, 200) {
		if e.Event == "identity.link-refused" {
			n++
		}
	}
	if n != 3 {
		t.Fatalf("%d refused links audited, want 3", n)
	}
}

func TestAnAccountWithoutPasswordLinksAfterAFreshSignIn(t *testing.T) {
	h, _, idp := ssoSetup(t)
	rec := signOn(t, h, idp, ssotest.Claims{Subject: "ext", PreferredUsername: "ext"})
	ext := &client{t: t, h: h, cookie: cookie(rec, SessionCookie)}
	if got := ext.expect("GET", "/api/me/identities", "", http.StatusOK); got["password"] != false {
		t.Fatalf("identities %v", got)
	}
	// Its only identity stays.
	if got := ext.expect("DELETE", "/api/me/identities/sso", "", http.StatusConflict); got["code"] != "last" {
		t.Fatalf("last way in: %v", got)
	}
	// One identity per provider, so a fresh sign-in may start a link only
	// to another provider; here the provider is taken.
	if got, _, _ := beginIdentityLink(t, ext, `{}`, http.StatusConflict); got["code"] != "linked" {
		t.Fatalf("second identity: %v", got)
	}
}

func TestASignInIsFreshForTenMinutes(t *testing.T) {
	began := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	if !signedInRecently(began, began.Add(9*time.Minute)) || signedInRecently(began, began.Add(11*time.Minute)) {
		t.Fatal("a sign-in is fresh for ten minutes")
	}
}

func TestAdministratorsSeeAndRemoveLinks(t *testing.T) {
	h, d, idp := ssoSetup(t)
	admin := &client{t: t, h: h}
	admin.login("admin", adminPassword)
	rec := signOn(t, h, idp, ssotest.Claims{Subject: "dup", PreferredUsername: "dup"})
	dup := (&client{t: t, h: h, cookie: cookie(rec, SessionCookie)}).expect("GET", "/api/me", "", http.StatusOK)["user"].(map[string]any)["id"].(string)

	var listed []any
	for _, u := range admin.expect("GET", "/api/users", "", http.StatusOK)["users"].([]any) {
		if u.(map[string]any)["id"] == dup {
			listed, _ = u.(map[string]any)["identities"].([]any)
		}
	}
	if len(listed) != 1 || listed[0] != "sso" {
		t.Fatalf("listed identities %v", listed)
	}

	bert, err := d.DB.Accounts.CreateUser("bert", "bert's password")
	if err != nil {
		t.Fatal(err)
	}
	b := &client{t: t, h: h}
	b.login("bert", "bert's password")
	b.expect("DELETE", "/api/users/"+dup+"/identities/sso", "", http.StatusForbidden)
	_ = bert

	// An administrator frees the identity, even the last way in of an
	// account made at a first sign-in.
	admin.expect("DELETE", "/api/users/"+dup+"/identities/sso", "", http.StatusOK)
	admin.expect("DELETE", "/api/users/"+dup+"/identities/sso", "", http.StatusNotFound)
	if _, ok := d.DB.Accounts.UserByIdentity(idp.URL, "dup"); ok {
		t.Fatal("the identity was not freed")
	}
	if ids := d.DB.Accounts.Identities(auth.UserID(dup)); len(ids) != 0 {
		t.Fatalf("identities left %v", ids)
	}
}
