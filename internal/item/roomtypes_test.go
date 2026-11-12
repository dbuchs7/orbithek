package item

import (
	"errors"
	"slices"
	"testing"
)

// roomType is a type of the space PLAT (ADR-0201).
func roomType() TypeDef {
	return TypeDef{Name: "incident", Space: "PLAT", Body: true,
		Fields:   []FieldDef{{Name: "severity", Kind: KindEnum, Values: []string{"low", "high"}}},
		Workflow: &Workflow{Initial: "open", States: []string{"open", "done"}, Transitions: []Transition{{Name: "finish", From: []string{"open"}, To: "done"}}}}
}

func typeNames(types []TypeDef) []string {
	var out []string
	for _, td := range types {
		out = append(out, td.Name)
	}
	return out
}

func TestRoomAdministratorsDefineTheirRoomsTypes(t *testing.T) {
	s := permFixture(t)
	pat, anna, dora := s.As("pat"), s.As("anna"), s.As("dora")

	td, err := pat.DefineType(roomType())
	noErr(t, err)
	if td.Version != 1 || td.Space != "PLAT" {
		t.Fatalf("defined %+v", td)
	}
	// An editor may not; nor one who sees nothing of the space, who is not
	// told it exists.
	if _, err := anna.DefineType(TypeDef{Name: "note", Space: "PLAT"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("editor defines a type: %v", err)
	}
	if _, err := dora.DefineType(TypeDef{Name: "note", Space: "PLAT"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("outsider defines a type: %v", err)
	}
	if _, err := pat.DefineType(TypeDef{Name: "note"}); !errors.Is(err, ErrValidation) {
		t.Fatalf("a room administrator defines an installation type: %v", err)
	}
	// An installation administrator may define a room's type, as the
	// installation's own types.
	noErr(t, s.GrantSystemRole("root", "ivy", RoleAdmin))
	if _, err := s.As("ivy").DefineType(TypeDef{Name: "note", Space: "OPS"}); err != nil {
		t.Fatalf("an installation administrator defines a room type: %v", err)
	}
	if !pat.MayDefineTypes("PLAT") || pat.MayDefineTypes("OPS") || pat.MayDefineTypes("") || !s.As("ivy").MayDefineTypes("") {
		t.Fatal("MayDefineTypes")
	}

	// Names stay unique across spaces and the installation; a type keeps
	// its space.
	for name, def := range map[string]TypeDef{
		"installation name": {Name: "task", Space: "PLAT"},
		"other room's name": {Name: "note", Space: "PLAT"},
		"unknown space":     {Name: "x", Space: "NOPE"},
	} {
		if err := s.DefineType("root", def); err == nil {
			t.Errorf("%s: defined", name)
		}
	}
	if err := s.DefineType("root", TypeDef{Name: "incident"}); !errors.Is(err, ErrValidation) {
		t.Fatalf("a room type became an installation type: %v", err)
	}
	next := roomType()
	next.Body = false
	td, err = pat.DefineType(next)
	noErr(t, err)
	if td.Version != 2 {
		t.Fatalf("redefined as version %d", td.Version)
	}
}

func TestARoomTypeIsUsedOnlyInItsRoom(t *testing.T) {
	s := permFixture(t)
	pat := s.As("pat")
	_, err := pat.DefineType(roomType())
	noErr(t, err)
	noErr(t, s.GrantRole("root", "OPS", "pat", SpaceAdmin))

	it := mustItem(t)(pat.Create(NewItem{Space: "PLAT", Type: "incident", Title: "Outage"}))
	if it.State != "open" {
		t.Fatalf("state %q", it.State)
	}
	if _, err := pat.Create(NewItem{Space: "OPS", Type: "incident", Title: "Elsewhere"}); !errors.Is(err, ErrValidation) {
		t.Fatalf("created in another room: %v", err)
	}
	if err := pat.Move(it.ID, "OPS", ""); !errors.Is(err, ErrValidation) {
		t.Fatalf("moved to another room: %v", err)
	}
	page := mustItem(t)(pat.Create(NewItem{Space: "PLAT", Type: "page", Title: "Parent"}))
	noErr(t, pat.Move(it.ID, "PLAT", page.ID))
	if err := pat.Move(page.ID, "OPS", ""); !errors.Is(err, ErrValidation) {
		t.Fatalf("moved a subtree holding a room type's item: %v", err)
	}
	if _, err := pat.SetKanban("OPS", Kanban{Name: "Incidents", Type: "incident"}); !errors.Is(err, ErrValidation) {
		t.Fatalf("a kanban of another room shows the type: %v", err)
	}
	if _, err := pat.SetKanban("PLAT", Kanban{Name: "Incidents", Type: "incident"}); err != nil {
		t.Fatalf("a kanban of the room: %v", err)
	}
	if _, _, err := s.Upsert("sync", Upsert{Space: "OPS", Type: "incident", Title: "x", Origin: Origin{System: "s", Instance: "i", ExternalID: "1"}}); !errors.Is(err, ErrValidation) {
		t.Fatalf("imported into another room: %v", err)
	}
	if err := s.TaskReach("OPS", "incident", "done", "done", nil); !errors.Is(err, ErrValidation) {
		t.Fatalf("linked tasks of another room: %v", err)
	}
	// Its room's administrator adds a transition, as an installation's
	// administrator does to any type.
	if _, err := pat.AddTransition("incident", "done", "open"); err != nil {
		t.Fatalf("room administrator adds a transition: %v", err)
	}
	if _, err := pat.AddTransition("task", "done", "open"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("room administrator changes an installation type: %v", err)
	}
}

func TestTypesAreListedByRoom(t *testing.T) {
	s := permFixture(t)
	pat := s.As("pat")
	_, err := pat.DefineType(roomType())
	noErr(t, err)
	noErr(t, s.DefineType("root", TypeDef{Name: "runbook", Space: "OPS"}))

	all := typeNames(s.Types())
	if !slices.Contains(all, "incident") || !slices.Contains(all, "runbook") {
		t.Fatalf("store lists %v", all)
	}
	// Ben reads PLAT only, Dora OPS only.
	if got := typeNames(s.As("ben").Types("")); !slices.Contains(got, "incident") || slices.Contains(got, "runbook") || !slices.Contains(got, "task") {
		t.Fatalf("ben sees %v", got)
	}
	if got := typeNames(s.As("dora").Types("OPS")); slices.Contains(got, "incident") || !slices.Contains(got, "runbook") {
		t.Fatalf("dora sees in OPS %v", got)
	}
	if got := typeNames(pat.Types("OPS")); slices.Contains(got, "incident") {
		t.Fatalf("types usable in OPS hold PLAT's: %v", got)
	}
}

func TestARoomTypeIsRemovedByItsRoomWhenUnused(t *testing.T) {
	s := permFixture(t)
	pat, anna := s.As("pat"), s.As("anna")
	_, err := pat.DefineType(roomType())
	noErr(t, err)
	it := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "incident", Title: "Outage"}))

	if err := anna.RemoveType("incident"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("editor removes: %v", err)
	}
	if err := s.As("dora").RemoveType("incident"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("outsider removes: %v", err)
	}
	if err := pat.RemoveType("task"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("room administrator removes an installation type: %v", err)
	}
	if err := pat.RemoveType("incident"); !errors.Is(err, ErrReferenced) {
		t.Fatalf("removed while used: %v", err)
	}
	noErr(t, anna.Trash(it.ID))
	noErr(t, pat.Purge(it.ID))
	noErr(t, pat.RemoveType("incident"))
	if err := pat.RemoveType("incident"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("removed twice: %v", err)
	}
	// The name is free again, for any space.
	noErr(t, s.DefineType("root", TypeDef{Name: "incident", Space: "OPS"}))
}

func TestAnAccessRequestsTaskIsOfATypeOfItsRoom(t *testing.T) {
	s := NewStore()
	noErr(t, s.CreateSpace("admin", "PLAT", "Platform"))
	noErr(t, s.CreateSpace("admin", "OPS", "Operations"))
	tmpl := TypeTemplates()[1]
	tmpl.Space = "OPS"
	noErr(t, s.DefineType("admin", tmpl))
	// The name task is taken by OPS, so a request in PLAT goes without a
	// task rather than with one of OPS's type.
	if td, _, ok := s.requestType("PLAT"); ok {
		t.Fatalf("PLAT's request uses %s of %q", td.Name, td.Space)
	}
	if td, define, ok := s.requestType("OPS"); !ok || define || td.Space != "OPS" {
		t.Fatalf("OPS's request: %v %v %v", td.Name, define, ok)
	}
}
