package item

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

// StarGroup is the kind of a node that stands for the items of one type in
// one space, on the starmap of all spaces when they are too many to show
// one by one (ADR-0191).
const StarGroup = "group"

// StarmapAll returns the starmap of every space the principal reads
// (ADR-0191): their readable live items, with containment, relations and
// the labels at least two of them carry, as a space's starmap shows them
// (ADR-0157). The pictures and drawings of those items fill what the
// budget leaves, each joined to its item and a copy to its original
// (ADR-0214): first those a text shows, each copy's original after it,
// then the labels, then the other pictures. With label, only the items
// that carry it, and their pictures, are on the map. When the items are
// more than maxNodes, the map shows a node per space and type instead
// (StarGroup, Count the items it stands for), no pictures, and an edge
// between two such nodes for the containment and relations between their
// items (Count how many); Grouped is set then. Nothing on it names or
// counts an item or a file the principal may not read (D10).
func (x *Session) StarmapAll(label string, maxNodes int) Starmap {
	label = strings.ToLower(strings.TrimSpace(label))
	var items []*Item
	for _, it := range x.s.items {
		if !x.liveReadable(it) {
			continue
		}
		if label != "" && !slices.Contains(it.Labels, label) {
			continue
		}
		items = append(items, it)
	}
	slices.SortFunc(items, func(a, b *Item) int {
		return cmp.Or(strings.Compare(a.Space, b.Space), cmp.Compare(a.created, b.created))
	})
	in := map[ID]*Item{}
	for _, it := range items {
		in[it.ID] = it
	}
	// The links between the map's items: a child below its parent, and
	// every relation the principal sees.
	type link struct {
		from, to *Item
		edge     StarEdge
	}
	var links []link
	for _, it := range items {
		if p := in[it.Parent]; p != nil {
			links = append(links, link{p, it, StarEdge{From: string(p.ID), To: string(it.ID), Kind: StarParent}})
		}
	}
	for _, r := range x.visibleRelations() {
		if from, to := in[r.From], in[r.To]; from != nil && to != nil {
			links = append(links, link{from, to, relationEdge(r)})
		}
	}

	if len(items) <= maxNodes {
		b := starBuilder{max: maxNodes, kept: map[string]bool{}}
		for _, it := range items {
			b.add(itemNode(it, false))
		}
		pictures := x.allStarPictures(in)
		for _, p := range pictures {
			if p.used {
				b.add(p.node)
				if p.src != nil {
					b.add(pictureNodeFor(p.src, p.srcTop))
				}
			}
		}
		carriers := map[string][]string{}
		for _, it := range items {
			for _, l := range it.Labels {
				carriers[l] = append(carriers[l], string(it.ID))
			}
		}
		var labels []string
		for l, ids := range carriers {
			if len(ids) >= 2 {
				labels = append(labels, l)
			}
		}
		slices.SortFunc(labels, func(a, b string) int {
			return cmp.Or(cmp.Compare(len(carriers[b]), len(carriers[a])), strings.Compare(a, b))
		})
		for _, l := range labels {
			id := "label:" + l
			b.add(StarNode{ID: id, Kind: StarLabel, Name: l})
			for _, it := range carriers[l] {
				b.edge(StarEdge{From: id, To: it, Kind: StarLabelled})
			}
		}
		for _, p := range pictures {
			b.add(p.node)
		}
		for _, l := range links {
			b.edge(l.edge)
		}
		for _, p := range pictures {
			b.edge(p.ownerEdge())
			if e, ok := p.copyEdge(); ok {
				b.edge(e)
			}
		}
		return Starmap{Nodes: b.nodes, Edges: b.edges, Truncated: b.truncated, Items: len(items)}
	}

	// Too many to show one by one: a node per space and type, as big as
	// the number of its items.
	groupID := func(it *Item) string { return "group:" + it.Space + "/" + it.Type }
	count := map[string]int{}
	var order []*Item // the first item of each group, in the items' order
	for _, it := range items {
		id := groupID(it)
		if count[id] == 0 {
			order = append(order, it)
		}
		count[id]++
	}
	b := starBuilder{max: maxNodes, kept: map[string]bool{}}
	for _, it := range order {
		id := groupID(it)
		name := it.Space
		if sp := x.s.spaces[it.Space]; sp != nil && sp.name != "" {
			name = sp.name
		}
		b.add(StarNode{ID: id, Kind: StarGroup, Space: it.Space, Type: it.Type, Title: name, Name: fmt.Sprintf("%s · %s", it.Space, it.Type), Count: count[id]})
	}
	// An edge between two groups counts the links between their items,
	// either way; links within a group are not drawn.
	type pair struct{ a, b string }
	between := map[pair]int{}
	var pairs []pair
	for _, l := range links {
		a, c := groupID(l.from), groupID(l.to)
		if a == c {
			continue
		}
		if a > c {
			a, c = c, a
		}
		k := pair{a, c}
		if between[k] == 0 {
			pairs = append(pairs, k)
		}
		between[k]++
	}
	for _, k := range pairs {
		b.edge(StarEdge{From: k.a, To: k.b, Kind: StarGroup, Count: between[k]})
	}
	return Starmap{Nodes: b.nodes, Edges: b.edges, Truncated: b.truncated, Grouped: true, Items: len(items)}
}

// allStarPictures are the picture nodes of the items on the starmap of all
// spaces (ADR-0214), newest first; the original of a copy is kept only
// when its item is on the map too.
func (x *Session) allStarPictures(on map[ID]*Item) []starPicture {
	var out []starPicture
	for _, p := range x.starPictures("") {
		if on[p.node.Item] == nil {
			continue
		}
		if p.src != nil && on[p.src.ID] == nil {
			p.src, p.srcTop = nil, nil
		}
		out = append(out, p)
	}
	return out
}

// pictureNodeFor is the node of the newest version of a file of an item.
func pictureNodeFor(it *Item, top *Attachment) StarNode {
	return StarNode{ID: pictureNodeID(it.ID, top.ID), Kind: StarPicture, Space: it.Space, Item: it.ID, Name: top.Name}
}
