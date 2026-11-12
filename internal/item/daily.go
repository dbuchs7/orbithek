package item

import (
	"cmp"
	"slices"
	"strings"
	"time"
)

// A daily note per person and day (ADR-0205). A daily note is an item
// like any other (D1): of a type that holds notes, labelled DailyLabel,
// its date in the date field DailyField, and made by its author. Who made
// it, the label and the date tell it, not a stored index and not the
// type's name, so a person who moves a note to another room or relabels a
// page keeps what they see in their hands. A note is made in the room the
// person chose and is restricted to its author, read and edit (ADR-0078),
// in the batch that makes it, so nobody else reads it in between; the
// room's administrators read it as they read everything (ADR-0022).
//
// The ready-made type DailyType is defined in the same batch when the
// installation has none of that name, as the task of an access request
// defines its type (ADR-0162). A type of that name that cannot hold notes
// is refused, with the reason, rather than replaced.

// DailyType is the name of the ready-made type of daily notes.
const DailyType = "daily-note"

// DailyLabel labels every daily note, so it is found, and so people find
// their notes by the label in queries, boards and the label's page.
const DailyLabel = "daily-note"

// DailyField is the date field that holds the day of a daily note.
const DailyField = "date"

// DailyActor defines the ready-made type of daily notes when a person's
// first note needs it: no person, so a type's history tells it apart from
// what administrators define.
const DailyActor = "daily-notes"

// dailyTemplate is the ready-made type of daily notes: a text and the day.
func dailyTemplate() TypeDef {
	return TypeDef{Name: DailyType, Body: true, Fields: []FieldDef{{Name: DailyField, Kind: KindDate}}}
}

// holdsDailyNotes reports whether items of a type can be daily notes: a
// body, a date field DailyField, and no other required field, so a note is
// made from the day alone.
func holdsDailyNotes(td TypeDef) bool {
	if !td.Body {
		return false
	}
	if f, ok := td.field(DailyField); !ok || f.Kind != KindDate {
		return false
	}
	return !slices.ContainsFunc(td.Fields, func(f FieldDef) bool { return f.Required && f.Name != DailyField })
}

// DayOf is the day a date value names, at midnight UTC, and whether the
// value is a date. Dates are stored at midnight UTC; a day is a day
// wherever the reader is (ADR-0166).
func DayOf(v any) (time.Time, bool) {
	t, ok := v.(time.Time)
	if !ok {
		return time.Time{}, false
	}
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), true
}

// IsDailyNote reports whether an item is a daily note, and its day: live
// or not, of a type that holds notes, labelled DailyLabel, with a day.
func (s *Store) IsDailyNote(it *Item) (time.Time, bool) {
	if !slices.Contains(it.Labels, DailyLabel) {
		return time.Time{}, false
	}
	td, ok := s.TypeAt(it.Type, it.TypeVersion)
	if !ok || !holdsDailyNotes(td) {
		return time.Time{}, false
	}
	return DayOf(it.Properties[DailyField])
}

// dailyNotes calls fn with every live daily note the principal made, in
// the order made, and its day.
func (s *Store) dailyNotes(principal string, fn func(it *Item, day time.Time)) {
	var notes []*Item
	for _, it := range s.items {
		if it.Presence == Live && s.log[it.created-1].Actor == principal {
			if _, ok := s.IsDailyNote(it); ok {
				notes = append(notes, it)
			}
		}
	}
	slices.SortFunc(notes, func(a, b *Item) int { return cmp.Compare(a.created, b.created) })
	for _, it := range notes {
		d, _ := s.IsDailyNote(it)
		fn(it, d)
	}
}

// dailyNote is the principal's first readable note of a day, or nil.
func (x *Session) dailyNote(day time.Time) *Item {
	day, _ = DayOf(day)
	var found *Item
	x.s.dailyNotes(x.who, func(it *Item, d time.Time) {
		if found == nil && d.Equal(day) && x.s.canRead(x.who, it) {
			found = it
		}
	})
	return found
}

// DailyNote returns the principal's daily note of a day, as they read it.
// A note someone else made is never theirs, even where they read it.
func (x *Session) DailyNote(day time.Time) (Item, bool) {
	it := x.dailyNote(day)
	if it == nil {
		return Item{}, false
	}
	return x.s.veiled(x.who, it).clone(), true
}

// DailyNoteDays lists the days from from to to, both included, on which
// the principal has a daily note, in order and each once.
func (x *Session) DailyNoteDays(from, to time.Time) []time.Time {
	from, _ = DayOf(from)
	to, _ = DayOf(to)
	var out []time.Time
	x.s.dailyNotes(x.who, func(it *Item, d time.Time) {
		if !d.Before(from) && !d.After(to) && x.s.canRead(x.who, it) && !slices.ContainsFunc(out, d.Equal) {
			out = append(out, d)
		}
	})
	slices.SortFunc(out, func(a, b time.Time) int { return a.Compare(b) })
	return out
}

// dailyType chooses the type of a new daily note: the one named DailyType
// when it holds notes, or the ready-made one to be defined in the same
// batch (define) when no type has that name.
func (s *Store) dailyType() (td TypeDef, define bool, err error) {
	if cur, ok := s.latest(DailyType); ok {
		if !holdsDailyNotes(cur) {
			return TypeDef{}, false, invalid("the type %s cannot hold daily notes: it needs a text and a date field %q and no other required field; an administrator changes it", DailyType, DailyField)
		}
		return cur, false, nil
	}
	td = dailyTemplate()
	td.Version = len(s.types[td.Name]) + 1
	return td, true, nil
}

// OpenDailyNote returns the principal's daily note of a day, or makes it
// in space when there is none (created). The store's single writer runs
// it as one command, so two requests at once make one note. title names a
// new note; empty names it by its day, as 2026-11-12. body, if given, is
// the new note's first text, committed in the same batch.
func (x *Session) OpenDailyNote(space string, day time.Time, title string) (Item, bool, error) {
	return x.openDailyNote(space, day, title, nil)
}

func (x *Session) openDailyNote(space string, day time.Time, title string, body *Doc) (Item, bool, error) {
	day, _ = DayOf(day)
	if it := x.dailyNote(day); it != nil {
		return x.s.veiled(x.who, it).clone(), false, nil
	}
	if strings.TrimSpace(space) == "" {
		return Item{}, false, invalid("choose the room your daily notes go to")
	}
	if !x.s.HasSpace(space) {
		return Item{}, false, notFound("space %q", space)
	}
	if err := x.canCreateIn(space, ""); err != nil {
		return Item{}, false, err
	}
	if strings.TrimSpace(title) == "" {
		title = day.Format(time.DateOnly)
	}
	td, define, err := x.s.dailyType()
	if err != nil {
		return Item{}, false, err
	}
	props := Props{DailyField: day}
	var events []Event
	var created Event
	if define {
		events = append(events, Event{Kind: TypeDefined, TypeDef: &td, Actor: DailyActor})
		created = Event{Kind: ItemCreated, Item: x.s.newID(), Space: space, TypeName: td.Name, TypeVersion: td.Version, Title: title, Props: props}
	} else if created, err = x.s.prepareCreate(NewItem{Space: space, Type: td.Name, Title: title, Properties: props}); err != nil {
		return Item{}, false, err
	}
	events = append(events, created)
	if body != nil {
		if created.Template != nil {
			// A Word template's start text comes first; the note keeps it.
			return Item{}, false, invalid("the type %s has a Word template; open the note first, then add to it", td.Name)
		}
		b, err := x.s.firstBodyOf(td, created, *body)
		if err != nil {
			return Item{}, false, err
		}
		events = append(events, b)
	}
	only := []string{x.who}
	events = append(events,
		Event{Kind: LabelsSet, Item: created.Item, Labels: []string{DailyLabel}},
		Event{Kind: RestrictionSet, Item: created.Item, Restriction: &Restriction{Read: only, Edit: slices.Clone(only)}},
		// A person watches what they make (ADR-0046).
		Event{Kind: Watched, Item: created.Item, Principal: x.who})
	if err := x.s.recordBatch(x.who, events...); err != nil {
		return Item{}, false, err
	}
	return x.s.veiled(x.who, x.s.items[created.Item]).clone(), true, nil
}

// AppendToDailyNote adds text at the end of the principal's daily note of
// a day, a paragraph per line, empty lines dropped; it makes the note in
// space first when there is none (created), in the same batch. This is
// what a quick capture adds to today's note.
func (x *Session) AppendToDailyNote(space string, day time.Time, title, text string) (Item, bool, error) {
	var add []Block
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			add = append(add, Block{Kind: Paragraph, Text: line})
		}
	}
	if len(add) == 0 {
		return Item{}, false, invalid("there is no text to add")
	}
	it := x.dailyNote(day)
	if it == nil {
		return x.openDailyNote(space, day, title, &Doc{Blocks: add})
	}
	d := it.Body.clone()
	d.Blocks = append(d.Blocks, add...)
	if err := x.CommitBody(it.ID, d); err != nil {
		return Item{}, false, err
	}
	return x.s.veiled(x.who, x.s.items[it.ID]).clone(), false, nil
}
