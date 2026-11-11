package item

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The order of the slides of a folder or a collection of the album, set
// by hand (ADR-0195). The album presents the pictures of a folder or a
// collection as slides (ADR-0176), and shows them in that order on asking.
// Whoever may edit the folder or the collection moves pictures within it;
// the order is kept on the folder's or collection's item as the first
// versions of the stacks (ADR-0103), so that a drawing saved again keeps
// its place and a picture moved to another item with its identity
// (ADR-0188) keeps it in a collection. A picture the order does not name
// comes after those it names, by name.
//
// The order names files, not their content: it is not sealed (ADR-0026),
// and it is no reference (D8): a file it names may be deleted or moved
// away, and is then passed over. It is never given to a caller; it only
// sorts what the caller reads (D10).

// slideStack is a stack of versions as the slides are ordered: its first
// version, the name of its newest version, its item's key and the newest
// version's ID.
type slideStack struct {
	first ID
	key   SlideKey
}

// SlideKey is what the slides are ordered by: the stack's first version,
// for the order set by hand, and the newest version's name, its item's
// key and its ID for the pictures the order does not name.
type SlideKey struct {
	First      ID
	Name       string
	ItemKey    string
	Attachment ID
}

// SlideRanks is the place of every stack an order set by hand names, by
// the stack's first version.
type SlideRanks map[ID]int

// Compare orders two slides: those the order names by their place in it,
// before every other; the others by name, then by their item's key and
// their file, so the order is the same everywhere.
func (r SlideRanks) Compare(a, b SlideKey) int {
	ra, oka := r[a.First]
	rb, okb := r[b.First]
	switch {
	case oka && okb && ra != rb:
		return ra - rb
	case oka && !okb:
		return -1
	case !oka && okb:
		return 1
	}
	if c := ComparePictureNames(SlideCaption(a.Name), SlideCaption(b.Name)); c != 0 {
		return c
	}
	if c := ComparePictureNames(a.ItemKey, b.ItemKey); c != 0 {
		return c
	}
	return strings.Compare(string(a.Attachment), string(b.Attachment))
}

// SlideCaption is a slide's name: its file name without the extension or
// a drawing's marker, as the album shows it.
func SlideCaption(name string) string {
	if strings.HasSuffix(name, drawingSuffix) {
		return strings.TrimSuffix(name, drawingSuffix)
	}
	if i := strings.LastIndexByte(name, '.'); i > 0 {
		return name[:i]
	}
	return name
}

// ComparePictureNames compares two names as the slides are ordered by
// name: a run of digits by its value, so "2 Plan" comes before
// "10 Outlook", and letters regardless of case. Of two runs of the same
// value, the one with fewer leading zeros comes first; names equal so are
// compared as they are written.
func ComparePictureNames(a, b string) int {
	x, y := a, b
	for x != "" && y != "" {
		if isDigit(x[0]) && isDigit(y[0]) {
			dx, dy := digitRun(x), digitRun(y)
			if c := compareNumbers(dx, dy); c != 0 {
				return c
			}
			x, y = x[len(dx):], y[len(dy):]
			continue
		}
		rx, nx := utf8.DecodeRuneInString(x)
		ry, ny := utf8.DecodeRuneInString(y)
		if lx, ly := unicode.ToLower(rx), unicode.ToLower(ry); lx != ly {
			if lx < ly {
				return -1
			}
			return 1
		}
		x, y = x[nx:], y[ny:]
	}
	switch {
	case x == "" && y != "":
		return -1
	case x != "" && y == "":
		return 1
	}
	return strings.Compare(a, b)
}

func isDigit(c byte) bool { return '0' <= c && c <= '9' }

// digitRun is the run of ASCII digits s starts with.
func digitRun(s string) string {
	i := 0
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	return s[:i]
}

// compareNumbers compares two runs of digits by their value, of any
// length; of equal value, the shorter run first.
func compareNumbers(a, b string) int {
	ta, tb := strings.TrimLeft(a, "0"), strings.TrimLeft(b, "0")
	if len(ta) != len(tb) {
		return len(ta) - len(tb)
	}
	if c := strings.Compare(ta, tb); c != 0 {
		return c
	}
	return len(a) - len(b)
}

// slideContainer returns a live folder or collection of the album the
// principal may read.
func (x *Session) slideContainer(id ID) (*Item, error) {
	it, err := x.readable(id)
	if err != nil {
		return nil, err
	}
	if it.Type != FolderType && it.Type != CollectionType {
		return nil, invalid("%s is no folder or collection of the album", it.Key)
	}
	if it.Presence != Live {
		return nil, invalid("%s is %s", it.Key, it.Presence)
	}
	return it, nil
}

// SlideRanks returns the order set by hand of a folder or collection the
// principal may read, as the places of the stacks it names; empty without
// one. It may name stacks the principal does not read: it only sorts what
// they read, and is never shown.
func (x *Session) SlideRanks(id ID) (SlideRanks, error) {
	it, err := x.slideContainer(id)
	if err != nil {
		return nil, err
	}
	return ranksOf(it.PictureOrder), nil
}

func ranksOf(order []ID) SlideRanks {
	r := SlideRanks{}
	for i, id := range order {
		if _, seen := r[id]; !seen {
			r[id] = i
		}
	}
	return r
}

// slideStacks returns the stacks the principal reads in a folder (its
// files) or a collection (the pictures of every room its rule gathers;
// a stack counts when one of its versions matches, as in the album).
func (x *Session) slideStacks(c *Item) []slideStack {
	var match func(Picture) bool
	if c.Type == FolderType {
		match = func(p Picture) bool { return p.Item.ID == c.ID }
	} else {
		rule := ruleOfProps(c.Properties)
		match = rule.Matches
	}
	space := ""
	if c.Type == FolderType {
		space = c.Space
	}
	type key struct{ item, first ID }
	at := map[key]int{}
	var out []slideStack
	var matched []bool
	for _, p := range x.Pictures(space, func(Attachment) bool { return true }) {
		k := key{p.Item.ID, p.First}
		i, ok := at[k]
		if !ok {
			// Pictures come newest first: the first of a stack is its top.
			i = len(out)
			at[k] = i
			out = append(out, slideStack{first: p.First, key: SlideKey{First: p.First, Name: p.Attachment.Name, ItemKey: p.Item.Key, Attachment: p.Attachment.ID}})
			matched = append(matched, false)
		}
		if match(p) {
			matched[i] = true
		}
	}
	kept := out[:0]
	for i, st := range out {
		if matched[i] {
			kept = append(kept, st)
		}
	}
	return kept
}

// liveFirsts is the first version of every stack with a version that is
// not deleted: in the folder c, or in every live item for a collection.
// It is read whatever the principal may read, to forget what the order
// names and is gone; nothing of it reaches the principal.
func (s *Store) liveFirsts(c *Item) map[ID]bool {
	out := map[ID]bool{}
	add := func(it *Item) {
		for _, a := range it.Attachments {
			if !a.Deleted {
				out[firstVersion(it, a)] = true
			}
		}
	}
	if c.Type == FolderType {
		add(c)
		return out
	}
	for _, it := range s.items {
		if it.Presence == Live {
			add(it)
		}
	}
	return out
}

// PlacePictures moves pictures of a folder or collection the principal
// may edit to a place in its order of slides (ADR-0195): moved, the first
// versions of their stacks in the order they are to have, go right before
// the stack before names, or to the end when before is empty. Every stack
// named must be one the principal reads there.
//
// The order kept names every stack in it at once: the stacks it named
// before, as far as they still have a version, then those the principal
// reads and it did not name, by name, as the principal saw them; then the
// moved ones take their new place. Stacks the principal does not read
// keep theirs.
func (x *Session) PlacePictures(id ID, moved []ID, before ID) error {
	if _, err := x.editable(id); err != nil {
		return err
	}
	c, err := x.slideContainer(id)
	if err != nil {
		return err
	}
	stacks := x.slideStacks(c)
	if len(moved) == 0 {
		return invalid("name the pictures to move")
	}
	if len(moved) > len(stacks) {
		return invalid("%s holds %d pictures; %d are named", c.Key, len(stacks), len(moved))
	}
	seen := map[ID]bool{}
	for _, st := range stacks {
		seen[st.first] = false
	}
	for _, m := range moved {
		used, ok := seen[m]
		switch {
		case !ok:
			return notFound("picture %s in %s", m, c.Key)
		case used:
			return invalid("picture %s is named twice", m)
		}
		seen[m] = true
	}
	if before != "" {
		used, ok := seen[before]
		switch {
		case !ok:
			return notFound("picture %s in %s", before, c.Key)
		case used:
			return invalid("picture %s cannot be placed before itself", before)
		}
	}
	// What the order named and is gone is forgotten.
	live := x.s.liveFirsts(c)
	var full []ID
	named := map[ID]bool{}
	for _, f := range c.PictureOrder {
		if live[f] && !named[f] {
			full = append(full, f)
			named[f] = true
		}
	}
	ranks := ranksOf(full)
	slices.SortFunc(stacks, func(a, b slideStack) int { return ranks.Compare(a.key, b.key) })
	for _, st := range stacks {
		if !named[st.first] {
			full = append(full, st.first)
			named[st.first] = true
		}
	}
	full = slices.DeleteFunc(full, func(f ID) bool { return seen[f] })
	at := len(full)
	if before != "" {
		at = slices.Index(full, before)
	}
	full = slices.Insert(full, at, moved...)
	if slices.Equal(full, c.PictureOrder) {
		return nil
	}
	return x.s.record(x.who, Event{Kind: PicturesOrdered, Item: c.ID, Order: full})
}

// ClearPictureOrder forgets the order set by hand of a folder or
// collection the principal may edit: its slides are ordered by name
// again.
func (x *Session) ClearPictureOrder(id ID) error {
	if _, err := x.editable(id); err != nil {
		return err
	}
	c, err := x.slideContainer(id)
	if err != nil {
		return err
	}
	if len(c.PictureOrder) == 0 {
		return nil
	}
	return x.s.record(x.who, Event{Kind: PicturesOrdered, Item: c.ID})
}

func (s *Store) applyPictureOrder(e Event, touch func(...ID)) {
	it := s.items[e.Item]
	if it == nil {
		return
	}
	it.PictureOrder = slices.Clone(e.Order)
	touch(it.ID)
}
