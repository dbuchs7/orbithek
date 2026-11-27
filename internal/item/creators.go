package item

// creatorOf is the user who made an item: the actor of its creating event.
// It is derived from the log, never stored on the item, so it cannot be
// changed or drift from what happened (ADR-0263).
func (s *Store) creatorOf(it *Item) string {
	if it.created < 1 || it.created > len(s.log) {
		return ""
	}
	return s.log[it.created-1].Actor
}

// eachContributor calls do with everyone who made or changed an item, each
// once: its creator first, then the actor of every event that changed the
// item itself, in the order of their first change, as the query field
// contributor counts them (changesItem; ADR-0261,
// ADR-draft-search-and-starmap-find-items-by-their-contributors). It
// allocates only for an item changed by many people.
func (s *Store) eachContributor(it *Item, do func(user string)) {
	var seen [8]string
	n := 0
	var more map[string]bool
	visit := func(user string) {
		if user == "" {
			return
		}
		for _, u := range seen[:n] {
			if u == user {
				return
			}
		}
		if more[user] {
			return
		}
		if n < len(seen) {
			seen[n] = user
			n++
		} else {
			if more == nil {
				more = map[string]bool{}
			}
			more[user] = true
		}
		do(user)
	}
	visit(s.creatorOf(it))
	for _, i := range s.touching[it.ID] {
		if e := s.log[i]; e.Item == it.ID && changesItem[e.Kind] {
			visit(e.Actor)
		}
	}
}

// contributorsOf lists everyone who made or changed an item, its creator
// first (eachContributor).
func (s *Store) contributorsOf(it *Item) []string {
	var out []string
	s.eachContributor(it, func(user string) { out = append(out, user) })
	return out
}

// Contributors counts, per user, the readable live items that user made or
// changed (ADR-draft-search-and-starmap-find-items-by-their-contributors).
// Only the contributors to items the principal reads are named, so the
// answer cannot list the accounts, nor reveal an item the principal may
// not read (D10).
func (x *Session) Contributors() map[string]int {
	out := map[string]int{}
	for _, it := range x.s.items {
		if it.Presence != Live || !x.s.canRead(x.who, it) {
			continue
		}
		x.s.eachContributor(it, func(user string) { out[user]++ })
	}
	return out
}
