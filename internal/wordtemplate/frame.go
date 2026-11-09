package wordtemplate

import (
	"path"
	"regexp"
	"strconv"
	"strings"
)

// Frame is a page drawn as its template's document, for the reading view:
// the template's content split into sheets at its page breaks, the place
// of the page's text among it, and the header and footer each sheet
// carries. It holds text and validated values only; the browser builds
// every element of it itself (ADR-0014).
type Frame struct {
	Page   Page     `json:"page"`
	Sheets [][]Node `json:"sheets"`
	Header []Node   `json:"header"`
	Footer []Node   `json:"footer"`
	// FirstHeader and FirstFooter are the first sheet's, when TitlePage
	// says it has its own.
	TitlePage   bool   `json:"titlePage"`
	FirstHeader []Node `json:"firstHeader"`
	FirstFooter []Node `json:"firstFooter"`
}

// Node is a paragraph, a table, or the place of the page's text.
type Node struct {
	P     *Para  `json:"p,omitempty"`
	Table *Table `json:"table,omitempty"`
	Body  bool   `json:"body,omitempty"`
}

// Para is a paragraph: its style's class in the stylesheet, its own
// alignment, its runs.
type Para struct {
	Class string `json:"class"`
	Align string `json:"align,omitempty"`
	Runs  []Run  `json:"runs"`
}

// Run is a stretch of text with its own formatting over the paragraph's,
// a tab, a line break, a page number, or a picture.
type Run struct {
	Text      string  `json:"text,omitempty"`
	Tab       bool    `json:"tab,omitempty"`
	Break     bool    `json:"br,omitempty"`
	Field     string  `json:"field,omitempty"` // "page" or "pages": the browser writes the number
	Bold      *bool   `json:"b,omitempty"`
	Italic    *bool   `json:"i,omitempty"`
	Underline *bool   `json:"u,omitempty"`
	Strike    *bool   `json:"s,omitempty"`
	Caps      *bool   `json:"caps,omitempty"`
	Size      float64 `json:"size,omitempty"`
	Colour    string  `json:"color,omitempty"`
	Fill      string  `json:"fill,omitempty"`
	Font      string  `json:"font,omitempty"`
	Image     *Image  `json:"image,omitempty"`
}

// Image is a picture of the template, by its address, and its size in
// points.
type Image struct {
	Src    string  `json:"src"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Align  string  `json:"align,omitempty"`
}

// Table is a table: rows of cells, with or without borders.
type Table struct {
	Rows    [][]Cell `json:"rows"`
	Borders bool     `json:"borders"`
}

// Cell is a table cell.
type Cell struct {
	Nodes []Node  `json:"nodes"`
	Fill  string  `json:"fill,omitempty"`
	Width float64 `json:"width,omitempty"`
	Span  int     `json:"span,omitempty"`
}

// framer converts one part.
type framer struct {
	t        *Template
	part     string
	values   map[string]string
	media    func(name string) string
	sheets   [][]Node
	body     bool
	maxNodes int
	count    int
}

// maxFrameNodes bounds the nodes of one part's frame: the shape of a
// template's page, like the number of its parts.
const maxFrameNodes = 20000

// Frame draws the template with the controls' values filled in; media
// makes a picture's address from its name under word/media.
func (t *Template) Frame(values map[string]string, media func(name string) string) Frame {
	f := Frame{Page: t.info.Page, Header: []Node{}, Footer: []Node{}, FirstHeader: []Node{}, FirstFooter: []Node{}}
	fr := &framer{t: t, part: "word/document.xml", values: values, media: media, sheets: [][]Node{{}}, maxNodes: maxFrameNodes}
	fr.blocks(t.doc.child("w:body").kids)
	if !fr.body {
		last := len(fr.sheets) - 1
		fr.sheets[last] = append(fr.sheets[last], Node{Body: true})
	}
	// A sheet the template leaves empty after a break at its end is no
	// sheet of the document.
	for len(fr.sheets) > 1 && len(fr.sheets[len(fr.sheets)-1]) == 0 {
		fr.sheets = fr.sheets[:len(fr.sheets)-1]
	}
	f.Sheets = fr.sheets
	sect := t.sectPr()
	f.TitlePage = sect.child("w:titlePg") != nil
	if on, ok := sect.on("w:titlePg"); ok {
		f.TitlePage = on
	}
	for _, ref := range sect.kids {
		if ref.name != "w:headerReference" && ref.name != "w:footerReference" {
			continue
		}
		r, ok := t.relByID("word/document.xml", ref.attr["r:id"])
		if !ok || t.parts[r.target] == nil {
			continue
		}
		root, err := parse(t.parts[r.target])
		if err != nil {
			continue
		}
		pf := &framer{t: t, part: r.target, values: values, media: media, sheets: [][]Node{{}}, maxNodes: maxFrameNodes}
		pf.blocks(root.kids)
		var nodes []Node
		for _, s := range pf.sheets {
			nodes = append(nodes, s...)
		}
		if nodes == nil {
			nodes = []Node{}
		}
		header := ref.name == "w:headerReference"
		switch ref.attr["w:type"] {
		case "first":
			if header {
				f.FirstHeader = nodes
			} else {
				f.FirstFooter = nodes
			}
		case "even":
		default:
			if header {
				f.Header = nodes
			} else {
				f.Footer = nodes
			}
		}
	}
	return f
}

func (fr *framer) add(n Node) {
	if fr.count >= fr.maxNodes {
		return
	}
	fr.count++
	last := len(fr.sheets) - 1
	fr.sheets[last] = append(fr.sheets[last], n)
}

func (fr *framer) newSheet() { fr.sheets = append(fr.sheets, []Node{}) }

// blocks converts the children of a body, a cell or a control.
func (fr *framer) blocks(kids []*node) {
	for _, k := range kids {
		switch k.name {
		case "w:p":
			fr.paragraph(k)
		case "w:tbl":
			if tb := fr.table(k); tb != nil {
				fr.add(Node{Table: tb})
			}
		case "w:sdt":
			fr.blockControl(k)
		case "w:customXml", "w:ins", "w:smartTag":
			fr.blocks(k.kids)
		}
	}
}

// blockControl is a content control around paragraphs or tables: the
// page's text, a field filled in, or the template's content.
func (fr *framer) blockControl(sdt *node) {
	content := sdt.child("w:sdtContent")
	tag, ok := controlTag(sdt)
	switch {
	case ok && tag == BodyControl && fr.part == "word/document.xml" && !fr.body:
		fr.body = true
		fr.add(Node{Body: true})
		return
	case ok && fr.values != nil:
		if v, has := fr.values[tag]; has && content != nil {
			class, align := fr.paraStyle(content.path("w:p", "w:pPr"))
			r := fr.runProps(firstRunPr(content))
			fr.add(Node{P: &Para{Class: class, Align: align, Runs: textRuns(v, r)}})
			return
		}
	}
	if content != nil {
		fr.blocks(content.kids)
	}
}

// firstRunPr is the properties of the first run below n, if it has any.
func firstRunPr(n *node) *node {
	var out *node
	found := false
	n.walk(func(x *node) bool {
		if found {
			return false
		}
		if x.name == "w:r" {
			out, found = x.child("w:rPr"), true
			return false
		}
		return true
	})
	return out
}

// textRuns are the runs of a filled control: its text, line by line, in
// the formatting r.
func textRuns(v string, r Run) []Run {
	var out []Run
	for i, line := range strings.Split(v, "\n") {
		if i > 0 {
			out = append(out, Run{Break: true})
		}
		x := r
		x.Text = line
		out = append(out, x)
	}
	return out
}

// paraStyle is the class and the alignment of a paragraph.
func (fr *framer) paraStyle(pPr *node) (string, string) {
	id := fr.t.DefaultParagraphStyle()
	if v, ok := pPr.val("w:pStyle"); ok && fr.t.styles.byID[v] != nil {
		id = v
	}
	class := ""
	if st := fr.t.styles.byID[id]; st != nil {
		class = st.class
	}
	var p props
	fr.t.para(pPr, &p)
	return class, p.align
}

// runProps is a run's formatting over its paragraph's: its character
// style's and its own.
func (fr *framer) runProps(rPr *node) Run {
	var p props
	if v, ok := rPr.val("w:rStyle"); ok {
		for _, st := range fr.t.styles.chain(v) {
			fr.t.run(st.rPr, &p)
		}
	}
	fr.t.run(rPr, &p)
	r := Run{Bold: p.bold, Italic: p.italic, Underline: p.underline, Strike: p.strike, Caps: p.caps, Size: p.size, Colour: p.colour, Fill: p.fill, Font: p.font}
	return r
}

// paragraph converts a paragraph; a page break in it, or a section's end,
// starts the next sheet.
func (fr *framer) paragraph(p *node) {
	pPr := p.child("w:pPr")
	class, align := fr.paraStyle(pPr)
	if on, ok := pPr.on("w:pageBreakBefore"); ok && on {
		if last := fr.sheets[len(fr.sheets)-1]; len(last) > 0 {
			fr.newSheet()
		}
	}
	para := &Para{Class: class, Align: align, Runs: []Run{}}
	var boxes [][]Node
	var marker bool
	st := &fieldState{}
	var walk func(kids []*node)
	walk = func(kids []*node) {
		for _, k := range kids {
			switch k.name {
			case "w:r":
				fr.runContent(k, para, &boxes, st)
			case "w:hyperlink", "w:ins", "w:smartTag", "w:customXml", "w:fldSimple":
				if k.name == "w:fldSimple" {
					if f := fieldKind(k.attr["w:instr"]); f != "" {
						para.Runs = append(para.Runs, Run{Field: f})
						continue
					}
				}
				walk(k.kids)
			case "w:sdt":
				tag, ok := controlTag(k)
				content := k.child("w:sdtContent")
				switch {
				case ok && tag == BodyControl && fr.part == "word/document.xml" && !fr.body:
					marker = true
				case ok && fr.values != nil && content != nil:
					if v, has := fr.values[tag]; has {
						para.Runs = append(para.Runs, textRuns(v, fr.runProps(firstRunPr(content)))...)
						continue
					}
					walk(content.kids)
				case content != nil:
					walk(content.kids)
				}
			}
		}
	}
	walk(p.kids)
	if marker {
		fr.body = true
		fr.add(Node{Body: true})
		return
	}
	// A page break inside the paragraph splits it between two sheets; the
	// text boxes follow the part of the paragraph they are anchored in,
	// the first.
	emit := func(runs []Run) {
		fr.add(Node{P: &Para{Class: class, Align: align, Runs: nonEmpty(runs)}})
		for _, b := range boxes {
			for _, n := range b {
				fr.add(n)
			}
		}
		boxes = nil
	}
	start := 0
	for i, r := range para.Runs {
		if r.Field == "pagebreak" {
			emit(para.Runs[start:i])
			fr.newSheet()
			start = i + 1
		}
	}
	if rest := para.Runs[start:]; start == 0 || len(rest) > 0 {
		emit(rest)
	}
	if pPr.child("w:sectPr") != nil {
		fr.newSheet()
	}
}

func nonEmpty(r []Run) []Run {
	if r == nil {
		return []Run{}
	}
	return r
}

// fieldState follows a complex field across the runs of a paragraph.
type fieldState struct {
	depth   int
	instr   strings.Builder
	result  bool
	written bool
}

// fieldKind names the fields the browser numbers itself.
func fieldKind(instr string) string {
	switch strings.ToUpper(strings.Fields(instr + " x")[0]) {
	case "PAGE":
		return "page"
	case "NUMPAGES", "SECTIONPAGES":
		return "pages"
	}
	return ""
}

// runContent converts the content of a run.
func (fr *framer) runContent(r *node, para *Para, boxes *[][]Node, st *fieldState) {
	base := fr.runProps(r.child("w:rPr"))
	for _, k := range r.kids {
		switch k.name {
		case "w:fldChar":
			switch k.attr["w:fldCharType"] {
			case "begin":
				st.depth++
				if st.depth == 1 {
					st.instr.Reset()
					st.result, st.written = false, false
				}
			case "separate":
				if st.depth == 1 {
					st.result = true
				}
			case "end":
				st.depth = max(st.depth-1, 0)
				if st.depth == 0 && !st.written {
					if f := fieldKind(st.instr.String()); f != "" && !st.result {
						para.Runs = append(para.Runs, Run{Field: f})
					}
				}
			}
			continue
		case "w:instrText":
			if st.depth == 1 {
				st.instr.WriteString(k.text.String())
			}
			continue
		}
		if st.depth > 0 && !st.result {
			continue // the field's instruction, not its text
		}
		if st.depth > 0 {
			if f := fieldKind(st.instr.String()); f != "" {
				if !st.written {
					para.Runs = append(para.Runs, Run{Field: f})
					st.written = true
				}
				continue
			}
		}
		switch k.name {
		case "w:t":
			x := base
			x.Text = k.text.String()
			para.Runs = append(para.Runs, x)
		case "w:tab", "w:ptab":
			para.Runs = append(para.Runs, Run{Tab: true})
		case "w:br", "w:cr":
			if k.attr["w:type"] == "page" {
				para.Runs = append(para.Runs, Run{Field: "pagebreak"})
			} else {
				para.Runs = append(para.Runs, Run{Break: true})
			}
		case "w:noBreakHyphen":
			x := base
			x.Text = "-"
			para.Runs = append(para.Runs, x)
		case "w:drawing", "w:pict", "w:object":
			fr.drawing(k, para, boxes)
		case "mc:AlternateContent":
			fr.alternate(k, para, boxes)
		}
	}
}

// alternate picks the content of markup compatibility a reader of this
// package understands: the shapes of Word 2010 when offered, the older
// form otherwise.
func (fr *framer) alternate(ac *node, para *Para, boxes *[][]Node) {
	for _, k := range ac.kids {
		if k.name == "mc:Choice" {
			req := k.attr["Requires"]
			if strings.Contains(req, "wps") || strings.Contains(req, "wpg") || req == "" {
				fr.drawing(k, para, boxes)
				return
			}
		}
	}
	if fb := ac.child("mc:Fallback"); fb != nil {
		fr.drawing(fb, para, boxes)
	}
}

var cssLength = regexp.MustCompile(`(?i)(width|height)\s*:\s*([0-9.]+)\s*(pt|in|cm|mm|px)?`)

// drawing takes the pictures and the text boxes of a drawing: the
// pictures into the paragraph, the boxes' text after it. A picture
// behind the text, a page's background, is left out.
func (fr *framer) drawing(d *node, para *Para, boxes *[][]Node) {
	d.walk(func(n *node) bool {
		switch n.name {
		case "wp:anchor":
			if on := n.attr["behindDoc"]; on == "1" || on == "true" {
				return false
			}
		case "w:txbxContent":
			sub := &framer{t: fr.t, part: fr.part, values: fr.values, media: fr.media, sheets: [][]Node{{}}, maxNodes: fr.maxNodes - fr.count, body: true}
			sub.blocks(n.kids)
			var nodes []Node
			for _, s := range sub.sheets {
				nodes = append(nodes, s...)
			}
			fr.count += sub.count
			*boxes = append(*boxes, nodes)
			return false
		case "pic:pic":
			blip := n.path("pic:blipFill", "a:blip")
			if img := fr.image(blip.attrOr("r:embed"), extentOf(n)); img != nil {
				img.Align = alignOf(n)
				para.Runs = append(para.Runs, Run{Image: img})
			}
			return false
		case "v:imagedata":
			w, h := 0.0, 0.0
			for p := n.parent; p != nil && (w == 0 || h == 0); p = p.parent {
				for _, m := range cssLength.FindAllStringSubmatch(p.attr["style"], -1) {
					v := length(m[2], m[3])
					if strings.EqualFold(m[1], "width") {
						w = v
					} else {
						h = v
					}
				}
			}
			if img := fr.image(n.attr["r:id"], [2]float64{w, h}); img != nil {
				para.Runs = append(para.Runs, Run{Image: img})
			}
			return false
		}
		return true
	})
}

// length converts a CSS length of VML to points.
func length(v, unit string) float64 {
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0
	}
	switch strings.ToLower(unit) {
	case "in":
		return f * 72
	case "cm":
		return f * 72 / 2.54
	case "mm":
		return f * 72 / 25.4
	case "px":
		return f * 0.75
	}
	return f
}

// extentOf is a drawing's size in points, from the extent of the anchor
// or inline element around the picture.
func extentOf(pic *node) [2]float64 {
	for p := pic.parent; p != nil; p = p.parent {
		if p.name == "wp:inline" || p.name == "wp:anchor" {
			if e := p.child("wp:extent"); e != nil {
				cx, _ := strconv.ParseFloat(e.attr["cx"], 64)
				cy, _ := strconv.ParseFloat(e.attr["cy"], 64)
				return [2]float64{cx / 12700, cy / 12700}
			}
		}
	}
	return [2]float64{}
}

// alignOf is how a floating picture is aligned on its line.
func alignOf(pic *node) string {
	for p := pic.parent; p != nil; p = p.parent {
		if p.name == "wp:anchor" {
			if a := p.path("wp:positionH", "wp:align"); a != nil {
				switch v := strings.TrimSpace(a.text.String()); v {
				case "left", "right", "center":
					return v
				case "inside":
					return "left"
				case "outside":
					return "right"
				}
			}
		}
	}
	return ""
}

// image is a picture of the part by its relationship, if it is one the
// browser shows.
func (fr *framer) image(id string, size [2]float64) *Image {
	r, ok := fr.t.relByID(fr.part, id)
	if !ok || r.external || path.Dir(r.target) != "word/media" {
		return nil
	}
	name := path.Base(r.target)
	if _, _, ok := fr.t.Media(name); !ok || fr.media == nil {
		return nil
	}
	w, h := size[0], size[1]
	if w <= 0 || h <= 0 || w > 2000 || h > 2000 {
		w, h = 0, 0
	}
	return &Image{Src: fr.media(name), Width: round2(w), Height: round2(h)}
}

func round2(f float64) float64 { return float64(int(f*100+0.5)) / 100 }

// table converts a table.
func (fr *framer) table(tbl *node) *Table {
	tb := &Table{Rows: [][]Cell{}}
	tblPr := tbl.child("w:tblPr")
	tb.Borders = hasBorders(tblPr.child("w:tblBorders"))
	if v, ok := tblPr.val("w:tblStyle"); ok && !tb.Borders {
		for _, st := range fr.t.styles.chain(v) {
			if b := st.tblPr.child("w:tblBorders"); b != nil {
				tb.Borders = hasBorders(b)
			}
		}
	}
	for _, tr := range tbl.kids {
		if tr.name != "w:tr" {
			continue
		}
		var row []Cell
		for _, tc := range tr.kids {
			if tc.name == "w:sdt" {
				tc = tc.path("w:sdtContent", "w:tc")
				if tc == nil {
					continue
				}
			}
			if tc.name != "w:tc" {
				continue
			}
			sub := &framer{t: fr.t, part: fr.part, values: fr.values, media: fr.media, sheets: [][]Node{{}}, maxNodes: fr.maxNodes - fr.count, body: fr.body}
			sub.blocks(tc.kids)
			fr.count += sub.count
			if sub.body && !fr.body {
				fr.body = true
			}
			var nodes []Node
			for _, s := range sub.sheets {
				nodes = append(nodes, s...)
			}
			if nodes == nil {
				nodes = []Node{}
			}
			c := Cell{Nodes: nodes}
			tcPr := tc.child("w:tcPr")
			c.Fill = fr.t.theme.colour(tcPr.child("w:shd"), "w:fill")
			if w := tcPr.child("w:tcW"); w != nil && w.attr["w:type"] == "dxa" {
				c.Width = round2(twips(w, "w:w", 0))
			}
			if v, ok := tcPr.val("w:gridSpan"); ok {
				if n, err := strconv.Atoi(v); err == nil && n > 1 && n < 64 {
					c.Span = n
				}
			}
			row = append(row, c)
		}
		if row != nil {
			tb.Rows = append(tb.Rows, row)
		}
	}
	return tb
}

// hasBorders reports whether a table's borders draw a line.
func hasBorders(b *node) bool {
	if b == nil {
		return false
	}
	for _, k := range b.kids {
		switch k.attr["w:val"] {
		case "", "nil", "none":
		default:
			return true
		}
	}
	return false
}
