// Package extcal shows a person's calendars of other services — Google
// Calendar, Apple iCloud, Outlook or any other that publishes iCalendar or
// speaks CalDAV — beside the items of the wiki's calendar (ADR-0174). It
// reads them, never writes them: what a person keeps elsewhere stays
// there, and an entry read from it is shown, not made an item.
//
// This file reads iCalendar (RFC 5545): the entries of a calendar with
// their times, repetitions and exceptions. Only what the calendar shows is
// kept: the title, the place and the time. Descriptions, attendees and
// attachments are left out.
package extcal

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	// The zone database is in the binary: the container image has none
	// (as for internal/item, ADR-0168).
	_ "time/tzdata"
)

// ErrNotICalendar is a text that is no iCalendar calendar.
var ErrNotICalendar = errors.New("extcal: not an iCalendar calendar")

// maxEntries is how many entries one calendar may hold; a larger one is
// read up to it. maxLine is the longest unfolded line read.
const (
	maxEntries = 20000
	maxLine    = 64 << 10
)

// Calendar is what is read from an iCalendar text.
type Calendar struct {
	// Name is the calendar's own name (X-WR-CALNAME), if it gives one.
	Name string
	// Zone is the calendar's own time zone (X-WR-TIMEZONE), for times it
	// gives without a zone; nil if it names none.
	Zone    *time.Location
	entries []entry
}

// entry is one VEVENT: an entry, the rule by which it repeats, or the
// change of one of its occurrences (RECURRENCE-ID).
type entry struct {
	uid, title, location string
	start, end           value
	duration             time.Duration
	durationDays         int
	hasEnd, hasDuration  bool
	rrule                string
	exdates, rdates      []value
	recurrenceID         *value
	cancelled            bool
}

// value is a DATE or a DATE-TIME as the text gives it: a date, a time in
// UTC, a time in a named zone, or a floating time, meant where the reader
// is.
type value struct {
	date     bool
	utc      bool
	zone     *time.Location
	y, mo, d int
	h, mi, s int
}

// property is one content line: its name, parameters and value.
type property struct {
	name   string
	params map[string]string
	value  string
}

// Parse reads an iCalendar text. Lines it cannot read are skipped, so one
// odd entry does not hide a whole calendar; a text without a VCALENDAR is
// refused.
func Parse(data []byte) (*Calendar, error) {
	lines, err := unfold(data)
	if err != nil {
		return nil, err
	}
	cal := &Calendar{}
	zones := map[string]*time.Location{}
	var (
		inCalendar bool
		stack      []string
		cur        *entry
		tz         *vtimezone
	)
	for _, line := range lines {
		p, ok := parseLine(line)
		if !ok {
			continue
		}
		switch p.name {
		case "BEGIN":
			comp := strings.ToUpper(p.value)
			stack = append(stack, comp)
			switch {
			case comp == "VCALENDAR":
				inCalendar = true
			case comp == "VEVENT" && len(stack) == 2 && len(cal.entries) < maxEntries:
				cur = &entry{}
			case comp == "VTIMEZONE" && len(stack) == 2:
				tz = &vtimezone{}
			case (comp == "STANDARD" || comp == "DAYLIGHT") && tz != nil:
				tz.part = comp
			}
			continue
		case "END":
			// An END that does not close the open component is skipped.
			if len(stack) == 0 || stack[len(stack)-1] != strings.ToUpper(p.value) {
				continue
			}
			comp := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			switch {
			case comp == "VEVENT" && cur != nil && len(stack) == 1:
				cal.entries = append(cal.entries, *cur)
				cur = nil
			case comp == "VTIMEZONE" && tz != nil:
				if loc := tz.location(); loc != nil && tz.id != "" {
					zones[tz.id] = loc
				}
				tz = nil
			case comp == "STANDARD" || comp == "DAYLIGHT":
				if tz != nil {
					tz.part = ""
				}
			}
			continue
		}
		depth := len(stack)
		switch {
		case depth == 1 && stack[0] == "VCALENDAR":
			switch p.name {
			case "X-WR-CALNAME":
				cal.Name = unescape(p.value)
			case "X-WR-TIMEZONE":
				cal.Zone = Zone(p.value, nil)
			}
		case depth == 2 && stack[1] == "VTIMEZONE" && tz != nil && p.name == "TZID":
			tz.id = p.value
		case depth == 3 && tz != nil && tz.part == "STANDARD" && p.name == "TZOFFSETTO":
			tz.standard = p.value
		case depth == 2 && stack[1] == "VEVENT" && cur != nil:
			cur.set(p, zones)
		}
	}
	if !inCalendar {
		return nil, ErrNotICalendar
	}
	return cal, nil
}

// set takes one property of a VEVENT. Zones defined later in the text
// than the entry are looked up when it is expanded; most calendars define
// them first.
func (e *entry) set(p property, zones map[string]*time.Location) {
	switch p.name {
	case "UID":
		e.uid = p.value
	case "SUMMARY":
		e.title = unescape(p.value)
	case "LOCATION":
		e.location = unescape(p.value)
	case "STATUS":
		e.cancelled = strings.EqualFold(p.value, "CANCELLED")
	case "DTSTART":
		if v, err := parseValue(p, zones); err == nil {
			e.start = v
		}
	case "DTEND":
		if v, err := parseValue(p, zones); err == nil {
			e.end, e.hasEnd = v, true
		}
	case "DURATION":
		if d, days, err := parseDuration(p.value); err == nil {
			e.duration, e.durationDays, e.hasDuration = d, days, true
		}
	case "RRULE":
		e.rrule = p.value
	case "EXDATE", "RDATE":
		if strings.EqualFold(p.params["VALUE"], "PERIOD") {
			return
		}
		for _, part := range strings.Split(p.value, ",") {
			v, err := parseValue(property{name: p.name, params: p.params, value: part}, zones)
			if err != nil {
				continue
			}
			if p.name == "EXDATE" {
				e.exdates = append(e.exdates, v)
			} else {
				e.rdates = append(e.rdates, v)
			}
		}
	case "RECURRENCE-ID":
		if v, err := parseValue(p, zones); err == nil {
			e.recurrenceID = &v
		}
	}
}

// unfold joins folded lines (RFC 5545, 3.1): a line that begins with a
// space or a tab continues the one before.
func unfold(data []byte) ([]string, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 4096), maxLine)
	var lines []string
	var cur strings.Builder
	started := false
	for sc.Scan() {
		l := strings.TrimSuffix(sc.Text(), "\r")
		if len(l) > 0 && (l[0] == ' ' || l[0] == '\t') && started {
			if cur.Len()+len(l) <= maxLine {
				cur.WriteString(l[1:])
			}
			continue
		}
		if started {
			lines = append(lines, cur.String())
			cur.Reset()
		}
		cur.WriteString(l)
		started = true
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotICalendar, err)
	}
	if started {
		lines = append(lines, cur.String())
	}
	return lines, nil
}

// parseLine splits a content line into its name, parameters and value;
// a parameter's value may be quoted and then hold ':', ';' and ','.
func parseLine(line string) (property, bool) {
	p := property{params: map[string]string{}}
	i := strings.IndexAny(line, ";:")
	if i <= 0 {
		return p, false
	}
	p.name = strings.ToUpper(line[:i])
	rest := line[i:]
	for len(rest) > 0 && rest[0] == ';' {
		rest = rest[1:]
		eq := strings.IndexByte(rest, '=')
		if eq < 0 {
			return p, false
		}
		key := strings.ToUpper(rest[:eq])
		rest = rest[eq+1:]
		var val string
		if strings.HasPrefix(rest, `"`) {
			end := strings.IndexByte(rest[1:], '"')
			if end < 0 {
				return p, false
			}
			val, rest = rest[1:end+1], rest[end+2:]
		} else {
			end := strings.IndexAny(rest, ";:")
			if end < 0 {
				return p, false
			}
			val, rest = rest[:end], rest[end:]
		}
		p.params[key] = val
	}
	if !strings.HasPrefix(rest, ":") {
		return p, false
	}
	p.value = rest[1:]
	return p, true
}

// unescape reads a TEXT value (RFC 5545, 3.3.11).
func unescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 == len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 'n', 'N':
			b.WriteByte('\n')
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// parseValue reads a DATE or DATE-TIME with its TZID.
func parseValue(p property, zones map[string]*time.Location) (value, error) {
	s := strings.TrimSpace(p.value)
	var v value
	bad := fmt.Errorf("extcal: %s %q is no date or time", p.name, s)
	if len(s) < 8 {
		return v, bad
	}
	var err error
	num := func(a, b int) int {
		n, e := strconv.Atoi(s[a:b])
		if e != nil {
			err = e
		}
		return n
	}
	v.y, v.mo, v.d = num(0, 4), num(4, 6), num(6, 8)
	switch {
	case len(s) == 8:
		v.date = true
	case len(s) >= 15 && s[8] == 'T':
		v.h, v.mi, v.s = num(9, 11), num(11, 13), num(13, 15)
		switch {
		case len(s) == 16 && s[15] == 'Z':
			v.utc = true
		case len(s) == 15:
			if id := p.params["TZID"]; id != "" {
				v.zone = Zone(id, zones)
			}
		default:
			return v, bad
		}
	default:
		return v, bad
	}
	if err != nil || v.mo < 1 || v.mo > 12 || v.d < 1 || v.d > 31 || v.h > 23 || v.mi > 59 || v.s > 60 {
		return v, bad
	}
	return v, nil
}

// at is the value as a point in time; a floating time or a date is taken
// in floating.
func (v value) at(floating *time.Location) time.Time {
	loc := floating
	switch {
	case v.utc:
		loc = time.UTC
	case v.zone != nil && !v.date:
		loc = v.zone
	}
	return time.Date(v.y, time.Month(v.mo), v.d, v.h, v.mi, v.s, 0, loc)
}

// day is the value's date, at midnight UTC.
func (v value) day() time.Time { return time.Date(v.y, time.Month(v.mo), v.d, 0, 0, 0, 0, time.UTC) }

// parseDuration reads a DURATION (RFC 5545, 3.3.6): weeks and days are
// kept apart from hours, minutes and seconds, as a day of a date lasts a
// day whatever the clock does.
func parseDuration(s string) (time.Duration, int, error) {
	orig := s
	bad := fmt.Errorf("extcal: %q is no duration", orig)
	neg := false
	switch {
	case strings.HasPrefix(s, "-"):
		neg, s = true, s[1:]
	case strings.HasPrefix(s, "+"):
		s = s[1:]
	}
	if !strings.HasPrefix(s, "P") || len(s) < 3 {
		return 0, 0, bad
	}
	s = s[1:]
	var d time.Duration
	days, inTime, n := 0, false, ""
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			n += string(r)
			continue
		case r == 'T':
			inTime = true
			continue
		}
		v, err := strconv.Atoi(n)
		if err != nil || v > 100000 {
			return 0, 0, bad
		}
		n = ""
		switch {
		case r == 'W' && !inTime:
			days += 7 * v
		case r == 'D' && !inTime:
			days += v
		case r == 'H' && inTime:
			d += time.Duration(v) * time.Hour
		case r == 'M' && inTime:
			d += time.Duration(v) * time.Minute
		case r == 'S' && inTime:
			d += time.Duration(v) * time.Second
		default:
			return 0, 0, bad
		}
	}
	if n != "" {
		return 0, 0, bad
	}
	if neg {
		return -d, -days, nil
	}
	return d, days, nil
}

// vtimezone is a zone the text defines itself. Only its standard offset
// is kept, for a zone whose name is unknown here; a named zone (an IANA
// name, or a Windows name such as Outlook writes) is taken from the zone
// database, which knows its summer time.
type vtimezone struct {
	id, part, standard string
}

func (z *vtimezone) location() *time.Location {
	if loc := Zone(z.id, nil); loc != nil {
		return loc
	}
	off, ok := offset(z.standard)
	if !ok {
		return nil
	}
	return time.FixedZone(z.id, off)
}

// offset reads a UTC offset such as +0100 or -053000, in seconds.
func offset(s string) (int, bool) {
	if len(s) != 5 && len(s) != 7 || s[0] != '+' && s[0] != '-' {
		return 0, false
	}
	h, err1 := strconv.Atoi(s[1:3])
	m, err2 := strconv.Atoi(s[3:5])
	sec := 0
	var err3 error
	if len(s) == 7 {
		sec, err3 = strconv.Atoi(s[5:7])
	}
	if err1 != nil || err2 != nil || err3 != nil || h > 23 || m > 59 || sec > 59 {
		return 0, false
	}
	off := h*3600 + m*60 + sec
	if s[0] == '-' {
		off = -off
	}
	return off, true
}

// windowsZones maps the zone names Outlook and Exchange write to IANA
// zones, for the zones most used; another Windows name falls back to the
// standard offset the calendar's VTIMEZONE gives.
var windowsZones = map[string]string{
	"W. Europe Standard Time":        "Europe/Zurich",
	"Central Europe Standard Time":   "Europe/Budapest",
	"Central European Standard Time": "Europe/Warsaw",
	"Romance Standard Time":          "Europe/Paris",
	"GMT Standard Time":              "Europe/London",
	"Greenwich Standard Time":        "Atlantic/Reykjavik",
	"E. Europe Standard Time":        "Europe/Chisinau",
	"FLE Standard Time":              "Europe/Kiev",
	"GTB Standard Time":              "Europe/Bucharest",
	"Russian Standard Time":          "Europe/Moscow",
	"Turkey Standard Time":           "Europe/Istanbul",
	"Eastern Standard Time":          "America/New_York",
	"Central Standard Time":          "America/Chicago",
	"Mountain Standard Time":         "America/Denver",
	"Pacific Standard Time":          "America/Los_Angeles",
	"Atlantic Standard Time":         "America/Halifax",
	"E. South America Standard Time": "America/Sao_Paulo",
	"China Standard Time":            "Asia/Shanghai",
	"Tokyo Standard Time":            "Asia/Tokyo",
	"Korea Standard Time":            "Asia/Seoul",
	"India Standard Time":            "Asia/Kolkata",
	"Singapore Standard Time":        "Asia/Singapore",
	"Arabian Standard Time":          "Asia/Dubai",
	"Israel Standard Time":           "Asia/Jerusalem",
	"South Africa Standard Time":     "Africa/Johannesburg",
	"AUS Eastern Standard Time":      "Australia/Sydney",
	"New Zealand Standard Time":      "Pacific/Auckland",
	"UTC":                            "UTC",
	"Coordinated Universal Time":     "UTC",
}

// Zone resolves a zone's name: one the text defines, an IANA name, also
// behind a prefix such as /mozilla.org/20050126_1/Europe/Berlin, or a
// Windows name; nil if none is known.
func Zone(name string, zones map[string]*time.Location) *time.Location {
	name = strings.Trim(strings.TrimSpace(name), `"`)
	if name == "" {
		return nil
	}
	if loc, ok := zones[name]; ok {
		return loc
	}
	if iana, ok := windowsZones[name]; ok {
		name = iana
	}
	if loc, err := time.LoadLocation(name); err == nil && !strings.HasPrefix(name, "/") && name != "Local" {
		return loc
	}
	// A prefixed name: try its last parts, longest first.
	parts := strings.Split(strings.Trim(name, "/"), "/")
	for i := 0; i < len(parts)-1; i++ {
		if loc, err := time.LoadLocation(strings.Join(parts[i:], "/")); err == nil {
			return loc
		}
	}
	return nil
}
