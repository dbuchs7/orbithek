package linkpreview

import (
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/dbuchs7/orbithek/internal/budget"
	"github.com/dbuchs7/orbithek/internal/item"
)

// skipped are the elements whose content is no part of a page's text:
// code, navigation, forms, embedded media and what surrounds the text.
var skipped = map[atom.Atom]bool{
	atom.Script: true, atom.Style: true, atom.Noscript: true, atom.Template: true, atom.Nav: true, atom.Aside: true,
	atom.Footer: true, atom.Header: true, atom.Form: true, atom.Button: true, atom.Select: true, atom.Textarea: true,
	atom.Input: true, atom.Svg: true, atom.Math: true, atom.Iframe: true, atom.Object: true, atom.Embed: true,
	atom.Video: true, atom.Audio: true, atom.Canvas: true, atom.Figure: true, atom.Dialog: true, atom.Menu: true,
}

// textBlocks turns a page's elements into blocks of a body: headings,
// paragraphs, list items, quotes, code, tables and dividers. Text, links,
// bold, italic and code are kept; everything else is left behind.
type textBlocks struct {
	base   *url.URL
	limit  int
	blocks []item.Block
	cut    bool
	// The block being written, its kind and its marks.
	kind  item.BlockKind
	level int
	text  strings.Builder
	runes int
	space bool // whether the text written ends with a space
	marks []item.Mark
	open  []item.Mark // marks started and not yet ended
	quote int         // depth of quotes around the text
}

// mainText reads the main text of a page: its first article, else its
// main element, else its body.
func mainText(root *html.Node, base *url.URL) ([]item.Block, bool) {
	start := find(root, func(n *html.Node) bool { return n.DataAtom == atom.Article })
	if start == nil {
		start = find(root, func(n *html.Node) bool { return n.DataAtom == atom.Main || strings.EqualFold(attr(n, "role"), "main") })
	}
	if start == nil {
		start = find(root, func(n *html.Node) bool { return n.DataAtom == atom.Body })
	}
	if start == nil {
		return nil, false
	}
	t := &textBlocks{base: base, limit: budget.Get().LinkPreviewBlocks, kind: item.Paragraph}
	t.children(start)
	t.flush()
	return t.blocks, t.cut
}

// find returns the first element below n that match accepts, depth first.
func find(n *html.Node, match func(*html.Node) bool) *html.Node {
	if n.Type == html.ElementNode && match(n) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if f := find(c, match); f != nil {
			return f
		}
	}
	return nil
}

func (t *textBlocks) children(n *html.Node) {
	for c := n.FirstChild; c != nil && !t.full(); c = c.NextSibling {
		t.node(c)
	}
}

func (t *textBlocks) full() bool {
	if len(t.blocks) >= t.limit {
		t.cut = true
		return true
	}
	return false
}

func (t *textBlocks) node(n *html.Node) {
	switch n.Type {
	case html.TextNode:
		t.write(n.Data)
		return
	case html.ElementNode:
	default:
		return
	}
	if skipped[n.DataAtom] || n.Data == "svg" || strings.EqualFold(attr(n, "hidden"), "hidden") || attr(n, "aria-hidden") == "true" {
		return
	}
	switch n.DataAtom {
	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		level := map[atom.Atom]int{atom.H1: 1, atom.H2: 2}[n.DataAtom]
		if level == 0 {
			level = 3
		}
		t.block(n, item.Heading, level)
	case atom.P:
		t.block(n, t.paragraphKind(), 0)
	case atom.Li:
		kind := item.BulletItem
		if n.Parent != nil && n.Parent.DataAtom == atom.Ol {
			kind = item.OrderedItem
		}
		t.block(n, kind, 0)
	case atom.Blockquote:
		t.flush()
		t.quote++
		t.children(n)
		t.flush()
		t.quote--
	case atom.Pre:
		t.flush()
		t.add(item.Block{Kind: item.CodeBlock, Text: strings.Trim(textOf(n), "\n")})
	case atom.Table:
		t.flush()
		t.table(n)
	case atom.Hr:
		t.flush()
		t.add(item.Block{Kind: item.Divider})
	case atom.Br:
		t.write("\n")
	case atom.Img:
		// A picture's text stands in for it only in a line of text.
		if t.runes > 0 {
			t.write(attr(n, "alt"))
		}
	case atom.A:
		href := absolute(t.base, attr(n, "href"))
		t.inline(n, item.Mark{Type: item.MarkLink, Href: href}, href != "")
	case atom.Strong, atom.B:
		t.inline(n, item.Mark{Type: item.MarkBold}, true)
	case atom.Em, atom.I:
		t.inline(n, item.Mark{Type: item.MarkItalic}, true)
	case atom.Code, atom.Kbd, atom.Samp:
		t.inline(n, item.Mark{Type: item.MarkCode}, true)
	case atom.Div, atom.Section, atom.Article, atom.Main, atom.Ul, atom.Ol, atom.Dl, atom.Dt, atom.Dd, atom.Details, atom.Summary,
		atom.Address, atom.Figcaption, atom.Caption:
		// Containers end the text before them and after them.
		t.flush()
		t.children(n)
		t.flush()
	default:
		t.children(n)
	}
}

// paragraphKind is a paragraph, or a quote inside a quotation.
func (t *textBlocks) paragraphKind() item.BlockKind {
	if t.quote > 0 {
		return item.Quote
	}
	return item.Paragraph
}

// block writes the text of n as one block of kind.
func (t *textBlocks) block(n *html.Node, kind item.BlockKind, level int) {
	t.flush()
	if t.quote > 0 && kind == item.Paragraph {
		kind = item.Quote
	}
	t.kind, t.level = kind, level
	t.children(n)
	t.flush()
}

// inline writes the text of n under a mark, when keep is true.
func (t *textBlocks) inline(n *html.Node, m item.Mark, keep bool) {
	if !keep {
		t.children(n)
		return
	}
	m.Start = t.runes
	t.open = append(t.open, m)
	t.children(n)
	last := t.open[len(t.open)-1]
	t.open = t.open[:len(t.open)-1]
	if last.Start < t.runes {
		last.End = t.runes
		t.marks = append(t.marks, last)
	}
}

// write adds text to the block being written, white space collapsed to
// single spaces and control characters dropped.
func (t *textBlocks) write(s string) {
	var b strings.Builder
	space := t.runes == 0 || t.space
	n := 0
	for _, r := range s {
		switch {
		case unicode.IsSpace(r):
			if !space {
				b.WriteByte(' ')
				space = true
				n++
			}
		case unicode.IsControl(r):
		default:
			b.WriteRune(r)
			space = false
			n++
		}
	}
	t.text.WriteString(b.String())
	t.runes += n
	t.space = space
}

// flush ends the block being written; an empty one is dropped.
func (t *textBlocks) flush() {
	text := t.text.String()
	trimmed := strings.TrimRight(text, " ")
	end := utf8.RuneCountInString(trimmed)
	if strings.TrimSpace(trimmed) != "" {
		b := item.Block{Kind: t.kind, Text: trimmed, Level: t.level}
		for _, m := range t.marks {
			m.End = min(m.End, end)
			if m.Start < m.End {
				b.Marks = append(b.Marks, m)
			}
		}
		if t.kind == item.Paragraph && t.quote > 0 {
			b.Kind = item.Quote
		}
		t.add(b)
	}
	t.text.Reset()
	t.runes, t.space = 0, false
	t.marks = nil
	for i := range t.open {
		t.open[i].Start = 0
	}
	t.kind, t.level = t.paragraphKind(), 0
}

func (t *textBlocks) add(b item.Block) {
	if b.Kind != item.Divider && strings.TrimSpace(b.Text) == "" || t.full() {
		return
	}
	if b.Level == 0 && b.Kind == item.Heading {
		b.Level = 1
	}
	t.blocks = append(t.blocks, b)
}

// table writes a table's rows, separated by new lines, and its cells, by
// tabs; the first row is the header, as a table block holds them
// (ADR-0042). A table whose rows differ in their numbers of cells is
// written as paragraphs, one per row.
func (t *textBlocks) table(n *html.Node) {
	var rows [][]string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == atom.Tr {
			var cells []string
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && (c.DataAtom == atom.Td || c.DataAtom == atom.Th) {
					cells = append(cells, clean(textOf(c)))
				}
			}
			if len(cells) > 0 {
				rows = append(rows, cells)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	if len(rows) == 0 {
		return
	}
	even := true
	for _, r := range rows {
		even = even && len(r) == len(rows[0])
	}
	if !even || len(rows) < 2 {
		for _, r := range rows {
			t.add(item.Block{Kind: item.Paragraph, Text: strings.Join(r, " · ")})
		}
		return
	}
	lines := make([]string, len(rows))
	for i, r := range rows {
		lines[i] = strings.Join(r, "\t")
	}
	t.add(item.Block{Kind: item.Table, Text: strings.Join(lines, "\n")})
}
