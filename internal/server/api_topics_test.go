package server

import (
	"net/http"
	"slices"
	"testing"
)

// TestALabelsPageAndItsOrderThroughTheAPI: a label is described by a page
// in a room; the label's page names it with the places of the items set
// by hand that the caller reads; an editor of the page moves them
// (ADR-0202).
func TestALabelsPageAndItsOrderThroughTheAPI(t *testing.T) {
	w := newWorld(t)
	page := w.vera.expect("GET", "/api/labels/onboarding/page", "", http.StatusOK)
	if page["page"] != nil {
		t.Fatalf("a page before one was made: %v", page)
	}
	w.vera.expect("POST", "/api/labels/onboarding/page", `{"space":"PLAT"}`, http.StatusForbidden)
	w.bert.expect("POST", "/api/labels/Not%20a%20label!/page", `{"space":"PLAT"}`, http.StatusBadRequest)
	w.bert.expect("POST", "/api/labels/onboarding/page", `{"space":"NOPE"}`, http.StatusForbidden)
	made := w.bert.expect("POST", "/api/labels/onboarding/page", `{"space":"PLAT","title":"Onboarding"}`, http.StatusCreated)
	topic := field(made, "item").(map[string]any)
	if topic["type"] != "topic" || topic["title"] != "Onboarding" || field(topic, "properties", "label") != "onboarding" {
		t.Fatalf("made %v", topic)
	}
	// The ready-made type is offered among the templates.
	found := false
	for _, ty := range w.vera.expect("GET", "/api/types/templates", "", http.StatusOK)["templates"].([]any) {
		found = found || ty.(map[string]any)["name"] == "topic"
	}
	if !found {
		t.Error("no topic template")
	}

	var ids []string
	for _, title := range []string{"A", "B", "C"} {
		it := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"`+title+`","properties":{"priority":"low"}}`, http.StatusCreated)
		id := field(it, "item", "id").(string)
		w.bert.expect("PUT", "/api/items/"+id+"/labels", `{"labels":["onboarding"]}`, http.StatusOK)
		ids = append(ids, id)
	}
	key := topic["key"].(string)
	got := w.vera.expect("GET", "/api/labels/onboarding/page", "", http.StatusOK)
	if field(got, "page", "key") != key || got["orderable"] != false || len(got["order"].([]any)) != 0 {
		t.Fatalf("vera reads %v", got)
	}
	order := "/api/items/" + key + "/order"
	w.vera.expect("PUT", order, `{"items":["`+ids[2]+`"]}`, http.StatusForbidden)
	w.bert.expect("PUT", order, `{"items":[]}`, http.StatusBadRequest)
	w.bert.expect("PUT", order, `{"items":["nothing"]}`, http.StatusNotFound)
	if res := w.bert.expect("PUT", order, `{"items":["`+ids[2]+`"],"before":"`+ids[0]+`"}`, http.StatusOK); res["ordered"] != true {
		t.Fatalf("answer %v", res)
	}
	got = w.bert.expect("GET", "/api/labels/onboarding/page", "", http.StatusOK)
	var placed []string
	for _, id := range got["order"].([]any) {
		placed = append(placed, id.(string))
	}
	if !slices.Equal(placed, []string{ids[2], ids[0], ids[1]}) || got["orderable"] != true {
		t.Fatalf("bert reads %v", got)
	}
	w.vera.expect("DELETE", order, "", http.StatusForbidden)
	w.bert.expect("DELETE", order, "", http.StatusOK)
	if got := w.bert.expect("GET", "/api/labels/onboarding/page", "", http.StatusOK); len(got["order"].([]any)) != 0 {
		t.Errorf("cleared %v", got)
	}
	// One who reads no room finds no page.
	if got := w.carla.expect("GET", "/api/labels/onboarding/page", "", http.StatusOK); got["page"] != nil {
		t.Errorf("carla reads %v", got)
	}
	w.bert.expect("GET", "/api/labels/Not%20a%20label!/page", "", http.StatusBadRequest)
	// A page that is no topic has no order.
	plain := field(w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"Plain","properties":{"priority":"low"}}`, http.StatusCreated), "item", "id").(string)
	w.bert.expect("PUT", "/api/items/"+plain+"/order", `{"items":["`+ids[0]+`"]}`, http.StatusBadRequest)
	w.bert.expect("DELETE", "/api/items/"+plain+"/order", "", http.StatusBadRequest)
	w.bert.expect("DELETE", "/api/items/NOPE-1/order", "", http.StatusNotFound)
}

// TestListingsTellWhenItemsChanged: a listing gives each item's last
// change, for the column updated of a view (ADR-0201).
func TestListingsTellWhenItemsChanged(t *testing.T) {
	w := newWorld(t)
	w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"A","properties":{"priority":"low"}}`, http.StatusCreated)
	items := w.vera.expect("GET", "/api/items?space=PLAT", "", http.StatusOK)["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["updated"] == nil {
		t.Errorf("listing %v", items)
	}
}

// TestAViewBlockIsWrittenAndReadThroughTheAPI: a view in a text keeps
// what it shows (ADR-0201).
func TestAViewBlockIsWrittenAndReadThroughTheAPI(t *testing.T) {
	w := newWorld(t)
	id := field(w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"Overview","properties":{"priority":"low"}}`, http.StatusCreated), "item", "id").(string)
	w.bert.expect("PUT", "/api/items/"+id+"/body", `{"blocks":[{"kind":"view","layout":"gallery","itemType":"task","space":"PLAT","query":"state = open",
		"group":"field:priority","columns":["title","field:priority"],"sort":"-updated"}]}`, http.StatusOK)
	b := field(w.vera.expect("GET", "/api/items/"+id, "", http.StatusOK), "item", "body").([]any)[0].(map[string]any)
	if b["kind"] != "view" || b["layout"] != "gallery" || b["itemType"] != "task" || b["space"] != "PLAT" || b["query"] != "state = open" ||
		b["group"] != "field:priority" || len(b["columns"].([]any)) != 2 || b["sort"] != "-updated" {
		t.Errorf("block %v", b)
	}
	w.bert.expect("PUT", "/api/items/"+id+"/body", `{"blocks":[{"kind":"view","layout":"table","columns":[]}]}`, http.StatusOK)
	w.bert.expect("PUT", "/api/items/"+id+"/body", `{"blocks":[{"kind":"view","layout":"board"}]}`, http.StatusBadRequest)
}
