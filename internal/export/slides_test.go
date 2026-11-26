package export

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dbuchs7/orbithek/internal/item"
)

// A slide deck is named in every document by its caption, which is not
// written again below it; a document does not show the slides
// (ADR-draft-a-slide-deck-in-a-text).
func TestSlideDecksAreNamedInDocuments(t *testing.T) {
	page := Page{ID: "p", Title: "Review", Body: []item.Block{
		{Kind: item.Slides, Ref: "f", Text: "Board, 4 December"},
		{Kind: item.Slides, Ref: "g"},
	}}
	doc := Document{Pages: []Page{page}}
	md := markdownOf(t, doc)
	if !strings.Contains(md, "Slide deck: Board, 4 December") || strings.Count(md, "Board, 4 December") != 1 || !strings.Contains(md, "Slide deck\n") {
		t.Errorf("the Markdown:\n%s", md)
	}
	var h bytes.Buffer
	if err := HTML(&h, doc, "en"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.String(), "[Slide deck: Board, 4 December]") || strings.Contains(h.String(), "<figcaption>Board") {
		t.Errorf("the HTML:\n%s", h.String())
	}
	var w bytes.Buffer
	if err := DOCX(&w, doc); err != nil {
		t.Fatal(err)
	}
}
