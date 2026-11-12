package item

import (
	"errors"
	"testing"
)

func notesOrigin(id string) Origin {
	return Origin{System: ImportSystemPrefix + "notes", Instance: "PLAT/vault", ExternalID: id, Raw: "# A"}
}

// A room administrator imports pages through their session (ADR-0211):
// others may not, a page imported before is updated where people moved
// it, and an import never takes over an item of another source.
func TestImportAsARoomAdministrator(t *testing.T) {
	s := permFixture(t)
	pat := s.As("pat")
	u := Upsert{Origin: notesOrigin("md/a.md"), Space: "PLAT", Type: "page", Title: "A", Key: "PLAT-90", Managed: "x", Linked: "x", ID: "x"}

	if _, _, err := s.As("anna").Import(u); !errors.Is(err, ErrForbidden) {
		t.Fatalf("an editor imports: %v", err)
	}
	if _, _, err := s.As("carl").Import(u); !errors.Is(err, ErrForbidden) {
		t.Fatalf("an outsider imports: %v", err)
	}
	if _, _, err := pat.Import(Upsert{Origin: Origin{System: "opendata.swiss", Instance: "x", ExternalID: "y"}, Space: "PLAT", Type: "page", Title: "A"}); !errors.Is(err, ErrValidation) {
		t.Fatalf("another source's origin: %v", err)
	}
	page, changed, err := pat.Import(u)
	if err != nil || !changed {
		t.Fatalf("import: %v %v", changed, err)
	}
	if page.Space != "PLAT" || page.Managed != "" || page.Linked != "" || page.Key == "PLAT-90" || page.Origin.Raw != "# A" {
		t.Fatalf("imported %+v", page)
	}
	// Below a page of its own import.
	child, _, err := pat.Import(Upsert{Origin: notesOrigin("md/a/b.md"), Space: "PLAT", Parent: page.ID, Type: "page", Title: "B"})
	if err != nil || child.Parent != page.ID {
		t.Fatalf("child %+v %v", child, err)
	}
	// A re-run that changes nothing writes nothing; one that changes
	// something keeps the page where a person moved it.
	if _, changed, err := pat.Import(u); err != nil || changed {
		t.Fatalf("unchanged re-run: %v %v", changed, err)
	}
	other := mustItem(t)(s.As("anna").Create(NewItem{Space: "PLAT", Type: "page", Title: "Elsewhere"}))
	noErr(t, s.As("anna").Move(child.ID, "PLAT", other.ID))
	again, changed, err := pat.Import(Upsert{Origin: notesOrigin("md/a/b.md"), Space: "PLAT", Parent: page.ID, Type: "page", Title: "B2"})
	if err != nil || !changed || again.ID != child.ID || again.Parent != other.ID || again.Title != "B2" {
		t.Fatalf("again %+v %v %v", again, changed, err)
	}
	// Not into another room, and not below an item of another room.
	if _, _, err := pat.Import(Upsert{Origin: notesOrigin("md/a.md"), Space: "OPS", Type: "page", Title: "A"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("into a room pat does not administer: %v", err)
	}
	noErr(t, s.GrantRole("root", "OPS", "pat", SpaceAdmin))
	if _, _, err := pat.Import(Upsert{Origin: notesOrigin("md/a.md"), Space: "OPS", Type: "page", Title: "A"}); !errors.Is(err, ErrValidation) {
		t.Fatalf("moved to another room: %v", err)
	}
	if _, _, err := pat.Import(Upsert{Origin: notesOrigin("md/c.md"), Space: "OPS", Parent: page.ID, Type: "page", Title: "C"}); !errors.Is(err, ErrValidation) {
		t.Fatalf("below an item of another room: %v", err)
	}
	// A room administrator reads and edits everything in the room, but
	// not what a running synchronisation manages.
	managed := notesOrigin("md/managed.md")
	if _, _, err := s.Upsert("sync:x", Upsert{Origin: managed, Space: "PLAT", Type: "page", Title: "M", Managed: "x"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := pat.Import(Upsert{Origin: managed, Space: "PLAT", Type: "page", Title: "M2"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("a managed page: %v", err)
	}
}
