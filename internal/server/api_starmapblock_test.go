package server

import (
	"net/http"
	"reflect"
	"strings"
	"testing"
)

// A starmap in a text keeps how it is narrowed when shown, its hidden
// kinds named once each in their order; the export names it
// (ADR-draft-a-starmap-in-a-text).
func TestAStarmapInATextThroughTheAPI(t *testing.T) {
	w := newWorld(t)
	w.admin.expect("POST", "/api/types", `{"name":"page","body":true,"fields":[]}`, http.StatusCreated)
	it := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"page","title":"Map"}`, http.StatusCreated)
	home := field(it, "item", "id").(string)
	w.bert.expect("PUT", "/api/items/"+home+"/body",
		`{"blocks":[{"kind":"starmap","space":"*","label":"onboarding","search":"plan","hideTypes":["task"],"hideKinds":["picture","done","picture"],"depth":2},{"kind":"starmap","hideKinds":[]}]}`, http.StatusOK)
	body := field(w.vera.expect("GET", "/api/items/"+home, "", http.StatusOK), "item", "body").([]any)
	first := body[0].(map[string]any)
	if first["kind"] != "starmap" || first["space"] != "*" || first["label"] != "onboarding" || first["search"] != "plan" || first["depth"] != 2.0 ||
		!reflect.DeepEqual(first["hideKinds"], []any{"done", "picture"}) || !reflect.DeepEqual(first["hideTypes"], []any{"task"}) {
		t.Errorf("first starmap %v", first)
	}
	if _, ok := body[1].(map[string]any)["hideKinds"]; ok {
		t.Errorf("an empty list is kept as none: %v", body[1])
	}
	w.bert.expect("PUT", "/api/items/"+home+"/body", `{"blocks":[{"kind":"starmap","hideKinds":["item"]}]}`, http.StatusBadRequest)
	w.bert.expect("PUT", "/api/items/"+home+"/body", `{"blocks":[{"kind":"starmap","depth":4}]}`, http.StatusBadRequest)
	w.bert.expect("PUT", "/api/items/"+home+"/body", `{"blocks":[{"kind":"paragraph","text":"x","label":"onboarding"}]}`, http.StatusBadRequest)
	md := w.vera.do("GET", "/api/items/"+home+"/export/md", "").Body.String()
	if !strings.Contains(md, "Starmap of every room around this item, depth 2, label onboarding, matching plan, without task, done, picture") {
		t.Errorf("export:\n%s", md)
	}
}
