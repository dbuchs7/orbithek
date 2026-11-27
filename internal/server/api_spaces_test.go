package server

import (
	"net/http"
	"strings"
	"testing"
)

func TestDeletingASpaceTakesTwoSteps(t *testing.T) {
	w := newWorld(t)
	created := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"Old work","properties":{"priority":"low"}}`, http.StatusCreated)
	key := field(created, "item", "key").(string)

	// Only administrators delete spaces; bert is an editor.
	w.bert.expect("DELETE", "/api/spaces/PLAT", "", http.StatusForbidden)
	w.bert.expect("GET", "/api/spaces/trash", "", http.StatusForbidden)

	w.admin.expect("DELETE", "/api/spaces/PLAT", "", http.StatusNoContent)
	if listed := mustJSON(t, w.bert.expect("GET", "/api/spaces", "", http.StatusOK)); strings.Contains(listed, "PLAT") {
		t.Fatalf("a space in the trash is still listed: %s", listed)
	}
	w.bert.expect("GET", "/api/items/"+key, "", http.StatusNotFound)
	w.admin.expect("GET", "/api/items/"+key, "", http.StatusNotFound)
	trash := w.admin.expect("GET", "/api/spaces/trash", "", http.StatusOK)["spaces"].([]any)
	if len(trash) != 1 || trash[0].(map[string]any)["key"] != "PLAT" || trash[0].(map[string]any)["items"] != float64(1) {
		t.Fatalf("trash: %v", trash)
	}

	// Restoring brings it back as it was.
	w.admin.expect("POST", "/api/spaces/PLAT/restore", "", http.StatusNoContent)
	w.bert.expect("GET", "/api/items/"+key, "", http.StatusOK)

	// Deleting for good needs the trash first and the key repeated.
	w.admin.expect("POST", "/api/spaces/PLAT/purge", `{"confirm":"PLAT"}`, http.StatusNotFound)
	w.admin.expect("DELETE", "/api/spaces/PLAT", "", http.StatusNoContent)
	w.admin.expect("POST", "/api/spaces/PLAT/purge", `{"confirm":"plat"}`, http.StatusBadRequest)
	w.bert.expect("POST", "/api/spaces/PLAT/purge", `{"confirm":"PLAT"}`, http.StatusForbidden)
	w.admin.expect("POST", "/api/spaces/PLAT/purge", `{"confirm":"PLAT"}`, http.StatusNoContent)
	if trash := w.admin.expect("GET", "/api/spaces/trash", "", http.StatusOK)["spaces"].([]any); len(trash) != 0 {
		t.Fatalf("trash after purge: %v", trash)
	}
	w.admin.expect("POST", "/api/spaces/PLAT/restore", "", http.StatusNotFound)
	// The key is not used again.
	w.admin.expect("POST", "/api/spaces", `{"key":"PLAT","name":"Again"}`, http.StatusBadRequest)

	audit := mustJSON(t, w.admin.expect("GET", "/api/audit?limit=20", "", http.StatusOK))
	for _, e := range []string{"space.trash", "space.restore", "space.purge"} {
		if !strings.Contains(audit, e) {
			t.Errorf("the audit trail lacks %s", e)
		}
	}
}

// TestTheTrashOfASpaceThroughTheAPI: an item put into the trash is found
// again in its space's trash and restored from there; a caller without a
// role in the space is answered as for a space that does not exist
// (ADR-0021).
func TestTheTrashOfASpaceThroughTheAPI(t *testing.T) {
	w := newWorld(t)
	mk := func(title, parent string) string {
		it := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"`+title+`","parent":"`+parent+`","properties":{"priority":"low"}}`, http.StatusCreated)
		return field(it, "item", "id").(string)
	}
	guide := mk("Guide", "")
	mk("Step", guide)
	alone := mk("Alone", "")
	w.bert.expect("POST", "/api/items/"+alone+"/trash", "", http.StatusOK)
	w.bert.expect("POST", "/api/items/"+guide+"/trash", "", http.StatusOK)

	got := w.bert.expect("GET", "/api/spaces/PLAT/trash", "", http.StatusOK)
	items := got["items"].([]any)
	if len(items) != 2 || field(got, "total", "value") != float64(2) {
		t.Fatalf("trash %v", got)
	}
	first := items[0].(map[string]any)
	if first["id"] != guide || first["title"] != "Guide" || first["below"] != float64(1) || first["by"] != w.ids["bert"] ||
		field(first, "may", "restore") != true || field(first, "may", "purge") != false {
		t.Errorf("the newest entry %v", first)
	}
	// Paged by the entry before.
	page := w.bert.expect("GET", "/api/spaces/PLAT/trash?limit=1", "", http.StatusOK)
	if len(page["items"].([]any)) != 1 || page["next"] != guide {
		t.Errorf("first page %v", page)
	}
	if rest := w.bert.expect("GET", "/api/spaces/PLAT/trash?limit=1&after="+guide, "", http.StatusOK); field(rest["items"].([]any)[0].(map[string]any), "id") != alone || rest["next"] != nil {
		t.Errorf("second page %v", rest)
	}
	// A viewer reads the trash and may change nothing in it.
	if v := w.vera.expect("GET", "/api/spaces/PLAT/trash", "", http.StatusOK)["items"].([]any)[0].(map[string]any); field(v, "may", "restore") != false {
		t.Errorf("vera may restore: %v", v)
	}
	// No role in the space reads as no such space.
	noRole := w.carla.do("GET", "/api/spaces/PLAT/trash", "")
	noSpace := w.carla.do("GET", "/api/spaces/NONE/trash", "")
	if noRole.Code != http.StatusNotFound || noSpace.Code != http.StatusNotFound {
		t.Errorf("no role %d, no space %d, want 404 for both", noRole.Code, noSpace.Code)
	}
	w.bert.expect("POST", "/api/items/"+guide+"/restore", "", http.StatusOK)
	if left := w.bert.expect("GET", "/api/spaces/PLAT/trash", "", http.StatusOK)["items"].([]any); len(left) != 1 {
		t.Errorf("after restoring: %v", left)
	}
}
