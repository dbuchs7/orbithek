package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/dbuchs7/orbithek/internal/budget"
	"github.com/dbuchs7/orbithek/internal/item"
)

// upload sends content as a file of an item.
func (c *client) upload(path, contentType string, content []byte) *httptest.ResponseRecorder {
	c.t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(content))
	req.RemoteAddr = "192.0.2.1:4000"
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.cookie != nil {
		req.AddCookie(c.cookie)
	}
	rec := httptest.NewRecorder()
	c.h.ServeHTTP(rec, req)
	return rec
}

func TestAttachmentsThroughTheAPI(t *testing.T) {
	w := newWorld(t)
	it := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"Patch","properties":{"priority":"low"}}`, http.StatusCreated)
	key := field(it, "item", "key").(string)
	base := "/api/items/" + key + "/attachments"
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 100)...)

	rec := w.bert.upload(base+"?name=Flow%20chart.png", "application/octet-stream", png)
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body)
	}
	var created struct{ Attachment attachmentJSON }
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	a := created.Attachment
	if a.Name != "Flow chart.png" || a.Type != "image/png" || !a.Inline || a.Size != int64(len(png)) || a.Author != w.ids["bert"] {
		t.Fatalf("attachment %+v", a)
	}
	html := w.bert.upload(base+"?name=page.html", "application/octet-stream", []byte("<html><script>alert(1)</script></html>"))
	if html.Code != http.StatusCreated {
		t.Fatalf("html upload: %d %s", html.Code, html.Body)
	}
	var page struct{ Attachment attachmentJSON }
	_ = json.Unmarshal(html.Body.Bytes(), &page)

	// Refused: a form's content type, a viewer, an outsider, a bad name,
	// a file over the budget.
	if r := w.bert.upload(base+"?name=x", "text/plain", []byte("x")); r.Code != http.StatusUnsupportedMediaType {
		t.Errorf("text/plain upload: %d", r.Code)
	}
	if r := w.vera.upload(base+"?name=x", "application/octet-stream", []byte("x")); r.Code != http.StatusForbidden {
		t.Errorf("viewer upload: %d", r.Code)
	}
	if r := w.carla.upload(base+"?name=x", "application/octet-stream", []byte("x")); r.Code != http.StatusNotFound {
		t.Errorf("outsider upload: %d", r.Code)
	}
	if r := w.bert.upload(base+"?name=a/b", "application/octet-stream", []byte("x")); r.Code != http.StatusBadRequest {
		t.Errorf("bad name: %d", r.Code)
	}
	small := budget.Get()
	small.AttachmentBytes = 10
	budget.Set(small)
	defer budget.Set(budget.Default())
	if r := w.bert.upload(base+"?name=big.bin", "application/octet-stream", make([]byte, 11)); r.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("too large: %d %s", r.Code, r.Body)
	}
	budget.Set(budget.Default())

	list := w.vera.expect("GET", base, "", http.StatusOK)["attachments"].([]any)
	if len(list) != 2 {
		t.Fatalf("list %v", list)
	}
	w.carla.expect("GET", base, "", http.StatusNotFound)

	// An image shows inline; anything else only downloads, sandboxed.
	img := w.vera.do("GET", base+"/"+string(a.ID), "")
	if img.Code != http.StatusOK || !bytes.Equal(img.Body.Bytes(), png) || img.Header().Get("Content-Type") != "image/png" ||
		!strings.HasPrefix(img.Header().Get("Content-Disposition"), "inline") {
		t.Fatalf("image download: %d %v", img.Code, img.Header())
	}
	dl := w.vera.do("GET", base+"/"+string(page.Attachment.ID), "")
	if dl.Header().Get("Content-Type") != "application/octet-stream" || !strings.HasPrefix(dl.Header().Get("Content-Disposition"), "attachment") ||
		dl.Header().Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(dl.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Fatalf("html download headers %v", dl.Header())
	}
	w.carla.expect("GET", base+"/"+string(a.ID), "", http.StatusNotFound)

	// An image block shows the file; deleting it hides it.
	w.bert.expect("PUT", "/api/items/"+key+"/body", `{"blocks":[{"kind":"image","ref":"`+string(a.ID)+`","text":"The flow"}]}`, http.StatusOK)
	w.vera.expect("DELETE", base+"/"+string(a.ID), "", http.StatusForbidden)
	w.bert.expect("DELETE", base+"/"+string(a.ID), "", http.StatusOK)
	w.vera.expect("GET", base+"/"+string(a.ID), "", http.StatusNotFound)
}

// TestAlbumListsReadableImages: the album (ADR-0054) lists the images and
// drawings of the items the caller may read, filtered and paged; never
// other files, never a restricted item's to those it excludes.
func TestAlbumListsReadableImages(t *testing.T) {
	w := newWorld(t)
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 100)...)
	mk := func(title string) (string, string) {
		it := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"`+title+`","properties":{"priority":"low"}}`, http.StatusCreated)
		return field(it, "item", "key").(string), field(it, "item", "id").(string)
	}
	openKey, _ := mk("Network plan")
	_, secretID := mk("Secret")
	for _, f := range []struct{ key, name string }{{openKey, "rack.png"}, {openKey, "flow" + DrawingSuffix}, {secretID, "secret.png"}} {
		if r := w.bert.upload("/api/items/"+f.key+"/attachments?name="+f.name, "application/octet-stream", png); r.Code != http.StatusCreated {
			t.Fatalf("upload %s: %d", f.name, r.Code)
		}
	}
	if r := w.bert.upload("/api/items/"+openKey+"/attachments?name=notes.txt", "application/octet-stream", []byte("plain text")); r.Code != http.StatusCreated {
		t.Fatalf("upload notes: %d", r.Code)
	}
	w.bert.expect("PUT", "/api/items/"+secretID+"/restriction", `{"read":["`+w.ids["bert"]+`"],"edit":[]}`, http.StatusOK)

	names := func(c *client, query string) []string {
		var out []string
		for _, p := range field(c.expect("GET", "/api/album"+query, "", http.StatusOK), "pictures").([]any) {
			out = append(out, p.(map[string]any)["attachment"].(map[string]any)["name"].(string))
		}
		return out
	}
	if got := names(w.bert, ""); len(got) != 3 {
		t.Errorf("bert sees %v", got)
	}
	if got := names(w.vera, ""); len(got) != 2 || strings.Contains(strings.Join(got, " "), "secret") {
		t.Errorf("vera sees %v", got)
	}
	if got := names(w.carla, ""); len(got) != 0 {
		t.Errorf("an outsider sees %v", got)
	}
	if got := names(w.vera, "?kind=drawing"); len(got) != 1 || got[0] != "flow"+DrawingSuffix {
		t.Errorf("drawings: %v", got)
	}
	if got := names(w.vera, "?kind=image&q=RACK"); len(got) != 1 || got[0] != "rack.png" {
		t.Errorf("search: %v", got)
	}
	if got := names(w.vera, "?q=network"); len(got) != 2 {
		t.Errorf("search by the item's title: %v", got)
	}
	page := w.vera.expect("GET", "/api/album?limit=1", "", http.StatusOK)
	if field(page, "total").(float64) != 2 || field(page, "next").(float64) != 1 {
		t.Errorf("paging: %v", page)
	}
	for _, bad := range []string{"?kind=audio", "?limit=0", "?limit=201", "?offset=-1"} {
		w.vera.expect("GET", "/api/album"+bad, "", http.StatusBadRequest)
	}
}

// TestCopiesThroughTheAPI: an upload names its source; the files of an
// item say where copies came from and whether the original was changed
// since, as far as the reader may know (ADR-0055).
func TestCopiesThroughTheAPI(t *testing.T) {
	w := newWorld(t)
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 100)...)
	mk := func(title string) (string, string) {
		it := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"`+title+`","properties":{"priority":"low"}}`, http.StatusCreated)
		return field(it, "item", "key").(string), field(it, "item", "id").(string)
	}
	planKey, planID := mk("Plans")
	_, runID := mk("Runbook")
	up := func(itemID, query string, content []byte, status int) map[string]any {
		r := w.bert.upload("/api/items/"+itemID+"/attachments?name=plan.drawing.png"+query, "application/octet-stream", content)
		if r.Code != status {
			t.Fatalf("upload %s: %d %s", query, r.Code, r.Body)
		}
		var out map[string]any
		_ = json.Unmarshal(r.Body.Bytes(), &out)
		return out
	}
	v1 := field(up(planID, "", png, http.StatusCreated), "attachment", "id").(string)
	from := "&sourceItem=" + planID + "&sourceAttachment="
	up(runID, from+v1, append(png, 1), http.StatusBadRequest) // other content is no copy
	c := up(runID, from+v1, png, http.StatusCreated)
	if field(c, "attachment", "origin", "item", "key") != planKey {
		t.Fatalf("copy %v", c)
	}
	origin := func(c *client) map[string]any {
		list := field(c.expect("GET", "/api/items/"+runID+"/attachments", "", http.StatusOK), "attachments").([]any)
		o, _ := list[0].(map[string]any)["origin"].(map[string]any)
		return o
	}
	if o := origin(w.vera); o["readable"] != true || o["newer"] != nil {
		t.Fatalf("a fresh copy: %v", o)
	}
	v2 := field(up(planID, from+v1, append(png, 2), http.StatusCreated), "attachment", "id").(string) // drawn again
	if o := origin(w.vera); o["newer"] == nil || o["newer"].(map[string]any)["id"] != v2 {
		t.Fatalf("a newer original: %v", o)
	}
	w.bert.expect("PUT", "/api/items/"+planID+"/restriction", `{"read":["`+w.ids["bert"]+`"],"edit":[]}`, http.StatusOK)
	if o := origin(w.vera); o["readable"] != false || o["item"] != nil || o["newer"] != nil {
		t.Fatalf("an unreadable source tells: %v", o)
	}
}

// TestAlbumStacksVersions: the versions of one file form one stack in the
// album, the newest on top and the older below it, newest first; a deleted
// version still links the chain, a copy into another item is a stack of
// its own, and a stack shows when any of its versions matches (ADR-0103).
func TestAlbumStacksVersions(t *testing.T) {
	w := newWorld(t)
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 100)...)
	mk := func(title string) string {
		it := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"`+title+`","properties":{"priority":"low"}}`, http.StatusCreated)
		return field(it, "item", "id").(string)
	}
	plans, runbook := mk("Plans"), mk("Runbook")
	up := func(itemID, name, query string, content []byte) string {
		r := w.bert.upload("/api/items/"+itemID+"/attachments?name="+name+query, "application/octet-stream", content)
		if r.Code != http.StatusCreated {
			t.Fatalf("upload %s: %d %s", name, r.Code, r.Body)
		}
		var out map[string]any
		_ = json.Unmarshal(r.Body.Bytes(), &out)
		return field(out, "attachment", "id").(string)
	}
	from := func(item, id string) string { return "&sourceItem=" + item + "&sourceAttachment=" + id }
	v1 := up(plans, "plan"+DrawingSuffix, "", png)
	v2 := up(plans, "plan"+DrawingSuffix, from(plans, v1), append(png, 2))
	v3 := up(plans, "plan"+DrawingSuffix, from(plans, v2), append(png, 3))
	v4 := up(plans, "plan"+DrawingSuffix, from(plans, v3), append(png, 4))
	other := up(plans, "rack.png", "", append(png, 5))
	copied := up(runbook, "plan"+DrawingSuffix, from(plans, v1), png)
	w.bert.expect("DELETE", "/api/items/"+plans+"/attachments/"+v3, "", http.StatusOK)
	w.bert.expect("PUT", "/api/items/"+plans+"/attachments/"+v1+"/labels", `{"labels":["first draft"]}`, http.StatusOK)

	type pic struct {
		id       string
		versions []string
	}
	album := func(query string) (out []pic) {
		for _, p := range field(w.vera.expect("GET", "/api/album"+query, "", http.StatusOK), "pictures").([]any) {
			m := p.(map[string]any)
			got := pic{id: m["attachment"].(map[string]any)["id"].(string)}
			vs, _ := m["versions"].([]any)
			for _, v := range vs {
				got.versions = append(got.versions, v.(map[string]any)["attachment"].(map[string]any)["id"].(string))
			}
			out = append(out, got)
		}
		return out
	}
	stacks := album("")
	if len(stacks) != 3 {
		t.Fatalf("three stacks expected, got %v", stacks)
	}
	byTop := map[string][]string{}
	for _, p := range stacks {
		byTop[p.id] = p.versions
	}
	if vs, ok := byTop[v4]; !ok || !slices.Equal(vs, []string{v2, v1}) {
		t.Errorf("the drawing's stack: %v", stacks)
	}
	if vs, ok := byTop[other]; !ok || vs != nil {
		t.Errorf("a picture of one version: %v", stacks)
	}
	if vs, ok := byTop[copied]; !ok || vs != nil {
		t.Errorf("a copy into another item stacks apart: %v", stacks)
	}
	if got := album("?label=first+draft"); len(got) != 1 || got[0].id != v4 || len(got[0].versions) != 2 {
		t.Errorf("an older version's label finds the stack, newest on top: %v", got)
	}
	if page := w.vera.expect("GET", "/api/album?limit=2", "", http.StatusOK); field(page, "total").(float64) != 3 {
		t.Errorf("paging counts stacks: %v", page)
	}
}

// TestAlbumFoldersAndLabelsThroughTheAPI: folders of the album hold
// pictures uploaded into them; pictures carry labels of their own, which
// narrow the album and never count as labels of items (ADR-0090).
func TestAlbumFoldersAndLabelsThroughTheAPI(t *testing.T) {
	w := newWorld(t)
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 100)...)
	w.vera.expect("POST", "/api/album/folders", `{"space":"PLAT","name":"Logos"}`, http.StatusForbidden)
	logos := field(w.bert.expect("POST", "/api/album/folders", `{"space":"PLAT","name":"Logos"}`, http.StatusCreated), "item").(map[string]any)
	id := logos["id"].(string)
	brand := field(w.bert.expect("POST", "/api/album/folders", `{"parent":"`+id+`","name":"Brand"}`, http.StatusCreated), "item").(map[string]any)
	folders := field(w.vera.expect("GET", "/api/album/folders?space=PLAT", "", http.StatusOK), "folders").([]any)
	if len(folders) != 1 || folders[0].(map[string]any)["name"] != "Logos" {
		t.Fatalf("top folders %v", folders)
	}
	// The listing says who may add pictures to a folder (ADR-0187).
	if folders[0].(map[string]any)["editable"] != false {
		t.Fatalf("a reader's folder %v", folders[0])
	}
	if own := field(w.bert.expect("GET", "/api/album/folders?space=PLAT", "", http.StatusOK), "folders").([]any); own[0].(map[string]any)["editable"] != true {
		t.Fatalf("an editor's folder %v", own[0])
	}
	if sub := field(w.vera.expect("GET", "/api/album/folders?parent="+id, "", http.StatusOK), "folders").([]any); len(sub) != 1 || sub[0].(map[string]any)["id"] != brand["id"] {
		t.Fatalf("sub folders %v", sub)
	}

	rec := w.bert.upload("/api/items/"+id+"/attachments?name=logo.png", "application/octet-stream", png)
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body)
	}
	var up struct {
		Attachment struct {
			ID string `json:"id"`
		} `json:"attachment"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &up); err != nil || up.Attachment.ID == "" {
		t.Fatalf("upload answer %s", rec.Body)
	}
	task := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"Patch","properties":{"priority":"low"}}`, http.StatusCreated)
	if r := w.bert.upload("/api/items/"+field(task, "item", "key").(string)+"/attachments?name=other.png", "application/octet-stream", png); r.Code != http.StatusCreated {
		t.Fatalf("upload other: %d", r.Code)
	}

	labels := "/api/items/" + id + "/attachments/" + up.Attachment.ID + "/labels"
	w.vera.expect("PUT", labels, `{"labels":["brand"]}`, http.StatusForbidden)
	w.bert.expect("PUT", labels, `{"labels":["bad label!"]}`, http.StatusBadRequest)
	w.bert.expect("PUT", labels, `{"labels":["Brand","print"]}`, http.StatusOK)

	// Speaker's notes for a presentation of the folder (ADR-0176).
	notes := "/api/items/" + id + "/attachments/" + up.Attachment.ID + "/notes"
	w.vera.expect("PUT", notes, `{"notes":"Hello"}`, http.StatusForbidden)
	w.bert.expect("PUT", notes, `{"notes":" Greet everyone.\r\nThen the logo. "}`, http.StatusOK)

	pictures := func(query string) []map[string]any {
		var out []map[string]any
		for _, p := range field(w.vera.expect("GET", "/api/album"+query, "", http.StatusOK), "pictures").([]any) {
			out = append(out, p.(map[string]any))
		}
		return out
	}
	if got := pictures("?folder=" + id); len(got) != 1 || got[0]["used"] != false || got[0]["editable"] != false || got[0]["notes"] != "Greet everyone.\nThen the logo." {
		t.Fatalf("the folder's pictures %v", got)
	}
	w.bert.expect("PUT", notes, `{"notes":""}`, http.StatusOK)

	// Renaming keeps the extension (ADR-0180).
	rename := "/api/items/" + id + "/attachments/" + up.Attachment.ID + "/name"
	w.vera.expect("PUT", rename, `{"name":"Mark"}`, http.StatusForbidden)
	w.bert.expect("PUT", rename, `{"name":"a/b"}`, http.StatusBadRequest)
	w.bert.expect("PUT", rename, `{"name":"Company mark"}`, http.StatusOK)
	if got := pictures("?folder=" + id); got[0]["attachment"].(map[string]any)["name"] != "Company mark.png" {
		t.Fatalf("renamed %v", got[0]["attachment"])
	}
	if got := pictures("?folder=" + id); got[0]["notes"] != nil {
		t.Fatalf("cleared notes %v", got[0]["notes"])
	}
	if got := pictures("?label=Brand"); len(got) != 1 || got[0]["attachment"].(map[string]any)["labels"].([]any)[1] != "print" {
		t.Fatalf("by label %v", got)
	}
	if got := pictures("?q=print"); len(got) != 1 {
		t.Fatalf("search by label %v", got)
	}
	if got := len(pictures("")); got != 2 {
		t.Fatalf("the whole album has %d pictures", got)
	}
	counted := field(w.vera.expect("GET", "/api/album/labels?q=pr", "", http.StatusOK), "labels").([]any)
	if len(counted) != 1 || counted[0].(map[string]any)["label"] != "print" {
		t.Fatalf("picture labels %v", counted)
	}
	if items := field(w.vera.expect("GET", "/api/labels", "", http.StatusOK), "labels").([]any); len(items) != 0 {
		t.Fatalf("a picture's labels count as an item's: %v", items)
	}
	if got := field(w.carla.expect("GET", "/api/album/labels", "", http.StatusOK), "labels").([]any); len(got) != 0 {
		t.Fatalf("an outsider sees %v", got)
	}
}

// A collection gathers the labelled pictures of every room, as far as each
// reader may read them, and its rule is changed by whoever may edit it
// (ADR-0175).
func TestAlbumCollectionsThroughTheAPI(t *testing.T) {
	w := newWorld(t)
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 100)...)
	w.admin.expect("POST", "/api/spaces", `{"key":"OPS","name":"Operations"}`, http.StatusCreated)
	w.admin.expect("PUT", "/api/spaces/OPS/roles/"+w.ids["bert"], `{"role":"editor"}`, http.StatusNoContent)
	picture := func(space, title, name string, labels string) string {
		t.Helper()
		key := field(w.bert.expect("POST", "/api/items", `{"space":"`+space+`","type":"task","title":"`+title+`","properties":{"priority":"low"}}`, http.StatusCreated), "item", "key").(string)
		rec := w.bert.upload("/api/items/"+key+"/attachments?name="+name, "application/octet-stream", png)
		if rec.Code != http.StatusCreated {
			t.Fatalf("upload %s: %d %s", name, rec.Code, rec.Body)
		}
		var up struct {
			Attachment struct {
				ID string `json:"id"`
			} `json:"attachment"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &up); err != nil {
			t.Fatal(err)
		}
		if labels != "" {
			w.bert.expect("PUT", "/api/items/"+key+"/attachments/"+up.Attachment.ID+"/labels", `{"labels":[`+labels+`]}`, http.StatusOK)
		}
		return key
	}
	picture("PLAT", "Brand", "plat.png", `"logo","print"`)
	picture("OPS", "Ops", "ops.png", `"logo"`)
	page := picture("PLAT", "Style", "style.png", "")
	w.bert.expect("PUT", "/api/items/"+page+"/labels", `{"labels":["logo"]}`, http.StatusOK)

	w.vera.expect("POST", "/api/album/collections", `{"space":"PLAT","name":"Logos","labels":["logo"]}`, http.StatusForbidden)
	w.bert.expect("POST", "/api/album/collections", `{"space":"PLAT","name":"Logos","labels":[]}`, http.StatusBadRequest)
	w.bert.expect("POST", "/api/album/collections", `{"space":"PLAT","name":"Logos","labels":["logo"],"match":"some"}`, http.StatusBadRequest)
	made := field(w.bert.expect("POST", "/api/album/collections", `{"space":"PLAT","name":"Logos","labels":["Logo"]}`, http.StatusCreated), "item").(map[string]any)
	id := made["id"].(string)

	listed := field(w.vera.expect("GET", "/api/album/folders?space=PLAT", "", http.StatusOK), "collections").([]any)
	if len(listed) != 1 || listed[0].(map[string]any)["name"] != "Logos" || listed[0].(map[string]any)["editable"] != false ||
		fmt.Sprint(listed[0].(map[string]any)["rule"]) != "map[items:false labels:[logo] match:any]" {
		t.Fatalf("collections %v", listed)
	}
	names := func(c *client, query string) []string {
		t.Helper()
		var out []string
		for _, p := range field(c.expect("GET", "/api/album"+query, "", http.StatusOK), "pictures").([]any) {
			out = append(out, p.(map[string]any)["attachment"].(map[string]any)["name"].(string))
		}
		slices.Sort(out)
		return out
	}
	if got := names(w.bert, "?collection="+id); !slices.Equal(got, []string{"ops.png", "plat.png"}) {
		t.Errorf("bert's collection %v", got)
	}
	if got := names(w.vera, "?collection="+id); !slices.Equal(got, []string{"plat.png"}) {
		t.Errorf("vera's collection %v", got)
	}
	if got := names(w.bert, "?collection="+id+"&space=OPS"); !slices.Equal(got, []string{"ops.png"}) {
		t.Errorf("the collection narrowed to a room %v", got)
	}
	w.carla.expect("GET", "/api/album?collection="+id, "", http.StatusNotFound)
	w.bert.expect("GET", "/api/album?collection="+page, "", http.StatusNotFound)

	// The collection keeps the folder its new pictures go into, for
	// everyone who adds to it (ADR-0221).
	if listed[0].(map[string]any)["folder"] != "" {
		t.Fatalf("a new collection keeps a folder: %v", listed[0])
	}
	shelf := field(w.bert.expect("POST", "/api/album/folders", `{"space":"PLAT","name":"Shelf"}`, http.StatusCreated), "item").(map[string]any)["id"].(string)
	w.vera.expect("PUT", "/api/album/collections/"+id+"/folder", `{"folder":"`+shelf+`"}`, http.StatusForbidden)
	w.bert.expect("PUT", "/api/album/collections/"+id+"/folder", `{"folder":"`+id+`"}`, http.StatusBadRequest) // a collection is no folder
	if got := field(w.bert.expect("PUT", "/api/album/collections/"+id+"/folder", `{"folder":"`+shelf+`"}`, http.StatusOK), "folder"); got != shelf {
		t.Errorf("the folder kept %v", got)
	}
	if kept := field(w.vera.expect("GET", "/api/album/folders?space=PLAT", "", http.StatusOK), "collections").([]any)[0].(map[string]any)["folder"]; kept != shelf {
		t.Errorf("another reader sees the folder %v", kept)
	}
	w.bert.expect("PUT", "/api/album/collections/"+id+"/folder", `{"folder":""}`, http.StatusOK)

	w.vera.expect("PUT", "/api/album/collections/"+id, `{"labels":["logo"],"items":true}`, http.StatusForbidden)
	w.bert.expect("PUT", "/api/album/collections/"+id, `{"labels":["logo"],"items":true}`, http.StatusOK)
	if got := names(w.bert, "?collection="+id); !slices.Equal(got, []string{"ops.png", "plat.png", "style.png"}) {
		t.Errorf("with item labels %v", got)
	}
	kept := w.bert.expect("PUT", "/api/album/collections/"+id, `{"labels":["Print","logo"],"match":"all"}`, http.StatusOK)
	if got := fmt.Sprint(kept["rule"]); got != "map[items:false labels:[logo print] match:all]" {
		t.Errorf("the rule kept %s", got)
	}
	if got := names(w.bert, "?collection="+id); !slices.Equal(got, []string{"plat.png"}) {
		t.Errorf("with every label %v", got)
	}
	w.bert.expect("PUT", "/api/album/collections/"+page, `{"labels":["logo"]}`, http.StatusNotFound)
}

// A video plays in the text: it is sent inline and in the parts a player
// asks for, and only a video a browser plays makes a video block
// (ADR-0097).
func TestVideosPlayInTheText(t *testing.T) {
	w := newWorld(t)
	it := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"Clip","properties":{"priority":"low"}}`, http.StatusCreated)
	key := field(it, "item", "key").(string)
	base := "/api/items/" + key + "/attachments"
	mp4 := append([]byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom"), bytes.Repeat([]byte{9}, 200000)...)
	var up struct{ Attachment attachmentJSON }
	rec := w.bert.upload(base+"?name=clip.mp4", "application/octet-stream", mp4)
	_ = json.Unmarshal(rec.Body.Bytes(), &up)
	if rec.Code != http.StatusCreated || up.Attachment.Type != "video/mp4" {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body)
	}
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 100)...)
	var pic struct{ Attachment attachmentJSON }
	_ = json.Unmarshal(w.bert.upload(base+"?name=still.png", "application/octet-stream", png).Body.Bytes(), &pic)

	req := httptest.NewRequest(http.MethodGet, base+"/"+string(up.Attachment.ID), nil)
	req.RemoteAddr = "192.0.2.1:4000"
	req.AddCookie(w.bert.cookie)
	req.Header.Set("Range", "bytes=70000-70009")
	part := httptest.NewRecorder()
	w.bert.h.ServeHTTP(part, req)
	if part.Code != http.StatusPartialContent || !bytes.Equal(part.Body.Bytes(), mp4[70000:70010]) ||
		part.Header().Get("Content-Type") != "video/mp4" || !strings.HasPrefix(part.Header().Get("Content-Disposition"), "inline") ||
		part.Header().Get("Content-Range") != fmt.Sprintf("bytes 70000-70009/%d", len(mp4)) {
		t.Fatalf("a part: %d %v %q", part.Code, part.Header(), part.Body.Bytes())
	}
	body := func(ref item.ID) string {
		return `{"blocks":[{"kind":"video","ref":"` + string(ref) + `","text":"The clip","width":60,"align":"center"}]}`
	}
	w.bert.expect("PUT", "/api/items/"+key+"/body", body(pic.Attachment.ID), http.StatusBadRequest)
	got := w.bert.expect("PUT", "/api/items/"+key+"/body", body(up.Attachment.ID), http.StatusOK)
	if b := field(got, "item", "body").([]any)[0].(map[string]any); b["kind"] != "video" || b["text"] != "The clip" {
		t.Fatalf("the video block %v", b)
	}
}

// TestBPMNDiagramsShowTheirFile: a BPMN model is uploaded as a file and
// shown by a bpmn block, which refuses a picture; a process block keeps its
// engine and process (ADR-0129).
func TestBPMNDiagramsShowTheirFile(t *testing.T) {
	w := newWorld(t)
	it := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"Flow","properties":{"priority":"low"}}`, http.StatusCreated)
	key := field(it, "item", "key").(string)
	base := "/api/items/" + key + "/attachments"
	var model, pic struct{ Attachment attachmentJSON }
	_ = json.Unmarshal(w.bert.upload(base+"?name=order.bpmn", "application/octet-stream", []byte(`<?xml version="1.0" encoding="UTF-8"?><bpmn:definitions/>`)).Body.Bytes(), &model)
	_ = json.Unmarshal(w.bert.upload(base+"?name=still.png", "application/octet-stream", append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 100)...)).Body.Bytes(), &pic)
	if model.Attachment.Type != "text/xml; charset=utf-8" {
		t.Fatalf("the model reads as %q", model.Attachment.Type)
	}
	body := func(ref item.ID) string {
		return `{"blocks":[{"kind":"bpmn","ref":"` + string(ref) + `","text":"Order","view":"both"},{"kind":"process","engine":"atlas","process":"order-to-cash","text":"Live","view":"text"}]}`
	}
	w.bert.expect("PUT", "/api/items/"+key+"/body", body(pic.Attachment.ID), http.StatusBadRequest)
	got := field(w.bert.expect("PUT", "/api/items/"+key+"/body", body(model.Attachment.ID), http.StatusOK), "item", "body").([]any)
	if d, p := got[0].(map[string]any), got[1].(map[string]any); d["kind"] != "bpmn" || d["ref"] != string(model.Attachment.ID) ||
		p["kind"] != "process" || p["engine"] != "atlas" || p["process"] != "order-to-cash" || p["text"] != "Live" ||
		d["view"] != "both" || p["view"] != "text" {
		t.Fatalf("the body %v", got)
	}
	w.bert.expect("PUT", "/api/items/"+key+"/body", `{"blocks":[{"kind":"paragraph","text":"x","engine":"atlas"}]}`, http.StatusBadRequest)
	// What a diagram shows is one of its views (ADR-0143), and only a diagram has one.
	w.bert.expect("PUT", "/api/items/"+key+"/body", `{"blocks":[{"kind":"process","engine":"atlas","process":"p","view":"all"}]}`, http.StatusBadRequest)
	w.bert.expect("PUT", "/api/items/"+key+"/body", `{"blocks":[{"kind":"paragraph","text":"x","view":"both"}]}`, http.StatusBadRequest)
}

// TestRecordingsPlayFromTheFileList: sound recorded in the browser is told
// by its tracks, not taken into the album as a video, and sent inline in
// parts so that the file list plays it
// (ADR-0219).
func TestRecordingsPlayFromTheFileList(t *testing.T) {
	w := newWorld(t)
	it := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"Call notes","properties":{"priority":"low"}}`, http.StatusCreated)
	key := field(it, "item", "key").(string)
	base := "/api/items/" + key + "/attachments"
	// el is an EBML element with a size of one byte.
	el := func(id string, content ...string) string {
		c := strings.Join(content, "")
		return id + string([]byte{0x80 | byte(len(c))}) + c
	}
	track := func(kind byte) string { return el("\xAE", el("\x83", string([]byte{kind}))) }
	webm := func(kinds ...byte) []byte {
		var entries []string
		for _, k := range kinds {
			entries = append(entries, track(k))
		}
		head := el("\x1A\x45\xDF\xA3", el("\x42\x82", "webm"))
		seg := "\x18\x53\x80\x67\x01\xFF\xFF\xFF\xFF\xFF\xFF\xFF" + el("\x16\x54\xAE\x6B", entries...) + "\x1F\x43\xB6\x75\x01\xFF\xFF\xFF\xFF\xFF\xFF\xFF"
		return append([]byte(head+seg), bytes.Repeat([]byte{5}, 50000)...)
	}
	cases := []struct {
		name, want string
		data       []byte
	}{
		{"Recording 2026-11-12 08-14.webm", "audio/webm", webm(2)},
		{"clip.webm", "video/webm", webm(1, 2)},
		{"memo.ogg", "audio/ogg", append([]byte("OggS\x00\x02"+strings.Repeat("\x00", 20)+"\x01\x13OpusHead"), bytes.Repeat([]byte{3}, 400)...)},
		{"song.mp3", "audio/mpeg", append([]byte("ID3\x04\x00\x00\x00\x00\x00\x00"), bytes.Repeat([]byte{1}, 400)...)},
	}
	ids := map[string]item.ID{}
	for _, c := range cases {
		var up struct{ Attachment attachmentJSON }
		rec := w.bert.upload(base+"?name="+strings.ReplaceAll(c.name, " ", "%20"), "application/octet-stream", c.data)
		_ = json.Unmarshal(rec.Body.Bytes(), &up)
		if rec.Code != http.StatusCreated || up.Attachment.Type != c.want {
			t.Fatalf("%s: %d %s, want the type %s", c.name, rec.Code, rec.Body, c.want)
		}
		ids[c.name] = up.Attachment.ID
	}
	// Sent inline, with its type, in parts.
	req := httptest.NewRequest(http.MethodGet, base+"/"+string(ids["Recording 2026-11-12 08-14.webm"]), nil)
	req.RemoteAddr = "192.0.2.1:4000"
	req.AddCookie(w.bert.cookie)
	req.Header.Set("Range", "bytes=100-109")
	part := httptest.NewRecorder()
	w.bert.h.ServeHTTP(part, req)
	if part.Code != http.StatusPartialContent || part.Header().Get("Content-Type") != "audio/webm" ||
		!strings.HasPrefix(part.Header().Get("Content-Disposition"), "inline") || part.Header().Get("X-Content-Type-Options") != "nosniff" ||
		!strings.Contains(part.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Fatalf("a part of the recording: %d %v", part.Code, part.Header())
	}
	for _, name := range []string{"memo.ogg", "song.mp3"} {
		rec := w.bert.do("GET", base+"/"+string(ids[name]), "")
		if !strings.HasPrefix(rec.Header().Get("Content-Disposition"), "inline") || !strings.HasPrefix(rec.Header().Get("Content-Type"), "audio/") {
			t.Fatalf("%s is sent as %v", name, rec.Header())
		}
	}
	// The album takes the video, not the recording.
	pictures := field(w.bert.expect("GET", "/api/album?kind=video", "", http.StatusOK), "pictures").([]any)
	if len(pictures) != 1 || pictures[0].(map[string]any)["attachment"].(map[string]any)["name"] != "clip.webm" {
		t.Fatalf("the album's videos: %v", pictures)
	}
	// A copy of the item keeps the recording sound.
	copied := field(w.bert.expect("POST", "/api/items/"+key+"/copy", `{"space":"PLAT","title":"Call notes (copy)"}`, http.StatusCreated), "item", "key").(string)
	var files struct{ Attachments []attachmentJSON }
	_ = json.Unmarshal(w.bert.do("GET", "/api/items/"+copied+"/attachments", "").Body.Bytes(), &files)
	types := map[string]string{}
	for _, a := range files.Attachments {
		types[a.Name] = a.Type
	}
	if types["Recording 2026-11-12 08-14.webm"] != "audio/webm" || types["clip.webm"] != "video/webm" {
		t.Fatalf("the copy's files: %v", types)
	}
	// A recording is no video a text plays.
	w.bert.expect("PUT", "/api/items/"+key+"/body", `{"blocks":[{"kind":"video","ref":"`+string(ids["Recording 2026-11-12 08-14.webm"])+`"}]}`, http.StatusBadRequest)
}

// TestSoundPlaysInTheText: a block of the kind audio plays a sound file of
// its item, refuses a video or a picture and a file before the item exists,
// and a copy of the item plays its own copy of the sound
// (ADR-0224).
func TestSoundPlaysInTheText(t *testing.T) {
	w := newWorld(t)
	it := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"Call","properties":{"priority":"low"}}`, http.StatusCreated)
	key := field(it, "item", "key").(string)
	base := "/api/items/" + key + "/attachments"
	upload := func(name string, data []byte) item.ID {
		var up struct{ Attachment attachmentJSON }
		rec := w.bert.upload(base+"?name="+name, "application/octet-stream", data)
		_ = json.Unmarshal(rec.Body.Bytes(), &up)
		if rec.Code != http.StatusCreated {
			t.Fatalf("upload %s: %d %s", name, rec.Code, rec.Body)
		}
		return up.Attachment.ID
	}
	sound := upload("memo.ogg", append([]byte("OggS\x00\x02"+strings.Repeat("\x00", 20)+"\x01\x13OpusHead"), bytes.Repeat([]byte{3}, 400)...))
	clip := upload("clip.mp4", append([]byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom"), bytes.Repeat([]byte{9}, 400)...))
	still := upload("still.png", append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 100)...))
	body := func(ref item.ID, extra string) string {
		return `{"blocks":[{"kind":"audio","ref":"` + string(ref) + `","text":"The call"` + extra + `}]}`
	}
	for _, ref := range []item.ID{clip, still} {
		w.bert.expect("PUT", "/api/items/"+key+"/body", body(ref, ""), http.StatusBadRequest)
	}
	// A sound has neither a width nor an alignment, and a plain caption.
	w.bert.expect("PUT", "/api/items/"+key+"/body", body(sound, `,"width":50`), http.StatusBadRequest)
	w.bert.expect("PUT", "/api/items/"+key+"/body", body(sound, `,"align":"center"`), http.StatusBadRequest)
	w.bert.expect("PUT", "/api/items/"+key+"/body", body(sound, `,"marks":[{"type":"bold","start":0,"end":3}]`), http.StatusBadRequest)
	got := w.bert.expect("PUT", "/api/items/"+key+"/body", body(sound, ""), http.StatusOK)
	if b := field(got, "item", "body").([]any)[0].(map[string]any); b["kind"] != "audio" || b["ref"] != string(sound) || b["text"] != "The call" {
		t.Fatalf("the sound block %v", b)
	}
	// A new item has no files yet.
	w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"x","properties":{"priority":"low"},"body":`+body(sound, "")+`}`, http.StatusBadRequest)
	// A copy plays its own file.
	copied := field(w.bert.expect("POST", "/api/items/"+key+"/copy", `{"space":"PLAT","title":"Call (copy)"}`, http.StatusCreated), "item", "key").(string)
	cb := field(w.bert.expect("GET", "/api/items/"+copied, "", http.StatusOK), "item", "body").([]any)[0].(map[string]any)
	if cb["kind"] != "audio" || cb["ref"] == string(sound) || cb["ref"] == "" {
		t.Fatalf("the copy's sound block %v", cb)
	}
}
