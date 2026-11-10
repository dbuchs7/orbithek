package item

import (
	"cmp"
	"slices"
	"strings"
)

// The album's collections (ADR-0175). A collection is an item of the type
// collection, placed in a space or a folder of the album as a folder is.
// It holds no pictures of its own: its rule names labels, and the album
// gathers under it the pictures of every room whose labels match, as far
// as the reader may read them. The rule is a set of properties, written
// through the type's validated fields (D3, D4); nothing new is persisted.

// CollectionType names the type of the album's collections.
const CollectionType = "collection"

// The fields of a collection and the values of its enums.
const (
	fieldRule   = "rule"
	fieldMatch  = "match"
	fieldSource = "source"

	matchAny = "any"
	matchAll = "all"

	sourcePictures = "pictures"
	sourceItems    = "pictures and items"
)

// collectionType is the type the album defines when its first collection
// is made.
func collectionType() TypeDef {
	return TypeDef{Name: CollectionType, Fields: []FieldDef{
		{Name: fieldRule, Kind: KindText, Required: true},
		{Name: fieldMatch, Kind: KindEnum, Values: []string{matchAny, matchAll}, Required: true},
		{Name: fieldSource, Kind: KindEnum, Values: []string{sourcePictures, sourceItems}, Required: true},
	}}
}

// isCollectionType reports whether a type is the album's collection: no
// body and no workflow, the fields of a rule in their kinds, and no other
// field one must fill.
func isCollectionType(td TypeDef) bool {
	if td.Body || td.Workflow != nil {
		return false
	}
	for _, want := range collectionType().Fields {
		f, ok := td.field(want.Name)
		if !ok || f.Kind != want.Kind || !slices.Equal(f.Values, want.Values) {
			return false
		}
	}
	return !slices.ContainsFunc(td.Fields, func(f FieldDef) bool {
		_, ours := collectionType().field(f.Name)
		return f.Required && !ours
	})
}

// CollectionRule says which pictures a collection gathers.
type CollectionRule struct {
	// Labels are the labels the rule names, normalised, sorted and unique.
	Labels []string
	// All: a picture must carry every label; else any one of them.
	All bool
	// Items: the labels of a picture's item count as the picture's own.
	// Else only the picture's own labels (ADR-0090) count.
	Items bool
}

// Matches reports whether a picture falls under the rule. A rule without
// labels gathers nothing.
func (r CollectionRule) Matches(p Picture) bool {
	if len(r.Labels) == 0 {
		return false
	}
	has := func(l string) bool {
		return slices.Contains(p.Attachment.Labels, l) || (r.Items && slices.Contains(p.Item.Labels, l))
	}
	if r.All {
		return !slices.ContainsFunc(r.Labels, func(l string) bool { return !has(l) })
	}
	return slices.ContainsFunc(r.Labels, has)
}

// props is the rule as the collection's properties hold it.
func (r CollectionRule) props() Props {
	match, source := matchAny, sourcePictures
	if r.All {
		match = matchAll
	}
	if r.Items {
		source = sourceItems
	}
	return Props{fieldRule: strings.Join(r.Labels, ", "), fieldMatch: match, fieldSource: source}
}

// ruleOfProps reads a collection's rule from its properties. A label of
// the rule that is no label is passed over: the rule field is text, so
// it may have been typed on the item's own page.
func ruleOfProps(p Props) CollectionRule {
	var r CollectionRule
	text, _ := p[fieldRule].(string)
	for _, l := range strings.Split(text, ",") {
		if n, ok := NormalLabel(l); ok {
			r.Labels = append(r.Labels, n)
		}
	}
	slices.Sort(r.Labels)
	r.Labels = slices.Compact(r.Labels)
	r.All = p[fieldMatch] == matchAll
	r.Items = p[fieldSource] == sourceItems
	return r
}

// normal checks and normalises a rule given by a person: at least one
// label and at most as many as a picture may carry.
func (r CollectionRule) normal() (CollectionRule, error) {
	out := CollectionRule{All: r.All, Items: r.Items}
	for _, l := range r.Labels {
		n, ok := NormalLabel(l)
		if !ok {
			return CollectionRule{}, invalid("label %q: use letters and digits, joined by -, _ or ., up to 64 characters", l)
		}
		out.Labels = append(out.Labels, n)
	}
	slices.Sort(out.Labels)
	out.Labels = slices.Compact(out.Labels)
	switch {
	case len(out.Labels) == 0:
		return CollectionRule{}, invalid("a collection names at least one label")
	case len(out.Labels) > maxPictureLabels:
		return CollectionRule{}, invalid("a collection names at most %d labels", maxPictureLabels)
	}
	return out, nil
}

// CreateCollection makes a collection of the album in a space, or in a
// folder of it. Who may create there may.
func (x *Session) CreateCollection(space string, parent ID, name string, rule CollectionRule) (Item, error) {
	rule, err := rule.normal()
	if err != nil {
		return Item{}, err
	}
	if parent != "" {
		p, err := x.readable(parent)
		if err != nil {
			return Item{}, err
		}
		if p.Type != FolderType {
			return Item{}, invalid("%s is no folder of the album", p.Key)
		}
		space = p.Space
	}
	if err := x.canCreateIn(space, parent); err != nil {
		return Item{}, err
	}
	// The collection's type is not defined for a collection that is
	// refused.
	if err := x.s.refusesNew(); err != nil {
		return Item{}, err
	}
	switch td, ok := x.s.latest(CollectionType); {
	case !ok:
		if err := x.s.DefineType(x.who, collectionType()); err != nil {
			return Item{}, err
		}
	case !isCollectionType(td):
		return Item{}, invalid("the type %s is in use for something else than the album's collections", CollectionType)
	}
	return x.Create(NewItem{Space: space, Type: CollectionType, Title: name, Parent: parent, Properties: rule.props()})
}

// Collection returns a live collection the principal may read, with its
// rule.
func (x *Session) Collection(id ID) (Item, CollectionRule, error) {
	it, err := x.readable(id)
	if err != nil {
		return Item{}, CollectionRule{}, err
	}
	if it.Presence != Live || it.Type != CollectionType {
		return Item{}, CollectionRule{}, notFound("collection %s", id)
	}
	return Item{ID: it.ID, Key: it.Key, Title: it.Title, Space: it.Space, Type: it.Type, Parent: it.Parent}, ruleOfProps(it.Properties), nil
}

// SetCollectionRule replaces the rule of a collection the principal may
// edit. Each changed part is written as the property it is (D3).
func (x *Session) SetCollectionRule(id ID, rule CollectionRule) error {
	rule, err := rule.normal()
	if err != nil {
		return err
	}
	it, err := x.editable(id)
	if err != nil {
		return err
	}
	if it.Presence != Live || it.Type != CollectionType {
		return notFound("collection %s", id)
	}
	now := it.Properties
	want := rule.props()
	for _, f := range []string{fieldRule, fieldMatch, fieldSource} {
		if now[f] == want[f] {
			continue
		}
		if err := x.SetProperty(id, f, want[f]); err != nil {
			return err
		}
	}
	return nil
}

// AlbumCollection is a collection as the album lists it.
type AlbumCollection struct {
	Item     Item
	Rule     CollectionRule
	Editable bool
}

// Collections lists the album's collections the principal may read in a
// space (every space when empty) right below parent (the top when empty),
// by name.
func (x *Session) Collections(space string, parent ID) []AlbumCollection {
	var out []AlbumCollection
	for _, it := range x.s.items {
		if it.Presence != Live || it.Type != CollectionType || it.Parent != parent || (space != "" && it.Space != space) || !x.s.canRead(x.who, it) {
			continue
		}
		out = append(out, AlbumCollection{
			Item:     Item{ID: it.ID, Key: it.Key, Title: it.Title, Space: it.Space, Type: it.Type, Parent: it.Parent},
			Rule:     ruleOfProps(it.Properties),
			Editable: x.s.canEdit(x.who, it) && it.Managed == "",
		})
	}
	slices.SortFunc(out, func(a, b AlbumCollection) int {
		if c := cmp.Compare(strings.ToLower(a.Item.Title), strings.ToLower(b.Item.Title)); c != 0 {
			return c
		}
		return cmp.Compare(a.Item.Key, b.Item.Key)
	})
	return out
}
