package item

import "strings"

// ImportSystemPrefix starts the origin's system of every item a person
// imports through their session (ADR-0211), e.g. "import:notes". No
// connector's ID holds a colon, so such an item never shares an origin
// with a connector's, a migration's or a mirrored file's, and a session
// cannot take one of theirs over.
const ImportSystemPrefix = "import:"

// Import makes or updates, as the principal, an item imported from an
// archive a person uploads (ADR-0211), matched by its origin as every
// import is (ADR-0023). Only a room administrator imports into a room. A
// new item goes below u.Parent, which the principal must be allowed to
// edit, or at the top of the room; an item imported before must be one
// the principal may edit, and stays where people moved it. It reports
// whether anything changed.
func (x *Session) Import(u Upsert) (Item, bool, error) {
	if !strings.HasPrefix(u.Origin.System, ImportSystemPrefix) {
		return Item{}, false, invalid("a person imports only with an origin of %s…", ImportSystemPrefix)
	}
	if !x.MayAdminister(u.Space) {
		return Item{}, false, forbidden("%s may not import into %s", x.who, u.Space)
	}
	if id, had := x.s.bySource[u.Origin.key()]; had {
		it, err := x.editable(id)
		if err != nil {
			return Item{}, false, err
		}
		if it.Space != u.Space {
			return Item{}, false, invalid("imported item %s is in %s; an import does not move it to %s", it.Key, it.Space, u.Space)
		}
		u.Parent = it.Parent
	} else if u.Parent != "" {
		p, err := x.editable(u.Parent)
		if err != nil {
			return Item{}, false, err
		}
		if p.Space != u.Space {
			return Item{}, false, invalid("parent %s is not in %s", p.Key, u.Space)
		}
	}
	u.Managed, u.Key, u.ID, u.Linked, u.LinkedFields = "", "", "", "", nil
	return x.s.Upsert(x.who, u)
}
