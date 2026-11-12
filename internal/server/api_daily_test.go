package server

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/dbuchs7/orbithek/internal/budget"
	"github.com/dbuchs7/orbithek/internal/item"
	"github.com/dbuchs7/orbithek/internal/storage"
)

func TestADailyNoteThroughTheAPI(t *testing.T) {
	w := newWorld(t)
	b := w.bert
	// Without a room chosen, nothing is made, and the answer says why.
	if got := b.expect("GET", "/api/daily?from=2026-11-01&to=2026-11-30", "", http.StatusOK); got["space"] != "" || len(got["days"].([]any)) != 0 {
		t.Fatalf("before: %v", got)
	}
	if rec := b.do("POST", "/api/daily/2026-11-12", ""); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), `"needsRoom":true`) {
		t.Fatalf("without a room: %d %s", rec.Code, rec.Body)
	}
	// A room one may not create in is not taken; one without a role is
	// not found.
	w.vera.expect("PUT", "/api/daily/room", `{"space":"PLAT"}`, http.StatusForbidden)
	w.carla.expect("PUT", "/api/daily/room", `{"space":"PLAT"}`, http.StatusNotFound)
	b.expect("PUT", "/api/daily/room", `{"space":"NOPE"}`, http.StatusNotFound)
	b.expect("PUT", "/api/daily/room", `{"space":"PLAT"}`, http.StatusOK)

	b.expect("GET", "/api/daily/2026-11-12", "", http.StatusNotFound)
	b.expect("GET", "/api/daily/2026-13-12", "", http.StatusBadRequest)
	b.expect("POST", "/api/daily/yesterday", "", http.StatusBadRequest)

	// Many requests at once make one note.
	var wg sync.WaitGroup
	ids := make([]string, 8)
	for i := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got := b.expect("POST", "/api/daily/2026-11-12", `{"title":"Thursday, 12 November"}`, http.StatusOK)
			ids[i] = field(got, "item", "id").(string)
		}()
	}
	wg.Wait()
	for _, id := range ids {
		if id != ids[0] {
			t.Fatalf("two notes for one day: %v", ids)
		}
	}
	got := b.expect("GET", "/api/daily/2026-11-12", "", http.StatusOK)
	if field(got, "item", "title") != "Thursday, 12 November" || field(got, "item", "properties", "date") != "2026-11-12T00:00:00Z" || got["editable"] != true {
		t.Fatalf("note %v", got)
	}
	if r := field(got, "item", "restriction").(map[string]any); fmt.Sprint(r["read"]) != "["+w.ids["bert"]+"]" || fmt.Sprint(r["edit"]) != "["+w.ids["bert"]+"]" {
		t.Fatalf("restriction %v", r)
	}
	key := field(got, "item", "key").(string)
	again := b.expect("POST", "/api/daily/2026-11-12", "", http.StatusOK)
	if again["created"] != false || field(again, "item", "key") != key {
		t.Fatalf("again %v", again)
	}
	// Nobody else reads it.
	w.vera.expect("GET", "/api/items/"+key, "", http.StatusNotFound)
	w.admin.expect("GET", "/api/daily/2026-11-12", "", http.StatusNotFound)

	// A paragraph is appended, making the note of another day first.
	app := b.expect("POST", "/api/daily/2026-11-13/paragraphs", `{"text":"Call the vendor."}`, http.StatusOK)
	if app["created"] != true || field(app, "item", "title") != "2026-11-13" {
		t.Fatalf("append %v", app)
	}
	app = b.expect("POST", "/api/daily/2026-11-13/paragraphs", `{"text":"Order parts."}`, http.StatusOK)
	if body := field(app, "item", "body").([]any); len(body) != 2 || body[1].(map[string]any)["text"] != "Order parts." {
		t.Fatalf("body %v", body)
	}
	b.expect("POST", "/api/daily/2026-11-13/paragraphs", `{"text":"  "}`, http.StatusBadRequest)
	b.expect("POST", "/api/daily/2026-11-13/paragraphs", `{"text":`, http.StatusBadRequest)
	b.expect("POST", "/api/daily/2026-13-13/paragraphs", `{"text":"x"}`, http.StatusBadRequest)
	b.expect("POST", "/api/daily/2026-11-13", `{"title":`, http.StatusBadRequest)
	b.expect("PUT", "/api/daily/room", `{"space":`, http.StatusBadRequest)

	days := b.expect("GET", "/api/daily?from=2026-11-01&to=2026-11-30", "", http.StatusOK)
	if days["space"] != "PLAT" || fmt.Sprint(days["days"]) != "[2026-11-12 2026-11-13]" || days["mayCreate"] != true {
		t.Fatalf("days %v", days)
	}
	b.expect("GET", "/api/daily?from=2026-11-30&to=2026-11-01", "", http.StatusBadRequest)
	b.expect("GET", "/api/daily?from=2020-01-01&to=2026-11-01", "", http.StatusBadRequest)
	b.expect("GET", "/api/daily?from=x", "", http.StatusBadRequest)

	// A read token reads but does not write.
	tok := b.expect("POST", "/api/tokens", `{"name":"reader","scope":"read","days":1}`, http.StatusCreated)
	reader := &client{t: t, h: b.h, bearer: tok["secret"].(string)}
	reader.expect("GET", "/api/daily/2026-11-12", "", http.StatusOK)
	reader.expect("POST", "/api/daily/2026-11-14", "", http.StatusForbidden)

	// A room one lost the right to create in refuses new notes.
	w.admin.expect("PUT", "/api/spaces/PLAT/roles/"+w.ids["bert"], `{"role":"viewer"}`, http.StatusNoContent)
	b.expect("POST", "/api/daily/2026-11-15", "", http.StatusForbidden)
	if days := b.expect("GET", "/api/daily?from=2026-11-01&to=2026-11-30", "", http.StatusOK); days["mayCreate"] != false {
		t.Fatalf("mayCreate %v", days)
	}
	b.expect("PUT", "/api/daily/room", `{"space":""}`, http.StatusOK)
}

func TestADailyNoteIsNotMadeAtCriticalStorage(t *testing.T) {
	w := newWorld(t)
	w.bert.expect("PUT", "/api/daily/room", `{"space":"PLAT"}`, http.StatusOK)
	set := func(f item.NothingNew) {
		_ = w.db.Write(func(s *item.Store) error { s.SetNothingNew(f); return nil })
	}
	set(func() error { return storage.ErrNothingNew })
	t.Cleanup(func() { set(nil) })
	w.bert.expect("POST", "/api/daily/2026-11-12", "", http.StatusInsufficientStorage)
	w.bert.expect("POST", "/api/daily/2026-11-12/paragraphs", `{"text":"x"}`, http.StatusInsufficientStorage)
}

func TestTheContextOfAnItemThroughTheAPI(t *testing.T) {
	w := newWorld(t)
	b := w.bert
	w.admin.expect("POST", "/api/types", `{"name":"page","body":true}`, http.StatusCreated)
	target := b.expect("POST", "/api/items", `{"space":"PLAT","type":"page","title":"Runbook"}`, http.StatusCreated)
	id := field(target, "item", "id").(string)
	b.expect("PUT", "/api/items/"+id+"/labels", `{"labels":["ops"]}`, http.StatusOK)
	src := b.expect("POST", "/api/items", `{"space":"PLAT","type":"page","title":"Incident","body":[{"kind":"paragraph","text":"It broke. Use the Runbook now. Done.",
		"mentions":["`+id+`"],"marks":[{"type":"mention","start":18,"end":25,"ref":"`+id+`"}]}]}`, http.StatusCreated)
	b.expect("PUT", "/api/items/"+field(src, "item", "id").(string)+"/labels", `{"labels":["ops","x"]}`, http.StatusOK)
	b.expect("PUT", "/api/daily/room", `{"space":"PLAT"}`, http.StatusOK)
	b.expect("POST", "/api/daily/2026-11-12/paragraphs", `{"text":"x"}`, http.StatusOK)
	note := b.expect("GET", "/api/daily/2026-11-12", "", http.StatusOK)
	b.expect("PUT", "/api/items/"+field(note, "item", "id").(string)+"/body", `{"blocks":[{"kind":"paragraph","text":"Read Runbook.","mentions":["`+id+`"],
		"marks":[{"type":"mention","start":5,"end":12,"ref":"`+id+`"}]}]}`, http.StatusOK)

	c := b.expect("GET", "/api/items/"+id+"/context", "", http.StatusOK)
	refs := c["references"].([]any)
	if len(refs) != 1 || field(refs[0].(map[string]any), "item", "title") != "Incident" || refs[0].(map[string]any)["sentence"] != "Use the Runbook now." || refs[0].(map[string]any)["kind"] != "mentions" {
		t.Fatalf("references %v", refs)
	}
	if sh := c["shared"].([]any); len(sh) != 1 || fmt.Sprint(sh[0].(map[string]any)["labels"]) != "[ops]" {
		t.Fatalf("shared %v", sh)
	}
	if d := c["daily"].([]any); len(d) != 1 || d[0].(map[string]any)["date"] != "2026-11-12" || d[0].(map[string]any)["sentence"] != "Read Runbook." {
		t.Fatalf("daily %v", d)
	}
	// vera reads the page, not bert's daily note.
	c = w.vera.expect("GET", "/api/items/"+id+"/context", "", http.StatusOK)
	if d := c["daily"].([]any); len(d) != 0 {
		t.Fatalf("vera sees bert's daily note: %v", d)
	}
	w.carla.expect("GET", "/api/items/"+id+"/context", "", http.StatusNotFound)

	small := budget.Default()
	small.ContextEntries = 1
	budget.Set(small)
	t.Cleanup(func() { budget.Set(budget.Default()) })
	b.expect("POST", "/api/items", `{"space":"PLAT","type":"page","title":"Second","body":[{"kind":"paragraph","text":"Runbook","mentions":["`+id+`"]}]}`, http.StatusCreated)
	c = b.expect("GET", "/api/items/"+id+"/context", "", http.StatusOK)
	if len(c["references"].([]any)) != 1 || field(c, "more", "references") != true {
		t.Fatalf("bounded: %v", c)
	}
}

func TestDailyNotesAndContextOverMCP(t *testing.T) {
	w := newWorld(t)
	if text, isErr := w.bert.tool("open_daily_note", `{"date":"2026-11-12"}`); !isErr || !strings.Contains(text, "409") {
		t.Fatalf("without a room: %v %s", isErr, text)
	}
	w.bert.expect("PUT", "/api/daily/room", `{"space":"PLAT"}`, http.StatusOK)
	text, isErr := w.bert.tool("open_daily_note", `{"date":"2026-11-12","title":"From an agent"}`)
	if isErr || !strings.Contains(text, `"created":true`) || !strings.Contains(text, "From an agent") {
		t.Fatalf("open: %v %s", isErr, text)
	}
	if text, _ = w.bert.tool("open_daily_note", `{"date":"2026-11-12"}`); !strings.Contains(text, `"created":false`) {
		t.Fatalf("again: %s", text)
	}
	key := w.bert.expect("GET", "/api/daily/2026-11-12", "", http.StatusOK)["item"].(map[string]any)["key"].(string)
	if text, isErr = w.bert.tool("get_context", `{"ref":"`+key+`"}`); isErr || !strings.Contains(text, `"references":[]`) {
		t.Fatalf("context: %v %s", isErr, text)
	}
	if text, isErr = w.vera.tool("get_context", `{"ref":"`+key+`"}`); !isErr || !strings.Contains(text, "404") {
		t.Fatalf("vera reads bert's note's context: %v %s", isErr, text)
	}
}
