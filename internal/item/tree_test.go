package item

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func titles(items []Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Title
	}
	return out
}

func nodeTitles(nodes []TreeNode) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n.Item.Title
	}
	return out
}

func TestSiblingsKeepThePlacedOrder(t *testing.T) {
	s := setup(t)
	home := mustItem(t)(s.Create("anna", NewItem{Space: "PLAT", Type: "page", Title: "Home"}))
	a := mustItem(t)(s.Create("anna", NewItem{Space: "PLAT", Type: "page", Title: "A", Parent: home.ID}))
	b := mustItem(t)(s.Create("anna", NewItem{Space: "PLAT", Type: "page", Title: "B", Parent: home.ID}))
	c := mustItem(t)(s.Create("anna", NewItem{Space: "PLAT", Type: "page", Title: "C", Parent: home.ID}))
	if got := titles(s.Children("PLAT", home.ID)); !slices.Equal(got, []string{"A", "B", "C"}) {
		t.Fatalf("unplaced children %v, want oldest first", got)
	}
	noErr(t, s.Place("anna", c.ID, "PLAT", home.ID, a.ID))
	if got := titles(s.Children("PLAT", home.ID)); !slices.Equal(got, []string{"C", "A", "B"}) {
		t.Fatalf("after placing C before A: %v", got)
	}
	noErr(t, s.Place("anna", c.ID, "PLAT", home.ID, ""))
	if got := titles(s.Children("PLAT", home.ID)); !slices.Equal(got, []string{"A", "B", "C"}) {
		t.Fatalf("after placing C last: %v", got)
	}
	// A child created later follows the placed ones.
	mustItem(t)(s.Create("anna", NewItem{Space: "PLAT", Type: "page", Title: "D", Parent: home.ID}))
	noErr(t, s.Place("anna", b.ID, "PLAT", "", home.ID)) // B to the top level, before Home
	if got := titles(s.Children("PLAT", home.ID)); !slices.Equal(got, []string{"A", "C", "D"}) {
		t.Fatalf("children after B left: %v", got)
	}
	if got := titles(s.Children("PLAT", "")); !slices.Equal(got, []string{"B", "Home"}) {
		t.Fatalf("top level %v", got)
	}
	// Trashed children are not listed; the order survives a replay.
	noErr(t, s.Trash("anna", a.ID))
	again, err := Replay(s.Log())
	noErr(t, err)
	for _, st := range []*Store{s, again} {
		if got := titles(st.Children("PLAT", home.ID)); !slices.Equal(got, []string{"C", "D"}) {
			t.Fatalf("children %v", got)
		}
	}
	// Placing before something that is not a sibling, or below itself, fails.
	if err := s.Place("anna", c.ID, "PLAT", home.ID, b.ID); !errors.Is(err, ErrValidation) {
		t.Errorf("before a non-sibling: %v", err)
	}
	if err := s.Place("anna", c.ID, "PLAT", home.ID, c.ID); !errors.Is(err, ErrValidation) {
		t.Errorf("before itself: %v", err)
	}
	if err := s.Place("anna", home.ID, "PLAT", c.ID, ""); !errors.Is(err, ErrValidation) {
		t.Errorf("below itself: %v", err)
	}
}

func TestTreeShowsOnlyWhatOneMayRead(t *testing.T) {
	s := permFixture(t)
	anna, ben, carl := s.As("anna"), s.As("ben"), s.As("carl")
	home := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Home"}))
	open := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Open", Parent: home.ID}))
	secret := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Secret", Parent: home.ID}))
	mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Below secret", Parent: secret.ID}))
	last := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Last", Parent: home.ID}))
	noErr(t, anna.Restrict(secret.ID, Restriction{Read: []string{"anna"}}))

	top, err := ben.Tree("PLAT", "")
	noErr(t, err)
	if len(top) != 1 || top[0].Item.ID != home.ID || !top[0].HasChildren {
		t.Fatalf("top %+v", top)
	}
	kids, err := ben.Tree("PLAT", home.ID)
	noErr(t, err)
	if got := nodeTitles(kids); !slices.Equal(got, []string{"Open", "Last"}) || kids[0].HasChildren {
		t.Fatalf("ben sees %v", got)
	}
	annas, err := anna.Tree("PLAT", home.ID)
	noErr(t, err)
	if got := nodeTitles(annas); !slices.Equal(got, []string{"Open", "Secret", "Last"}) || !annas[1].HasChildren {
		t.Fatalf("anna sees %+v", annas)
	}
	// Ben moves Last before Open; the hidden Secret keeps its place after Open.
	if err := ben.Place(last.ID, "PLAT", home.ID, open.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("a viewer places: %v", err)
	}
	noErr(t, anna.Place(last.ID, "PLAT", home.ID, open.ID))
	annas, _ = anna.Tree("PLAT", home.ID)
	if got := nodeTitles(annas); !slices.Equal(got, []string{"Last", "Open", "Secret"}) {
		t.Fatalf("after placing %v", got)
	}
	// An outsider learns nothing, and one cannot place before an unreadable item.
	if nodes, err := carl.Tree("PLAT", ""); err != nil || len(nodes) != 0 {
		t.Fatalf("outsider %+v, %v", nodes, err)
	}
	if _, err := carl.Tree("PLAT", home.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("outsider below home: %v", err)
	}
	if _, err := anna.Tree("NOPE", ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown space: %v", err)
	}
	noErr(t, s.GrantRole("root", "PLAT", "ben", Editor))
	if err := ben.Place(last.ID, "PLAT", home.ID, secret.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("placing before an unreadable sibling: %v", err)
	}
}

// childTitles writes a list of child pages as titles, a page's children
// in brackets after it and "+" for a page with more than the list shows.
func childTitles(pages []ChildPage) string {
	var b strings.Builder
	for i, p := range pages {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(p.Item.Title)
		if p.More {
			b.WriteString("+")
		}
		if len(p.Children) > 0 {
			b.WriteString("[" + childTitles(p.Children) + "]")
		}
	}
	return b.String()
}

func TestChildPagesListToTheChosenDepth(t *testing.T) {
	s := permFixture(t)
	anna, ben, carl := s.As("anna"), s.As("ben"), s.As("carl")
	page := func(title string, parent ID) Item {
		return mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: title, Parent: parent}))
	}
	home := page("Home", "")
	a := page("A", home.ID)
	a1 := page("A1", a.ID)
	page("A1a", a1.ID)
	page("A2", a.ID)
	b := page("B", home.ID)
	secret := page("Secret", b.ID)
	page("Below secret", secret.ID)
	noErr(t, anna.Restrict(secret.ID, Restriction{Read: []string{"anna"}}))

	cases := []struct {
		who   *Session
		depth int
		want  string
	}{
		{anna, 0, "A+ B+"},
		{anna, 1, "A+ B+"},
		{anna, 2, "A[A1+ A2] B[Secret+]"},
		{anna, 3, "A[A1[A1a] A2] B[Secret[Below secret]]"},
		{anna, ChildrenAll, "A[A1[A1a] A2] B[Secret[Below secret]]"},
		// Ben does not read Secret, nor what lies below it, and is not
		// told that B has more.
		{ben, ChildrenAll, "A[A1[A1a] A2] B"},
	}
	for _, c := range cases {
		got, cut, err := c.who.ChildPages("PLAT", home.ID, c.depth, 100)
		noErr(t, err)
		if s := childTitles(got); s != c.want || cut {
			t.Errorf("%s at depth %d: %q (cut %v), want %q", c.who.who, c.depth, s, cut, c.want)
		}
	}
	// The top of a space is a parent like any other.
	top, _, err := anna.ChildPages("PLAT", "", 2, 100)
	noErr(t, err)
	if got := childTitles(top); got != "Home[A+ B+]" {
		t.Errorf("top of the space: %q", got)
	}
	// The budget cuts the deepest level first and says so.
	got, cut, err := anna.ChildPages("PLAT", home.ID, ChildrenAll, 5)
	noErr(t, err)
	if s := childTitles(got); s != "A[A1+ A2] B[Secret+]" || !cut {
		t.Errorf("within 5 pages: %q (cut %v)", s, cut)
	}
	got, cut, _ = anna.ChildPages("PLAT", home.ID, ChildrenAll, 3)
	if s := childTitles(got); s != "A+[A1+] B+" || !cut {
		t.Errorf("within 3 pages: %q (cut %v)", s, cut)
	}
	// An outsider learns nothing of a page they may not read.
	if _, _, err := carl.ChildPages("PLAT", home.ID, 2, 100); !errors.Is(err, ErrNotFound) {
		t.Errorf("outsider: %v", err)
	}
	if _, _, err := anna.ChildPages("NOPE", "", 2, 100); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown space: %v", err)
	}
}
