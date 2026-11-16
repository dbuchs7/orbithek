package server

import (
	"archive/zip"
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"log/slog"
	"net/http"
	"testing"

	"github.com/dbuchs7/orbithek/internal/photo"
)

// TestAShareEntryIsShownUprightOnce: the preview of an entry of a file
// share is made upright from its original when a run reads it (ADR-0197),
// so its thumbnails are made from that preview as it is and not turned a
// second time by the orientation the original tells.
func TestAShareEntryIsShownUprightOnce(t *testing.T) {
	w, sh, dir, _ := shareWorld(t)
	// A picture of blocks of different grey, so its fingerprint tells which
	// way up it lies.
	stored := image.NewRGBA(image.Rect(0, 0, 240, 120))
	for y := range 120 {
		for x := range 240 {
			v := uint8(((x/20)*7 + (y/20)*13) % 11 * 23) // blocks of strongly different grey
			stored.Set(x, y, color.RGBA{v, v, v, 255})
		}
	}
	var plain bytes.Buffer
	if err := jpeg.Encode(&plain, stored, nil); err != nil {
		t.Fatal(err)
	}
	turned, ok := photo.SetOrientation(plain.Bytes(), "image/jpeg", 6) // stored 240 × 120, shown 120 × 240
	if !ok {
		t.Fatal("no orientation written")
	}
	put(t, dir, "phone.jpg", turned)
	mustRun(t, sh, false, false)
	entry := entriesOf(t, w, "")["phone.jpg"]
	if entry.Photo == nil || entry.Photo.Orientation != 6 {
		t.Fatalf("the entry's photo data %+v", entry.Photo)
	}
	folder, _ := folderOfEntry(t, w, entry.ID)
	for _, edge := range []string{"400", "1600"} {
		r := w.admin.do("GET", "/api/items/"+string(folder)+"/attachments/"+string(entry.ID)+"/thumbnail?edge="+edge, "")
		if r.Code != http.StatusOK {
			t.Fatalf("thumbnail %s: %d %s", edge, r.Code, r.Body)
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(r.Body.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Width != 120 || cfg.Height != 240 {
			t.Errorf("the thumbnail at %s is %d × %d, want 120 × 240 (upright once)", edge, cfg.Width, cfg.Height)
		}
	}

	// A fingerprint made later from the preview is the picture's upright,
	// as the run made it from the original (ADR-0196).
	h := itemHandlers{d: w.deps, log: slog.New(slog.DiscardHandler)}
	want, err := photo.Fingerprint(bytes.NewReader(turned), "image/jpeg", 6, 1<<24)
	if err != nil || want == "" {
		t.Fatalf("the original's fingerprint %q: %v", want, err)
	}
	if got := h.makeFingerprint(context.Background(), folder, entry); photo.Distance(got, want) < 0 || photo.Distance(got, want) > photo.SimilarBits {
		t.Errorf("the fingerprint made from the preview %q is not the original's upright %q", got, want)
	}
}

// TestAShareEntryOfAnotherFormatHasAThumbnail: a run stores the preview of
// an entry as JPEG (PNG when it has transparency), whatever the original's
// format (ADR-0197). Its thumbnail and fingerprint are made from the
// preview as what it is, not read as the original's format, so a PNG, GIF
// or WebP of a share gets a thumbnail of its own size rather than the whole
// preview.
func TestAShareEntryOfAnotherFormatHasAThumbnail(t *testing.T) {
	w, sh, dir, _ := shareWorld(t)
	put(t, dir, "hut.png", picture(t, 800, 400, 20))
	mustRun(t, sh, false, false)
	entry := entriesOf(t, w, "")["hut.png"]
	if entry.Type != "image/png" {
		t.Fatalf("the entry's type %q", entry.Type)
	}
	folder, _ := folderOfEntry(t, w, entry.ID)
	r := w.admin.do("GET", "/api/items/"+string(folder)+"/attachments/"+string(entry.ID)+"/thumbnail?edge=400", "")
	if r.Code != http.StatusOK {
		t.Fatalf("thumbnail: %d, Location %q", r.Code, r.Header().Get("Location"))
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(r.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != 400 || cfg.Height != 200 {
		t.Errorf("the thumbnail is %d × %d, want 400 × 200", cfg.Width, cfg.Height)
	}
	h := itemHandlers{d: w.deps, log: slog.New(slog.DiscardHandler)}
	if fp := h.makeFingerprint(context.Background(), folder, entry); fp == "" {
		t.Error("no fingerprint is made from the preview of a PNG")
	}
}

// TestAShareEntryIsGivenAsWhatItHolds: what leaves the server of an entry
// of a file share is its preview (ADR-0197), so a download tells the
// preview's type and a file name with its extension, and so does its name
// in a ZIP archive; an entry whose original is a JPEG keeps its name.
func TestAShareEntryIsGivenAsWhatItHolds(t *testing.T) {
	w, sh, dir, _ := shareWorld(t)
	put(t, dir, "hut.png", picture(t, 80, 40, 20))
	var lake bytes.Buffer
	if err := jpeg.Encode(&lake, image.NewRGBA(image.Rect(0, 0, 80, 40)), nil); err != nil {
		t.Fatal(err)
	}
	put(t, dir, "lake.jpeg", lake.Bytes())
	mustRun(t, sh, false, false)
	entries := entriesOf(t, w, "")
	for _, c := range []struct{ path, typ, name string }{
		{"hut.png", "image/jpeg", "hut.jpg"},
		{"lake.jpeg", "image/jpeg", "lake.jpeg"},
	} {
		e := entries[c.path]
		folder, _ := folderOfEntry(t, w, e.ID)
		r := w.admin.do("GET", "/api/items/"+string(folder)+"/attachments/"+string(e.ID), "")
		if r.Code != http.StatusOK {
			t.Fatalf("%s: download %d", c.path, r.Code)
		}
		if got := r.Header().Get("Content-Type"); got != c.typ {
			t.Errorf("%s: the download says %q, want %q", c.path, got, c.typ)
		}
		if got := r.Header().Get("Content-Disposition"); got != `inline; filename=`+c.name {
			t.Errorf("%s: the download is named %q, want %q", c.path, got, c.name)
		}
		z := w.admin.do("POST", "/api/album/zip", `{"pictures":[{"item":"`+string(folder)+`","file":"`+string(e.ID)+`"}]}`)
		zr, err := zip.NewReader(bytes.NewReader(z.Body.Bytes()), int64(z.Body.Len()))
		if err != nil || len(zr.File) != 1 {
			t.Fatalf("%s: archive %v", c.path, err)
		}
		if zr.File[0].Name != c.name {
			t.Errorf("%s: the archive names it %q, want %q", c.path, zr.File[0].Name, c.name)
		}
	}
}
