package item

import (
	"errors"
	"testing"
)

// TestTheTrashOfASpaceListsWhatTheTrashStartedFrom: a space's trash lists
// the items the trash started from, not what went with them, to those who
// may read them; restoring takes an entry out again (ADR-0021).
func TestTheTrashOfASpaceListsWhatTheTrashStartedFrom(t *testing.T) {
	s := setup(t)
	noErr(t, s.GrantRole("admin", "PLAT", "anna", Editor))
	noErr(t, s.GrantRole("admin", "PLAT", "vera", Viewer))
	noErr(t, s.GrantRole("admin", "PLAT", "ada", SpaceAdmin))
	anna := s.As("anna")
	guide := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Guide"}))
	step := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Step", Parent: guide.ID}))
	alone := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Alone"}))
	secret := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Secret"}))
	noErr(t, anna.Restrict(secret.ID, Restriction{Read: []string{"anna"}, Edit: []string{"anna"}}))
	mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Stays"}))
	noErr(t, anna.Trash(alone.ID))
	noErr(t, anna.Trash(guide.ID)) // takes Step along
	noErr(t, anna.Trash(secret.ID))

	titles := func(x *Session) []string {
		t.Helper()
		list, err := x.TrashOf("PLAT")
		noErr(t, err)
		var out []string
		for _, e := range list {
			out = append(out, e.Item.Title)
		}
		return out
	}
	list, err := anna.TrashOf("PLAT")
	noErr(t, err)
	if len(list) != 3 {
		t.Fatalf("anna's trash: %+v", list)
	}
	for _, e := range list {
		want := 0
		if e.Item.ID == guide.ID {
			want = 1
		}
		if e.Below != want || e.By != "anna" || e.At.IsZero() || !e.MayRestore || e.MayPurge || e.Item.ID == step.ID {
			t.Errorf("entry %+v, want %d below, by anna, restorable, not purgeable", e, want)
		}
	}
	// What one may not read is not listed, not even counted.
	if got := titles(s.As("vera")); len(got) != 2 {
		t.Errorf("vera's trash: %v, want Guide and Alone", got)
	}
	if list, _ := s.As("vera").TrashOf("PLAT"); list[0].MayRestore || list[0].MayPurge {
		t.Errorf("a viewer restores and purges nothing: %+v", list[0])
	}
	if list, _ := s.As("ada").TrashOf("PLAT"); len(list) != 3 || !list[0].MayPurge {
		t.Errorf("a space administrator purges: %+v", list)
	}
	// No role in the space, or no such space: the same answer.
	_, noRole := s.As("zoe").TrashOf("PLAT")
	_, noSpace := s.As("zoe").TrashOf("NONE")
	if !errors.Is(noRole, ErrNotFound) || !errors.Is(noSpace, ErrNotFound) {
		t.Errorf("no role: %v; no space: %v", noRole, noSpace)
	}
	if _, err := anna.TrashOf("OPS"); !errors.Is(err, ErrNotFound) {
		t.Errorf("a space anna holds no role in: %v", err)
	}
	noErr(t, anna.Restore(guide.ID))
	if got := titles(anna); len(got) != 2 {
		t.Errorf("after restoring Guide: %v", got)
	}
	// Replayed from the log, the trash is the same.
	r, err := Replay(s.Log())
	noErr(t, err)
	if got := titles(r.As("anna")); len(got) != 2 {
		t.Errorf("after replay: %v", got)
	}
}
