package item

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// recurrenceCases are the cases the server and the browser share
// (testdata/recurrence-cases.json, ADR-0168).
type recurrenceCases struct {
	Parse []struct {
		In    string `json:"in"`
		Norm  string `json:"norm"`
		Error bool   `json:"error"`
	} `json:"parse"`
	Expand []struct {
		Name, Rule, Start, End, From, To string
		Timed                            bool
		Want                             []struct {
			Original, Start, End string
			Moved                bool
		}
	} `json:"expand"`
}

func loadRecurrenceCases(t *testing.T) recurrenceCases {
	t.Helper()
	data, err := os.ReadFile("testdata/recurrence-cases.json")
	noErr(t, err)
	var c recurrenceCases
	noErr(t, json.Unmarshal(data, &c))
	return c
}

func TestARuleIsReadAndWrittenInOneForm(t *testing.T) {
	for _, c := range loadRecurrenceCases(t).Parse {
		norm, err := NormalRecurrence(c.In)
		if c.Error {
			if err == nil {
				t.Errorf("%q: want a refusal, got %q", c.In, norm)
			}
			continue
		}
		if err != nil || norm != c.Norm {
			t.Errorf("%q = %q, %v; want %q", c.In, norm, err, c.Norm)
		}
		if again, _ := NormalRecurrence(norm); again != norm {
			t.Errorf("%q is not stable: %q", norm, again)
		}
	}
	r, err := ParseRecurrence("FREQ=DAILY;EXDATE=2026-01-01")
	noErr(t, err)
	r.ExDates = make([]string, maxExceptions+1)
	for i := range r.ExDates {
		r.ExDates[i] = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i).Format(time.DateOnly)
	}
	if _, err := ParseRecurrence(r.String()); err == nil {
		t.Error("too many cancelled occurrences are refused")
	}
}

func TestARuleIsExpandedIntoItsOccurrences(t *testing.T) {
	parse := func(s string) time.Time {
		if s == "" {
			return time.Time{}
		}
		v, err := time.Parse(time.RFC3339, s)
		noErr(t, err)
		return v
	}
	for _, c := range loadRecurrenceCases(t).Expand {
		r, err := ParseRecurrence(c.Rule)
		noErr(t, err)
		got := r.Expand(parse(c.Start), parse(c.End), c.Timed, parse(c.From), parse(c.To))
		var have, want []string
		for _, o := range got {
			have = append(have, strings.Join([]string{o.Original, o.Start.UTC().Format(time.RFC3339), o.End.UTC().Format(time.RFC3339), boolText(o.Moved)}, " "))
		}
		for _, w := range c.Want {
			want = append(want, strings.Join([]string{w.Original, w.Start, w.End, boolText(w.Moved)}, " "))
		}
		if !slices.Equal(have, want) {
			t.Errorf("%s:\n got  %q\n want %q", c.Name, have, want)
		}
	}
}

func boolText(b bool) string {
	if b {
		return "moved"
	}
	return "-"
}

func TestARecurrenceFieldIsValidated(t *testing.T) {
	s := permFixture(t)
	noErr(t, s.DefineType("admin", TypeDef{Name: "meeting", Fields: []FieldDef{{Name: "starts", Kind: KindDateTime}, {Name: "repeats", Kind: KindRecurrence}}}))
	anna := s.As("anna")
	it := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "meeting", Title: "Standup", Properties: Props{"repeats": "FREQ=WEEKLY;BYDAY=MO"}}))
	if err := anna.SetProperty(it.ID, "repeats", "FREQ=SOMETIMES"); err == nil || !strings.Contains(err.Error(), "FREQ") {
		t.Fatalf("a wrong rule is refused with its reason: %v", err)
	}
	if err := anna.SetProperty(it.ID, "repeats", 7); err == nil {
		t.Fatal("a number is no rule")
	}
	got, err := anna.Select(Query{Type: "meeting"}, "repeats IS NOT EMPTY", QueryEnv{Now: time.Now()})
	noErr(t, err)
	if len(got) != 1 {
		t.Fatalf("a repeating item is found by its rule: %d", len(got))
	}
	if _, err := anna.Select(Query{}, "repeats > x", QueryEnv{Now: time.Now()}); err == nil {
		t.Fatal("a rule has no order")
	}
}
