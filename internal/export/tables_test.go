package export

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/dbuchs7/orbithek/internal/item"
)

// styledTable has formatted cells, a column width, a row's height and
// coloured cells (ADR-0214), and text in a tone and a size
// (ADR-0238).
var styledTable = item.Block{Kind: item.Table, Text: "Host\tRole\ndb1\t primary ",
	Marks: []item.Mark{{Type: item.MarkItalic, Start: 0, End: 4}, {Type: item.MarkBold, Start: 15, End: 22}, {Type: item.MarkLink, Start: 10, End: 13, Href: "https://db1.example.org"},
		{Type: item.MarkColor, Start: 5, End: 9, Value: "purple"}, {Type: item.MarkSize, Start: 5, End: 9, Value: "large"}},
	Widths: []int{200, 0}, Heights: []int{0, 40},
	Cells: []item.Cell{{Row: 0, Col: 1, Fill: "blue"}, {Row: 1, Col: 0, Fill: "yellow", Border: "red"}}}

func TestATableKeepsItsFormatsSizesAndColoursInHTML(t *testing.T) {
	var b bytes.Buffer
	if err := HTML(&b, Document{Pages: []Page{{ID: "p", Title: "Hosts", Body: []item.Block{styledTable}}}}, "en"); err != nil {
		t.Fatal(err)
	}
	h := b.String()
	for _, want := range []string{`<colgroup><col width="200"><col></colgroup>`, `<th><em>Host</em></th>`, `<th class="fill-blue"><span class="text-purple size-large">Role</span></th>`,
		`<tr height="40">`, `<td class="fill-yellow border-red"><a href="https://db1.example.org">db1</a></td>`, `<td><strong>primary</strong></td>`} {
		if !strings.Contains(h, want) {
			t.Errorf("the HTML lacks %q:\n%s", want, h)
		}
	}
	if strings.Contains(h, "style=") {
		t.Error("the export writes a style attribute, which its policy refuses")
	}
}

func TestATableKeepsItsFormatsInMarkdown(t *testing.T) {
	md := BlocksMarkdown([]item.Block{styledTable})
	for _, want := range []string{"| *Host* | Role |", "| [db1](https://db1.example.org) | **primary** |"} {
		if !strings.Contains(md, want) {
			t.Errorf("the Markdown lacks %q:\n%s", want, md)
		}
	}
}

func TestATableKeepsItsFormatsSizesAndColoursInWord(t *testing.T) {
	var buf bytes.Buffer
	if err := DOCX(&buf, Document{Pages: []Page{{ID: "p", Title: "Hosts", Body: []item.Block{styledTable}}}}); err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var doc string
	for _, f := range z.File {
		if f.Name == "word/document.xml" {
			r, _ := f.Open()
			data, _ := io.ReadAll(r)
			doc = string(data)
		}
	}
	for _, want := range []string{`<w:gridCol w:w="3000"/>`, `<w:tblLayout w:type="fixed"/>`, `<w:trHeight w:val="600" w:hRule="atLeast"/>`,
		`w:fill="D5E2F8"`, `w:fill="F6ECCD"`, `w:color="D93636"`, `<w:color w:val="8A4FD8"/><w:sz w:val="28"/><w:szCs w:val="28"/>`, `<w:i/>`, "<w:hyperlink", ">primary<"} {
		if !strings.Contains(doc, want) {
			t.Errorf("the document lacks %q", want)
		}
	}
}

// TestAParagraphKeepsItsColourAndSize: a paragraph's text in a tone and a
// size is written as classes of the export's stylesheet and as Word's run
// properties, as a cell's is (ADR-0238).
func TestAParagraphKeepsItsColourAndSize(t *testing.T) {
	para := item.Block{Kind: item.Paragraph, Text: "Stop the writers.", Marks: []item.Mark{{Type: item.MarkColor, Start: 0, End: 4, Value: "red"}, {Type: item.MarkSize, Start: 0, End: 4, Value: "small"}}}
	var h bytes.Buffer
	if err := HTML(&h, Document{Pages: []Page{{ID: "p", Title: "Writers", Body: []item.Block{para}}}}, "en"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.String(), `<span class="text-red size-small">Stop</span> the writers.`) {
		t.Errorf("the HTML lacks the paragraph's colour and size:\n%s", h.String())
	}
	var buf bytes.Buffer
	if err := DOCX(&buf, Document{Pages: []Page{{ID: "p", Title: "Writers", Body: []item.Block{para}}}}); err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range z.File {
		if f.Name == "word/document.xml" {
			r, _ := f.Open()
			data, _ := io.ReadAll(r)
			if !strings.Contains(string(data), `<w:color w:val="D93636"/><w:sz w:val="18"/><w:szCs w:val="18"/>`) {
				t.Error("the document lacks the paragraph's colour and size")
			}
		}
	}
}
