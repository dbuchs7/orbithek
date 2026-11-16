package server

import (
	"bytes"
	"encoding/json"
	"image"
	_ "image/jpeg"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/dbuchs7/orbithek/internal/item"
	"github.com/dbuchs7/orbithek/internal/photo"
)

// thumbSize decodes a thumbnail and answers its size.
func thumbSize(t *testing.T, body []byte) (int, int) {
	t.Helper()
	cfg, _, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("the thumbnail does not decode: %v", err)
	}
	return cfg.Width, cfg.Height
}

// TestAPictureTurnedThroughTheAPI: an editor turns a picture by a quarter
// turn; its thumbnail is made turned under another ETag, and every file
// given out tells the orientation in its EXIF, while the stored file stays
// as it was; turned back, the download is the stored file again; a reader
// may not turn it, nor is a file that is no picture turned
// (ADR-draft-pictures-turned-by-hand).
func TestAPictureTurnedThroughTheAPI(t *testing.T) {
	w := newPhotoWorld(t)
	original := exifJPEG(t, "2024:05:01 13:22:10", 0, 0, 64) // 64 × 32
	lake := w.put(t, "lake.jpg", original)
	id := lake["id"].(string)
	file := "/api/items/" + w.folder + "/attachments/" + id

	before := w.vera.do("GET", file+"/thumbnail?edge=400", "")
	if wd, ht := thumbSize(t, before.Body.Bytes()); wd != 64 || ht != 32 {
		t.Fatalf("the thumbnail before is %d × %d", wd, ht)
	}

	got := w.bert.expect("POST", file+"/turn", `{"by":1}`, http.StatusOK)
	p := field(got, "attachment", "photo").(map[string]any)
	if p["orientation"] != float64(6) || p["orientationManual"] != true || p["taken"] != "2024-05-01T13:22:10" {
		t.Errorf("photo data %v", p)
	}
	after := w.vera.do("GET", file+"/thumbnail?edge=400&o=6", "")
	if wd, ht := thumbSize(t, after.Body.Bytes()); wd != 32 || ht != 64 {
		t.Errorf("the turned thumbnail is %d × %d", wd, ht)
	}
	if after.Header().Get("ETag") == before.Header().Get("ETag") {
		t.Error("the turned thumbnail keeps the ETag of the former")
	}

	r := w.vera.do("GET", file, "")
	if m := photo.Read(bytes.NewReader(r.Body.Bytes()), "image/jpeg"); m.Orientation != 6 || m.Taken != "2024-05-01T13:22:10" {
		t.Errorf("the download tells %+v", m)
	}
	if r.Header().Get("ETag") == `"`+lake["sha256"].(string)+`"` {
		t.Error("the turned download has the stored file's ETag")
	}
	// With a time set by hand as well, the file given out tells both.
	w.bert.expect("PUT", file+"/taken", `{"taken":"1987-06-05T04:03:02"}`, http.StatusOK)
	if m := photo.Read(bytes.NewReader(w.vera.do("GET", file, "").Body.Bytes()), "image/jpeg"); m.Orientation != 6 || m.Taken != "1987-06-05T04:03:02" {
		t.Errorf("with a time set by hand the download tells %+v", m)
	}
	w.bert.expect("DELETE", file+"/taken", "", http.StatusOK)

	stored, err := w.db.OpenAttachment(item.ID(w.folder), item.ID(id))
	if err != nil {
		t.Fatal(err)
	}
	kept, _ := io.ReadAll(stored)
	stored.Close()
	if !bytes.Equal(kept, original) {
		t.Error("the stored file was changed")
	}

	// Turned back, it is no longer turned by hand and given as stored.
	got = w.bert.expect("POST", file+"/turn", `{"by":-1}`, http.StatusOK)
	if p := field(got, "attachment", "photo").(map[string]any); p["orientationManual"] != nil || p["orientation"] != nil {
		t.Errorf("turned back: %v", p)
	}
	if body := w.vera.do("GET", file, "").Body.Bytes(); !bytes.Equal(body, original) {
		t.Error("turned back, the download is not the stored file")
	}

	w.vera.expect("POST", file+"/turn", `{"by":1}`, http.StatusForbidden)
	w.bert.expect("POST", file+"/turn", `{"by":3}`, http.StatusBadRequest)
	w.bert.expect("POST", file+"/turn", `{}`, http.StatusBadRequest)
	txt := w.put(t, "notes.txt", []byte("plain text"))
	w.bert.expect("POST", "/api/items/"+w.folder+"/attachments/"+txt["id"].(string)+"/turn", `{"by":1}`, http.StatusBadRequest)
}

// TestACopyOfAPictureTurnedByHand: a copy uploaded as it was downloaded,
// with the orientation written into it, is taken as a copy and is shown
// turned as its original is (ADR-draft-pictures-turned-by-hand).
func TestACopyOfAPictureTurnedByHand(t *testing.T) {
	w := newPhotoWorld(t)
	lake := w.put(t, "lake.jpg", exifJPEG(t, "2024:05:01 13:22:10", 0, 0, 64))
	id := lake["id"].(string)
	file := "/api/items/" + w.folder + "/attachments/" + id
	w.bert.expect("POST", file+"/turn", `{"by":2}`, http.StatusOK)

	other := w.admin.expect("POST", "/api/album/folders", `{"space":"PLAT","name":"Mine"}`, http.StatusCreated)
	mine := field(other, "item", "id").(string)
	src := "&sourceItem=" + w.folder + "&sourceAttachment=" + id
	given := w.admin.do("GET", file, "").Body.Bytes()
	r := w.admin.upload("/api/items/"+mine+"/attachments?name=lake.jpg"+src, "application/octet-stream", given)
	var out map[string]any
	_ = json.Unmarshal(r.Body.Bytes(), &out)
	if r.Code != http.StatusCreated || field(out, "attachment", "photo", "orientationManual") != true || field(out, "attachment", "photo", "orientation") != float64(3) {
		t.Errorf("a copy as given: %d %s", r.Code, r.Body)
	}
	if r := w.admin.upload("/api/items/"+mine+"/attachments?name=lake.jpg"+src, "application/octet-stream", append(given, 0)); r.Code != http.StatusBadRequest {
		t.Errorf("other content as a copy: %d %s", r.Code, strings.TrimSpace(r.Body.String()))
	}
}
