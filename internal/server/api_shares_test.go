package server

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSharesThroughTheAPI: administrators bind an empty album folder to a
// directory the operator allows or to an S3 bucket, change and remove the
// binding; everyone else is refused (ADR-0197).
func TestSharesThroughTheAPI(t *testing.T) {
	roots := t.TempDir()
	photos := filepath.Join(roots, "photos")
	if err := os.Mkdir(photos, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OT_SHARE_ROOTS", roots)
	t.Setenv("OT_SHARE_BUCKET_ACCESS_KEY", "AKID")
	w := newWorld(t)
	folder := field(w.admin.expect("POST", "/api/album/folders", `{"space":"PLAT","name":"NAS"}`, http.StatusCreated), "item", "id").(string)
	other := field(w.admin.expect("POST", "/api/album/folders", `{"space":"PLAT","name":"Bucket"}`, http.StatusCreated), "item", "id").(string)
	full := field(w.admin.expect("POST", "/api/album/folders", `{"space":"PLAT","name":"Full"}`, http.StatusCreated), "item", "id").(string)
	if rec := w.admin.upload("/api/items/"+full+"/attachments?name=a.png", "application/octet-stream", append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 100)...)); rec.Code != http.StatusCreated {
		t.Fatalf("upload %d %s", rec.Code, rec.Body)
	}
	page := field(w.admin.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"A task","properties":{"priority":"low"}}`, http.StatusCreated), "item", "id").(string)

	w.bert.expect("GET", "/api/shares", "", http.StatusForbidden)
	w.bert.expect("POST", "/api/shares", `{}`, http.StatusForbidden)
	w.bert.expect("PUT", "/api/shares/nas", `{}`, http.StatusForbidden)
	w.bert.expect("DELETE", "/api/shares/nas", "", http.StatusForbidden)

	got := w.admin.expect("GET", "/api/shares", "", http.StatusOK)
	if len(got["shares"].([]any)) != 0 || jsonOf(got["roots"]) != jsonOf([]string{roots}) || got["rootsVar"] != "OT_SHARE_ROOTS" || got["allowPrivate"] != false {
		t.Fatalf("before any %s", jsonOf(got))
	}
	body := func(id, address, folder string) string {
		return `{"id":"` + id + `","name":"NAS","address":` + jsonOf(address) + `,"folder":"` + folder + `","link":"smb://nas/photos/{path}","enabled":true}`
	}
	w.admin.expect("POST", "/api/shares", `{`, http.StatusBadRequest)
	w.admin.expect("POST", "/api/shares", body("Bad ID", photos, folder), http.StatusBadRequest)
	w.admin.expect("POST", "/api/shares", body("nas", "/etc", folder), http.StatusBadRequest)
	w.admin.expect("POST", "/api/shares", body("nas", filepath.Join(roots, "missing"), folder), http.StatusBadRequest)
	w.admin.expect("POST", "/api/shares", body("nas", photos, page), http.StatusBadRequest)
	w.admin.expect("POST", "/api/shares", body("nas", photos, "nope"), http.StatusBadRequest)
	if got := w.admin.expect("POST", "/api/shares", body("nas", photos, full), http.StatusBadRequest); !strings.Contains(jsonOf(got), "pictures of its own") {
		t.Errorf("a folder with pictures: %s", jsonOf(got))
	}
	kept := w.admin.expect("POST", "/api/shares", body("nas", photos, folder), http.StatusCreated)
	if kept["kind"] != "dir" || kept["folderTitle"] != "NAS" || kept["intervalHours"].(float64) != 6 || kept["enabled"] != true {
		t.Fatalf("kept %s", jsonOf(kept))
	}
	w.admin.expect("POST", "/api/shares", body("nas", photos, other), http.StatusConflict)
	w.admin.expect("POST", "/api/shares", body("nas2", photos, folder), http.StatusBadRequest) // one share per folder

	bucket := w.admin.expect("POST", "/api/shares", body("bucket", "s3://photos?endpoint=https://s3.example.com&region=eu-central-1", other), http.StatusCreated)
	if bucket["kind"] != "s3" || bucket["accessKeyVar"] != "OT_SHARE_BUCKET_ACCESS_KEY" || bucket["secretKeyVar"] != "OT_SHARE_BUCKET_SECRET_KEY" || bucket["keysSet"] != nil {
		t.Fatalf("bucket %s", jsonOf(bucket))
	}
	if strings.Contains(jsonOf(w.admin.expect("GET", "/api/shares", "", http.StatusOK)), "AKID") {
		t.Fatal("a key's value is shown")
	}

	w.admin.expect("PUT", "/api/shares/nope", body("nope", photos, folder), http.StatusNotFound)
	w.admin.expect("PUT", "/api/shares/nas", `{`, http.StatusBadRequest)
	// The ID is the path's and the folder stays when the body names none.
	changed := w.admin.expect("PUT", "/api/shares/nas", `{"id":"other","name":"Fotos","address":`+jsonOf(photos)+`,"intervalHours":24}`, http.StatusOK)
	if changed["id"] != "nas" || changed["folder"] != folder || changed["name"] != "Fotos" || changed["intervalHours"].(float64) != 24 || changed["enabled"] != false {
		t.Fatalf("changed %s", jsonOf(changed))
	}
	w.admin.expect("PUT", "/api/shares/nas", body("nas", photos, other), http.StatusBadRequest) // a share keeps its folder
	w.admin.expect("PUT", "/api/shares/nas", `{"name":"Fotos","address":`+jsonOf(photos)+`,"intervalHours":1000}`, http.StatusBadRequest)

	w.admin.expect("DELETE", "/api/shares/nope", "", http.StatusNotFound)
	w.admin.expect("DELETE", "/api/shares/nas", "", http.StatusNoContent)
	got = w.admin.expect("GET", "/api/shares", "", http.StatusOK)
	if list := got["shares"].([]any); len(list) != 1 || list[0].(map[string]any)["id"] != "bucket" {
		t.Fatalf("after removal %s", jsonOf(got))
	}
	audit := jsonOf(w.admin.expect("GET", "/api/audit", "", http.StatusOK))
	for _, e := range []string{"share.add", "share.change", "share.remove"} {
		if !strings.Contains(audit, e) {
			t.Errorf("no %s in the audit trail", e)
		}
	}
}
