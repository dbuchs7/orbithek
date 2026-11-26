package export

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/dbuchs7/orbithek/internal/item"
)

// A starmap in a text is named in every document, as a view is: a
// document does not draw it (ADR-0252).
func TestStarmapsAreNamedInDocuments(t *testing.T) {
	page := Page{ID: "p", Title: "Overview", Body: []item.Block{
		{Kind: item.StarmapBlock, Space: "PLAT", Label: "onboarding", Search: "plan", HideTypes: []string{"task"}, HideKinds: []string{"done"}},
		{Kind: item.StarmapBlock, Depth: 2},
	}}
	doc := Document{Pages: []Page{page}}
	first := "Starmap of PLAT, label onboarding, matching plan, without task, done"
	second := "Starmap of the item's room around this item, depth 2"
	md := markdownOf(t, doc)
	for _, want := range []string{first, second} {
		if !strings.Contains(md, want) {
			t.Errorf("the Markdown lacks %q:\n%s", want, md)
		}
	}
	var h bytes.Buffer
	if err := HTML(&h, doc, "en"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.String(), "["+first+"]") {
		t.Errorf("the HTML lacks the starmap:\n%s", h.String())
	}
	var w bytes.Buffer
	if err := DOCX(&w, doc); err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(w.Bytes()), int64(w.Len()))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range z.File {
		if f.Name == "word/document.xml" {
			r, _ := f.Open()
			data, _ := io.ReadAll(r)
			found = strings.Contains(string(data), "["+second+"]")
		}
	}
	if !found {
		t.Errorf("the Word document lacks the starmap")
	}
	if got := StarmapName(item.Block{Kind: item.StarmapBlock, Space: item.StarmapAllRooms}); got != "Starmap of every room" {
		t.Errorf("every room %q", got)
	}
}
