package server

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/dbuchs7/orbithek/internal/engineconn"
	"github.com/dbuchs7/orbithek/internal/extcal"
)

// TestAnAdministratorSuspendsAndReactivatesAnAccount: a suspended account's
// session and token stop at once, its password is answered with the
// reason, and reactivating it lets it in again; only administrators
// decide, never about their own account
// (ADR-draft-suspending-and-deleting-accounts).
func TestAnAdministratorSuspendsAndReactivatesAnAccount(t *testing.T) {
	w := newWorld(t)
	bert := w.ids["bert"]
	secret := w.bert.expect("POST", "/api/tokens", `{"name":"script","scope":"full","days":1}`, http.StatusCreated)["secret"].(string)
	script := &client{t: t, h: w.admin.h, bearer: secret}
	script.expect("GET", "/api/me", "", http.StatusOK)

	w.vera.expect("PUT", "/api/users/"+bert+"/suspended", `{"suspended":true}`, http.StatusForbidden)
	w.admin.expect("PUT", "/api/users/"+bert+"/suspended", `{}`, http.StatusBadRequest)
	w.admin.expect("PUT", "/api/users/nobody/suspended", `{"suspended":true}`, http.StatusNotFound)
	w.admin.expect("PUT", "/api/users/"+bert+"/suspended", `{"suspended":true}`, http.StatusNoContent)

	w.bert.expect("GET", "/api/me", "", http.StatusUnauthorized)
	script.expect("GET", "/api/me", "", http.StatusUnauthorized)
	again := &client{t: t, h: w.admin.h}
	if got := again.expect("POST", "/api/login", `{"username":"bert","password":"bert's password"}`, http.StatusForbidden); !strings.Contains(got["error"].(string), "suspended") {
		t.Fatalf("login answer %v", got)
	}
	again.expect("POST", "/api/login", `{"username":"bert","password":"a wrong password"}`, http.StatusUnauthorized)
	if u := userOf(t, w.admin, bert); u["suspended"] != true {
		t.Fatalf("listed as %v", u)
	}

	w.admin.expect("PUT", "/api/users/"+bert+"/suspended", `{"suspended":false}`, http.StatusNoContent)
	script.expect("GET", "/api/me", "", http.StatusOK)
	again.login("bert", "bert's password")
	if u := userOf(t, w.admin, bert); u["suspended"] != nil {
		t.Fatalf("listed as %v", u)
	}

	trail := strings.Join(eventsOf(w.admin.expect("GET", "/api/audit", "", http.StatusOK)), ",")
	for _, e := range []string{"user.suspend", "user.reactivate"} {
		if !strings.Contains(trail, e) {
			t.Errorf("the audit trail has no %s", e)
		}
	}
}

// TestTheLastAdministratorWhoCanSignInStays: an administrator suspends
// or deletes neither their own account nor the last other administrator
// who can sign in, and does not give up the role while the only other
// holder is suspended (ADR-0257, ADR-draft-suspending-and-deleting-accounts).
func TestTheLastAdministratorWhoCanSignInStays(t *testing.T) {
	w := newWorld(t)
	me := w.admin.expect("GET", "/api/me", "", http.StatusOK)
	admin := field(me, "user", "id").(string)
	w.admin.expect("PUT", "/api/users/"+admin+"/suspended", `{"suspended":true}`, http.StatusBadRequest)
	w.admin.expect("DELETE", "/api/users/"+admin, "", http.StatusBadRequest)

	// Vera becomes an administrator, and is suspended.
	vera := w.ids["vera"]
	w.admin.expect("PUT", "/api/system-roles/admin/"+vera, "", http.StatusNoContent)
	w.admin.expect("PUT", "/api/users/"+vera+"/suspended", `{"suspended":true}`, http.StatusNoContent)
	w.admin.expect("DELETE", "/api/system-roles/admin/"+admin, "", http.StatusBadRequest)
	w.admin.expect("PUT", "/api/users/"+vera+"/suspended", `{"suspended":false}`, http.StatusNoContent)

	// Vera, an administrator who can sign in, may now end the first one.
	veraC := &client{t: t, h: w.admin.h}
	veraC.login("vera", "vera's password")
	veraC.expect("PUT", "/api/users/"+admin+"/suspended", `{"suspended":true}`, http.StatusNoContent)
	veraC.expect("PUT", "/api/users/"+admin+"/suspended", `{"suspended":false}`, http.StatusNoContent)
	veraC.expect("DELETE", "/api/users/"+admin, "", http.StatusNoContent)
	veraC.expect("DELETE", "/api/users/"+vera, "", http.StatusBadRequest)
}

// TestADeletedAccountIsGoneButItsIDStays: deleting an account takes back
// its roles and memberships, ends its ways in, frees its username and
// leaves its ID known as deleted, so its work still says who did it
// (ADR-draft-suspending-and-deleting-accounts).
func TestADeletedAccountIsGoneButItsIDStays(t *testing.T) {
	w := newWorld(t)
	bert := w.ids["bert"]
	created := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"Patch","properties":{"priority":"low"}}`, http.StatusCreated)
	key := field(created, "item", "key").(string)
	w.admin.expect("PUT", "/api/groups/ops/members/"+bert, "", http.StatusNoContent)
	w.admin.expect("PUT", "/api/system-roles/erasure/"+bert, "", http.StatusNoContent)

	// What the account keeps outside the accounts ends with it.
	if _, err := w.db.Connections.Connect(bert, "atlas", engineconn.Profile{Subject: "s-bert", Username: "bert"}, "a refresh token"); err != nil {
		t.Fatal(err)
	}
	acc, err := w.db.Calendars.Connect(bert, extcal.Account{Kind: extcal.KindCalDAV, Name: "bert", Server: "https://dav.example.org/"}, "a password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.db.Calendars.AddCalendar(bert, acc.ID, "https://dav.example.org/bert/work/", "Work", "#1a2b3c"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.db.Calendars.AddICS(bert, "Holidays", "#1a2b3c", "https://example.org/holidays.ics"); err != nil {
		t.Fatal(err)
	}

	w.vera.expect("DELETE", "/api/users/"+bert, "", http.StatusForbidden)
	w.admin.expect("DELETE", "/api/users/"+bert, "", http.StatusNoContent)
	if len(w.db.Connections.Of(bert, "")) != 0 || len(w.db.Calendars.Accounts(bert)) != 0 || len(w.db.Calendars.Sources(bert)) != 0 {
		t.Fatal("a connection or calendar outlived the account")
	}
	w.admin.expect("DELETE", "/api/users/"+bert, "", http.StatusNotFound)

	w.bert.expect("GET", "/api/me", "", http.StatusUnauthorized)
	(&client{t: t, h: w.admin.h}).expect("POST", "/api/login", `{"username":"bert","password":"bert's password"}`, http.StatusUnauthorized)
	for _, x := range w.admin.expect("GET", "/api/users", "", http.StatusOK)["users"].([]any) {
		if x.(map[string]any)["id"] == bert {
			t.Fatal("a deleted account is listed")
		}
	}
	roles := w.admin.expect("GET", "/api/spaces/PLAT/roles", "", http.StatusOK)
	if strings.Contains(strings.Join(principalsOf(roles), ","), bert) {
		t.Fatalf("a role is left: %v", roles)
	}
	for _, g := range w.admin.expect("GET", "/api/groups", "", http.StatusOK)["groups"].([]any) {
		if slices.Contains(g.(map[string]any)["members"].([]any), any(bert)) {
			t.Fatalf("a membership is left: %v", g)
		}
	}
	for _, x := range w.admin.expect("GET", "/api/system-roles", "", http.StatusOK)["holders"].([]any) {
		if x.(map[string]any)["principal"] == bert {
			t.Fatalf("a system role is left: %v", x)
		}
	}

	// The task still names its author, who is now a deleted account.
	item := w.vera.expect("GET", "/api/items/"+key, "", http.StatusOK)
	names := w.vera.expect("GET", "/api/users/names?id="+bert, "", http.StatusOK)
	if len(names["names"].(map[string]any)) != 0 || len(names["deleted"].([]any)) != 1 || names["deleted"].([]any)[0] != bert {
		t.Fatalf("names %v (item %v)", names, item)
	}
	// No role goes to the deleted account again.
	w.admin.expect("PUT", "/api/spaces/PLAT/roles/"+bert, `{"role":"viewer"}`, http.StatusBadRequest)

	// The username is free: a new account gets a new ID.
	u := w.admin.expect("POST", "/api/users", `{"username":"bert","password":"bert's new password"}`, http.StatusCreated)
	if field(u, "user", "id") == bert {
		t.Fatal("a deleted account's ID was reused")
	}
	if !strings.Contains(strings.Join(eventsOf(w.admin.expect("GET", "/api/audit", "", http.StatusOK)), ","), "user.delete") {
		t.Error("the audit trail has no user.delete")
	}
}

// userOf finds an account in the administrators' list.
func userOf(t *testing.T, c *client, id string) map[string]any {
	t.Helper()
	for _, x := range c.expect("GET", "/api/users", "", http.StatusOK)["users"].([]any) {
		if m := x.(map[string]any); m["id"] == id {
			return m
		}
	}
	t.Fatalf("no account %s", id)
	return nil
}

// principalsOf lists the principals of a space's roles.
func principalsOf(roles map[string]any) []string {
	var out []string
	for _, x := range roles["roles"].([]any) {
		out = append(out, x.(map[string]any)["principal"].(string))
	}
	return out
}
