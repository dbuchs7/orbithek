package item

import (
	"errors"
	"testing"
)

// trashKeys lists the keys of a space's trash as a principal reads it.
func trashKeys(t *testing.T, x *Session, space string) []string {
	t.Helper()
	entries, err := x.TrashOf(space)
	noErr(t, err)
	keys := []string{}
	for _, e := range entries {
		keys = append(keys, e.Item.Key)
	}
	return keys
}

func TestTheTrashOfASpaceListsTrashRootsNewestFirst(t *testing.T) {
	s := permFixture(t)
	anna := s.As("anna")
	guide := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Guide"}))
	mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Chapter", Parent: guide.ID}))
	note := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Note"}))
	live := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Live"}))
	noErr(t, anna.Trash(guide.ID))
	noErr(t, anna.Trash(note.ID))

	// The chapter went with the guide: it is no root of its own.
	entries, err := anna.TrashOf("PLAT")
	noErr(t, err)
	if len(entries) != 2 || entries[0].Item.ID != note.ID || entries[1].Item.ID != guide.ID {
		t.Fatalf("trash = %v, want [%s %s]", trashKeys(t, anna, "PLAT"), note.Key, guide.Key)
	}
	g := entries[1]
	if g.Items != 2 || g.By != "anna" || g.At.IsZero() || g.Item.Presence != Trashed {
		t.Fatalf("guide's entry = %+v, want 2 items trashed by anna", g)
	}
	if !g.MayRestore || g.MayPurge {
		t.Fatalf("editor: restore %v, purge %v; want true, false", g.MayRestore, g.MayPurge)
	}
	if e, _ := s.As("pat").TrashOf("PLAT"); !e[1].MayRestore || !e[1].MayPurge {
		t.Fatalf("space admin: restore %v, purge %v; want true, true", e[1].MayRestore, e[1].MayPurge)
	}
	if e, _ := s.As("ben").TrashOf("PLAT"); len(e) != 2 || e[1].MayRestore || e[1].MayPurge {
		t.Fatalf("viewer: %+v, want both roots, neither restorable nor purgeable", e)
	}
	for _, k := range trashKeys(t, anna, "PLAT") {
		if k == live.Key {
			t.Fatalf("a live item is in the trash")
		}
	}

	// Restored and purged items leave the trash.
	noErr(t, anna.Restore(guide.ID))
	noErr(t, s.As("pat").Purge(note.ID))
	if got := trashKeys(t, anna, "PLAT"); len(got) != 0 {
		t.Fatalf("trash after restore and purge = %v, want none", got)
	}
}

func TestTheTrashShowsOnlyWhatOneReads(t *testing.T) {
	s := permFixture(t)
	anna := s.As("anna")
	open := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Open"}))
	hidden := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Hidden", Parent: open.ID}))
	noErr(t, anna.Restrict(hidden.ID, Restriction{Read: []string{"anna"}}))
	private := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Private"}))
	noErr(t, anna.Restrict(private.ID, Restriction{Read: []string{"anna"}}))
	noErr(t, anna.Trash(open.ID))
	noErr(t, anna.Trash(private.ID))

	// ben reads the open page but neither the page below it nor the
	// private one: they are not counted, not listed (D10).
	entries, err := s.As("ben").TrashOf("PLAT")
	noErr(t, err)
	if len(entries) != 1 || entries[0].Item.ID != open.ID || entries[0].Items != 1 {
		t.Fatalf("viewer's trash = %+v, want only %s counting 1 item", entries, open.Key)
	}
	if e, _ := anna.TrashOf("PLAT"); len(e) != 2 || e[1].Items != 2 {
		t.Fatalf("author's trash = %+v, want both roots, the open page counting 2", e)
	}
	// An outsider sees an empty trash; an unknown space is not found.
	if got := trashKeys(t, s.As("carl"), "PLAT"); len(got) != 0 {
		t.Fatalf("outsider's trash = %v, want none", got)
	}
	if _, err := anna.TrashOf("NOPE"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown space: err = %v, want ErrNotFound", err)
	}
}

func TestATrashRootUnderATrashedParentWaitsForIt(t *testing.T) {
	s := permFixture(t)
	anna := s.As("anna")
	parent := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Parent"}))
	child := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Child", Parent: parent.ID}))
	noErr(t, anna.Trash(child.ID))
	noErr(t, anna.Trash(parent.ID))

	entries, err := anna.TrashOf("PLAT")
	noErr(t, err)
	if len(entries) != 2 || entries[1].Item.ID != child.ID {
		t.Fatalf("trash = %v, want the parent, then the child", trashKeys(t, anna, "PLAT"))
	}
	if entries[1].Waiting != parent.Key || entries[0].Waiting != "" {
		t.Fatalf("waiting = %q and %q, want %q for the child only", entries[0].Waiting, entries[1].Waiting, parent.Key)
	}
	noErr(t, anna.Restore(parent.ID))
	if e, _ := anna.TrashOf("PLAT"); len(e) != 1 || e[0].Waiting != "" {
		t.Fatalf("after the parent's restore: %+v, want the child waiting for nothing", e)
	}
}
