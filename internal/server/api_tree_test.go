package server

import (
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"strings"
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

// nested writes a list of child pages as titles, a page's children in
// brackets after it and "+" for a page with more than the list shows.
func nested(items []any) string {
	out := ""
	for i, n := range items {
		m := n.(map[string]any)
		if i > 0 {
			out += " "
		}
		out += m["title"].(string)
		if m["more"] == true {
			out += "+"
		}
		if kids, ok := m["children"].([]any); ok {
			out += "[" + nested(kids) + "]"
		}
	}
	return out
}

func TestChildPagesToAChosenDepthThroughTheAPI(t *testing.T) {
	w := newWorld(t)
	w.admin.expect("POST", "/api/types", `{"name":"page","body":true,"fields":[]}`, http.StatusCreated)
	create := func(title, parent string) string {
		it := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"page","title":"`+title+`","parent":"`+parent+`"}`, http.StatusCreated)
		return field(it, "item", "id").(string)
	}
	home := create("Home", "")
	a := create("A", home)
	a1 := create("A1", a)
	create("A1a", a1)
	create("B", home)

	for depth, want := range map[string]string{
		"1":   "A+ B",
		"2":   "A[A1+] B",
		"3":   "A[A1[A1a]] B",
		"all": "A[A1[A1a]] B",
	} {
		res := w.vera.expect("GET", "/api/spaces/PLAT/tree?parent="+home+"&depth="+depth, "", http.StatusOK)
		if got := nested(res["items"].([]any)); got != want || res["truncated"] != false {
			t.Errorf("depth %s: %q (truncated %v), want %q", depth, got, res["truncated"], want)
		}
	}
	for _, bad := range []string{"0", "6", "-1", "x", ""} {
		w.vera.expect("GET", "/api/spaces/PLAT/tree?parent="+home+"&depth="+bad, "", http.StatusBadRequest)
	}
	w.carla.expect("GET", "/api/spaces/PLAT/tree?parent="+home+"&depth=2", "", http.StatusNotFound)

	// A body keeps the depth of its lists; one level is written as none.
	w.bert.expect("PUT", "/api/items/"+home+"/body", `{"blocks":[{"kind":"children","depth":3},{"kind":"children","depth":1},{"kind":"children","depth":-1}]}`, http.StatusOK)
	body := field(w.vera.expect("GET", "/api/items/"+home, "", http.StatusOK), "item", "body").([]any)
	for i, want := range []any{3.0, nil, -1.0} {
		if got := body[i].(map[string]any)["depth"]; got != want {
			t.Errorf("block %d: depth %v, want %v", i+1, got, want)
		}
	}
	// The export lists each list to its own depth.
	md := w.vera.do("GET", "/api/items/"+home+"/export/md", "").Body.String()
	if !strings.Contains(md, "- A\n  - A1\n    - A1a\n- B\n") || !strings.Contains(md, "\n- A\n- B\n") {
		t.Errorf("export:\n%s", md)
	}
	w.bert.expect("PUT", "/api/items/"+home+"/body", `{"blocks":[{"kind":"children","depth":9}]}`, http.StatusBadRequest)
	w.bert.expect("PUT", "/api/items/"+home+"/body", `{"blocks":[{"kind":"toc","depth":2}]}`, http.StatusBadRequest)
}

func TestChildPagesLeaveOutTypesThroughTheAPI(t *testing.T) {
	w := newWorld(t)
	w.admin.expect("POST", "/api/types", `{"name":"page","body":true,"fields":[]}`, http.StatusCreated)
	w.admin.expect("POST", "/api/types", `{"name":"chore","body":true,"fields":[]}`, http.StatusCreated)
	create := func(title, typ, parent string) string {
		it := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"`+typ+`","title":"`+title+`","parent":"`+parent+`"}`, http.StatusCreated)
		return field(it, "item", "id").(string)
	}
	home := create("Home", "page", "")
	a := create("A", "page", home)
	create("Sweep", "chore", a)
	create("A1", "page", a)
	chore := create("Dust", "chore", home)
	create("Below the dust", "page", chore)

	for hide, want := range map[string]string{
		"":             "A[Sweep A1] Dust[Below the dust]",
		"chore":        "A[A1] Below the dust",
		" chore , x ,": "A[A1] Below the dust",
	} {
		res := w.vera.expect("GET", "/api/spaces/PLAT/tree?parent="+home+"&depth=all&hide="+url.QueryEscape(hide), "", http.StatusOK)
		if got := nested(res["items"].([]any)); got != want {
			t.Errorf("hide %q: %q, want %q", hide, got, want)
		}
	}

	// A body keeps the types a list leaves out, named once each and sorted.
	w.bert.expect("PUT", "/api/items/"+home+"/body", `{"blocks":[{"kind":"children","depth":-1,"hideTypes":["task","chore","task"]},{"kind":"children","hideTypes":[]}]}`, http.StatusOK)
	body := field(w.vera.expect("GET", "/api/items/"+home, "", http.StatusOK), "item", "body").([]any)
	if got := body[0].(map[string]any)["hideTypes"]; !reflect.DeepEqual(got, []any{"chore", "task"}) {
		t.Errorf("hidden types %v", got)
	}
	if _, ok := body[1].(map[string]any)["hideTypes"]; ok {
		t.Errorf("an empty list is kept as none: %v", body[1])
	}
	w.bert.expect("PUT", "/api/items/"+home+"/body", `{"blocks":[{"kind":"toc","hideTypes":["chore"]}]}`, http.StatusBadRequest)

	// The export leaves them out too.
	md := w.vera.do("GET", "/api/items/"+home+"/export/md", "").Body.String()
	// The first list leaves the chores out, the page below one in its
	// place; the second, which leaves nothing out, shows one level.
	if !strings.Contains(md, "- A\n  - A1\n- Below the dust\n\n- A\n- Dust\n") || strings.Contains(md, "Sweep") {
		t.Errorf("export:\n%s", md)
	}
}
