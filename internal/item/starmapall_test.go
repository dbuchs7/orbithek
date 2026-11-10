package item

import (
	"slices"
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
