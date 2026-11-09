package wordtemplate

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// props is the formatting of a paragraph or a run as far as the frame and
// its stylesheet show it. Pointers tell "not said" from "off".
type props struct {
	font                                         string
	size                                         float64 // points
	bold, italic, underline, strike, caps, small *bool
	colour, fill                                 string
	align                                        string
	before, after, left, right, first            *float64 // points
	line                                         string   // a CSS line-height
	border                                       map[string]string
}

func ptr[T any](v T) *T { return &v }

// highlights are the colours of w:highlight.
var highlights = map[string]string{
	"yellow": "#FFFF00", "green": "#00FF00", "cyan": "#00FFFF", "magenta": "#FF00FF", "blue": "#0000FF", "red": "#FF0000",
	"darkBlue": "#000080", "darkCyan": "#008080", "darkGreen": "#008000", "darkMagenta": "#800080", "darkRed": "#800000",
	"darkYellow": "#808000", "darkGray": "#808080", "lightGray": "#C0C0C0", "black": "#000000", "white": "#FFFFFF",
}

// fontName admits the names of fonts: letters, digits, spaces, dots,
// dashes and underscores.
var fontName = regexp.MustCompile(`^[\p{L}\p{N} ._-]{1,64}$`)

// run merges a run's properties (w:rPr) into p.
func (t *Template) run(rPr *node, p *props) {
	if rPr == nil {
		return
	}
	if f := rPr.child("w:rFonts"); f != nil {
		name := f.attr["w:ascii"]
		if name == "" {
			name = f.attr["w:hAnsi"]
		}
		th := f.attr["w:asciiTheme"]
		if th == "" {
			th = f.attr["w:hAnsiTheme"]
		}
		switch {
		case strings.HasPrefix(th, "major"):
			name = t.theme.major
		case strings.HasPrefix(th, "minor"):
			name = t.theme.minor
		}
		if fontName.MatchString(name) {
			p.font = name
		}
	}
	if v, ok := rPr.val("w:sz"); ok {
		if n, err := strconv.ParseFloat(v, 64); err == nil && n > 0 && n < 4000 {
			p.size = n / 2
		}
	}
	for _, tg := range []struct {
		name string
		to   **bool
	}{{"w:b", &p.bold}, {"w:i", &p.italic}, {"w:strike", &p.strike}, {"w:dstrike", &p.strike}, {"w:caps", &p.caps}, {"w:smallCaps", &p.small}} {
		if on, ok := rPr.on(tg.name); ok {
			*tg.to = ptr(on)
		}
	}
	if v, ok := rPr.val("w:u"); ok {
		p.underline = ptr(v != "none" && v != "0" && v != "false")
	}
	if c := t.theme.colour(rPr.child("w:color"), "w:val"); c != "" {
		p.colour = c
	}
	if v, ok := rPr.val("w:highlight"); ok && highlights[v] != "" {
		p.fill = highlights[v]
	} else if c := t.theme.colour(rPr.child("w:shd"), "w:fill"); c != "" {
		p.fill = c
	}
}

// para merges a paragraph's properties (w:pPr) into p.
func (t *Template) para(pPr *node, p *props) {
	if pPr == nil {
		return
	}
	if v, ok := pPr.val("w:jc"); ok {
		switch v {
		case "left", "start":
			p.align = "left"
		case "center":
			p.align = "center"
		case "right", "end":
			p.align = "right"
		case "both", "distribute", "justify", "lowKashida", "mediumKashida", "highKashida", "thaiDistribute":
			p.align = "justify"
		}
	}
	if sp := pPr.child("w:spacing"); sp != nil {
		if _, ok := sp.attr["w:before"]; ok {
			p.before = ptr(twips(sp, "w:before", 0))
		}
		if _, ok := sp.attr["w:after"]; ok {
			p.after = ptr(twips(sp, "w:after", 0))
		}
		if line, err := strconv.ParseFloat(sp.attr["w:line"], 64); err == nil && line > 0 {
			switch sp.attr["w:lineRule"] {
			case "exact", "atLeast":
				p.line = num(line/20) + "pt"
			default:
				p.line = num(line / 240)
			}
		}
	}
	if ind := pPr.child("w:ind"); ind != nil {
		for _, a := range []struct {
			names []string
			to    **float64
		}{{[]string{"w:left", "w:start"}, &p.left}, {[]string{"w:right", "w:end"}, &p.right}, {[]string{"w:firstLine"}, &p.first}} {
			for _, n := range a.names {
				if _, ok := ind.attr[n]; ok {
					*a.to = ptr(twips(ind, n, 0))
				}
			}
		}
		if _, ok := ind.attr["w:hanging"]; ok {
			p.first = ptr(-twips(ind, "w:hanging", 0))
		}
	}
	if b := pPr.child("w:pBdr"); b != nil {
		for _, side := range []string{"top", "bottom", "left", "right"} {
			e := b.child("w:" + side)
			if e == nil {
				continue
			}
			if p.border == nil {
				p.border = map[string]string{}
			}
			switch e.attr["w:val"] {
			case "nil", "none", "":
				p.border[side] = "none"
				continue
			}
			w := 0.5
			if sz, err := strconv.ParseFloat(e.attr["w:sz"], 64); err == nil && sz > 0 {
				w = max(sz/8, 0.25)
			}
			c := t.theme.colour(e, "w:color")
			if c == "" {
				c = "#000000"
			}
			space, _ := strconv.ParseFloat(e.attr["w:space"], 64) // in points
			p.border[side] = fmt.Sprintf("%spt solid %s;padding-%s:%spt", num(w), c, side, num(min(max(space, 0), 31)))
		}
	}
	if c := t.theme.colour(pPr.child("w:shd"), "w:fill"); c != "" {
		p.fill = c
	}
}

// resolved is a paragraph style's formatting: the document's defaults,
// then the styles it is based on, then its own.
func (t *Template) resolved(id string) props {
	var p props
	t.para(t.styles.defPPr, &p)
	t.run(t.styles.defRPr, &p)
	for _, st := range t.styles.chain(id) {
		t.para(st.pPr, &p)
		t.run(st.rPr, &p)
	}
	return p
}

// num writes a number for CSS with at most two decimals.
func num(f float64) string {
	return strconv.FormatFloat(float64(int(f*100+0.5*sign(f)))/100, 'f', -1, 64)
}

func sign(f float64) float64 {
	if f < 0 {
		return -1
	}
	return 1
}

// css writes the declarations of p; paragraph says whether the paragraph
// properties count.
func (p props) css(paragraph bool) string {
	var out []string
	add := func(format string, args ...any) { out = append(out, fmt.Sprintf(format, args...)) }
	if p.font != "" {
		add(`font-family:"%s",system-ui,sans-serif`, p.font)
	}
	if p.size > 0 {
		add("font-size:%spt", num(p.size))
	}
	if p.bold != nil {
		add("font-weight:%s", map[bool]string{true: "700", false: "400"}[*p.bold])
	}
	if p.italic != nil {
		add("font-style:%s", map[bool]string{true: "italic", false: "normal"}[*p.italic])
	}
	var deco []string
	if p.underline != nil && *p.underline {
		deco = append(deco, "underline")
	}
	if p.strike != nil && *p.strike {
		deco = append(deco, "line-through")
	}
	if len(deco) > 0 {
		add("text-decoration:%s", strings.Join(deco, " "))
	} else if p.underline != nil || p.strike != nil {
		add("text-decoration:none")
	}
	if p.caps != nil {
		add("text-transform:%s", map[bool]string{true: "uppercase", false: "none"}[*p.caps])
	}
	if p.small != nil {
		add("font-variant:%s", map[bool]string{true: "small-caps", false: "normal"}[*p.small])
	}
	if p.colour != "" {
		add("color:%s", p.colour)
	}
	if p.fill != "" {
		add("background-color:%s", p.fill)
	}
	if paragraph {
		if p.align != "" {
			add("text-align:%s", p.align)
		}
		if p.before != nil {
			add("margin-top:%spt", num(*p.before))
		}
		if p.after != nil {
			add("margin-bottom:%spt", num(*p.after))
		}
		if p.left != nil {
			add("margin-left:%spt", num(*p.left))
		}
		if p.right != nil {
			add("margin-right:%spt", num(*p.right))
		}
		if p.first != nil {
			add("text-indent:%spt", num(*p.first))
		}
		if p.line != "" {
			add("line-height:%s", p.line)
		}
		for _, side := range []string{"top", "bottom", "left", "right"} {
			if b, ok := p.border[side]; ok {
				add("border-%s:%s", side, b)
			}
		}
	}
	return strings.Join(out, ";")
}

// scopeName admits the scope a stylesheet is written for: classes and
// attribute selectors of the server's own making.
var scopeName = regexp.MustCompile(`^[.A-Za-z0-9_-]+$`)

// bodyStyles are the styles the text of a page takes, by the elements the
// reading view draws it with: a heading of level 1 is h2, the page's
// title being h1.
var bodyStyles = []struct{ typ, name, selector string }{
	{"paragraph", "heading 1", "h2"},
	{"paragraph", "heading 2", "h3"},
	{"paragraph", "heading 3", "h4"},
	{"paragraph", "Quote", "blockquote"},
	{"paragraph", "caption", "figcaption"},
	{"paragraph", "Code", "pre"},
	{"paragraph", "Panel", ".pm-panel"},
}

// Stylesheet writes the CSS that draws a page as this template: the
// sheet with the page's size and margins, a class per paragraph style
// for the template's own content, and its styles applied to the page's
// text. Every rule is under scope, so it draws nothing else.
func (t *Template) Stylesheet(scope string) string {
	if !scopeName.MatchString(scope) {
		return ""
	}
	var out strings.Builder
	rule := func(sel, decl string) {
		if decl != "" {
			fmt.Fprintf(&out, "%s %s{%s}\n", scope, sel, decl)
		}
	}
	pg := t.info.Page
	normal := t.resolved(t.DefaultParagraphStyle())
	base := normal
	base.before, base.after, base.left, base.right, base.first, base.border, base.align, base.fill = nil, nil, nil, nil, nil, nil, "", ""
	rule(".wt-sheet", fmt.Sprintf("width:%spt;min-height:%spt;", num(pg.Width), num(pg.Height))+base.css(true))
	rule(".wt-head", fmt.Sprintf("min-height:%spt;padding:%spt %spt 0 %spt", num(pg.Top), num(pg.Header), num(pg.Right), num(pg.Left)))
	rule(".wt-main", fmt.Sprintf("padding:0 %spt 0 %spt", num(pg.Right), num(pg.Left)))
	rule(".wt-foot", fmt.Sprintf("min-height:%spt;padding:0 %spt %spt %spt", num(pg.Bottom), num(pg.Right), num(pg.Footer), num(pg.Left)))
	for _, st := range t.styles.list {
		if st.typ == "paragraph" {
			rule(".wt-"+st.class, t.resolved(st.id).css(true))
		}
	}
	text := ".wt-body .ProseMirror"
	rule(text+" p,"+scope+" "+text+" .pm-bullet,"+scope+" "+text+" .pm-ordered,"+scope+" "+text+" .pm-task,"+scope+" "+text+" td,"+scope+" "+text+" th", normal.css(true))
	for _, b := range bodyStyles {
		if id := t.StyleID(b.typ, b.name); id != "" {
			rule(text+" "+b.selector, t.resolved(id).css(true))
		}
	}
	if id := t.StyleID("character", "Hyperlink"); id != "" {
		var p props
		for _, st := range t.styles.chain(id) {
			t.run(st.rPr, &p)
		}
		rule(text+" a", p.css(false))
	}
	t.headingNumbers(&out, scope, text)
	fmt.Fprintf(&out, "@media (max-width: 760px){%s .wt-sheet{width:auto;min-height:0}%s .wt-head,%s .wt-main,%s .wt-foot{padding-left:16px;padding-right:16px}}\n", scope, scope, scope, scope)
	return out.String()
}

// literal admits the text a level's number shows around its counters.
var literal = regexp.MustCompile(`[^\p{L}\p{N} .:,;()\-–/]`)

// counterStyles are the CSS counter styles of Word's number formats.
var counterStyles = map[string]string{
	"decimal": "decimal", "decimalZero": "decimal-leading-zero", "upperRoman": "upper-roman", "lowerRoman": "lower-roman",
	"upperLetter": "upper-alpha", "lowerLetter": "lower-alpha",
}

// headingNumbers numbers the headings of a page's text as the template's
// heading styles number theirs, by CSS counters.
func (t *Template) headingNumbers(out *strings.Builder, scope, text string) {
	type numbered struct {
		level int
		lvl   map[int]level
	}
	var found []numbered
	for k := 1; k <= 3; k++ {
		id := t.StyleID("paragraph", fmt.Sprintf("heading %d", k))
		if id == "" {
			continue
		}
		numID, ilvl := "", k-1
		for _, st := range t.styles.chain(id) {
			if np := st.pPr.child("w:numPr"); np != nil {
				if v, ok := np.val("w:numId"); ok {
					numID = v
				}
				if v, ok := np.val("w:ilvl"); ok {
					if n, err := strconv.Atoi(v); err == nil {
						ilvl = n
					}
				}
			}
		}
		levels := t.num.abstract[t.num.nums[numID]]
		if numID == "" || numID == "0" || levels == nil || ilvl != k-1 {
			continue
		}
		if counterStyles[levels[k-1].format] == "" {
			continue
		}
		found = append(found, numbered{k, levels})
	}
	if len(found) == 0 {
		return
	}
	fmt.Fprintf(out, "%s %s{counter-reset:wtn1 wtn2 wtn3}\n", scope, text)
	for _, f := range found {
		resets := []string{}
		for j := f.level + 1; j <= 3; j++ {
			resets = append(resets, fmt.Sprintf("wtn%d", j))
		}
		sel := fmt.Sprintf("%s %s h%d", scope, text, f.level+1)
		decl := fmt.Sprintf("counter-increment:wtn%d", f.level)
		if len(resets) > 0 {
			decl += ";counter-reset:" + strings.Join(resets, " ")
		}
		fmt.Fprintf(out, "%s{%s}\n", sel, decl)
		fmt.Fprintf(out, "%s::before{content:%s}\n", sel, levelContent(f.lvl[f.level-1].text, f.lvl))
	}
}

var levelRef = regexp.MustCompile(`%[1-9]`)

// levelContent writes a level's text, "%1.%2", as CSS content.
func levelContent(text string, levels map[int]level) string {
	var parts []string
	at := 0
	for _, m := range levelRef.FindAllStringIndex(text, -1) {
		if lit := literal.ReplaceAllString(text[at:m[0]], ""); lit != "" {
			parts = append(parts, strconv.Quote(lit))
		}
		n := int(text[m[0]+1] - '0')
		style := counterStyles[levels[n-1].format]
		if style == "" {
			style = "decimal"
		}
		if n <= 3 {
			parts = append(parts, fmt.Sprintf("counter(wtn%d,%s)", n, style))
		}
		at = m[1]
	}
	if lit := literal.ReplaceAllString(text[at:], ""); lit != "" {
		parts = append(parts, strconv.Quote(lit))
	}
	parts = append(parts, `" "`)
	return strings.Join(parts, " ")
}
