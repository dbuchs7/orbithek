package server

import (
	"net/http"
	"strings"
	"testing"
)

// A slide deck in a text names an album folder by its ID and keeps its
// caption; the API refuses one of a page, and the folder stays while a
// text shows it; the exports name the deck
// (ADR-0260).
func TestASlideDeckInATextThroughTheAPI(t *testing.T) {
	w := newWorld(t)
	w.admin.expect("POST", "/api/types", `{"name":"page","body":true,"fields":[]}`, http.StatusCreated)
	it := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"page","title":"Review"}`, http.StatusCreated)
	home := field(it, "item", "id").(string)
	folder := field(w.bert.expect("POST", "/api/album/folders", `{"space":"PLAT","name":"Talk"}`, http.StatusCreated), "item", "id").(string)
	w.bert.expect("PUT", "/api/items/"+home+"/body", `{"blocks":[{"kind":"slides","ref":"`+folder+`","text":"Board, 4 December"}]}`, http.StatusOK)
	body := field(w.vera.expect("GET", "/api/items/"+home, "", http.StatusOK), "item", "body").([]any)
	deck := body[0].(map[string]any)
	if deck["kind"] != "slides" || deck["ref"] != folder || deck["text"] != "Board, 4 December" {
		t.Errorf("deck %v", deck)
	}
	w.bert.expect("PUT", "/api/items/"+home+"/body", `{"blocks":[{"kind":"slides","ref":"`+home+`"}]}`, http.StatusBadRequest)
	w.bert.expect("PUT", "/api/items/"+home+"/body", `{"blocks":[{"kind":"slides"}]}`, http.StatusBadRequest)
	w.bert.expect("POST", "/api/items/"+folder+"/trash", "", http.StatusConflict)
	md := w.vera.do("GET", "/api/items/"+home+"/export/md", "").Body.String()
	if !strings.Contains(md, "Slide deck: Board, 4 December") || strings.Count(md, "Board, 4 December") != 1 {
		t.Errorf("export:\n%s", md)
	}
}
