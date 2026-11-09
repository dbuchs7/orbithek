package wordtemplate

import (
	"archive/zip"
	"fmt"
	"io"
	"path"
	"slices"
	"strings"
)

// Content is what an export puts into a template. The body refers to
// styles by the IDs StyleID gave or to the styles it adds, to numbering
// from NumberingBase on, and to relationships by IDs that start with
// RelPrefix.
type Content struct {
	// Body is WordprocessingML: paragraphs and tables.
	Body string
	// Styles are <w:style> elements the template lacks, added as given.
	Styles []string
	// AbstractNums and Nums are <w:abstractNum> and <w:num> elements.
	AbstractNums, Nums []string
	// Links are hyperlink targets by relationship ID; Media are pictures.
	Links map[string]string
	Media []Media
	// Controls are the texts of the content controls by tag, after
	// ControlPrefix, in lower case. A control whose tag is not here keeps
	// the template's content.
	Controls map[string]string
	// Title is the document's title in its properties.
	Title string
}

// Media is a picture an export adds, under word/media.
type Media struct {
	ID, Name string
	Data     []byte
}

// RelPrefix starts every relationship ID an export may add: no ID of the
// template's document starts with it.
func (t *Template) RelPrefix() string {
	p := "orb"
	for {
		taken := false
		for _, r := range t.rels["word/document.xml"] {
			if strings.HasPrefix(r.id, p) {
				taken = true
				break
			}
		}
		if !taken {
			return p
		}
		p += "x"
	}
}

// MediaPrefix starts every picture name an export may add.
func (t *Template) MediaPrefix() string {
	p := "orbithek-"
	for {
		taken := false
		for name := range t.parts {
			if strings.HasPrefix(name, "word/media/"+p) {
				taken = true
				break
			}
		}
		if !taken {
			return p
		}
		p = "x" + p
	}
}

const (
	relStyles    = relBase + "styles"
	relNumbering = relBase + "numbering"
	relHyperlink = relBase + "hyperlink"
	relImage     = relBase + "image"
	numberingCT  = "application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"
)

// Write writes a Word document: the template's package with the content
// in it.
func (t *Template) Write(w io.Writer, c Content) error {
	out := map[string][]byte{}
	var added []string
	set := func(name string, data []byte) {
		if _, ok := t.parts[name]; !ok {
			if _, again := out[name]; !again {
				added = append(added, name)
			}
		}
		out[name] = data
	}
	doc, err := t.document(c)
	if err != nil {
		return err
	}
	set("word/document.xml", doc)
	for _, part := range t.headersFooters() {
		root, err := parse(t.parts[part])
		if err != nil {
			return err
		}
		edits := fillControls(root, c.Controls)
		b, err := splice(t.parts[part], edits)
		if err != nil {
			return err
		}
		set(part, b)
	}
	if len(c.Styles) > 0 {
		b, err := insertBefore(t.parts["word/styles.xml"], "", strings.Join(c.Styles, ""))
		if err != nil {
			return err
		}
		set("word/styles.xml", b)
	}
	newRels := []string{}
	if len(c.AbstractNums)+len(c.Nums) > 0 {
		if nb := t.relTarget("word/document.xml", "numbering"); nb != "" && t.parts[nb] != nil {
			b, err := t.mergeNumbering(t.parts[nb], c)
			if err != nil {
				return err
			}
			set(nb, b)
		} else {
			set("word/numbering.xml", []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
				`<w:numbering xmlns:w="`+nsW+`">`+strings.Join(c.AbstractNums, "")+strings.Join(c.Nums, "")+`</w:numbering>`))
			newRels = append(newRels, fmt.Sprintf(`<Relationship Id="%sN" Type="%s" Target="numbering.xml"/>`, t.RelPrefix(), relNumbering))
		}
	}
	ids := make([]string, 0, len(c.Links))
	for id := range c.Links {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		newRels = append(newRels, fmt.Sprintf(`<Relationship Id="%s" Type="%s" Target="%s" TargetMode="External"/>`, esc(id), relHyperlink, esc(c.Links[id])))
	}
	for _, m := range c.Media {
		newRels = append(newRels, fmt.Sprintf(`<Relationship Id="%s" Type="%s" Target="media/%s"/>`, esc(m.ID), relImage, esc(m.Name)))
		set("word/media/"+m.Name, m.Data)
	}
	if len(newRels) > 0 {
		const relsName = "word/_rels/document.xml.rels"
		src := t.parts[relsName]
		if src == nil {
			src = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="` + nsRel + `"></Relationships>`)
		}
		b, err := insertBefore(src, "", strings.Join(newRels, ""))
		if err != nil {
			return err
		}
		set(relsName, b)
	}
	ct, err := t.contentTypes(c, slices.Contains(added, "word/numbering.xml"))
	if err != nil {
		return err
	}
	set("[Content_Types].xml", ct)
	if core := t.parts["docProps/core.xml"]; core != nil && c.Title != "" {
		if b, err := setTitle(core, c.Title); err == nil {
			set("docProps/core.xml", b)
		}
	}
	z := zip.NewWriter(w)
	for _, name := range append(slices.Clone(t.names), added...) {
		data, ok := out[name]
		if !ok {
			data = t.parts[name]
		}
		fw, err := z.Create(name)
		if err != nil {
			return err
		}
		if _, err := fw.Write(data); err != nil {
			return err
		}
	}
	return z.Close()
}

// headersFooters are the template's header and footer parts.
func (t *Template) headersFooters() []string {
	var out []string
	for _, r := range t.rels["word/document.xml"] {
		kind := path.Base(r.typ)
		if (kind == "header" || kind == "footer") && t.parts[r.target] != nil && !slices.Contains(out, r.target) {
			out = append(out, r.target)
		}
	}
	slices.Sort(out)
	return out
}

// document writes word/document.xml: the body where the template marks
// it, or after the template's content, and the controls filled.
func (t *Template) document(c Content) ([]byte, error) {
	src := t.parts["word/document.xml"]
	root, err := parse(src)
	if err != nil {
		return nil, err
	}
	body := root.child("w:body")
	var edits []edit
	var marker *node
	body.walk(func(n *node) bool {
		if marker != nil {
			return false
		}
		if n.name == "w:sdt" {
			if tag, ok := controlTag(n); ok && tag == BodyControl {
				marker = n
				return false
			}
		}
		return true
	})
	if marker != nil {
		// A marker inside a paragraph takes the whole paragraph's place.
		target := marker
		for p := marker.parent; p != nil && p != body; p = p.parent {
			if p.name == "w:p" {
				target = p
				break
			}
		}
		edits = append(edits, edit{target.start, target.end, c.Body})
		for _, e := range fillControls(body, c.Controls) {
			if e.end <= target.start || e.start >= target.end {
				edits = append(edits, e)
			}
		}
	} else {
		at := body.innerEnd
		if n := len(body.kids); n > 0 && body.kids[n-1].name == "w:sectPr" {
			at = body.kids[n-1].start
		}
		edits = append(edits, edit{at, at, c.Body})
		edits = append(edits, fillControls(body, c.Controls)...)
	}
	// What the body brings needs the prefixes r and wp declared.
	var decl string
	for _, ns := range [][2]string{{"r", nsR}, {"wp", nsWP}} {
		if _, ok := root.attr["xmlns:"+ns[0]]; !ok {
			decl += fmt.Sprintf(` xmlns:%s="%s"`, ns[0], ns[1])
		}
	}
	if decl != "" {
		at := root.inner - 1
		if src[at] != '>' {
			return nil, invalid("word/document.xml has an unexpected root element")
		}
		edits = append(edits, edit{at, at, decl})
	}
	return splice(src, edits)
}

// fillControls writes the texts of the content controls below n whose
// tags have one, in the formatting of each control's first paragraph and
// run. A filled control no longer shows its placeholder and is no longer
// bound to the document's data, which Word would otherwise write over it.
func fillControls(n *node, values map[string]string) []edit {
	var edits []edit
	n.walk(func(x *node) bool {
		if x.name != "w:sdt" {
			return true
		}
		tag, ok := controlTag(x)
		if !ok || tag == BodyControl {
			return true
		}
		v, ok := values[tag]
		if !ok {
			return true
		}
		content := x.child("w:sdtContent")
		if content == nil {
			return false
		}
		if pr := x.child("w:sdtPr"); pr != nil {
			for _, k := range pr.kids {
				if k.name == "w:showingPlcHdr" || k.name == "w:dataBinding" {
					edits = append(edits, edit{k.start, k.end, ""})
				}
			}
		}
		var pPr, rPr *node
		block := content.child("w:p") != nil
		var first *node
		content.walk(func(y *node) bool {
			switch {
			case y.name == "w:p" && first == nil:
				first = y
				pPr = y.child("w:pPr")
			case y.name == "w:r" && rPr == nil:
				rPr = y.child("w:rPr")
				return false
			}
			return true
		})
		if content.child("w:tc") != nil || content.child("w:tr") != nil {
			return false // a control around cells or rows is left as it is
		}
		edits = append(edits, edit{content.inner, content.innerEnd, filled(v, pPr, rPr, block)})
		return false
	})
	return edits
}

// filled writes a control's text: a paragraph for a control around
// paragraphs, a run for one inside a paragraph.
func filled(v string, pPr, rPr *node, block bool) string {
	r := "<w:r>" + rPr.raw()
	for i, line := range strings.Split(v, "\n") {
		if i > 0 {
			r += "<w:br/>"
		}
		r += `<w:t xml:space="preserve">` + esc(line) + "</w:t>"
	}
	r += "</w:r>"
	if !block {
		return r
	}
	return "<w:p>" + paraPr(pPr) + r + "</w:p>"
}

// paraPr is a paragraph's properties without a section's: those belong to
// the paragraph that ends a section and must not be copied.
func paraPr(pPr *node) string {
	if pPr == nil {
		return ""
	}
	raw := pPr.raw()
	if s := pPr.child("w:sectPr"); s != nil {
		raw = raw[:s.start-pPr.start] + raw[s.end-pPr.start:]
	}
	return raw
}

// insertBefore puts text before the end of a part's root element.
func insertBefore(src []byte, _ string, text string) ([]byte, error) {
	root, err := parse(src)
	if err != nil {
		return nil, err
	}
	if root.innerEnd == root.end {
		return nil, invalid("the part's root element %s is empty", root.name)
	}
	return splice(src, []edit{{root.innerEnd, root.innerEnd, text}})
}

// mergeNumbering adds the content's lists to the template's: the abstract
// numberings after the template's, the numberings after its own, as the
// schema orders them.
func (t *Template) mergeNumbering(src []byte, c Content) ([]byte, error) {
	root, err := parse(src)
	if err != nil {
		return nil, err
	}
	absAt, numAt := root.innerEnd, root.innerEnd
	for _, k := range root.kids {
		if k.name == "w:numIdMacAtCleanup" {
			numAt = k.start
			if absAt == root.innerEnd {
				absAt = k.start
			}
		}
	}
	for _, k := range root.kids {
		if k.name == "w:num" {
			absAt = k.start
			break
		}
	}
	if root.innerEnd == root.end {
		return nil, invalid("the numbering part is empty")
	}
	edits := []edit{{absAt, absAt, strings.Join(c.AbstractNums, "")}}
	if numAt == absAt {
		edits[0].with += strings.Join(c.Nums, "")
	} else {
		edits = append(edits, edit{numAt, numAt, strings.Join(c.Nums, "")})
	}
	return splice(src, edits)
}

// contentTypes names the main part a document's, not a template's, and
// adds the types of the parts an export adds.
func (t *Template) contentTypes(c Content, numberingAdded bool) ([]byte, error) {
	src := t.parts["[Content_Types].xml"]
	root, err := parse(src)
	if err != nil {
		return nil, err
	}
	var edits []edit
	defaults := map[string]bool{}
	for _, k := range root.kids {
		switch k.name {
		case "ct:Override":
			if strings.EqualFold(k.attr["PartName"], "/word/document.xml") && k.attr["ContentType"] == templateMain {
				edits = append(edits, edit{k.start, k.end, strings.Replace(k.raw(), templateMain, documentMain, 1)})
			}
		case "ct:Default":
			defaults[strings.ToLower(k.attr["Extension"])] = true
		}
	}
	var add string
	for _, m := range c.Media {
		ext := strings.ToLower(path.Ext(m.Name))
		ext = strings.TrimPrefix(ext, ".")
		if ext == "" || defaults[ext] {
			continue
		}
		defaults[ext] = true
		typ := map[string]string{"png": "image/png", "jpeg": "image/jpeg", "jpg": "image/jpeg", "gif": "image/gif"}[ext]
		if typ == "" {
			continue
		}
		add += fmt.Sprintf(`<Default Extension="%s" ContentType="%s"/>`, ext, typ)
	}
	if numberingAdded {
		add += `<Override PartName="/word/numbering.xml" ContentType="` + numberingCT + `"/>`
	}
	if add != "" {
		edits = append(edits, edit{root.innerEnd, root.innerEnd, add})
	}
	return splice(src, edits)
}

// setTitle writes the document's title into its core properties.
func setTitle(src []byte, title string) ([]byte, error) {
	root, err := parse(src)
	if err != nil {
		return nil, err
	}
	if el := root.child("dc:title"); el != nil {
		if el.inner == el.end {
			return splice(src, []edit{{el.start, el.end, "<dc:title>" + esc(title) + "</dc:title>"}})
		}
		return splice(src, []edit{{el.inner, el.innerEnd, esc(title)}})
	}
	if root.attr["xmlns:dc"] != nsDC || root.innerEnd == root.end {
		return src, nil
	}
	return splice(src, []edit{{root.innerEnd, root.innerEnd, "<dc:title>" + esc(title) + "</dc:title>"}})
}
