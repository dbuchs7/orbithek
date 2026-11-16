package server

import (
	"net/http"
	"slices"
	"testing"
)

func treeTitles(t *testing.T, c *client, path string) []string {
	t.Helper()
	var out []string
	for _, n := range c.expect("GET", path, "", http.StatusOK)["items"].([]any) {
		out = append(out, n.(map[string]any)["title"].(string))
	}
	return out
}

func TestPageTreeThroughTheAPI(t *testing.T) {
	w := newWorld(t)
	w.admin.expect("POST", "/api/types", `{"name":"page","body":true,"fields":[]}`, http.StatusCreated)
	create := func(title, parent string) string {
		it := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"page","title":"`+title+`","parent":"`+parent+`"}`, http.StatusCreated)
		return field(it, "item", "key").(string)
	}
	home := create("Home", "")
	homeID := field(w.bert.expect("GET", "/api/items/"+home, "", http.StatusOK), "item", "id").(string)
	a, b := create("A", homeID), create("B", homeID)

	top := w.vera.expect("GET", "/api/spaces/PLAT/tree", "", http.StatusOK)["items"].([]any)
	if len(top) != 1 || top[0].(map[string]any)["key"] != home || top[0].(map[string]any)["hasChildren"] != true {
		t.Fatalf("top %v", top)
	}
	if got := treeTitles(t, w.vera, "/api/spaces/PLAT/tree?parent="+home); !slices.Equal(got, []string{"A", "B"}) {
		t.Fatalf("children %v", got)
	}
	// Place B before A, by key; a viewer may not.
	w.vera.expect("POST", "/api/items/"+b+"/move", `{"space":"PLAT","parent":"`+homeID+`","before":"`+a+`"}`, http.StatusForbidden)
	w.bert.expect("POST", "/api/items/"+b+"/move", `{"space":"PLAT","parent":"`+homeID+`","before":"`+a+`"}`, http.StatusOK)
	if got := treeTitles(t, w.vera, "/api/spaces/PLAT/tree?parent="+home); !slices.Equal(got, []string{"B", "A"}) {
		t.Fatalf("after placing %v", got)
	}
	// "before": "" places last; a move without it keeps the order.
	w.bert.expect("POST", "/api/items/"+b+"/move", `{"space":"PLAT","parent":"`+homeID+`","before":""}`, http.StatusOK)
	w.bert.expect("POST", "/api/items/"+a+"/move", `{"space":"PLAT","parent":"`+homeID+`"}`, http.StatusOK)
	if got := treeTitles(t, w.vera, "/api/spaces/PLAT/tree?parent="+home); !slices.Equal(got, []string{"A", "B"}) {
		t.Fatalf("after placing last %v", got)
	}
	w.bert.expect("POST", "/api/items/"+b+"/move", `{"space":"PLAT","parent":"`+homeID+`","before":"`+home+`"}`, http.StatusBadRequest)

	// Paged, and nothing for someone without a role.
	page := w.vera.expect("GET", "/api/spaces/PLAT/tree?parent="+home+"&limit=1", "", http.StatusOK)
	if len(page["items"].([]any)) != 1 || page["next"] == nil {
		t.Fatalf("page %v", page)
	}
	if got := w.carla.expect("GET", "/api/spaces/PLAT/tree", "", http.StatusOK)["items"].([]any); len(got) != 0 {
		t.Fatalf("outsider sees %v", got)
	}
	w.carla.expect("GET", "/api/spaces/PLAT/tree?parent="+home, "", http.StatusNotFound)
	w.vera.expect("GET", "/api/spaces/NOPE/tree", "", http.StatusNotFound)
}

// A page moves into another room, to its top or below a page there; it
// takes its child pages with it and gets a key of the new room, while the
// old key keeps resolving (ADR-0067, D2, D5).
func TestMovingAPageIntoAnotherRoom(t *testing.T) {
	w := newWorld(t)
	w.admin.expect("POST", "/api/types", `{"name":"page","body":true,"fields":[]}`, http.StatusCreated)
	w.admin.expect("POST", "/api/spaces", `{"key":"OPS","name":"Operations"}`, http.StatusCreated)
	w.admin.expect("PUT", "/api/spaces/OPS/roles/"+w.ids["bert"], `{"role":"editor"}`, http.StatusNoContent)
	create := func(space, title, parent string) (key, id string) {
		it := w.bert.expect("POST", "/api/items", `{"space":"`+space+`","type":"page","title":"`+title+`","parent":"`+parent+`"}`, http.StatusCreated)
		return field(it, "item", "key").(string), field(it, "item", "id").(string)
	}
	guide, guideID := create("PLAT", "Guide", "")
	_, _ = create("PLAT", "Install", guideID)
	target, targetID := create("OPS", "Runbooks", "")

	// Below a page of the other room.
	w.bert.expect("POST", "/api/items/"+guide+"/move", `{"space":"OPS","parent":"`+targetID+`"}`, http.StatusOK)
	if got := treeTitles(t, w.bert, "/api/spaces/OPS/tree?parent="+target); !slices.Equal(got, []string{"Guide"}) {
		t.Fatalf("below the target %v", got)
	}
	moved := w.bert.expect("GET", "/api/items/"+guide, "", http.StatusOK)
	if field(moved, "item", "space") != "OPS" || field(moved, "item", "id") != guideID {
		t.Fatalf("the old key resolves to %v", moved)
	}
	newKey := field(moved, "item", "key").(string)
	if got := treeTitles(t, w.bert, "/api/spaces/OPS/tree?parent="+newKey); !slices.Equal(got, []string{"Install"}) {
		t.Fatalf("its child page came along: %v", got)
	}

	// To the top of a room.
	w.bert.expect("POST", "/api/items/"+newKey+"/move", `{"space":"PLAT","parent":""}`, http.StatusOK)
	if got := treeTitles(t, w.bert, "/api/spaces/PLAT/tree"); !slices.Equal(got, []string{"Guide"}) {
		t.Fatalf("top of PLAT %v", got)
	}

	// A parent that does not exist is not found; one in another room is refused.
	w.bert.expect("POST", "/api/items/"+guideID+"/move", `{"space":"OPS","parent":"nope"}`, http.StatusNotFound)
	w.bert.expect("POST", "/api/items/"+guideID+"/move", `{"space":"PLAT","parent":"`+targetID+`"}`, http.StatusBadRequest)
	// A viewer of the room the page would go to may not create there.
	w.admin.expect("PUT", "/api/spaces/OPS/roles/"+w.ids["vera"], `{"role":"viewer"}`, http.StatusNoContent)
	w.admin.expect("PUT", "/api/spaces/PLAT/roles/"+w.ids["vera"], `{"role":"editor"}`, http.StatusNoContent)
	w.vera.expect("POST", "/api/items/"+guideID+"/move", `{"space":"OPS","parent":"`+targetID+`"}`, http.StatusForbidden)
}
