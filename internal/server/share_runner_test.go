package server

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dbuchs7/orbithek/internal/budget"
	"github.com/dbuchs7/orbithek/internal/item"
	"github.com/dbuchs7/orbithek/internal/share"
)

// picture is a PNG of the given size and shade, different per shade.
func picture(t *testing.T, w, h int, shade uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{shade, uint8(x), uint8(y), 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func put(t *testing.T, root, p string, data []byte) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(p))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// shareWorld binds a folder to a directory and answers the runner, the
// directory and the folder.
func shareWorld(t *testing.T) (world, *Shares, string, string) {
	t.Helper()
	roots := t.TempDir()
	dir := filepath.Join(roots, "photos")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OT_SHARE_ROOTS", roots)
	sh := NewShares(os.Getenv)
	w := newWorld(t, func(d *Deps) { d.Shares = sh })
	folder := field(w.admin.expect("POST", "/api/album/folders", `{"space":"PLAT","name":"NAS"}`, http.StatusCreated), "item", "id").(string)
	w.admin.expect("POST", "/api/shares", `{"id":"nas","name":"NAS","address":`+jsonOf(dir)+`,"folder":"`+folder+`","link":"smb://nas/photos/{path}","enabled":true}`, http.StatusCreated)
	return w, sh, dir, folder
}

func mustRun(t *testing.T, sh *Shares, dryRun, confirm bool) ShareRun {
	t.Helper()
	run, err := sh.Run(context.Background(), "nas", dryRun, confirm)
	if err != nil {
		t.Fatalf("run: %v (%+v)", err, run)
	}
	return run
}

// entriesOf answers the share's entries by path, wherever they lie.
func entriesOf(t *testing.T, w world, _ string) map[string]item.Attachment {
	t.Helper()
	out := map[string]item.Attachment{}
	w.deps.DB.Read(func(s *item.Store) {
		entries, _ := shareItems(s, "nas")
		for _, e := range entries {
			out[e.a.Share.Path] = e.a
		}
	})
	return out
}

// folderOfEntry answers the folder that holds an entry, and the folders
// the share's runs made, by directory.
func folderOfEntry(t *testing.T, w world, id item.ID) (item.ID, map[string]item.Item) {
	t.Helper()
	var in item.ID
	var made map[string]item.Item
	w.deps.DB.Read(func(s *item.Store) {
		var entries []*entryState
		entries, made = shareItems(s, "nas")
		for _, e := range entries {
			if e.a.ID == id {
				in = e.folder
			}
		}
	})
	return in, made
}

// TestASharesRunsFollowTheShare: runs link new files with a preview,
// read changed ones again, recognise moved ones by their content, keep
// what people set, withdraw files gone and restore them when they come
// back (ADR-0197).
func TestASharesRunsFollowTheShare(t *testing.T) {
	w, sh, dir, folder := shareWorld(t)
	lake, hut := picture(t, 2000, 1000, 10), picture(t, 40, 30, 20)
	put(t, dir, "2024/lake.png", lake)
	put(t, dir, "hut.png", hut)
	put(t, dir, "notes.txt", []byte("no picture"))
	put(t, dir, ".hidden.png", hut)
	put(t, dir, "@eaDir/thumb.png", hut)
	put(t, dir, "clip.mp4", append([]byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom"), bytes.Repeat([]byte{1}, 64)...))

	dry := mustRun(t, sh, true, false)
	if dry.ToRead != 4 || dry.Linked != 0 || len(entriesOf(t, w, folder)) != 0 {
		t.Fatalf("dry run %+v", dry)
	}
	run := mustRun(t, sh, false, false)
	if run.Seen != 4 || run.Linked != 3 || run.Skipped != 1 || run.NoPreview != 1 || run.Error != "" {
		t.Fatalf("first run %+v", run)
	}
	entries := entriesOf(t, w, folder)
	lakeEntry := entries["2024/lake.png"]
	if lakeEntry.Name != "lake.png" || lakeEntry.Type != "image/png" || lakeEntry.Size != int64(len(lake)) || lakeEntry.Photo == nil || lakeEntry.Photo.Width != 2000 ||
		lakeEntry.Author != "sync:share:nas" || entries["clip.mp4"].Type != "video/mp4" {
		t.Fatalf("entries %+v", entries)
	}
	// The picture lies in the folder of its directory, made below the
	// bound folder.
	lakeFolder, made := folderOfEntry(t, w, lakeEntry.ID)
	if dir, ok := made["2024"]; !ok || dir.ID != lakeFolder || dir.Parent != item.ID(folder) || dir.Title != "2024" || dir.Type != item.FolderType {
		t.Fatalf("the folder of 2024: %+v, the entry in %s", made, lakeFolder)
	}
	got := w.admin.do("GET", "/api/items/"+string(lakeFolder)+"/attachments/"+string(lakeEntry.ID), "")
	img, _, err := image.DecodeConfig(bytes.NewReader(got.Body.Bytes()))
	if got.Code != http.StatusOK || err != nil || img.Width != previewEdge || img.Height != previewEdge/2 {
		t.Fatalf("preview %d %v %+v", got.Code, err, img)
	}
	album := append(field(w.admin.expect("GET", "/api/album?folder="+folder, "", http.StatusOK), "pictures").([]any),
		field(w.admin.expect("GET", "/api/album?folder="+string(lakeFolder), "", http.StatusOK), "pictures").([]any)...)
	if len(album) != 3 {
		t.Fatalf("album %d", len(album))
	}
	var shown map[string]any
	for _, p := range album {
		if a := p.(map[string]any)["attachment"].(map[string]any); a["id"] == string(lakeEntry.ID) {
			shown = a["share"].(map[string]any)
		}
	}
	if shown["path"] != "2024/lake.png" || shown["link"] != "smb://nas/photos/2024/lake.png" || shown["preview"] != true {
		t.Fatalf("shown %v", shown)
	}
	if again := mustRun(t, sh, false, false); again.Unchanged != 3 || again.Linked+again.Changed+again.Moved != 0 {
		t.Fatalf("a run with nothing new %+v", again)
	}

	// People label it; the share moves it and changes another.
	w.admin.expect("PUT", "/api/items/"+string(lakeFolder)+"/attachments/"+string(lakeEntry.ID)+"/labels", `{"labels":["see"]}`, http.StatusOK)
	// The folders of a share are its runs' alone.
	w.admin.expect("POST", "/api/album/folders", `{"parent":"`+string(lakeFolder)+`","name":"Mine"}`, http.StatusConflict)
	w.admin.expect("POST", "/api/items/"+string(lakeFolder)+"/trash", "", http.StatusConflict)
	w.admin.expect("POST", "/api/items/"+folder+"/trash", "", http.StatusConflict)
	w.admin.expect("POST", "/api/items/"+string(lakeFolder)+"/move", `{"space":"PLAT","parent":""}`, http.StatusConflict)
	if rec := w.admin.upload("/api/items/"+string(lakeFolder)+"/attachments?name=x.png", "application/octet-stream", hut); rec.Code != http.StatusConflict {
		t.Errorf("an upload into a folder of the share: %d", rec.Code)
	}
	// A title changed by hand is set back by the next run.
	w.admin.expect("PUT", "/api/items/"+string(lakeFolder)+"/title", `{"title":"Renamed"}`, http.StatusOK)
	if err := os.Rename(filepath.Join(dir, "2024", "lake.png"), filepath.Join(dir, "lake-moved.png")); err != nil {
		t.Fatal(err)
	}
	put(t, dir, "hut.png", picture(t, 40, 30, 99))
	run = mustRun(t, sh, false, false)
	if run.Moved != 1 || run.Changed != 1 || run.Withdrawn != 0 || run.FoldersRemoved != 1 {
		t.Fatalf("moved and changed %+v", run)
	}
	// It left 2024, whose folder is gone now that the directory is empty
	// and gone.
	if in, made := folderOfEntry(t, w, lakeEntry.ID); in != item.ID(folder) || len(made) != 0 {
		t.Fatalf("after the move: in %s, folders %+v", in, made)
	}
	entries = entriesOf(t, w, folder)
	if moved := entries["lake-moved.png"]; moved.ID != lakeEntry.ID || len(moved.Labels) != 1 {
		t.Fatalf("the moved entry %+v", moved)
	}

	// A file gone is withdrawn, and comes back with its content elsewhere.
	if err := os.Remove(filepath.Join(dir, "lake-moved.png")); err != nil {
		t.Fatal(err)
	}
	if run = mustRun(t, sh, false, false); run.Withdrawn != 1 {
		t.Fatalf("withdrawn %+v", run)
	}
	if len(field(w.admin.expect("GET", "/api/album?folder="+folder, "", http.StatusOK), "pictures").([]any)) != 2 {
		t.Error("the album shows a withdrawn entry")
	}
	put(t, dir, "Archiv/lake.png", lake)
	if run = mustRun(t, sh, false, false); run.Restored != 1 {
		t.Fatalf("restored %+v", run)
	}
	if in, made := folderOfEntry(t, w, lakeEntry.ID); made["Archiv"].ID != in || made["Archiv"].Title != "Archiv" {
		t.Fatalf("restored into %s, folders %+v", in, made)
	}
	if back := entriesOf(t, w, folder)["Archiv/lake.png"]; back.Name != "lake.png" {
		t.Errorf("the entry back under another name keeps %q", back.Name)
	}
	// A folder's title changed by hand is set back while its directory is
	// there.
	archivID := made2(t, w)["Archiv"].ID
	w.admin.expect("PUT", "/api/items/"+string(archivID)+"/title", `{"title":"Renamed"}`, http.StatusOK)
	mustRun(t, sh, false, false)
	if a := made2(t, w)["Archiv"]; a.Title != "Archiv" {
		t.Fatalf("the title set by hand stays: %q", a.Title)
	}
	if back := entriesOf(t, w, folder)["Archiv/lake.png"]; back.ID != lakeEntry.ID || back.Withdrawn() || len(back.Labels) != 1 {
		t.Fatalf("back %+v", back)
	}

	// Withdrawn long enough, an entry is deleted with its preview.
	if err := os.Remove(filepath.Join(dir, "Archiv", "lake.png")); err != nil {
		t.Fatal(err)
	}
	archiv := made2(t, w)["Archiv"]
	mustRun(t, sh, false, false)
	// The folder holds the withdrawn entry, so it stays.
	if a := made2(t, w)["Archiv"]; a.ID != archiv.ID {
		t.Fatalf("the folder of a withdrawn entry: %+v", a)
	}
	sh.now = func() time.Time { return time.Now().Add((withdrawnDays + 1) * 24 * time.Hour) }
	defer func() { sh.now = time.Now }()
	if run = mustRun(t, sh, false, false); run.Deleted != 1 {
		t.Fatalf("deleted %+v", run)
	}
	if archiv := made2(t, w)["Archiv"]; archiv.ID != "" {
		if w.deps.DB.HasPreview(archiv.ID, lakeEntry.ID) {
			t.Error("the preview of a deleted entry stays")
		}
	}
	if h := sh.Status("nas").History; len(h) != 9 || h[0].Deleted != 1 || h[0].FoldersRemoved != 1 {
		t.Errorf("history: %d runs, newest %+v", len(h), h[0])
	}
	// A directory whose folder a run removed brings that folder back.
	put(t, dir, "2024/again.png", picture(t, 8, 8, 77))
	mustRun(t, sh, false, false)
	if back := made2(t, w)["2024"]; back.ID != lakeFolder || back.Presence != item.Live {
		t.Fatalf("the folder of 2024 again: %+v, was %s", back, lakeFolder)
	}
}

// TestAShareThatFailsEmptiesNothing: an unmounted share and one that lost
// most of its files withdraw nothing until an administrator confirms
// (ADR-0197).
func TestAShareThatFailsEmptiesNothing(t *testing.T) {
	w, sh, dir, folder := shareWorld(t)
	for i := range heldMin + 5 {
		put(t, dir, fmt.Sprintf("p%02d.png", i), picture(t, 8, 8, uint8(i)))
	}
	if run := mustRun(t, sh, false, false); run.Linked != heldMin+5 {
		t.Fatalf("linked %+v", run)
	}
	for i := range heldMin + 5 {
		os.Remove(filepath.Join(dir, fmt.Sprintf("p%02d.png", i)))
	}
	run := mustRun(t, sh, false, false)
	if run.Held != heldMin+5 || run.Withdrawn != 0 || len(run.Problems) == 0 {
		t.Fatalf("held %+v", run)
	}
	if dry := mustRun(t, sh, true, false); dry.Held != heldMin+5 {
		t.Fatalf("dry run held %+v", dry)
	}
	if dry := mustRun(t, sh, true, true); dry.Withdrawn != heldMin+5 || dry.Held != 0 {
		t.Fatalf("dry run confirmed %+v", dry)
	}
	for _, e := range entriesOf(t, w, folder) {
		if e.Withdrawn() {
			t.Fatal("withdrawn without confirmation")
		}
	}

	// The share is not mounted: its directory is on another device now.
	s, _ := w.deps.DB.Site.Share("nas")
	if s.Device != 0 {
		s.Device++
		if _, err := w.deps.DB.Site.SetShare("admin", s); err != nil {
			t.Fatal(err)
		}
		if _, err := sh.Run(context.Background(), "nas", false, true); err == nil {
			t.Fatal("an unmounted share was read")
		}
		s.Device--
		w.deps.DB.Site.SetShare("admin", s)
	}
	if st := sh.Status("nas"); st.History[0].Error == "" && s.Device != 0 {
		t.Errorf("no error reported %+v", st.History[0])
	}
	if run := mustRun(t, sh, false, true); run.Withdrawn != heldMin+5 {
		t.Fatalf("confirmed %+v", run)
	}
}

func TestSharesThroughTheRunRoute(t *testing.T) {
	w, sh, dir, folder := shareWorld(t)
	put(t, dir, "a.png", picture(t, 8, 8, 1))
	put(t, dir, "sub/deeper/b.png", picture(t, 8, 8, 2))
	w.bert.expect("POST", "/api/shares/nas/run", `{}`, http.StatusForbidden)
	w.admin.expect("POST", "/api/shares/nope/run", `{}`, http.StatusNotFound)
	w.admin.expect("POST", "/api/shares/nas/run", `{`, http.StatusBadRequest)
	w.admin.expect("POST", "/api/shares/nas/run", `{"confirm":true}`, http.StatusAccepted)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && (sh.Running("nas") || len(sh.Status("nas").History) == 0) {
		time.Sleep(10 * time.Millisecond)
	}
	got := w.admin.expect("GET", "/api/shares", "", http.StatusOK)
	s := got["shares"].([]any)[0].(map[string]any)
	if s["entries"].(float64) != 2 || len(s["status"].(map[string]any)["history"].([]any)) != 1 {
		t.Fatalf("status %s", jsonOf(s))
	}
	if !strings.Contains(jsonOf(w.admin.expect("GET", "/api/audit", "", http.StatusOK)), "share.confirm") {
		t.Error("a confirmation is not audited")
	}
	// A due share runs on the tick; a busy one is refused.
	sh.mu.Lock()
	sh.last["nas"] = time.Time{}
	sh.running["nas"] = true
	sh.mu.Unlock()
	w.admin.expect("POST", "/api/shares/nas/run", `{}`, http.StatusConflict)
	w.admin.expect("DELETE", "/api/shares/nas", "", http.StatusConflict)
	if _, err := sh.Run(context.Background(), "nas", false, false); err != ErrShareBusy {
		t.Errorf("a busy share ran: %v", err)
	}
	sh.mu.Lock()
	sh.running["nas"] = false
	sh.mu.Unlock()
	sh.Tick(context.Background())
	if len(sh.Status("nas").History) != 2 {
		t.Errorf("the tick did not run the due share: %d runs", len(sh.Status("nas").History))
	}
	// Removing the share removes its entries and previews.
	entry := entriesOf(t, w, folder)["a.png"]
	w.admin.expect("DELETE", "/api/shares/nas", "", http.StatusNoContent)
	if len(entriesOf(t, w, folder)) != 0 || w.deps.DB.HasPreview(item.ID(folder), entry.ID) {
		t.Error("the share's entries stayed")
	}
	if made := made2(t, w); len(made) != 0 {
		t.Errorf("the share's folders stayed: %+v", made)
	}
	if _, err := sh.Run(context.Background(), "nas", false, false); err == nil {
		t.Error("a removed share ran")
	}
	// The loop ticks once and ends with its context.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	sh.Loop(ctx)
	(&Shares{}).Tick(ctx) // not bound to a data directory: nothing to do
}

func TestShareRunsBoundWhatTheyRead(t *testing.T) {
	_, sh, dir, _ := shareWorld(t)
	b := budget.Default()
	b.AttachmentBytes, b.ShareFiles = 4000, 2
	budget.Set(b)
	defer budget.Set(budget.Default())
	put(t, dir, "a.png", picture(t, 4, 4, 1))
	// A PNG with bytes after its end is larger than AttachmentBytes.
	put(t, dir, "b.png", append(picture(t, 8, 8, 2), make([]byte, 5000)...))
	put(t, dir, "c.png", picture(t, 4, 4, 3))
	run := mustRun(t, sh, false, false)
	if !run.More || run.Seen != 2 || run.Linked != 1 || run.TooLarge != 1 {
		t.Fatalf("bounded run %+v", run)
	}
}

func TestRangeReader(t *testing.T) {
	dir := t.TempDir()
	data := bytes.Repeat([]byte("0123456789"), 20000)
	put(t, dir, "v.bin", data)
	a, _ := share.ParseAddress(dir)
	src, err := share.Open(a, "x", 0, func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	r := &rangeReader{ctx: context.Background(), src: src, path: "v.bin", size: int64(len(data))}
	if _, err := r.Seek(100005, 0); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 5)
	if n, _ := r.Read(buf); n != 5 || string(buf) != "56789" {
		t.Errorf("read %q", buf[:n])
	}
	if p, _ := r.Seek(-10, 2); p != int64(len(data))-10 {
		t.Errorf("from the end %d", p)
	}
	if p, _ := r.Seek(5, 1); p != int64(len(data))-5 {
		t.Errorf("from here %d", p)
	}
	if _, err := r.Seek(-1, 0); err == nil {
		t.Error("a negative position")
	}
	if _, err := r.Seek(0, 7); err == nil {
		t.Error("an unknown whence")
	}
	r.Seek(0, 2)
	if _, err := r.Read(buf); err == nil {
		t.Error("read past the end")
	}
}

func made2(t *testing.T, w world) map[string]item.Item {
	_, made := folderOfEntry(t, w, "")
	return made
}

// TestHEICAndRAWOfAShareAreListed: a HEIC and a camera's RAW are linked
// without a preview, larger than AttachmentBytes too, and listed in the
// album, where they show a symbol (ADR-0197).
func TestHEICAndRAWOfAShareAreListed(t *testing.T) {
	w, sh, dir, folder := shareWorld(t)
	b := budget.Default()
	b.AttachmentBytes = 1000
	budget.Set(b)
	defer budget.Set(budget.Default())
	put(t, dir, "phone.heic", append([]byte("\x00\x00\x00\x18ftypheic\x00\x00\x00\x00mif1heic"), make([]byte, 3000)...))
	put(t, dir, "camera.cr2", append([]byte("II*\x00\x10\x00\x00\x00CR\x02\x00"), make([]byte, 3000)...))
	put(t, dir, "notes.txt", []byte("no picture"))
	run := mustRun(t, sh, false, false)
	if run.Linked != 2 || run.NoPreview != 2 || run.Skipped != 1 || run.TooLarge != 0 {
		t.Fatalf("run %+v", run)
	}
	entries := entriesOf(t, w, folder)
	if entries["phone.heic"].Type != "image/heic" || entries["camera.cr2"].Type != "image/tiff" || entries["camera.cr2"].Size != 3012 {
		t.Fatalf("entries %+v", entries)
	}
	album := field(w.admin.expect("GET", "/api/album?folder="+folder, "", http.StatusOK), "pictures").([]any)
	if len(album) != 2 {
		t.Fatalf("album %d", len(album))
	}
	for _, p := range album {
		if field(p.(map[string]any), "attachment", "share", "preview") != false {
			t.Errorf("a preview claimed for %s", jsonOf(p))
		}
	}
	if got := w.admin.do("GET", "/api/items/"+folder+"/attachments/"+string(entries["phone.heic"].ID), ""); got.Code == http.StatusOK {
		t.Errorf("a HEIC of a share gives out %d: the original is not here, and it has no preview", got.Code)
	}
}

func TestRawType(t *testing.T) {
	for head, want := range map[string]string{
		"\x00\x00\x00\x18ftypheic....": "image/heic",
		"\x00\x00\x00\x18ftypmif1....": "image/heic",
		"\x00\x00\x00\x18ftypavif....": "image/avif",
		"\x00\x00\x00\x18ftypcrx ....": "image/x-canon-cr3",
		"\x00\x00\x00\x18ftypisom....": "",
		"FUJIFILMCCD-RAW 0201":         "image/x-fuji-raf",
		"IIRO\x08\x00\x00\x00":         "image/x-olympus-orf",
		"IIU\x00\x08\x00\x00\x00":      "image/x-panasonic-rw2",
		"II*\x00\x08\x00\x00\x00":      "image/tiff",
		"MM\x00*\x00\x00\x00\x08":      "image/tiff",
		"\x89PNG\r\n\x1a\n":            "",
		"":                             "",
	} {
		if got := rawType([]byte(head)); got != want {
			t.Errorf("rawType(%q) = %q, want %q", head, got, want)
		}
	}
}
