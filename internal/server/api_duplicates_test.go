package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/dbuchs7/orbithek/internal/budget"
	"github.com/dbuchs7/orbithek/internal/item"
	"github.com/dbuchs7/orbithek/internal/photo"
)

// landscape is a w×h picture of soft light and shade drawn from seed, so
// that pictures of different seeds look different.
func landscape(w, h int, seed float64) *image.RGBA {
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			fx, fy := float64(x)/float64(w), float64(y)/float64(h)
			v := 128 + 60*math.Sin(seed*3.1*fx+seed) + 50*math.Cos((seed+1.7)*2.3*fy+seed*seed)
			c := uint8(max(0, min(255, v)))
			m.SetRGBA(x, y, color.RGBA{c, c / 2, 255 - c, 255})
		}
	}
	return m
}

func jpegOf(t *testing.T, m image.Image, quality int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, m, &jpeg.Options{Quality: quality}); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func sumOf(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// groupNames are the names of the pictures of each group, sorted.
func groupNames(t *testing.T, c *client, query string) ([][]string, map[string]any) {
	t.Helper()
	out := c.expect("GET", "/api/album/duplicates"+query, "", http.StatusOK)
	var groups [][]string
	for _, g := range out["groups"].([]any) {
		var names []string
		for _, p := range g.(map[string]any)["pictures"].([]any) {
			names = append(names, field(p.(map[string]any), "attachment", "name").(string))
		}
		slices.Sort(names)
		groups = append(groups, names)
	}
	return groups, out
}

// TestTheAlbumFindsDuplicates: the same file in two folders and the same
// picture scaled and encoded again form one group; another picture is in
// none; only pictures one may read are compared (ADR-0196).
func TestTheAlbumFindsDuplicates(t *testing.T) {
	w := newPhotoWorld(t)
	lake := landscape(400, 300, 1)
	orig := jpegOf(t, lake, 92)
	w.put(t, "lake.jpg", orig)
	w.put(t, "forest.jpg", jpegOf(t, landscape(400, 300, 4), 92))
	w.put(t, "lake-small.jpg", jpegOf(t, scaleTo(lake, 200, 150), 60))
	// The same proportions are needed: a picture cut otherwise is none.
	w.put(t, "lake-wide.jpg", jpegOf(t, scaleTo(lake, 400, 200), 92))
	other := w.bert.expect("POST", "/api/album/folders", `{"space":"PLAT","name":"Archive"}`, http.StatusCreated)
	otherID := field(other, "item", "id").(string)
	if r := w.bert.upload("/api/items/"+otherID+"/attachments?name=lake-again.jpg", "application/octet-stream", orig); r.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", r.Code, r.Body)
	}

	groups, out := groupNames(t, w.bert, "")
	if want := [][]string{{"lake-again.jpg", "lake-small.jpg", "lake.jpg"}}; fmt.Sprint(groups) != fmt.Sprint(want) {
		t.Fatalf("groups %v, want %v", groups, want)
	}
	g := out["groups"].([]any)[0].(map[string]any)

	if g["same"] != false || out["pending"] != float64(0) || out["total"] != float64(1) {
		t.Errorf("group %v, pending %v, total %v", g["same"], out["pending"], out["total"])
	}
	for _, p := range g["pictures"].([]any) {
		name, same := field(p.(map[string]any), "attachment", "name").(string), p.(map[string]any)["same"]
		if want := name != "lake-small.jpg"; same != want {
			t.Errorf("%s: same %v, want %v", name, same, want)
		}
	}
	// Whoever reads the room sees the same; whoever reads nothing, nothing.
	if got, _ := groupNames(t, w.vera, ""); len(got) != 1 {
		t.Errorf("vera sees %v", got)
	}
	if got, _ := groupNames(t, w.carla, ""); len(got) != 0 {
		t.Errorf("carla sees %v", got)
	}
	if got, _ := groupNames(t, w.bert, "?space=NONE"); len(got) != 0 {
		t.Errorf("another room shows %v", got)
	}
	// A folder kept from vera takes its picture out of what she compares.
	w.bert.expect("PUT", "/api/items/"+otherID+"/restriction", `{"read":["`+w.ids["bert"]+`"],"edit":[]}`, http.StatusOK)
	got, _ := groupNames(t, w.vera, "")
	if want := [][]string{{"lake-small.jpg", "lake.jpg"}}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("vera sees %v, want %v", got, want)
	}
}

func scaleTo(m image.Image, w, h int) *image.RGBA {
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	b := m.Bounds()
	for y := range h {
		for x := range w {
			out.Set(x, y, m.At(b.Min.X+x*b.Dx()/w, b.Min.Y+y*b.Dy()/h))
		}
	}
	return out
}

// TestAGroupBeginsWithTheOldest: of the same file added twice to a
// folder, also within the same second, the first upload comes first, so
// the browser keeps it when it deletes the duplicates at once.
func TestAGroupBeginsWithTheOldest(t *testing.T) {
	for range 5 {
		w := newPhotoWorld(t)
		data := jpegOf(t, landscape(300, 200, 5), 90)
		w.put(t, "first.jpg", data)
		w.put(t, "second.jpg", data)
		w.put(t, "third.jpg", data)
		_, out := groupNames(t, w.bert, "")
		var order []string
		for _, p := range out["groups"].([]any)[0].(map[string]any)["pictures"].([]any) {
			order = append(order, field(p.(map[string]any), "attachment", "name").(string))
		}
		if fmt.Sprint(order) != "[first.jpg second.jpg third.jpg]" {
			t.Fatalf("group in the order %v, want the uploads' order", order)
		}
	}
}

// TestACopyIsNoDuplicate: a picture copied into another item, which
// remembers its source, is no duplicate of its original (ADR-0055).
func TestACopyIsNoDuplicate(t *testing.T) {
	w := newPhotoWorld(t)
	orig := jpegOf(t, landscape(300, 200, 2), 90)
	a := w.put(t, "lake.jpg", orig)
	page := w.bert.expect("POST", "/api/album/folders", `{"space":"PLAT","name":"Trip"}`, http.StatusCreated)
	r := w.bert.upload("/api/items/"+field(page, "item", "id").(string)+"/attachments?name=lake.jpg&sourceItem="+w.folder+"&sourceAttachment="+a["id"].(string), "application/octet-stream", orig)
	if r.Code != http.StatusCreated {
		t.Fatalf("copy: %d %s", r.Code, r.Body)
	}
	if got, _ := groupNames(t, w.bert, ""); len(got) != 0 {
		t.Errorf("a copy is grouped with its original: %v", got)
	}
}

// TestOlderPicturesAreComparedAsTheyAreRead: a picture stored without a
// fingerprint gets one made, at most FingerprintReads per request; until
// then only the same file finds it, and pending says how many wait.
func TestOlderPicturesAreComparedAsTheyAreRead(t *testing.T) {
	w := newPhotoWorld(t)
	lake := landscape(320, 240, 3)
	w.put(t, "lake.jpg", jpegOf(t, lake, 90))
	var older []item.Attachment
	for i, q := range []int{50, 70} {
		data := jpegOf(t, lake, q)
		a, err := w.db.StoreAttachment(item.ID(w.folder), bytes.NewReader(data), 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		a.Name = fmt.Sprintf("older-%d.jpg", i)
		if err := w.db.Write(func(s *item.Store) error {
			got, err := s.As(w.ids["bert"]).AddAttachment(item.ID(w.folder), a)
			older = append(older, got)
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	if older[0].Fingerprint != "" {
		t.Fatal("the picture stored by hand has a fingerprint")
	}
	b := budget.Get()
	b.FingerprintReads = 1
	budget.Set(b)
	defer budget.Set(budget.Default())
	_, first := groupNames(t, w.bert, "")
	if first["pending"] != float64(1) || len(first["groups"].([]any)) != 1 {
		t.Fatalf("first request: pending %v, groups %v", first["pending"], first["groups"])
	}
	groups, second := groupNames(t, w.bert, "")
	if second["pending"] != float64(0) || fmt.Sprint(groups) != fmt.Sprint([][]string{{"lake.jpg", "older-0.jpg", "older-1.jpg"}}) {
		t.Fatalf("second request: pending %v, groups %v", second["pending"], groups)
	}
	b.FingerprintReads = 0
	budget.Set(b)
	if _, again := groupNames(t, w.bert, ""); again["pending"] != float64(0) {
		t.Errorf("fingerprints made before are forgotten: pending %v", again["pending"])
	}
}

// TestFilesAboutToBeAddedAreChecked: the browser asks before an upload
// which readable pictures are the same file or look the same as the files
// chosen (ADR-0196).
func TestFilesAboutToBeAddedAreChecked(t *testing.T) {
	w := newPhotoWorld(t)
	lake := landscape(400, 300, 1)
	orig := jpegOf(t, lake, 92)
	w.put(t, "lake.jpg", orig)
	var p bytes.Buffer
	if err := png.Encode(&p, scaleTo(lake, 200, 150)); err != nil {
		t.Fatal(err)
	}
	look, err := photo.Fingerprint(bytes.NewReader(p.Bytes()), "image/png", 1, 1<<24)
	if err != nil || look == "" {
		t.Fatal(look, err)
	}
	unknown := strings.Repeat("0", 64)
	body := fmt.Sprintf(`{"files":[{"sha256":%q},{"sha256":%q,"fingerprint":%q,"width":200,"height":150},{"sha256":%q,"fingerprint":%q,"width":100,"height":150},{"sha256":%q}]}`,
		sumOf(orig), sumOf(p.Bytes()), look, unknown, look, unknown)
	out := w.vera.expect("POST", "/api/album/duplicates", body, http.StatusOK)
	m := out["matches"].([]any)
	count := func(i int) (same, similar int) {
		for _, x := range m[i].([]any) {
			if x.(map[string]any)["same"] == true {
				same++
			} else {
				similar++
			}
		}
		return
	}
	if s, l := count(0); s != 1 || l != 0 {
		t.Errorf("the same file: %d same, %d alike", s, l)
	}
	if s, l := count(1); s != 0 || l != 1 {
		t.Errorf("the picture scaled: %d same, %d alike", s, l)
	}
	if s, l := count(2); s+l != 0 {
		t.Errorf("other proportions: %d same, %d alike", s, l)
	}
	if s, l := count(3); s+l != 0 {
		t.Errorf("an unknown file: %d same, %d alike", s, l)
	}
	if got := field(m[0].([]any)[0].(map[string]any), "attachment", "name"); got != "lake.jpg" {
		t.Errorf("match %v", got)
	}
	// Whoever reads nothing finds nothing.
	if got := w.carla.expect("POST", "/api/album/duplicates", body, http.StatusOK)["matches"].([]any); len(got[0].([]any)) != 0 {
		t.Errorf("carla finds %v", got[0])
	}
	for _, bad := range []string{`{"files":[{"sha256":"abc"}]}`, `{"files":[{"sha256":"` + unknown + `","fingerprint":"XYZ"}]}`, `{"files":[{"sha256":"` + unknown + `","width":-1}]}`} {
		w.vera.expect("POST", "/api/album/duplicates", bad, http.StatusBadRequest)
	}
	b := budget.Get()
	b.DuplicateChecks = 2
	budget.Set(b)
	defer budget.Set(budget.Default())
	w.vera.expect("POST", "/api/album/duplicates", body, http.StatusRequestEntityTooLarge)
}

func TestFingerprintPartsMeetWhenNear(t *testing.T) {
	a := "690d69db9bd296b0"
	for _, b := range []string{"690d69db9bd296b0", "690d69db9bd296bf", "e90d69db9bd29630", "f90d69db9bd29631"} {
		if photo.Distance(a, b) > photo.SimilarBits {
			continue
		}
		pa, pb := fingerprintParts(a), fingerprintParts(b)
		if !slices.ContainsFunc(pa, func(x uint64) bool { return slices.Contains(pb, x) }) {
			t.Errorf("%s and %s are %d bits apart but share no part", a, b, photo.Distance(a, b))
		}
	}
	if fingerprintParts("not one") != nil {
		t.Error("parts of no fingerprint")
	}
	if !sameShape(0, 0, 4, 3) || !sameShape(4000, 3000, 400, 300) || sameShape(4, 3, 16, 9) {
		t.Error("sameShape")
	}
}
