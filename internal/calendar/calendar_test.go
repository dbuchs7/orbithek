package calendar

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/dbuchs7/orbithek/internal/item"
)

var types = []item.TypeDef{
	{Name: "task", Fields: []item.FieldDef{{Name: "assignee", Kind: item.KindUser}, {Name: "due", Kind: item.KindDate}}},
	{Name: "epic", Fields: []item.FieldDef{{Name: "start", Kind: item.KindDate}, {Name: "due", Kind: item.KindDate}}},
	{Name: "decision", Fields: []item.FieldDef{{Name: "decided", Kind: item.KindDate}}},
	{Name: "page", Fields: []item.FieldDef{{Name: "owner", Kind: item.KindUser}}},
	{Name: "meeting", Fields: []item.FieldDef{{Name: "owner", Kind: item.KindUser}, {Name: "starts", Kind: item.KindDateTime}, {Name: "ends", Kind: item.KindDateTime}, {Name: "repeats", Kind: item.KindRecurrence}}},
}

func d(s string) time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return t
}

// The browser's calendar.test.mjs holds the same cases.
func TestFieldsAreGuessedAsTheBrowserGuessesThem(t *testing.T) {
	got := TypeFields(types, View{})
	want := []Fields{
		{Type: "task", End: "due", Assignee: "assignee"},
		{Type: "epic", Start: "start", End: "due"},
		{Type: "decision", End: "decided"},
		{Type: "meeting", Start: "starts", End: "ends", StartTimed: true, EndTimed: true, Assignee: "owner", Recurrence: "repeats"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%d: got %+v, want %+v", i, got[i], want[i])
		}
	}
	if got := TypeFields(types, View{End: "due"}); len(got) != 2 || got[0].Start != "" || got[1].Type != "epic" {
		t.Errorf("a chosen end: %+v", got)
	}
	if got := TypeFields(types, View{Type: "decision", Start: "decided"}); len(got) != 1 || got[0].Start != "decided" || got[0].End != "" {
		t.Errorf("a chosen start: %+v", got)
	}
	if s, e := guess([]string{"start"}); s != "start" || e != "" {
		t.Errorf("a start alone is a start: %q %q", s, e)
	}
}

func TestTheQueryIsWrittenAsTheBrowserWritesIt(t *testing.T) {
	fields := TypeFields(types, View{})
	task := []Fields{fields[0], fields[2]}
	v := ParseView(`?who=me&status=open&text=say+"hi"&labels=a,b,a&q=state+!%3D+done+ORDER+BY+due&space=PLAT&type=&who=boss`)
	if v.Space != "PLAT" || v.Who != "me" || v.Status != "open" || len(v.Labels) != 2 {
		t.Fatalf("view %+v", v)
	}
	got := Query(View{Who: "me", Status: "open", Text: `say "hi"`}, task, d("2026-11-09"), d("2026-11-15"))
	want := `finished != true AND text ~ "say \"hi\"" AND ((type = "task" AND "assignee" = currentUser() AND ("due" >= 2026-11-09 AND "due" <= 2026-11-15)))`
	if got != want {
		t.Errorf("query\n got  %s\n want %s", got, want)
	}
	got = Query(View{Status: "done", Labels: []string{"plan-q1"}, Q: `title ~ "order by" order by key`}, fields[1:2], d("2026-11-09"), d("2026-11-15"))
	want = `finished = true AND label IN ("plan-q1") AND (title ~ "order by") AND ((type = "epic" AND (("start" <= 2026-11-15 AND "due" >= 2026-11-09) OR ("start" >= 2026-11-09 AND "start" <= 2026-11-15) OR ("due" >= 2026-11-09 AND "due" <= 2026-11-15))))`
	if got != want {
		t.Errorf("query\n got  %s\n want %s", got, want)
	}
	// A repeating type is read whenever it began; a point in time a day
	// wider on either side.
	got = Query(View{Who: "none"}, fields[3:], d("2026-11-09"), d("2026-11-15"))
	want = `((type = "meeting" AND "owner" IS EMPTY AND ((("starts" <= 2026-11-16 AND "ends" >= 2026-11-08) OR ("starts" >= 2026-11-08 AND "starts" <= 2026-11-16) OR ("ends" >= 2026-11-08 AND "ends" <= 2026-11-16)) OR ("repeats" IS NOT EMPTY AND "starts" <= 2026-11-16))))`
	if got != want {
		t.Errorf("query\n got  %s\n want %s", got, want)
	}
	if Query(View{Who: "me"}, fields[2:3], d("2026-11-09"), d("2026-11-15")) != "" {
		t.Error("no type assigned by a field: nothing to read")
	}
	if withoutOrder("ORDER BY key") != "" || withoutOrder("(a = 1)ORDER BY key") != "(a = 1)" || withoutOrder("border = x") != "border = x" {
		t.Error("withoutOrder")
	}
}

func TestEventsOfItemsAndOccurrences(t *testing.T) {
	fields := TypeFields(types, View{})
	at := func(s string) time.Time { v, _ := time.Parse(time.RFC3339, s); return v }
	items := []item.Item{
		{ID: "a", Key: "PLAT-1", Type: "task", Title: "Due", Properties: item.Props{"due": d("2026-11-10")}},
		{ID: "b", Key: "PLAT-2", Type: "epic", Title: "Span", Properties: item.Props{"start": d("2026-11-08"), "due": d("2026-11-12")}},
		{ID: "c", Key: "PLAT-3", Type: "epic", Title: "Inverted", Properties: item.Props{"start": d("2026-11-20"), "due": d("2026-11-18")}},
		{ID: "e", Key: "PLAT-5", Type: "meeting", Title: "Standup", Properties: item.Props{"starts": at("2026-11-09T08:00:00Z"), "repeats": "FREQ=DAILY;COUNT=2"}},
		{ID: "f", Key: "PLAT-6", Type: "page", Title: "No dates"},
		{ID: "g", Key: "PLAT-7", Type: "decision", Title: "Long ago", Properties: item.Props{"decided": d("2020-01-01")}},
		{ID: "h", Key: "PLAT-8", Type: "meeting", Title: "Broken rule", Properties: item.Props{"starts": at("2026-11-11T08:00:00Z"), "repeats": "FREQ=NEVER"}},
	}
	events := Events(items, fields, d("2026-11-01"), d("2026-12-01"))
	var got []string
	for _, e := range events {
		got = append(got, e.Item.Key+" "+e.Occurrence+" "+e.Start.UTC().Format(time.RFC3339)+" "+e.End.UTC().Format(time.RFC3339))
	}
	want := []string{
		"PLAT-2  2026-11-08T00:00:00Z 2026-11-12T00:00:00Z",
		"PLAT-5 2026-11-09 2026-11-09T08:00:00Z 2026-11-09T08:00:00Z",
		"PLAT-1  2026-11-10T00:00:00Z 2026-11-10T00:00:00Z",
		"PLAT-5 2026-11-10 2026-11-10T08:00:00Z 2026-11-10T08:00:00Z",
		"PLAT-8  2026-11-11T08:00:00Z 2026-11-11T08:00:00Z",
		"PLAT-3  2026-11-18T00:00:00Z 2026-11-18T00:00:00Z",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("events\n got  %q\n want %q", got, want)
	}

	feed := Feed("Mine; week", "https://wiki.example.org/", events, at("2026-11-09T10:00:00Z"))
	if !bytes.Contains(feed, []byte("X-WR-CALNAME:Mine\\; week\r\n")) || !bytes.Contains(feed, []byte("UID:e-20261110@wiki.example.org\r\n")) ||
		!bytes.Contains(feed, []byte("DTEND:20261109T090000Z\r\n")) || !bytes.Contains(feed, []byte("DTEND;VALUE=DATE:20261113\r\n")) {
		t.Errorf("feed:\n%s", feed)
	}
	if !bytes.Contains(Feed("x", "", nil, time.Now()), []byte("BEGIN:VCALENDAR")) {
		t.Error("an empty feed is a calendar")
	}
}

func TestLongLinesAreFoldedWithoutSplittingACharacter(t *testing.T) {
	var b bytes.Buffer
	fold(&b, "SUMMARY:"+strings.Repeat("ä", 60))
	lines := strings.Split(strings.TrimSuffix(b.String(), "\r\n"), "\r\n")
	if len(lines) < 2 {
		t.Fatalf("not folded: %q", b.String())
	}
	joined := lines[0]
	for i, l := range lines {
		if len(l) > 75 {
			t.Errorf("line %d has %d octets", i, len(l))
		}
		if i > 0 {
			if !strings.HasPrefix(l, " ") {
				t.Errorf("a folded line starts without a space: %q", l)
			}
			joined += l[1:]
		}
	}
	if joined != "SUMMARY:"+strings.Repeat("ä", 60) {
		t.Errorf("unfolded %q", joined)
	}
	if escape("a\\b;c,d\ne\r\nf") != `a\\b\;c\,d\ne\nf` {
		t.Errorf("escape %q", escape("a\\b;c,d\ne\r\nf"))
	}
}
