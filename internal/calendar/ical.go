package calendar

import (
	"bytes"
	"strings"
	"time"
	"unicode/utf8"
)

// Feed is a calendar as iCalendar (RFC 5545): one VEVENT per entry, each
// with its key and title, its time and a link back, and nothing of its
// text or comments (ADR-0167). base is the address the server names
// itself by; host names it in the entries' identifiers.
func Feed(name, base string, events []Event, now time.Time) []byte {
	host := strings.TrimPrefix(strings.TrimPrefix(base, "https://"), "http://")
	host = strings.Trim(strings.SplitN(host, "/", 2)[0], " ")
	host = strings.SplitN(host, ":", 2)[0]
	if host == "" {
		host = "orbithek"
	}
	var b bytes.Buffer
	line := func(s string) { fold(&b, s) }
	line("BEGIN:VCALENDAR")
	line("VERSION:2.0")
	line("PRODID:-//Orbithek//Calendar feed//EN")
	line("CALSCALE:GREGORIAN")
	line("METHOD:PUBLISH")
	line("X-WR-CALNAME:" + escape(name))
	line("REFRESH-INTERVAL;VALUE=DURATION:PT1H")
	line("X-PUBLISHED-TTL:PT1H")
	stamp := now.UTC().Format("20060102T150405Z")
	for _, e := range events {
		it := e.Item
		uid := string(it.ID)
		if e.Occurrence != "" {
			uid += "-" + strings.ReplaceAll(e.Occurrence, "-", "")
		}
		link := strings.TrimRight(base, "/") + "/#/item/" + it.Key
		line("BEGIN:VEVENT")
		line("UID:" + uid + "@" + host)
		line("DTSTAMP:" + stamp)
		if e.AllDay {
			// The last day is included; iCalendar's end is the day after.
			line("DTSTART;VALUE=DATE:" + e.Start.Format("20060102"))
			line("DTEND;VALUE=DATE:" + e.End.AddDate(0, 0, 1).Format("20060102"))
		} else {
			end := e.End
			if !end.After(e.Start) {
				// A point in time alone takes an hour, as on the calendar.
				end = e.Start.Add(time.Hour)
			}
			line("DTSTART:" + e.Start.UTC().Format("20060102T150405Z"))
			line("DTEND:" + end.UTC().Format("20060102T150405Z"))
		}
		line("SUMMARY:" + escape(it.Key+" "+it.Title))
		line("URL:" + link)
		line("DESCRIPTION:" + escape(link))
		line("END:VEVENT")
	}
	line("END:VCALENDAR")
	return b.Bytes()
}

// escape writes a text value of iCalendar: backslashes, semicolons and
// commas escaped, line breaks as \n.
func escape(s string) string {
	return strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\r\n", `\n`, "\n", `\n`, "\r", `\n`).Replace(s)
}

// fold writes a content line, folded at 75 octets without splitting a
// character, each line ending in CRLF.
func fold(b *bytes.Buffer, s string) {
	limit := 75
	for len(s) > limit {
		cut := limit
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}
		b.WriteString(s[:cut])
		b.WriteString("\r\n ")
		s = s[cut:]
		limit = 74 // the space that starts a folded line counts
	}
	b.WriteString(s)
	b.WriteString("\r\n")
}
