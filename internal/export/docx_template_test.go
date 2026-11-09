package export

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"

	"github.com/dbuchs7/orbithek/internal/item"
	"github.com/dbuchs7/orbithek/internal/wordtemplate"
)

const (
	nsWT = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	nsRT = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
)

// smallTemplate is a template as a German Word writes one, with a control
// for the title and the author, the place of the text, a style named
// "Quote" missing but its ID taken by another, and no lists of its own.
func smallTemplate(t *testing.T) *wordtemplate.Template {
	t.Helper()
	parts := map[string]string{
		"[Content_Types].xml": `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/>` +
			`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.template.main+xml"/></Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`,
		"word/styles.xml": `<w:styles xmlns:w="` + nsWT + `">` +
			`<w:style w:type="paragraph" w:default="1" w:styleId="Standard"><w:name w:val="Normal"/></w:style>` +
			`<w:style w:type="paragraph" w:styleId="berschrift1"><w:name w:val="heading 1"/><w:basedOn w:val="Standard"/></w:style>` +
			`<w:style w:type="paragraph" w:styleId="Quote"><w:name w:val="Zitat der Firma"/></w:style>` +
			`<w:style w:type="table" w:styleId="Tabellenraster"><w:name w:val="Table Grid"/></w:style></w:styles>`,
		"word/document.xml": `<w:document xmlns:w="` + nsWT + `" xmlns:r="` + nsRT + `"><w:body>` +
			`<w:sdt><w:sdtPr><w:tag w:val="orbithek:title"/></w:sdtPr><w:sdtContent><w:p><w:r><w:t>[Titel]</w:t></w:r></w:p></w:sdtContent></w:sdt>` +
			`<w:p><w:sdt><w:sdtPr><w:tag w:val="orbithek:owner"/></w:sdtPr><w:sdtContent><w:r><w:t>[Owner]</w:t></w:r></w:sdtContent></w:sdt></w:p>` +
			`<w:sdt><w:sdtPr><w:tag w:val="orbithek:body"/></w:sdtPr><w:sdtContent><w:p/></w:sdtContent></w:sdt>` +
			`<w:sectPr><w:pgSz w:w="16838" w:h="11906" w:orient="landscape"/><w:pgMar w:top="1000" w:right="1000" w:bottom="1000" w:left="1000"/></w:sectPr></w:body></w:document>`,
	}
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for _, name := range []string{"[Content_Types].xml", "_rels/.rels", "word/_rels/document.xml.rels", "word/styles.xml", "word/document.xml"} {
		w, _ := z.Create(name)
		_, _ = io.WriteString(w, parts[name])
	}
	_ = z.Close()
	tp, err := wordtemplate.Parse(buf.Bytes(), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	return tp
}

func TestADocumentIsWrittenIntoItsTemplate(t *testing.T) {
	d := Document{Pages: []Page{{
		ID: "root", Title: "Quartalsbericht",
		Facts: [][2]string{{"Key", "SECRET-FACT"}},
		Body: []item.Block{
			{Kind: item.TOC},
			{Kind: item.Heading, Level: 1, Text: "Ausgangslage"},
			{Kind: item.Paragraph, Text: "Siehe Quelle", Marks: []item.Mark{{Type: item.MarkLink, Start: 6, End: 12, Href: "https://example.org/q"}, {Type: item.MarkCode, Start: 0, End: 5}}},
			{Kind: item.BulletItem, Text: "eins"}, {Kind: item.OrderedItem, Text: "erstens"},
			{Kind: item.Quote, Text: "gesagt"},
			{Kind: item.Table, Text: "A\tB\n1\t2"},
			{Kind: item.Image, Ref: "pic", Text: "Plan", Width: 100},
		},
		Images: map[item.ID]Image{"pic": {Name: "plan.png", Type: "image/png", Data: pngOf(t, 4000, 100)}},
	}}, Template: smallTemplate(t), Controls: map[string]string{"title": "Quartalsbericht", "owner": "Ada"}}
	var buf bytes.Buffer
	if err := DOCX(&buf, d); err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	parts := map[string]string{}
	for _, f := range z.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		parts[f.Name] = string(b)
		if strings.HasSuffix(f.Name, ".xml") || strings.HasSuffix(f.Name, ".rels") {
			dec := xml.NewDecoder(bytes.NewReader(b))
			for {
				if _, err := dec.Token(); err == io.EOF {
					break
				} else if err != nil {
					t.Fatalf("%s is not well-formed: %v\n%s", f.Name, err, b)
				}
			}
		}
	}
	doc := parts["word/document.xml"]
	for _, want := range []string{
		`<w:t xml:space="preserve">Quartalsbericht</w:t>`,
		`<w:t xml:space="preserve">Ada</w:t>`,
		`<w:fldChar w:fldCharType="begin" w:dirty="true"/>`, `TOC \o "1-3" \h \z \u`,
		`<w:pStyle w:val="berschrift1"/>`,
		`<w:hyperlink r:id="orbL1">`, `<w:rStyle w:val="Hyperlink"/>`, `<w:rStyle w:val="CodeChar"/>`,
		`<w:pStyle w:val="ListParagraph"/><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr>`,
		`<w:numId w:val="2"/>`,
		`<w:pStyle w:val="OrbithekQuote"/>`,
		`<w:tblStyle w:val="Tabellenraster"/>`,
		`<w:gridCol w:w="7419"/>`,
		`r:embed="orbI1"`,
		`<wp:extent cx="9422130"`,
		`xmlns:wp=`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("the document lacks %s:\n%s", want, doc)
		}
	}
	for _, gone := range []string{"SECRET-FACT", "[Titel]", "[Owner]", `w:val="Title"`} {
		if strings.Contains(doc, gone) {
			t.Errorf("the document holds %q", gone)
		}
	}
	st := parts["word/styles.xml"]
	for _, want := range []string{`w:styleId="OrbithekQuote"><w:name w:val="Quote"/><w:basedOn w:val="Standard"/>`, `w:styleId="ListParagraph"`, `w:styleId="Hyperlink"`, `w:styleId="Caption"`} {
		if !strings.Contains(st, want) {
			t.Errorf("styles lack %s", want)
		}
	}
	if strings.Contains(st, `w:styleId="Heading1"`) || strings.Contains(st, `w:styleId="TableGrid"`) {
		t.Error("a style the template has was added again")
	}
	if !strings.Contains(parts["word/numbering.xml"], `<w:num w:numId="2"><w:abstractNumId w:val="1"/>`) {
		t.Errorf("numbering: %s", parts["word/numbering.xml"])
	}
	if parts["word/media/orbithek-image1.png"] == "" {
		t.Error("the picture is missing")
	}
	if !strings.Contains(parts["[Content_Types].xml"], "document.main+xml") {
		t.Error("the main part is still a template's")
	}
}

func TestATemplateWithoutTitleControlGetsTheTitleAndATOCWithoutHeadings(t *testing.T) {
	tp := smallTemplate(t)
	d := Document{Pages: []Page{{ID: "r", Title: "Ohne", Body: []item.Block{{Kind: item.TOC}}}}, Template: tp, Controls: map[string]string{}}
	b := &docxBuilder{d: d, tpl: tp, rel: "orb", numBase: 1, width: tp.TextWidth(), styleN: map[string]string{}}
	if got := b.tocField(nil); got != `<w:p><w:r><w:fldChar w:fldCharType="begin" w:dirty="true"/></w:r><w:r><w:instrText xml:space="preserve"> TOC \o "1-3" \h \z \u </w:instrText></w:r><w:r><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r></w:p>` {
		t.Errorf("an empty table of contents = %s", got)
	}
	if len(styleDefs) != 18 || styleDefs["Heading1"].name != "heading 1" || styleDefs["TableGrid"].typ != "table" || styleDefs["Normal"].name != "Normal" {
		t.Errorf("styleDefs = %d entries: %+v", len(styleDefs), styleDefs["Heading1"])
	}
}
