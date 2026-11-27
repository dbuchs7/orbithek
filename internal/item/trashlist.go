package item

import (
	"slices"
	"strings"
	"time"
)

// TrashEntry is one entry of a space's trash (ADR-0021): an item the trash
// started from, with what was trashed with it.
type TrashEntry struct {
	// Item names the item: its ID, key, title, space, type and parent.
	Item Item
	// At is when it was trashed, By who trashed it.
	At time.Time
	By string
	// Below counts the items trashed with it that the principal may read,
	// the item itself not counted.
	Below int
	// MayRestore and MayPurge say what the principal may do with it, as
	// Restore and Purge check it.
	MayRestore, MayPurge bool
}

// TrashOf lists the trash of a space: the items the trash started from
// that the principal may read, newest first. Restore and purge act on
// exactly these (ADR-0021). A space the principal holds no role in is
// answered as one that does not exist (D10).
func (x *Session) TrashOf(space string) ([]TrashEntry, error) {
	if !x.s.HasSpace(space) || x.s.spaceRole(x.who, space) < Viewer {
		return nil, notFound("space %q", space)
	}
	// What each trash event took along, counted once for all entries.
	below := map[int]int{}
	for _, it := range x.s.items {
		if it.Presence == Trashed && it.Space == space && x.s.canRead(x.who, it) {
			below[it.trashedBy]++
		}
	}
	var out []TrashEntry
	for _, it := range x.s.items {
		if it.Presence != Trashed || it.Space != space || it.trashedBy == 0 || !x.s.canRead(x.who, it) {
			continue
		}
		e := x.s.log[it.trashedBy-1]
		if e.Item != it.ID {
			continue // trashed with another item: that one is listed
		}
		out = append(out, TrashEntry{
			Item: Item{ID: it.ID, Key: it.Key, Title: it.Title, Space: it.Space, Type: it.Type, Parent: it.Parent, Presence: it.Presence},
			At:   e.At, By: e.Actor, Below: below[it.trashedBy] - 1,
			MayRestore: x.MayRestore(it.ID), MayPurge: x.MayPurge(it.ID),
		})
	}
	slices.SortFunc(out, func(a, b TrashEntry) int {
		if c := b.At.Compare(a.At); c != 0 {
			return c
		}
		return strings.Compare(string(a.Item.ID), string(b.Item.ID))
	})
	return out, nil
}
