package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"testing"
)

// TestTheSlidesOfAFolderAreOrderedByHandThroughTheAPI: whoever may edit a
// folder moves its pictures within the order of its slides; the album
// lists them so with order=slides, those not yet placed after the others
// by name (ADR-0195).
func TestTheSlidesOfAFolderAreOrderedByHandThroughTheAPI(t *testing.T) {
	w := newWorld(t)
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 100)...)
	talk := field(w.bert.expect("POST", "/api/album/folders", `{"space":"PLAT","name":"Talk"}`, http.StatusCreated), "item").(map[string]any)
	id, key := talk["id"].(string), talk["key"].(string)
	ids := map[string]string{}
	for _, name := range []string{"10 End.png", "2 Goal.png", "1 Start.png"} {
		rec := w.bert.upload("/api/items/"+id+"/attachments?name="+url.QueryEscape(name), "application/octet-stream", png)
		if rec.Code != http.StatusCreated {
			t.Fatalf("upload %s: %d %s", name, rec.Code, rec.Body)
		}
		var up struct {
			Attachment struct {
				ID string `json:"id"`
			} `json:"attachment"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &up); err != nil {
			t.Fatal(err)
		}
		ids[name] = up.Attachment.ID
	}
	names := func(query string) ([]string, bool) {
		page := w.vera.expect("GET", "/api/album"+query, "", http.StatusOK)
		var out []string
		for _, p := range field(page, "pictures").([]any) {
			out = append(out, p.(map[string]any)["attachment"].(map[string]any)["name"].(string))
		}
		return out, page["ordered"] == true
	}
	if got, ordered := names("?order=slides&folder=" + id); ordered || !slices.Equal(got, []string{"1 Start.png", "2 Goal.png", "10 End.png"}) {
		t.Fatalf("by name %v %v", got, ordered)
	}
	order := "/api/album/order/" + key
	w.vera.expect("PUT", order, `{"pictures":["`+ids["10 End.png"]+`"],"before":"`+ids["1 Start.png"]+`"}`, http.StatusForbidden)
	w.bert.expect("PUT", order, `{"pictures":["0123456789abcdef0123456789abcdef"]}`, http.StatusNotFound)
	w.bert.expect("PUT", order, `{"pictures":[]}`, http.StatusBadRequest)
	if got := w.bert.expect("PUT", order, `{"pictures":["`+ids["10 End.png"]+`"],"before":"`+ids["1 Start.png"]+`"}`, http.StatusOK); got["ordered"] != true {
		t.Fatalf("answer %v", got)
	}
	if got, ordered := names("?order=slides&folder=" + id); !ordered || !slices.Equal(got, []string{"10 End.png", "1 Start.png", "2 Goal.png"}) {
		t.Fatalf("ordered by hand %v %v", got, ordered)
	}
	if page := w.bert.expect("GET", "/api/album?order=slides&folder="+id, "", http.StatusOK); page["orderable"] != true {
		t.Errorf("an editor may order: %v", page["orderable"])
	}
	if page := w.vera.expect("GET", "/api/album?order=slides&folder="+id, "", http.StatusOK); page["orderable"] != nil {
		t.Errorf("a reader may order: %v", page["orderable"])
	}
	// Paged as ordered.
	if got, _ := names("?order=slides&limit=1&offset=1&folder=" + id); !slices.Equal(got, []string{"1 Start.png"}) {
		t.Fatalf("second page %v", got)
	}
	// Without order=slides the album stays newest first.
	if got, _ := names("?folder=" + id); !slices.Equal(got, []string{"1 Start.png", "2 Goal.png", "10 End.png"}) && !slices.Equal(got, []string{"10 End.png", "2 Goal.png", "1 Start.png"}) {
		t.Fatalf("newest first %v", got)
	}
	w.bert.expect("GET", "/api/album?order=sideways", "", http.StatusBadRequest)
	w.vera.expect("DELETE", order, "", http.StatusForbidden)
	if got := w.bert.expect("DELETE", order, "", http.StatusOK); got["ordered"] != false {
		t.Fatalf("answer %v", got)
	}
	if got, ordered := names("?order=slides&folder=" + id); ordered || got[0] != "1 Start.png" {
		t.Fatalf("cleared %v %v", got, ordered)
	}
	page := field(w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"Patch","properties":{"priority":"low"}}`, http.StatusCreated), "item", "key").(string)
	w.bert.expect("DELETE", "/api/album/order/"+page, "", http.StatusBadRequest)
}

// TestTheSlidesOfACollectionAreOrderedByHandThroughTheAPI: a collection's
// order names pictures of every room it gathers.
func TestTheSlidesOfACollectionAreOrderedByHandThroughTheAPI(t *testing.T) {
	w := newWorld(t)
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 100)...)
	folder := field(w.bert.expect("POST", "/api/album/folders", `{"space":"PLAT","name":"Talk"}`, http.StatusCreated), "item").(map[string]any)
	id := folder["id"].(string)
	var first []string
	for _, name := range []string{"a", "b"} {
		rec := w.bert.upload("/api/items/"+id+"/attachments?name="+name+".png", "application/octet-stream", png)
		var up struct {
			Attachment struct {
				ID string `json:"id"`
			} `json:"attachment"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &up); err != nil {
			t.Fatal(err)
		}
		w.bert.expect("PUT", "/api/items/"+id+"/attachments/"+up.Attachment.ID+"/labels", `{"labels":["talk"]}`, http.StatusOK)
		first = append(first, up.Attachment.ID)
	}
	coll := field(w.bert.expect("POST", "/api/album/collections", `{"space":"PLAT","name":"Talks","labels":["talk"],"match":"any"}`, http.StatusCreated), "item").(map[string]any)
	cid := coll["id"].(string)
	w.bert.expect("PUT", "/api/album/order/"+cid, `{"pictures":["`+first[0]+`"]}`, http.StatusOK)
	page := w.vera.expect("GET", "/api/album?order=slides&collection="+cid, "", http.StatusOK)
	var got []string
	for _, p := range field(page, "pictures").([]any) {
		got = append(got, p.(map[string]any)["first"].(string))
	}
	if page["ordered"] != true || !slices.Equal(got, []string{first[1], first[0]}) {
		t.Fatalf("collection %v %v", got, page["ordered"])
	}
}
