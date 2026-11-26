package server

import (
	"bytes"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

// A slide deck in a text keeps its slides as files of its item, copies of
// an album folder's pictures, and names the folder it came from; the
// folder may go while the deck keeps its slides; the exports name the
// deck (ADR-0260).
func TestASlideDeckInATextThroughTheAPI(t *testing.T) {
	w := newWorld(t)
	w.admin.expect("POST", "/api/types", `{"name":"page","body":true,"fields":[]}`, http.StatusCreated)
	it := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"page","title":"Review"}`, http.StatusCreated)
	home := field(it, "item", "id").(string)
	folder := field(w.bert.expect("POST", "/api/album/folders", `{"space":"PLAT","name":"Talk"}`, http.StatusCreated), "item", "id").(string)
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 100)...)
	original := w.bert.uploaded(folder, "01-Intro.png", png, "")
	// A copy of the folder's picture, as the browser makes it.
	rec := w.bert.upload("/api/items/"+home+"/attachments?name=01-Intro.png&sourceItem="+folder+"&sourceAttachment="+original, "application/octet-stream", png)
	if rec.Code != http.StatusCreated {
		t.Fatalf("copy: %d %s", rec.Code, rec.Body)
	}
	first := field(decodeBody(t, rec), "attachment", "id").(string)
	second := w.bert.uploaded(home, "02-Plan.png", png, "")
	w.bert.expect("PUT", "/api/items/"+home+"/body", `{"blocks":[{"kind":"slides","ref":"`+folder+`","files":["`+second+`","`+first+`"],"text":"Board, 4 December"}]}`, http.StatusOK)
	body := field(w.vera.expect("GET", "/api/items/"+home, "", http.StatusOK), "item", "body").([]any)
	deck := body[0].(map[string]any)
	if deck["kind"] != "slides" || deck["ref"] != folder || deck["text"] != "Board, 4 December" || !reflect.DeepEqual(deck["files"], []any{second, first}) {
		t.Errorf("deck %v", deck)
	}
	// Only files of the item, a source that is a folder, slides kept.
	w.bert.expect("PUT", "/api/items/"+home+"/body", `{"blocks":[{"kind":"slides","ref":"`+folder+`","files":["`+original+`"]}]}`, http.StatusNotFound)
	w.bert.expect("PUT", "/api/items/"+home+"/body", `{"blocks":[{"kind":"slides","ref":"`+home+`","files":["`+first+`"]}]}`, http.StatusBadRequest)
	w.bert.expect("PUT", "/api/items/"+home+"/body", `{"blocks":[{"kind":"slides","ref":"`+folder+`"}]}`, http.StatusBadRequest)
	// The album leaves the slides out, but lists them for their own item,
	// as a deck's presentation reads them; it says they are used, so their
	// versions are kept.
	for _, p := range w.vera.expect("GET", "/api/album?q=Intro", "", http.StatusOK)["pictures"].([]any) {
		if field(p.(map[string]any), "item", "id") == home {
			t.Errorf("the album shows a slide: %v", p)
		}
	}
	album := w.vera.expect("GET", "/api/album?folder="+home, "", http.StatusOK)
	if len(album["pictures"].([]any)) != 2 {
		t.Errorf("the item's own listing shows its two slides: %v", album["pictures"])
	}
	for _, p := range album["pictures"].([]any) {
		if !p.(map[string]any)["used"].(bool) {
			t.Errorf("a slide the body shows is used: %v", p)
		}
	}
	// The folder may go; the deck keeps its slides.
	w.bert.expect("POST", "/api/items/"+folder+"/trash", "", http.StatusOK)
	if got := field(w.vera.expect("GET", "/api/items/"+home, "", http.StatusOK), "item", "body").([]any)[0].(map[string]any)["files"]; !reflect.DeepEqual(got, []any{second, first}) {
		t.Errorf("the deck keeps its slides: %v", got)
	}
	// A copy of the page shows its own copies of the slides.
	made := w.bert.expect("POST", "/api/items/"+home+"/copy", `{"space":"PLAT","title":"Review (copy)"}`, http.StatusCreated)
	copyID := field(made, "item", "id").(string)
	copied := field(w.bert.expect("GET", "/api/items/"+copyID, "", http.StatusOK), "item", "body").([]any)[0].(map[string]any)
	if copied["ref"] != nil {
		t.Errorf("the copy forgets a source in the trash: %v", copied["ref"])
	}
	files := copied["files"].([]any)
	if len(files) != 2 || files[0] == second || files[1] == first {
		t.Errorf("the copy's deck shows the copy's files: %v", files)
	}
	md := w.vera.do("GET", "/api/items/"+home+"/export/md", "").Body.String()
	if !strings.Contains(md, "Slide deck: Board, 4 December (2 slides)") || strings.Count(md, "Board, 4 December") != 1 {
		t.Errorf("export:\n%s", md)
	}
}
