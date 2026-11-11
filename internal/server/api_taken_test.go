package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/dbuchs7/orbithek/internal/item"
	"github.com/dbuchs7/orbithek/internal/photo"
)

// TestATimeSetByHandThroughTheAPI: an editor sets when a photo was taken;
// the album orders it by that time, and every file given out — a
// download, a ZIP archive — carries it in its EXIF with the file's other
// data, while the stored file stays as it was; going back gives the
// file's own time again; a reader may not set one, nor has a file that is
// no picture one (ADR-0198).
func TestATimeSetByHandThroughTheAPI(t *testing.T) {
	w := newPhotoWorld(t)
	original := exifJPEG(t, "2024:05:01 13:22:10", 46.5584, 7.8352, 64)
	lake := w.put(t, "lake.jpg", original)
	w.put(t, "old.jpg", exifJPEG(t, "2019:05:01 08:00:00", 0, 0, 66))
	id := lake["id"].(string)
	file := "/api/items/" + w.folder + "/attachments/" + id

	got := w.bert.expect("PUT", file+"/taken", `{"taken":"1987-06-05T04:03:02","offset":"+01:00"}`, http.StatusOK)
	p := field(got, "attachment", "photo").(map[string]any)
	if p["taken"] != "1987-06-05T04:03:02" || p["offset"] != "+01:00" || p["takenManual"] != true || p["lat"] == nil {
		t.Errorf("photo data %v", p)
	}
	if order := names(w.vera, "?order=taken"); len(order) != 2 || order[0] != "old.jpg" {
		t.Errorf("the timeline orders %v", order)
	}

	r := w.vera.do("GET", file, "")
	m := photo.Read(bytes.NewReader(r.Body.Bytes()), "image/jpeg")
	if m.Taken != "1987-06-05T04:03:02" || m.Offset != "+01:00" || m.Lat == nil {
		t.Errorf("the download tells %+v", m)
	}
	etag := r.Header().Get("ETag")
	if etag == `"`+lake["sha256"].(string)+`"` {
		t.Error("the download with the time written has the stored file's ETag")
	}
	zr := w.vera.do("POST", "/api/album/zip", `{"pictures":[{"item":"`+w.folder+`","file":"`+id+`"}]}`)
	zf, err := zip.NewReader(bytes.NewReader(zr.Body.Bytes()), int64(zr.Body.Len()))
	if err != nil || len(zf.File) != 1 {
		t.Fatalf("archive: %v", err)
	}
	f, _ := zf.File[0].Open()
	inZip, _ := io.ReadAll(f)
	if m := photo.Read(bytes.NewReader(inZip), "image/jpeg"); m.Taken != "1987-06-05T04:03:02" {
		t.Errorf("the archive tells %+v", m)
	}
	// The stored file is the uploaded one.
	stored, err := w.db.OpenAttachment(item.ID(w.folder), item.ID(id))
	if err != nil {
		t.Fatal(err)
	}
	kept, _ := io.ReadAll(stored)
	stored.Close()
	if !bytes.Equal(kept, original) {
		t.Error("the stored file was changed")
	}

	// Set again, the download is another.
	w.bert.expect("PUT", file+"/taken", `{"taken":"1987-06-06T12:00:00"}`, http.StatusOK)
	r = w.vera.do("GET", file, "")
	if m := photo.Read(bytes.NewReader(r.Body.Bytes()), "image/jpeg"); m.Taken != "1987-06-06T12:00:00" || m.Offset != "" {
		t.Errorf("set again, the download tells %+v", m)
	}
	if r.Header().Get("ETag") == etag {
		t.Error("set again, the download keeps its ETag")
	}

	// Back to the file's own.
	got = w.bert.expect("DELETE", file+"/taken", "", http.StatusOK)
	if p := field(got, "attachment", "photo").(map[string]any); p["taken"] != "2024-05-01T13:22:10" || p["takenManual"] != nil {
		t.Errorf("back to the file's own: %v", p)
	}
	if body := w.vera.do("GET", file, "").Body.Bytes(); !bytes.Equal(body, original) {
		t.Error("back to the file's own, the download is not the stored file")
	}

	w.vera.expect("PUT", file+"/taken", `{"taken":"1987-06-05T04:03:02"}`, http.StatusForbidden)
	w.bert.expect("PUT", file+"/taken", `{"taken":""}`, http.StatusBadRequest)
	w.bert.expect("PUT", file+"/taken", `{"taken":"1987-06-05"}`, http.StatusBadRequest)
	w.bert.expect("PUT", file+"/taken", `{"taken":"1987-06-05T04:03:02","offset":"CET"}`, http.StatusBadRequest)
	txt := w.put(t, "notes.txt", []byte("plain text"))
	w.bert.expect("PUT", "/api/items/"+w.folder+"/attachments/"+txt["id"].(string)+"/taken", `{"taken":"1987-06-05T04:03:02"}`, http.StatusBadRequest)

	// An older picture without recorded data gets them with the time.
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 100)...)
	scan := w.put(t, "scan.png", png)
	got = w.bert.expect("PUT", "/api/items/"+w.folder+"/attachments/"+scan["id"].(string)+"/taken", `{"taken":"1950-12-24T18:00:00"}`, http.StatusOK)
	if p := field(got, "attachment", "photo").(map[string]any); p["taken"] != "1950-12-24T18:00:00" {
		t.Errorf("older picture %v", p)
	}
	// A file whose EXIF cannot be written is given as it is.
	if body := w.vera.do("GET", "/api/items/"+w.folder+"/attachments/"+scan["id"].(string), "").Body.Bytes(); !bytes.Equal(body, png) {
		t.Error("a file the time cannot be written into changed")
	}
}

// TestACopyOfAPictureWithATimeSetByHand: a copy uploaded as it was
// downloaded, with the time written into it, is taken as a copy and
// carries the time set by hand; other content is not; a copy of a whole
// item copies the file as it is given (ADR-0198).
func TestACopyOfAPictureWithATimeSetByHand(t *testing.T) {
	w := newPhotoWorld(t)
	lake := w.put(t, "lake.jpg", exifJPEG(t, "2024:05:01 13:22:10", 0, 0, 64))
	id := lake["id"].(string)
	file := "/api/items/" + w.folder + "/attachments/" + id
	w.bert.expect("PUT", file+"/taken", `{"taken":"1962-07-14T15:30:00","offset":"+01:00"}`, http.StatusOK)

	other := w.admin.expect("POST", "/api/album/folders", `{"space":"PLAT","name":"Mine"}`, http.StatusCreated)
	mine := field(other, "item", "id").(string)
	src := "&sourceItem=" + w.folder + "&sourceAttachment=" + id
	given := w.admin.do("GET", file, "").Body.Bytes()
	r := w.admin.upload("/api/items/"+mine+"/attachments?name=lake.jpg"+src, "application/octet-stream", given)
	var out map[string]any
	_ = json.Unmarshal(r.Body.Bytes(), &out)
	if r.Code != http.StatusCreated || field(out, "attachment", "photo", "takenManual") != true || field(out, "attachment", "photo", "taken") != "1962-07-14T15:30:00" {
		t.Errorf("a copy as given: %d %s", r.Code, r.Body)
	}
	if r := w.admin.upload("/api/items/"+mine+"/attachments?name=lake.jpg"+src, "application/octet-stream", append(given, 0)); r.Code != http.StatusBadRequest {
		t.Errorf("other content as a copy: %d %s", r.Code, r.Body)
	}
}
