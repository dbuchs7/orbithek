package item

import (
	"errors"
	"strings"
	"testing"
)

const (
	nasRun = "sync:share:nas"
	lakeID = "0123456789abcdef0123456789abc197"
	hutID  = "0123456789abcdef0123456789abd197"
)

func linked(id, name, path, sum string) Attachment {
	return Attachment{ID: ID(id), Name: name, Type: "image/jpeg", Size: 3, SHA256: strings.Repeat(sum, 32), Share: &ShareFile{Share: "nas", Path: path, Tag: "t1"}}
}

func refusedAsInvalid(err error) bool { return errors.Is(err, ErrValidation) }

// TestLinkedEntriesFollowTheirShare: a share's run links files into a
// folder, reads them again, withdraws and restores them; what people set
// on an entry stays through all of it, and a withdrawn entry is not in the
// album (ADR-0197).
func TestLinkedEntriesFollowTheirShare(t *testing.T) {
	s := setup(t)
	noErr(t, s.GrantRole("admin", "PLAT", "anna", Editor))
	anna := s.As("anna")
	folder := mustItem(t)(anna.CreateFolder("PLAT", "", "NAS"))

	a := linked(lakeID, "lake.jpg", "2024/lake.jpg", "a1")
	a.Photo = &Photo{Taken: "2024-07-01T10:00:00"}
	got, err := s.LinkFile(nasRun, folder.ID, a)
	if err != nil || got.Author != nasRun || !got.Linked() || got.Withdrawn() || got.Share.Path != "2024/lake.jpg" {
		t.Fatalf("linked %+v %v", got, err)
	}
	if _, err := s.LinkFile(nasRun, folder.ID, a); !refusedAsInvalid(err) {
		t.Errorf("linked twice: %v", err)
	}
	noErr(t, anna.SetPictureLabels(folder.ID, lakeID, []string{"ferien"}))
	lat, lon := 46.5, 8.9
	noErr(t, anna.SetPlace(folder.ID, lakeID, &lat, &lon, "Tessin", nil))
	noErr(t, anna.SetTaken(folder.ID, lakeID, "1999-01-02T03:04:05", "", nil))

	b := linked(lakeID, "lake.jpg", "2024/Ferien/lake.jpg", "c3")
	b.Size, b.Photo = 9, &Photo{Taken: "2024-07-02T11:00:00", Make: "Camera"}
	got, err = s.RelinkFile(nasRun, folder.ID, b)
	if err != nil {
		t.Fatal(err)
	}
	if got.Size != 9 || got.SHA256 != strings.Repeat("c3", 32) || got.Share.Path != "2024/Ferien/lake.jpg" ||
		len(got.Labels) != 1 || got.Photo.Make != "Camera" || !got.Photo.PlaceManual || got.Photo.PlaceName != "Tessin" ||
		!got.Photo.TakenManual || got.Photo.Taken != "1999-01-02T03:04:05" {
		t.Fatalf("read again %+v %+v", got, got.Photo)
	}
	other := b
	other.Share = &ShareFile{Share: "bucket", Path: "x.jpg"}
	if _, err := s.RelinkFile(nasRun, folder.ID, other); !refusedAsInvalid(err) {
		t.Errorf("another share: %v", err)
	}

	pictures := func() int { return len(anna.Pictures("", func(Attachment) bool { return true })) }
	if pictures() != 1 {
		t.Fatalf("pictures before withdrawal: %d", pictures())
	}
	noErr(t, s.WithdrawFile(nasRun, folder.ID, lakeID))
	if err := s.WithdrawFile(nasRun, folder.ID, lakeID); !refusedAsInvalid(err) {
		t.Errorf("withdrawn twice: %v", err)
	}
	if pictures() != 0 {
		t.Error("a withdrawn entry is in the album")
	}
	files, _ := anna.Attachments(folder.ID)
	if len(files) != 1 || !files[0].Withdrawn() || len(files[0].Labels) != 1 {
		t.Fatalf("withdrawn %+v", files)
	}
	if err := s.RestoreFile(nasRun, folder.ID, lakeID, ShareFile{Path: "../x"}); !refusedAsInvalid(err) {
		t.Errorf("restored at a bad path: %v", err)
	}
	noErr(t, s.RestoreFile(nasRun, folder.ID, lakeID, ShareFile{Share: "ignored", Path: "Archiv/lake.jpg", Tag: "t2"}))
	if err := s.RestoreFile(nasRun, folder.ID, lakeID, ShareFile{Path: "a.jpg"}); !refusedAsInvalid(err) {
		t.Errorf("restored twice: %v", err)
	}
	files, _ = anna.Attachments(folder.ID)
	if f := files[0].Share; f.Share != "nas" || f.Path != "Archiv/lake.jpg" || f.Tag != "t2" || f.Withdrawn != 0 || pictures() != 1 {
		t.Fatalf("restored %+v", f)
	}

	// The log rebuilds the same state, the time of a withdrawal included.
	noErr(t, s.WithdrawFile(nasRun, folder.ID, lakeID))
	again, err := Replay(s.Log())
	noErr(t, err)
	it, _ := again.Get(folder.ID)
	now, _ := s.Get(folder.ID)
	if w := it.Attachments[0].Share.Withdrawn; w == 0 || w != now.Attachments[0].Share.Withdrawn {
		t.Errorf("replayed withdrawal %d", w)
	}

	// Ordinary files are no entries of a share.
	if _, err := s.RelinkFile(nasRun, folder.ID, linked(hutID, "hut.jpg", "hut.jpg", "b2")); err == nil {
		t.Error("an unknown entry was read again")
	}
	if err := s.WithdrawFile(nasRun, "nope", lakeID); err == nil {
		t.Error("an entry of a missing item was withdrawn")
	}
}

func TestLinkFileChecksWhatItRecords(t *testing.T) {
	s := setup(t)
	noErr(t, s.GrantRole("admin", "PLAT", "anna", Editor))
	folder := mustItem(t)(s.As("anna").CreateFolder("PLAT", "", "NAS"))
	for name, change := range map[string]func(*Attachment){
		"no share":    func(a *Attachment) { a.Share = nil },
		"share ID":    func(a *Attachment) { a.Share.Share = "Not OK" },
		"empty path":  func(a *Attachment) { a.Share.Path = "" },
		"escape":      func(a *Attachment) { a.Share.Path = "a/../b" },
		"absolute":    func(a *Attachment) { a.Share.Path = "/etc/passwd" },
		"backslash":   func(a *Attachment) { a.Share.Path = `a\b` },
		"control":     func(a *Attachment) { a.Share.Path = "a\tb" },
		"long path":   func(a *Attachment) { a.Share.Path = strings.Repeat("a", maxSharePath+1) },
		"long tag":    func(a *Attachment) { a.Share.Tag = strings.Repeat("t", maxShareTag+1) },
		"tag control": func(a *Attachment) { a.Share.Tag = "a\nb" },
		"id":          func(a *Attachment) { a.ID = "x" },
		"hash":        func(a *Attachment) { a.SHA256 = "nope" },
		"name":        func(a *Attachment) { a.Name = "a/b" },
		"photo":       func(a *Attachment) { a.Photo = &Photo{Taken: "yesterday"} },
		"fingerprint": func(a *Attachment) { a.Fingerprint = "xyz" },
	} {
		a := linked(lakeID, "lake.jpg", "lake.jpg", "a1")
		change(&a)
		if _, err := s.LinkFile(nasRun, folder.ID, a); !refusedAsInvalid(err) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := s.LinkFile(nasRun, "nope", linked(lakeID, "lake.jpg", "lake.jpg", "a1")); err == nil {
		t.Error("linked into a missing item")
	}
}

// TestPeopleDoNotChangeASharesFiles: people may not delete, rename, move
// or version an entry of a share; a copy elsewhere is its preview, an
// ordinary file (ADR-0197).
func TestPeopleDoNotChangeASharesFiles(t *testing.T) {
	s := setup(t)
	noErr(t, s.GrantRole("admin", "PLAT", "anna", Editor))
	anna := s.As("anna")
	folder := mustItem(t)(anna.CreateFolder("PLAT", "", "NAS"))
	other := mustItem(t)(anna.CreateFolder("PLAT", "", "Mine"))
	_, err := s.LinkFile(nasRun, folder.ID, linked(lakeID, "lake.jpg", "lake.jpg", "a1"))
	noErr(t, err)

	refused := func(what string, err error) {
		t.Helper()
		if !refusedAsInvalid(err) || !strings.Contains(err.Error(), "file share nas") {
			t.Errorf("%s: %v", what, err)
		}
	}
	refused("delete", anna.DeleteAttachment(folder.ID, lakeID))
	refused("rename", anna.RenameFile(folder.ID, lakeID, "see"))
	_, _, err = anna.PlanMove(folder.ID, lakeID, other.ID)
	refused("move", err)
	_, err = anna.AddAttachment(folder.ID, Attachment{ID: hutID, Name: "lake.jpg", Type: "image/jpeg", Size: 3, SHA256: strings.Repeat("d4", 32),
		Source: &Source{Item: folder.ID, Attachment: lakeID}})
	refused("new version", err)

	copyOf := Attachment{ID: hutID, Name: "lake.jpg", Type: "image/jpeg", Size: 2, SHA256: strings.Repeat("e5", 32), Source: &Source{Item: folder.ID, Attachment: lakeID}}
	if _, err := anna.AddAttachment(other.ID, copyOf); !refusedAsInvalid(err) {
		t.Errorf("a copy without the preview: %v", err)
	}
	copyOf.FromPreview, copyOf.Share = true, &ShareFile{Share: "nas", Path: "lake.jpg"}
	got, err := anna.AddAttachment(other.ID, copyOf)
	if err != nil || got.Linked() || got.Source == nil || got.SHA256 != strings.Repeat("e5", 32) {
		t.Fatalf("copy %+v %v", got, err)
	}
	ordinary := Attachment{ID: "0123456789abcdef0123456789abe197", Name: "x.jpg", Type: "image/jpeg", Size: 2, SHA256: got.SHA256, FromPreview: true, Source: &Source{Item: other.ID, Attachment: hutID}}
	if _, err := anna.AddAttachment(folder.ID, ordinary); !refusedAsInvalid(err) {
		t.Errorf("a preview of an ordinary file: %v", err)
	}

	// What people set on an entry, they still set.
	noErr(t, anna.SetPictureLabels(folder.ID, lakeID, []string{"see"}))
	// The run deletes, as for an ordinary file.
	noErr(t, s.DeleteAttachment(nasRun, folder.ID, lakeID))
}

func TestRelinkedPhoto(t *testing.T) {
	lat := 1.0
	read := &Photo{Taken: "2024-01-01T00:00:00", Lat: &lat, Lon: &lat}
	if relinkedPhoto(nil, read) != read {
		t.Error("nothing set by hand: what the file tells")
	}
	if relinkedPhoto(&Photo{Taken: "x"}, nil) != nil {
		t.Error("a file that tells nothing any more")
	}
	if p := relinkedPhoto(&Photo{PlacePrivate: true}, nil); p == nil || !p.PlacePrivate {
		t.Errorf("a private place stays private: %+v", p)
	}
	if p := relinkedPhoto(&Photo{PlacePrivate: true}, read); !p.PlacePrivate || *p.Lat != 1 {
		t.Errorf("privacy kept with the file's place: %+v", p)
	}
}

// TestALinkedFileMovesToTheFolderOfItsDirectory: a run moves an entry to
// the folder it made for the entry's directory; the entry keeps its
// identity and what people set, and the log rebuilds it (ADR-0197).
func TestALinkedFileMovesToTheFolderOfItsDirectory(t *testing.T) {
	s := setup(t)
	noErr(t, s.GrantRole("admin", "PLAT", "anna", Editor))
	anna := s.As("anna")
	folder := mustItem(t)(anna.CreateFolder("PLAT", "", "NAS"))
	page := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "A page"}))
	_, err := s.LinkFile(nasRun, folder.ID, linked(lakeID, "lake.jpg", "2024/lake.jpg", "a1"))
	noErr(t, err)
	noErr(t, anna.SetPictureLabels(folder.ID, lakeID, []string{"see"}))
	dir, _, err := s.Upsert(nasRun, Upsert{Origin: Origin{System: "share", Instance: "nas", ExternalID: "2024"}, Space: "PLAT", Type: FolderType, Parent: folder.ID, Title: "2024"})
	noErr(t, err)
	if err := s.MoveLinkedFile(nasRun, folder.ID, lakeID, page.ID); !refusedAsInvalid(err) {
		t.Errorf("into a page: %v", err)
	}
	if err := s.MoveLinkedFile(nasRun, folder.ID, lakeID, folder.ID); !refusedAsInvalid(err) {
		t.Errorf("into its own folder: %v", err)
	}
	if err := s.MoveLinkedFile(nasRun, folder.ID, lakeID, "nope"); err == nil {
		t.Error("into a missing folder")
	}
	noErr(t, s.MoveLinkedFile(nasRun, folder.ID, lakeID, dir.ID))
	moved, _ := s.Get(dir.ID)
	if len(moved.Attachments) != 1 || moved.Attachments[0].Share.Path != "2024/lake.jpg" || len(moved.Attachments[0].Labels) != 1 || moved.Attachments[0].Author != nasRun {
		t.Fatalf("moved %+v", moved.Attachments)
	}
	if left, _ := s.Get(folder.ID); len(left.Attachments) != 0 {
		t.Fatalf("left behind %+v", left.Attachments)
	}
	again, err := Replay(s.Log())
	noErr(t, err)
	if it, _ := again.Get(dir.ID); len(it.Attachments) != 1 || !it.Attachments[0].Linked() {
		t.Fatalf("replayed %+v", it.Attachments)
	}
	noErr(t, s.WithdrawFile(nasRun, dir.ID, lakeID))
	if err := s.MoveLinkedFile(nasRun, dir.ID, lakeID, folder.ID); !refusedAsInvalid(err) {
		t.Errorf("a withdrawn entry moved: %v", err)
	}
}
