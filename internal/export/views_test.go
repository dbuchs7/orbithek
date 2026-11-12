package export

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/dbuchs7/orbithek/internal/item"
)

// A view of items in a text is named in every document, as a diagram is:
// a document does not compute it (ADR-0203).
func TestViewsAreNamedInDocuments(t *testing.T) {
	page := Page{ID: "p", Title: "Overview", Body: []item.Block{
		{Kind: item.ItemsView, Layout: item.ViewGallery, ItemType: "task", Space: "PLAT", Query: "state = open"},
		{Kind: item.ItemsView, Layout: item.ViewList},
	}}
	doc := Document{Pages: []Page{page}}
	md := markdownOf(t, doc)
	for _, want := range []string{"View: gallery of task in PLAT where state = open", "View: list of every type in every room"} {
		if !strings.Contains(md, want) {
			t.Errorf("the Markdown lacks %q:\n%s", want, md)
		}
	}
	var h bytes.Buffer
	if err := HTML(&h, doc, "en"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.String(), "[View: gallery of task in PLAT where state = open]") {
		t.Errorf("the HTML lacks the view:\n%s", h.String())
	}
	var w bytes.Buffer
	if err := DOCX(&w, doc); err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(w.Bytes()), int64(w.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range z.File {
		if f.Name == "word/document.xml" {
			r, _ := f.Open()
			data, _ := io.ReadAll(r)
			if !strings.Contains(string(data), "[View: list of every type in every room]") {
				t.Errorf("the Word document lacks the view")
			}
		}
	}
	if got := ViewName(item.Block{Kind: item.ItemsView}); got != "View: table of every type in every room" {
		t.Errorf("default %q", got)
	}
}
