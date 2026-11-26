package item

import (
	"errors"
	"strings"
	"testing"
)

// A slide deck in a text names an album folder or collection and has a
// plain caption (ADR-draft-a-slide-deck-in-a-text).
func TestASlideDeckNamesAnItemAndHasAPlainCaption(t *testing.T) {
	for i, b := range []Block{{Kind: Slides, Ref: "f"}, {Kind: Slides, Ref: "f", Text: "Board, 4 December"}} {
		if err := checkBlock(b, "self"); err != nil {
			t.Errorf("good %d: %v", i, err)
		}
	}
	bad := map[string]struct {
		b    Block
		want string
	}{
		"no reference":        {Block{Kind: Slides}, "names an album folder"},
		"itself":              {Block{Kind: Slides, Ref: "self"}, "names an album folder"},
		"marks":               {Block{Kind: Slides, Ref: "f", Text: "Board", Marks: []Mark{{Type: MarkBold, Start: 0, End: 5}}}, "plain caption"},
		"mentions":            {Block{Kind: Slides, Ref: "f", Mentions: []ID{"x"}}, "plain caption"},
		"aligned":             {Block{Kind: Slides, Ref: "f", Align: "center"}, "no alignment"},
		"reference elsewhere": {Block{Kind: Paragraph, Text: "x", Ref: "f"}, "slide deck"},
	}
	for name, c := range bad {
		if err := checkBlock(c.b, "self"); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want an error with %q", name, err, c.want)
		}
	}
	if to, ok := (Block{Kind: Slides, Ref: "f"}).Embedded(); !ok || to != "f" {
		t.Errorf("a slide deck embeds its folder: %q %v", to, ok)
	}
	if _, ok := (Block{Kind: Image, Ref: "a"}).Embedded(); ok {
		t.Error("an image embeds no item")
	}
}

// A slide deck shows a folder or a collection, nothing else; it implies
// the relation embeds, so the folder stays while a text shows it (D8),
// and only who reads the folder may place it in a text (D10).
func TestASlideDeckShowsAFolderOrCollectionAndEmbedsIt(t *testing.T) {
	s := setup(t)
	noErr(t, s.GrantRole("admin", "PLAT", "anna", Editor))
	noErr(t, s.GrantRole("admin", "PLAT", "bert", Editor))
	noErr(t, s.GrantRole("admin", "OPS", "anna", Editor))
	anna, bert := s.As("anna"), s.As("bert")
	page := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Review"}))
	folder := mustItem(t)(anna.CreateFolder("PLAT", "", "Talk"))
	coll := mustItem(t)(anna.CreateCollection("PLAT", "", "Logos", CollectionRule{Labels: []string{"logo"}}))
	other := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Other"}))
	hidden := mustItem(t)(anna.CreateFolder("OPS", "", "Secret talk"))

	noErr(t, anna.CommitBody(page.ID, Doc{Blocks: []Block{{Kind: Slides, Ref: folder.ID, Text: "Board"}, {Kind: Slides, Ref: coll.ID}}}))
	embeds := 0
	for _, r := range s.Relations(page.ID, Outgoing) {
		if r.Type == RelEmbeds && r.Derived && (r.To == folder.ID || r.To == coll.ID) {
			embeds++
		}
	}
	if embeds != 2 {
		t.Errorf("the decks imply %d embeds, want 2", embeds)
	}
	if err := anna.CommitBody(page.ID, Doc{Blocks: []Block{{Kind: Slides, Ref: other.ID}}}); err == nil || !strings.Contains(err.Error(), "no album folder") {
		t.Errorf("a page is no deck: %v", err)
	}
	if err := anna.CommitBody(page.ID, Doc{Blocks: []Block{{Kind: Slides, Ref: "nothing"}}}); !errors.Is(err, ErrNotFound) {
		t.Errorf("an unknown folder: %v", err)
	}
	if err := anna.Trash(folder.ID); !errors.Is(err, ErrReferenced) {
		t.Errorf("a folder a text shows is not trashed: %v", err)
	}
	if err := bert.CommitBody(page.ID, Doc{Blocks: []Block{{Kind: Slides, Ref: hidden.ID}}}); !errors.Is(err, ErrNotFound) {
		t.Errorf("a folder bert does not read is not found for him: %v", err)
	}
	// A new item may start with a deck too.
	if _, err := anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Copy", Body: &Doc{Blocks: []Block{{Kind: Slides, Ref: other.ID}}}}); err == nil {
		t.Error("a new page with a deck of a page")
	}
	mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Second", Body: &Doc{Blocks: []Block{{Kind: Slides, Ref: folder.ID}}}}))
	if bodyEmpty(Doc{Blocks: []Block{{Kind: Slides, Ref: folder.ID}}}) {
		t.Error("a body with a deck is not empty")
	}
}
