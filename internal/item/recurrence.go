package item

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	// A repeating point in time keeps its time of day in a named time zone
	// (ADR-0168); the zones travel with the binary, which runs on images
	// without a zone database.
	_ "time/tzdata"
)

// KindRecurrence is the rule by which an item repeats, with its exceptions
// (ADR-0168). Its value is a text of parts separated by semicolons:
//
//	FREQ=DAILY|WEEKLY|MONTHLY|YEARLY   required
//	INTERVAL=n                         every n periods, 1 to 999
//	BYDAY=MO,WE,...                    the weekdays of a weekly rule
//	UNTIL=YYYY-MM-DD | COUNT=n         the last day, or how many times
//	TZID=Area/City                     the zone its time of day is kept in
//	EXDATE=YYYY-MM-DD,...              occurrences cancelled, by their day
//	MOVE=YYYY-MM-DD@YYYY-MM-DD[THH:MM] occurrences moved, from their day
//
// The parts follow iCalendar (RFC 5545) where it has them; EXDATE and MOVE
// name an occurrence by the day it would fall on.
const KindRecurrence FieldKind = "recurrence"

// Limits of a rule, so a value stays small enough for every listing.
const (
	maxInterval   = 999
	maxCount      = 1000
	maxExceptions = 500
)

// Recurrence is a parsed rule of a KindRecurrence field.
type Recurrence struct {
	Freq     string
	Interval int
	ByDay    []time.Weekday
	Until    string // a day, YYYY-MM-DD, or empty
	Count    int
	TZID     string
	ExDates  []string          // days of cancelled occurrences
	Moves    map[string]string // day of an occurrence -> its new day or day and time
}

var (
	dayRE     = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	dayTimeRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$`)
	weekdays  = map[string]time.Weekday{"MO": time.Monday, "TU": time.Tuesday, "WE": time.Wednesday, "TH": time.Thursday, "FR": time.Friday, "SA": time.Saturday, "SU": time.Sunday}
	dayNames  = map[time.Weekday]string{time.Monday: "MO", time.Tuesday: "TU", time.Wednesday: "WE", time.Thursday: "TH", time.Friday: "FR", time.Saturday: "SA", time.Sunday: "SU"}
)

// validDay reports whether s is a real day written YYYY-MM-DD.
func validDay(s string) bool {
	if !dayRE.MatchString(s) {
		return false
	}
	_, err := time.Parse(time.DateOnly, s)
	return err == nil
}

// validDayTime reports whether s is a real day and time written
// YYYY-MM-DDTHH:MM.
func validDayTime(s string) bool {
	if !dayTimeRE.MatchString(s) {
		return false
	}
	_, err := time.Parse("2006-01-02T15:04", s)
	return err == nil
}

// ParseRecurrence reads a rule, refusing what it does not know.
func ParseRecurrence(s string) (Recurrence, error) {
	r := Recurrence{Interval: 1}
	seen := map[string]bool{}
	for part := range strings.SplitSeq(strings.TrimSpace(s), ";") {
		if part == "" {
			continue
		}
		name, value, ok := strings.Cut(part, "=")
		name = strings.ToUpper(strings.TrimSpace(name))
		value = strings.TrimSpace(value)
		if !ok || value == "" {
			return r, fmt.Errorf("%q is not NAME=value", part)
		}
		if seen[name] {
			return r, fmt.Errorf("%s is given twice", name)
		}
		seen[name] = true
		switch name {
		case "FREQ":
			r.Freq = strings.ToUpper(value)
			if !slices.Contains([]string{"DAILY", "WEEKLY", "MONTHLY", "YEARLY"}, r.Freq) {
				return r, fmt.Errorf("FREQ %q is none of DAILY, WEEKLY, MONTHLY, YEARLY", value)
			}
		case "INTERVAL":
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 || n > maxInterval {
				return r, fmt.Errorf("INTERVAL %q is not a whole number from 1 to %d", value, maxInterval)
			}
			r.Interval = n
		case "BYDAY":
			for d := range strings.SplitSeq(value, ",") {
				wd, ok := weekdays[strings.ToUpper(strings.TrimSpace(d))]
				if !ok {
					return r, fmt.Errorf("BYDAY %q is not a weekday (MO to SU)", d)
				}
				if !slices.Contains(r.ByDay, wd) {
					r.ByDay = append(r.ByDay, wd)
				}
			}
			slices.SortFunc(r.ByDay, func(a, b time.Weekday) int { return (int(a)+6)%7 - (int(b)+6)%7 })
		case "UNTIL":
			if !validDay(value) {
				return r, fmt.Errorf("UNTIL %q is not a day YYYY-MM-DD", value)
			}
			r.Until = value
		case "COUNT":
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 || n > maxCount {
				return r, fmt.Errorf("COUNT %q is not a whole number from 1 to %d", value, maxCount)
			}
			r.Count = n
		case "TZID":
			if _, err := time.LoadLocation(value); err != nil || value == "Local" {
				return r, fmt.Errorf("TZID %q is no known time zone", value)
			}
			r.TZID = value
		case "EXDATE":
			for d := range strings.SplitSeq(value, ",") {
				d = strings.TrimSpace(d)
				if !validDay(d) {
					return r, fmt.Errorf("EXDATE %q is not a day YYYY-MM-DD", d)
				}
				if !slices.Contains(r.ExDates, d) {
					r.ExDates = append(r.ExDates, d)
				}
			}
		case "MOVE":
			r.Moves = map[string]string{}
			for m := range strings.SplitSeq(value, ",") {
				from, to, ok := strings.Cut(strings.TrimSpace(m), "@")
				if !ok || !validDay(from) || (!validDay(to) && !validDayTime(to)) {
					return r, fmt.Errorf("MOVE %q is not YYYY-MM-DD@YYYY-MM-DD or YYYY-MM-DD@YYYY-MM-DDTHH:MM", m)
				}
				r.Moves[from] = to
			}
		default:
			return r, fmt.Errorf("%s is not a part of a rule", name)
		}
	}
	switch {
	case r.Freq == "":
		return r, fmt.Errorf("FREQ is missing")
	case len(r.ByDay) > 0 && r.Freq != "WEEKLY":
		return r, fmt.Errorf("BYDAY belongs to a weekly rule only")
	case r.Until != "" && r.Count > 0:
		return r, fmt.Errorf("UNTIL and COUNT cannot both end a rule")
	case len(r.ExDates) > maxExceptions || len(r.Moves) > maxExceptions:
		return r, fmt.Errorf("at most %d cancelled and %d moved occurrences", maxExceptions, maxExceptions)
	}
	for from := range r.Moves {
		if slices.Contains(r.ExDates, from) {
			return r, fmt.Errorf("the occurrence of %s is both cancelled and moved", from)
		}
	}
	slices.Sort(r.ExDates)
	return r, nil
}

// String writes the rule in its one canonical form, which the API stores.
func (r Recurrence) String() string {
	parts := []string{"FREQ=" + r.Freq}
	if r.Interval > 1 {
		parts = append(parts, "INTERVAL="+strconv.Itoa(r.Interval))
	}
	if len(r.ByDay) > 0 {
		days := make([]string, len(r.ByDay))
		for i, d := range r.ByDay {
			days[i] = dayNames[d]
		}
		parts = append(parts, "BYDAY="+strings.Join(days, ","))
	}
	if r.Until != "" {
		parts = append(parts, "UNTIL="+r.Until)
	}
	if r.Count > 0 {
		parts = append(parts, "COUNT="+strconv.Itoa(r.Count))
	}
	if r.TZID != "" {
		parts = append(parts, "TZID="+r.TZID)
	}
	if len(r.ExDates) > 0 {
		parts = append(parts, "EXDATE="+strings.Join(r.ExDates, ","))
	}
	if len(r.Moves) > 0 {
		moves := make([]string, 0, len(r.Moves))
		for _, from := range slices.Sorted(maps.Keys(r.Moves)) {
			moves = append(moves, from+"@"+r.Moves[from])
		}
		parts = append(parts, "MOVE="+strings.Join(moves, ","))
	}
	return strings.Join(parts, ";")
}

// NormalRecurrence is a rule in its canonical form, or the reason it is
// none.
func NormalRecurrence(s string) (string, error) {
	r, err := ParseRecurrence(s)
	if err != nil {
		return "", err
	}
	return r.String(), nil
}

// Occurrence is one time a repeating item takes place.
type Occurrence struct {
	// Original is the day it falls on by its rule, YYYY-MM-DD, which
	// names it in EXDATE and MOVE.
	Original string
	Start    time.Time
	End      time.Time
	Moved    bool
}

// maxSteps bounds the days a rule is walked through, so no rule runs on.
const maxSteps = 100000

// Expand lists the occurrences of a series that touch the stretch of time
// from (included) to to (excluded). start and end are the item's first
// occurrence: for a series of days (timed false) both are days at midnight
// UTC and an occurrence spans as many days as the first; for a series of
// points in time they are instants, kept at their time of day in the
// rule's zone (UTC when it names none). end may be the zero time: then an
// occurrence ends where it starts.
func (r Recurrence) Expand(start, end time.Time, timed bool, from, to time.Time) []Occurrence {
	loc := time.UTC
	if timed && r.TZID != "" {
		if l, err := time.LoadLocation(r.TZID); err == nil {
			loc = l
		}
	}
	first := start.In(loc)
	if !timed {
		first = start.UTC()
	}
	length := time.Duration(0)
	if !end.IsZero() && end.After(start) {
		length = end.Sub(start)
	}
	anchor := time.Date(first.Year(), first.Month(), first.Day(), 0, 0, 0, 0, time.UTC)
	until := time.Time{}
	if r.Until != "" {
		until, _ = time.Parse(time.DateOnly, r.Until)
	}
	at := func(day time.Time, clock string) time.Time {
		h, m, s := first.Hour(), first.Minute(), first.Second()
		if clock != "" {
			if t, err := time.Parse("15:04", clock); err == nil {
				h, m, s = t.Hour(), t.Minute(), 0
			}
		}
		if !timed {
			return time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
		}
		return time.Date(day.Year(), day.Month(), day.Day(), h, m, s, 0, loc)
	}
	var out []Occurrence
	add := func(day time.Time) {
		original := day.Format(time.DateOnly)
		if slices.Contains(r.ExDates, original) {
			return
		}
		o := Occurrence{Original: original}
		if moved, ok := r.Moves[original]; ok {
			newDay, clock, _ := strings.Cut(moved, "T")
			d, err := time.Parse(time.DateOnly, newDay)
			if err != nil {
				return
			}
			o.Start, o.Moved = at(d, clock), true
		} else {
			o.Start = at(day, "")
		}
		o.End = o.Start.Add(length)
		// A day's end is the last day it covers; a time's end is when it
		// is over.
		if !o.Start.Before(to) || o.End.Before(from) || (timed && length > 0 && !o.End.After(from)) {
			return
		}
		out = append(out, o)
	}
	// Moved occurrences may land outside the walk's days; the walk ends
	// a while after the stretch ends so they are found.
	horizon := to.UTC().AddDate(1, 0, 0)
	n := 0
	for _, day := range r.days(anchor, horizon) {
		if !until.IsZero() && day.After(until) {
			break
		}
		n++
		if r.Count > 0 && n > r.Count {
			break
		}
		add(day)
	}
	slices.SortFunc(out, func(a, b Occurrence) int { return a.Start.Compare(b.Start) })
	return out
}

// days lists the days a rule falls on from its first day, in order, up to
// a day after horizon, within maxSteps.
func (r Recurrence) days(anchor, horizon time.Time) []time.Time {
	var out []time.Time
	step := r.Interval
	switch r.Freq {
	case "DAILY":
		for k := 0; k < maxSteps; k++ {
			d := anchor.AddDate(0, 0, k*step)
			if d.After(horizon) {
				break
			}
			out = append(out, d)
		}
	case "WEEKLY":
		days := r.ByDay
		if len(days) == 0 {
			days = []time.Weekday{anchor.Weekday()}
		}
		monday := anchor.AddDate(0, 0, -((int(anchor.Weekday()) + 6) % 7))
		for k := 0; k*7 < maxSteps; k++ {
			week := monday.AddDate(0, 0, 7*step*k)
			if week.After(horizon) {
				break
			}
			for _, wd := range days {
				d := week.AddDate(0, 0, (int(wd)+6)%7)
				if !d.Before(anchor) && !d.After(horizon) {
					out = append(out, d)
				}
			}
		}
	case "MONTHLY", "YEARLY":
		months := step
		if r.Freq == "YEARLY" {
			months = 12 * step
		}
		for k := 0; k < maxSteps; k++ {
			y, m := anchor.Year(), int(anchor.Month())-1+k*months
			d := time.Date(y+m/12, time.Month(m%12+1), anchor.Day(), 0, 0, 0, 0, time.UTC)
			if d.After(horizon) {
				break
			}
			// A month without the day is skipped, as iCalendar skips it.
			if d.Day() == anchor.Day() {
				out = append(out, d)
			}
		}
	}
	return out
}
