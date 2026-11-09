package export

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/dbuchs7/orbithek/internal/item"
	"github.com/dbuchs7/orbithek/internal/wordtemplate"
)

// styleDef is one of the styles the export writes, as defined in styles:
// its type and name, by which a template's own is found, and its markup,
// added to a template that has none of the name.
type styleDef struct{ typ, name, xml string }

var styleDefs = func() map[string]styleDef {
	out := map[string]styleDef{}
	re := regexp.MustCompile(`<w:style w:type="(\w+)"(?: w:default="1")? w:styleId="(\w+)"><w:name w:val="([^"]+)"/>.*?</w:style>`)
	for _, m := range re.FindAllStringSubmatch(styles, -1) {
		out[m[2]] = styleDef{typ: m[1], name: m[3], xml: m[0]}
	}
	return out
}()

// docxFromTemplate writes the document into its template (ADR-0167):
// the template's package with the text where the template marks it, its
// content controls filled, and the styles, lists, links and pictures the
// text needs added without touching the template's own.
func docxFromTemplate(w io.Writer, d Document) error {
	t := d.Template
	abs, num := t.NumberingBase()
	b := &docxBuilder{d: d, tpl: t, rel: t.RelPrefix(), mediaPrefix: t.MediaPrefix(), numBase: num, absBase: abs, width: t.TextWidth(), styleN: map[string]string{}}
	body := b.body()
	c := wordtemplate.Content{Body: body, Styles: b.added, Controls: d.Controls, Links: map[string]string{}, Title: b.title()}
	if b.lists {
		c.AbstractNums, c.Nums = b.numberingDefs()
	}
	for i, l := range b.links {
		c.Links[fmt.Sprintf("%sL%d", b.rel, i+1)] = l
	}
	for _, m := range b.media {
		c.Media = append(c.Media, wordtemplate.Media{ID: m.id, Name: m.file, Data: m.data})
	}
	return t.Write(w, c)
}

// tocField writes a table of contents as Word's own field, which Word
// fills with page numbers: it is marked as changed, so Word offers to
// bring it up to date when the document opens. Until then it shows the
// page's headings.
func (b *docxBuilder) tocField(body []item.Block) string {
	begin := `<w:r><w:fldChar w:fldCharType="begin" w:dirty="true"/></w:r>` +
		`<w:r><w:instrText xml:space="preserve"> TOC \o "1-3" \h \z \u </w:instrText></w:r>` +
		`<w:r><w:fldChar w:fldCharType="separate"/></w:r>`
	end := `<w:r><w:fldChar w:fldCharType="end"/></w:r>`
	var lines []string
	for _, h := range body {
		if h.Kind == item.Heading && strings.TrimSpace(h.Text) != "" {
			lines = append(lines, paraPr(fmt.Sprintf(`<w:ind w:left="%d"/>`, (max(h.Level, 1)-1)*360), run(h.Text, "")))
		}
	}
	if len(lines) == 0 {
		return paraPr("", begin+end)
	}
	lines[0] = strings.Replace(lines[0], "<w:r>", begin+"<w:r>", 1)
	last := len(lines) - 1
	lines[last] = strings.TrimSuffix(lines[last], "</w:p>") + end + "</w:p>"
	return strings.Join(lines, "")
}
