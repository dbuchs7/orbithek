package server

import (
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
