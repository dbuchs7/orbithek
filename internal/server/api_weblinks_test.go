package server

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/dbuchs7/orbithek/internal/collab"
	"github.com/dbuchs7/orbithek/internal/item"
	"github.com/dbuchs7/orbithek/internal/linkpreview"
)

// pagePNG is a small picture of the given colour.
func pagePNG(t *testing.T, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 3, 3))
	img.Set(1, 1, c)
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// fakeSite serves a page whose description and picture a test changes.
type fakeSite struct {
	mu          sync.Mutex
	description string
	picture     []byte
	srv         *httptest.Server
}

func newFakeSite(t *testing.T) *fakeSite {
	t.Helper()
	f := &fakeSite{description: "What the story is about", picture: pagePNG(t, color.RGBA{R: 200, A: 255})}
	mux := http.NewServeMux()
	mux.HandleFunc("/story", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<html><head><title>The story</title><meta property="og:description" content="` + f.description + `">
<meta property="og:site_name" content="Example News"><meta property="og:image" content="/card.png"><meta name="author" content="Ada Muster">
<meta property="article:published_time" content="2026-10-30T08:00:00Z"></head>
<body><article><h2>Part one</h2><p>Some <b>words</b>.</p></article></body></html>`))
	})
	mux.HandleFunc("/card.png", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		w.Write(f.picture)
	})
	mux.HandleFunc("/plain", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<title>Plain</title><meta property="og:image" content="/missing.png">`))
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeSite) set(description string, picture []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.description, f.picture = description, picture
}

// withPreviews switches link previews on for the tests' own server, which
// is local.
func withPreviews(d *Deps) {
	d.LinkPreviews = NewLinkPreviews(linkpreview.New(linkpreview.Options{Allowed: func(a netip.Addr) bool { return a.IsLoopback() }}))
}

// holdBlock opens a live session on an item for who and holds its first
// block, as one editing it does (ADR-0010).
func holdBlock(t *testing.T, w world, id item.ID, who string) {
	t.Helper()
	sub, err := w.deps.Collab.Join(id, w.ids[who], who)
	if err != nil {
		t.Fatal(err)
	}
	snap := <-sub.C
	if len(snap.Blocks) == 0 {
		t.Fatal("the session has no block to hold")
	}
	if err := w.deps.Collab.Apply(id, sub.Client, w.ids[who], []collab.Op{{Op: "lock", ID: snap.Blocks[0].ID}}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sub.Leave)
}

// TestWebLinksWithoutPreviews: with OT_LINK_PREVIEWS off, a web link is
// still saved as an item, by one who may create in the room, the
// ready-made type defined with the first; the server reads no page.
func TestWebLinksWithoutPreviews(t *testing.T) {
	w := newWorld(t)
	if got := w.bert.expect("GET", "/api/linkpreview", "", http.StatusOK); got["enabled"] != false {
		t.Fatalf("previews %v", got)
	}
	w.bert.expect("POST", "/api/linkpreview", `{"url":"https://example.org/story"}`, http.StatusConflict)

	got := w.bert.expect("POST", "/api/weblinks", `{"url":"https://example.org/story?id=1","space":"PLAT","quote":"First line\n\nSecond line"}`, http.StatusCreated)
	it := got["item"].(map[string]any)
	if it["type"] != item.WebLinkType || it["title"] != "example.org/story" || field(it, "properties", "url") != "https://example.org/story?id=1" {
		t.Fatalf("item %v", it)
	}
	body := it["body"].([]any)
	if len(body) != 2 || body[0].(map[string]any)["kind"] != "quote" || body[1].(map[string]any)["text"] != "Second line" {
		t.Fatalf("body %v", body)
	}
	if got["problem"] != nil {
		t.Fatalf("no page was read, so none failed: %v", got["problem"])
	}
	types := w.admin.expect("GET", "/api/types", "", http.StatusOK)["types"].([]any)
	found := false
	for _, ty := range types {
		found = found || ty.(map[string]any)["name"] == item.WebLinkType
	}
	if !found {
		t.Fatal("the ready-made type was not defined")
	}
	// A title given is kept; one below a page lives below it.
	parent := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"Reading","properties":{"priority":"low"}}`, http.StatusCreated)
	pid := field(parent, "item", "id").(string)
	got = w.bert.expect("POST", "/api/weblinks", `{"url":"https://example.org/b","space":"PLAT","parent":"`+pid+`","title":"Mine"}`, http.StatusCreated)
	if field(got, "item", "title") != "Mine" || field(got, "item", "parent") != pid {
		t.Fatalf("item %v", got)
	}

	w.vera.expect("POST", "/api/weblinks", `{"url":"https://example.org/c","space":"PLAT"}`, http.StatusForbidden)
	w.carla.expect("POST", "/api/weblinks", `{"url":"https://example.org/c","space":"PLAT"}`, http.StatusForbidden)
	for _, bad := range []string{`{"url":"javascript:alert(1)","space":"PLAT"}`, `{"url":"","space":"PLAT"}`, `{"url":"/x","space":"PLAT"}`} {
		w.bert.expect("POST", "/api/weblinks", bad, http.StatusBadRequest)
	}
	w.bert.expect("POST", "/api/items/"+it["key"].(string)+"/weblink", `{}`, http.StatusConflict)
}

// TestWebLinkPreviews: with previews on, the server reads what a page
// says about itself for an editor, fills a new web link's fields from it,
// keeps its picture as a file of the item, reads it again on request and
// takes the page's text into the body.
func TestWebLinkPreviews(t *testing.T) {
	site := newFakeSite(t)
	w := newWorld(t, withPreviews)
	story := site.srv.URL + "/story"
	if got := w.vera.expect("GET", "/api/linkpreview", "", http.StatusOK); got["enabled"] != true {
		t.Fatalf("previews %v", got)
	}
	p := w.vera.expect("POST", "/api/linkpreview", `{"url":"`+story+`"}`, http.StatusOK)
	if p["title"] != "The story" || p["site"] != "Example News" || p["image"] != site.srv.URL+"/card.png" || p["published"] != "2026-10-30" {
		t.Fatalf("preview %v", p)
	}
	w.vera.expect("POST", "/api/linkpreview", `{"url":"ftp://example.org"}`, http.StatusBadRequest)
	w.vera.expect("POST", "/api/linkpreview", `{"url":"`+site.srv.URL+`/gone"}`, http.StatusBadGateway)

	got := w.bert.expect("POST", "/api/weblinks", `{"url":"`+story+`","space":"PLAT","text":true}`, http.StatusCreated)
	it := got["item"].(map[string]any)
	key := it["key"].(string)
	props := it["properties"].(map[string]any)
	if it["title"] != "The story" || props["site"] != "Example News" || props["description"] != "What the story is about" ||
		props["author"] != "Ada Muster" || !strings.HasPrefix(props["published"].(string), "2026-10-30") || props["url"] != story {
		t.Fatalf("item %v", it)
	}
	body := it["body"].([]any)
	if len(body) != 2 || body[0].(map[string]any)["kind"] != "heading" || body[1].(map[string]any)["text"] != "Some words." {
		t.Fatalf("the page's text: %v", body)
	}
	files := w.bert.expect("GET", "/api/items/"+key+"/attachments", "", http.StatusOK)["attachments"].([]any)
	if len(files) != 1 || files[0].(map[string]any)["name"] != "preview.png" || files[0].(map[string]any)["type"] != "image/png" {
		t.Fatalf("files %v", files)
	}
	first := files[0].(map[string]any)["id"].(string)

	// Read again: the description changed, the picture did not.
	site.set("A new description", site.picture)
	got = w.bert.expect("POST", "/api/items/"+key+"/weblink", `{}`, http.StatusOK)
	if field(got, "item", "properties", "description") != "A new description" || got["picture"] != false {
		t.Fatalf("read again: %v", got)
	}
	if changed := got["changed"].([]any); len(changed) != 1 || changed[0] != "description" {
		t.Fatalf("changed %v", changed)
	}
	// A new picture is a new version of the old one.
	site.set("A new description", pagePNG(t, color.RGBA{B: 200, A: 255}))
	got = w.bert.expect("POST", "/api/items/"+key+"/weblink", `{"text":true}`, http.StatusOK)
	if got["picture"] != true || len(got["changed"].([]any)) != 0 {
		t.Fatalf("read again: %v", got)
	}
	files = w.bert.expect("GET", "/api/items/"+key+"/attachments", "", http.StatusOK)["attachments"].([]any)
	if len(files) != 2 {
		t.Fatalf("files %v", files)
	}
	var stored item.Item
	w.db.Read(func(s *item.Store) { stored, _ = s.Get(item.ID(it["id"].(string))) })
	if src := stored.Attachments[1].Source; src == nil || src.Attachment != item.ID(first) || src.Item != stored.ID {
		t.Fatalf("the new picture is no version of the old one: %+v", stored.Attachments[1])
	}
	// The text is taken again below what the body holds.
	body = field(got, "item", "body").([]any)
	if len(body) != 5 || body[2].(map[string]any)["kind"] != "divider" || body[4].(map[string]any)["text"] != "Some words." {
		t.Fatalf("the text again: %v", body)
	}

	// Who may only read does not read the page again for the item.
	w.vera.expect("POST", "/api/items/"+key+"/weblink", `{}`, http.StatusForbidden)
	// While people edit the text live, its text is not taken; the fields are.
	holdBlock(t, w, stored.ID, "bert")
	w.bert.expect("POST", "/api/items/"+key+"/weblink", `{"text":true}`, http.StatusConflict)
	w.bert.expect("POST", "/api/items/"+key+"/weblink", `{}`, http.StatusOK)

	// A page that does not answer leaves the link saved, and says why.
	got = w.bert.expect("POST", "/api/weblinks", `{"url":"`+site.srv.URL+`/gone","space":"PLAT"}`, http.StatusCreated)
	if !strings.Contains(got["problem"].(string), "404") || field(got, "item", "title") == "" {
		t.Fatalf("a missing page: %v", got)
	}
	w.bert.expect("POST", "/api/items/"+field(got, "item", "key").(string)+"/weblink", `{}`, http.StatusBadGateway)
	// A picture that cannot be read leaves the link without one.
	got = w.bert.expect("POST", "/api/weblinks", `{"url":"`+site.srv.URL+`/plain","space":"PLAT"}`, http.StatusCreated)
	if field(got, "item", "title") != "Plain" || !strings.Contains(got["problem"].(string), "picture") {
		t.Fatalf("a missing picture: %v", got)
	}
	// An item without an address in its url field has nothing to read.
	task := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"t","properties":{"priority":"low"}}`, http.StatusCreated)
	w.bert.expect("POST", "/api/items/"+field(task, "item", "key").(string)+"/weblink", `{}`, http.StatusBadRequest)
}

// TestAppendingToABody: blocks are added at the end of a body in one
// step, as the caller reads it; never while people edit it live.
func TestAppendingToABody(t *testing.T) {
	w := newWorld(t)
	page := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"Notes","properties":{"priority":"low"},
		"body":[{"kind":"paragraph","text":"Kept."}]}`, http.StatusCreated)
	key := field(page, "item", "key").(string)
	got := w.bert.expect("POST", "/api/items/"+key+"/body/blocks", `{"blocks":[{"kind":"quote","text":"Quoted"},
		{"kind":"paragraph","text":"Source","marks":[{"type":"link","start":0,"end":6,"href":"https://example.org"}]}]}`, http.StatusOK)
	body := field(got, "item", "body").([]any)
	if len(body) != 3 || body[0].(map[string]any)["text"] != "Kept." || body[1].(map[string]any)["kind"] != "quote" {
		t.Fatalf("body %v", body)
	}
	// A protected passage its protector reads stays protected.
	w.bert.expect("POST", "/api/items/"+key+"/secrets", `{"block":0,"start":0,"end":4}`, http.StatusOK)
	got = w.bert.expect("POST", "/api/items/"+key+"/body/blocks", `{"blocks":[{"kind":"paragraph","text":"More"}]}`, http.StatusOK)
	if body := field(got, "item", "body").([]any); len(body) != 4 || len(field(got, "item", "secrets").([]any)) != 1 {
		t.Fatalf("after protecting: %v", got)
	}
	w.vera.expect("POST", "/api/items/"+key+"/body/blocks", `{"blocks":[{"kind":"paragraph","text":"x"}]}`, http.StatusForbidden)
	w.bert.expect("POST", "/api/items/"+key+"/body/blocks", `{"blocks":[]}`, http.StatusBadRequest)
	w.bert.expect("POST", "/api/items/"+key+"/body/blocks", `{"blocks":[{"kind":"nonsense"}]}`, http.StatusBadRequest)
	var id item.ID
	w.db.Read(func(s *item.Store) { it, _ := s.ByKey(key); id = it.ID })
	holdBlock(t, w, id, "bert")
	w.bert.expect("POST", "/api/items/"+key+"/body/blocks", `{"blocks":[{"kind":"paragraph","text":"x"}]}`, http.StatusConflict)
}

// TestTheInstallableApp: the web app's manifest is public, names the
// installation, points at icons that exist and shares into the capture
// view; a share is passed on in the address's fragment, which no server
// or proxy after this one sees.
func TestTheInstallableApp(t *testing.T) {
	w := newWorld(t)
	anon := &client{t: t, h: w.admin.h}
	rec := anon.do("GET", "/manifest.webmanifest", "")
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/manifest+json") {
		t.Fatalf("manifest: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	var m struct {
		Name      string `json:"name"`
		ShortName string `json:"short_name"`
		Start     string `json:"start_url"`
		Display   string `json:"display"`
		Icons     []struct {
			Src, Purpose string
		} `json:"icons"`
		Share struct {
			Action, Method string
			Params         map[string]string
		} `json:"share_target"`
		Shortcuts []struct{ URL string } `json:"shortcuts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m.Name != "Orbithek" || m.Start != "/" || m.Display != "standalone" || m.Share.Action != "/capture" || m.Share.Method != "POST" ||
		m.Share.Params["url"] != "url" || m.Share.Params["text"] != "text" || len(m.Shortcuts) != 1 || m.Shortcuts[0].URL != "/#/capture" {
		t.Fatalf("manifest %s", rec.Body.String())
	}
	maskable := false
	for _, ic := range m.Icons {
		if r := anon.do("GET", ic.Src, ""); r.Code != http.StatusOK || r.Header().Get("Content-Type") != "image/png" {
			t.Errorf("icon %s: %d", ic.Src, r.Code)
		}
		maskable = maskable || ic.Purpose == "maskable"
	}
	if len(m.Icons) < 2 || !maskable {
		t.Fatalf("icons %+v", m.Icons)
	}
	// The installation's name names the app.
	design := w.db.Site.Design()
	design.Name = "Wiki Blumer"
	if err := w.db.Site.SetDesign("admin", design); err != nil {
		t.Fatal(err)
	}
	rec = anon.do("GET", "/manifest.webmanifest", "")
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil || m.Name != "Wiki Blumer" {
		t.Fatalf("named %v %s", err, rec.Body.String())
	}
	if page := anon.do("GET", "/", "").Body.String(); !strings.Contains(page, `<link rel="manifest" href="/manifest.webmanifest">`) {
		t.Fatal("the page links no manifest")
	}

	// A share arrives as a form and is passed on in the fragment.
	req := httptest.NewRequest("POST", "/capture", strings.NewReader(url.Values{"title": {"A page"}, "text": {"Some text & more"}, "url": {"https://example.org/x?y=1"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	w.admin.h.ServeHTTP(rec, req)
	loc := rec.Header().Get("Location")
	if rec.Code != http.StatusSeeOther || !strings.HasPrefix(loc, "/#/capture?") {
		t.Fatalf("share: %d %q", rec.Code, loc)
	}
	q, _ := url.ParseQuery(strings.TrimPrefix(loc, "/#/capture?"))
	if q.Get("title") != "A page" || q.Get("text") != "Some text & more" || q.Get("url") != "https://example.org/x?y=1" {
		t.Fatalf("share passed on as %q", loc)
	}
	// A share by GET, and nothing at all, alike; other fields are dropped.
	if rec := anon.do("GET", "/capture?title=T&secret=x", ""); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/#/capture?title=T" {
		t.Fatalf("GET share: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if rec := anon.do("GET", "/capture", ""); rec.Header().Get("Location") != "/#/capture" {
		t.Fatalf("empty share: %q", rec.Header().Get("Location"))
	}
}

// TestWebLinksThroughMCP: an agent saves a web link and adds to a body
// with its caller's rights.
func TestWebLinksThroughMCP(t *testing.T) {
	w := newWorld(t)
	text, isErr := w.bert.tool("save_web_link", `{"url":"https://example.org/doc","space":"PLAT","quote":"Kept."}`)
	if isErr || !strings.Contains(text, `"type":"weblink"`) {
		t.Fatalf("save: %v %s", isErr, text)
	}
	if text, isErr := w.vera.tool("save_web_link", `{"url":"https://example.org/doc","space":"PLAT"}`); !isErr {
		t.Fatalf("a viewer saved a link: %s", text)
	}
	text, isErr = w.bert.tool("append_to_body", `{"ref":"PLAT-1","blocks":[{"kind":"paragraph","text":"Added."}]}`)
	if isErr || !strings.Contains(text, `"Added."`) || !strings.Contains(text, `"Kept."`) {
		t.Fatalf("append: %v %s", isErr, text)
	}
	if text, isErr := w.bert.tool("read_web_link_again", `{"ref":"PLAT-1"}`); !isErr || !strings.Contains(text, "OT_LINK_PREVIEWS") {
		t.Fatalf("read again while off: %v %s", isErr, text)
	}
}
