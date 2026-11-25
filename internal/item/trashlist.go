package item

import (
	"slices"
	"time"
)

// TrashEntry is one trash root of a space as a principal reads it: the
// item a trash started from, with who trashed it and when (ADR-0021).
type TrashEntry struct {
	Item Item
	By   string
	At   time.Time
	// Items counts the items trashed with it that the principal reads,
	// the root included; those it may not read are not counted (D10).
	Items int
	// MayRestore and MayPurge are what Restore and Purge would allow.
	MayRestore bool
	MayPurge   bool
	// Waiting is the key of the item's parent when that parent is not
	// live and the principal reads it: it is to be restored first.
	Waiting string
}

// TrashOf lists the trash roots of a space that the principal reads,
// the most recently trashed first. An item trashed with another is not
// listed on its own: restore and purge act on the root (ADR-0021).
func (x *Session) TrashOf(space string) ([]TrashEntry, error) {
	if !x.s.HasSpace(space) {
		return nil, notFound("space %q", space)
	}
	// The items of each trash event, gathered once for every root.
	batches := map[int][]*Item{}
	for _, it := range x.s.items {
		if it.Presence == Trashed && it.Space == space {
			batches[it.trashedBy] = append(batches[it.trashedBy], it)
		}
	}
	admin := x.s.spaceRole(x.who, space) >= SpaceAdmin
	var out []TrashEntry
	for seq, members := range batches {
		e := x.s.log[seq-1]
		root := x.s.items[e.Item]
		if root == nil || root.trashedBy != seq || root.Space != space || !x.s.canRead(x.who, root) {
			continue
		}
		entry := TrashEntry{Item: x.s.veiled(x.who, root).clone(), By: e.Actor, At: e.At, MayPurge: admin}
		for _, m := range members {
			if x.s.canRead(x.who, m) {
				entry.Items++
			}
		}
		if _, err := x.editable(root.ID); err == nil {
			entry.MayRestore = x.editableEach(root.ID, members) == nil
		}
		if p := x.s.items[root.Parent]; p != nil && p.Presence != Live && x.s.canRead(x.who, p) {
			entry.Waiting = p.Key
		}
		out = append(out, entry)
	}
	slices.SortFunc(out, func(a, b TrashEntry) int { return b.Item.trashedBy - a.Item.trashedBy })
	return out, nil
}
