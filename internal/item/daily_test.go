package item

import (
	"errors"
	"slices"
	"testing"
	"time"
)

// dailyFixture: an installation without the daily note type. anna and ben
// edit PLAT, vic only views it, pat administers it.
func dailyFixture(t *testing.T) *Store {
	t.Helper()
	s := newTestStore(t)
	noErr(t, s.DefineType("admin", TypeTemplates()[0]))
	noErr(t, s.CreateSpace("admin", "PLAT", "Platform"))
	noErr(t, s.CreateSpace("admin", "OPS", "Operations"))
	for who, role := range map[string]Role{"anna": Editor, "ben": Editor, "vic": Viewer, "pat": SpaceAdmin} {
		noErr(t, s.GrantRole("root", "PLAT", who, role))
	}
	return s
}

func onDay(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func TestTheDailyNoteTemplateHoldsNotes(t *testing.T) {
	var tmpl TypeDef
	for _, td := range TypeTemplates() {
		if td.Name == DailyType {
			tmpl = td
		}
	}
	if !holdsDailyNotes(tmpl) {
		t.Fatalf("the ready-made %s type cannot hold daily notes: %+v", DailyType, tmpl)
	}
	// The access request's task type stays the first template that can
	// hold a request (ADR-0162).
	if holdsRequests(tmpl) {
		t.Fatal("a daily note type would hold access requests")
	}
	for _, td := range []TypeDef{
		{Name: "x", Body: false, Fields: []FieldDef{{Name: DailyField, Kind: KindDate}}},
		{Name: "x", Body: true, Fields: []FieldDef{{Name: DailyField, Kind: KindText}}},
		{Name: "x", Body: true},
		{Name: "x", Body: true, Fields: []FieldDef{{Name: DailyField, Kind: KindDate}, {Name: "mood", Kind: KindText, Required: true}}},
	} {
		if holdsDailyNotes(td) {
			t.Errorf("%+v holds daily notes", td)
		}
	}
}

func TestADailyNoteIsMadeOnceRestrictedToItsAuthor(t *testing.T) {
	s := dailyFixture(t)
	anna := s.As("anna")
	if _, ok := anna.DailyNote(onDay(2026, 11, 12)); ok {
		t.Fatal("a note before any was made")
	}
	note, created, err := anna.OpenDailyNote("PLAT", onDay(2026, 11, 12), "")
	noErr(t, err)
	if !created {
		t.Fatal("the first open did not make the note")
	}
	if note.Type != DailyType || note.Title != "2026-11-12" || !slices.Contains(note.Labels, DailyLabel) {
		t.Fatalf("note %+v", note)
	}
	if got := note.Properties[DailyField]; got != onDay(2026, 11, 12) {
		t.Fatalf("date %v", got)
	}
	if !slices.Equal(note.Restriction.Read, []string{"anna"}) || !slices.Equal(note.Restriction.Edit, []string{"anna"}) {
		t.Fatalf("restriction %+v", note.Restriction)
	}
	// The type was defined from the template in the same batch.
	if td := s.Type(DailyType); !holdsDailyNotes(td) {
		t.Fatalf("type %+v", td)
	}
	created1 := s.log[s.items[note.ID].created-1]
	for _, e := range s.log {
		if e.Batch != 0 && e.Batch == created1.Batch && e.Kind == TypeDefined && e.Actor != DailyActor {
			t.Errorf("the type is defined by %q", e.Actor)
		}
	}
	if !slices.ContainsFunc(s.log, func(e Event) bool { return e.Kind == RestrictionSet && e.Item == note.ID && e.Batch == created1.Batch }) {
		t.Fatal("the restriction is not in the batch that makes the note")
	}
	if !s.As("anna").Watching(note.ID) {
		t.Error("the author does not watch the note")
	}

	again, created, err := anna.OpenDailyNote("OPS", onDay(2026, 11, 12), "Another title")
	noErr(t, err)
	if created || again.ID != note.ID {
		t.Fatalf("a second open made %v (%s)", created, again.ID)
	}
	if got, ok := anna.DailyNote(onDay(2026, 11, 12)); !ok || got.ID != note.ID {
		t.Fatalf("DailyNote: %v %v", got.ID, ok)
	}

	// Nobody else reads it, not even who edits the room; ben has a note
	// of his own for the same day.
	if _, err := s.As("ben").Get(note.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ben reads anna's note: %v", err)
	}
	if _, ok := s.As("ben").DailyNote(onDay(2026, 11, 12)); ok {
		t.Fatal("ben finds anna's note as his own")
	}
	bens, created, err := s.As("ben").OpenDailyNote("PLAT", onDay(2026, 11, 12), "Thursday")
	noErr(t, err)
	if !created || bens.ID == note.ID || bens.Title != "Thursday" {
		t.Fatalf("ben's note %+v %v", bens, created)
	}
	// The type is defined once.
	n := 0
	for _, e := range s.log {
		if e.Kind == TypeDefined && e.TypeDef.Name == DailyType {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("the type was defined %d times", n)
	}
}

func TestADailyNoteNeedsARoomToCreateIn(t *testing.T) {
	s := dailyFixture(t)
	if _, _, err := s.As("vic").OpenDailyNote("PLAT", onDay(2026, 11, 12), ""); !errors.Is(err, ErrForbidden) {
		t.Fatalf("a viewer makes a note: %v", err)
	}
	if _, _, err := s.As("anna").OpenDailyNote("OPS", onDay(2026, 11, 12), ""); !errors.Is(err, ErrForbidden) {
		t.Fatalf("a note in a room without a role: %v", err)
	}
	if _, _, err := s.As("anna").OpenDailyNote("NOPE", onDay(2026, 11, 12), ""); err == nil {
		t.Fatal("a note in no room")
	}
	if _, _, err := s.As("anna").OpenDailyNote("", onDay(2026, 11, 12), ""); !errors.Is(err, ErrValidation) {
		t.Fatalf("no room: %v", err)
	}
	// Nothing new is made at critical storage (ADR-0163).
	full := errors.New("full")
	s.SetNothingNew(func() error { return full })
	if _, _, err := s.As("anna").OpenDailyNote("PLAT", onDay(2026, 11, 12), ""); !errors.Is(err, full) {
		t.Fatalf("at critical storage: %v", err)
	}
	if len(s.Types()) != 1 {
		t.Fatal("the type was defined although the note was refused")
	}
	s.SetNothingNew(nil)
}

func TestADailyNoteTypeThatCannotHoldNotesIsRefused(t *testing.T) {
	s := dailyFixture(t)
	noErr(t, s.DefineType("admin", TypeDef{Name: DailyType, Body: false}))
	if _, _, err := s.As("anna").OpenDailyNote("PLAT", onDay(2026, 11, 12), ""); !errors.Is(err, ErrValidation) {
		t.Fatalf("a note of a type without a body: %v", err)
	}
}

func TestADailyNoteKeepsItsIdentityByLabelDateAndAuthor(t *testing.T) {
	s := dailyFixture(t)
	anna := s.As("anna")
	note, _, err := anna.OpenDailyNote("PLAT", onDay(2026, 11, 12), "")
	noErr(t, err)
	// A page anna labels by hand with a date is not her daily note: it
	// is not of a type that holds notes.
	noErr(t, s.DefineType("admin", TypeDef{Name: "log", Body: true, Fields: []FieldDef{{Name: DailyField, Kind: KindDate}}}))
	other := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "log", Title: "x", Properties: Props{DailyField: onDay(2026, 11, 13)}}))
	noErr(t, anna.SetLabels(other.ID, []string{DailyLabel}))
	if got, ok := anna.DailyNote(onDay(2026, 11, 13)); !ok || got.ID != other.ID {
		t.Fatalf("a labelled item of a type that holds notes is a note: %v %v", got.ID, ok)
	}
	// Without its label the note is no longer found, and a new one is made.
	noErr(t, anna.SetLabels(note.ID, nil))
	if _, ok := anna.DailyNote(onDay(2026, 11, 12)); ok {
		t.Fatal("an unlabelled note is still found")
	}
	// Trashed, it is not found either.
	noErr(t, anna.SetLabels(note.ID, []string{DailyLabel}))
	noErr(t, anna.Trash(note.ID))
	if _, ok := anna.DailyNote(onDay(2026, 11, 12)); ok {
		t.Fatal("a trashed note is found")
	}
}

func TestDailyNoteDaysListsTheAuthorsDaysInAPeriod(t *testing.T) {
	s := dailyFixture(t)
	for _, d := range []time.Time{onDay(2026, 10, 31), onDay(2026, 11, 2), onDay(2026, 11, 12), onDay(2026, 12, 1)} {
		_, _, err := s.As("anna").OpenDailyNote("PLAT", d, "")
		noErr(t, err)
	}
	_, _, err := s.As("ben").OpenDailyNote("PLAT", onDay(2026, 11, 20), "")
	noErr(t, err)
	got := s.As("anna").DailyNoteDays(onDay(2026, 11, 1), onDay(2026, 11, 30))
	if want := []time.Time{onDay(2026, 11, 2), onDay(2026, 11, 12)}; !slices.Equal(got, want) {
		t.Fatalf("days %v, want %v", got, want)
	}
	if got := s.As("vic").DailyNoteDays(onDay(2026, 1, 1), onDay(2026, 12, 31)); len(got) != 0 {
		t.Fatalf("vic's days %v", got)
	}
}

func TestAParagraphIsAppendedToTheDailyNote(t *testing.T) {
	s := dailyFixture(t)
	anna := s.As("anna")
	// The first paragraph makes the note, in one batch.
	note, created, err := anna.AppendToDailyNote("PLAT", onDay(2026, 11, 12), "", "Call the vendor.")
	noErr(t, err)
	if !created || len(note.Body.Blocks) != 1 || note.Body.Blocks[0].Text != "Call the vendor." || note.Body.Blocks[0].Kind != Paragraph {
		t.Fatalf("note %+v", note.Body)
	}
	note, created, err = anna.AppendToDailyNote("PLAT", onDay(2026, 11, 12), "", "Order the parts.")
	noErr(t, err)
	if created || len(note.Body.Blocks) != 2 || note.Body.Blocks[1].Text != "Order the parts." {
		t.Fatalf("note %+v", note.Body)
	}
	for _, bad := range []string{"", "   ", "\n \r\n"} {
		if _, _, err := anna.AppendToDailyNote("PLAT", onDay(2026, 11, 12), "", bad); !errors.Is(err, ErrValidation) {
			t.Errorf("%q: %v", bad, err)
		}
	}
	// Lines become paragraphs of their own; empty lines are dropped.
	note, _, err = anna.AppendToDailyNote("PLAT", onDay(2026, 11, 12), "", "two\nlines\n\nand more")
	noErr(t, err)
	if n := len(note.Body.Blocks); n != 5 || note.Body.Blocks[4].Text != "and more" {
		t.Fatalf("%d blocks: %+v", n, note.Body.Blocks)
	}
	// Sensitive data keeps the note to its author (ADR-0040).
	note, _, err = anna.AppendToDailyNote("PLAT", onDay(2026, 11, 13), "", "password: Sommer2026!")
	noErr(t, err)
	if !slices.Equal(note.Restriction.Read, []string{"anna"}) {
		t.Fatalf("restriction %+v", note.Restriction)
	}
	if _, err := s.As("pat").Get(note.ID); err != nil {
		t.Fatalf("the room's administrator reads every item (ADR-0022): %v", err)
	}
}
