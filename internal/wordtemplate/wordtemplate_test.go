package wordtemplate

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestParseCleansATemplateAndSaysWhatItHolds(t *testing.T) {
	tp := parsed(t, fixture())
	info := tp.Info()
	if fmt.Sprint(info.Controls) != "[author klassifizierung title unknown]" {
		t.Errorf("controls = %v", info.Controls)
	}
	if !info.Body || !info.Header || !info.Footer {
		t.Errorf("body %v, header %v, footer %v; want all", info.Body, info.Header, info.Footer)
	}
	if fmt.Sprint(info.Removed) != "[attached template reference]" {
		t.Errorf("removed = %v", info.Removed)
	}
	if !strings.Contains(strings.Join(info.Styles, "|"), "Normal|Title|heading 1|heading 2|Quote|header") {
		t.Errorf("styles = %v", info.Styles)
	}
	if info.Page.Width != 595.3 || info.Page.Height != 841.9 || info.Page.Top != 85.05 || info.Page.Left != 70.9 || info.Page.Header != 28.35 {
		t.Errorf("page = %+v", info.Page)
	}
	if got, want := tp.TextWidth(), int64((595.3-56.7-70.9)*12700); got != want {
		t.Errorf("text width = %d, want %d", got, want)
	}
	if !tp.HasControl("Title") || tp.HasControl("body") {
		t.Error("HasControl: title is a control, body is the text's place")
	}
	// The kept package is the cleaned one: no reference to the share.
	pkg, err := tp.Package()
	if err != nil {
		t.Fatal(err)
	}
	parts := unpack(t, pkg)
	if strings.Contains(parts["word/settings.xml"], "attachedTemplate") || strings.Contains(parts["word/_rels/settings.xml.rels"], "share") {
		t.Errorf("the attached template is still referred to:\n%s\n%s", parts["word/settings.xml"], parts["word/_rels/settings.xml.rels"])
	}
	if parts["word/document.xml"] != fixture()["word/document.xml"] {
		t.Error("the document part changed; cleaning must keep it byte for byte")
	}
	again, err := Parse(pkg, 10<<20)
	if err != nil {
		t.Fatalf("the cleaned package is refused: %v", err)
	}
	if len(again.Info().Removed) != 0 {
		t.Errorf("cleaning the cleaned package removed %v", again.Info().Removed)
	}
}

func TestParseRefusesWhatMayRunOrReachOut(t *testing.T) {
	with := func(change func(p map[string]string)) []byte {
		p := fixture()
		change(p)
		return pack(t, p)
	}
	rels := func(p map[string]string, rel string) {
		p["word/_rels/document.xml.rels"] = strings.Replace(p["word/_rels/document.xml.rels"], "</Relationships>", rel+"</Relationships>", 1)
	}
	many := map[string]string{}
	for k, v := range fixture() {
		many[k] = v
	}
	for i := range maxParts {
		many[fmt.Sprintf("customXml/item%d.xml", i)] = "<a/>"
	}
	cases := map[string][]byte{
		"not a zip": []byte("PK? no"),
		"macros by content type": with(func(p map[string]string) {
			p["[Content_Types].xml"] = strings.Replace(p["[Content_Types].xml"], templateMain, "application/vnd.ms-word.template.macroEnabledTemplate.main+xml", 1)
		}),
		"macros by part": with(func(p map[string]string) { p["word/vbaProject.bin"] = "x" }),
		"activex":        with(func(p map[string]string) { p["word/activeX/activeX1.xml"] = "<a/>" }),
		"external picture": with(func(p map[string]string) {
			rels(p, `<Relationship Id="x" Type="`+relImage+`" Target="http://evil/p.png" TargetMode="External"/>`)
		}),
		"embedded object": with(func(p map[string]string) {
			rels(p, `<Relationship Id="x" Type="`+relBase+`oleObject" Target="embeddings/o.bin"/>`)
		}),
		"document type": with(func(p map[string]string) {
			p["word/styles.xml"] = strings.Replace(p["word/styles.xml"], xmlHead, xmlHead+`<!DOCTYPE x [<!ENTITY e "e">]>`, 1)
		}),
		"broken xml": with(func(p map[string]string) { p["word/footer1.xml"] = "<w:ftr>" }),
		"a spreadsheet": with(func(p map[string]string) {
			p["[Content_Types].xml"] = strings.Replace(p["[Content_Types].xml"], templateMain, "application/vnd.ms-excel", 1)
		}),
		"no document":     with(func(p map[string]string) { delete(p, "word/document.xml") }),
		"no styles":       with(func(p map[string]string) { delete(p, "word/styles.xml") }),
		"a climbing name": with(func(p map[string]string) { p["word/../../x.xml"] = "<a/>" }),
		"another main": with(func(p map[string]string) {
			p["_rels/.rels"] = strings.Replace(p["_rels/.rels"], "word/document.xml", "word/other.xml", 1)
		}),
		"w bound elsewise": with(func(p map[string]string) { p["word/header1.xml"] = `<w:hdr xmlns:w="urn:other"/>` }),
		"no body":          with(func(p map[string]string) { p["word/document.xml"] = `<w:document xmlns:w="` + nsW + `"/>` }),
		"too many parts":   pack(t, many),
		"rels outside a folder": with(func(p map[string]string) {
			p["word/stray.rels"] = `<Relationships xmlns="` + nsRel + `"/>`
		}),
	}
	for name, data := range cases {
		if _, err := Parse(data, 10<<20); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
	if _, err := Parse(pack(t, fixture()), 1000); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "unpacks to more") {
		t.Errorf("a package larger than the bound: err = %v", err)
	}
	// A duplicate part name, which a zip allows.
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for _, name := range []string{"[Content_Types].xml", "[content_types].xml"} {
		w, _ := z.Create(name)
		_, _ = w.Write([]byte(fixture()["[Content_Types].xml"]))
	}
	_ = z.Close()
	if _, err := Parse(buf.Bytes(), 10<<20); !errors.Is(err, ErrInvalid) {
		t.Errorf("a part twice: err = %v", err)
	}
}

func TestStylesAreFoundByTheirEnglishNames(t *testing.T) {
	tp := parsed(t, fixture())
	for _, c := range []struct{ typ, name, want string }{
		{"paragraph", "Heading 1", "berschrift1"},
		{"paragraph", "title", "Titel"},
		{"character", "hyperlink", "Hyperlink"},
		{"table", "Table Grid", "Tabellenraster"},
		{"paragraph", "Code", ""},
		{"character", "heading 1", ""},
	} {
		if got := tp.StyleID(c.typ, c.name); got != c.want {
			t.Errorf("StyleID(%s, %s) = %q, want %q", c.typ, c.name, got, c.want)
		}
	}
	if tp.DefaultParagraphStyle() != "Standard" || !tp.StyleIDTaken("Fett") || tp.StyleIDTaken("Code") {
		t.Error("default paragraph style or taken IDs are wrong")
	}
	if a, n := tp.NumberingBase(); a != 4 || n != 5 {
		t.Errorf("numbering base = %d, %d; want 4, 5", a, n)
	}
	p := fixture()
	delete(p, "word/numbering.xml")
	p["word/_rels/document.xml.rels"] = strings.Replace(p["word/_rels/document.xml.rels"], `Target="numbering.xml"`, `Target="missing.xml"`, 1)
	if a, n := parsed(t, p).NumberingBase(); a != 0 || n != 1 {
		t.Errorf("numbering base without numbering = %d, %d", a, n)
	}
	p = fixture()
	p["word/styles.xml"] = strings.Replace(p["word/styles.xml"], ` w:default="1"`, "", 1)
	if parsed(t, p).DefaultParagraphStyle() != "Normal" {
		t.Error("without a default paragraph style, Normal is assumed")
	}
}

func TestMediaGivesOnlyPicturesTheBrowserShows(t *testing.T) {
	p := fixture()
	p["word/media/image2.emf"] = "\x01\x00\x00\x00emf"
	p["word/media/image3.gif"] = "GIF89a..."
	p["word/media/image4.jpeg"] = "\xff\xd8\xff\xe0jpeg"
	tp := parsed(t, p)
	for name, want := range map[string]string{"image1.png": "image/png", "image3.gif": "image/gif", "image4.jpeg": "image/jpeg", "image2.emf": "", "../document.xml": "", "none.png": ""} {
		_, typ, ok := tp.Media(name)
		if typ != want || ok != (want != "") {
			t.Errorf("Media(%s) = %q, %v; want %q", name, typ, ok, want)
		}
	}
}

func content() Content {
	return Content{
		Body:         `<w:p><w:pPr><w:pStyle w:val="berschrift1"/></w:pPr><w:r><w:t>BODY</w:t></w:r></w:p>`,
		Styles:       []string{`<w:style w:type="paragraph" w:styleId="Code"><w:name w:val="Code"/><w:basedOn w:val="Standard"/></w:style>`},
		AbstractNums: []string{`<w:abstractNum w:abstractNumId="4"/>`},
		Nums:         []string{`<w:num w:numId="5"><w:abstractNumId w:val="4"/></w:num>`},
		Links:        map[string]string{"orbL1": "https://example.org/?a=1&b=2"},
		Media:        []Media{{ID: "orbI1", Name: "orbithek-image1.png", Data: png1}, {ID: "orbI2", Name: "orbithek-image2.gif", Data: []byte("GIF89a")}},
		Controls:     map[string]string{"title": "Mein Bericht", "author": "Ada\nLovelace", "klassifizierung": ""},
		Title:        "Mein <Bericht>",
	}
}

func TestWritePutsTheTextAndTheFieldsIntoTheTemplate(t *testing.T) {
	tp := parsed(t, fixture())
	var buf bytes.Buffer
	if err := tp.Write(&buf, content()); err != nil {
		t.Fatal(err)
	}
	parts := unpack(t, buf.Bytes())
	doc := parts["word/document.xml"]
	for _, want := range []string{
		`<w:sdtContent><w:p><w:pPr><w:pStyle w:val="Titel"/></w:pPr><w:r><w:rPr><w:color w:val="808080"/></w:rPr><w:t xml:space="preserve">Mein Bericht</w:t></w:r></w:p></w:sdtContent>`,
		`<w:sdtContent><w:r><w:rPr><w:i/></w:rPr><w:t xml:space="preserve">Ada</w:t><w:br/><w:t xml:space="preserve">Lovelace</w:t></w:r></w:sdtContent>`,
		`<w:t>bleibt</w:t>`,
		`<w:t>Inhalt</w:t>`,
		`</w:tbl><w:p><w:pPr><w:pStyle w:val="berschrift1"/></w:pPr><w:r><w:t>BODY</w:t></w:r></w:p><w:p><w:r><w:t>Schluss</w:t>`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("the document lacks %s:\n%s", want, doc)
		}
	}
	for _, gone := range []string{"Hier steht der Text", "[Titel des Berichts]", "[Autor]", "showingPlcHdr", "dataBinding"} {
		if strings.Contains(doc, gone) {
			t.Errorf("the document still holds %q", gone)
		}
	}
	if !strings.Contains(parts["[Content_Types].xml"], documentMain) || strings.Contains(parts["[Content_Types].xml"], templateMain) {
		t.Errorf("the main part is still a template's:\n%s", parts["[Content_Types].xml"])
	}
	if strings.Count(parts["[Content_Types].xml"], `Extension="png"`) != 1 || !strings.Contains(parts["[Content_Types].xml"], `<Default Extension="gif" ContentType="image/gif"/>`) {
		t.Errorf("the content types of the pictures:\n%s", parts["[Content_Types].xml"])
	}
	if !strings.Contains(parts["docProps/core.xml"], "<dc:title>Mein &lt;Bericht&gt;</dc:title>") {
		t.Errorf("core properties: %s", parts["docProps/core.xml"])
	}
	if !strings.Contains(parts["word/header1.xml"], `<w:sdtContent><w:r><w:rPr><w:b/></w:rPr><w:t xml:space="preserve">Mein Bericht</w:t></w:r></w:sdtContent>`) {
		t.Errorf("header: %s", parts["word/header1.xml"])
	}
	if !strings.Contains(parts["word/footer1.xml"], `<w:sdtContent><w:r><w:t xml:space="preserve"></w:t></w:r></w:sdtContent>`) {
		t.Errorf("footer: %s", parts["word/footer1.xml"])
	}
	if !strings.Contains(parts["word/numbering.xml"], `</w:abstractNum><w:abstractNum w:abstractNumId="4"/><w:num w:numId="1">`) ||
		!strings.HasSuffix(parts["word/numbering.xml"], `<w:num w:numId="5"><w:abstractNumId w:val="4"/></w:num></w:numbering>`) {
		t.Errorf("numbering: %s", parts["word/numbering.xml"])
	}
	if !strings.Contains(parts["word/styles.xml"], `<w:style w:type="paragraph" w:styleId="Code">`) {
		t.Error("the added style is missing")
	}
	rels := parts["word/_rels/document.xml.rels"]
	if !strings.Contains(rels, `Id="orbL1"`) || !strings.Contains(rels, `Target="https://example.org/?a=1&amp;b=2" TargetMode="External"`) || !strings.Contains(rels, `Target="media/orbithek-image1.png"`) {
		t.Errorf("relationships: %s", rels)
	}
	if parts["word/media/orbithek-image1.png"] != string(png1) || parts["word/media/image1.png"] != string(png1) {
		t.Error("pictures are missing")
	}
	if strings.Contains(parts["word/settings.xml"], "attachedTemplate") {
		t.Error("the written document refers to the attached template")
	}
	// What was written is a document this package accepts again.
	if _, err := Parse(buf.Bytes(), 10<<20); err != nil {
		t.Errorf("the written document is refused: %v", err)
	}
}

func TestWriteWithoutAMarkerAddsWhatTheTemplateLacks(t *testing.T) {
	p := fixture()
	p["word/document.xml"] = xmlHead + `<w:document xmlns:w="` + nsW + `"><w:body><w:p><w:r><w:t>Kopf</w:t></w:r></w:p>` +
		`<w:p><w:r><w:t>Zwischen</w:t></w:r><w:sdt><w:sdtPr><w:tag w:val="orbithek:body"/></w:sdtPr><w:sdtContent><w:r><w:t>x</w:t></w:r></w:sdtContent></w:sdt></w:p>` +
		`<w:sectPr/></w:body></w:document>`
	delete(p, "word/numbering.xml")
	p["word/_rels/document.xml.rels"] = strings.Replace(p["word/_rels/document.xml.rels"], `<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering" Target="numbering.xml"/>`, "", 1)
	tp := parsed(t, p)
	var buf bytes.Buffer
	c := content()
	c.AbstractNums, c.Nums = []string{`<w:abstractNum w:abstractNumId="0"/>`}, []string{`<w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>`}
	if err := tp.Write(&buf, c); err != nil {
		t.Fatal(err)
	}
	parts := unpack(t, buf.Bytes())
	doc := parts["word/document.xml"]
	if !strings.Contains(doc, `<w:document xmlns:w="`+nsW+`" xmlns:r="`+nsR+`" xmlns:wp="`+nsWP+`">`) {
		t.Errorf("the prefixes r and wp are not declared: %s", doc)
	}
	// An inline marker takes its paragraph's place.
	if !strings.Contains(doc, `<w:t>Kopf</w:t></w:r></w:p><w:p><w:pPr><w:pStyle w:val="berschrift1"/></w:pPr><w:r><w:t>BODY</w:t></w:r></w:p><w:sectPr/>`) || strings.Contains(doc, "Zwischen") {
		t.Errorf("document: %s", doc)
	}
	if !strings.Contains(parts["word/numbering.xml"], `<w:num w:numId="1">`) ||
		!strings.Contains(parts["word/_rels/document.xml.rels"], `Id="orbN" Type="`+relNumbering+`" Target="numbering.xml"`) ||
		!strings.Contains(parts["[Content_Types].xml"], `<Override PartName="/word/numbering.xml" ContentType="`+numberingCT+`"/>`) {
		t.Errorf("the numbering part was not added:\n%s\n%s", parts["word/_rels/document.xml.rels"], parts["[Content_Types].xml"])
	}
	if _, err := Parse(buf.Bytes(), 10<<20); err != nil {
		t.Errorf("the written document is refused: %v", err)
	}

	// Without a marker the text follows the template's content.
	p = fixture()
	p["word/document.xml"] = strings.Replace(p["word/document.xml"], "orbithek:body", "something", 1)
	buf.Reset()
	if err := parsed(t, p).Write(&buf, Content{Body: "<w:p/>", Controls: map[string]string{}}); err != nil {
		t.Fatal(err)
	}
	if doc := unpack(t, buf.Bytes())["word/document.xml"]; !strings.Contains(doc, `<w:t>Schluss</w:t></w:r></w:p><w:p/><w:sectPr>`) {
		t.Errorf("document: %s", doc)
	}
}

func TestRelAndMediaPrefixesAvoidTheTemplates(t *testing.T) {
	p := fixture()
	p["word/_rels/document.xml.rels"] = strings.Replace(p["word/_rels/document.xml.rels"], `Id="rId8"`, `Id="orb1"`, 1)
	p["word/document.xml"] = strings.Replace(p["word/document.xml"], `r:id="rId8"`, `r:id="orb1"`, 1)
	p["word/media/orbithek-x.png"] = string(png1)
	tp := parsed(t, p)
	if tp.RelPrefix() != "orbx" || tp.MediaPrefix() != "xorbithek-" {
		t.Errorf("prefixes = %s, %s", tp.RelPrefix(), tp.MediaPrefix())
	}
}

func TestFrameDrawsTheTemplateWithTheFieldsFilledIn(t *testing.T) {
	tp := parsed(t, fixture())
	f := tp.Frame(map[string]string{"title": "Mein Bericht", "author": "Ada", "klassifizierung": "VERTRAULICH"}, func(name string) string { return "/m/" + name })
	if len(f.Sheets) != 2 {
		t.Fatalf("sheets = %d, want 2: %+v", len(f.Sheets), f.Sheets)
	}
	cover := f.Sheets[0]
	title := tp.styles.byID["Titel"].class
	if cover[0].P == nil || cover[0].P.Class != title || cover[0].P.Runs[0].Text != "Mein Bericht" || cover[0].P.Runs[0].Colour != "#808080" {
		t.Errorf("cover title = %+v", cover[0].P)
	}
	var author []string
	for _, r := range cover[1].P.Runs {
		author = append(author, r.Text)
	}
	if strings.Join(author, "") != "Verfasst von Ada am bleibt" || cover[1].P.Runs[1].Italic == nil || !*cover[1].P.Runs[1].Italic {
		t.Errorf("author line = %+v", cover[1].P.Runs)
	}
	second := f.Sheets[1]
	if second[0].P == nil || second[0].P.Runs[0].Text != "Inhalt" {
		t.Errorf("the second sheet starts with %+v", second[0])
	}
	tb := second[1].Table
	if tb == nil || !tb.Borders || len(tb.Rows) != 1 || tb.Rows[0][0].Fill != "#F2F2F2" || tb.Rows[0][0].Width != 100 || tb.Rows[0][1].Span != 2 {
		t.Fatalf("table = %+v", tb)
	}
	link := tb.Rows[0][1].Nodes[0].P.Runs[0]
	if link.Text != "Link" || link.Bold == nil || !*link.Bold || link.Fill != "#FFFF00" {
		t.Errorf("a run of a character style = %+v", link)
	}
	if !second[2].Body || second[3].P.Runs[0].Text != "Schluss" {
		t.Errorf("the text's place and what follows = %+v", second[2:])
	}
	if !f.TitlePage || len(f.FirstHeader) == 0 || len(f.FirstFooter) != 0 {
		t.Errorf("first page: %v, %d, %d", f.TitlePage, len(f.FirstHeader), len(f.FirstFooter))
	}
	head := f.Header[0].P
	if head.Class != tp.styles.byID["Kopfzeile"].class {
		t.Errorf("header class = %s", head.Class)
	}
	var kinds []string
	for _, r := range head.Runs {
		switch {
		case r.Image != nil:
			kinds = append(kinds, fmt.Sprintf("img:%s:%vx%v:%s", r.Image.Src, r.Image.Width, r.Image.Height, r.Image.Align))
		case r.Tab:
			kinds = append(kinds, "tab")
		case r.Field != "":
			kinds = append(kinds, "field:"+r.Field)
		default:
			kinds = append(kinds, r.Text)
		}
	}
	if strings.Join(kinds, "|") != "img:/m/image1.png:100x50:right|Mein Bericht|tab|Seite |field:page" {
		t.Errorf("header runs = %v", kinds)
	}
	var foot []string
	for _, r := range f.Footer[0].P.Runs {
		foot = append(foot, r.Text+r.Field)
	}
	if strings.Join(foot, "|") != "von |pages| · |VERTRAULICH" {
		t.Errorf("footer runs = %v", foot)
	}
	// The frame is what the browser receives: JSON of text and values.
	if _, err := json.Marshal(f); err != nil {
		t.Fatal(err)
	}

	// Without values, the controls show the template's content, and a
	// template without a marker has the text at its end.
	p := fixture()
	p["word/document.xml"] = strings.Replace(p["word/document.xml"], "orbithek:body", "orbithek:other", 1)
	f = parsed(t, p).Frame(nil, nil)
	last := f.Sheets[len(f.Sheets)-1]
	if !last[len(last)-1].Body || f.Sheets[0][0].P.Runs[0].Text != "[Titel des Berichts]" {
		t.Errorf("frame without values or marker: %+v", f.Sheets)
	}
	if r := f.Header[0].P.Runs[0]; r.Image != nil {
		t.Error("a picture is shown without a way to address it")
	}
}

func TestFrameKeepsTextBoxesAndSplitsAtSectionsAndBreaks(t *testing.T) {
	p := fixture()
	p["word/document.xml"] = xmlHead + `<w:document xmlns:w="` + nsW + `" xmlns:r="` + nsR + `" xmlns:wp="` + nsWP + `" xmlns:a="` + nsA + `" xmlns:mc="` + nsMC + `" xmlns:wps="` + nsWPS + `" xmlns:v="` + nsV + `"><w:body>` +
		`<w:p><w:r><mc:AlternateContent><mc:Choice Requires="wps"><w:drawing><wp:anchor><a:graphic><a:graphicData><wps:wsp><wps:txbx><w:txbxContent><w:p><w:r><w:t>Im Kasten</w:t></w:r></w:p></w:txbxContent></wps:txbx></wps:wsp></a:graphicData></a:graphic></wp:anchor></w:drawing></mc:Choice>` +
		`<mc:Fallback><w:pict><v:shape><v:textbox><w:txbxContent><w:p><w:r><w:t>Doppelt</w:t></w:r></w:p></w:txbxContent></v:textbox></v:shape></w:pict></mc:Fallback></mc:AlternateContent></w:r>` +
		`<w:r><w:t>vor</w:t><w:br w:type="page"/><w:t>nach</w:t></w:r></w:p>` +
		`<w:p><w:pPr><w:sectPr/></w:pPr></w:p>` +
		`<w:p><w:pPr><w:pageBreakBefore/></w:pPr><w:r><w:pict><v:shape style="width:72pt;height:1in"><v:imagedata r:id="rId7"/></v:shape></w:pict></w:r></w:p>` +
		`<w:p><w:fldSimple w:instr="AUTHOR"><w:r><w:t>Muster</w:t></w:r></w:fldSimple><w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText>DATE</w:instrText></w:r><w:r><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:t>1.1.2026</w:t></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r></w:p>` +
		`<w:p><w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText>NUMPAGES</w:instrText></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r><w:r><w:noBreakHyphen/><w:cr/></w:r></w:p>` +
		`<w:sectPr/></w:body></w:document>`
	f := parsed(t, p).Frame(nil, func(name string) string { return name })
	if len(f.Sheets) != 3 {
		t.Fatalf("sheets = %d: %+v", len(f.Sheets), f.Sheets)
	}
	var first []string
	for _, n := range f.Sheets[0] {
		for _, r := range n.P.Runs {
			first = append(first, r.Text)
		}
	}
	if strings.Join(first, "|") != "vor|Im Kasten" {
		t.Errorf("first sheet = %v", first)
	}
	if f.Sheets[1][0].P.Runs[0].Text != "nach" || len(f.Sheets[1]) != 2 {
		t.Errorf("second sheet = %+v", f.Sheets[1])
	}
	third := f.Sheets[2]
	img := third[0].P.Runs[0].Image
	if img == nil || img.Width != 72 || img.Height != 72 {
		t.Errorf("a picture of the older form = %+v", third[0].P.Runs)
	}
	if third[1].P.Runs[0].Text != "Muster" || third[1].P.Runs[1].Text != "1.1.2026" {
		t.Errorf("fields show their results: %+v", third[1].P.Runs)
	}
	if r := third[2].P.Runs; r[0].Field != "pages" || r[1].Text != "-" || !r[2].Break {
		t.Errorf("a field without a result, a hyphen and a break: %+v", r)
	}
	if !third[3].Body {
		t.Error("the text belongs at the end")
	}
}

func TestStylesheetDrawsThePageAndTheStyles(t *testing.T) {
	tp := parsed(t, fixture())
	css := tp.Stylesheet(".wt-doc.wt-t-bericht-1")
	for _, want := range []string{
		`.wt-doc.wt-t-bericht-1 .wt-sheet{width:595.3pt;min-height:841.9pt;font-family:"Frutiger",system-ui,sans-serif;font-size:10pt;line-height:1.1}`,
		`.wt-head{min-height:85.05pt;padding:28.35pt 56.7pt 0 70.9pt}`,
		`.wt-foot{min-height:56.7pt;padding:0 56.7pt 28.35pt 70.9pt}`,
		`.wt-body .ProseMirror h2{font-family:"Frutiger",system-ui,sans-serif;font-size:16pt;font-weight:700;margin-top:24pt;margin-bottom:6pt;line-height:1.1;border-bottom:1pt solid #DC0018;padding-bottom:4pt}`,
		`.wt-body .ProseMirror h3{font-family:"Frutiger",system-ui,sans-serif;font-size:13pt;font-weight:700;font-style:italic;margin-top:24pt;margin-bottom:6pt;line-height:1.1;border-bottom:none}`,
		`.wt-body .ProseMirror blockquote{font-family:"Frutiger",system-ui,sans-serif;font-size:10pt;font-style:italic;text-transform:uppercase;margin-bottom:6pt;margin-left:28.35pt;text-indent:-14.15pt;line-height:1.1}`,
		`.wt-body .ProseMirror a{text-decoration:underline;color:#0563C1}`,
		`.wt-` + tp.styles.byID["Titel"].class + `{font-family:"Frutiger Bold",system-ui,sans-serif;font-size:28pt;color:#A50012;text-align:center;margin-bottom:6pt;line-height:1.1}`,
		`.wt-body .ProseMirror{counter-reset:wtn1 wtn2 wtn3}`,
		`.wt-body .ProseMirror h2{counter-increment:wtn1;counter-reset:wtn2 wtn3}`,
		`.wt-body .ProseMirror h2::before{content:counter(wtn1,decimal) " "}`,
		`.wt-body .ProseMirror h3::before{content:counter(wtn1,decimal) "." counter(wtn2,decimal) " "}`,
		`@media (max-width: 760px)`,
	} {
		if !strings.Contains(css, want) {
			t.Errorf("the stylesheet lacks\n%s\nin\n%s", want, css)
		}
	}
	if strings.Contains(css, "figcaption") || strings.Contains(css, " pre{") {
		t.Error("rules for styles the template lacks")
	}
	if tp.Stylesheet(`x{}body{color:red}`) != "" {
		t.Error("a scope with markup of CSS is written")
	}
}

func TestLevelContentAndLengths(t *testing.T) {
	levels := map[int]level{0: {"upperRoman", "%1."}, 1: {"lowerLetter", ""}, 3: {"bullet", ""}}
	if got := levelContent(`Kapitel %1.%2)"x\`, levels); got != `"Kapitel " counter(wtn1,upper-roman) "." counter(wtn2,lower-alpha) ")x" " "` {
		t.Errorf("levelContent = %s", got)
	}
	if got := levelContent("%4", levels); got != `" "` {
		t.Errorf("a fourth level = %s", got)
	}
	for _, c := range []struct {
		v, unit string
		want    float64
	}{{"1", "in", 72}, {"2.54", "cm", 72}, {"25.4", "mm", 72}, {"4", "px", 3}, {"9", "", 9}, {"x", "pt", 0}} {
		if got := length(c.v, c.unit); got < c.want-0.001 || got > c.want+0.001 {
			t.Errorf("length(%s%s) = %v", c.v, c.unit, got)
		}
	}
}
