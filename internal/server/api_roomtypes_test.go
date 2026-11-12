package server

import (
	"net/http"
	"slices"
	"strings"
	"testing"
)

func typeNamesOf(answer map[string]any) []string {
	var out []string
	for _, ty := range answer["types"].([]any) {
		out = append(out, ty.(map[string]any)["name"].(string))
	}
	return out
}

// TestRoomTypesThroughTheAPI: a space's administrators define, list and
// remove its own types through POST and DELETE /api/types (ADR-0199).
func TestRoomTypesThroughTheAPI(t *testing.T) {
	w := newWorld(t)
	w.admin.expect("POST", "/api/spaces", `{"key":"OPS","name":"Operations"}`, http.StatusCreated)
	w.admin.expect("PUT", "/api/spaces/PLAT/roles/"+w.ids["bert"], `{"role":"space-admin"}`, http.StatusNoContent)
	b := w.bert

	incident := `{"name":"incident","space":"PLAT","body":true,"fields":[{"name":"severity","kind":"enum","values":["low","high"]}]}`
	w.vera.expect("POST", "/api/types", incident, http.StatusForbidden)
	w.carla.expect("POST", "/api/types", incident, http.StatusNotFound)
	b.expect("POST", "/api/types", `{"name":"note","body":true,"fields":[]}`, http.StatusForbidden)
	b.expect("POST", "/api/types", strings.Replace(incident, "PLAT", "OPS", 1), http.StatusNotFound)
	got := b.expect("POST", "/api/types", incident, http.StatusCreated)
	if field(got, "type", "space") != "PLAT" || field(got, "type", "editable") != true {
		t.Fatalf("defined %v", got)
	}
	// The name is taken everywhere.
	w.admin.expect("POST", "/api/types", `{"name":"incident","fields":[]}`, http.StatusBadRequest)

	// Listing: Vera reads PLAT and sees its type, Carla holds no role and
	// does not; OPS's items may not have it.
	if names := typeNamesOf(w.vera.expect("GET", "/api/types", "", http.StatusOK)); !slices.Contains(names, "incident") {
		t.Fatalf("vera sees %v", names)
	}
	if names := typeNamesOf(w.carla.expect("GET", "/api/types", "", http.StatusOK)); slices.Contains(names, "incident") || !slices.Contains(names, "task") {
		t.Fatalf("carla sees %v", names)
	}
	if names := typeNamesOf(w.admin.expect("GET", "/api/types?space=OPS", "", http.StatusOK)); slices.Contains(names, "incident") {
		t.Fatalf("types of OPS hold %v", names)
	}
	for _, ty := range b.expect("GET", "/api/types?space=PLAT", "", http.StatusOK)["types"].([]any) {
		m := ty.(map[string]any)
		if editable := m["editable"] == true; editable != (m["name"] == "incident") {
			t.Fatalf("bert may change %v: %v", m["name"], editable)
		}
	}

	it := b.expect("POST", "/api/items", `{"space":"PLAT","type":"incident","title":"Outage"}`, http.StatusCreated)
	w.admin.expect("POST", "/api/items", `{"space":"OPS","type":"incident","title":"Elsewhere"}`, http.StatusBadRequest)
	b.expect("DELETE", "/api/types/incident", "", http.StatusConflict)
	w.vera.expect("DELETE", "/api/types/incident", "", http.StatusForbidden)
	b.expect("DELETE", "/api/types/task", "", http.StatusForbidden)
	id := field(it, "item", "id").(string)
	b.expect("POST", "/api/items/"+id+"/trash", "", http.StatusOK)
	b.expect("POST", "/api/items/"+id+"/purge", "", http.StatusOK)
	b.expect("DELETE", "/api/types/incident", "", http.StatusNoContent)
}

// TestTheNewFieldKindsThroughTheAPI: item, multi, rating and file fields
// take their values as JSON, an item by its key too, and read back as the
// caller may read them (ADR-0200).
func TestTheNewFieldKindsThroughTheAPI(t *testing.T) {
	w := newWorld(t)
	w.admin.expect("POST", "/api/types", `{"name":"story","body":true,"fields":[
		{"name":"epic","kind":"item","targets":["task"]},{"name":"platforms","kind":"multi","values":["web","ios"]},
		{"name":"rating","kind":"rating"},{"name":"cover","kind":"file"}]}`, http.StatusCreated)
	types := w.admin.expect("GET", "/api/types", "", http.StatusOK)["types"].([]any)
	for _, ty := range types {
		if m := ty.(map[string]any); m["name"] == "story" {
			if f := m["fields"].([]any)[0].(map[string]any); f["targets"].([]any)[0] != "task" {
				t.Fatalf("epic %v", f)
			}
		}
	}
	b := w.bert
	task := b.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"Epic","properties":{"priority":"low"}}`, http.StatusCreated)
	taskID, taskKey := field(task, "item", "id").(string), field(task, "item", "key").(string)
	st := b.expect("POST", "/api/items", `{"space":"PLAT","type":"story","title":"Login","properties":{"epic":"`+taskKey+`","platforms":["ios","web"],"rating":4}}`, http.StatusCreated)
	id := field(st, "item", "id").(string)
	props := field(st, "item", "properties").(map[string]any)
	if props["epic"] != taskID || props["rating"] != float64(4) || len(props["platforms"].([]any)) != 2 || props["platforms"].([]any)[0] != "web" {
		t.Fatalf("properties %v", props)
	}
	b.expect("PUT", "/api/items/"+id+"/properties/platforms", `{"value":"ios"}`, http.StatusOK)
	for _, bad := range []string{`{"value":7}`, `{"value":3.5}`, `{"value":"x"}`} {
		b.expect("PUT", "/api/items/"+id+"/properties/rating", bad, http.StatusBadRequest)
	}
	b.expect("PUT", "/api/items/"+id+"/properties/platforms", `{"value":[1]}`, http.StatusBadRequest)
	b.expect("PUT", "/api/items/"+id+"/properties/epic", `{"value":"PLAT-999"}`, http.StatusNotFound)
	b.expect("PUT", "/api/items/"+id+"/properties/epic", `{"value":"`+id+`"}`, http.StatusBadRequest)
	b.expect("PUT", "/api/items/"+id+"/properties/cover", `{"value":"0123456789abcdef0123456789abcdef"}`, http.StatusNotFound)

	in := b.expect("GET", "/api/items/"+taskID+"/relations?direction=incoming", "", http.StatusOK)
	rels := in["relations"].([]any)
	if len(rels) != 1 || rels[0].(map[string]any)["type"] != "epic" || rels[0].(map[string]any)["derived"] != true {
		t.Fatalf("incoming %v", rels)
	}
	b.expect("POST", "/api/items/"+taskID+"/trash", "", http.StatusConflict)

	q := b.expect("GET", "/api/items?query="+strings.ReplaceAll("platforms = ios AND rating >= 4 AND epic = "+taskKey, " ", "%20"), "", http.StatusOK)
	if n := len(q["items"].([]any)); n != 1 {
		t.Fatalf("query found %d", n)
	}
	rec := b.do("GET", "/api/items.csv?type=story", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), taskKey) || !strings.Contains(rec.Body.String(), "ios") {
		t.Fatalf("csv %d %s", rec.Code, rec.Body)
	}
}
