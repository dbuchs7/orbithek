package server

import (
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
)

// A search field in a text keeps its scope and the page it names; the
// search narrows to a page, the page with the pages below it, or the
// pages below it only; the export names it
// (ADR-0264).
func TestASearchFieldInATextThroughTheAPI(t *testing.T) {
	w := newWorld(t)
	w.admin.expect("POST", "/api/types", `{"name":"page","body":true,"fields":[]}`, http.StatusCreated)
	mk := func(title, parent string) string {
		body := `{"space":"PLAT","type":"page","title":"` + title + `"`
		if parent != "" {
			body += `,"parent":"` + parent + `"`
		}
		return field(w.bert.expect("POST", "/api/items", body+`}`, http.StatusCreated), "item", "id").(string)
	}
	top := mk("Handbook rocket", "")
	child := mk("Chapter rocket", top)
	mk("Section rocket", child)
	mk("Elsewhere rocket", "")
	home := mk("Find", "")

	w.bert.expect("PUT", "/api/items/"+home+"/body",
		`{"blocks":[{"kind":"search","scope":"below","ref":"`+top+`"},{"kind":"search","space":"PLAT"}]}`, http.StatusOK)
	body := field(w.vera.expect("GET", "/api/items/"+home, "", http.StatusOK), "item", "body").([]any)
	first := body[0].(map[string]any)
	if first["kind"] != "search" || first["scope"] != "below" || first["ref"] != top {
		t.Errorf("first search field %v", first)
	}
	if second := body[1].(map[string]any); second["space"] != "PLAT" || second["scope"] != nil {
		t.Errorf("second search field %v", second)
	}
	w.bert.expect("PUT", "/api/items/"+home+"/body", `{"blocks":[{"kind":"search","scope":"children"}]}`, http.StatusBadRequest)
	w.bert.expect("PUT", "/api/items/"+home+"/body", `{"blocks":[{"kind":"search","ref":"`+top+`"}]}`, http.StatusBadRequest)
	w.bert.expect("PUT", "/api/items/"+home+"/body", `{"blocks":[{"kind":"paragraph","text":"x","scope":"tree"}]}`, http.StatusBadRequest)

	titles := func(within, scope string) []string {
		t.Helper()
		res := w.vera.expect("GET", "/api/search?q=rocket&within="+url.QueryEscape(within)+"&scope="+scope, "", http.StatusOK)
		var out []string
		for _, h := range res["hits"].([]any) {
			out = append(out, field(h.(map[string]any), "item", "title").(string))
		}
		slices.Sort(out)
		return out
	}
	if got := titles(top, "page"); !slices.Equal(got, []string{"Handbook rocket"}) {
		t.Errorf("page: %v", got)
	}
	if got := titles(top, "tree"); !slices.Equal(got, []string{"Chapter rocket", "Handbook rocket", "Section rocket"}) {
		t.Errorf("tree: %v", got)
	}
	if got := titles(top, "below"); !slices.Equal(got, []string{"Chapter rocket", "Section rocket"}) {
		t.Errorf("below: %v", got)
	}
	key := field(w.vera.expect("GET", "/api/items/"+top, "", http.StatusOK), "item", "key").(string)
	if got := titles(key, "below"); len(got) != 2 {
		t.Errorf("below, by key: %v", got)
	}
	w.vera.expect("GET", "/api/search?q=rocket&within="+top, "", http.StatusBadRequest)
	w.vera.expect("GET", "/api/search?q=rocket&scope=tree", "", http.StatusBadRequest)
	w.vera.expect("GET", "/api/search?q=rocket&within=nothing&scope=tree", "", http.StatusNotFound)

	md := w.vera.do("GET", "/api/items/"+home+"/export/md", "").Body.String()
	for _, want := range []string{"Search field: the pages below another page", "Search field: room PLAT"} {
		if !strings.Contains(md, want) {
			t.Errorf("export lacks %q:\n%s", want, md)
		}
	}
}

// A search field in a text keeps a label and a type its hits carry, as
// the search page's filters narrow, together with its room or page; the
// search narrows by both within a page, and the export names them
// (ADR-0264).
func TestASearchFieldNarrowedByLabelAndTypeThroughTheAPI(t *testing.T) {
	w := newWorld(t)
	w.admin.expect("POST", "/api/types", `{"name":"page","body":true,"fields":[]}`, http.StatusCreated)
	w.admin.expect("POST", "/api/types", `{"name":"note","body":true,"fields":[]}`, http.StatusCreated)
	mk := func(typ, title, parent string, labels ...string) string {
		body := `{"space":"PLAT","type":"` + typ + `","title":"` + title + `"`
		if parent != "" {
			body += `,"parent":"` + parent + `"`
		}
		id := field(w.bert.expect("POST", "/api/items", body+`}`, http.StatusCreated), "item", "id").(string)
		if len(labels) > 0 {
			w.bert.expect("PUT", "/api/items/"+id+"/labels", `{"labels":["`+strings.Join(labels, `","`)+`"]}`, http.StatusOK)
		}
		return id
	}
	top := mk("page", "Handbook rocket", "", "how-to")
	mk("page", "Chapter rocket", top, "how-to")
	mk("note", "Note rocket", top, "how-to")
	mk("page", "Draft rocket", top)
	mk("page", "Elsewhere rocket", "", "how-to")

	w.bert.expect("PUT", "/api/items/"+top+"/body",
		`{"blocks":[{"kind":"search","scope":"below","label":"how-to","itemType":"page"},{"kind":"search","space":"PLAT","label":"how-to"}]}`, http.StatusOK)
	body := field(w.vera.expect("GET", "/api/items/"+top, "", http.StatusOK), "item", "body").([]any)
	if first := body[0].(map[string]any); first["label"] != "how-to" || first["itemType"] != "page" || first["scope"] != "below" {
		t.Errorf("first search field %v", first)
	}
	if second := body[1].(map[string]any); second["label"] != "how-to" || second["itemType"] != nil {
		t.Errorf("second search field %v", second)
	}
	w.bert.expect("PUT", "/api/items/"+top+"/body", `{"blocks":[{"kind":"search","label":"How To"}]}`, http.StatusBadRequest)
	w.bert.expect("PUT", "/api/items/"+top+"/body", `{"blocks":[{"kind":"search","itemType":" page"}]}`, http.StatusBadRequest)
	w.bert.expect("PUT", "/api/items/"+top+"/body", `{"blocks":[{"kind":"search","layout":"table"}]}`, http.StatusBadRequest)

	titles := func(query string) []string {
		t.Helper()
		res := w.vera.expect("GET", "/api/search?q=rocket&"+query, "", http.StatusOK)
		var out []string
		for _, h := range res["hits"].([]any) {
			out = append(out, field(h.(map[string]any), "item", "title").(string))
		}
		slices.Sort(out)
		return out
	}
	if got := titles("within=" + top + "&scope=below&label=how-to&type=page"); !slices.Equal(got, []string{"Chapter rocket"}) {
		t.Errorf("below, label and type: %v", got)
	}
	if got := titles("within=" + top + "&scope=tree&label=how-to"); !slices.Equal(got, []string{"Chapter rocket", "Handbook rocket", "Note rocket"}) {
		t.Errorf("tree and label: %v", got)
	}
	if got := titles("space=PLAT&label=how-to&type=page"); !slices.Equal(got, []string{"Chapter rocket", "Elsewhere rocket", "Handbook rocket"}) {
		t.Errorf("room, label and type: %v", got)
	}

	md := w.vera.do("GET", "/api/items/"+top+"/export/md", "").Body.String()
	for _, want := range []string{"Search field: the pages below this page, label how-to, type page", "Search field: room PLAT, label how-to"} {
		if !strings.Contains(md, want) {
			t.Errorf("export lacks %q:\n%s", want, md)
		}
	}
}
