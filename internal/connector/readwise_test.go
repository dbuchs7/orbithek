package connector

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dbuchs7/orbithek/internal/item"
	"github.com/dbuchs7/orbithek/internal/site"
)

// fakeReadwise answers the export of Readwise's API v2 (ADR-0212): the
// full export in two pages, or with updatedAfter the books whose
// highlights changed; it asks the first request to wait once.
type fakeReadwise struct {
	mu      sync.Mutex
	full    [][]map[string]any
	changed []map[string]any
	asked   []string
	waited  bool
	refuse  bool
}

func (f *fakeReadwise) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.URL.Path != "/api/v2/export/" || r.Header.Get("Authorization") != "Token secret" || f.refuse {
		http.Error(w, "no", http.StatusUnauthorized)
		return
	}
	if !f.waited {
		f.waited = true
		w.Header().Set("Retry-After", "3")
		http.Error(w, "slow down", http.StatusTooManyRequests)
		return
	}
	q := r.URL.Query()
	f.asked = append(f.asked, q.Encode())
	if q.Has("updatedAfter") {
		json.NewEncoder(w).Encode(map[string]any{"count": len(f.changed), "nextPageCursor": nil, "results": f.changed})
		return
	}
	page := 0
	if q.Get("pageCursor") == "2" {
		page = 1
	}
	next := any(nil)
	if page == 0 {
		next = 2
	}
	json.NewEncoder(w).Encode(map[string]any{"count": 2, "nextPageCursor": next, "results": f.full[page]})
}

func book(id int, title string, tags []string, highlights ...map[string]any) map[string]any {
	var bt []any
	for _, t := range tags {
		bt = append(bt, map[string]any{"id": 1, "name": t})
	}
	return map[string]any{"user_book_id": id, "title": title, "readable_title": title, "author": "Ann Author", "category": "books",
		"source": "kindle", "cover_image_url": "https://img.example/" + title + ".jpg", "source_url": "https://books.example/" + title,
		"readwise_url": "https://read.example/bookreview/1", "book_tags": bt, "summary": "", "document_note": "", "highlights": highlights}
}

func highlight(id int, text, note string, location int, kind string) map[string]any {
	return map[string]any{"id": id, "text": text, "note": note, "location": location, "location_type": kind,
		"highlighted_at": "2026-11-01T10:00:00Z", "url": nil, "readwise_url": "https://read.example/open/1", "tags": []any{}}
}

func readingSetup(t *testing.T) (*fixture, *fakeReadwise, site.Connector) {
	t.Helper()
	f := setup(t)
	src := &fakeReadwise{full: [][]map[string]any{
		{book(1, "Walden", []string{"nature", "Classic Books"}, highlight(10, "Simplify, simplify.", "my motto", 12, "page"), highlight(11, "I went to the woods.", "", 3, "page"))},
		{book(2, "Podcast", nil, highlight(20, "Listen.", "", 200, "time_offset"))},
	}}
	srv := httptest.NewServer(src)
	t.Cleanup(srv.Close)
	base := f.r.getenv
	f.r.getenv = func(k string) string {
		if k == TokenVar("reading") {
			return "secret"
		}
		return base(k)
	}
	var waited []time.Duration
	orig := pause
	pause = func(_ context.Context, d time.Duration) error { waited = append(waited, d); return nil }
	t.Cleanup(func() {
		pause = orig
		if len(waited) == 0 {
			t.Error("the run did not wait when asked to")
		}
	})
	if err := f.db.Write(func(s *item.Store) error {
		return s.DefineType("admin", item.TypeDef{Name: "book", Body: true, Fields: []item.FieldDef{{Name: "author", Kind: item.KindText}, {Name: "cover", Kind: item.KindURL}}})
	}); err != nil {
		t.Fatal(err)
	}
	c := site.ConnectorTemplates()[slices.IndexFunc(site.ConnectorTemplates(), func(c site.Connector) bool { return c.Kind == site.KindHighlights })]
	c.ID, c.Name, c.Space, c.Type, c.Highlights.URL, c.Highlights.Language = "reading", "Reading", "API", "book", srv.URL, "en"
	c.Fields = map[string]string{"author": "author", "cover": "cover"}
	c, err := f.db.Site.SetConnector("admin", c)
	if err != nil {
		t.Fatal(err)
	}
	return f, src, c
}

func itemOf(t *testing.T, f *fixture, id string) item.Item {
	t.Helper()
	var it item.Item
	var ok bool
	f.db.Read(func(s *item.Store) { it, ok = s.FindByOrigin("reading", f.cfgURL(t), "entry/"+id) })
	if !ok {
		t.Fatalf("no item of book %s", id)
	}
	return it
}

func (f *fixture) cfgURL(t *testing.T) string {
	c, _ := f.db.Site.Connector("reading")
	return c.Highlights.URL
}

func texts(it item.Item) []string {
	var out []string
	for _, b := range it.Body.Blocks {
		out = append(out, string(b.Kind)+":"+b.Text)
	}
	return out
}

// Each book becomes an item of the chosen type with its fields and tags;
// its highlights are quotes with their notes and places above a line;
// a later run reads what changed, merges it and keeps what people wrote
// below the line (ADR-0212).
func TestHighlightsBecomeItems(t *testing.T) {
	f, src, _ := readingSetup(t)
	ctx := context.Background()
	first := f.clock
	res, err := f.r.Run(ctx, "reading", false, "")
	if err != nil || res.New != 2 || res.Datasets != 2 || !res.Since.IsZero() {
		t.Fatalf("first run %+v %v", res, err)
	}
	if len(src.asked) != 2 || src.asked[1] != "pageCursor=2" {
		t.Fatalf("pages asked %q", src.asked)
	}
	walden := itemOf(t, f, "1")
	if walden.Type != "book" || walden.Title != "Walden" || walden.Properties["author"] != "Ann Author" || walden.Properties["cover"] != "https://img.example/Walden.jpg" {
		t.Fatalf("walden %+v", walden)
	}
	if !slices.Equal(walden.Labels, []string{"classic-books", "nature"}) {
		t.Errorf("labels %v", walden.Labels)
	}
	got := texts(walden)
	want := []string{"quote:I went to the woods.", "paragraph:Page 3", "quote:Simplify, simplify.", "paragraph:my motto", "paragraph:Page 12", "divider:",
		"panel:Write your own notes below this line: a sync keeps them."}
	if len(got) < len(want) || !slices.Equal(got[len(got)-len(want):], want) || !strings.HasPrefix(got[0], "panel:These highlights are written from") {
		t.Fatalf("walden's text %q", got)
	}
	if podcast := texts(itemOf(t, f, "2")); !slices.Contains(podcast, "paragraph:At 03:20") {
		t.Errorf("podcast %q", podcast)
	}

	// People write below the line and add a label.
	body := append(slices.Clone(walden.Body.Blocks), item.Block{Kind: item.Paragraph, Text: "Read again in spring."})
	if err := f.db.Write(func(s *item.Store) error {
		if err := s.CommitBody("admin", walden.ID, item.Doc{Blocks: body}); err != nil {
			return err
		}
		return s.SetLabels("admin", walden.ID, append(walden.Labels, "mine"))
	}); err != nil {
		t.Fatal(err)
	}

	// The next day the export gives Walden with one new highlight and one
	// deleted.
	f.clock = first.Add(24 * time.Hour)
	deleted := highlight(11, "", "", 3, "page")
	deleted["is_deleted"] = true
	src.changed = []map[string]any{book(1, "Walden", []string{"nature"}, highlight(12, "Heaven is under our feet.", "", 40, "page"), deleted)}
	res, err = f.r.Run(ctx, "reading", false, "")
	if err != nil || res.Changed != 1 || res.Withdrawn != 0 || !res.Since.Equal(first.Add(-sinceOverlap).UTC()) {
		t.Fatalf("second run %+v %v", res, err)
	}
	if !strings.HasPrefix(src.asked[len(src.asked)-1], "updatedAfter=") {
		t.Errorf("asked %q", src.asked)
	}
	walden = itemOf(t, f, "1")
	got = texts(walden)
	if slices.Contains(got, "quote:I went to the woods.") || !slices.Contains(got, "quote:Simplify, simplify.") || !slices.Contains(got, "quote:Heaven is under our feet.") {
		t.Errorf("merged highlights %q", got)
	}
	if got[len(got)-1] != "paragraph:Read again in spring." || !slices.Contains(walden.Labels, "mine") {
		t.Errorf("what people wrote is lost: %q %v", got, walden.Labels)
	}
	// An export that changes nothing writes nothing.
	f.clock = first.Add(48 * time.Hour)
	if res, err := f.r.Run(ctx, "reading", false, ""); err != nil || res.Unchanged != 1 || res.Changed != 0 {
		t.Errorf("an unchanged export %+v %v", res, err)
	}

	// Where people removed the line, a change keeps their whole text below
	// the new one.
	walden = itemOf(t, f, "1")
	if err := f.db.Write(func(s *item.Store) error {
		return s.CommitBody("admin", walden.ID, item.Doc{Blocks: []item.Block{{Kind: item.Paragraph, Text: "Only mine."}}})
	}); err != nil {
		t.Fatal(err)
	}
	f.clock = first.Add(72 * time.Hour)
	src.changed = []map[string]any{book(1, "Walden", nil, highlight(13, "More.", "", 50, "page"))}
	res, _ = f.r.Run(ctx, "reading", false, "")
	got = texts(itemOf(t, f, "1"))
	if got[len(got)-1] != "paragraph:Only mine." || len(res.Problems) == 0 {
		t.Errorf("without the line %q %v", got, res.Problems)
	}
}

// Without its token, or with one the service refuses, a run reads
// nothing and says why.
func TestHighlightsNeedTheirToken(t *testing.T) {
	f, src, _ := readingSetup(t)
	ctx := context.Background()
	src.refuse = true
	if _, err := f.r.Run(ctx, "reading", false, ""); err == nil || !strings.Contains(err.Error(), TokenVar("reading")) {
		t.Errorf("a refused token: %v", err)
	}
	src.refuse = false
	if _, err := f.r.Run(ctx, "reading", false, ""); err != nil {
		t.Errorf("after waiting: %v", err)
	}
	f.r.getenv = allowPrivate(func(string) string { return "" })
	if _, err := f.r.Run(ctx, "reading", false, ""); !errors.Is(err, ErrNoToken) {
		t.Errorf("no token: %v", err)
	}
	if st := f.r.Status("reading"); st.CredentialVar != "OT_CONNECTOR_READING_TOKEN" {
		t.Errorf("status %+v", st)
	}
}

// A highlight's place is told as its kind of place reads, with its tags.
func TestAHighlightsPlace(t *testing.T) {
	w := wordsIn("en")
	for _, c := range []struct {
		h    rwHighlight
		want string
	}{
		{rwHighlight{Location: json.RawMessage("12"), LocationType: "page"}, "Page 12"},
		{rwHighlight{Location: json.RawMessage("1834"), LocationType: "location"}, "Location 1834"},
		{rwHighlight{Location: json.RawMessage("125"), LocationType: "time_offset"}, "At 02:05"},
		{rwHighlight{Location: json.RawMessage("3"), LocationType: "order", Tags: []rwTag{{"key"}, {" "}}}, "Position 3 · #key"},
		{rwHighlight{Location: json.RawMessage("null")}, ""},
	} {
		if got := whereIs(c.h, w); got != c.want {
			t.Errorf("%+v: %q, want %q", c.h, got, c.want)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := pause(ctx, time.Hour); err == nil {
		t.Error("a wait outlives its run")
	}
	if err := pause(context.Background(), time.Millisecond); err != nil {
		t.Error(err)
	}
	if webOnly("javascript:alert(1)") != "" || webOnly(" https://a.example/x ") != "https://a.example/x" {
		t.Error("addresses")
	}
}
