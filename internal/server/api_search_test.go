package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func searchKeys(t *testing.T, res map[string]any) []string {
	t.Helper()
	var keys []string
	for _, h := range res["hits"].([]any) {
		keys = append(keys, field(h.(map[string]any), "item", "key").(string))
	}
	return keys
}

func TestSearchThroughTheAPI(t *testing.T) {
	w := newWorld(t)
	b := w.bert
	for _, title := range []string{"Patch the kernel", "Kernel upgrade notes", "Unrelated"} {
		b.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"`+title+`","properties":{"priority":"low"}}`, http.StatusCreated)
	}
	b.expect("PUT", "/api/items/PLAT-3/body", `{"blocks":[{"kind":"paragraph","text":"Mentions the kernel only in passing."}]}`, http.StatusOK)

	res := w.vera.expect("GET", "/api/search?q=kernel", "", http.StatusOK)
	if got := strings.Join(searchKeys(t, res), ","); got != "PLAT-2,PLAT-1,PLAT-3" {
		t.Fatalf("hits %s", got) // titles first, newer first among equals
	}
	last := res["hits"].([]any)[2].(map[string]any)
	if last["snippet"] != "Mentions the kernel only in passing." {
		t.Fatalf("snippet %v", last["snippet"])
	}
	if field(res, "total", "value") != 3.0 {
		t.Fatalf("total %v", res["total"])
	}

	page := w.vera.expect("GET", "/api/search?q=kernel&limit=2", "", http.StatusOK)
	next := page["next"].(string)
	rest := w.vera.expect("GET", "/api/search?q=kernel&limit=2&after="+url.QueryEscape(next), "", http.StatusOK)
	if got := strings.Join(searchKeys(t, rest), ","); got != "PLAT-3" {
		t.Fatalf("second page %s", got)
	}

	// Someone without a role finds nothing, not even that items exist.
	if got := w.carla.expect("GET", "/api/search?q=kernel", "", http.StatusOK); len(got["hits"].([]any)) != 0 || field(got, "total", "value") != 0.0 {
		t.Fatalf("carla finds %v", got)
	}

	w.vera.expect("GET", "/api/search?q=", "", http.StatusBadRequest)
	w.vera.expect("GET", "/api/search?q=kernel&limit=0", "", http.StatusBadRequest)
	w.vera.expect("GET", "/api/search?q=kernel&after=nope", "", http.StatusBadRequest)
	anon := &client{t: t, h: b.h}
	anon.expect("GET", "/api/search?q=kernel", "", http.StatusUnauthorized)
}

// A search finds an item by its creator's username, and the people who
// made readable items are suggested by it, never their mail address, nor
// anyone whose items the caller does not read
// (ADR-0263).
func TestSearchFindsItemsByTheirCreator(t *testing.T) {
	w := newWorld(t)
	w.bert.expect("PUT", "/api/me/display-name", `{"displayName":"Robert Huber"}`, http.StatusNoContent)
	w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"Runbook","properties":{"priority":"low"}}`, http.StatusCreated)
	w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"Release notes","properties":{"priority":"low"}}`, http.StatusCreated)

	res := w.vera.expect("GET", "/api/search?q=ber", "", http.StatusOK)
	if got := strings.Join(searchKeys(t, res), ","); got != "PLAT-2,PLAT-1" {
		t.Fatalf("hits by the creator %q", got)
	}
	// The display name is seen by its owner only (ADR-0246).
	if got := w.vera.expect("GET", "/api/search?q=huber", "", http.StatusOK); len(got["hits"].([]any)) != 0 {
		t.Fatalf("found by the display name: %v", searchKeys(t, got))
	}

	people := w.vera.expect("GET", "/api/people?q=BER", "", http.StatusOK)["people"].([]any)
	if len(people) != 1 || field(people[0].(map[string]any), "username") != "bert" || field(people[0].(map[string]any), "count") != 2.0 {
		t.Fatalf("people %v", people)
	}
	if got := w.vera.expect("GET", "/api/people?q=nobody", "", http.StatusOK)["people"].([]any); len(got) != 0 {
		t.Fatalf("people for nobody %v", got)
	}
	// Who reads no item learns of nobody, and the accounts are not listed.
	if got := w.carla.expect("GET", "/api/people", "", http.StatusOK)["people"].([]any); len(got) != 0 {
		t.Fatalf("carla learns of %v", got)
	}

	// The starmap names an item's creator by username.
	nodes := w.vera.expect("GET", "/api/spaces/PLAT/starmap", "", http.StatusOK)["nodes"].([]any)
	for _, n := range nodes {
		if m := n.(map[string]any); m["kind"] == "item" && m["creator"] != "bert" {
			t.Fatalf("node %v names creator %v", m["key"], m["creator"])
		}
	}
}
