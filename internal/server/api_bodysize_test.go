package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/dbuchs7/orbithek/internal/budget"
)

// longBlocks returns the JSON array of a body of plain paragraphs that is
// at least n bytes long, like a long article.
func longBlocks(n int) string {
	para := `{"kind":"paragraph","text":"` + strings.Repeat("A long article has many sentences of ordinary prose. ", 20) + `"}`
	var b strings.Builder
	b.WriteString("[")
	for b.Len() < n {
		if b.Len() > 1 {
			b.WriteString(",")
		}
		b.WriteString(para)
	}
	b.WriteString("]")
	return b.String()
}

// TestALongBodyIsSavedBeyondTheRequestBudget: a body larger than the
// budget of an ordinary request is saved by every route that carries a
// body, over the API and over MCP, up to BodyRequestBytes (ADR-0015).
func TestALongBodyIsSavedBeyondTheRequestBudget(t *testing.T) {
	w := newWorld(t)
	blocks := longBlocks(int(budget.Get().RequestBody) * 3)
	if int64(len(blocks)) >= budget.Get().BodyRequestBytes {
		t.Fatalf("the test body (%d bytes) does not fit the default body budget", len(blocks))
	}

	created := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"Long","properties":{"priority":"low"},"body":`+blocks+`}`, http.StatusCreated)
	key := field(created, "item", "key").(string)
	w.bert.expect("PUT", "/api/items/"+key+"/body", `{"blocks":`+blocks+`}`, http.StatusOK)
	w.bert.expect("POST", "/api/items/"+key+"/body/blocks", `{"blocks":`+blocks+`}`, http.StatusOK)

	if text, isErr := w.bert.tool("commit_body", `{"ref":"`+key+`","blocks":`+blocks+`}`); isErr {
		t.Fatalf("commit_body over MCP: %.200s", text)
	}
	if text, isErr := w.bert.tool("append_to_body", `{"ref":"`+key+`","blocks":`+blocks+`}`); isErr {
		t.Fatalf("append_to_body over MCP: %.200s", text)
	}
	got := w.bert.expect("GET", "/api/items/"+key, "", http.StatusOK)
	var want []any
	if err := json.Unmarshal([]byte(blocks), &want); err != nil {
		t.Fatal(err)
	}
	if n := len(field(got, "item", "body").([]any)); n != 2*len(want) {
		t.Fatalf("the body holds %d blocks, want %d", n, 2*len(want))
	}

	// Routes that carry no body keep the budget of an ordinary request.
	long := strings.Repeat("a", int(budget.Get().RequestBody))
	w.bert.expect("PUT", "/api/items/"+key+"/title", `{"title":"`+long+`"}`, http.StatusRequestEntityTooLarge)
	w.bert.expect("POST", "/api/items/"+key+"/comments", `{"text":"`+long+`"}`, http.StatusRequestEntityTooLarge)
}

// TestABodyBeyondItsBudgetIsRefused: BodyRequestBytes is a ceiling like
// every budget; a body beyond it is answered 413, over MCP too.
func TestABodyBeyondItsBudgetIsRefused(t *testing.T) {
	w := newWorld(t)
	b := budget.Default()
	b.BodyRequestBytes = 2 * b.RequestBody
	budget.Set(b)
	t.Cleanup(func() { budget.Set(budget.Default()) })

	created := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"Long","properties":{"priority":"low"}}`, http.StatusCreated)
	key := field(created, "item", "key").(string)
	fits := longBlocks(int(b.RequestBody) + 1)
	w.bert.expect("PUT", "/api/items/"+key+"/body", `{"blocks":`+fits+`}`, http.StatusOK)

	tooLong := longBlocks(int(b.BodyRequestBytes))
	w.bert.expect("PUT", "/api/items/"+key+"/body", `{"blocks":`+tooLong+`}`, http.StatusRequestEntityTooLarge)
	w.bert.expect("POST", "/api/items/"+key+"/body/blocks", `{"blocks":`+tooLong+`}`, http.StatusRequestEntityTooLarge)
	w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"x","properties":{"priority":"low"},"body":`+tooLong+`}`, http.StatusRequestEntityTooLarge)
	bigBlock := `{"kind":"paragraph","text":"` + strings.Repeat("a", int(b.BodyRequestBytes)) + `"}`
	w.bert.expect("POST", "/api/items/"+key+"/live", `{"client":"c","ops":[{"op":"set","id":"x","block":`+bigBlock+`}]}`, http.StatusRequestEntityTooLarge)
	rec := w.bert.do("POST", MCPPath, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"commit_body","arguments":{"ref":"`+key+`","blocks":`+tooLong+`}}}`)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("an MCP message beyond the body budget: status %d", rec.Code)
	}
}
