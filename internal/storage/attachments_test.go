package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dbuchs7/orbithek/internal/diskspace"
	"github.com/dbuchs7/orbithek/internal/item"
)

// pageIn opens a data directory with a page to attach files to.
func pageIn(t *testing.T, dir string) (*DB, item.ID) {
	t.Helper()
	db, _, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	var id item.ID
	err = db.Write(func(s *item.Store) error {
		if err := s.DefineType("admin", item.TypeDef{Name: "page", Body: true}); err != nil {
			return err
		}
		if err := s.CreateSpace("admin", "PLAT", "Platform"); err != nil {
			return err
		}
		it, err := s.Create("admin", item.NewItem{Space: "PLAT", Type: "page", Title: "Runbook"})
		id = it.ID
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return db, id
}

func attach(t *testing.T, db *DB, id item.ID, content []byte, name string) item.Attachment {
	t.Helper()
	a, err := db.StoreAttachment(id, bytes.NewReader(content), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	a.Name = name
	err = db.Write(func(s *item.Store) error {
		a, err = s.AddAttachment("admin", id, a)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func readAttachment(db *DB, id, a item.ID) ([]byte, error) {
	r, err := db.OpenAttachment(id, a)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func TestAttachmentsRoundTripEncrypted(t *testing.T) {
	dir := t.TempDir()
	db, id := pageIn(t, dir)
	defer db.Close()
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte("secret pixels "), 9000)...) // over one chunk
	for _, size := range []int{0, 1, chunkSize - 1, chunkSize, chunkSize + 1, 3 * chunkSize} {
		content := bytes.Repeat([]byte("x"), size)
		if size > 8 {
			content = png[:min(size, len(png))]
			content = append(content, bytes.Repeat([]byte("y"), size-len(content))...)
		}
		a := attach(t, db, id, content, "file.bin")
		sum := sha256.Sum256(content)
		if a.Size != int64(size) || a.SHA256 != hex.EncodeToString(sum[:]) {
			t.Fatalf("size %d: %+v", size, a)
		}
		if size > 8 && a.Type != "image/png" {
			t.Errorf("size %d: type %s", size, a.Type)
		}
		got, err := readAttachment(db, id, a.ID)
		if err != nil || !bytes.Equal(got, content) {
			t.Fatalf("size %d: read %d bytes, %v", size, len(got), err)
		}
		raw, _ := os.ReadFile(attachmentPath(dir, id, a.ID))
		if size > 20 && bytes.Contains(raw, content[len(content)-20:]) {
			t.Fatalf("size %d: stored in the clear", size)
		}
	}
	if _, err := db.StoreAttachment(id, bytes.NewReader(make([]byte, 101)), 100); !errors.Is(err, ErrAttachmentTooLarge) {
		t.Errorf("too large: %v", err)
	}
	// An almost full file system refuses uploads from the critical level
	// on (ADR-0150), and counts what is kept.
	if db.AttachmentBytes() <= 0 {
		t.Error("the attachments kept have no size")
	}
	for _, l := range []diskspace.Level{diskspace.Critical, diskspace.Full} {
		db.SetSpace(l)
		if _, err := db.StoreAttachment(id, bytes.NewReader([]byte("x")), 100); !errors.Is(err, ErrNoSpace) {
			t.Errorf("%v: %v", l, err)
		}
	}
	db.SetSpace(diskspace.Warning)
	if _, err := db.StoreAttachment(id, bytes.NewReader([]byte("x")), 100); err != nil {
		t.Errorf("warning: %v", err)
	}
	db.SetSpace(diskspace.OK)
	if _, err := db.OpenAttachment(id, "../../keys.wal"); err == nil {
		t.Error("a path as attachment ID was opened")
	}
}

func TestAttachmentsRefuseTampering(t *testing.T) {
	dir := t.TempDir()
	db, id := pageIn(t, dir)
	defer db.Close()
	content := bytes.Repeat([]byte("0123456789"), chunkSize/5) // two chunks
	a := attach(t, db, id, content, "two-chunks.txt")
	path := attachmentPath(dir, id, a.ID)
	orig, _ := os.ReadFile(path)
	sealed := chunkSize + 16
	head := len(attachmentMagic) + 8
	for name, change := range map[string]func([]byte) []byte{
		"a flipped byte":    func(b []byte) []byte { b[len(b)/2] ^= 1; return b },
		"cut after a chunk": func(b []byte) []byte { return b[:head+sealed] },
		"an extra chunk":    func(b []byte) []byte { return append(b, b[head:head+sealed]...) },
		"chunks swapped": func(b []byte) []byte {
			return append(append(append([]byte{}, b[:head]...), b[head+sealed:]...), b[head:head+sealed]...)
		},
		"a different magic":     func(b []byte) []byte { b[0] = 'X'; return b },
		"cut inside the header": func(b []byte) []byte { return b[:4] },
	} {
		os.WriteFile(path, change(append([]byte{}, orig...)), 0o640)
		if got, err := readAttachment(db, id, a.ID); err == nil {
			t.Errorf("%s: read %d bytes without error", name, len(got))
		}
	}
	// A file of another item does not decrypt under this item's name.
	os.WriteFile(path, orig, 0o640)
	other := attach(t, db, id, []byte("other"), "o.txt")
	os.WriteFile(attachmentPath(dir, id, other.ID), orig, 0o640)
	if _, err := readAttachment(db, id, other.ID); err == nil {
		t.Error("a file moved to another attachment decrypts")
	}
}

func TestAttachmentFilesGoWithTheirAttachments(t *testing.T) {
	dir := t.TempDir()
	db, id := pageIn(t, dir)
	kept := attach(t, db, id, []byte("kept"), "kept.txt")
	deleted := attach(t, db, id, []byte("deleted"), "deleted.txt")
	if err := db.Write(func(s *item.Store) error { return s.DeleteAttachment("admin", id, deleted.ID) }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(attachmentPath(dir, id, deleted.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a deleted attachment's file stays: %v", err)
	}
	// An upload never recorded, and a crash's temporary file, go on the
	// next start.
	orphan, err := db.StoreAttachment(id, bytes.NewReader([]byte("never recorded")), 100)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, AttachmentsDir, string(id), "upload-1.tmp"), []byte("x"), 0o640)
	db.Close()
	db, rec, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Orphans != 2 {
		t.Errorf("removed %d orphans, want 2", rec.Orphans)
	}
	if _, err := os.Stat(attachmentPath(dir, id, orphan.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Error("an unrecorded upload stays")
	}
	if got, err := readAttachment(db, id, kept.ID); err != nil || string(got) != "kept" {
		t.Fatalf("kept file %q, %v", got, err)
	}
	// Purging the item removes its files and its key.
	err = db.Write(func(s *item.Store) error {
		if err := s.Trash("admin", id); err != nil {
			return err
		}
		return s.Purge("admin", id)
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, AttachmentsDir, string(id))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a purged item's files stay: %v", err)
	}
	if _, err := db.StoreAttachment(id, bytes.NewReader([]byte("x")), 100); err == nil {
		t.Error("stored a file for a purged item")
	}
	db.Close()
}

// An attachment is read from any position, as a video player asks for its
// parts (ADR-0097), and each part still authenticates.
func TestAttachmentsAreReadFromAnyPosition(t *testing.T) {
	dir := t.TempDir()
	db, id := pageIn(t, dir)
	defer db.Close()
	content := make([]byte, 3*chunkSize+123)
	for i := range content {
		content[i] = byte(i * 7)
	}
	a := attach(t, db, id, content, "clip.bin")
	r, err := db.OpenAttachment(id, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, at := range []int64{0, 1, chunkSize - 1, chunkSize, 2*chunkSize + 5, int64(len(content)) - 1} {
		if _, err := r.Seek(at, io.SeekStart); err != nil {
			t.Fatal(err)
		}
		got := make([]byte, 200)
		n, err := io.ReadFull(r, got)
		want := content[at:min(at+200, int64(len(content)))]
		if (err != nil && !errors.Is(err, io.ErrUnexpectedEOF)) || !bytes.Equal(got[:n], want) {
			t.Fatalf("at %d: %d bytes, %v", at, n, err)
		}
	}
	if end, _ := r.Seek(0, io.SeekEnd); end != int64(len(content)) {
		t.Fatalf("the end is at %d", end)
	}
	// A changed byte in the last chunk fails when that part is read.
	path := attachmentPath(dir, id, a.ID)
	raw, _ := os.ReadFile(path)
	raw[len(raw)-40] ^= 1
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	r2, err := db.OpenAttachment(id, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer r2.Close()
	if _, err := r2.Seek(int64(len(content))-10, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(r2); err == nil {
		t.Fatal("a changed part was read")
	}
}

// A file moved to another item keeps its ID: its content is sealed again
// with the new item's key, and the former content goes once the move is
// recorded; content stored for a move never recorded goes on the next
// start (ADR-0188).
func TestAMovedFileIsSealedForItsNewItem(t *testing.T) {
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
	content := bytes.Repeat([]byte("moved "), chunkSize/3) // more than one chunk
	a := attach(t, db, from, content, "moved.bin")
	if err := db.MoveAttachmentContent(from, to, a); err != nil {
		t.Fatal(err)
	}
	if got, err := readAttachment(db, to, a.ID); err != nil || !bytes.Equal(got, content) {
		t.Fatalf("content for the new item: %d bytes, %v", len(got), err)
	}
	if got, err := readAttachment(db, from, a.ID); err != nil || !bytes.Equal(got, content) {
		t.Fatalf("the former content went before the move was recorded: %v", err)
	}
	// The new file is sealed for its new item: read as the old item's, it
	// does not authenticate.
	os.Rename(attachmentPath(dir, from, a.ID), attachmentPath(dir, from, a.ID)+".keep")
	data, err := os.ReadFile(attachmentPath(dir, to, a.ID))
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(attachmentPath(dir, from, a.ID), data, 0o640)
	if _, err := readAttachment(db, from, a.ID); err == nil {
		t.Error("the moved content opens as the old item's")
	}
	os.Rename(attachmentPath(dir, from, a.ID)+".keep", attachmentPath(dir, from, a.ID))
	if err := db.Write(func(s *item.Store) error {
		return s.As("admin").MoveFile(from, a.ID, to, []item.ID{a.ID})
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(attachmentPath(dir, from, a.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the former content stays: %v", err)
	}
	// A move stored but never recorded leaves nothing behind.
	b := attach(t, db, from, []byte("stays"), "stays.txt")
	// Content that is not the file recorded is refused, and not kept.
	wrong := b
	wrong.SHA256 = strings.Repeat("0", 64)
	if err := db.MoveAttachmentContent(from, to, wrong); err == nil {
		t.Error("moved content that is not the file recorded")
	}
	if _, err := os.Stat(attachmentPath(dir, to, b.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("refused content stays: %v", err)
	}
	if err := db.MoveAttachmentContent(from, to, b); err != nil {
		t.Fatal(err)
	}
	db.Close()
	db, rec, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if rec.Orphans != 1 {
		t.Errorf("removed %d orphans, want 1", rec.Orphans)
	}
	if got, err := readAttachment(db, to, a.ID); err != nil || !bytes.Equal(got, content) {
		t.Fatalf("moved content after a restart: %v", err)
	}
	if got, err := readAttachment(db, from, b.ID); err != nil || string(got) != "stays" {
		t.Fatalf("the file not moved: %q %v", got, err)
	}
}

// A video's poster is sealed beside it, kept while the video is, backed
// up, moved and removed with it (ADR-0189).
func TestAPosterGoesWithItsVideo(t *testing.T) {
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
	video := attach(t, db, from, []byte("a video"), "clip.mp4")
	if _, err := db.OpenPoster(from, video.ID); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a poster before one is stored: %v", err)
	}
	if _, err := db.StorePoster(from, video.ID, bytes.NewReader([]byte("a poster")), 100); err != nil {
		t.Fatal(err)
	}
	if got, err := readPoster(db, from, video.ID); err != nil || string(got) != "a poster" {
		t.Fatalf("poster %q %v", got, err)
	}
	raw, _ := os.ReadFile(attachmentPath(dir, from, video.ID+PosterSuffix))
	if bytes.Contains(raw, []byte("a poster")) {
		t.Fatal("the poster is stored in plaintext")
	}
	files, err := db.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	backedUp := false
	for _, f := range files {
		backedUp = backedUp || strings.HasSuffix(f.Name, string(video.ID)+PosterSuffix)
	}
	if !backedUp {
		t.Error("the poster is not backed up")
	}
	// A restart keeps it: it belongs to a recorded video.
	db.Close()
	db, rec, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Orphans != 0 {
		t.Errorf("removed %d files at the start", rec.Orphans)
	}
	// It moves with its video.
	if err := db.MoveAttachmentContent(from, to, video); err != nil {
		t.Fatal(err)
	}
	if err := db.Write(func(s *item.Store) error { return s.As("admin").MoveFile(from, video.ID, to, []item.ID{video.ID}) }); err != nil {
		t.Fatal(err)
	}
	if got, err := readPoster(db, to, video.ID); err != nil || string(got) != "a poster" {
		t.Fatalf("poster after the move %q %v", got, err)
	}
	if _, err := os.Stat(attachmentPath(dir, from, video.ID+PosterSuffix)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the poster stayed behind: %v", err)
	}
	// It goes with its video.
	if err := db.Write(func(s *item.Store) error { return s.DeleteAttachment("admin", to, video.ID) }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(attachmentPath(dir, to, video.ID+PosterSuffix)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the poster of a deleted video stays: %v", err)
	}
	db.Close()
}

func readPoster(db *DB, id, video item.ID) ([]byte, error) {
	r, err := db.OpenPoster(id, video)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// The preview of an entry of a file share is sealed beside the entry,
// which has no file of its own; it is backed up, kept at a restart,
// moved and removed with the entry, and one never recorded goes at the
// next start (ADR-0197).
func TestAPreviewStandsForAnEntryOfAShare(t *testing.T) {
	dir := t.TempDir()
	db, from := pageIn(t, dir)
	var to item.ID
	if err := db.Write(func(s *item.Store) error {
		it, err := s.Create("admin", item.NewItem{Space: "PLAT", Type: "page", Title: "Archive"})
		to = it.ID
		return err
	}); err != nil {
		t.Fatal(err)
	}
	const entry, stray = item.ID("0123456789abcdef0123456789abc197"), item.ID("0123456789abcdef0123456789abd197")
	if _, err := db.OpenPreview(from, entry); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a preview before one is stored: %v", err)
	}
	if _, err := db.OpenPreview(from, "../x"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a preview of a bad ID: %v", err)
	}
	if _, err := db.StorePreview(from, "../x", bytes.NewReader([]byte("x")), 100); err == nil {
		t.Fatal("a preview stored under a bad ID")
	}
	for _, id := range []item.ID{entry, stray} {
		if _, err := db.StorePreview(from, id, bytes.NewReader([]byte("a preview")), 100); err != nil {
			t.Fatal(err)
		}
	}
	linked := item.Attachment{ID: entry, Name: "lake.jpg", Type: "image/jpeg", Size: 9, SHA256: strings.Repeat("ab", 32), Share: &item.ShareFile{Share: "nas", Path: "lake.jpg"}}
	if err := db.Write(func(s *item.Store) error { _, err := s.LinkFile("sync:share:nas", from, linked); return err }); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(attachmentPath(dir, from, entry+PreviewSuffix))
	if bytes.Contains(raw, []byte("a preview")) {
		t.Fatal("the preview is stored in plaintext")
	}
	files, err := db.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range files {
		if strings.Contains(f.Name, string(entry)) {
			names = append(names, f.Name)
		}
	}
	if len(names) != 1 || !strings.HasSuffix(names[0], string(entry)+PreviewSuffix) {
		t.Errorf("backed up for the entry: %v", names)
	}
	// A restart keeps the recorded entry's preview and removes the other.
	db.Close()
	db, rec, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Orphans != 1 {
		t.Errorf("removed %d files at the start, want the preview never recorded", rec.Orphans)
	}
	if got, err := readPreview(db, from, entry); err != nil || string(got) != "a preview" {
		t.Fatalf("preview after a restart %q %v", got, err)
	}
	// It moves with its entry: only the preview, no file of its own.
	if err := db.MoveAttachmentContent(from, to, linked); err != nil {
		t.Fatal(err)
	}
	if got, err := readPreview(db, to, entry); err != nil || string(got) != "a preview" {
		t.Fatalf("preview after the move %q %v", got, err)
	}
	if err := db.MoveAttachmentContent(from, to, item.Attachment{ID: stray, Share: linked.Share}); err == nil {
		t.Error("moved an entry without a preview")
	}
	db.DiscardAttachment(to, entry)
	// It goes with its entry.
	if err := db.Write(func(s *item.Store) error { return s.DeleteAttachment("sync:share:nas", from, entry) }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(attachmentPath(dir, from, entry+PreviewSuffix)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the preview of a deleted entry stays: %v", err)
	}
	db.Close()
}

func readPreview(db *DB, id, entry item.ID) ([]byte, error) {
	r, err := db.OpenPreview(id, entry)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}
