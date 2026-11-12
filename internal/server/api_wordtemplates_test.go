package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dbuchs7/orbithek/internal/budget"
	"github.com/dbuchs7/orbithek/internal/item"
)

var onePixel = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89\x00\x00\x00\rIDATx\xdac\xfc\xcf\xc0\xf0\x1f\x00\x05\x05\x02\x00_\xc8\xf1\xd2\x00\x00\x00\x00IEND\xaeB`\x82")

const (
	nsW = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	nsR = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
)

// dotx makes a template with a cover line of title and owner, a heading
// style, the place of the text and a header with a logo and the page
// number; label tells versions apart.
func dotx(t *testing.T, label string) []byte {
	t.Helper()
	parts := [][2]string{
		{"[Content_Types].xml", `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Default Extension="png" ContentType="image/png"/>` +
			`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.template.main+xml"/></Types>`},
		{"_rels/.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`},
		{"word/_rels/document.xml.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>` +
			`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/></Relationships>`},
		{"word/_rels/header1.xml.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/logo.png"/></Relationships>`},
		{"word/media/logo.png", string(onePixel)},
		{"word/styles.xml", `<w:styles xmlns:w="` + nsW + `"><w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:ascii="Frutiger"/><w:sz w:val="22"/></w:rPr></w:rPrDefault></w:docDefaults>` +
			`<w:style w:type="paragraph" w:default="1" w:styleId="Standard"><w:name w:val="Normal"/></w:style>` +
			`<w:style w:type="paragraph" w:styleId="berschrift1"><w:name w:val="heading 1"/><w:basedOn w:val="Standard"/><w:rPr><w:b/><w:color w:val="DC0018"/></w:rPr></w:style></w:styles>`},
		{"word/header1.xml", `<w:hdr xmlns:w="` + nsW + `" xmlns:r="` + nsR + `" xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture">` +
			`<w:p><w:r><w:drawing><wp:inline><wp:extent cx="1270000" cy="635000"/><a:graphic><a:graphicData><pic:pic><pic:blipFill><a:blip r:embed="rId1"/></pic:blipFill></pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing></w:r>` +
			`<w:r><w:t xml:space="preserve">` + label + ` </w:t></w:r><w:fldSimple w:instr="PAGE"><w:r><w:t>1</w:t></w:r></w:fldSimple></w:p></w:hdr>`},
		{"word/document.xml", `<w:document xmlns:w="` + nsW + `" xmlns:r="` + nsR + `"><w:body>` +
			`<w:p><w:sdt><w:sdtPr><w:tag w:val="orbithek:title"/></w:sdtPr><w:sdtContent><w:r><w:t>[Titel]</w:t></w:r></w:sdtContent></w:sdt>` +
			`<w:r><w:t xml:space="preserve"> von </w:t></w:r><w:sdt><w:sdtPr><w:tag w:val="orbithek:owner"/></w:sdtPr><w:sdtContent><w:r><w:t>[Owner]</w:t></w:r></w:sdtContent></w:sdt></w:p>` +
			`<w:sdt><w:sdtPr><w:tag w:val="orbithek:body"/></w:sdtPr><w:sdtContent><w:p/></w:sdtContent></w:sdt>` +
			`<w:sectPr><w:headerReference w:type="default" r:id="rId2"/><w:pgSz w:w="11906" w:h="16838"/></w:sectPr></w:body></w:document>`},
	}
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for _, p := range parts {
		w, _ := z.Create(p[0])
		_, _ = io.WriteString(w, p[1])
	}
	_ = z.Close()
	return buf.Bytes()
}

func (c *client) post(path, contentType string, content []byte) *httptest.ResponseRecorder {
	c.t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(content))
	req.RemoteAddr = "192.0.2.1:4000"
	req.Header.Set("Content-Type", contentType)
	if c.cookie != nil {
		req.AddCookie(c.cookie)
	}
	rec := httptest.NewRecorder()
	c.h.ServeHTTP(rec, req)
	return rec
}

func TestWordTemplatesAreKeptByAdministratorsAndFramePages(t *testing.T) {
	w := newWorld(t)
	tpl := dotx(t, "Bundesamt")

	// Only administrators keep templates, and only clean Word files.
	if r := w.bert.post("/api/word-templates/bericht?name=Bericht", "application/octet-stream", tpl); r.Code != http.StatusForbidden {
		t.Fatalf("an editor uploads: %d", r.Code)
	}
	w.bert.expect("GET", "/api/word-templates", "", http.StatusForbidden)
	if r := w.admin.post("/api/word-templates/bericht?name=Bericht", "application/json", tpl); r.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("a template as JSON: %d", r.Code)
	}
	if r := w.admin.post("/api/word-templates/bericht?name=Bericht", "application/octet-stream", []byte("no zip")); r.Code != http.StatusBadRequest {
		t.Fatalf("no Word file: %d %s", r.Code, r.Body)
	}
	if r := w.admin.post("/api/word-templates/Bericht?name=Bericht", "application/octet-stream", tpl); r.Code != http.StatusBadRequest {
		t.Fatalf("a bad ID: %d %s", r.Code, r.Body)
	}
	r := w.admin.post("/api/word-templates/bericht?name=Bericht&file=Bericht.dotx", "application/octet-stream", tpl)
	if r.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", r.Code, r.Body)
	}
	var added map[string]any
	if err := json.Unmarshal(r.Body.Bytes(), &added); err != nil {
		t.Fatal(err)
	}
	if added["version"] != 1.0 || field(added, "template", "info", "body") != true || len(field(added, "template", "info", "controls").([]any)) != 2 || len(field(added, "template", "info", "removed").([]any)) != 0 {
		t.Fatalf("added %v", added)
	}
	list := w.admin.expect("GET", "/api/word-templates", "", http.StatusOK)
	if got := list["templates"].([]any); len(got) != 1 || got[0].(map[string]any)["name"] != "Bericht" || list["controlPrefix"] != "orbithek:" {
		t.Fatalf("list %v", list)
	}

	// A type names it and a start text; the template must exist.
	w.admin.expect("POST", "/api/types", `{"name":"report","body":true,"template":"none","fields":[{"name":"owner","kind":"user"}]}`, http.StatusBadRequest)
	w.admin.expect("POST", "/api/types", `{"name":"report","body":true,"template":"bericht","startMarkdown":"# Ausgangslage\n\n> [!NOTE]\n> Hier beschreiben.\n\n[toc]\n\n![Bild](x.png)","fields":[{"name":"owner","kind":"user"}]}`, http.StatusCreated)
	types := w.bert.expect("GET", "/api/types", "", http.StatusOK)["types"].([]any)
	var report map[string]any
	for _, ty := range types {
		if ty.(map[string]any)["name"] == "report" {
			report = ty.(map[string]any)
		}
	}
	if report["template"] != "bericht" || !strings.Contains(report["startMarkdown"].(string), "# Ausgangslage") || !strings.Contains(report["startMarkdown"].(string), "[toc]") {
		t.Fatalf("type %v", report)
	}
	start := report["start"].([]any)
	if len(start) != 4 || start[0].(map[string]any)["kind"] != "heading" || start[1].(map[string]any)["kind"] != "panel" || start[2].(map[string]any)["kind"] != "toc" {
		t.Fatalf("start %v", start)
	}

	// A new page begins with the start text and remembers version 1.
	made := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"report","title":"Quartal","properties":{"owner":"`+w.ids["bert"]+`"}}`, http.StatusCreated)
	key := field(made, "item", "key").(string)
	if len(field(made, "item", "body").([]any)) != 4 || field(made, "item", "template", "version") != 1.0 {
		t.Fatalf("made %v", made)
	}
	// A page with its own text keeps it.
	own := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"report","title":"Eigen","body":[{"kind":"paragraph","text":"selbst"}]}`, http.StatusCreated)
	if len(field(own, "item", "body").([]any)) != 1 {
		t.Fatalf("own text %v", own)
	}

	// The frame: the fields filled, the logo by its address, the number.
	doc := w.vera.expect("GET", "/api/items/"+key+"/document", "", http.StatusOK)
	if field(doc, "template", "version") != 1.0 || field(doc, "template", "newest") != 1.0 || doc["scope"] != "wt-t-bericht-1" || doc["stylesheet"] != "/api/word-templates/bericht/versions/1/style.css" {
		t.Fatalf("document %v", doc)
	}
	sheet := field(doc, "frame", "sheets").([]any)[0].([]any)
	var cover []string
	for _, run := range sheet[0].(map[string]any)["p"].(map[string]any)["runs"].([]any) {
		cover = append(cover, run.(map[string]any)["text"].(string))
	}
	if strings.Join(cover, "") != "Quartal von bert" || sheet[1].(map[string]any)["body"] != true {
		t.Fatalf("cover %v, sheet %v", cover, sheet)
	}
	head := field(doc, "frame", "header").([]any)[0].(map[string]any)["p"].(map[string]any)["runs"].([]any)
	src := head[0].(map[string]any)["image"].(map[string]any)["src"].(string)
	if src != "/api/word-templates/bericht/versions/1/media/logo.png" || head[2].(map[string]any)["field"] != "page" {
		t.Fatalf("header %v", head)
	}
	logo := w.vera.do("GET", src, "")
	if logo.Code != http.StatusOK || logo.Header().Get("Content-Type") != "image/png" || !strings.Contains(logo.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Fatalf("logo %d %v", logo.Code, logo.Header())
	}
	w.vera.expect("GET", "/api/word-templates/bericht/versions/1/media/none.png", "", http.StatusNotFound)
	w.vera.expect("GET", "/api/word-templates/bericht/versions/9/media/logo.png", "", http.StatusNotFound)
	w.vera.expect("GET", "/api/word-templates/Bad/versions/1/style.css", "", http.StatusNotFound)
	css := w.vera.do("GET", "/api/word-templates/bericht/versions/1/style.css", "")
	if css.Code != http.StatusOK || !strings.Contains(css.Body.String(), `.wt-doc.wt-t-bericht-1 .wt-body .ProseMirror h2{font-family:"Frutiger"`) {
		t.Fatalf("css %d %s", css.Code, css.Body)
	}
	// Who may not read the page gets no frame of it.
	w.carla.expect("GET", "/api/items/"+key+"/document", "", http.StatusNotFound)
	// A page of a type without a template has no document.
	page := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"T","properties":{"priority":"low"}}`, http.StatusCreated)
	w.bert.expect("GET", "/api/items/"+field(page, "item", "key").(string)+"/document", "", http.StatusNotFound)
	w.bert.expect("POST", "/api/items/"+field(page, "item", "key").(string)+"/template", "", http.StatusBadRequest)

	// The Word export is the template with the page in it.
	exp := w.bert.do("GET", "/api/items/"+key+"/export/docx", "")
	if exp.Code != http.StatusOK {
		t.Fatalf("export %d %s", exp.Code, exp.Body)
	}
	docx := unzipParts(t, exp.Body.Bytes())
	if d := docx["word/document.xml"]; !strings.Contains(d, ">Quartal</w:t>") || !strings.Contains(d, ">bert</w:t>") || !strings.Contains(d, `<w:pStyle w:val="berschrift1"/>`) || !strings.Contains(d, ">Ausgangslage</w:t>") {
		t.Fatalf("exported document: %s", d)
	}
	if !strings.Contains(docx["word/header1.xml"], "Bundesamt") {
		t.Fatal("the export lost the template's header")
	}

	// A new version: the page stays on its own until moved.
	if r := w.admin.post("/api/word-templates/bericht", "application/octet-stream", dotx(t, "Neues Amt")); r.Code != http.StatusCreated {
		t.Fatalf("second version: %d %s", r.Code, r.Body)
	}
	if t2 := w.admin.expect("GET", "/api/word-templates", "", http.StatusOK)["templates"].([]any)[0].(map[string]any); t2["name"] != "Bericht" || len(t2["versions"].([]any)) != 2 || t2["items"] != 2.0 {
		t.Fatalf("after the second version: %v", t2)
	}
	exp = w.bert.do("GET", "/api/items/"+key+"/export/docx", "")
	if !strings.Contains(unzipParts(t, exp.Body.Bytes())["word/header1.xml"], "Bundesamt") {
		t.Fatal("the page exports with a version it was not made with")
	}
	doc = w.bert.expect("GET", "/api/items/"+key+"/document", "", http.StatusOK)
	if field(doc, "template", "version") != 1.0 || field(doc, "template", "newest") != 2.0 {
		t.Fatalf("document after a new version: %v", doc["template"])
	}
	w.vera.expect("POST", "/api/items/"+key+"/template", "", http.StatusForbidden)
	w.bert.expect("POST", "/api/items/"+key+"/template", `{"version":3}`, http.StatusBadRequest)
	moved := w.bert.expect("POST", "/api/items/"+key+"/template", "", http.StatusOK)
	if field(moved, "item", "template", "version") != 2.0 {
		t.Fatalf("moved %v", moved)
	}
	exp = w.bert.do("GET", "/api/items/"+key+"/export/docx", "")
	if !strings.Contains(unzipParts(t, exp.Body.Bytes())["word/header1.xml"], "Neues Amt") {
		t.Fatal("the moved page exports with the old version")
	}
	if back := w.bert.expect("POST", "/api/items/"+key+"/template", `{"version":1}`, http.StatusOK); field(back, "item", "template", "version") != 1.0 {
		t.Fatalf("back %v", back)
	}
	// A copy keeps its original's version.
	cp := w.bert.expect("POST", "/api/items/"+key+"/copy", `{"space":"PLAT","title":"Kopie"}`, http.StatusCreated)
	copied := w.bert.expect("GET", "/api/items/"+field(cp, "item", "key").(string), "", http.StatusOK)
	if field(copied, "item", "template", "version") != 1.0 {
		t.Fatalf("copy %v", copied["item"])
	}

	// Renaming, downloading, and removing only what nothing uses.
	w.admin.expect("PUT", "/api/word-templates/bericht", `{"name":"Jahresbericht"}`, http.StatusOK)
	w.admin.expect("PUT", "/api/word-templates/none", `{"name":"X"}`, http.StatusNotFound)
	w.admin.expect("PUT", "/api/word-templates/bericht", `{"name":""}`, http.StatusBadRequest)
	file := w.admin.do("GET", "/api/word-templates/bericht/versions/1/file", "")
	if file.Code != http.StatusOK || !strings.Contains(file.Header().Get("Content-Disposition"), "Bericht.dotx") {
		t.Fatalf("download %d %v", file.Code, file.Header())
	}
	w.bert.expect("GET", "/api/word-templates/bericht/versions/1/file", "", http.StatusForbidden)
	w.admin.expect("GET", "/api/word-templates/bericht/versions/7/file", "", http.StatusNotFound)
	w.admin.expect("DELETE", "/api/word-templates/bericht", "", http.StatusConflict)
	w.admin.expect("DELETE", "/api/word-templates/none", "", http.StatusNotFound)
	if r := w.admin.post("/api/word-templates/memo?name=Memo", "application/octet-stream", tpl); r.Code != http.StatusCreated {
		t.Fatalf("a second template: %d", r.Code)
	}
	w.bert.expect("DELETE", "/api/word-templates/memo", "", http.StatusForbidden)
	w.admin.expect("DELETE", "/api/word-templates/memo", "", http.StatusNoContent)
}

func TestAWordTemplateLargerThanTheBudgetIsRefused(t *testing.T) {
	w := newWorld(t)
	b := budget.Get()
	small := b
	small.WordTemplateBytes = 100
	budget.Set(small)
	t.Cleanup(func() { budget.Set(b) })
	if r := w.admin.post("/api/word-templates/bericht?name=B", "application/octet-stream", dotx(t, "x")); r.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("a large template: %d", r.Code)
	}
}

func TestStartMarkdownRoundTrips(t *testing.T) {
	blocks := startFromMarkdown("# Eins\n\nText mit [Link](https://example.org).\n\n[TOC]\n\n- a\n- b\n\n  [children]  \n")
	var kinds []string
	for _, b := range blocks {
		kinds = append(kinds, string(b.Kind))
	}
	if strings.Join(kinds, " ") != "heading paragraph toc bullet bullet children" || blocks[0].Level != 1 {
		t.Fatalf("blocks %v", kinds)
	}
	md := startMarkdown(blocks)
	if !strings.HasPrefix(md, "# Eins\n\nText mit [Link](https://example.org).\n\n[toc]\n\n- a\n- b\n\n[children]") {
		t.Fatalf("markdown %q", md)
	}
	if startMarkdown(nil) != "" || len(startFromMarkdown("  \n")) != 0 {
		t.Error("empty start text")
	}
	if got := controls(nil, item.Item{Title: "T", Key: "K-1", Labels: []string{"a", "b"}, Properties: item.Props{"Owner": "u1"}}, item.TypeDef{Fields: []item.FieldDef{{Name: "Owner", Kind: item.KindUser}}}, "Raum", nil); got["owner"] != "u1" || got["labels"] != "a, b" || got["space"] != "Raum" {
		t.Errorf("controls %v", got)
	}
}

func unzipParts(t *testing.T, data []byte) map[string]string {
	t.Helper()
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, f := range z.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		out[f.Name] = string(b)
	}
	return out
}
