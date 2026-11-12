package item

import (
	"slices"
	"strings"
	"testing"
)

// The starmap of all spaces shows the readable items of every space one
// reads, with their tree, relations and shared labels; with a label only
// the items that carry it (ADR-0191).
func TestTheStarmapOfAllSpaces(t *testing.T) {
	s := permFixture(t)
	noErr(t, s.GrantRole("root", "OPS", "anna", Editor))
	anna, ben := s.As("anna"), s.As("ben")
	plan := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Plan"}))
	child := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Detail", Parent: plan.ID}))
	task := mustItem(t)(anna.Create(NewItem{Space: "OPS", Type: "task", Title: "Patch", Properties: Props{"priority": "low"}}))
	secret := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Secret"}))
	noErr(t, s.Restrict("root", secret.ID, Restriction{Read: []string{"anna"}}))
	noErr(t, anna.AddRelation("depends-on", task.ID, plan.ID))
	noErr(t, anna.SetLabels(plan.ID, []string{"q4"}))
	noErr(t, anna.SetLabels(task.ID, []string{"q4"}))

	ids := func(m Starmap) []string {
		var out []string
		for _, n := range m.Nodes {
			out = append(out, n.ID)
		}
		return out
	}
	hasEdge := func(m Starmap, from, to ID, kind string) bool {
		return slices.ContainsFunc(m.Edges, func(e StarEdge) bool { return e.From == string(from) && e.To == string(to) && e.Kind == kind })
	}
	m := anna.StarmapAll("", 100)
	if m.Grouped || m.Items != 4 || !slices.Contains(ids(m), string(task.ID)) || !slices.Contains(ids(m), "label:q4") {
		t.Fatalf("anna's map %v grouped %v items %d", ids(m), m.Grouped, m.Items)
	}
	if !hasEdge(m, plan.ID, child.ID, StarParent) || !hasEdge(m, task.ID, plan.ID, StarRelation) {
		t.Errorf("edges %v", m.Edges)
	}
	// ben reads PLAT only, and not the restricted page.
	m = ben.StarmapAll("", 100)
	if got := ids(m); slices.Contains(got, string(task.ID)) || slices.Contains(got, string(secret.ID)) || m.Items != 2 {
		t.Errorf("ben's map %v (%d items)", got, m.Items)
	}
	// A label narrows the map to the items that carry it, in every space.
	m = anna.StarmapAll(" Q4 ", 100)
	if got := ids(m); len(got) != 3 || !slices.Contains(got, string(plan.ID)) || !slices.Contains(got, string(task.ID)) {
		t.Errorf("the map of a label %v", got)
	}

	// Beyond the budget, a node per space and type, as big as its items,
	// and an edge counting the links between them.
	m = anna.StarmapAll("", 2)
	if !m.Grouped || m.Items != 4 {
		t.Fatalf("grouped %v, items %d", m.Grouped, m.Items)
	}
	count := map[string]int{}
	for _, n := range m.Nodes {
		if n.Kind != StarGroup {
			t.Errorf("a node that is no group: %+v", n)
		}
		count[n.ID] = n.Count
	}
	if count["group:PLAT/page"] != 3 || count["group:OPS/task"] != 1 || len(count) != 2 {
		t.Errorf("groups %v", count)
	}
	if len(m.Edges) != 1 || m.Edges[0].Count != 1 || m.Edges[0].Kind != StarGroup {
		t.Errorf("edges between groups %+v", m.Edges)
	}
	// ben's groups count only what he reads.
	m = ben.StarmapAll("", 1)
	if len(m.Nodes) != 1 || m.Nodes[0].Count != 2 || m.Nodes[0].Space != "PLAT" {
		t.Errorf("ben's groups %+v", m.Nodes)
	}
	// Groups beyond the budget are left out, and the map says so.
	m = anna.StarmapAll("", 1)
	if !m.Grouped || !m.Truncated || len(m.Nodes) != 1 {
		t.Errorf("groups beyond the budget %+v", m)
	}
}

// The starmap of all spaces shows the pictures and drawings of its items,
// each joined to the item that holds it and a copy to its original: first
// those a text shows, with their originals, then the labels, then the
// rest; beyond the budget for items, in groups, none (ADR-0215).
func TestTheStarmapOfAllSpacesShowsPicturesWhereTheyAreUsed(t *testing.T) {
	s := permFixture(t)
	noErr(t, s.GrantRole("root", "OPS", "anna", Editor))
	anna, ben := s.As("anna"), s.As("ben")
	folder := mustItem(t)(anna.Create(NewItem{Space: "OPS", Type: "page", Title: "Photos"}))
	page := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Plan"}))
	task := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "task", Title: "Patch", Properties: Props{"priority": "low"}}))
	secret := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Secret"}))
	noErr(t, s.Restrict("root", secret.ID, Restriction{Read: []string{"anna"}}))
	noErr(t, anna.SetLabels(page.ID, []string{"q4"}))
	noErr(t, anna.SetLabels(task.ID, []string{"q4"}))

	sum := strings.Repeat("ab", 32)
	file := func(on, id ID, name, typ string, src *Source) {
		_, err := anna.AddAttachment(on, Attachment{ID: id, Name: name, Type: typ, Size: 3, SHA256: sum, Source: src})
		noErr(t, err)
	}
	const (
		rack     ID = "00000000000000000000000000000001" // a photo of the album folder
		rackCopy ID = "00000000000000000000000000000002" // its copy, shown in the page
		draw1    ID = "00000000000000000000000000000003" // a drawing of the task
		draw2    ID = "00000000000000000000000000000004" // its new version
		spare    ID = "00000000000000000000000000000005" // a photo no text shows
		hidden   ID = "00000000000000000000000000000006" // a photo of the restricted page
		notes    ID = "00000000000000000000000000000007" // no picture
	)
	file(folder.ID, rack, "rack.jpg", "image/jpeg", nil)
	file(page.ID, rackCopy, "rack.jpg", "image/jpeg", &Source{Item: folder.ID, Attachment: rack})
	file(task.ID, draw1, "sketch.excalidraw.png", "image/png", nil)
	file(task.ID, draw2, "sketch.excalidraw.png", "image/png", &Source{Item: task.ID, Attachment: draw1})
	file(folder.ID, spare, "spare.jpg", "image/jpeg", nil)
	file(secret.ID, hidden, "hidden.png", "image/png", nil)
	file(page.ID, notes, "notes.pdf", "application/pdf", nil)
	noErr(t, anna.CommitBody(page.ID, Doc{Blocks: []Block{{Kind: Image, Ref: rackCopy}}}))
	// The text shows the drawing's first version; its newest stands for it.
	noErr(t, anna.CommitBody(task.ID, Doc{Blocks: []Block{{Kind: Image, Ref: draw1}}}))

	pic := func(it Item, a ID) string { return pictureNodeID(it.ID, a) }
	has := func(m Starmap, from, to, kind string) bool {
		return slices.ContainsFunc(m.Edges, func(e StarEdge) bool { return e.From == from && e.To == to && e.Kind == kind })
	}
	m := anna.StarmapAll("", 100)
	want := []string{
		string(folder.ID), string(page.ID), string(task.ID), string(secret.ID), // the items, by space and as made
		pic(task, draw2), pic(page, rackCopy), pic(folder, rack), // shown in a text, newest first, each copy's original after it
		"label:q4",
		pic(secret, hidden), pic(folder, spare), // the rest, newest first
	}
	if got := starIDs(m); !slices.Equal(got, want) {
		t.Fatalf("nodes\n got %v\nwant %v", got, want)
	}
	if m.Items != 4 || m.Truncated || m.Grouped {
		t.Errorf("items %d, truncated %v, grouped %v", m.Items, m.Truncated, m.Grouped)
	}
	for _, e := range [][3]string{
		{pic(page, rackCopy), string(page.ID), StarOwner},
		{pic(folder, rack), string(folder.ID), StarOwner},
		{pic(page, rackCopy), pic(folder, rack), StarCopy},
		{pic(task, draw2), string(task.ID), StarOwner},
		{pic(folder, spare), string(folder.ID), StarOwner},
	} {
		if !has(m, e[0], e[1], e[2]) {
			t.Errorf("no edge %v in %v", e, m.Edges)
		}
	}
	for _, n := range m.Nodes {
		if n.ID == pic(task, draw2) && (n.Kind != StarPicture || n.Item != task.ID || n.Space != "PLAT" || n.Name != "sketch.excalidraw.png" || n.Outside) {
			t.Errorf("drawing %+v", n)
		}
	}

	// The budget left by the items is filled in that order.
	m = anna.StarmapAll("", 7)
	if got := starIDs(m); !slices.Equal(got, want[:7]) || !m.Truncated || m.Grouped || !has(m, pic(page, rackCopy), pic(folder, rack), StarCopy) {
		t.Errorf("within seven nodes %v, truncated %v", got, m.Truncated)
	}
	// A copy whose original the budget left out has no copy edge.
	m = anna.StarmapAll("", 6)
	if got := starIDs(m); !slices.Equal(got, want[:6]) || slices.ContainsFunc(m.Edges, func(e StarEdge) bool { return e.Kind == StarCopy }) {
		t.Errorf("within six nodes %v, edges %v", got, m.Edges)
	}

	// ben reads PLAT only, and not the restricted page: neither the
	// original in OPS nor the restricted page's photo, not even as an edge.
	m = ben.StarmapAll("", 100)
	if got := starIDs(m); !slices.Equal(got, []string{string(page.ID), string(task.ID), pic(task, draw2), pic(page, rackCopy), "label:q4"}) {
		t.Errorf("ben's map %v", got)
	}
	if slices.ContainsFunc(m.Edges, func(e StarEdge) bool { return e.Kind == StarCopy }) {
		t.Errorf("ben sees the copy's origin: %v", m.Edges)
	}

	// A label narrows the pictures to those of the items carrying it; an
	// original whose item does not carry it stays off the map.
	m = anna.StarmapAll("q4", 100)
	if got := starIDs(m); !slices.Equal(got, []string{string(page.ID), string(task.ID), pic(task, draw2), pic(page, rackCopy), "label:q4"}) {
		t.Errorf("the map of a label %v", got)
	}

	// In groups, no pictures.
	m = anna.StarmapAll("", 3)
	if !m.Grouped || slices.ContainsFunc(m.Nodes, func(n StarNode) bool { return n.Kind != StarGroup }) {
		t.Errorf("groups %+v", m.Nodes)
	}
}

// On the starmap of all spaces a picture's own labels (ADR-0090) count
// as an item's do: a label shows the pictures that carry it with the
// items that hold them, and the label stars join pictures too
// (ADR-draft-album-labels-on-the-starmap).
func TestTheStarmapOfAllSpacesFollowsAlbumLabels(t *testing.T) {
	s := permFixture(t)
	noErr(t, s.GrantRole("root", "OPS", "anna", Editor))
	noErr(t, s.GrantRole("root", "OPS", "ben", Viewer))
	anna, ben := s.As("anna"), s.As("ben")
	folder := mustItem(t)(anna.Create(NewItem{Space: "OPS", Type: "page", Title: "Photos"}))
	page := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Plan"}))
	task := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "task", Title: "Patch", Properties: Props{"priority": "low"}}))
	other := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Other"}))
	secret := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Secret"}))
	noErr(t, s.Restrict("root", secret.ID, Restriction{Read: []string{"anna"}}))
	noErr(t, anna.SetLabels(page.ID, []string{"q4"}))

	sum := strings.Repeat("ab", 32)
	file := func(on, id ID, name string, src *Source) {
		_, err := anna.AddAttachment(on, Attachment{ID: id, Name: name, Type: "image/png", Size: 3, SHA256: sum, Source: src})
		noErr(t, err)
	}
	const (
		tagged   ID = "00000000000000000000000000000001" // a photo of the folder labelled q4
		untagged ID = "00000000000000000000000000000002" // one without labels
		onPage   ID = "00000000000000000000000000000003" // a picture of the labelled page
		draw1    ID = "00000000000000000000000000000004" // a drawing labelled q4 on its first version
		draw2    ID = "00000000000000000000000000000005" // its new version, without
		elsewise ID = "00000000000000000000000000000006" // a picture of another page
		hidden   ID = "00000000000000000000000000000007" // a photo labelled q4 on the restricted page
	)
	file(folder.ID, tagged, "a.png", nil)
	file(folder.ID, untagged, "b.png", nil)
	file(page.ID, onPage, "c.png", nil)
	file(task.ID, draw1, "d.png", nil)
	noErr(t, anna.SetPictureLabels(task.ID, draw1, []string{"q4"}))
	file(task.ID, draw2, "d.png", &Source{Item: task.ID, Attachment: draw1})
	file(other.ID, elsewise, "e.png", nil)
	file(secret.ID, hidden, "f.png", nil)
	noErr(t, anna.SetPictureLabels(folder.ID, tagged, []string{"Q4"}))
	noErr(t, anna.SetPictureLabels(secret.ID, hidden, []string{"q4"}))

	pic := func(it Item, a ID) string { return pictureNodeID(it.ID, a) }
	has := func(m Starmap, from, to, kind string) bool {
		return slices.ContainsFunc(m.Edges, func(e StarEdge) bool { return e.From == from && e.To == to && e.Kind == kind })
	}

	// A label brings in the pictures that carry it, with the items that
	// hold them, and every picture of an item that carries it; no other.
	m := anna.StarmapAll("q4", 100)
	want := []string{
		string(folder.ID), string(page.ID), string(task.ID), string(secret.ID), // carriers and holders, by space and as made
		"label:q4",
		pic(secret, hidden), pic(task, draw2), pic(page, onPage), pic(folder, tagged), // newest first
	}
	if got := starIDs(m); !slices.Equal(got, want) || m.Items != 4 || m.Grouped {
		t.Fatalf("the map of a label\n got %v (%d items)\nwant %v", got, m.Items, want)
	}
	for _, e := range [][3]string{
		{pic(folder, tagged), string(folder.ID), StarOwner},
		{pic(task, draw2), string(task.ID), StarOwner},
		{"label:q4", pic(folder, tagged), StarLabelled},
		{"label:q4", pic(task, draw2), StarLabelled}, // a label of any version is the picture's
		{"label:q4", string(page.ID), StarLabelled},
	} {
		if !has(m, e[0], e[1], e[2]) {
			t.Errorf("no edge %v in %v", e, m.Edges)
		}
	}
	if has(m, "label:q4", string(folder.ID), StarLabelled) {
		t.Errorf("the folder is labelled by its picture: %v", m.Edges)
	}
	// ben reads neither the restricted page nor its picture.
	m = ben.StarmapAll("q4", 100)
	if got := starIDs(m); slices.Contains(got, string(secret.ID)) || slices.Contains(got, pic(secret, hidden)) || m.Items != 3 {
		t.Errorf("ben's map of a label %v (%d items)", got, m.Items)
	}
	// Beyond the budget, the holders count in their groups.
	m = anna.StarmapAll("q4", 3)
	count := map[string]int{}
	for _, n := range m.Nodes {
		count[n.ID] = n.Count
	}
	if !m.Grouped || count["group:OPS/page"] != 1 || count["group:PLAT/page"] != 2 || count["group:PLAT/task"] != 1 {
		t.Errorf("groups of a label %v", count)
	}

	// Without a label the star joins the pictures that carry it.
	m = anna.StarmapAll("", 100)
	if !has(m, "label:q4", pic(folder, tagged), StarLabelled) || !has(m, "label:q4", string(page.ID), StarLabelled) || !slices.Contains(starIDs(m), pic(other, elsewise)) {
		t.Errorf("the map without a label %v, edges %v", starIDs(m), m.Edges)
	}
	// A star whose carriers the budget left out but one is dropped.
	m = anna.StarmapAll("", 6)
	if got := starIDs(m); slices.Contains(got, "label:q4") || !m.Truncated {
		t.Errorf("a star with one carrier on the map: %v", got)
	}
}
