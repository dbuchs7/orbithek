package item

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/dbuchs7/orbithek/internal/query"
)

// Views of the items of a type (ADR-0203): a table, a gallery or a list,
// grouped by a field, computed when shown from the items the reader may
// read. A view of its own is an address of the UI and keeps nothing; a
// view placed in a text is a block of the kind view that names what it
// shows: its layout, the type, the room (none for every room the reader
// reads), a query of the query language (ADR-0136), the grouping, the
// columns and the sort. The block holds no item and no copy of one (D4),
// so a reader sees exactly the items they read (D10).

// The layouts of a view.
const (
	ViewTable   = "table"
	ViewGallery = "gallery"
	ViewList    = "list"
)

// ViewLayouts are the layouts a view block may have.
var ViewLayouts = []string{ViewTable, ViewGallery, ViewList}

// ViewBuiltins are the columns every item has: its key, title, state,
// labels, room, type and when it last changed. A field of its type is
// named ViewFieldPrefix and the field's name.
var ViewBuiltins = []string{"key", "title", "state", "labels", "space", "type", "updated"}

// ViewFieldPrefix begins the name of a column, grouping or sort that is a
// field of the type.
const ViewFieldPrefix = "field:"

// ViewGroups are the built-in groupings of a view besides a field: by
// workflow state and by label.
var ViewGroups = []string{"state", "labels"}

// MaxViewColumns bounds the columns a view block names.
const MaxViewColumns = 24

// maxViewName bounds the name of a type or a field a view block names.
const maxViewName = 200

// validViewName reports whether name can be the name of a type or a field
// a view names: one to maxViewName characters without control characters
// or surrounding spaces.
func validViewName(name string) bool {
	if name == "" || utf8.RuneCountInString(name) > maxViewName || strings.TrimSpace(name) != name || !utf8.ValidString(name) {
		return false
	}
	return !strings.ContainsFunc(name, unicode.IsControl)
}

// validViewColumn reports whether c names a column: a built-in one or a
// field.
func validViewColumn(c string) bool {
	if f, ok := strings.CutPrefix(c, ViewFieldPrefix); ok {
		return validViewName(f)
	}
	return slices.Contains(ViewBuiltins, c)
}

// validViewGroup reports whether g names what a view is grouped by: none,
// the state, the labels or a field.
func validViewGroup(g string) bool {
	if f, ok := strings.CutPrefix(g, ViewFieldPrefix); ok {
		return validViewName(f)
	}
	return g == "" || slices.Contains(ViewGroups, g)
}

// checkViewBlock validates a view block (ADR-0203).
func checkViewBlock(b Block) error {
	switch {
	case b.Text != "" || len(b.Marks) > 0 || len(b.Mentions) > 0:
		return invalid("a view has no text: it names what it shows")
	case !slices.Contains(ViewLayouts, b.Layout):
		return invalid("layout %q: a view is a %s", b.Layout, strings.Join(ViewLayouts, ", "))
	case b.Space != "" && !spaceKeyRE.MatchString(b.Space):
		return invalid("room %q: a view names a room by its key, or none for every room", b.Space)
	case b.ItemType != "" && !validViewName(b.ItemType):
		return invalid("type %q: a view names a type by its name, or none for every type", b.ItemType)
	case !validViewGroup(b.Group):
		return invalid("group %q: a view is grouped by %s or a field (%s<name>), or not at all", b.Group, strings.Join(ViewGroups, ", "), ViewFieldPrefix)
	}
	if b.Columns != nil {
		if len(b.Columns) == 0 || len(b.Columns) > MaxViewColumns {
			return invalid("columns: a view shows 1 to %d columns, or none named for its default", MaxViewColumns)
		}
		seen := map[string]bool{}
		for _, c := range b.Columns {
			if !validViewColumn(c) || seen[c] {
				return invalid("column %q: a view shows each of %s or a field (%s<name>) once", c, strings.Join(ViewBuiltins, ", "), ViewFieldPrefix)
			}
			seen[c] = true
		}
	}
	if b.Sort != "" && !validViewColumn(strings.TrimPrefix(b.Sort, "-")) {
		return invalid("sort %q: a view is sorted by a column, - before it for the other way", b.Sort)
	}
	if b.Query != "" {
		if _, err := query.Parse(b.Query); err != nil {
			return invalid("the view's query: %v", err)
		}
	}
	return nil
}
