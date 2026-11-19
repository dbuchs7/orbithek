package export

import (
	"strconv"
	"strings"

	"github.com/dbuchs7/orbithek/internal/item"
)

// Child is a child page of a page, with its own child pages, as a list of
// child pages shows them (ADR-0042, ADR-0239, ADR-0240).
type Child struct {
	Title    string
	Children []Child
	// Under is the title of the left-out item the page stands in place
	// of, which the list names beside it; empty when the page stands
	// below its own parent (ADR-0240).
	Under string
}

// ChildListKey is what a list of child pages lists its pages by: its
// depth and the types it leaves out, so that the lists of a page that
// show the same pages share them (ADR-0239, ADR-0240).
func ChildListKey(b item.Block) string {
	depth := b.Depth
	if depth == 0 {
		depth = 1
	}
	return strconv.Itoa(depth) + "|" + strings.Join(b.HideTypes, ",")
}

// childLine is one entry of a list of child pages: its title and how far
// below the page it stands, 0 for a child of the page itself.
type childLine struct {
	Title string
	Level int
}

// childLines lists the child pages of a list, each followed by its own,
// as the server read them for the list: to its depth, without the types
// it leaves out, the pages below a left-out item in its place and naming
// it, "A (under B)", in the document's words (ADR-0240).
func (d Document) childLines(kids []Child) []childLine {
	under := d.Under
	if under == "" {
		under = "under"
	}
	var out []childLine
	var walk func(kids []Child, level int)
	walk = func(kids []Child, level int) {
		for _, c := range kids {
			title := c.Title
			if c.Under != "" {
				title += " (" + under + " " + c.Under + ")"
			}
			out = append(out, childLine{Title: title, Level: level})
			walk(c.Children, level+1)
		}
	}
	walk(kids, 0)
	return out
}
