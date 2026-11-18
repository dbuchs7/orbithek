package server

import (
	"net/http"
	"testing"
)

// TestATypeNumbersTheHeadingsOfItsNewItems: a type may say that its new
// items begin with their headings numbered, with or without start text;
// a request may say otherwise, a copy keeps its original's, and a type
// without a body refuses it (ADR-0241).
func TestATypeNumbersTheHeadingsOfItsNewItems(t *testing.T) {
	w := newWorld(t)
	got := w.admin.expect("POST", "/api/types", `{"name":"concept","body":true,"numbered":true}`, http.StatusCreated)
	if field(got, "type", "numbered") != true {
		t.Fatalf("the type does not say it numbers: %v", got)
	}
	w.admin.expect("POST", "/api/types", `{"name":"plain","body":true,"startMarkdown":"# Goals","numbered":true}`, http.StatusCreated)
	w.admin.expect("POST", "/api/types", `{"name":"counter","body":false,"numbered":true}`, http.StatusBadRequest)
	numbered := func(item map[string]any) bool {
		t.Helper()
		v, _ := field(item, "item", "numbered").(bool)
		return v
	}

	// Without a text, with a text, and against the type's choice.
	if it := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"concept","title":"Empty"}`, http.StatusCreated); !numbered(it) {
		t.Fatalf("a new item without a text is not numbered: %v", it)
	}
	if it := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"concept","title":"Given","body":[{"kind":"heading","level":1,"text":"Aims"}]}`, http.StatusCreated); !numbered(it) {
		t.Fatalf("a new item with a text is not numbered: %v", it)
	}
	if it := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"concept","title":"Unnumbered","numbered":false}`, http.StatusCreated); numbered(it) {
		t.Fatalf("an item asked not to be numbered is: %v", it)
	}
	start := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"plain","title":"Started"}`, http.StatusCreated)
	if body := field(start, "item", "body").([]any); !numbered(start) || len(body) != 1 {
		t.Fatalf("the start text and its numbering: %v", start)
	}
	// A type that does not number leaves new items unnumbered unless asked.
	w.admin.expect("POST", "/api/types", `{"name":"note","body":true}`, http.StatusCreated)
	if it := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"note","title":"Quiet"}`, http.StatusCreated); numbered(it) {
		t.Fatalf("a type that does not number numbered: %v", it)
	}
	if it := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"note","title":"Asked","numbered":true}`, http.StatusCreated); !numbered(it) {
		t.Fatalf("an item asked to be numbered is not: %v", it)
	}

	// The type gives its choice back as it was given, and loses it when
	// redefined without it.
	types := w.bert.expect("GET", "/api/types", "", http.StatusOK)["types"].([]any)
	seen := false
	for _, ty := range types {
		if m := ty.(map[string]any); m["name"] == "concept" {
			seen = m["numbered"] == true
		}
	}
	if !seen {
		t.Fatalf("the type list does not say concept numbers: %v", types)
	}
	w.admin.expect("POST", "/api/types", `{"name":"concept","body":true}`, http.StatusCreated)
	if it := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"concept","title":"Later"}`, http.StatusCreated); numbered(it) {
		t.Fatalf("a type redefined without numbering still numbers: %v", it)
	}
}
