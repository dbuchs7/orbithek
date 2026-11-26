package server

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/dbuchs7/orbithek/internal/item"
)

func TestAccessIsListedAndTakenBackThroughTheAPI(t *testing.T) {
	w := newWorld(t)
	roles := fmt.Sprint(w.admin.expect("GET", "/api/spaces/PLAT/roles", "", http.StatusOK)["roles"])
	if !strings.Contains(roles, w.ids["vera"]) || !strings.Contains(roles, "viewer") {
		t.Fatalf("roles %s", roles)
	}
	w.bert.expect("GET", "/api/spaces/PLAT/roles", "", http.StatusForbidden)
	w.carla.expect("GET", "/api/spaces/PLAT/roles", "", http.StatusNotFound)

	w.bert.expect("DELETE", "/api/spaces/PLAT/roles/"+w.ids["vera"], "", http.StatusForbidden)
	w.admin.expect("DELETE", "/api/spaces/PLAT/roles/"+w.ids["vera"], "", http.StatusNoContent)
	w.vera.expect("GET", "/api/spaces/PLAT/roles", "", http.StatusNotFound)

	w.admin.expect("PUT", "/api/groups/ops/members/"+w.ids["carla"], "", http.StatusNoContent)
	if g := fmt.Sprint(w.admin.expect("GET", "/api/groups", "", http.StatusOK)["groups"]); !strings.Contains(g, "ops") || !strings.Contains(g, w.ids["carla"]) {
		t.Fatalf("groups %s", g)
	}
	w.bert.expect("GET", "/api/groups", "", http.StatusForbidden)
	w.admin.expect("DELETE", "/api/groups/ops/members/"+w.ids["carla"], "", http.StatusNoContent)

	w.admin.expect("PUT", "/api/system-roles/erasure/"+w.ids["bert"], "", http.StatusNoContent)
	if h := fmt.Sprint(w.admin.expect("GET", "/api/system-roles", "", http.StatusOK)["holders"]); !strings.Contains(h, w.ids["bert"]) || !strings.Contains(h, "erasure") {
		t.Fatalf("holders %s", h)
	}
	w.admin.expect("DELETE", "/api/system-roles/erasure/"+w.ids["bert"], "", http.StatusNoContent)
	adminID := w.admin.expect("GET", "/api/me", "", http.StatusOK)["user"].(map[string]any)["id"].(string)
	w.admin.expect("DELETE", "/api/system-roles/admin/"+adminID, "", http.StatusBadRequest) // the last administrator

	audit := fmt.Sprint(w.admin.expect("GET", "/api/audit", "", http.StatusOK))
	for _, e := range []string{"role.revoke", "group.remove", "system-role.revoke"} {
		if !strings.Contains(audit, e) {
			t.Errorf("the audit trail has no %s", e)
		}
	}
}

// A role goes only to a person with an account or to a group with members:
// a name that resolves to nobody would be a grant that opens the space to
// no one, shown in the list as if it worked. A grant made before the check
// is listed as unknown and can still be taken back (ADR-0255).
func TestARoleGoesOnlyToPeopleAndGroupsThatExist(t *testing.T) {
	w := newWorld(t)
	// A group nobody belongs to, a name its "@" was encoded twice on the
	// way (a proxy's doing), an ID no account has.
	w.admin.expect("PUT", "/api/spaces/PLAT/roles/%40ops", `{"role":"viewer"}`, http.StatusBadRequest)
	w.admin.expect("PUT", "/api/spaces/PLAT/roles/%2540ops", `{"role":"viewer"}`, http.StatusBadRequest)
	w.admin.expect("PUT", "/api/spaces/PLAT/roles/0123456789abcdef", `{"role":"viewer"}`, http.StatusBadRequest)
	// Whoever may not grant learns nothing about who exists.
	w.bert.expect("PUT", "/api/spaces/PLAT/roles/0123456789abcdef", `{"role":"viewer"}`, http.StatusForbidden)
	w.carla.expect("PUT", "/api/spaces/PLAT/roles/0123456789abcdef", `{"role":"viewer"}`, http.StatusForbidden)
	w.admin.expect("PUT", "/api/spaces/NOPE/roles/@ops", `{"role":"viewer"}`, http.StatusNotFound)
	if len(w.carla.expect("GET", "/api/spaces", "", http.StatusOK)["spaces"].([]any)) != 0 {
		t.Fatal("a refused grant opened the space")
	}

	// Once the group has a member it takes the role, encoded or not.
	w.admin.expect("PUT", "/api/groups/ops/members/"+w.ids["carla"], "", http.StatusNoContent)
	w.admin.expect("PUT", "/api/spaces/PLAT/roles/%40ops", `{"role":"viewer"}`, http.StatusNoContent)
	if len(w.carla.expect("GET", "/api/spaces", "", http.StatusOK)["spaces"].([]any)) != 1 {
		t.Fatal("the group's role did not reach its member")
	}

	// A grant from before the check: listed as unknown, not changed,
	// taken back.
	adminID := w.admin.expect("GET", "/api/me", "", http.StatusOK)["user"].(map[string]any)["id"].(string)
	if err := w.db.Write(func(s *item.Store) error { return s.GrantRole(adminID, "PLAT", "%40ops", item.Viewer) }); err != nil {
		t.Fatal(err)
	}
	unknown := func() map[string]bool {
		out := map[string]bool{}
		for _, g := range w.admin.expect("GET", "/api/spaces/PLAT/roles", "", http.StatusOK)["roles"].([]any) {
			m := g.(map[string]any)
			out[m["principal"].(string)] = m["unknown"] == true
		}
		return out
	}
	if got := unknown(); !got["%40ops"] || got["@ops"] || got[w.ids["vera"]] || got[w.ids["bert"]] || len(got) != 4 {
		t.Fatalf("unknown principals %v", got)
	}
	w.admin.expect("PUT", "/api/spaces/PLAT/roles/%2540ops", `{"role":"editor"}`, http.StatusBadRequest)
	w.admin.expect("DELETE", "/api/spaces/PLAT/roles/%2540ops", "", http.StatusNoContent)

	// A group whose last member left gives nobody access either.
	w.admin.expect("DELETE", "/api/groups/ops/members/"+w.ids["carla"], "", http.StatusNoContent)
	if got := unknown(); !got["@ops"] || len(got) != 3 {
		t.Fatalf("unknown principals after the group emptied %v", got)
	}
}

// An editor protects an item for a group and for people by name; only
// groups and people that exist are taken, and the editor finds groups by
// name without seeing their members (ADR-0078).
func TestProtectingAnItemForAGroup(t *testing.T) {
	w := newWorld(t)
	w.admin.expect("PUT", "/api/groups/redaktion/members/"+w.ids["vera"], "", http.StatusNoContent)
	w.admin.expect("PUT", "/api/groups/reviewers/members/"+w.ids["bert"], "", http.StatusNoContent)
	got := fmt.Sprint(w.bert.expect("GET", "/api/groups/search?q=RE", "", http.StatusOK)["groups"])
	if got != "[redaktion reviewers]" {
		t.Fatalf("groups found %s", got)
	}
	if got := fmt.Sprint(w.bert.expect("GET", "/api/groups/search?q=view", "", http.StatusOK)["groups"]); got != "[reviewers]" {
		t.Fatalf("inner match %s", got)
	}
	if got := fmt.Sprint(w.bert.expect("GET", "/api/groups/search?q=zzz", "", http.StatusOK)["groups"]); got != "[]" {
		t.Fatalf("nothing found %s", got)
	}
	w.vera.expect("GET", "/api/groups/search?q=r", "", http.StatusForbidden)
	w.carla.expect("GET", "/api/groups/search?q=r", "", http.StatusForbidden)

	created := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"Protected","properties":{"priority":"low"}}`, http.StatusCreated)
	id := created["item"].(map[string]any)["id"].(string)
	path := "/api/items/" + id + "/restriction"
	w.bert.expect("PUT", path, `{"read":["@nobody-here"],"edit":[]}`, http.StatusBadRequest)
	w.bert.expect("PUT", path, `{"read":["`+w.ids["bert"]+`","no-such-user"],"edit":[]}`, http.StatusBadRequest)
	// Bert would lock himself out with the group alone.
	w.bert.expect("PUT", path, `{"read":["@redaktion"],"edit":[]}`, http.StatusBadRequest)
	w.bert.expect("PUT", path, `{"read":["@redaktion","`+w.ids["bert"]+`"],"edit":["`+w.ids["bert"]+`"]}`, http.StatusOK)
	w.vera.expect("GET", "/api/items/"+id, "", http.StatusOK)
	w.admin.expect("DELETE", "/api/groups/redaktion/members/"+w.ids["vera"], "", http.StatusNoContent)
	w.vera.expect("GET", "/api/items/"+id, "", http.StatusNotFound)
}
