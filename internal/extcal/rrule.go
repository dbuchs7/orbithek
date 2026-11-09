package extcal

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Limits of expanding a repeating entry: how many periods (days, weeks,
// months or years) are walked from its start, and how many occurrences of
// one entry a window shows. A rule that repeats every day since 1970
// still ends within maxPeriods.
const (
	maxPeriods     = 100000
	maxOccurrences = 2000
)

// Event is one occurrence of an entry of an external calendar.
type Event struct {
	UID      string
	Title    string
	Location string
	// Start and End bound the occurrence; End is exclusive. For an
	// all-day occurrence they are dates at midnight UTC.
	Start, End time.Time
	AllDay     bool
}

// weekday is a BYDAY value: a weekday, every one (n 0) or the nth of its
// month or year, counted from the end when negative.
type weekday struct {
	n  int
	wd time.Weekday
}

// rule is a recurrence rule (RFC 5545, 3.3.10) as far as calendar
// programs write them. A rule with a part this reader does not know
// (BYYEARDAY, BYWEEKNO, BYHOUR and the like, or a FREQ below a day) is
// unsupported: its entry is shown at its start only, rather than on days
// it does not repeat on.
type rule struct {
	freq        string
	interval    int
	count       int
	until       *value
	byDay       []weekday
	byMonthDay  []int
	byMonth     []int
	bySetPos    []int
	wkst        time.Weekday
	unsupported bool
}

var weekdays = map[string]time.Weekday{"SU": time.Sunday, "MO": time.Monday, "TU": time.Tuesday, "WE": time.Wednesday, "TH": time.Thursday, "FR": time.Friday, "SA": time.Saturday}

// parseRule reads an RRULE.
func parseRule(s string) (rule, error) {
	r := rule{interval: 1, wkst: time.Monday}
	ints := func(v string, lo, hi int) ([]int, error) {
		var out []int
		for _, p := range strings.Split(v, ",") {
			n, err := strconv.Atoi(p)
			if err != nil || n == 0 || n < lo || n > hi {
				return nil, fmt.Errorf("extcal: %q in a rule", v)
			}
			out = append(out, n)
		}
		return out, nil
	}
	for _, part := range strings.Split(s, ";") {
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		var err error
		switch strings.ToUpper(k) {
		case "FREQ":
			r.freq = strings.ToUpper(v)
		case "INTERVAL":
			r.interval, err = strconv.Atoi(v)
			if err == nil && (r.interval < 1 || r.interval > 1000) {
				err = fmt.Errorf("extcal: interval %d", r.interval)
			}
		case "COUNT":
			r.count, err = strconv.Atoi(v)
			if err == nil && r.count < 1 {
				err = fmt.Errorf("extcal: count %d", r.count)
			}
		case "UNTIL":
			var u value
			u, err = parseValue(property{name: "UNTIL", value: v}, nil)
			r.until = &u
		case "BYDAY":
			for _, d := range strings.Split(strings.ToUpper(v), ",") {
				if len(d) < 2 {
					return r, fmt.Errorf("extcal: BYDAY %q", v)
				}
				wd, ok := weekdays[d[len(d)-2:]]
				n := 0
				if p := d[:len(d)-2]; p != "" {
					n, err = strconv.Atoi(strings.TrimPrefix(p, "+"))
					if err != nil || n == 0 || n < -53 || n > 53 {
						return r, fmt.Errorf("extcal: BYDAY %q", v)
					}
				}
				if !ok {
					return r, fmt.Errorf("extcal: BYDAY %q", v)
				}
				r.byDay = append(r.byDay, weekday{n, wd})
			}
		case "BYMONTHDAY":
			r.byMonthDay, err = ints(v, -31, 31)
		case "BYMONTH":
			r.byMonth, err = ints(v, 1, 12)
		case "BYSETPOS":
			r.bySetPos, err = ints(v, -366, 366)
		case "WKST":
			wd, ok := weekdays[strings.ToUpper(v)]
			if !ok {
				err = fmt.Errorf("extcal: WKST %q", v)
			}
			r.wkst = wd
		default:
			r.unsupported = true
		}
		if err != nil {
			return r, err
		}
	}
	switch r.freq {
	case "DAILY", "WEEKLY", "MONTHLY", "YEARLY":
	case "":
		return r, fmt.Errorf("extcal: a rule without FREQ")
	default:
		r.unsupported = true
	}
	return r, nil
}

// Events lists the occurrences that touch the window [from, to), sorted by
// their start. A time the calendar gives without a zone is taken in the
// calendar's own zone, else in reader, the zone of the person who reads
// it, else in UTC; an all-day occurrence covers its days in that zone.
func (c *Calendar) Events(from, to time.Time, reader *time.Location) []Event {
	floating := cmp.Or(c.Zone, reader, time.UTC)
	// The changed occurrences of each entry, by the occurrence they
	// replace; a changed occurrence is shown by itself, at its own time.
	changed := map[string]map[string]bool{}
	var out []Event
	for _, e := range c.entries {
		if e.recurrenceID == nil {
			continue
		}
		if changed[e.uid] == nil {
			changed[e.uid] = map[string]bool{}
		}
		changed[e.uid][key(*e.recurrenceID, floating)] = true
		if !e.cancelled {
			if ev, ok := e.occurrence(e.start, floating); ok && touches(ev, from, to, floating) {
				out = append(out, ev)
			}
		}
	}
	for _, e := range c.entries {
		if e.recurrenceID != nil || e.cancelled || e.start.y == 0 {
			continue
		}
		skip := map[string]bool{}
		for _, x := range e.exdates {
			skip[key(x, floating)] = true
		}
		n := 0
		e.each(floating, to, func(start value) bool {
			k, dk := key(start, floating), dateKey(start)
			if skip[k] || skip[dk] || changed[e.uid][k] || changed[e.uid][dk] {
				return true
			}
			ev, ok := e.occurrence(start, floating)
			if ok && touches(ev, from, to, floating) {
				out = append(out, ev)
				n++
			}
			return n < maxOccurrences
		})
	}
	slices.SortStableFunc(out, func(a, b Event) int {
		return cmp.Or(a.Start.Compare(b.Start), cmp.Compare(a.Title, b.Title))
	})
	return out
}

// key names an occurrence by its start: a date by its date, a time by its
// instant; an exception given as a date matches a time on that date.
func key(v value, floating *time.Location) string {
	if v.date {
		return dateKey(v)
	}
	return "T" + strconv.FormatInt(v.at(floating).Unix(), 10)
}

func dateKey(v value) string { return fmt.Sprintf("D%04d%02d%02d", v.y, v.mo, v.d) }

// occurrence is the entry at start, lasting as long as the entry does.
func (e entry) occurrence(start value, floating *time.Location) (Event, bool) {
	ev := Event{UID: e.uid, Title: e.title, Location: e.location, AllDay: start.date}
	if start.y == 0 {
		return ev, false
	}
	if start.date {
		days := 1
		switch {
		case e.hasEnd && e.end.date:
			days = int(e.end.day().Sub(e.start.day()).Hours() / 24)
		case e.hasDuration:
			days = e.durationDays
		}
		ev.Start = start.day()
		ev.End = ev.Start.AddDate(0, 0, max(days, 1))
		return ev, true
	}
	ev.Start = start.at(floating)
	switch {
	case e.hasEnd:
		ev.End = ev.Start.Add(max(e.end.at(floating).Sub(e.start.at(floating)), 0))
	case e.hasDuration:
		ev.End = ev.Start.AddDate(0, 0, e.durationDays).Add(e.duration)
		if ev.End.Before(ev.Start) {
			ev.End = ev.Start
		}
	default:
		ev.End = ev.Start
	}
	return ev, true
}

// touches reports whether an occurrence lies in the window, or reaches
// into it; one without length must start in it.
func touches(ev Event, from, to time.Time, floating *time.Location) bool {
	start, end := ev.Start, ev.End
	if ev.AllDay {
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, floating)
		end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, floating)
	}
	if !end.After(start) {
		return !start.Before(from) && start.Before(to)
	}
	return start.Before(to) && end.After(from)
}

// each calls fn with the starts of the entry's occurrences, in order,
// from its start until one begins at or after to, the rule ends, or fn
// returns false: its start, those of its rule, and the extra dates it
// names (RDATE).
func (e entry) each(floating *time.Location, to time.Time, fn func(value) bool) {
	var starts []value
	for _, r := range e.rdates {
		if r.date == e.start.date {
			starts = append(starts, r)
		}
	}
	slices.SortFunc(starts, func(a, b value) int { return a.at(floating).Compare(b.at(floating)) })
	emit := func(v value) bool {
		for len(starts) > 0 && starts[0].at(floating).Before(v.at(floating)) {
			if !starts[0].at(floating).Before(to) || !fn(starts[0]) {
				return false
			}
			starts = starts[1:]
		}
		return fn(v)
	}
	defer func() {
		for _, r := range starts {
			if !r.at(floating).Before(to) || !fn(r) {
				return
			}
		}
	}()
	r, err := parseRule(e.rrule)
	if e.rrule == "" || err != nil || r.unsupported {
		if e.start.at(floating).Before(to) {
			emit(e.start)
		}
		return
	}
	r.expand(e.start, floating, to, emit)
}

// expand walks the rule's periods from start and emits each occurrence's
// start (RFC 5545, 3.3.10 and 3.8.5.3): the start itself first, as it is
// always the first occurrence, then the days of each period the BY parts
// choose, at the start's time of day in the start's zone.
func (r rule) expand(start value, floating *time.Location, to time.Time, emit func(value) bool) {
	first := start.at(floating)
	loc := first.Location()
	base := start.day()
	at := func(d time.Time) value {
		v := start
		v.y, v.mo, v.d = d.Year(), int(d.Month()), d.Day()
		return v
	}
	var until time.Time
	untilDate := false
	if r.until != nil {
		untilDate = r.until.date
		if untilDate {
			until = r.until.day()
		} else {
			until = r.until.at(loc)
		}
	}
	after := func(v value) bool {
		switch {
		case r.until == nil:
			return false
		case untilDate:
			return v.day().After(until)
		default:
			return v.at(floating).After(until)
		}
	}
	count := 1
	if !first.Before(to) || !emit(start) {
		return
	}
	for i := 0; i < maxPeriods; i++ {
		days, periodStart := r.period(base, i)
		if !periodStart.Before(time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)) {
			return
		}
		for _, d := range days {
			v := at(d)
			t := v.at(floating)
			if !t.After(first) {
				continue
			}
			if after(v) || !t.Before(to) {
				return
			}
			count++
			if r.count > 0 && count > r.count {
				return
			}
			if !emit(v) {
				return
			}
		}
	}
}

// period answers the days the rule chooses in its ith period after the
// one of base, sorted, and the first day of that period.
func (r rule) period(base time.Time, i int) ([]time.Time, time.Time) {
	var days []time.Time
	var first time.Time
	switch r.freq {
	case "DAILY":
		d := base.AddDate(0, 0, i*r.interval)
		first = d
		if r.inMonth(d) && r.onMonthDay(d) && r.onWeekday(d) {
			days = append(days, d)
		}
	case "WEEKLY":
		off := (int(base.Weekday()) - int(r.wkst) + 7) % 7
		ws := base.AddDate(0, 0, -off+7*i*r.interval)
		first = ws
		for k := range 7 {
			d := ws.AddDate(0, 0, k)
			ok := d.Weekday() == base.Weekday()
			if len(r.byDay) > 0 {
				ok = r.onWeekday(d)
			}
			if ok && r.inMonth(d) {
				days = append(days, d)
			}
		}
	case "MONTHLY":
		m := time.Date(base.Year(), base.Month()+time.Month(i*r.interval), 1, 0, 0, 0, 0, time.UTC)
		first = m
		if r.inMonth(m) {
			days = r.monthDays(m, base.Day())
		}
	case "YEARLY":
		y := base.Year() + i*r.interval
		first = time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC)
		switch {
		case len(r.byMonth) == 0 && len(r.byMonthDay) == 0 && len(r.byDay) > 0:
			days = r.yearDays(y)
		case len(r.byMonth) == 0 && len(r.byMonthDay) > 0:
			for mo := 1; mo <= 12; mo++ {
				days = append(days, r.monthDays(time.Date(y, time.Month(mo), 1, 0, 0, 0, 0, time.UTC), base.Day())...)
			}
		case len(r.byMonth) == 0:
			if d := time.Date(y, base.Month(), base.Day(), 0, 0, 0, 0, time.UTC); d.Month() == base.Month() {
				days = append(days, d)
			}
		default:
			for _, mo := range r.byMonth {
				days = append(days, r.monthDays(time.Date(y, time.Month(mo), 1, 0, 0, 0, 0, time.UTC), base.Day())...)
			}
		}
	}
	slices.SortFunc(days, func(a, b time.Time) int { return a.Compare(b) })
	days = slices.CompactFunc(days, func(a, b time.Time) bool { return a.Equal(b) })
	return r.setPos(days), first
}

// monthDays are the days of the month that starts at m the rule chooses:
// by day of the month, by weekday (the nth of the month), both, or the
// day of the start's month, which a shorter month lacks.
func (r rule) monthDays(m time.Time, baseDay int) []time.Time {
	n := m.AddDate(0, 1, -1).Day()
	var byMD []int
	for _, md := range r.byMonthDay {
		d := md
		if md < 0 {
			d = n + md + 1
		}
		if d >= 1 && d <= n {
			byMD = append(byMD, d)
		}
	}
	var byWD []int
	for _, w := range r.byDay {
		var all []int
		for d := 1; d <= n; d++ {
			if time.Date(m.Year(), m.Month(), d, 0, 0, 0, 0, time.UTC).Weekday() == w.wd {
				all = append(all, d)
			}
		}
		byWD = append(byWD, nth(all, w.n)...)
	}
	var chosen []int
	switch {
	case len(r.byMonthDay) > 0 && len(r.byDay) > 0:
		for _, d := range byMD {
			if slices.Contains(byWD, d) {
				chosen = append(chosen, d)
			}
		}
	case len(r.byMonthDay) > 0:
		chosen = byMD
	case len(r.byDay) > 0:
		chosen = byWD
	case baseDay <= n:
		chosen = []int{baseDay}
	}
	out := make([]time.Time, 0, len(chosen))
	for _, d := range chosen {
		out = append(out, time.Date(m.Year(), m.Month(), d, 0, 0, 0, 0, time.UTC))
	}
	return out
}

// yearDays are the weekdays of a year a rule without BYMONTH chooses: the
// nth of the year.
func (r rule) yearDays(y int) []time.Time {
	var out []time.Time
	for _, w := range r.byDay {
		var all []int
		for d := time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC); d.Year() == y; d = d.AddDate(0, 0, 1) {
			if d.Weekday() == w.wd {
				all = append(all, d.YearDay())
			}
		}
		for _, yd := range nth(all, w.n) {
			out = append(out, time.Date(y, 1, yd, 0, 0, 0, 0, time.UTC))
		}
	}
	return out
}

// nth picks the nth of days, from the end when negative, or all of them
// for 0.
func nth(days []int, n int) []int {
	switch {
	case n == 0:
		return days
	case n > 0 && n <= len(days):
		return []int{days[n-1]}
	case n < 0 && -n <= len(days):
		return []int{days[len(days)+n]}
	}
	return nil
}

// setPos keeps the days of a period BYSETPOS names.
func (r rule) setPos(days []time.Time) []time.Time {
	if len(r.bySetPos) == 0 {
		return days
	}
	var out []time.Time
	for _, p := range r.bySetPos {
		i := p - 1
		if p < 0 {
			i = len(days) + p
		}
		if i >= 0 && i < len(days) {
			out = append(out, days[i])
		}
	}
	slices.SortFunc(out, func(a, b time.Time) int { return a.Compare(b) })
	return slices.CompactFunc(out, func(a, b time.Time) bool { return a.Equal(b) })
}

func (r rule) inMonth(d time.Time) bool {
	return len(r.byMonth) == 0 || slices.Contains(r.byMonth, int(d.Month()))
}

func (r rule) onMonthDay(d time.Time) bool {
	if len(r.byMonthDay) == 0 {
		return true
	}
	n := d.AddDate(0, 1, -d.Day()).Day()
	for _, md := range r.byMonthDay {
		if md == d.Day() || md < 0 && n+md+1 == d.Day() {
			return true
		}
	}
	return false
}

// onWeekday reports whether d falls on a weekday BYDAY names; in a daily
// or weekly rule an ordinal has no meaning and is ignored.
func (r rule) onWeekday(d time.Time) bool {
	if len(r.byDay) == 0 {
		return true
	}
	return slices.ContainsFunc(r.byDay, func(w weekday) bool { return w.wd == d.Weekday() })
}
