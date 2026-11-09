package item

import (
	"slices"
	"strings"
	"testing"
	"time"
)

// dateTimeFixture: a meeting type with a point in time and a day, on
// PLAT, edited by anna (ADR-0166).
//
//	PLAT-1 "Standup"   starts 2026-11-09 08:30 UTC, day 2026-11-09
//	PLAT-2 "Review"    starts 2026-11-09 13:30 UTC
//	PLAT-3 "Retro"     starts 2026-11-10 00:15 UTC (01:15 in Zurich)
//	PLAT-4 "Someday"   no start
func dateTimeFixture(t *testing.T) (*Store, map[string]Item) {
	t.Helper()
	s := permFixture(t)
	noErr(t, s.DefineType("admin", TypeDef{Name: "meeting", Fields: []FieldDef{
		{Name: "starts", Kind: KindDateTime}, {Name: "day", Kind: KindDate}, {Name: "due", Kind: KindDateTime},
	}, Workflow: &Workflow{Initial: "planned", States: []string{"planned", "held"}, Transitions: []Transition{{Name: "hold", From: []string{"planned"}, To: "held"}}}}))
	anna := s.As("anna")
	items := map[string]Item{}
	add := func(name, title string, props Props) {
		items[name] = mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "meeting", Title: title, Properties: props}))
	}
	add("standup", "Standup", Props{"starts": time.Date(2026, 11, 9, 8, 30, 0, 0, time.UTC), "day": time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)})
	add("review", "Review", Props{"starts": time.Date(2026, 11, 9, 14, 30, 0, 0, time.FixedZone("CET", 3600))})
	add("retro", "Retro", Props{"starts": time.Date(2026, 11, 10, 0, 15, 0, 0, time.UTC)})
	add("someday", "Someday", nil)
	return s, items
}

func TestADateTimeFieldHoldsAPointInTime(t *testing.T) {
	s, items := dateTimeFixture(t)
	anna := s.As("anna")
	got, _ := anna.Get(items["review"].ID)
	if want := time.Date(2026, 11, 9, 13, 30, 0, 0, time.UTC); !got.Properties["starts"].(time.Time).Equal(want) {
		t.Fatalf("starts = %v, want %v", got.Properties["starts"], want)
	}
	if err := anna.SetProperty(items["review"].ID, "starts", "2026-11-09T14:30:00Z"); err == nil {
		t.Fatal("a text is no point in time; the API turns text into one first")
	}
	f := FieldDef{Name: "starts", Kind: KindDateTime}
	if s := FormatField(f, got.Properties["starts"]); s != "2026-11-09T13:30:00Z" {
		t.Fatalf("FormatField = %q", s)
	}
	if s := FormatField(FieldDef{Kind: KindDate}, got.Properties["starts"]); s != "2026-11-09" {
		t.Fatalf("a date field still reads as its day: %q", s)
	}
	if s := FormatField(f, nil); s != "" {
		t.Fatalf("no value reads as nothing: %q", s)
	}
}

func TestADateTimeFieldIsFoundByItsPointInTime(t *testing.T) {
	s, _ := dateTimeFixture(t)
	found := s.Find(Query{Type: "meeting", WhereText: map[string]string{"starts": "2026-11-09T13:30:00Z"}})
	if keys := keysOf(found); !slices.Equal(keys, []string{"PLAT-2"}) {
		t.Fatalf("where by a point in time = %v", keys)
	}
	if keys := keysOf(s.Find(Query{Type: "meeting", WhereText: map[string]string{"starts": "2026-11-09"}})); len(keys) != 0 {
		t.Fatalf("a day is not a point in time: %v", keys)
	}
	if keys := keysOf(s.Find(Query{Type: "meeting", WhereText: map[string]string{"day": "2026-11-09"}})); !slices.Equal(keys, []string{"PLAT-1"}) {
		t.Fatalf("a date field is still found by its day: %v", keys)
	}
}

func TestSelectComparesPointsInTime(t *testing.T) {
	s, _ := dateTimeFixture(t)
	env := QueryEnv{Now: time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)}
	cases := []struct {
		query string
		want  []string
	}{
		{"starts = 2026-11-09", []string{"PLAT-1", "PLAT-2"}},
		{"starts >= 2026-11-09 AND starts <= 2026-11-09", []string{"PLAT-1", "PLAT-2"}},
		{"starts > 2026-11-09T12:00:00Z", []string{"PLAT-2", "PLAT-3"}},
		{"starts < 2026-11-09T14:30:00+01:00", []string{"PLAT-1"}},
		{"starts <= 2026-11-09T14:30:00+01:00", []string{"PLAT-1", "PLAT-2"}},
		{"starts > now()", []string{"PLAT-2", "PLAT-3"}},
		{"starts < -1h", []string{"PLAT-1"}},
		{"starts < +3h", []string{"PLAT-1", "PLAT-2"}},
		{"starts is empty", []string{"PLAT-4"}},
		{"\"starts\" >= 2026-11-10", []string{"PLAT-3"}},
		{"starts > 2026-11-09 ORDER BY starts DESC", []string{"PLAT-3"}},
		{"starts is not empty ORDER BY starts DESC", []string{"PLAT-3", "PLAT-2", "PLAT-1"}},
	}
	for _, c := range cases {
		got, err := s.As("ben").Select(Query{Type: "meeting"}, c.query, env)
		if err != nil {
			t.Errorf("%s: %v", c.query, err)
			continue
		}
		if keys := keysOf(got); !slices.Equal(keys, c.want) {
			t.Errorf("%s = %v, want %v", c.query, keys, c.want)
		}
	}
	if _, err := s.As("ben").Select(Query{}, "starts = soon", env); err == nil {
		t.Error("a word that is no time is refused")
	}
	if _, err := s.As("ben").Select(Query{}, "starts ~ nine", env); err == nil {
		t.Error("~ is for text")
	}
}

func TestAPassedDeadlineAtAnHourIsImplausible(t *testing.T) {
	s, items := dateTimeFixture(t)
	anna := s.As("anna")
	noErr(t, anna.SetProperty(items["someday"].ID, "due", time.Date(2026, 11, 9, 9, 0, 0, 0, time.UTC)))
	rep, err := s.As("pat").Governance("PLAT", time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC), 365*24*time.Hour)
	noErr(t, err)
	var problems []string
	for _, f := range rep.Implausible {
		problems = append(problems, f.Key+": "+f.Problem)
	}
	if !slices.ContainsFunc(problems, func(p string) bool { return strings.HasPrefix(p, "PLAT-4: due 2026-11-09 has passed") }) {
		t.Fatalf("a passed deadline with a time is not reported: %v", problems)
	}
	noErr(t, anna.Transition(items["someday"].ID, "hold"))
	rep, _ = s.As("pat").Governance("PLAT", time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC), 365*24*time.Hour)
	for _, f := range rep.Implausible {
		if f.Key == "PLAT-4" {
			t.Fatalf("a finished item is not overdue: %v", f.Problem)
		}
	}
}

func TestADateTimeFieldSurvivesTheJournal(t *testing.T) {
	s, items := dateTimeFixture(t)
	// Every test's journal encodes and decodes each batch (ADR-0025); a
	// replay of the log then rebuilds the same state.
	replayed, err := Replay(s.Log())
	noErr(t, err)
	got, err := replayed.As("anna").Get(items["retro"].ID)
	noErr(t, err)
	if want := time.Date(2026, 11, 10, 0, 15, 0, 0, time.UTC); !got.Properties["starts"].(time.Time).Equal(want) {
		t.Fatalf("after replay starts = %v", got.Properties["starts"])
	}
	td, _ := replayed.latest("meeting")
	if f, _ := td.field("starts"); f.Kind != KindDateTime {
		t.Fatalf("the kind after replay is %q", f.Kind)
	}
}
