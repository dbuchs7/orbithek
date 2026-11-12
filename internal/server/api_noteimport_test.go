package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/dbuchs7/orbithek/internal/budget"
	"github.com/dbuchs7/orbithek/internal/item"
)

func notesZip(t *testing.T, files ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i := 0; i+1 < len(files); i += 2 {
		f, err := zw.Create(files[i])
		if err != nil {
			t.Fatal(err)
		}
		_, _ = f.Write([]byte(files[i+1]))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func decoded(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("%s: %v", body, err)
	}
	return out
}

// A room administrator rehearses an import of notes and runs it; others
// may not, and what is not an archive, too large or of a type without a
// text is refused (ADR-0211).
func TestImportingNotesThroughTheAPI(t *testing.T) {
	w := newWorld(t)
	w.admin.expect("POST", "/api/types", `{"name":"page","body":true}`, http.StatusCreated)
	w.admin.expect("POST", "/api/types", `{"name":"form","fields":[{"name":"x","kind":"text"}]}`, http.StatusCreated)
	w.admin.expect("PUT", "/api/spaces/PLAT/roles/"+w.ids["vera"], `{"role":"space-admin"}`, http.StatusNoContent)
	archive := notesZip(t, "Vault/Home.md", "# Home\n\nSee [[Other]]. #start", "Vault/Other.md", "Other text.")
	base := "/api/spaces/PLAT/imports/notes"

	if rec := w.bert.upload(base+"?dryRun=true", "application/octet-stream", archive); rec.Code != http.StatusForbidden {
		t.Fatalf("an editor: %d %s", rec.Code, rec.Body)
	}
	if rec := w.carla.upload(base+"?dryRun=true", "application/octet-stream", archive); rec.Code != http.StatusForbidden {
		t.Fatalf("an outsider: %d %s", rec.Code, rec.Body)
	}
	if rec := w.vera.upload("/api/spaces/NOPE/imports/notes", "application/octet-stream", archive); rec.Code != http.StatusNotFound {
		t.Fatalf("an unknown room: %d", rec.Code)
	}
	if rec := w.vera.upload(base, "application/json", archive); rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("as JSON: %d", rec.Code)
	}
	if rec := w.vera.upload(base, "application/octet-stream", []byte("no zip")); rec.Code != http.StatusBadRequest {
		t.Fatalf("no archive: %d", rec.Code)
	}
	if rec := w.vera.upload(base+"?type=form", "application/octet-stream", archive); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "no text") {
		t.Fatalf("a type without a text: %d %s", rec.Code, rec.Body)
	}
	if rec := w.vera.upload(base+"?type=missing", "application/octet-stream", archive); rec.Code != http.StatusBadRequest {
		t.Fatalf("a missing type: %d", rec.Code)
	}
	b := budget.Get()
	small := b
	small.NoteImportBytes = 10
	budget.Set(small)
	rec := w.vera.upload(base, "application/octet-stream", archive)
	budget.Set(b)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("too large: %d", rec.Code)
	}

	rec = w.vera.upload(base+"?dryRun=true&lang=de", "application/octet-stream", archive)
	if rec.Code != http.StatusOK {
		t.Fatalf("dry run: %d %s", rec.Code, rec.Body)
	}
	if dry := decoded(t, rec.Body.Bytes()); dry["new"] != 2.0 || dry["dryRun"] != true || dry["name"] != "Vault" {
		t.Fatalf("dry run %s", rec.Body)
	}
	rec = w.vera.upload(base+"?name=My%20notes", "application/octet-stream", archive)
	if rec.Code != http.StatusOK {
		t.Fatalf("run: %d %s", rec.Code, rec.Body)
	}
	run := decoded(t, rec.Body.Bytes())
	if run["new"] != 2.0 || run["format"] != "folder" {
		t.Fatalf("run %s", rec.Body)
	}
	home := field(run, "examples").([]any)[0].(map[string]any)
	got := w.vera.expect("GET", "/api/items/"+home["key"].(string), "", http.StatusOK)
	if field(got, "item", "title") != "Home" || !strings.Contains(jsonOf(field(got, "item", "labels")), "start") ||
		!strings.Contains(jsonOf(field(got, "item", "body")), `"mention"`) {
		t.Fatalf("home %s", jsonOf(got))
	}
	imports := w.vera.expect("GET", "/api/spaces/PLAT/imports", "", http.StatusOK)
	if jsonOf(imports["imports"]) != `[{"name":"My notes","pages":2}]` {
		t.Fatalf("imports %s", jsonOf(imports))
	}
	w.bert.expect("GET", "/api/spaces/PLAT/imports", "", http.StatusForbidden)
	audit := w.admin.expect("GET", "/api/audit?limit=5", "", http.StatusOK)
	if !strings.Contains(jsonOf(audit), `"notes.import"`) || !strings.Contains(jsonOf(audit), "My notes") {
		t.Fatalf("audit %s", jsonOf(audit))
	}
	// A re-run with the same name makes nothing twice.
	again := decoded(t, w.vera.upload(base+"?name=My%20notes", "application/octet-stream", archive).Body.Bytes())
	if again["new"] != 0.0 || again["unchanged"] != 2.0 {
		t.Fatalf("again %v", again)
	}
	var n int
	w.db.Read(func(s *item.Store) { n = len(s.Find(item.Query{Space: "PLAT"})) })
	if n != 2 {
		t.Fatalf("%d items in the room", n)
	}
}
