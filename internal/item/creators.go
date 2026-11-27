package item

// creatorOf is the user who made an item: the actor of its creating event.
// It is derived from the log, never stored on the item, so it cannot be
// changed or drift from what happened (ADR-draft-search-and-starmap-find-items-by-their-creator).
func (s *Store) creatorOf(it *Item) string {
	if it.created < 1 || it.created > len(s.log) {
		return ""
	}
	return s.log[it.created-1].Actor
}

// Creators counts, per user, the readable live items that user made. Only
// the makers of items the principal reads are named, so the answer cannot
// list the accounts, nor reveal an item the principal may not read (D10).
func (x *Session) Creators() map[string]int {
	out := map[string]int{}
	for _, it := range x.s.items {
		if it.Presence != Live || !x.s.canRead(x.who, it) {
			continue
		}
		if c := x.s.creatorOf(it); c != "" {
			out[c]++
		}
	}
	return out
}
