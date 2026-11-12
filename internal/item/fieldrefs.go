package item

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Room types (ADR-0201) and the field kinds that refer to items and files
// (ADR-0202).

// usable returns the latest version of a type when items of it may live
// in space: an installation type everywhere, a room type in its room only.
func (s *Store) usable(name, space string) (TypeDef, error) {
	td, ok := s.latest(name)
	if !ok {
		return TypeDef{}, notFound("type %q", name)
	}
	if !td.UsableIn(space) {
		return TypeDef{}, invalid("type %q belongs to the space %s; its items live only there", name, td.Space)
	}
	return td, nil
}

// checkTypeDef checks what a definition must agree on with the store: the
// room it belongs to exists and is kept across versions, so a name is
// never used by two spaces at once. The types an item field refers to may
// be defined later, as a restore or an import defines them in turn; until
// then no item fits them.
func (s *Store) checkTypeDef(td TypeDef) error {
	if td.Space != "" && !s.HasSpace(td.Space) {
		return notFound("space %q", td.Space)
	}
	if cur, ok := s.latest(td.Name); ok && cur.Space != td.Space {
		if cur.Space == "" {
			return invalid("the name %q is taken by a type of the whole installation", td.Name)
		}
		return invalid("the name %q is taken by a type of another space", td.Name)
	}
	return nil
}

// checkRefs checks the values of item and file fields that a change sets
// on the item self (nil for a new item): a referenced item exists, is live
// unless the item already refers to it there, is not the item itself and
// is of a type the field allows; a file is one of the item's own, not
// deleted (ADR-0202).
func (s *Store) checkRefs(td TypeDef, self *Item, props Props) error {
	for _, name := range slices.Sorted(maps.Keys(props)) {
		f, ok := td.field(name)
		if !ok || props[name] == nil {
			continue
		}
		switch f.Kind {
		case KindItem:
			to, _ := props[name].(ID)
			target, ok := s.items[to]
			if !ok {
				return notFound("referenced item %s", to)
			}
			if self != nil && to == self.ID {
				return invalid("field %q: an item cannot refer to itself", name)
			}
			kept := self != nil && self.Properties[name] == to
			if target.Presence != Live && !kept {
				return invalid("item %s is %s and cannot be newly referenced", target.Key, target.Presence)
			}
			if len(f.Targets) > 0 && !slices.Contains(f.Targets, target.Type) && !kept {
				return invalid("field %q refers to items of the types %s, not to %s of the type %s", name, strings.Join(f.Targets, ", "), target.Key, target.Type)
			}
		case KindFile:
			file := ID(props[name].(string))
			if self == nil {
				return invalid("field %q names a file of the item; add the file after creating the item", name)
			}
			if a, ok := self.attachment(file); !ok || (a.Deleted && self.Properties[name] != string(file)) {
				return notFound("field %q: file %s of item %s", name, file, self.Key)
			}
		}
	}
	return nil
}

// fieldRelID is the ID of the relation an item field derives.
func fieldRelID(from ID, field string, to ID) ID {
	return ID(fmt.Sprintf("%s/field/%s/%s", from, field, to))
}

// unrelateField drops the relation the old value of an item field derived.
func (s *Store) unrelateField(it *Item, field string, old any, touch func(...ID)) {
	if to, ok := old.(ID); ok {
		delete(s.rels, fieldRelID(it.ID, field, to))
		touch(to)
	}
}

// relateField derives the relation of an item field's value (D4): the
// field's name is its type, it is derived like a body's mentions, and it
// changes only with the property.
func (s *Store) relateField(it *Item, field string, v any, seq int, touch func(...ID)) {
	to, ok := v.(ID)
	if !ok || s.items[to] == nil {
		return
	}
	id := fieldRelID(it.ID, field, to)
	s.rels[id] = &Relation{ID: id, Type: field, From: it.ID, To: to, Derived: true, created: seq, field: field}
	touch(to)
}

// relateFields derives the relations of every item field of an item whose
// properties were all set at once, after dropping those of old.
func (s *Store) relateFields(it *Item, old Props, seq int, touch func(...ID)) {
	for _, name := range slices.Sorted(maps.Keys(old)) {
		s.unrelateField(it, name, old[name], touch)
	}
	for _, name := range slices.Sorted(maps.Keys(it.Properties)) {
		s.relateField(it, name, it.Properties[name], seq, touch)
	}
}

// Field names the item field a relation is derived from, or is empty for
// a relation made by hand or derived from a body (ADR-0202).
func (r Relation) Field() string { return r.field }

// --- sessions ---------------------------------------------------------------

// MayDefineTypes reports whether the principal may define types of a
// space: its administrators may, and the installation's (ADR-0201). For
// the empty space, only the installation's administrators may.
func (x *Session) MayDefineTypes(space string) bool {
	if space == "" {
		return x.s.hasSystemRole(x.who, RoleAdmin)
	}
	return x.s.HasSpace(space) && x.s.spaceRole(x.who, space) == SpaceAdmin
}

// DefineType defines a room type, or a new version of one, for its
// space's administrators (ADR-0201). A type of the whole installation is
// defined by the installation's administrators through the trusted
// command, as before (ADR-0028).
func (x *Session) DefineType(td TypeDef) (TypeDef, error) {
	if td.Space == "" {
		return TypeDef{}, invalid("a space's type names its space")
	}
	if !x.MayDefineTypes(td.Space) {
		if x.s.spaceRole(x.who, td.Space) < Viewer {
			return TypeDef{}, notFound("space %q", td.Space)
		}
		return TypeDef{}, forbidden("%s may not define the types of %s", x.who, td.Space)
	}
	if err := x.s.DefineType(x.who, td); err != nil {
		return TypeDef{}, err
	}
	return x.s.Type(td.Name), nil
}

// RemoveType takes a room type out of use, for its space's administrators,
// when nothing uses it (ADR-0094, ADR-0201). An installation type is
// removed by the installation's administrators through the trusted
// command.
func (x *Session) RemoveType(name string) error {
	td, ok := x.s.latest(name)
	if !ok || (td.Space != "" && x.s.spaceRole(x.who, td.Space) < Viewer) {
		return notFound("type %q", name)
	}
	if td.Space == "" {
		return forbidden("%s is a type of the whole installation; its administrators remove it", name)
	}
	if !x.MayDefineTypes(td.Space) {
		return forbidden("%s may not remove the types of %s", x.who, td.Space)
	}
	return x.s.RemoveType(x.who, name)
}

// Types lists the latest version of the types the principal sees, ordered
// by name: those of the whole installation and those of the spaces it
// holds a role in (ADR-0201). With a space, only the types items there may
// have: the installation's and that space's own.
func (x *Session) Types(space string) []TypeDef {
	var out []TypeDef
	for _, td := range x.s.Types() {
		if td.Space != "" && x.s.spaceRole(x.who, td.Space) < Viewer {
			continue
		}
		if space != "" && !td.UsableIn(space) {
			continue
		}
		out = append(out, td)
	}
	return out
}

// readableRefs checks that the item fields a principal sets refer to items
// it may read: one it may not read does not exist for it (D10). A value
// the item already holds may stay.
func (x *Session) readableRefs(self *Item, props Props) error {
	for _, name := range slices.Sorted(maps.Keys(props)) {
		to, ok := props[name].(ID)
		if !ok || (self != nil && self.Properties[name] == to) {
			continue
		}
		if _, err := x.readable(to); err != nil {
			return err
		}
	}
	return nil
}

// unreadableRefs names the item fields of it whose target the principal
// may not read; nil when there are none.
func (s *Store) unreadableRefs(principal string, props Props) []string {
	var out []string
	for name, v := range props {
		if to, ok := v.(ID); ok {
			if target := s.items[to]; target == nil || !s.canRead(principal, target) {
				out = append(out, name)
			}
		}
	}
	return out
}

// hideRefs empties the item fields whose target the principal may not
// read, in a copy: such a target is not revealed, not even by its ID
// (D10).
func (s *Store) hideRefs(principal string, props Props) Props {
	hide := s.unreadableRefs(principal, props)
	if hide == nil {
		return props
	}
	out := maps.Clone(props)
	for _, name := range hide {
		delete(out, name)
	}
	return out
}

// hideEventRefs is an event of an item's history as the principal reads
// it: the values of item fields that refer to an item it may not read are
// empty.
func (s *Store) hideEventRefs(principal string, e Event) Event {
	if to, ok := e.Value.(ID); ok {
		if target := s.items[to]; target == nil || !s.canRead(principal, target) {
			e.Value = nil
		}
	}
	if e.Props != nil {
		e.Props = s.hideRefs(principal, e.Props)
	}
	return e
}
