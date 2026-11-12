package site

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

const folderA = "0123456789abcdef0123456789abcdef"

// TestSharesBindFoldersToFileShares: administrators add, change and
// remove shares; each folder shows one share, a share keeps its folder,
// and the journal gives them back (ADR-0197).
func TestSharesBindFoldersToFileShares(t *testing.T) {
	var records [][]byte
	s, err := New(nil, func(r []byte) error { records = append(records, r); return nil })
	if err != nil {
		t.Fatal(err)
	}
	nas, err := s.SetShare("admin", Share{ID: " nas ", Name: " NAS ", Address: "/mnt/nas/photos", Folder: folderA, Link: "smb://nas/photos/{path}", Device: 42})
	if err != nil {
		t.Fatal(err)
	}
	if nas.ID != "nas" || nas.Name != "NAS" || nas.IntervalHours != ShareIntervalDefault || nas.Kind() != "dir" || nas.Device != 42 {
		t.Fatalf("kept %+v", nas)
	}
	bucket, err := s.SetShare("admin", Share{ID: "bucket", Name: "Bucket", Address: "s3://photos?endpoint=https://s3.example.com", Folder: strings.Repeat("b", 32), IntervalHours: 24, Device: 7})
	if err != nil || bucket.Kind() != "s3" || bucket.Device != 0 {
		t.Fatalf("s3: %+v %v", bucket, err)
	}
	if _, err := s.SetShare("admin", Share{ID: "other", Name: "Other", Address: "/mnt/nas/other", Folder: folderA}); !errors.Is(err, ErrInvalid) {
		t.Errorf("a second share in a folder: %v", err)
	}
	if _, err := s.SetShare("admin", Share{ID: "nas", Name: "NAS", Address: "/mnt/nas/photos", Folder: strings.Repeat("c", 32)}); !errors.Is(err, ErrInvalid) {
		t.Errorf("a share moved to another folder: %v", err)
	}
	nas.Enabled = true
	if _, err := s.SetShare("admin", nas); err != nil {
		t.Fatal(err)
	}
	if got := s.Shares(); len(got) != 2 || got[0].ID != "bucket" || got[1].ID != "nas" || !got[1].Enabled {
		t.Fatalf("shares %+v", got)
	}
	if sh, ok := s.ShareOfFolder(folderA); !ok || sh.ID != "nas" {
		t.Errorf("share of the folder: %+v %v", sh, ok)
	}
	if _, ok := s.ShareOfFolder(strings.Repeat("d", 32)); ok {
		t.Error("a share of a folder that has none")
	}
	if err := s.RemoveShare("admin", "bucket"); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveShare("admin", "bucket"); !errors.Is(err, ErrInvalid) {
		t.Errorf("removed twice: %v", err)
	}
	again, err := New(records, func([]byte) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if sh, ok := again.Share("nas"); !ok || !sh.Enabled || sh.Device != 42 || sh.Link != "smb://nas/photos/{path}" {
		t.Fatalf("read again: %+v %v", sh, ok)
	}
	if _, ok := again.Share("bucket"); ok {
		t.Error("a removed share came back")
	}
	if _, err := New([][]byte{[]byte(`{"v":1,"kind":"ShareSet"}`)}, nil); err == nil {
		t.Fatal("a share event without a share")
	}
}

func TestShareValidation(t *testing.T) {
	ok := Share{ID: "nas", Name: "NAS", Address: "/mnt/nas", Folder: folderA, IntervalHours: 6}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*Share){
		"id":       func(s *Share) { s.ID = "Not OK" },
		"name":     func(s *Share) { s.Name = "" },
		"long":     func(s *Share) { s.Name = strings.Repeat("x", 81) },
		"markup":   func(s *Share) { s.Name = "<b>" },
		"address":  func(s *Share) { s.Address = "relative" },
		"folder":   func(s *Share) { s.Folder = "a/b" },
		"link":     func(s *Share) { s.Link = "file:///mnt/{path}" },
		"too fast": func(s *Share) { s.IntervalHours = 0 },
		"too slow": func(s *Share) { s.IntervalHours = ShareIntervalMax + 1 },
	} {
		s := ok
		change(&s)
		if err := s.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if (Share{Address: "nowhere"}).Kind() != "" {
		t.Error("a kind for a bad address")
	}
}

func TestSharesAreBounded(t *testing.T) {
	s, _ := New(nil, func([]byte) error { return nil })
	for i := range maxShares {
		if _, err := s.SetShare("admin", Share{ID: fmt.Sprintf("s%d", i), Name: "S", Address: "/mnt/s", Folder: fmt.Sprintf("%032x", i)}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.SetShare("admin", Share{ID: "one-more", Name: "S", Address: "/mnt/s", Folder: strings.Repeat("e", 32)}); !errors.Is(err, ErrInvalid) {
		t.Errorf("beyond the bound: %v", err)
	}
	if _, err := s.SetShare("admin", Share{ID: "s0", Name: "Renamed", Address: "/mnt/s", Folder: fmt.Sprintf("%032x", 0)}); err != nil {
		t.Errorf("changing one at the bound: %v", err)
	}
}

func TestOnlyAnS3ShareSignsLinks(t *testing.T) {
	s, _ := New(nil, func([]byte) error { return nil })
	if _, err := s.SetShare("admin", Share{ID: "nas", Name: "NAS", Address: "/mnt/nas", Folder: folderA, SignedLinks: true}); !errors.Is(err, ErrInvalid) {
		t.Errorf("a directory signs links: %v", err)
	}
	sh, err := s.SetShare("admin", Share{ID: "bucket", Name: "Bucket", Address: "s3://photos?endpoint=https://s3.example.com", Folder: folderA, SignedLinks: true})
	if err != nil || !sh.SignedLinks {
		t.Fatalf("an S3 share: %+v %v", sh, err)
	}
}
