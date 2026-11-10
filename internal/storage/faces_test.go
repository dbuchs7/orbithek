package storage

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/dbuchs7/orbithek/internal/item"
)

func vector(seed float32) []float32 {
	v := make([]float32, FaceVectorLen)
	for i := range v {
		v[i] = seed + float32(i)/1000
	}
	return v
}

// What was computed of the faces of a picture is sealed beside it, kept
// while it is, moved and removed with it, never backed up, and forgotten
// as a whole (ADR-0194).
func TestFaceVectorsBesideTheirPicture(t *testing.T) {
	dir := t.TempDir()
	db, from := pageIn(t, dir)
	var to item.ID
	if err := db.Write(func(s *item.Store) error {
		if err := s.GrantRole("admin", "PLAT", "admin", item.Editor); err != nil {
			return err
		}
		it, err := s.Create("admin", item.NewItem{Space: "PLAT", Type: "page", Title: "Archive"})
		to = it.ID
		return err
	}); err != nil {
		t.Fatal(err)
	}
	pic := attach(t, db, from, []byte("a picture"), "lake.png")
	if _, err := db.FaceVectors(from, pic.ID); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("vectors before any: %v", err)
	}
	if err := db.StoreFaceVectors(from, pic.ID, [][]float32{vector(0.25), vector(-0.5)}); err != nil {
		t.Fatal(err)
	}
	if err := db.StoreFaceVectors(from, pic.ID, [][]float32{{1, 2}}); err == nil {
		t.Error("a vector of the wrong length was kept")
	}
	got, err := db.FaceVectors(from, pic.ID)
	if err != nil || len(got) != 2 || got[0][1] != 0.251 || got[1][0] != -0.5 {
		t.Fatalf("vectors %v %v", got, err)
	}
	raw, _ := os.ReadFile(attachmentPath(dir, from, pic.ID+FacesSuffix))
	if bytes.Contains(raw, []byte("0.251")) {
		t.Fatal("the vectors are stored in plaintext")
	}
	files, err := db.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f.Name, FacesSuffix) {
			t.Error("face vectors are backed up")
		}
	}
	// A restart keeps them: they belong to a recorded picture.
	db.Close()
	db, rec, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Orphans != 0 {
		t.Errorf("removed %d files at the start", rec.Orphans)
	}
	// They move with their picture.
	if err := db.MoveAttachmentContent(from, to, pic); err != nil {
		t.Fatal(err)
	}
	if err := db.Write(func(s *item.Store) error { return s.As("admin").MoveFile(from, pic.ID, to, []item.ID{pic.ID}) }); err != nil {
		t.Fatal(err)
	}
	if got, err := db.FaceVectors(to, pic.ID); err != nil || len(got) != 2 {
		t.Fatalf("vectors after the move %v %v", got, err)
	}
	if _, err := os.Stat(attachmentPath(dir, from, pic.ID+FacesSuffix)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the vectors stayed behind: %v", err)
	}
	// Forgetting removes every one; storing none removes them too.
	other := attach(t, db, to, []byte("another"), "other.png")
	if err := db.StoreFaceVectors(to, other.ID, [][]float32{vector(1)}); err != nil {
		t.Fatal(err)
	}
	if n, err := db.ForgetFaceVectors(); err != nil || n != 2 {
		t.Fatalf("forgot %d, %v", n, err)
	}
	if _, err := db.FaceVectors(to, pic.ID); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("vectors after forgetting: %v", err)
	}
	if err := db.StoreFaceVectors(to, other.ID, [][]float32{vector(1)}); err != nil {
		t.Fatal(err)
	}
	if err := db.StoreFaceVectors(to, other.ID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := db.FaceVectors(to, other.ID); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("vectors after storing none: %v", err)
	}
	// They go with their picture.
	if err := db.StoreFaceVectors(to, other.ID, [][]float32{vector(1)}); err != nil {
		t.Fatal(err)
	}
	if err := db.Write(func(s *item.Store) error { return s.DeleteAttachment("admin", to, other.ID) }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(attachmentPath(dir, to, other.ID+FacesSuffix)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the vectors of a deleted picture stay: %v", err)
	}
	db.Close()
}
