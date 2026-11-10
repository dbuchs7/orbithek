package server

import (
	"net/http"
	"slices"
	"testing"

	"github.com/dbuchs7/orbithek/internal/budget"
)

// The starmap of a space (ADR-0157): its items, a relation and a shared
// label as nodes and edges, read with a read token too; cut at the budget
// and said so; an unknown space or one without a role is not found.
func TestStarmapThroughTheAPI(t *testing.T) {
	w := newWorld(t)
	b := w.bert
	create := func(title, parent string) string {
		it := b.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"`+title+`","parent":"`+parent+`","properties":{"priority":"low"}}`, http.StatusCreated)
		return field(it, "item", "id").(string)
	}
	home := create("Home", "")
	child := create("Child", home)
	b.expect("POST", "/api/relations", `{"type":"blocks","from":"`+child+`","to":"`+home+`"}`, http.StatusCreated)
	for _, id := range []string{home, child} {
		b.expect("PUT", "/api/items/"+id+"/labels", `{"labels":["ops"]}`, http.StatusOK)
	}

	m := readToken(w.vera).expect("GET", "/api/spaces/PLAT/starmap", "", http.StatusOK)
	var ids []string
	for _, n := range m["nodes"].([]any) {
		ids = append(ids, n.(map[string]any)["id"].(string))
	}
	if !slices.Equal(ids, []string{home, child, "label:ops"}) || m["truncated"] != false {
		t.Fatalf("starmap %v", m)
	}
	first := m["nodes"].([]any)[0].(map[string]any)
	if first["kind"] != "item" || first["key"] != "PLAT-1" || first["title"] != "Home" || first["type"] != "task" ||
		first["space"] != "PLAT" || first["state"] != "open" || first["outside"] != nil {
		t.Errorf("node %v", first)
	}
	kinds := map[string]bool{}
	for _, e := range m["edges"].([]any) {
		e := e.(map[string]any)
		kinds[e["kind"].(string)] = true
		if e["kind"] == "relation" && (e["type"] != "blocks" || e["from"] != child || e["to"] != home) {
			t.Errorf("relation %v", e)
		}
	}
	if !kinds["parent"] || !kinds["relation"] || !kinds["label"] {
		t.Errorf("edges %v", m["edges"])
	}

	small := budget.Default()
	small.StarmapNodes = 1
	budget.Set(small)
	defer budget.Set(budget.Default())
	m = w.vera.expect("GET", "/api/spaces/PLAT/starmap", "", http.StatusOK)
	if len(m["nodes"].([]any)) != 1 || len(m["edges"].([]any)) != 0 || m["truncated"] != true {
		t.Errorf("truncated starmap %v", m)
	}

	w.vera.expect("GET", "/api/spaces/NOPE/starmap", "", http.StatusNotFound)
	w.carla.expect("GET", "/api/spaces/PLAT/starmap", "", http.StatusNotFound)
}

// The starmap of all spaces (ADR-0191): every readable item of every
// space, narrowed by a label, and grouped per space and type beyond the
// budget; an outsider sees an empty map.
func TestStarmapOfAllSpacesThroughTheAPI(t *testing.T) {
	w := newWorld(t)
	b := w.bert
	var ids []string
	for _, title := range []string{"One", "Two", "Three"} {
		it := b.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"`+title+`","properties":{"priority":"low"}}`, http.StatusCreated)
		ids = append(ids, field(it, "item", "id").(string))
	}
	b.expect("PUT", "/api/items/"+ids[0]+"/labels", `{"labels":["q4"]}`, http.StatusOK)

	m := w.vera.expect("GET", "/api/starmap", "", http.StatusOK)
	if len(m["nodes"].([]any)) != 3 || m["grouped"] != false || m["items"] != 3.0 {
		t.Fatalf("all spaces %v", m)
	}
	m = w.vera.expect("GET", "/api/starmap?label=q4", "", http.StatusOK)
	if n := m["nodes"].([]any); len(n) != 1 || n[0].(map[string]any)["id"] != ids[0] {
		t.Errorf("by label %v", m)
	}
	bud := budget.Get()
	bud.StarmapNodes = 2
	budget.Set(bud)
	t.Cleanup(func() { budget.Set(budget.Default()) })
	m = w.vera.expect("GET", "/api/starmap", "", http.StatusOK)
	g := m["nodes"].([]any)
	if m["grouped"] != true || len(g) != 1 || g[0].(map[string]any)["kind"] != "group" || g[0].(map[string]any)["count"] != 3.0 {
		t.Errorf("grouped %v", m)
	}
	if m := w.carla.expect("GET", "/api/starmap", "", http.StatusOK); len(m["nodes"].([]any)) != 0 || m["items"] != 0.0 {
		t.Errorf("an outsider's map %v", m)
	}
}
