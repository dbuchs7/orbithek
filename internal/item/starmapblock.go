package item

import (
	"slices"
	"strings"
)

// A starmap in a text (ADR-0252) is a block of the
// kind starmap that names the map it shows and how it is narrowed when it
// is shown: the room (none for the room of the item the text is on,
// StarmapAllRooms for every room the reader reads), a label, a search
// text, the types and kinds of node it leaves out, and the reach of the
// neighbourhood of the item it is on. The reader may change all of it
// while reading; nothing of that is kept. The block holds no item and no
// copy of one (D4): every reader's browser draws the map the server
// derives for them (ADR-0157, ADR-0191), so a reader sees only what they
// read (D10).

// StarmapAllRooms names every room the reader reads as the room of a
// starmap in a text, as "*" does in the starmap's address (ADR-0191).
const StarmapAllRooms = "*"

// StarmapKinds are the kinds of node a starmap in a text may leave out,
// in sorted order, as the starmap's filters name them (ADR-0157,
// ADR-0250): finished tasks, labels, items of other rooms and pictures.
var StarmapKinds = []string{"done", "label", "outside", "picture"}

// MaxStarmapDepth is the farthest reach of the neighbourhood a starmap in
// a text shows of the item it is on, as a separated neighbourhood reaches
// on the starmap (ADR-0158).
const MaxStarmapDepth = 3

// checkStarmapBlock validates a starmap in a text.
func checkStarmapBlock(b Block) error {
	switch {
	case b.Text != "" || len(b.Marks) > 0 || len(b.Mentions) > 0:
		return invalid("a starmap has no text: it names what it shows")
	case b.Space != "" && b.Space != StarmapAllRooms && !spaceKeyRE.MatchString(b.Space):
		return invalid("room %q: a starmap names a room by its key, %s for every room, or none for the room of its item", b.Space, StarmapAllRooms)
	case b.Label != "" && !validStarmapLabel(b.Label):
		return invalid("label %q: a starmap names one label, as stored (ADR-0044), or none", b.Label)
	case b.Search != "" && !validViewName(b.Search):
		return invalid("search %q: a starmap's search text has 1 to %d characters without control characters or surrounding spaces", b.Search, maxViewName)
	case b.Depth < 0 || b.Depth > MaxStarmapDepth:
		return invalid("depth %d: a starmap shows the neighbourhood of its item to a depth of 1 to %d, or 0 for the whole map", b.Depth, MaxStarmapDepth)
	}
	if b.HideKinds != nil {
		if len(b.HideKinds) == 0 {
			return invalid("a starmap leaves out some of %s, or none named", strings.Join(StarmapKinds, ", "))
		}
		last := -1
		for _, k := range b.HideKinds {
			i := slices.Index(StarmapKinds, k)
			if i <= last {
				return invalid("kind %q: a starmap leaves out some of %s, each once and in this order", k, strings.Join(StarmapKinds, ", "))
			}
			last = i
		}
	}
	return nil
}

// validStarmapLabel reports whether label is a label as it is stored.
func validStarmapLabel(label string) bool {
	l, ok := NormalLabel(label)
	return ok && l == label
}
