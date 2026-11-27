package item

import (
	"errors"
	"slices"
	"testing"
)

// A search field in a text names what it searches: a room, or a page,
// the page with the pages below it, or the pages below it only
// (ADR-draft-a-search-field-in-a-text).
func TestASearchBlockNamesItsRoomOrPageAndScope(t *testing.T) {
	good := []Block{
		{Kind: SearchBlock},
		{Kind: SearchBlock, Space: "PLAT"},
		{Kind: SearchBlock, Scope: ScopePage},
		{Kind: SearchBlock, Scope: ScopeTree, Ref: "other"},
		{Kind: SearchBlock, Scope: ScopeBelow, Ref: "other"},
	}
	for i, b := range good {
		if err := checkBlock(b, "self"); err != nil {
			t.Errorf("good %d: %v", i, err)
		}
	}
	bad := map[string]struct {
		b    Block
		want string
	}{
		"text":              {Block{Kind: SearchBlock, Text: "find"}, "no text"},
		"mentions":          {Block{Kind: SearchBlock, Mentions: []ID{"x"}}, "no text"},
		"unknown scope":     {Block{Kind: SearchBlock, Scope: "children"}, "scope"},
		"room with a page":  {Block{Kind: SearchBlock, Ref: "other"}, "names the room"},
		"page with a room":  {Block{Kind: SearchBlock, Scope: ScopeTree, Space: "PLAT"}, "names the page"},
		"bad room":          {Block{Kind: SearchBlock, Space: "plat"}, "room"},
		"itself":            {Block{Kind: SearchBlock, Scope: ScopeTree, Ref: "self"}, "as none"},
		"scope elsewhere":   {Block{Kind: Paragraph, Scope: ScopeTree}, "only a search field"},
		"ref elsewhere":     {Block{Kind: Paragraph, Ref: "other"}, "reference"},
		"query on a search": {Block{Kind: SearchBlock, Query: "state = open"}, "query"},
		"label on a search": {Block{Kind: SearchBlock, Label: "x"}, "only a starmap"},
	}
	for name, c := range bad {
		isInvalid(t, name, checkBlock(c.b, "self"), c.want)
	}
}

// A search field is kept in the body and replayed as written; the page it
// names is mentioned, so it must be readable when written and is not
// deleted while named (D8).
func TestASearchBlockIsKeptAndMentionsItsPage(t *testing.T) {
	s := permFixture(t)
	anna := s.As("anna")
	handbook := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Handbook"}))
	page := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Find"}))
	b := Block{Kind: SearchBlock, Scope: ScopeTree, Ref: handbook.ID}
	noErr(t, anna.CommitBody(page.ID, Doc{Blocks: []Block{{Kind: Paragraph, Text: "Search the handbook"}, b, {Kind: SearchBlock}}}))
	got := mustItem(t)(anna.Get(page.ID)).Body.Blocks[1]
	if got.Kind != SearchBlock || got.Scope != ScopeTree || got.Ref != handbook.ID {
		t.Errorf("kept %+v", got)
	}
	re, err := Replay(s.Log())
	noErr(t, err)
	if back := mustItem(t)(re.As("anna").Get(page.ID)).Body.Blocks[1]; back.Scope != ScopeTree || back.Ref != handbook.ID {
		t.Errorf("replayed %+v", back)
	}
	mentioned := false
	for _, r := range s.Relations(handbook.ID, Incoming) {
		mentioned = mentioned || r.Derived && r.Type == RelMentions && r.From == page.ID
	}
	if !mentioned {
		t.Error("the search field does not mention the page it searches within")
	}
	if err := anna.Trash(handbook.ID); err == nil {
		t.Error("a page a search field names was trashed")
	}
	if err := anna.CommitBody(page.ID, Doc{Blocks: []Block{{Kind: SearchBlock, Scope: ScopePage, Ref: "nothing"}}}); !errors.Is(err, ErrNotFound) {
		t.Errorf("an unknown page: %v", err)
	}
}

// A search narrowed to a page finds the page alone, the page with the
// pages below it, or the pages below it only, every level deep, and never
// a page beside it; a page the reader may not read is not found as a
// scope (D10) (ADR-draft-a-search-field-in-a-text).
func TestASearchIsNarrowedToAPageAndThePagesBelowIt(t *testing.T) {
	s := permFixture(t)
	anna := s.As("anna")
	mk := func(title string, parent ID) Item {
		return mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: title, Parent: parent}))
	}
	top := mk("Handbook rocket", "")
	child := mk("Chapter rocket", top.ID)
	grand := mk("Section rocket", child.ID)
	beside := mk("Elsewhere rocket", "")
	titles := func(scope string, within ID) []string {
		t.Helper()
		hits, err := anna.Search("rocket", Query{Within: within, Scope: scope})
		noErr(t, err)
		var out []string
		for _, h := range hits {
			out = append(out, h.Item.Title)
		}
		slices.Sort(out)
		return out
	}
	cases := map[string][]string{
		ScopePage:  {"Handbook rocket"},
		ScopeTree:  {"Chapter rocket", "Handbook rocket", "Section rocket"},
		ScopeBelow: {"Chapter rocket", "Section rocket"},
	}
	for scope, want := range cases {
		if got := titles(scope, top.ID); !slices.Equal(got, want) {
			t.Errorf("%s: %v, want %v", scope, got, want)
		}
	}
	if got := titles(ScopeBelow, grand.ID); len(got) != 0 {
		t.Errorf("below a page without pages below it: %v", got)
	}
	if got := titles("", ""); len(got) != 4 || !slices.Contains(got, beside.Title) {
		t.Errorf("unnarrowed: %v", got)
	}
	if _, err := anna.Search("rocket", Query{Scope: ScopeTree}); err == nil {
		t.Error("a scope without its page was taken")
	}
	if _, err := anna.Search("rocket", Query{Within: top.ID, Scope: "children"}); err == nil {
		t.Error("an unknown scope was taken")
	}
	noErr(t, anna.Restrict(child.ID, Restriction{Read: []string{"anna"}}))
	ben := s.As("ben")
	if _, err := ben.Search("rocket", Query{Within: child.ID, Scope: ScopeTree}); !errors.Is(err, ErrNotFound) {
		t.Errorf("an unreadable page as scope: %v", err)
	}
	hits, err := ben.Search("rocket", Query{Within: top.ID, Scope: ScopeTree})
	noErr(t, err)
	if len(hits) != 1 || hits[0].Item.ID != top.ID {
		t.Errorf("the reader finds what they may not read: %v", hits)
	}
}
