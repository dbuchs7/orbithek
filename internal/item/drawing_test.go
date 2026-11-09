package item

import (
	"errors"
	"strings"
	"testing"
)

func TestDrawingVersions(t *testing.T) {
	s := permFixture(t)
	page := mustItem(t)(s.Create("anna", NewItem{Space: "PLAT", Type: "page", Title: "Workshop"}))
	file := func(id string, src string) Attachment {
		a := Attachment{ID: ID(id), Name: "board.drawing.png", Type: "image/png", Size: 3, SHA256: strings.Repeat("ab", 32)}
		if src != "" {
			a.Source = &Source{Item: page.ID, Attachment: ID(src)}
		}
		return a
	}
	const v1, v2, v3, other = "00000000000000000000000000000001", "00000000000000000000000000000002", "00000000000000000000000000000003", "0000000000000000000000000000000f"
	for _, a := range []Attachment{file(v1, ""), file(v2, v1), file(other, ""), file(v3, v2)} {
		if _, err := s.AddAttachment("anna", page.ID, a); err != nil {
			t.Fatal(err)
		}
	}
	noErr(t, s.DeleteAttachment("anna", page.ID, v1))
	for _, from := range []ID{v1, v2, v3} {
		d, err := s.Drawing(page.ID, from)
		if err != nil || d.First != v1 || d.Newest.ID != v3 || len(d.Versions) != 3 {
			t.Fatalf("from %s: %+v %v", from, d, err)
		}
	}
	if d, err := s.As("ben").Drawing(page.ID, v2); err != nil || d.Newest.ID != v3 {
		t.Fatalf("a reader: %+v %v", d, err)
	}
	if _, err := s.As("nobody").Drawing(page.ID, v2); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a stranger: %v", err)
	}
	if _, err := s.Drawing(page.ID, "0000000000000000000000000000000e"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("no such file: %v", err)
	}
	if _, err := s.As("anna").Drawing(page.ID, "0000000000000000000000000000000e"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("no such file for a reader: %v", err)
	}
	noErr(t, s.DeleteAttachment("anna", page.ID, other))
	if _, err := s.Drawing(page.ID, other); !errors.Is(err, ErrNotFound) {
		t.Fatalf("every version deleted: %v", err)
	}
	if _, err := s.Drawing("nope", v1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("no item: %v", err)
	}
}

func TestCommentOnAnElementOfADrawing(t *testing.T) {
	s := setup(t)
	page := mustItem(t)(s.Create("anna", NewItem{Space: "PLAT", Type: "page", Title: "Workshop"}))
	const v1 = "00000000000000000000000000000001"
	if _, err := s.AddAttachment("anna", page.ID, Attachment{ID: v1, Name: "b.drawing.png", Type: "image/png", Size: 3, SHA256: strings.Repeat("ab", 32)}); err != nil {
		t.Fatal(err)
	}
	c, err := s.Comment("anna", page.ID, NewComment{Text: "?", Anchor: &Anchor{Drawing: v1, Element: "el_1", Prefix: "x", Suffix: "y"}})
	if err != nil || c.Anchor.Element != "el_1" || c.Anchor.Drawing != v1 || c.Anchor.Prefix != "" || c.Anchor.Quote != "" {
		t.Fatalf("comment = %+v %v", c.Anchor, err)
	}
	for name, a := range map[string]Anchor{
		"bad element": {Drawing: v1, Element: "a b"},
		"no element":  {Drawing: v1},
		"no drawing":  {Element: "el"},
		"other file":  {Drawing: "0000000000000000000000000000000e", Element: "el"},
	} {
		if _, err := s.Comment("anna", page.ID, NewComment{Text: "?", Anchor: &a}); !errors.Is(err, ErrValidation) {
			t.Errorf("%s: %v", name, err)
		}
	}
}
