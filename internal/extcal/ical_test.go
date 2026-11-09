package extcal

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

func cal(t *testing.T, body string) *Calendar {
	t.Helper()
	c, err := Parse([]byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\n" + strings.ReplaceAll(strings.TrimSpace(body), "\n", "\r\n") + "\r\nEND:VCALENDAR\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func day(s string) time.Time {
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return d
}

func utc(s string) time.Time {
	d, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return d
}

// list writes occurrences as "title start end", times in UTC, all-day
// ones as dates.
func list(evs []Event) []string {
	var out []string
	for _, e := range evs {
		f := time.RFC3339
		if e.AllDay {
			f = time.DateOnly
		}
		out = append(out, e.Title+" "+e.Start.UTC().Format(f)+" "+e.End.UTC().Format(f))
	}
	return out
}

func want(t *testing.T, got []Event, w ...string) {
	t.Helper()
	if g := list(got); !slices.Equal(g, w) {
		t.Errorf("\n got  %q\n want %q", g, w)
	}
}

// TestAGoogleCalendarIsRead: a calendar as Google writes it — its name and
// zone, a zone defined in the text, folded and escaped lines, an alarm
// inside an entry, a weekly series with a cancelled day, a moved day and a
// day called off by a change of its own.
func TestAGoogleCalendarIsRead(t *testing.T) {
	c := cal(t, `
X-WR-CALNAME:Team\, Zürich
X-WR-TIMEZONE:Europe/Zurich
BEGIN:VTIMEZONE
TZID:Europe/Zurich
BEGIN:STANDARD
TZOFFSETFROM:+0200
TZOFFSETTO:+0100
DTSTART:19701025T030000
END:STANDARD
END:VTIMEZONE
BEGIN:VEVENT
UID:standup@google.com
DTSTART;TZID=Europe/Zurich:20261109T090000
DTEND;TZID=Europe/Zurich:20261109T091500
RRULE:FREQ=WEEKLY;BYDAY=MO,WE
EXDATE;TZID=Europe/Zurich:20261111T090000
SUMMARY:Stand
  up\; daily
LOCATION:Room 3\nfloor 2
BEGIN:VALARM
ACTION:DISPLAY
SUMMARY:Reminder
TRIGGER:-PT10M
END:VALARM
END:VEVENT
BEGIN:VEVENT
UID:standup@google.com
RECURRENCE-ID;TZID=Europe/Zurich:20261116T090000
DTSTART;TZID=Europe/Zurich:20261117T100000
DTEND;TZID=Europe/Zurich:20261117T101500
SUMMARY:Stand up (moved)
END:VEVENT
BEGIN:VEVENT
UID:standup@google.com
RECURRENCE-ID;TZID=Europe/Zurich:20261118T090000
DTSTART;TZID=Europe/Zurich:20261118T090000
STATUS:CANCELLED
SUMMARY:Stand up; daily
END:VEVENT`)
	if c.Name != "Team, Zürich" || c.Zone == nil || c.Zone.String() != "Europe/Zurich" {
		t.Fatalf("name %q, zone %v", c.Name, c.Zone)
	}
	evs := c.Events(utc("2026-11-09T00:00:00Z"), utc("2026-11-24T00:00:00Z"), nil)
	want(t, evs,
		"Stand up; daily 2026-11-09T08:00:00Z 2026-11-09T08:15:00Z",
		"Stand up (moved) 2026-11-17T09:00:00Z 2026-11-17T09:15:00Z",
		"Stand up; daily 2026-11-23T08:00:00Z 2026-11-23T08:15:00Z")
	if evs[0].Location != "Room 3\nfloor 2" || evs[0].UID != "standup@google.com" {
		t.Errorf("location %q", evs[0].Location)
	}
}

func TestDaysAndDurations(t *testing.T) {
	c := cal(t, `
BEGIN:VEVENT
UID:a
SUMMARY:Holiday
DTSTART;VALUE=DATE:20261224
DTEND;VALUE=DATE:20261227
END:VEVENT
BEGIN:VEVENT
UID:b
SUMMARY:Birthday
DTSTART;VALUE=DATE:20261225
END:VEVENT
BEGIN:VEVENT
UID:c
SUMMARY:Workshop
DTSTART:20261223T130000Z
DURATION:PT2H30M
END:VEVENT
BEGIN:VEVENT
UID:d
SUMMARY:Retreat
DTSTART;VALUE=DATE:20261228
DURATION:P2D
END:VEVENT
BEGIN:VEVENT
UID:e
SUMMARY:Call
DTSTART:20261222T080000Z
END:VEVENT`)
	want(t, c.Events(utc("2026-12-22T00:00:00Z"), utc("2027-01-01T00:00:00Z"), time.UTC),
		"Call 2026-12-22T08:00:00Z 2026-12-22T08:00:00Z",
		"Workshop 2026-12-23T13:00:00Z 2026-12-23T15:30:00Z",
		"Holiday 2026-12-24 2026-12-27",
		"Birthday 2026-12-25 2026-12-26",
		"Retreat 2026-12-28 2026-12-30")
	// An all-day entry covers its days where the reader is.
	zurich, _ := time.LoadLocation("Europe/Zurich")
	want(t, c.Events(utc("2026-12-26T23:30:00Z"), utc("2026-12-27T00:00:00Z"), zurich))
	want(t, c.Events(utc("2026-12-26T22:30:00Z"), utc("2026-12-26T23:00:00Z"), zurich), "Holiday 2026-12-24 2026-12-27")
}

func TestMonthlyAndYearlyRules(t *testing.T) {
	c := cal(t, `
BEGIN:VEVENT
UID:a
SUMMARY:Last Friday
DTSTART;VALUE=DATE:20260130
RRULE:FREQ=MONTHLY;BYDAY=-1FR;COUNT=4
END:VEVENT
BEGIN:VEVENT
UID:b
SUMMARY:On the 31st
DTSTART;VALUE=DATE:20260131
RRULE:FREQ=MONTHLY;UNTIL=20260531
END:VEVENT
BEGIN:VEVENT
UID:c
SUMMARY:Last workday
DTSTART;VALUE=DATE:20260130
RRULE:FREQ=MONTHLY;BYDAY=MO,TU,WE,TH,FR;BYSETPOS=-1;COUNT=3
END:VEVENT
BEGIN:VEVENT
UID:d
SUMMARY:Second Tuesday
DTSTART:20260113T170000Z
DTEND:20260113T180000Z
RRULE:FREQ=MONTHLY;INTERVAL=2;BYDAY=2TU;UNTIL=20260601T000000Z
END:VEVENT`)
	want(t, c.Events(utc("2026-01-01T00:00:00Z"), utc("2026-07-01T00:00:00Z"), time.UTC),
		"Second Tuesday 2026-01-13T17:00:00Z 2026-01-13T18:00:00Z",
		"Last Friday 2026-01-30 2026-01-31",
		"Last workday 2026-01-30 2026-01-31",
		"On the 31st 2026-01-31 2026-02-01",
		"Last Friday 2026-02-27 2026-02-28",
		"Last workday 2026-02-27 2026-02-28",
		"Second Tuesday 2026-03-10T17:00:00Z 2026-03-10T18:00:00Z",
		"Last Friday 2026-03-27 2026-03-28",
		"Last workday 2026-03-31 2026-04-01",
		"On the 31st 2026-03-31 2026-04-01",
		"Last Friday 2026-04-24 2026-04-25",
		"Second Tuesday 2026-05-12T17:00:00Z 2026-05-12T18:00:00Z",
		"On the 31st 2026-05-31 2026-06-01")

	y := cal(t, `
BEGIN:VEVENT
UID:a
SUMMARY:Leap day
DTSTART;VALUE=DATE:20280229
RRULE:FREQ=YEARLY
END:VEVENT
BEGIN:VEVENT
UID:b
SUMMARY:Summer time
DTSTART;VALUE=DATE:20270328
RRULE:FREQ=YEARLY;BYMONTH=3;BYDAY=-1SU
END:VEVENT
BEGIN:VEVENT
UID:c
SUMMARY:First Monday of the year
DTSTART;VALUE=DATE:20270104
RRULE:FREQ=YEARLY;BYDAY=1MO
END:VEVENT
BEGIN:VEVENT
UID:d
SUMMARY:Quarter
DTSTART;VALUE=DATE:20270101
RRULE:FREQ=YEARLY;BYMONTHDAY=1;BYMONTH=1,4,7,10;COUNT=3
END:VEVENT
BEGIN:VEVENT
UID:e
SUMMARY:Monthly by year
DTSTART;VALUE=DATE:20270115
RRULE:FREQ=YEARLY;BYMONTHDAY=15;COUNT=2
END:VEVENT`)
	want(t, y.Events(utc("2027-01-01T00:00:00Z"), utc("2029-01-02T00:00:00Z"), time.UTC),
		"Quarter 2027-01-01 2027-01-02",
		"First Monday of the year 2027-01-04 2027-01-05",
		"Monthly by year 2027-01-15 2027-01-16",
		"Monthly by year 2027-02-15 2027-02-16",
		"Summer time 2027-03-28 2027-03-29",
		"Quarter 2027-04-01 2027-04-02",
		"Quarter 2027-07-01 2027-07-02",
		"First Monday of the year 2028-01-03 2028-01-04",
		"Leap day 2028-02-29 2028-03-01",
		"Summer time 2028-03-26 2028-03-27",
		"First Monday of the year 2029-01-01 2029-01-02")
}

// TestWeeksDaysAndExtraDates: every other week from the week's first day,
// a daily rule counted from its start, not from the window, extra dates,
// and a time that stays on the clock across the end of summer time.
func TestWeeksDaysAndExtraDates(t *testing.T) {
	c := cal(t, `
BEGIN:VEVENT
UID:a
SUMMARY:Fortnight
DTSTART:20261103T070000Z
RRULE:FREQ=WEEKLY;INTERVAL=2;BYDAY=TU,TH;WKST=SU;COUNT=4
END:VEVENT
BEGIN:VEVENT
UID:b
SUMMARY:Daily
DTSTART;VALUE=DATE:20261101
RRULE:FREQ=DAILY;INTERVAL=3;COUNT=5;BYMONTH=11
RDATE;VALUE=DATE:20261120,20261121
END:VEVENT
BEGIN:VEVENT
UID:c
SUMMARY:Weekday
DTSTART;VALUE=DATE:20261113
RRULE:FREQ=DAILY;BYDAY=MO,FR;BYMONTHDAY=13,16,-1;UNTIL=20261130
END:VEVENT`)
	want(t, c.Events(utc("2026-11-05T00:00:00Z"), utc("2026-12-01T00:00:00Z"), time.UTC),
		"Fortnight 2026-11-05T07:00:00Z 2026-11-05T07:00:00Z",
		"Daily 2026-11-07 2026-11-08",
		"Daily 2026-11-10 2026-11-11",
		"Daily 2026-11-13 2026-11-14",
		"Weekday 2026-11-13 2026-11-14",
		"Weekday 2026-11-16 2026-11-17",
		"Fortnight 2026-11-17T07:00:00Z 2026-11-17T07:00:00Z",
		"Fortnight 2026-11-19T07:00:00Z 2026-11-19T07:00:00Z",
		"Daily 2026-11-20 2026-11-21",
		"Daily 2026-11-21 2026-11-22",
		"Weekday 2026-11-30 2026-12-01")

	d := cal(t, `
BEGIN:VEVENT
UID:a
SUMMARY:Nine
DTSTART;TZID=Europe/Zurich:20261023T090000
RRULE:FREQ=DAILY;COUNT=4;INTERVAL=2
END:VEVENT`)
	want(t, d.Events(utc("2026-10-01T00:00:00Z"), utc("2026-11-01T00:00:00Z"), nil),
		"Nine 2026-10-23T07:00:00Z 2026-10-23T07:00:00Z",
		"Nine 2026-10-25T08:00:00Z 2026-10-25T08:00:00Z",
		"Nine 2026-10-27T08:00:00Z 2026-10-27T08:00:00Z",
		"Nine 2026-10-29T08:00:00Z 2026-10-29T08:00:00Z")
}

// TestZonesOfOtherPrograms: Outlook's Windows names, a prefixed name, a
// zone known only by its own definition, a floating time where the reader
// is, and a time in a zone nobody knows, taken as floating.
func TestZonesOfOtherPrograms(t *testing.T) {
	c := cal(t, `
BEGIN:VTIMEZONE
TZID:Customized Time Zone
BEGIN:STANDARD
TZOFFSETTO:+0530
END:STANDARD
BEGIN:DAYLIGHT
TZOFFSETTO:+0630
END:DAYLIGHT
END:VTIMEZONE
BEGIN:VEVENT
UID:a
SUMMARY:Outlook
DTSTART;TZID="W. Europe Standard Time":20260715T100000
DTEND;TZID="W. Europe Standard Time":20260715T110000
END:VEVENT
BEGIN:VEVENT
UID:b
SUMMARY:Thunderbird
DTSTART;TZID=/mozilla.org/20050126_1/America/New_York:20260715T100000
END:VEVENT
BEGIN:VEVENT
UID:c
SUMMARY:Own zone
DTSTART;TZID=Customized Time Zone:20260715T100000
END:VEVENT
BEGIN:VEVENT
UID:d
SUMMARY:Floating
DTSTART:20260715T100000
END:VEVENT
BEGIN:VEVENT
UID:e
SUMMARY:Nowhere
DTSTART;TZID=Mars/Olympus:20260715T100000
END:VEVENT`)
	tokyo, _ := time.LoadLocation("Asia/Tokyo")
	want(t, c.Events(utc("2026-07-14T00:00:00Z"), utc("2026-07-16T00:00:00Z"), tokyo),
		"Floating 2026-07-15T01:00:00Z 2026-07-15T01:00:00Z",
		"Nowhere 2026-07-15T01:00:00Z 2026-07-15T01:00:00Z",
		"Own zone 2026-07-15T04:30:00Z 2026-07-15T04:30:00Z",
		"Outlook 2026-07-15T08:00:00Z 2026-07-15T09:00:00Z",
		"Thunderbird 2026-07-15T14:00:00Z 2026-07-15T14:00:00Z")
	if Zone("", nil) != nil || Zone("Local", nil) != nil || Zone("/a/b", nil) != nil {
		t.Error("an empty, local or unknown name is no zone")
	}
}

// TestWhatIsNotReadIsSkipped: a text that is no calendar is refused; a
// line, a rule or an entry that cannot be read is skipped, an unknown
// rule part shows the entry at its start only, and a cancelled entry not
// at all.
func TestWhatIsNotReadIsSkipped(t *testing.T) {
	if _, err := Parse([]byte("<html>not found</html>")); !errors.Is(err, ErrNotICalendar) {
		t.Fatalf("html: %v", err)
	}
	if _, err := Parse([]byte("BEGIN:VCALENDAR\n" + strings.Repeat("x", maxLine+1) + "\nEND:VCALENDAR")); !errors.Is(err, ErrNotICalendar) {
		t.Fatalf("an endless line: %v", err)
	}
	c := cal(t, `
garbage without a colon
X-BROKEN;PARAM="unterminated:value
END:VTODO
BEGIN:VEVENT
UID:a
SUMMARY:Hourly
DTSTART:20261109T090000Z
RRULE:FREQ=HOURLY;COUNT=5
END:VEVENT
BEGIN:VEVENT
UID:b
SUMMARY:By hour
DTSTART:20261109T100000Z
RRULE:FREQ=DAILY;BYHOUR=10,12
END:VEVENT
BEGIN:VEVENT
UID:c
SUMMARY:Broken rule
DTSTART:20261109T110000Z
RRULE:FREQ=DAILY;COUNT=zero
END:VEVENT
BEGIN:VEVENT
UID:d
SUMMARY:Cancelled
STATUS:CANCELLED
DTSTART:20261109T120000Z
END:VEVENT
BEGIN:VEVENT
UID:e
SUMMARY:No start
DTSTART:2026-11-09
END:VEVENT
BEGIN:VEVENT
UID:f
SUMMARY:Bad values
DTSTART:20261309T120000Z
DTEND:20261109T1200
DURATION:P1X
EXDATE;VALUE=PERIOD:20261109T120000Z/PT1H
EXDATE:nonsense
END:VEVENT`)
	want(t, c.Events(utc("2026-11-01T00:00:00Z"), utc("2026-12-01T00:00:00Z"), time.UTC),
		"Hourly 2026-11-09T09:00:00Z 2026-11-09T09:00:00Z",
		"By hour 2026-11-09T10:00:00Z 2026-11-09T10:00:00Z",
		"Broken rule 2026-11-09T11:00:00Z 2026-11-09T11:00:00Z")
	for _, r := range []string{"INTERVAL=2", "FREQ=DAILY;INTERVAL=0", "FREQ=DAILY;BYDAY=X", "FREQ=DAILY;BYDAY=9XMO", "FREQ=DAILY;BYDAY=XX",
		"FREQ=DAILY;BYMONTH=13", "FREQ=DAILY;WKST=XX", "FREQ=DAILY;UNTIL=soon", "FREQ=DAILY;BYDAY=0MO"} {
		if _, err := parseRule(r); err == nil {
			t.Errorf("%q is refused", r)
		}
	}
	for _, d := range []string{"P", "PT", "1H", "PT5", "P1H", "PT1D", "P999999D"} {
		if _, _, err := parseDuration(d); err == nil {
			t.Errorf("duration %q is refused", d)
		}
	}
	if d, days, err := parseDuration("-P1W2DT3H4M5S"); err != nil || days != -9 || d != -(3*time.Hour+4*time.Minute+5*time.Second) {
		t.Errorf("a negative duration: %v %d %v", d, days, err)
	}
	if _, _, err := parseDuration("+PT1H"); err != nil {
		t.Error(err)
	}
	for _, o := range []string{"0100", "+01", "+2500", "+01x0", "+010060"} {
		if _, ok := offset(o); ok {
			t.Errorf("offset %q is refused", o)
		}
	}
	if off, ok := offset("-053000"); !ok || off != -(5*3600+30*60) {
		t.Errorf("offset -053000 = %d", off)
	}
	if unescape(`a\\b\,c\Nd\`) != "a\\b,c\nd\\" {
		t.Errorf("unescape: %q", unescape(`a\\b\,c\Nd\`))
	}
}

// TestALongSeriesIsBounded: a series that never ends is walked only to
// the window, and a window shows at most maxOccurrences of one entry.
func TestALongSeriesIsBounded(t *testing.T) {
	c := cal(t, `
BEGIN:VEVENT
UID:a
SUMMARY:Forever
DTSTART;VALUE=DATE:19700101
RRULE:FREQ=DAILY
END:VEVENT`)
	if n := len(c.Events(utc("2026-01-01T00:00:00Z"), utc("2026-01-08T00:00:00Z"), time.UTC)); n != 7 {
		t.Errorf("a week of a daily series since 1970: %d", n)
	}
	if n := len(c.Events(utc("2000-01-01T00:00:00Z"), utc("2026-01-01T00:00:00Z"), time.UTC)); n != maxOccurrences {
		t.Errorf("26 years: %d", n)
	}
}
