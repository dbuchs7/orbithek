package item

import (
	"slices"
	"time"
)

// A label's page and its order set by hand (ADR-0202). Labels are
// strings on their items (ADR-0044); a label may have a page that says
// what it is about: an item of a type with a body and a text field label
// that names the label, the ready-made type topic. The label's view shows
// the first such page the reader reads, by the order the items were made
// in, in every room.
//
// Whoever may edit a topic orders the items that carry its label by hand.
// The order is kept on the topic as the IDs of those items (D2), in the
// event ItemsOrdered, not sealed: it names items, not their content
// (ADR-0026), like the order of slides of ADR-0195. It is no reference
// (D8): an item it names may lose the label, be trashed or be purged, and
// is then passed over. A reader is told only the places of the items they
// read (D10); the others keep their places, unseen. Items the order does
// not name come after those it names, in the order they were made.

// TopicType names the ready-made type of a label's page; TopicLabelField
// is the field that names the label.
const (
	TopicType       = "topic"
	TopicLabelField = "label"
)

// topicType is the ready-made type of a label's page: a text, and the
// label it describes, one page per label in a room.
func topicType() TypeDef {
	return TypeDef{Name: TopicType, Body: true, Fields: []FieldDef{{Name: TopicLabelField, Kind: KindText, Required: true, Unique: true}}}
}

// isTopicType reports whether a type can hold a label's page: it has a
// body and a text field label.
func isTopicType(td TypeDef) bool {
	f, ok := td.field(TopicLabelField)
	return td.Body && ok && f.Kind == KindText
}

// topicLabel is the label a topic names, as stored (ADR-0044); ok is
// false for an item that is no topic or names no valid label.
func (s *Store) topicLabel(it *Item) (string, bool) {
	v := s.types[it.Type]
	if it.TypeVersion < 1 || it.TypeVersion > len(v) || !isTopicType(v[it.TypeVersion-1]) {
		return "", false
	}
	text, _ := it.Properties[TopicLabelField].(string)
	l, ok := NormalLabel(text)
	return l, ok && l != ""
}

// TopicOf returns the page of a label the principal reads: the live topic
// naming it that was made first, in any room; ok is false without one.
func (x *Session) TopicOf(label string) (Item, bool) {
	l, valid := NormalLabel(label)
	if !valid {
		return Item{}, false
	}
	var found *Item
	for _, it := range x.s.items {
		if it.Presence != Live || (found != nil && it.created > found.created) {
			continue
		}
		if tl, ok := x.s.topicLabel(it); ok && tl == l && x.s.canRead(x.who, it) {
			found = it
		}
	}
	if found == nil {
		return Item{}, false
	}
	return x.s.veiled(x.who, found).clone(), true
}

// DescribeLabel makes the page of a label in a space where the principal
// may create: an item of the type topic that names the label, titled
// title or the label. The type is defined from its template when no type
// of its name exists yet, as the album defines its collection's type
// (ADR-0175); a type of that name that cannot hold a label's page is not
// taken over.
func (x *Session) DescribeLabel(space, label, title string) (Item, error) {
	l, ok := NormalLabel(label)
	if !ok {
		return Item{}, invalid("label %q: lower-case letters and digits joined by -, _ or ., at most 64 (ADR-0044)", label)
	}
	if err := x.canCreateIn(space, ""); err != nil {
		return Item{}, err
	}
	if title == "" {
		title = l
	}
	// The type is not defined for a page that is refused.
	if err := x.s.refusesNew(); err != nil {
		return Item{}, err
	}
	td, ok := x.s.latest(TopicType)
	switch {
	case !ok:
		if err := x.s.DefineType(x.who, topicType()); err != nil {
			return Item{}, err
		}
	case !isTopicType(td):
		return Item{}, invalid("the type %s is in use for something else than the pages of labels", TopicType)
	}
	return x.Create(NewItem{Space: space, Type: TopicType, Title: title, Properties: Props{TopicLabelField: l}})
}

// topic returns a live topic the principal may read, with its label.
func (x *Session) topic(id ID) (*Item, string, error) {
	it, err := x.readable(id)
	if err != nil {
		return nil, "", err
	}
	if it.Presence != Live {
		return nil, "", invalid("%s is %s", it.Key, it.Presence)
	}
	l, ok := x.s.topicLabel(it)
	if !ok {
		return nil, "", invalid("%s is no page of a label", it.Key)
	}
	return it, l, nil
}

// carrying returns the live items other than the topic that carry its
// label, in the order they were made, whoever may read them; keep, if
// set, narrows them.
func (s *Store) carrying(topic *Item, label string, keep func(*Item) bool) []*Item {
	return s.matching(Query{Label: label}, func(it *Item) bool {
		return it.ID != topic.ID && (keep == nil || keep(it))
	})
}

// ItemOrder returns the items of a topic the principal reads in the order
// set by hand, as far as it names them; empty without an order. Items the
// principal does not read are left out (D10).
func (x *Session) ItemOrder(id ID) ([]ID, error) {
	t, l, err := x.topic(id)
	if err != nil {
		return nil, err
	}
	var out []ID
	seen := map[ID]bool{}
	for _, m := range t.ItemOrder {
		it := x.s.items[m]
		if seen[m] || it == nil || it.Presence != Live || it.ID == t.ID || !slices.Contains(it.Labels, l) || !x.s.canRead(x.who, it) {
			continue
		}
		seen[m] = true
		out = append(out, m)
	}
	return out, nil
}

// PlaceItems moves items of a topic the principal may edit to a place in
// its order set by hand: moved, in the order they are to have, go right
// before the item before names, or to the end when before is empty. Every
// item named must be one the principal reads that carries the topic's
// label.
//
// The order kept names every item in it at once: the items it named
// before, as far as they still carry the label, then those the principal
// reads and it did not name, in the order they were made; then the moved
// ones take their new place. Items the principal does not read keep
// theirs.
func (x *Session) PlaceItems(id ID, moved []ID, before ID) error {
	if _, err := x.editable(id); err != nil {
		return err
	}
	t, l, err := x.topic(id)
	if err != nil {
		return err
	}
	members := x.s.carrying(t, l, func(it *Item) bool { return x.s.canRead(x.who, it) })
	if len(moved) == 0 {
		return invalid("name the items to move")
	}
	if len(moved) > len(members) {
		return invalid("%d items carry the label %s; %d are named", len(members), l, len(moved))
	}
	seen := map[ID]bool{}
	for _, it := range members {
		seen[it.ID] = false
	}
	for _, m := range moved {
		used, ok := seen[m]
		switch {
		case !ok:
			return notFound("item %s with the label %s", m, l)
		case used:
			return invalid("item %s is named twice", m)
		}
		seen[m] = true
	}
	if before != "" {
		used, ok := seen[before]
		switch {
		case !ok:
			return notFound("item %s with the label %s", before, l)
		case used:
			return invalid("item %s cannot be placed before itself", before)
		}
	}
	// What the order named and no longer carries the label is forgotten,
	// whoever reads it.
	still := map[ID]bool{}
	for _, it := range x.s.carrying(t, l, nil) {
		still[it.ID] = true
	}
	var full []ID
	named := map[ID]bool{}
	for _, m := range t.ItemOrder {
		if still[m] && !named[m] {
			full = append(full, m)
			named[m] = true
		}
	}
	for _, it := range members {
		if !named[it.ID] {
			full = append(full, it.ID)
			named[it.ID] = true
		}
	}
	full = slices.DeleteFunc(full, func(m ID) bool { return seen[m] })
	at := len(full)
	if before != "" {
		at = slices.Index(full, before)
	}
	full = slices.Insert(full, at, moved...)
	if slices.Equal(full, t.ItemOrder) {
		return nil
	}
	return x.s.record(x.who, Event{Kind: ItemsOrdered, Item: t.ID, Order: full})
}

// ClearItemOrder forgets the order set by hand of a topic the principal
// may edit: its items are listed in the order they were made again.
func (x *Session) ClearItemOrder(id ID) error {
	if _, err := x.editable(id); err != nil {
		return err
	}
	t, _, err := x.topic(id)
	if err != nil {
		return err
	}
	if len(t.ItemOrder) == 0 {
		return nil
	}
	return x.s.record(x.who, Event{Kind: ItemsOrdered, Item: t.ID})
}

func (s *Store) applyItemOrder(e Event, touch func(...ID)) {
	it := s.items[e.Item]
	if it == nil {
		return
	}
	it.ItemOrder = slices.Clone(e.Order)
	touch(it.ID)
}

// LastChanges tells when each of the items the principal reads last
// changed itself (its content, state, place, files or comments), as the
// query language's field updated reads it (ADR-0136); items it does not
// read, or that do not exist, are left out (D10).
func (x *Session) LastChanges(ids []ID) map[ID]time.Time {
	out := map[ID]time.Time{}
	for _, id := range ids {
		it := x.s.items[id]
		if it == nil || !x.s.canRead(x.who, it) {
			continue
		}
		out[id] = x.s.lastChange(it).At
	}
	return out
}
