package export

import (
	"strings"
	"testing"

	"github.com/dbuchs7/orbithek/internal/item"
)

// A search field in a text is named in every document by what it
// searches; another page is not named by its title, which the reader of
// the document may not read (ADR-draft-a-search-field-in-a-text).
func TestSearchFieldsAreNamedInDocuments(t *testing.T) {
	cases := map[string]item.Block{
		"Search field: the item's room":                     {Kind: item.SearchBlock},
		"Search field: room PLAT":                           {Kind: item.SearchBlock, Space: "PLAT"},
		"Search field: this page":                           {Kind: item.SearchBlock, Scope: item.ScopePage},
		"Search field: another page and the pages below it": {Kind: item.SearchBlock, Scope: item.ScopeTree, Ref: "x"},
		"Search field: the pages below this page":           {Kind: item.SearchBlock, Scope: item.ScopeBelow},
	}
	for want, b := range cases {
		if got := SearchName(b); got != want {
			t.Errorf("%+v: %q, want %q", b, got, want)
		}
	}
	md := markdownOf(t, Document{Pages: []Page{{ID: "p", Title: "Find", Body: []item.Block{{Kind: item.SearchBlock, Scope: item.ScopeTree}}}}})
	if want := "Search field: this page and the pages below it"; !strings.Contains(md, want) {
		t.Errorf("the Markdown lacks %q:\n%s", want, md)
	}
}
