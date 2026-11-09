// Package wordtemplate reads Word templates (.dotx) and writes documents
// from them (ADR-0167): a page of a type whose template it is exports as
// a Word document with the template's styles, page, header, footer and
// cover, its text where the template marks it and its fields in the
// template's content controls; and the reading view shows the page framed
// as that document.
//
// A template is untrusted input. Parse refuses what can run or reach out
// when the document is opened — macros, ActiveX controls, embedded
// objects, references to files or addresses outside the package other
// than hyperlinks — and removes the one such reference Word itself writes
// into templates, the attached template. Everything else of the package
// is kept byte for byte: a change replaces the span of an element in its
// part, so the template's own markup never passes through an XML encoder.
package wordtemplate

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// ErrInvalid marks a file that is not a template this package accepts;
// the error says why.
var ErrInvalid = errors.New("invalid template")

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

// maxParts bounds the files of a template's package. It is part of what a
// template is, like the part names it may use: Word's templates hold a few
// dozen; thousands are no template, and each is read.
const maxParts = 1000

// ControlPrefix starts the tag of every content control this package
// fills: "orbithek:title", "orbithek:author", "orbithek:body".
const ControlPrefix = "orbithek:"

// BodyControl is the tag, after ControlPrefix, of the control that marks
// where a page's text goes.
const BodyControl = "body"

// The content types of a package's main part that are accepted: a
// template, or a document used as one.
const (
	templateMain = "application/vnd.openxmlformats-officedocument.wordprocessingml.template.main+xml"
	documentMain = "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"
)

// Relationship types this package reads or writes, by their last segment.
const (
	relBase = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/"
	relDoc  = relBase + "officeDocument"
)

// Template is a checked and cleaned template.
type Template struct {
	parts map[string][]byte
	names []string // in the order of the package
	// doc is word/document.xml; rels are the relationships by the part
	// they belong to.
	doc    *node
	rels   map[string][]rel
	styles *styleSheet
	theme  theme
	num    numbering
	info   Info
}

// rel is one relationship of a part.
type rel struct {
	id, typ, target string
	external        bool
}

// Info is what the administration shows of a template.
type Info struct {
	// Styles are the names of the template's paragraph styles.
	Styles []string `json:"styles"`
	// Controls are the tags of its content controls that start with
	// ControlPrefix, without it: the fields it shows.
	Controls []string `json:"controls"`
	// Body says whether a control marks where the text goes; without one
	// the text follows the template's content.
	Body bool `json:"body"`
	// Header and Footer say whether the template has them.
	Header bool `json:"header"`
	Footer bool `json:"footer"`
	// Removed names what cleaning took out of the package.
	Removed []string `json:"removed"`
	Page    Page     `json:"page"`
}

// Page is the size and the margins of the template's page, in points.
type Page struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Top    float64 `json:"top"`
	Right  float64 `json:"right"`
	Bottom float64 `json:"bottom"`
	Left   float64 `json:"left"`
	// Header and Footer are the distances of the header from the top and
	// of the footer from the bottom of the page.
	Header float64 `json:"header"`
	Footer float64 `json:"footer"`
}

// Info returns what the template holds.
func (t *Template) Info() Info { return t.info }

var partName = regexp.MustCompile(`^[A-Za-z0-9._\-\[\]/]+$`)

// Parse checks a template and cleans it. unpacked bounds the bytes its
// parts may hold once unpacked, so a small file cannot make the server
// hold a large one.
func Parse(data []byte, unpacked int64) (*Template, error) {
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, invalid("not a Word file (%v)", err)
	}
	if len(z.File) > maxParts {
		return nil, invalid("the package has more than %d parts", maxParts)
	}
	t := &Template{parts: map[string][]byte{}, rels: map[string][]rel{}}
	seen := map[string]bool{}
	left := unpacked
	for _, f := range z.File {
		name := f.Name
		if strings.HasSuffix(name, "/") {
			continue // a folder entry carries nothing
		}
		if !partName.MatchString(name) || strings.HasPrefix(name, "/") || slices.Contains(strings.Split(name, "/"), "..") || slices.Contains(strings.Split(name, "/"), ".") || strings.Contains(name, "//") {
			return nil, invalid("the part name %q is not accepted", name)
		}
		if seen[strings.ToLower(name)] {
			return nil, invalid("the part %q is there twice", name)
		}
		seen[strings.ToLower(name)] = true
		rc, err := f.Open()
		if err != nil {
			return nil, invalid("the part %q cannot be read (%v)", name, err)
		}
		b, err := io.ReadAll(io.LimitReader(rc, left+1))
		rc.Close()
		if err != nil {
			return nil, invalid("the part %q cannot be read (%v)", name, err)
		}
		left -= int64(len(b))
		if left < 0 {
			return nil, invalid("the package unpacks to more than %d bytes", unpacked)
		}
		t.parts[name] = b
		t.names = append(t.names, name)
	}
	if err := t.check(); err != nil {
		return nil, err
	}
	return t, nil
}

// check refuses what may not be in a template, cleans what is removed,
// and reads what the export and the frame need.
func (t *Template) check() error {
	for _, name := range t.names {
		low := strings.ToLower(name)
		switch {
		case strings.HasSuffix(low, "vbaproject.bin") || strings.HasSuffix(low, "vbadata.xml"):
			return invalid("templates with macros are not accepted")
		case strings.HasPrefix(low, "word/activex/"):
			return invalid("templates with ActiveX controls are not accepted")
		}
	}
	for _, required := range []string{"[Content_Types].xml", "_rels/.rels", "word/document.xml"} {
		if t.parts[required] == nil {
			return invalid("the part %s is missing", required)
		}
	}
	// Every XML part must be well-formed: the export and the frame read
	// them, and Word refuses a document with a broken one.
	trees := map[string]*node{}
	for _, name := range t.names {
		if !strings.HasSuffix(name, ".xml") && !strings.HasSuffix(name, ".rels") {
			continue
		}
		n, err := parse(t.parts[name])
		if err != nil {
			return invalid("the part %s is not well-formed XML (%v)", name, err)
		}
		trees[name] = n
	}
	if err := t.checkContentTypes(trees["[Content_Types].xml"]); err != nil {
		return err
	}
	if err := t.readRels(trees); err != nil {
		return err
	}
	main := ""
	for _, r := range t.rels[""] {
		if r.typ == relDoc {
			main = r.target
		}
	}
	if main != "word/document.xml" {
		return invalid("the main part must be word/document.xml")
	}
	t.doc = trees["word/document.xml"]
	if t.doc.name != "w:document" || t.doc.child("w:body") == nil {
		return invalid("word/document.xml is not a Word document")
	}
	// What this package writes into the parts uses the prefixes w, r and
	// wp; a part that binds them to something else cannot take it.
	for name, root := range trees {
		if !strings.HasPrefix(name, "word/") || strings.HasSuffix(name, ".rels") {
			continue
		}
		for _, p := range []string{"w", "r", "wp"} {
			if uri, ok := root.attr["xmlns:"+p]; ok && uri != map[string]string{"w": nsW, "r": nsR, "wp": nsWP}[p] {
				return invalid("the part %s binds the prefix %s to another namespace", name, p)
			}
		}
	}
	for _, name := range []string{"word/document.xml", "word/styles.xml", "word/numbering.xml"} {
		if root := trees[name]; root != nil && root.attr["xmlns:w"] != nsW {
			return invalid("the part %s does not bind the prefix w to the main namespace", name)
		}
	}
	if trees["word/styles.xml"] == nil {
		return invalid("the template has no styles (word/styles.xml)")
	}
	t.styles = readStyles(trees["word/styles.xml"])
	if th := t.relTarget("word/document.xml", "theme"); th != "" && trees[th] != nil {
		t.theme = readTheme(trees[th])
	}
	if nb := t.relTarget("word/document.xml", "numbering"); nb != "" && trees[nb] != nil {
		t.num = readNumbering(trees[nb])
	}
	if err := t.cleanSettings(trees); err != nil {
		return err
	}
	t.readInfo(trees)
	return nil
}

// checkContentTypes refuses a main part that is no template or document,
// and every part that holds macros.
func (t *Template) checkContentTypes(ct *node) error {
	if ct == nil || ct.name != "ct:Types" {
		return invalid("[Content_Types].xml is not a list of content types")
	}
	main := ""
	for _, k := range ct.kids {
		typ := strings.ToLower(k.attr["ContentType"])
		if strings.Contains(typ, "macroenabled") || strings.Contains(typ, "vbaproject") || strings.Contains(typ, "activex") {
			return invalid("templates with macros or controls are not accepted")
		}
		if k.name == "ct:Override" && strings.EqualFold(k.attr["PartName"], "/word/document.xml") {
			main = k.attr["ContentType"]
		}
	}
	if main != templateMain && main != documentMain {
		return invalid("the file is not a Word template or document")
	}
	return nil
}

// readRels reads every relationship part and refuses the relationships
// that run something or reach out of the package when the document is
// opened. A reference to the template a document was made from, which
// Word writes and which names a file on the author's computer or a
// network share, is removed: opening such a document makes Word fetch it.
func (t *Template) readRels(trees map[string]*node) error {
	for _, name := range t.names {
		if !strings.HasSuffix(name, ".rels") {
			continue
		}
		dir, file := path.Split(name)
		if path.Base(dir) != "_rels" {
			return invalid("the relationship part %s is not in a _rels folder", name)
		}
		source := path.Join(path.Dir(strings.TrimSuffix(dir, "/")), strings.TrimSuffix(file, ".rels"))
		if source == "." {
			source = ""
		}
		root := trees[name]
		if root.name != "rel:Relationships" {
			return invalid("%s is not a list of relationships", name)
		}
		var drop []edit
		for _, k := range root.kids {
			if k.name != "rel:Relationship" {
				continue
			}
			r := rel{id: k.attr["Id"], typ: k.attr["Type"], external: k.attr["TargetMode"] == "External"}
			kind := path.Base(r.typ)
			switch {
			case kind == "attachedTemplate":
				drop = append(drop, edit{k.start, k.end, ""})
				t.info.Removed = append(t.info.Removed, "attached template reference")
				continue
			case kind == "oleObject" || kind == "control" || kind == "aFChunk" || kind == "subDocument" || kind == "frame" || strings.Contains(strings.ToLower(r.typ), "vbaproject"):
				return invalid("the template holds an embedded object or control (%s in %s), which is not accepted", kind, name)
			case r.external && kind != "hyperlink":
				return invalid("the template refers to something outside the file (%s in %s), which is not accepted", kind, name)
			}
			if r.external {
				r.target = k.attr["Target"]
			} else {
				r.target = resolve(path.Dir(source), k.attr["Target"])
			}
			t.rels[source] = append(t.rels[source], r)
		}
		if len(drop) > 0 {
			b, err := splice(t.parts[name], drop)
			if err != nil {
				return err
			}
			t.parts[name] = b
		}
	}
	return nil
}

// resolve makes a relationship's target a part name.
func resolve(dir, target string) string {
	if strings.HasPrefix(target, "/") {
		return strings.TrimPrefix(path.Clean(target), "/")
	}
	if dir == "." || dir == "" {
		return path.Clean(target)
	}
	return path.Clean(path.Join(dir, target))
}

// relTarget is the part a part refers to by a relationship of the type
// ending in kind, the first one.
func (t *Template) relTarget(source, kind string) string {
	for _, r := range t.rels[source] {
		if path.Base(r.typ) == kind && !r.external {
			return r.target
		}
	}
	return ""
}

// relByID is the relationship of a part by its ID.
func (t *Template) relByID(source, id string) (rel, bool) {
	for _, r := range t.rels[source] {
		if r.id == id {
			return r, true
		}
	}
	return rel{}, false
}

// cleanSettings takes the attached template's element out of the
// settings, since its relationship is gone.
func (t *Template) cleanSettings(trees map[string]*node) error {
	const name = "word/settings.xml"
	root := trees[name]
	if root == nil {
		return nil
	}
	var drop []edit
	for _, k := range root.kids {
		if k.name == "w:attachedTemplate" {
			drop = append(drop, edit{k.start, k.end, ""})
		}
	}
	if len(drop) == 0 {
		return nil
	}
	b, err := splice(t.parts[name], drop)
	if err != nil {
		return err
	}
	t.parts[name] = b
	return nil
}

// readInfo gathers what the administration shows.
func (t *Template) readInfo(trees map[string]*node) {
	for _, st := range t.styles.list {
		if st.typ == "paragraph" && st.name != "" {
			t.info.Styles = append(t.info.Styles, st.name)
		}
	}
	controls := map[string]bool{}
	scan := func(root *node) {
		for _, sdt := range root.all("w:sdt") {
			sdt.walk(func(n *node) bool {
				if n.name == "w:sdt" {
					if tag, ok := controlTag(n); ok {
						if tag == BodyControl {
							t.info.Body = true
						} else {
							controls[tag] = true
						}
					}
				}
				return true
			})
		}
	}
	scan(t.doc)
	sect := t.sectPr()
	for _, ref := range sect.kids {
		if ref.name != "w:headerReference" && ref.name != "w:footerReference" {
			continue
		}
		r, ok := t.relByID("word/document.xml", ref.attr["r:id"])
		if !ok || trees[r.target] == nil {
			continue
		}
		if ref.name == "w:headerReference" {
			t.info.Header = true
		} else {
			t.info.Footer = true
		}
		scan(trees[r.target])
	}
	for tag := range controls {
		t.info.Controls = append(t.info.Controls, tag)
	}
	slices.Sort(t.info.Controls)
	t.info.Page = t.page()
	if t.info.Removed == nil {
		t.info.Removed = []string{}
	}
	if t.info.Controls == nil {
		t.info.Controls = []string{}
	}
}

// controlTag is the tag of a content control after ControlPrefix, in
// lower case, if it has one.
func controlTag(sdt *node) (string, bool) {
	tag := sdt.path("w:sdtPr", "w:tag")
	if tag == nil {
		return "", false
	}
	v := strings.ToLower(strings.TrimSpace(tag.attr["w:val"]))
	if !strings.HasPrefix(v, ControlPrefix) || len(v) == len(ControlPrefix) {
		return "", false
	}
	return strings.TrimPrefix(v, ControlPrefix), true
}

// sectPr is the last section's properties: the body's own.
func (t *Template) sectPr() *node {
	body := t.doc.child("w:body")
	if n := len(body.kids); n > 0 && body.kids[n-1].name == "w:sectPr" {
		return body.kids[n-1]
	}
	return &node{attr: map[string]string{}}
}

// twips reads an attribute in twentieths of a point.
func twips(n *node, name string, def float64) float64 {
	if n == nil {
		return def
	}
	v, err := strconv.ParseFloat(n.attr[name], 64)
	if err != nil {
		return def
	}
	return v / 20
}

// page reads the page of the last section; a template without one has
// A4 with margins of 2.5 cm.
func (t *Template) page() Page {
	s := t.sectPr()
	sz, mar := s.child("w:pgSz"), s.child("w:pgMar")
	p := Page{
		Width: twips(sz, "w:w", 595.3), Height: twips(sz, "w:h", 841.9),
		Top: twips(mar, "w:top", 70.85), Right: twips(mar, "w:right", 70.85), Bottom: twips(mar, "w:bottom", 56.7), Left: twips(mar, "w:left", 70.85),
		Header: twips(mar, "w:header", 35.4), Footer: twips(mar, "w:footer", 35.4),
	}
	if sz != nil && sz.attr["w:orient"] == "landscape" && p.Width < p.Height {
		p.Width, p.Height = p.Height, p.Width
	}
	return p
}

// TextWidth is the width of the text column in EMU (914400 to the inch):
// the page's width without its margins.
func (t *Template) TextWidth() int64 {
	p := t.info.Page
	w := int64((p.Width - p.Left - p.Right) * 12700)
	return max(w, 12700*72)
}

// HasControl reports whether the template has a content control of the
// tag (after ControlPrefix) in its text, header or footer.
func (t *Template) HasControl(tag string) bool {
	return slices.Contains(t.info.Controls, strings.ToLower(tag))
}

// Package returns the cleaned template as a file, to keep in place of
// the one uploaded.
func (t *Template) Package() ([]byte, error) {
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for _, name := range t.names {
		w, err := z.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(t.parts[name]); err != nil {
			return nil, err
		}
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

var mediaName = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)

// Media returns a picture of the template, word/media/<name>, with its
// content type, when it is a PNG, JPEG or GIF picture: the formats a
// browser shows and that run nothing.
func (t *Template) Media(name string) ([]byte, string, bool) {
	if !mediaName.MatchString(name) {
		return nil, "", false
	}
	data, ok := t.parts["word/media/"+name]
	if !ok {
		return nil, "", false
	}
	typ := pictureType(data)
	return data, typ, typ != ""
}

// pictureType names a PNG, JPEG or GIF picture by its first bytes.
func pictureType(data []byte) string {
	switch {
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		return "image/png"
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg"
	case bytes.HasPrefix(data, []byte("GIF87a")), bytes.HasPrefix(data, []byte("GIF89a")):
		return "image/gif"
	}
	return ""
}
