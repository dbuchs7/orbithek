package wordtemplate

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
)

// The namespaces a template's parts use, by the prefix this package names
// them with. A part may bind them to other prefixes; the tree resolves
// every name to these, so a lookup never depends on a part's own choice.
const (
	nsW    = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	nsR    = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	nsWP   = "http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
	nsA    = "http://schemas.openxmlformats.org/drawingml/2006/main"
	nsPic  = "http://schemas.openxmlformats.org/drawingml/2006/picture"
	nsMC   = "http://schemas.openxmlformats.org/markup-compatibility/2006"
	nsWPS  = "http://schemas.microsoft.com/office/word/2010/wordprocessingShape"
	nsV    = "urn:schemas-microsoft-com:vml"
	nsRel  = "http://schemas.openxmlformats.org/package/2006/relationships"
	nsCT   = "http://schemas.openxmlformats.org/package/2006/content-types"
	nsDC   = "http://purl.org/dc/elements/1.1/"
	nsCP   = "http://schemas.openxmlformats.org/package/2006/metadata/core-properties"
	nsXML  = "http://www.w3.org/XML/1998/namespace"
	nsWPG  = "http://schemas.microsoft.com/office/word/2010/wordprocessingGroup"
	nsW14  = "http://schemas.microsoft.com/office/word/2010/wordml"
	nsMath = "http://schemas.openxmlformats.org/officeDocument/2006/math"
)

var prefixOf = map[string]string{
	nsW: "w", nsR: "r", nsWP: "wp", nsA: "a", nsPic: "pic", nsMC: "mc", nsWPS: "wps", nsV: "v",
	nsRel: "rel", nsCT: "ct", nsDC: "dc", nsCP: "cp", nsXML: "xml", nsWPG: "wpg", nsW14: "w14", nsMath: "m",
}

// node is an element of a part, with where it stands in the part's bytes,
// so that a change replaces exactly its span and leaves every other byte
// as the template has it.
type node struct {
	name string // "w:p": the canonical prefix and the local name
	attr map[string]string
	kids []*node
	// text is the character data directly inside the element.
	text strings.Builder
	// start and end are the element's span; inner and innerEnd the span
	// of its content. A self-closing element has inner == innerEnd == end.
	start, inner, innerEnd, end int
	parent                      *node
	// src is the part the element is in.
	src []byte
}

// raw is the element's markup as the part has it; empty for nil.
func (n *node) raw() string {
	if n == nil {
		return ""
	}
	return string(n.src[n.start:n.end])
}

// parse reads a part into a tree of its elements. It refuses what is not
// well-formed XML, a document type declaration among it, since a part of a
// package never has one and entities are where XML hides its tricks.
func parse(data []byte) (*node, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Strict = true
	root := &node{name: "#document", attr: map[string]string{}}
	cur := root
	type scope map[string]string
	scopes := []scope{{"xml": nsXML}}
	resolve := func(prefix, local string, attr bool) string {
		if prefix == "" && attr {
			return local
		}
		if prefix == "xmlns" {
			return "xmlns:" + local
		}
		for i := len(scopes) - 1; i >= 0; i-- {
			if uri, ok := scopes[i][prefix]; ok {
				if p, known := prefixOf[uri]; known {
					return p + ":" + local
				}
				return "{" + uri + "}" + local
			}
		}
		if prefix == "" {
			return local
		}
		return prefix + ":" + local
	}
	depth := 0
	for {
		at := int(d.InputOffset())
		tok, err := d.RawToken()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			sc := scope{}
			for _, a := range t.Attr {
				switch {
				case a.Name.Space == "xmlns":
					sc[a.Name.Local] = a.Value
				case a.Name.Space == "" && a.Name.Local == "xmlns":
					sc[""] = a.Value
				}
			}
			scopes = append(scopes, sc)
			n := &node{name: resolve(t.Name.Space, t.Name.Local, false), attr: map[string]string{}, start: at, inner: int(d.InputOffset()), parent: cur, src: data}
			for _, a := range t.Attr {
				n.attr[resolve(a.Name.Space, a.Name.Local, true)] = a.Value
			}
			cur.kids = append(cur.kids, n)
			cur = n
			depth++
		case xml.EndElement:
			if depth == 0 || cur.name != resolve(t.Name.Space, t.Name.Local, false) {
				return nil, fmt.Errorf("element %s:%s closes what it did not open", t.Name.Space, t.Name.Local)
			}
			cur.innerEnd = at
			if int(d.InputOffset()) == at {
				// A self-closing element: its end token takes no bytes.
				cur.innerEnd = cur.inner
			}
			cur.end = int(d.InputOffset())
			cur = cur.parent
			scopes = scopes[:len(scopes)-1]
			depth--
		case xml.CharData:
			if depth > 0 {
				cur.text.Write(t)
			}
		case xml.Directive:
			return nil, errors.New("a document type declaration is not accepted")
		}
	}
	if depth != 0 || len(root.kids) != 1 {
		return nil, errors.New("not one complete root element")
	}
	return root.kids[0], nil
}

// child is the first child of the name, or nil.
func (n *node) child(name string) *node {
	if n == nil {
		return nil
	}
	for _, k := range n.kids {
		if k.name == name {
			return k
		}
	}
	return nil
}

// path follows children by name.
func (n *node) path(names ...string) *node {
	for _, name := range names {
		n = n.child(name)
	}
	return n
}

// val is the w:val attribute of the named child, if the child is there.
func (n *node) val(name string) (string, bool) {
	c := n.child(name)
	if c == nil {
		return "", false
	}
	return c.attr["w:val"], true
}

// on reports whether a toggle property (w:b, w:i, …) is set on n: present
// and not switched off.
func (n *node) on(name string) (bool, bool) {
	c := n.child(name)
	if c == nil {
		return false, false
	}
	switch c.attr["w:val"] {
	case "0", "false", "off", "none":
		return false, true
	}
	return true, true
}

// walk visits n and every element below it, depth first, until visit
// returns false for an element, whose children it then skips.
func (n *node) walk(visit func(*node) bool) {
	if !visit(n) {
		return
	}
	for _, k := range n.kids {
		k.walk(visit)
	}
}

// all collects every element of the name below n, n excluded, not looking
// inside the matches.
func (n *node) all(name string) []*node {
	var out []*node
	for _, k := range n.kids {
		k.walk(func(x *node) bool {
			if x.name == name {
				out = append(out, x)
				return false
			}
			return true
		})
	}
	return out
}

// edit replaces the bytes from start to end.
type edit struct {
	start, end int
	with       string
}

// splice applies edits that do not overlap; nested ones are refused.
func splice(src []byte, edits []edit) ([]byte, error) {
	slices.SortFunc(edits, func(a, b edit) int { return a.start - b.start })
	var out bytes.Buffer
	at := 0
	for _, e := range edits {
		if e.start < at || e.end < e.start || e.end > len(src) {
			return nil, errors.New("overlapping changes to a part")
		}
		out.Write(src[at:e.start])
		out.WriteString(e.with)
		at = e.end
	}
	out.Write(src[at:])
	return out.Bytes(), nil
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
