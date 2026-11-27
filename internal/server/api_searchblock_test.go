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
// (ADR-draft-a-search-field-in-a-text).
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
