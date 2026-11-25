package server

import (
	"net/http"
	"testing"
)

func TestTheTrashOfASpaceIsListed(t *testing.T) {
	w := newWorld(t)
	guide := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"Guide","properties":{"priority":"low"}}`, http.StatusCreated)
	id := field(guide, "item", "id").(string)
	w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"Chapter","parent":"`+id+`","properties":{"priority":"low"}}`, http.StatusCreated)
	w.bert.expect("POST", "/api/items/"+id+"/trash", "", http.StatusOK)

	list := w.bert.expect("GET", "/api/spaces/PLAT/trash", "", http.StatusOK)
	items := list["items"].([]any)
	if len(items) != 1 || field(list, "total", "value") != float64(1) {
		t.Fatalf("trash: %v, want the guide alone", list)
	}
	e := items[0].(map[string]any)
	if e["id"] != id || e["title"] != "Guide" || e["type"] != "task" || e["items"] != float64(2) || e["by"] != w.ids["bert"] || e["at"] == "" {
		t.Fatalf("entry: %v", e)
	}
	if e["mayRestore"] != true || e["mayPurge"] != false {
		t.Fatalf("editor may: restore %v, purge %v", e["mayRestore"], e["mayPurge"])
	}
	if e := w.admin.expect("GET", "/api/spaces/PLAT/trash", "", http.StatusOK)["items"].([]any)[0].(map[string]any); e["mayPurge"] != true {
		t.Fatalf("administrator may not purge: %v", e)
	}
	if e := w.vera.expect("GET", "/api/spaces/PLAT/trash", "", http.StatusOK)["items"].([]any)[0].(map[string]any); e["mayRestore"] != false {
		t.Fatalf("viewer may restore: %v", e)
	}
	// One without a role in the space sees an empty trash (D10).
	if got := w.carla.expect("GET", "/api/spaces/PLAT/trash", "", http.StatusOK)["items"].([]any); len(got) != 0 {
		t.Fatalf("outsider's trash: %v", got)
	}
	w.bert.expect("GET", "/api/spaces/NOPE/trash", "", http.StatusNotFound)
	w.bert.expect("GET", "/api/spaces/PLAT/trash?limit=0", "", http.StatusBadRequest)

	// Restored, it leaves the trash.
	w.bert.expect("POST", "/api/items/"+id+"/restore", "", http.StatusOK)
	if got := w.bert.expect("GET", "/api/spaces/PLAT/trash", "", http.StatusOK)["items"].([]any); len(got) != 0 {
		t.Fatalf("trash after restore: %v", got)
	}
}
