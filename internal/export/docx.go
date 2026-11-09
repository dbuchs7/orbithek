package export

import (
	"archive/zip"
	"bytes"
	"cmp"
	"fmt"
	"image"
	_ "image/gif" // the formats image.DecodeConfig reads for picture sizes
	_ "image/jpeg"
	_ "image/png"
	"io"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/dbuchs7/orbithek/internal/item"
	"github.com/dbuchs7/orbithek/internal/wordtemplate"
)

// Document is what an export writes: one page, or a page with the pages
// below it as a handbook (ADR-0071).
type Document struct {
	// Pages are in reading order; the first is the root, Depth 0.
	Pages []Page
	// Embeds are the items the bodies embed, by ID, as far as the reader
	// may read them; an embed missing here is shown as Restricted.
	Embeds     map[item.ID]Page
	Restricted string
	// Contents, when set, heads a list of the pages at the start.
	Contents string
	// Figure and Table name an entry of a list of figures or of tables,
	// "Figure 1: caption" (ADR-0119); English when empty.
	Figure, Table string
	// Template, when set, is the Word template the document is written
	// into (ADR-0167): its styles, page, header, footer and cover, the
	// text where it marks it. Controls are the texts of its content
	// controls by tag; the facts of the root take their place.
	Template *wordtemplate.Template
	Controls map[string]string
}

// diagramName names what a bpmn, process, form, decision, architecture or
// start form block shows, for documents that do not draw it (ADR-0129,
// ADR-0144, ADR-0146, ADR-0148): "BPMN:" and the file, or the process,
// form or decision and its engine. An architecture sketch holds only its
// application's ID, so it is named by its caption, which the picker fills
// with the application's name, and its release or "newest release". A
// start form is named by the process it starts, or by its caption when
// the link was pasted without one; its link is a document's to keep out,
// since anyone who holds it starts the process.
func diagramName(bl item.Block, files map[item.ID]Image) string {
	switch bl.Kind {
	case item.Process:
		return "BPMN: " + bl.Process + " (" + bl.Engine + ")"
	case item.Form:
		return "Form: " + bl.Form + " (" + bl.Engine + ")"
	case item.Decision:
		return "Decision: " + bl.Decision + " (" + bl.Engine + ")"
	case item.Architecture:
		name := cmp.Or(strings.TrimSpace(bl.Text), bl.Application)
		if bl.Release == 0 {
			return "Architecture sketch: " + name + ", newest release (" + bl.Engine + ")"
		}
		return fmt.Sprintf("Architecture sketch: %s v%d (%s)", name, bl.Release, bl.Engine)
	case item.StartForm:
		return "Start form: " + cmp.Or(bl.Process, strings.TrimSpace(bl.Text), "public start link") + " (" + bl.Engine + ")"
	}
	return "BPMN: " + files[bl.Ref].Name
}

// namedByCaption reports whether a block's name in a document is its
// caption, which is then not written again below it.
func namedByCaption(bl item.Block) bool {
	return bl.Kind == item.Architecture || bl.Kind == item.StartForm && bl.Process == ""
}

// listing is what a list of figures (kind Figures) or of tables (Tables)
// shows for a body: each picture or table, numbered in the order of the
// body, with its caption or a table's header row (ADR-0119).
func listing(d Document, body []item.Block, kind item.BlockKind) []string {
	figure, table := cmp.Or(d.Figure, "Figure"), cmp.Or(d.Table, "Table")
	var out []string
	for _, b := range body {
		switch {
		case kind == item.Figures && b.Kind == item.Image:
			out = append(out, entry(fmt.Sprintf("%s %d", figure, len(out)+1), b.Text))
		case kind == item.Tables && b.Kind == item.Table:
			head, _, _ := strings.Cut(b.Text, "\n")
			var cells []string
			for _, c := range strings.Split(head, "\t") {
				if c = strings.TrimSpace(c); c != "" {
					cells = append(cells, c)
				}
			}
			out = append(out, entry(fmt.Sprintf("%s %d", table, len(out)+1), strings.Join(cells, ", ")))
		}
	}
	return out
}

func entry(label, text string) string {
	if text = strings.TrimSpace(text); text != "" {
		return label + ": " + text
	}
	return label
}

// TaskListKey is what a task list block lists its tasks by: its label,
// or its query (ADR-0151), told apart, as Page.Tasks keys them.
func TaskListKey(b item.Block) string {
	if b.Query != "" {
		return "?" + b.Query
	}
	return b.Text
}

// TaskListTitle is the line a task list starts with in a document: its
// label as a label is written, or its query.
func TaskListTitle(b item.Block) string {
	if b.Query != "" {
		return b.Query
	}
	return "# " + b.Text
}

// Page is an item as exported.
type Page struct {
	ID    item.ID
	Key   string
	Title string
	// Facts are label and value pairs shown under the title: the key, the
	// type, the state and the properties.
	Facts [][2]string
	Body  []item.Block
	// Images are the item's pictures by attachment ID; a picture block
	// whose file is missing or in a format the target cannot show keeps
	// only its caption.
	Images map[item.ID]Image
	// Children are the titles of the child pages, for a list of them.
	Children []string
	// Tasks are, by TaskListKey, the tasks a task list shows: key, title
	// and state, as a reader read them when the document was made
	// (ADR-0114, ADR-0151).
	Tasks map[string][]string
	// Depth is the page's level below the root.
	Depth int
}

// Image is a picture file.
type Image struct {
	Name, Type string
	Data       []byte
}

// maxEmbedDepth bounds how deep embedded items are written inside each
// other; deeper ones, and an item inside itself, show by key and title.
const maxEmbedDepth = 3

// name names an item: key and title.
func (p Page) name() string { return strings.TrimSpace(p.Key + " " + p.Title) }

// textWidth is the width of the text column of an A4 page with margins of
// 2.5 cm, in EMU (English Metric Units, 914400 to the inch).
const textWidth = 5760000

// emuPerPixel converts pixels at 96 dpi.
const emuPerPixel = 9525

// panelShades are the background colours of the panel tones.
var panelShades = map[string]string{"info": "E8F0FE", "note": "F3EEFC", "warning": "FFF6E0", "tip": "E9F6EC", "error": "FDECEA"}

// DOCX writes the document as an Office Open XML word-processing file.
func DOCX(w io.Writer, d Document) error {
	if d.Template != nil {
		return docxFromTemplate(w, d)
	}
	b := newBuilder(d)
	body := b.body()
	z := zip.NewWriter(w)
	files := []struct{ name, data string }{
		{"[Content_Types].xml", contentTypes},
		{"_rels/.rels", rootRels},
		{"docProps/core.xml", fmt.Sprintf(coreProps, esc(b.title()))},
		{"word/document.xml", documentHead + body + sectPr + "</w:body></w:document>"},
		{"word/styles.xml", styles},
		{"word/numbering.xml", b.numbering()},
		{"word/_rels/document.xml.rels", b.rels()},
	}
	for _, f := range files {
		fw, err := z.Create(f.name)
		if err != nil {
			return err
		}
		if _, err := io.WriteString(fw, f.data); err != nil {
			return err
		}
	}
	for _, m := range b.media {
		fw, err := z.Create("word/media/" + m.file)
		if err != nil {
			return err
		}
		if _, err := fw.Write(m.data); err != nil {
			return err
		}
	}
	return z.Close()
}

type media struct {
	id, file string
	data     []byte
}

type docxBuilder struct {
	d       Document
	links   []string // targets of hyperlinks; rId is rel, "L" and the index
	media   []media
	ordered int // ordered lists so far; each gets its own numbering
	shapes  int
	// rel starts the IDs of relationships, mediaPrefix the names of
	// pictures; numBase is the bullet list's numbering, the ordered lists'
	// follow it, and absBase the first abstract numbering; width is the
	// text column in EMU. A template sets them so nothing collides with
	// its own (ADR-0167).
	rel, mediaPrefix string
	numBase, absBase int
	width            int64
	// tpl is the template written into; style maps the export's style IDs
	// to its, adding what it lacks to added.
	tpl    *wordtemplate.Template
	styleN map[string]string
	added  []string
	lists  bool
}

func newBuilder(d Document) *docxBuilder {
	return &docxBuilder{d: d, rel: "r", numBase: 1, width: textWidth}
}

// s is the ID of one of the export's styles in the document written.
func (b *docxBuilder) s(id string) string {
	if b.tpl == nil {
		return id
	}
	if got, ok := b.styleN[id]; ok {
		return got
	}
	def := styleDefs[id]
	got := b.tpl.StyleID(def.typ, def.name)
	if got == "" {
		got = id
		for b.tpl.StyleIDTaken(got) {
			got = "Orbithek" + got
		}
		x := strings.Replace(def.xml, `w:styleId="`+id+`"`, `w:styleId="`+got+`"`, 1)
		normal := b.tpl.DefaultParagraphStyle()
		x = strings.ReplaceAll(x, `<w:basedOn w:val="Normal"/>`, `<w:basedOn w:val="`+normal+`"/>`)
		x = strings.ReplaceAll(x, `<w:next w:val="Normal"/>`, `<w:next w:val="`+normal+`"/>`)
		b.added = append(b.added, x)
	}
	b.styleN[id] = got
	return got
}

// pStyle is a paragraph's style property.
func (b *docxBuilder) pStyle(id string) string { return `<w:pStyle w:val="` + b.s(id) + `"/>` }

// bulletNum is the bullet list's numbering; an ordered list's is listNum.
func (b *docxBuilder) bulletNum() string {
	b.lists = true
	return fmt.Sprintf(`<w:numPr><w:ilvl w:val="0"/><w:numId w:val="%d"/></w:numPr>`, b.numBase)
}

func (b *docxBuilder) body() string {
	var out strings.Builder
	if len(b.d.Pages) == 0 {
		return ""
	}
	root := b.d.Pages[0]
	// A template with a control for the title shows it there.
	if b.tpl == nil || !b.tpl.HasControl("title") {
		out.WriteString(para(b.s("Title"), "", runs(b, root.Title, nil)))
	}
	if b.d.Contents != "" && len(b.d.Pages) > 1 {
		out.WriteString(para(b.s("TOCHeading"), "", run(b.d.Contents, "")))
		for _, p := range b.d.Pages[1:] {
			out.WriteString(paraPr(fmt.Sprintf(`<w:ind w:left="%d"/>`, (p.Depth-1)*360), run(p.Title, "")))
		}
	}
	for i, p := range b.d.Pages {
		if i > 0 {
			// A chapter starts on a new page; a section below it does not.
			pPr := b.pStyle(fmt.Sprintf("Heading%d", min(p.Depth, 6)))
			if p.Depth == 1 {
				pPr += `<w:pageBreakBefore/>`
			}
			out.WriteString(paraPr(pPr, runs(b, p.Title, nil)))
		}
		b.page(&out, p, p.Depth, map[item.ID]bool{p.ID: true}, 0)
	}
	return out.String()
}

// page writes an item's facts and body; base is the heading level of its
// title, so its own headings come below it.
func (b *docxBuilder) page(out *strings.Builder, p Page, base int, path map[item.ID]bool, embedDepth int) {
	// Under a template its controls show the facts (ADR-0167).
	if len(p.Facts) > 0 && b.tpl == nil {
		var rows [][]string
		for _, f := range p.Facts {
			rows = append(rows, []string{f[0], f[1]})
		}
		out.WriteString(b.table(rows, false))
		out.WriteString(para("", "", ""))
	}
	listNum := 0
	for i, bl := range p.Body {
		if bl.Kind == item.OrderedItem && (i == 0 || p.Body[i-1].Kind != item.OrderedItem) {
			b.ordered++
			listNum = b.numBase + b.ordered
		}
		if bl.Kind == item.Embed {
			b.embed(out, bl.Ref, base, path, embedDepth)
			continue
		}
		out.WriteString(b.block(bl, p, base, listNum))
	}
}

// embed writes an embedded item where it is embedded: its name, facts and
// body, or its name alone when it is too deep or already open around it.
func (b *docxBuilder) embed(out *strings.Builder, id item.ID, base int, path map[item.ID]bool, embedDepth int) {
	e, ok := b.d.Embeds[id]
	if !ok {
		out.WriteString(para(b.s("EmbedTitle"), "", run("↳ "+b.d.Restricted, "")))
		return
	}
	out.WriteString(para(b.s("EmbedTitle"), "", run("↳ "+e.name(), "")))
	if path[id] || embedDepth >= maxEmbedDepth {
		return
	}
	path[id] = true
	b.page(out, e, base+1, path, embedDepth+1)
	delete(path, id)
	out.WriteString(paraPr(`<w:pBdr><w:bottom w:val="dotted" w:sz="4" w:space="1" w:color="BFBFBF"/></w:pBdr>`, ""))
}

func (b *docxBuilder) block(bl item.Block, p Page, base, listNum int) string {
	jc := ""
	switch bl.Align {
	case "center", "right":
		jc = bl.Align
	case "justify":
		jc = "both"
	}
	text := runs(b, bl.Text, bl.Marks)
	switch bl.Kind {
	case item.Heading:
		level := min(base+min(max(bl.Level, 1), 3), 6)
		return paraPr(b.pStyle(fmt.Sprintf("Heading%d", level))+align(jc), text)
	case item.BulletItem:
		return paraPr(b.pStyle("ListParagraph")+b.bulletNum(), text)
	case item.OrderedItem:
		b.lists = true
		return paraPr(b.pStyle("ListParagraph")+fmt.Sprintf(`<w:numPr><w:ilvl w:val="0"/><w:numId w:val="%d"/></w:numPr>`, listNum), text)
	case item.TaskItem:
		box := "☐ "
		if bl.Checked {
			box = "☑ "
		}
		return para(b.s("ListParagraph"), "", run(box, "")+text)
	case item.Quote:
		return paraPr(b.pStyle("Quote")+align(jc), text)
	case item.CodeBlock:
		return para(b.s("Code"), "", run(bl.Text, ""))
	case item.Panel:
		shade := panelShades[bl.Tone]
		if shade == "" {
			shade = panelShades["info"]
		}
		return paraPr(b.pStyle("Panel")+`<w:shd w:val="clear" w:color="auto" w:fill="`+shade+`"/>`+align(jc), text)
	case item.Table:
		return b.table(TableRows(bl.Text), true)
	case item.Divider:
		return paraPr(`<w:pBdr><w:bottom w:val="single" w:sz="6" w:space="1" w:color="BFBFBF"/></w:pBdr>`, "")
	case item.Image:
		return b.picture(bl, p.Images)
	case item.Video:
		// A document does not play a video; it names it (ADR-0097).
		out := para("", "", run("[Video: "+p.Images[bl.Ref].Name+"]", ""))
		if strings.TrimSpace(bl.Text) != "" {
			out += para(b.s("Caption"), "", run(bl.Text, ""))
		}
		return out
	case item.BPMN, item.Process, item.Form, item.Decision, item.Architecture, item.StartForm:
		// A document does not draw a diagram, a form, a decision or a
		// sketch; it names the file, the process, the form, the decision,
		// the sketch or the start form (ADR-0129, ADR-0144, ADR-0146,
		// ADR-0148).
		out := para("", "", run("["+diagramName(bl, p.Images)+"]", ""))
		if strings.TrimSpace(bl.Text) != "" && !namedByCaption(bl) {
			out += para(b.s("Caption"), "", run(bl.Text, ""))
		}
		return out
	case item.Figures, item.Tables:
		var out strings.Builder
		for _, e := range listing(b.d, p.Body, bl.Kind) {
			out.WriteString(para("", "", run(e, "")))
		}
		return out.String()
	case item.TOC:
		if b.tpl != nil {
			return b.tocField(p.Body)
		}
		var out strings.Builder
		for _, h := range p.Body {
			if h.Kind == item.Heading && strings.TrimSpace(h.Text) != "" {
				out.WriteString(paraPr(fmt.Sprintf(`<w:ind w:left="%d"/>`, (max(h.Level, 1)-1)*360), run(h.Text, "")))
			}
		}
		return out.String()
	case item.TaskList:
		out := strings.Builder{}
		out.WriteString(para("", "", run(TaskListTitle(bl), "")))
		for _, c := range p.Tasks[TaskListKey(bl)] {
			out.WriteString(paraPr(b.pStyle("ListParagraph")+b.bulletNum(), run(c, "")))
		}
		return out.String()
	case item.Children:
		var out strings.Builder
		for _, c := range p.Children {
			out.WriteString(paraPr(b.pStyle("ListParagraph")+b.bulletNum(), run(c, "")))
		}
		return out.String()
	default: // paragraph, and content an import could not convert
		return paraPr(align(jc), text)
	}
}

// picture places an image of the item, sized as in the page.
func (b *docxBuilder) picture(bl item.Block, images map[item.ID]Image) string {
	caption := ""
	if strings.TrimSpace(bl.Text) != "" {
		caption = para(b.s("Caption"), "", run(bl.Text, ""))
	}
	img, ok := images[bl.Ref]
	if !ok {
		return caption
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(img.Data))
	if err != nil || cfg.Width == 0 || cfg.Height == 0 {
		return para("", "", run("["+img.Name+"]", "")) + caption
	}
	ext := map[string]string{"png": "png", "jpeg": "jpeg", "gif": "gif"}[format]
	cx := int64(cfg.Width) * emuPerPixel
	if bl.Width > 0 {
		cx = b.width * int64(bl.Width) / 100
	}
	cx = min(cx, b.width)
	cy := cx * int64(cfg.Height) / int64(cfg.Width)
	b.shapes++
	id := fmt.Sprintf("%sI%d", b.rel, b.shapes)
	b.media = append(b.media, media{id: id, file: fmt.Sprintf("%simage%d.%s", b.mediaPrefix, b.shapes, ext), data: img.Data})
	jc := `<w:jc w:val="center"/>`
	switch bl.Align {
	case "left", "right":
		jc = `<w:jc w:val="` + bl.Align + `"/>`
	}
	drawing := fmt.Sprintf(`<w:r><w:drawing><wp:inline distT="0" distB="0" distL="0" distR="0"><wp:extent cx="%d" cy="%d"/>`+
		`<wp:docPr id="%d" name="Picture %d" descr="%s"/><a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">`+
		`<a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture">`+
		`<pic:nvPicPr><pic:cNvPr id="%d" name="%s"/><pic:cNvPicPr/></pic:nvPicPr><pic:blipFill><a:blip r:embed="%s"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill>`+
		`<pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr></pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing></w:r>`,
		cx, cy, b.shapes, b.shapes, esc(bl.Text), b.shapes, esc(img.Name), id, cx, cy)
	return paraPr(jc, drawing) + caption
}

// segment is a stretch of text with one set of marks.
type segment struct {
	text                                          string
	bold, italic, under, strike, code, high, link bool
	href                                          string
}

// segments cuts a text at its marks' boundaries, counted in code points.
func segments(text string, marks []item.Mark) []segment {
	if text == "" {
		return nil
	}
	n := utf8.RuneCountInString(text)
	cuts := []int{0, n}
	for _, m := range marks {
		cuts = append(cuts, min(max(m.Start, 0), n), min(max(m.End, 0), n))
	}
	slices.Sort(cuts)
	cuts = slices.Compact(cuts)
	chars := []rune(text)
	var out []segment
	for i := 0; i+1 < len(cuts); i++ {
		from, to := cuts[i], cuts[i+1]
		if from >= to {
			continue
		}
		sg := segment{text: string(chars[from:to])}
		for _, m := range marks {
			if m.Start > from || m.End < to {
				continue
			}
			switch m.Type {
			case item.MarkBold:
				sg.bold = true
			case item.MarkItalic:
				sg.italic = true
			case item.MarkUnderline:
				sg.under = true
			case item.MarkStrike:
				sg.strike = true
			case item.MarkCode:
				sg.code = true
			case item.MarkHighlight:
				sg.high = true
			case item.MarkLink:
				sg.link, sg.href = safeLink(m.Href), m.Href
			}
		}
		out = append(out, sg)
	}
	return out
}

// runs writes a text with its marks as runs of equal formatting.
func runs(b *docxBuilder, text string, marks []item.Mark) string {
	var out strings.Builder
	for _, sg := range segments(text, marks) {
		var props string
		// The order of the run properties is the schema's.
		if sg.link {
			props += `<w:rStyle w:val="` + b.s("Hyperlink") + `"/>`
		} else if sg.code {
			props += `<w:rStyle w:val="` + b.s("CodeChar") + `"/>`
		}
		if sg.bold {
			props += "<w:b/>"
		}
		if sg.italic {
			props += "<w:i/>"
		}
		if sg.strike {
			props += "<w:strike/>"
		}
		if sg.high {
			props += `<w:highlight w:val="yellow"/>`
		}
		if sg.under {
			props += `<w:u w:val="single"/>`
		}
		r := run(sg.text, props)
		if sg.link {
			b.links = append(b.links, sg.href)
			r = fmt.Sprintf(`<w:hyperlink r:id="%sL%d">%s</w:hyperlink>`, b.rel, len(b.links), r)
		}
		out.WriteString(r)
	}
	return out.String()
}

// safeLink admits the link targets a page admits (ADR-0014).
func safeLink(href string) bool {
	h := strings.ToLower(href)
	return strings.HasPrefix(h, "https://") || strings.HasPrefix(h, "http://") || strings.HasPrefix(h, "mailto:")
}

// run is one run of text; line breaks become breaks.
func run(text, props string) string {
	var out strings.Builder
	out.WriteString("<w:r>")
	if props != "" {
		out.WriteString("<w:rPr>" + props + "</w:rPr>")
	}
	for i, line := range strings.Split(text, "\n") {
		if i > 0 {
			out.WriteString("<w:br/>")
		}
		out.WriteString(`<w:t xml:space="preserve">` + esc(line) + "</w:t>")
	}
	out.WriteString("</w:r>")
	return out.String()
}

func para(style, props, content string) string {
	if style != "" {
		props = `<w:pStyle w:val="` + style + `"/>` + props
	}
	return paraPr(props, content)
}

func paraPr(props, content string) string {
	if props != "" {
		props = "<w:pPr>" + props + "</w:pPr>"
	}
	return "<w:p>" + props + content + "</w:p>"
}

func align(jc string) string {
	if jc == "" {
		return ""
	}
	return `<w:jc w:val="` + jc + `"/>`
}

// table writes rows as a table with borders; the first row is a header
// when header is set.
func (b *docxBuilder) table(rows [][]string, header bool) string {
	if len(rows) == 0 {
		return ""
	}
	cols := 0
	for _, r := range rows {
		cols = max(cols, len(r))
	}
	var out strings.Builder
	out.WriteString(`<w:tbl><w:tblPr><w:tblStyle w:val="` + b.s("TableGrid") + `"/><w:tblW w:w="5000" w:type="pct"/></w:tblPr><w:tblGrid>`)
	for range cols {
		fmt.Fprintf(&out, `<w:gridCol w:w="%d"/>`, int(b.width/635)/cols)
	}
	out.WriteString("</w:tblGrid>")
	for i, r := range rows {
		head := header && i == 0
		out.WriteString("<w:tr>")
		if head {
			out.WriteString(`<w:trPr><w:tblHeader/></w:trPr>`)
		}
		for c := range cols {
			cell := ""
			if c < len(r) {
				cell = r[c]
			}
			props := ""
			cellPr := ""
			if head {
				props = "<w:b/>"
				cellPr = `<w:tcPr><w:shd w:val="clear" w:color="auto" w:fill="F2F2F2"/></w:tcPr>`
			}
			out.WriteString("<w:tc>" + cellPr + paraPr("", run(cell, props)) + "</w:tc>")
		}
		out.WriteString("</w:tr>")
	}
	out.WriteString("</w:tbl>")
	return out.String()
}

func (b *docxBuilder) title() string {
	if len(b.d.Pages) == 0 {
		return ""
	}
	return b.d.Pages[0].Title
}

func (b *docxBuilder) rels() string {
	var out strings.Builder
	out.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rS" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>` +
		`<Relationship Id="rN" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering" Target="numbering.xml"/>`)
	for i, l := range b.links {
		fmt.Fprintf(&out, `<Relationship Id="rL%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="%s" TargetMode="External"/>`, i+1, esc(l))
	}
	for _, m := range b.media {
		fmt.Fprintf(&out, `<Relationship Id="%s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/%s"/>`, m.id, m.file)
	}
	out.WriteString("</Relationships>")
	return out.String()
}

// numbering defines the bullet list (numBase) and one decimal list per
// ordered list of the page (numBase+1 and on), each starting at 1.
func (b *docxBuilder) numbering() string {
	abs, nums := b.numberingDefs()
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		strings.Join(abs, "") + strings.Join(nums, "") + "</w:numbering>"
}

// numberingDefs are the abstract numberings and the numberings of the
// document's lists.
func (b *docxBuilder) numberingDefs() (abs, nums []string) {
	abs = []string{
		fmt.Sprintf(`<w:abstractNum w:abstractNumId="%d"><w:multiLevelType w:val="singleLevel"/><w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="•"/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="720" w:hanging="360"/></w:pPr></w:lvl></w:abstractNum>`, b.absBase),
		fmt.Sprintf(`<w:abstractNum w:abstractNumId="%d"><w:multiLevelType w:val="singleLevel"/><w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%%1."/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="720" w:hanging="360"/></w:pPr></w:lvl></w:abstractNum>`, b.absBase+1),
	}
	nums = []string{fmt.Sprintf(`<w:num w:numId="%d"><w:abstractNumId w:val="%d"/></w:num>`, b.numBase, b.absBase)}
	for i := 1; i <= b.ordered; i++ {
		nums = append(nums, fmt.Sprintf(`<w:num w:numId="%d"><w:abstractNumId w:val="%d"/><w:lvlOverride w:ilvl="0"><w:startOverride w:val="1"/></w:lvlOverride></w:num>`, b.numBase+i, b.absBase+1))
	}
	return abs, nums
}

// esc escapes text for XML and drops the characters XML cannot hold.
func esc(s string) string {
	var out strings.Builder
	for _, r := range s {
		switch {
		case r == '&':
			out.WriteString("&amp;")
		case r == '<':
			out.WriteString("&lt;")
		case r == '>':
			out.WriteString("&gt;")
		case r == '"':
			out.WriteString("&quot;")
		case r == '\t' || r == '\n' || r == '\r' || (r >= 0x20 && r <= 0xD7FF) || (r >= 0xE000 && r <= 0xFFFD) || r >= 0x10000:
			out.WriteRune(r)
		}
	}
	return out.String()
}

const contentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
	`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
	`<Default Extension="xml" ContentType="application/xml"/>` +
	`<Default Extension="png" ContentType="image/png"/><Default Extension="jpeg" ContentType="image/jpeg"/><Default Extension="gif" ContentType="image/gif"/>` +
	`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
	`<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>` +
	`<Override PartName="/word/numbering.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"/>` +
	`<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>` +
	`</Types>`

const rootRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
	`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>` +
	`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>` +
	`</Relationships>`

const coreProps = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/">` +
	`<dc:title>%s</dc:title><dc:creator>Orbithek</dc:creator></cp:coreProperties>`

const documentHead = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
	`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
	`xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"><w:body>`

// sectPr is an A4 page with margins of 2.5 cm.
const sectPr = `<w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1417" w:right="1417" w:bottom="1134" w:left="1417" w:header="708" w:footer="708" w:gutter="0"/></w:sectPr>`

const styles = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
	`<w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri" w:eastAsia="Calibri" w:cs="Calibri"/><w:sz w:val="22"/><w:szCs w:val="22"/><w:lang w:val="de-CH"/></w:rPr></w:rPrDefault>` +
	`<w:pPrDefault><w:pPr><w:spacing w:after="120" w:line="276" w:lineRule="auto"/></w:pPr></w:pPrDefault></w:docDefaults>` +
	`<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/><w:qFormat/></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Title"><w:name w:val="Title"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/><w:pPr><w:spacing w:after="240"/></w:pPr><w:rPr><w:b/><w:sz w:val="40"/><w:szCs w:val="40"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/><w:pPr><w:keepNext/><w:spacing w:before="360" w:after="120"/><w:outlineLvl w:val="0"/></w:pPr><w:rPr><w:b/><w:sz w:val="32"/><w:szCs w:val="32"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Heading2"><w:name w:val="heading 2"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/><w:pPr><w:keepNext/><w:spacing w:before="240" w:after="120"/><w:outlineLvl w:val="1"/></w:pPr><w:rPr><w:b/><w:sz w:val="28"/><w:szCs w:val="28"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Heading3"><w:name w:val="heading 3"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/><w:pPr><w:keepNext/><w:spacing w:before="200" w:after="80"/><w:outlineLvl w:val="2"/></w:pPr><w:rPr><w:b/><w:sz w:val="24"/><w:szCs w:val="24"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Heading4"><w:name w:val="heading 4"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/><w:pPr><w:keepNext/><w:spacing w:before="160" w:after="60"/><w:outlineLvl w:val="3"/></w:pPr><w:rPr><w:b/><w:sz w:val="22"/><w:szCs w:val="22"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Heading5"><w:name w:val="heading 5"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/><w:pPr><w:keepNext/><w:spacing w:before="120" w:after="60"/><w:outlineLvl w:val="4"/></w:pPr><w:rPr><w:b/><w:i/><w:sz w:val="22"/><w:szCs w:val="22"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Heading6"><w:name w:val="heading 6"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/><w:pPr><w:keepNext/><w:spacing w:before="120" w:after="40"/><w:outlineLvl w:val="5"/></w:pPr><w:rPr><w:i/><w:sz w:val="22"/><w:szCs w:val="22"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="TOCHeading"><w:name w:val="TOC Heading"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:pPr><w:spacing w:before="240" w:after="120"/></w:pPr><w:rPr><w:b/><w:sz w:val="26"/><w:szCs w:val="26"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="EmbedTitle"><w:name w:val="Embedded item"/><w:basedOn w:val="Normal"/><w:pPr><w:keepNext/><w:pBdr><w:top w:val="dotted" w:sz="4" w:space="1" w:color="BFBFBF"/></w:pBdr><w:spacing w:before="160" w:after="80"/></w:pPr><w:rPr><w:b/><w:color w:val="404040"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="ListParagraph"><w:name w:val="List Paragraph"/><w:basedOn w:val="Normal"/><w:qFormat/><w:pPr><w:spacing w:after="60"/><w:ind w:left="720"/></w:pPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Quote"><w:name w:val="Quote"/><w:basedOn w:val="Normal"/><w:qFormat/><w:pPr><w:pBdr><w:left w:val="single" w:sz="18" w:space="8" w:color="BFBFBF"/></w:pBdr><w:ind w:left="360"/></w:pPr><w:rPr><w:i/><w:color w:val="595959"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Code"><w:name w:val="Code"/><w:basedOn w:val="Normal"/><w:pPr><w:shd w:val="clear" w:color="auto" w:fill="F4F5F7"/><w:spacing w:after="120" w:line="240" w:lineRule="auto"/></w:pPr><w:rPr><w:rFonts w:ascii="Consolas" w:hAnsi="Consolas" w:cs="Consolas"/><w:sz w:val="19"/><w:szCs w:val="19"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Panel"><w:name w:val="Panel"/><w:basedOn w:val="Normal"/><w:pPr><w:pBdr><w:left w:val="single" w:sz="24" w:space="8" w:color="8C8C8C"/></w:pBdr><w:ind w:left="240" w:right="120"/></w:pPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Caption"><w:name w:val="caption"/><w:basedOn w:val="Normal"/><w:pPr><w:jc w:val="center"/></w:pPr><w:rPr><w:i/><w:color w:val="595959"/><w:sz w:val="18"/><w:szCs w:val="18"/></w:rPr></w:style>` +
	`<w:style w:type="character" w:styleId="Hyperlink"><w:name w:val="Hyperlink"/><w:rPr><w:color w:val="0563C1"/><w:u w:val="single"/></w:rPr></w:style>` +
	`<w:style w:type="character" w:styleId="CodeChar"><w:name w:val="Code Char"/><w:rPr><w:rFonts w:ascii="Consolas" w:hAnsi="Consolas" w:cs="Consolas"/><w:shd w:val="clear" w:color="auto" w:fill="F4F5F7"/></w:rPr></w:style>` +
	`<w:style w:type="table" w:styleId="TableGrid"><w:name w:val="Table Grid"/><w:tblPr><w:tblBorders>` +
	`<w:top w:val="single" w:sz="4" w:space="0" w:color="BFBFBF"/><w:left w:val="single" w:sz="4" w:space="0" w:color="BFBFBF"/><w:bottom w:val="single" w:sz="4" w:space="0" w:color="BFBFBF"/>` +
	`<w:right w:val="single" w:sz="4" w:space="0" w:color="BFBFBF"/><w:insideH w:val="single" w:sz="4" w:space="0" w:color="BFBFBF"/><w:insideV w:val="single" w:sz="4" w:space="0" w:color="BFBFBF"/>` +
	`</w:tblBorders><w:tblCellMar><w:left w:w="100" w:type="dxa"/><w:right w:w="100" w:type="dxa"/></w:tblCellMar></w:tblPr></w:style>` +
	`</w:styles>`
