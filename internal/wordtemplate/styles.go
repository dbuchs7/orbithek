package wordtemplate

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// style is one style of a template's styles.xml.
type style struct {
	id, name, typ, basedOn string
	def                    bool
	pPr, rPr, tblPr        *node
	// class is the style's class in the stylesheet of the frame: "s" and
	// its place among the styles, never the template's ID, which may hold
	// anything.
	class string
}

type styleSheet struct {
	list      []*style
	byID      map[string]*style
	byName    map[string]*style // "paragraph/heading 1", lower case
	defPPr    *node
	defRPr    *node
	defaultID map[string]string // the default style of each type
}

func readStyles(root *node) *styleSheet {
	s := &styleSheet{byID: map[string]*style{}, byName: map[string]*style{}, defaultID: map[string]string{}}
	if dd := root.child("w:docDefaults"); dd != nil {
		s.defPPr = dd.path("w:pPrDefault", "w:pPr")
		s.defRPr = dd.path("w:rPrDefault", "w:rPr")
	}
	for _, k := range root.kids {
		if k.name != "w:style" {
			continue
		}
		st := &style{id: k.attr["w:styleId"], typ: k.attr["w:type"], pPr: k.child("w:pPr"), rPr: k.child("w:rPr"), tblPr: k.child("w:tblPr")}
		st.name, _ = k.val("w:name")
		st.basedOn, _ = k.val("w:basedOn")
		switch k.attr["w:default"] {
		case "1", "true", "on":
			st.def = true
			if s.defaultID[st.typ] == "" {
				s.defaultID[st.typ] = st.id
			}
		}
		if st.id == "" || s.byID[st.id] != nil {
			continue
		}
		st.class = fmt.Sprintf("s%d", len(s.list))
		s.list = append(s.list, st)
		s.byID[st.id] = st
		key := st.typ + "/" + strings.ToLower(st.name)
		if s.byName[key] == nil {
			s.byName[key] = st
		}
	}
	return s
}

// StyleID is the ID of the template's style of the type ("paragraph",
// "character", "table") with the name, matched without regard to case:
// Word names its built-in styles in English ("heading 1", "Title") in
// every language, while their IDs follow the language the template was
// made in. Empty when the template has no such style.
func (t *Template) StyleID(typ, name string) string {
	if st := t.styles.byName[typ+"/"+strings.ToLower(name)]; st != nil {
		return st.id
	}
	return ""
}

// StyleIDTaken reports whether a style of any type has the ID.
func (t *Template) StyleIDTaken(id string) bool { return t.styles.byID[id] != nil }

// DefaultParagraphStyle is the ID of the paragraph style a paragraph
// without one has: "Normal" in English templates.
func (t *Template) DefaultParagraphStyle() string {
	if id := t.styles.defaultID["paragraph"]; id != "" {
		return id
	}
	return "Normal"
}

// chain is a style with the styles it is based on, the farthest first.
func (s *styleSheet) chain(id string) []*style {
	var out []*style
	seen := map[string]bool{}
	for id != "" && !seen[id] && len(out) < 32 {
		seen[id] = true
		st := s.byID[id]
		if st == nil {
			break
		}
		out = append([]*style{st}, out...)
		id = st.basedOn
	}
	return out
}

// theme holds a template's theme colours and fonts.
type theme struct {
	colours      map[string]string // "accent1" → "4472C4"
	major, minor string
}

func readTheme(root *node) theme {
	th := theme{colours: map[string]string{}}
	el := root.child("a:themeElements")
	if cs := el.child("a:clrScheme"); cs != nil {
		for _, c := range cs.kids {
			name := strings.TrimPrefix(c.name, "a:")
			if v := c.child("a:srgbClr"); v != nil {
				th.colours[name] = v.attr["val"]
			} else if v := c.child("a:sysClr"); v != nil {
				th.colours[name] = v.attr["lastClr"]
			}
		}
	}
	if fs := el.child("a:fontScheme"); fs != nil {
		th.major = fs.path("a:majorFont", "a:latin").attrOr("typeface")
		th.minor = fs.path("a:minorFont", "a:latin").attrOr("typeface")
	}
	return th
}

func (n *node) attrOr(name string) string {
	if n == nil {
		return ""
	}
	return n.attr[name]
}

// themeColourNames maps a w:themeColor to the theme's colour.
var themeColourNames = map[string]string{
	"dark1": "dk1", "text1": "dk1", "light1": "lt1", "background1": "lt1",
	"dark2": "dk2", "text2": "dk2", "light2": "lt2", "background2": "lt2",
	"accent1": "accent1", "accent2": "accent2", "accent3": "accent3", "accent4": "accent4", "accent5": "accent5", "accent6": "accent6",
	"hyperlink": "hlink", "followedHyperlink": "folHlink",
}

var hexColour = regexp.MustCompile(`^[0-9A-Fa-f]{6}$`)

// colour reads a w:color or w:shd element's colour as #rrggbb; the theme
// colour with its tint or shade when it names one.
func (th theme) colour(n *node, attr string) string {
	if n == nil {
		return ""
	}
	if name := n.attr[themeAttr(attr)]; name != "" {
		if v := th.colours[themeColourNames[name]]; hexColour.MatchString(v) {
			return "#" + strings.ToUpper(shade(v, n, attr))
		}
	}
	v := n.attr[attr]
	if !hexColour.MatchString(v) {
		return ""
	}
	return "#" + strings.ToUpper(v)
}

// themeAttr is the theme attribute that goes with a colour attribute.
func themeAttr(attr string) string {
	if attr == "w:fill" {
		return "w:themeFill"
	}
	return "w:themeColor"
}

// shade applies a theme colour's tint or shade, given in hexadecimal as
// a fraction of 255.
func shade(hexv string, n *node, attr string) string {
	tintName, shadeName := "w:themeTint", "w:themeShade"
	if attr == "w:fill" {
		tintName, shadeName = "w:themeFillTint", "w:themeFillShade"
	}
	tint, terr := strconv.ParseUint(n.attr[tintName], 16, 8)
	sh, serr := strconv.ParseUint(n.attr[shadeName], 16, 8)
	if terr != nil && serr != nil {
		return hexv
	}
	var out strings.Builder
	for i := 0; i < 6; i += 2 {
		c, _ := strconv.ParseUint(hexv[i:i+2], 16, 8)
		f := float64(c)
		if serr == nil {
			f = f * float64(sh) / 255
		}
		if terr == nil {
			f = f + (255-f)*(1-float64(tint)/255)
		}
		fmt.Fprintf(&out, "%02X", int(math.Round(math.Min(255, math.Max(0, f)))))
	}
	return out.String()
}

// numbering holds the levels of the template's lists.
type numbering struct {
	abstract map[string]map[int]level // abstractNumId → level → its format
	nums     map[string]string        // numId → abstractNumId
	maxAbs   int
	maxNum   int
}

type level struct{ format, text string }

func readNumbering(root *node) numbering {
	nb := numbering{abstract: map[string]map[int]level{}, nums: map[string]string{}, maxAbs: -1}
	for _, k := range root.kids {
		switch k.name {
		case "w:abstractNum":
			id := k.attr["w:abstractNumId"]
			if n, err := strconv.Atoi(id); err == nil {
				nb.maxAbs = max(nb.maxAbs, n)
			}
			levels := map[int]level{}
			for _, l := range k.kids {
				if l.name != "w:lvl" {
					continue
				}
				i, err := strconv.Atoi(l.attr["w:ilvl"])
				if err != nil {
					continue
				}
				f, _ := l.val("w:numFmt")
				txt, _ := l.val("w:lvlText")
				levels[i] = level{f, txt}
			}
			nb.abstract[id] = levels
		case "w:num":
			id := k.attr["w:numId"]
			if n, err := strconv.Atoi(id); err == nil {
				nb.maxNum = max(nb.maxNum, n)
			}
			nb.nums[id], _ = k.val("w:abstractNumId")
		}
	}
	return nb
}

// NumberingBase returns the first abstract numbering ID and the first
// numbering ID that the template does not use, for the lists an export
// adds.
func (t *Template) NumberingBase() (abstract, num int) {
	if t.num.abstract == nil {
		return 0, 1
	}
	return t.num.maxAbs + 1, t.num.maxNum + 1
}
