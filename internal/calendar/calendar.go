// Package calendar turns a saved calendar view into the entries of a
// calendar feed (ADR-0167): which date fields each type stands on, the
// query that reads the items of a stretch of time, the occurrences of
// repeating items, and the feed in iCalendar (RFC 5545).
//
// The browser's calendar (internal/web/static/assets/calendar.js) chooses
// fields and writes its query the same way; the tests of both hold the
// same cases, and both expand repeating items from the same cases
// (internal/item/testdata/recurrence-cases.json).
package calendar

import (
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/dbuchs7/orbithek/internal/item"
)

// View is what a saved view chooses, read from its address's parameters.
type View struct {
	Space, Type       string
	Labels            []string
	Who, Status, Text string
	Q, Start, End     string
}

// ParseView reads a view from the parameters of its address, as the
// calendar writes them (#/calendar?who=me&status=open…).
func ParseView(params string) View {
	q, _ := url.ParseQuery(strings.TrimPrefix(params, "?"))
	v := View{Space: q.Get("space"), Type: q.Get("type"), Text: strings.TrimSpace(q.Get("text")), Q: q.Get("q"), Start: q.Get("start"), End: q.Get("end")}
	if w := q.Get("who"); w == "me" || w == "none" {
		v.Who = w
	}
	if s := q.Get("status"); s == "open" || s == "done" {
		v.Status = s
	}
	for l := range strings.SplitSeq(q.Get("labels"), ",") {
		if l = strings.TrimSpace(l); l != "" && !slices.Contains(v.Labels, l) {
			v.Labels = append(v.Labels, l)
		}
	}
	return v
}

// Fields are the fields one type's items are placed by.
type Fields struct {
	Type                 string
	Start, End           string
	StartTimed, EndTimed bool
	Assignee, Recurrence string
}

var (
	startNames = regexp.MustCompile(`(?i)^(start|starts|begin|begins|from|since|kickoff|start_?date|begin_?date|planned_?start)$`)
	endNames   = regexp.MustCompile(`(?i)^(due|end|ends|until|deadline|finish|finishes|target|end_?date|due_?date|planned_?end|release|delivery)$`)
	assignees  = regexp.MustCompile(`(?i)^(assignee|zugewiesen|bearbeiter)`)
)

// guess picks a start and an end among a type's date fields by their
// names, as the roadmap does (timeline.js guessFields).
func guess(names []string) (start, end string) {
	for _, n := range names {
		if endNames.MatchString(n) {
			end = n
			break
		}
	}
	if end == "" {
		for _, n := range names {
			if !startNames.MatchString(n) {
				end = n
				break
			}
		}
	}
	for _, n := range names {
		if startNames.MatchString(n) && n != end {
			start = n
			break
		}
	}
	return start, end
}

// TypeFields is, for every type with date fields that the view shows, the
// fields its items are placed by: guessed from their names, or the start
// and end the view chose, for every type that has them.
func TypeFields(types []item.TypeDef, v View) []Fields {
	var out []Fields
	for _, td := range types {
		if v.Type != "" && td.Name != v.Type {
			continue
		}
		kinds := map[string]item.FieldKind{}
		var names []string
		var users []string
		f := Fields{Type: td.Name}
		for _, fd := range td.Fields {
			kinds[fd.Name] = fd.Kind
			switch fd.Kind {
			case item.KindDate, item.KindDateTime:
				names = append(names, fd.Name)
			case item.KindUser:
				users = append(users, fd.Name)
			case item.KindRecurrence:
				if f.Recurrence == "" {
					f.Recurrence = fd.Name
				}
			}
		}
		if len(names) == 0 {
			continue
		}
		slices.Sort(names)
		if v.Start != "" || v.End != "" {
			if slices.Contains(names, v.Start) {
				f.Start = v.Start
			}
			if slices.Contains(names, v.End) {
				f.End = v.End
			}
		} else {
			f.Start, f.End = guess(names)
		}
		if f.Start == "" && f.End == "" {
			continue
		}
		f.StartTimed, f.EndTimed = kinds[f.Start] == item.KindDateTime, kinds[f.End] == item.KindDateTime
		for _, u := range users {
			if assignees.MatchString(u) {
				f.Assignee = u
				break
			}
		}
		if f.Assignee == "" && len(users) > 0 {
			f.Assignee = users[0]
		}
		out = append(out, f)
	}
	return out
}

// quote writes a name or a value as a quoted text of the query language.
func quote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

var orderBy = regexp.MustCompile(`(?i)^ORDER\s+BY\b`)

// withoutOrder is a query without its ORDER BY, outside quoted texts.
func withoutOrder(q string) string {
	var quoteChar byte
	for i := 0; i < len(q); i++ {
		c := q[i]
		switch {
		case quoteChar != 0:
			if c == '\\' {
				i++
			} else if c == quoteChar {
				quoteChar = 0
			}
		case c == '"' || c == '\'':
			quoteChar = c
		case (i == 0 || strings.ContainsRune(" \t\r\n)", rune(q[i-1]))) && orderBy.MatchString(q[i:]):
			return strings.TrimSpace(q[:i])
		}
	}
	return strings.TrimSpace(q)
}

// day writes a day of the query language.
func day(t time.Time) string { return t.UTC().Format(time.DateOnly) }

// Query is the query that reads the items of the days from to to, both
// included, as the browser's calendar writes it (calendar.js listQuery),
// with the state, words, labels and assignee chosen and the advanced query.
// A repeating item is read whenever it began by the last day; its
// occurrences are worked out afterwards. It is empty when no type has a
// date field to place items by.
func Query(v View, fields []Fields, from, to time.Time) string {
	first := func(timed bool) string {
		if timed {
			return day(from.AddDate(0, 0, -1))
		}
		return day(from)
	}
	last := func(timed bool) string {
		if timed {
			return day(to.AddDate(0, 0, 1))
		}
		return day(to)
	}
	within := func(name string, timed bool) string {
		return quote(name) + " >= " + first(timed) + " AND " + quote(name) + " <= " + last(timed)
	}
	whom := map[string]string{"me": "= currentUser()", "none": "IS EMPTY"}[v.Who]
	var period []string
	for _, f := range fields {
		if whom != "" && f.Assignee == "" {
			continue
		}
		var cond string
		if f.Start != "" && f.End != "" {
			cond = "(" + quote(f.Start) + " <= " + last(f.StartTimed) + " AND " + quote(f.End) + " >= " + first(f.EndTimed) + ") OR (" +
				within(f.Start, f.StartTimed) + ") OR (" + within(f.End, f.EndTimed) + ")"
		} else if f.Start != "" {
			cond = within(f.Start, f.StartTimed)
		} else {
			cond = within(f.End, f.EndTimed)
		}
		if f.Recurrence != "" {
			anchor, timed := f.Start, f.StartTimed
			if anchor == "" {
				anchor, timed = f.End, f.EndTimed
			}
			cond = "(" + cond + ") OR (" + quote(f.Recurrence) + " IS NOT EMPTY AND " + quote(anchor) + " <= " + last(timed) + ")"
		}
		who := ""
		if whom != "" {
			who = " AND " + quote(f.Assignee) + " " + whom
		}
		period = append(period, "(type = "+quote(f.Type)+who+" AND ("+cond+"))")
	}
	if len(period) == 0 {
		return ""
	}
	var conds []string
	switch v.Status {
	case "open":
		conds = append(conds, "finished != true")
	case "done":
		conds = append(conds, "finished = true")
	}
	if v.Text != "" {
		conds = append(conds, "text ~ "+quote(v.Text))
	}
	if len(v.Labels) > 0 {
		quoted := make([]string, len(v.Labels))
		for i, l := range v.Labels {
			quoted[i] = quote(l)
		}
		conds = append(conds, "label IN ("+strings.Join(quoted, ", ")+")")
	}
	if own := withoutOrder(v.Q); own != "" {
		conds = append(conds, "("+own+")")
	}
	conds = append(conds, "("+strings.Join(period, " OR ")+")")
	return strings.Join(conds, " AND ")
}

// Event is one entry of a feed: an item, or one occurrence of a repeating
// item.
type Event struct {
	Item       item.Item
	Occurrence string // the day an occurrence falls on by its rule, or empty
	AllDay     bool
	Start, End time.Time // for a day: its first and its last day, both included
}

// Events are the entries of the items that touch the stretch of time from
// (included) to to (excluded), each placed by its type's fields. An item
// with both dates spans them; with one, or with an end before its start,
// it stands on the one it has (the end, when it has both). A repeating item
// gives an entry per occurrence in the stretch.
func Events(items []item.Item, fields []Fields, from, to time.Time) []Event {
	byType := map[string]Fields{}
	for _, f := range fields {
		byType[f.Type] = f
	}
	var out []Event
	for _, it := range items {
		f, ok := byType[it.Type]
		if !ok {
			continue
		}
		start, hasStart := it.Properties[f.Start].(time.Time)
		end, hasEnd := it.Properties[f.End].(time.Time)
		startTimed, endTimed := f.StartTimed, f.EndTimed
		switch {
		case hasStart && hasEnd && !end.Before(start):
		case hasEnd:
			start, startTimed, hasStart = end, endTimed, true
			end, hasEnd = time.Time{}, false
		case hasStart:
		default:
			continue
		}
		timed := startTimed || (hasEnd && endTimed)
		if !timed {
			start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
			if hasEnd {
				end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
			}
		}
		rule, _ := it.Properties[f.Recurrence].(string)
		if r, err := item.ParseRecurrence(rule); f.Recurrence != "" && rule != "" && err == nil {
			for _, o := range r.Expand(start, end, timed, from, to) {
				out = append(out, Event{Item: it, Occurrence: o.Original, AllDay: !timed, Start: o.Start, End: o.End})
			}
			continue
		}
		if !hasEnd {
			end = start
		}
		if !start.Before(to) || end.Before(from) {
			continue
		}
		out = append(out, Event{Item: it, AllDay: !timed, Start: start, End: end})
	}
	slices.SortStableFunc(out, func(a, b Event) int {
		return a.Start.Compare(b.Start)
	})
	return out
}
