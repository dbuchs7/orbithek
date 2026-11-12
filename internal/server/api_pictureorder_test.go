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

// TestOrderedSaysOnlyWhatTheCallerReads: a listing says it is ordered by
// hand only when a picture it lists has a place in the order. An order
// that names only pictures the caller does not read, or files that are
// gone, tells them nothing (D10, ADR-0195).
func TestOrderedSaysOnlyWhatTheCallerReads(t *testing.T) {
	w := newWorld(t)
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 100)...)
	upload := func(folder, name string) string {
		t.Helper()
		rec := w.bert.upload("/api/items/"+folder+"/attachments?name="+url.QueryEscape(name), "application/octet-stream", png)
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
		w.bert.expect("PUT", "/api/items/"+folder+"/attachments/"+up.Attachment.ID+"/labels", `{"labels":["talk"]}`, http.StatusOK)
		return up.Attachment.ID
	}
	open := field(w.bert.expect("POST", "/api/album/folders", `{"space":"PLAT","name":"Open"}`, http.StatusCreated), "item", "id").(string)
	closed := field(w.bert.expect("POST", "/api/album/folders", `{"space":"PLAT","name":"Closed"}`, http.StatusCreated), "item", "id").(string)
	w.bert.expect("PUT", "/api/items/"+closed+"/restriction", `{"read":["`+w.ids["bert"]+`"],"edit":[]}`, http.StatusOK)
	// The order is set while the secret picture is the only one, so it
	// names nothing else.
	secret := upload(closed, "secret.png")
	coll := field(w.bert.expect("POST", "/api/album/collections", `{"space":"PLAT","name":"Talks","labels":["talk"],"match":"any"}`, http.StatusCreated), "item", "id").(string)
	w.bert.expect("PUT", "/api/album/order/"+coll, `{"pictures":["`+secret+`"]}`, http.StatusOK)
	upload(open, "a.png")
	listing := "/api/album?order=slides&collection=" + coll
	if page := w.bert.expect("GET", listing, "", http.StatusOK); page["ordered"] != true {
		t.Fatalf("bert reads the placed picture: ordered %v", page["ordered"])
	}
	if page := w.vera.expect("GET", listing, "", http.StatusOK); page["ordered"] != nil {
		t.Fatalf("vera reads no placed picture, yet the listing says ordered: %v", page["ordered"])
	}

	// A placed picture that is deleted leaves the order naming it until
	// the next change; the listing does not count it.
	talk := field(w.bert.expect("POST", "/api/album/folders", `{"space":"PLAT","name":"Talk"}`, http.StatusCreated), "item", "id").(string)
	gone := upload(talk, "gone.png")
	w.bert.expect("PUT", "/api/album/order/"+talk, `{"pictures":["`+gone+`"]}`, http.StatusOK)
	upload(talk, "kept.png")
	w.bert.expect("DELETE", "/api/items/"+talk+"/attachments/"+gone, "", http.StatusOK)
	if page := w.bert.expect("GET", "/api/album?order=slides&folder="+talk, "", http.StatusOK); page["ordered"] != nil {
		t.Fatalf("only a deleted picture is placed, yet the listing says ordered: %v", page["ordered"])
	}
}
