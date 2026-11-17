package export

import "github.com/dbuchs7/orbithek/internal/item"

// Child is a child page of a page, with its own child pages as deep as the
// page's lists of child pages show (ADR-0042,
// ADR-draft-child-pages-to-a-chosen-depth).
type Child struct {
	Title    string
	Children []Child
}

// childLine is one entry of a list of child pages: its title and how far
// below the page it stands, 0 for a child of the page itself.
type childLine struct {
	Title string
	Level int
}

// childLines lists the child pages a list of child pages shows, each
// followed by its own, to the list's depth: 0 for one level, as before,
// or item.ChildrenAll for every level.
func childLines(kids []Child, depth int) []childLine {
	if depth == 0 {
		depth = 1
	}
	var out []childLine
	var walk func(kids []Child, level int)
	walk = func(kids []Child, level int) {
		for _, c := range kids {
			out = append(out, childLine{Title: c.Title, Level: level})
			if depth == item.ChildrenAll || level+1 < depth {
				walk(c.Children, level+1)
			}
		}
	}
	walk(kids, 0)
	return out
}
