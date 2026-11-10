package server

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// The album's search also finds pictures by the day they were taken, or
// were added when they tell no time (item.Photo.Day): a year, a month or a
// day, written as people write them in English and German. A query that
// is a date matches by date as well as by its words.

// monthNames are the names and short forms of the months, English and
// German, lower case.
var monthNames = func() map[string]int {
	m := map[string]int{}
	for i, names := range [][]string{
		{"january", "jan", "januar", "jänner", "jaenner"},
		{"february", "feb", "februar"},
		{"march", "mar", "märz", "maerz", "marz", "mär", "mrz"},
		{"april", "apr"},
		{"may", "mai"},
		{"june", "jun", "juni"},
		{"july", "jul", "juli"},
		{"august", "aug"},
		{"september", "sep", "sept"},
		{"october", "oct", "oktober", "okt"},
		{"november", "nov"},
		{"december", "dec", "dezember", "dez"},
	} {
		for _, n := range names {
			m[n] = i + 1
		}
	}
	return m
}()

var (
	yearOnlyRE   = regexp.MustCompile(`^(\d{4})$`)
	isoMonthRE   = regexp.MustCompile(`^(\d{4})[-/.](\d{1,2})$`)
	isoDayRE     = regexp.MustCompile(`^(\d{4})-(\d{1,2})-(\d{1,2})$`)
	dottedDayRE  = regexp.MustCompile(`^(\d{1,2})\.(\d{1,2})\.(\d{4}|\d{2})$`)
	dottedMonRE  = regexp.MustCompile(`^(\d{1,2})[./](\d{4})$`)
	everyDayRE   = regexp.MustCompile(`^(\d{1,2})\.(\d{1,2})\.?$`)
	namedMonthRE = regexp.MustCompile(`^(\p{L}+)\.?(?: (\d{4}))?$`)
	namedDayRE   = regexp.MustCompile(`^(\d{1,2})\.? (\p{L}+)\.?(?: (\d{4}))?$`)
	enDayRE      = regexp.MustCompile(`^(\p{L}+)\.? (\d{1,2}),? (\d{4})$`)
)

// dateQuery reads a search as a date. It answers a test of a day,
// "2006-01-02", or nil when the search is no date. A year, a month of a
// year and a day match those; a month or a day without a year matches it
// in every year.
func dateQuery(q string) func(day string) bool {
	q = strings.Join(strings.Fields(strings.ToLower(q)), " ")
	num := func(s string) int { n, _ := strconv.Atoi(s); return n }
	year := func(s string) (int, bool) {
		if s == "" {
			return 0, true // every year
		}
		y := num(s)
		if len(s) == 2 {
			y += 2000
		}
		return y, y >= 1800 && y <= 2200
	}
	month := func(name string) (int, bool) { m, ok := monthNames[name]; return m, ok }
	switch {
	case yearOnlyRE.MatchString(q):
		if y, ok := year(q); ok {
			return byPrefix(fmt.Sprintf("%04d-", y))
		}
	case isoDayRE.MatchString(q):
		m := isoDayRE.FindStringSubmatch(q)
		return byDay(num(m[1]), num(m[2]), num(m[3]))
	case isoMonthRE.MatchString(q):
		m := isoMonthRE.FindStringSubmatch(q)
		return byMonth(num(m[1]), num(m[2]))
	case dottedDayRE.MatchString(q):
		m := dottedDayRE.FindStringSubmatch(q)
		if y, ok := year(m[3]); ok {
			return byDay(y, num(m[2]), num(m[1]))
		}
	case dottedMonRE.MatchString(q):
		m := dottedMonRE.FindStringSubmatch(q)
		return byMonth(num(m[2]), num(m[1]))
	case everyDayRE.MatchString(q):
		m := everyDayRE.FindStringSubmatch(q)
		return byDay(0, num(m[2]), num(m[1]))
	case namedMonthRE.MatchString(q):
		m := namedMonthRE.FindStringSubmatch(q)
		mo, ok := month(m[1])
		y, valid := year(m[2])
		if ok && valid {
			return byMonth(y, mo)
		}
	case namedDayRE.MatchString(q):
		m := namedDayRE.FindStringSubmatch(q)
		mo, ok := month(m[2])
		y, valid := year(m[3])
		if ok && valid {
			return byDay(y, mo, num(m[1]))
		}
	case enDayRE.MatchString(q):
		m := enDayRE.FindStringSubmatch(q)
		mo, ok := month(m[1])
		y, valid := year(m[3])
		if ok && valid {
			return byDay(y, mo, num(m[2]))
		}
	}
	return nil
}

func byPrefix(p string) func(string) bool {
	return func(day string) bool { return strings.HasPrefix(day, p) }
}

// byMonth matches a month of a year, or of every year when year is 0.
func byMonth(year, month int) func(string) bool {
	if month < 1 || month > 12 || (year != 0 && (year < 1800 || year > 2200)) {
		return nil
	}
	if year == 0 {
		mm := fmt.Sprintf("-%02d-", month)
		return func(day string) bool { return len(day) == 10 && day[4:8] == mm }
	}
	return byPrefix(fmt.Sprintf("%04d-%02d-", year, month))
}

// byDay matches a day of a year, or that day in every year when year is
// 0. A day the calendar does not have, such as 31 April, is no date.
func byDay(year, month, day int) func(string) bool {
	check := year
	if check == 0 {
		check = 2000 // a leap year, so 29 February is a day
	}
	t := time.Date(check, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if month < 1 || month > 12 || t.Day() != day || int(t.Month()) != month || (year != 0 && (year < 1800 || year > 2200)) {
		return nil
	}
	if year == 0 {
		md := fmt.Sprintf("%02d-%02d", month, day)
		return func(d string) bool { return len(d) == 10 && d[5:] == md }
	}
	want := fmt.Sprintf("%04d-%02d-%02d", year, month, day)
	return func(d string) bool { return d == want }
}
