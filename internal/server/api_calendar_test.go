package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestSavedCalendarViewsAndTheirFeed: a person saves views, makes a feed
// for one and an anonymous calendar program reads it: the key, title, time
// and link of each entry its owner may read, a repeating item once per
// occurrence, a cancelled one left out; a new or ended feed closes the old
// address (ADR-0168).
func TestSavedCalendarViewsAndTheirFeed(t *testing.T) {
	w := newWorld(t, feedsOn)
	w.admin.expect("POST", "/api/types", `{"name":"meeting","fields":[{"name":"assignee","kind":"user"},{"name":"starts","kind":"datetime"},{"name":"ends","kind":"datetime"},{"name":"repeats","kind":"recurrence"}]}`, http.StatusCreated)
	b := w.bert
	today := time.Now().UTC()
	day := func(n int) string { return today.AddDate(0, 0, n).Format(time.DateOnly) }
	task := b.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"Review, plan; go","properties":{"priority":"low","due":"`+day(3)+`"}}`, http.StatusCreated)
	taskKey := field(task, "item", "key").(string)
	meeting := b.expect("POST", "/api/items", `{"space":"PLAT","type":"meeting","title":"Standup","properties":{"assignee":"`+w.ids["bert"]+`","starts":"`+day(1)+`T08:00:00Z","ends":"`+day(1)+`T08:15:00Z"}}`, http.StatusCreated)
	id := field(meeting, "item", "id").(string)
	// A rule is kept in its one form; a wrong one is refused.
	b.expect("PUT", "/api/items/"+id+"/properties/repeats", `{"value":"freq=daily;count=3;exdate=`+day(2)+`"}`, http.StatusOK)
	got := b.expect("GET", "/api/items/"+id, "", http.StatusOK)
	if r := field(got, "item", "properties", "repeats"); r != "FREQ=DAILY;COUNT=3;EXDATE="+day(2) {
		t.Fatalf("rule kept as %v", r)
	}
	b.expect("PUT", "/api/items/"+id+"/properties/repeats", `{"value":"FREQ=SOMETIMES"}`, http.StatusBadRequest)

	// Views: saved, listed, renamed; nobody else sees them.
	v := b.expect("POST", "/api/me/calendar-views", `{"name":"Platform","params":"space=PLAT"}`, http.StatusCreated)
	vid := field(v, "view", "id").(string)
	b.expect("POST", "/api/me/calendar-views", `{"name":"","params":""}`, http.StatusBadRequest)
	b.expect("PUT", "/api/me/calendar-views/"+vid, `{"name":"Platform team","params":"space=PLAT"}`, http.StatusOK)
	list := b.expect("GET", "/api/me/calendar-views", "", http.StatusOK)
	if views := list["views"].([]any); len(views) != 1 || views[0].(map[string]any)["name"] != "Platform team" || views[0].(map[string]any)["feed"] != false {
		t.Fatalf("views %v", views)
	}
	if views := w.vera.expect("GET", "/api/me/calendar-views", "", http.StatusOK)["views"].([]any); len(views) != 0 {
		t.Fatalf("vera sees bert's views: %v", views)
	}
	w.vera.expect("POST", "/api/me/calendar-views/"+vid+"/feed", "{}", http.StatusNotFound)

	// The feed: made once, read by anyone who holds its address.
	f := b.expect("POST", "/api/me/calendar-views/"+vid+"/feed", "{}", http.StatusCreated)
	feedURL, _ := url.Parse(f["url"].(string))
	secret := f["secret"].(string)
	if !strings.HasSuffix(feedURL.Path, "/calendar/"+secret+".ics") || field(f, "view", "feed") != true {
		t.Fatalf("feed %v", f)
	}
	anon := &client{t: t, h: b.h}
	rec := anon.do("GET", feedURL.Path, "")
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/calendar") {
		t.Fatalf("feed: %d %s %s", rec.Code, rec.Header().Get("Content-Type"), rec.Body)
	}
	ics := rec.Body.String()
	compact := strings.ReplaceAll(ics, "\r\n ", "")
	for _, want := range []string{
		"BEGIN:VCALENDAR\r\n", "X-WR-CALNAME:Platform team\r\n",
		"SUMMARY:" + taskKey + ` Review\, plan\; go` + "\r\n",
		"DTSTART;VALUE=DATE:" + strings.ReplaceAll(day(3), "-", "") + "\r\n",
		"DTEND;VALUE=DATE:" + strings.ReplaceAll(day(4), "-", "") + "\r\n",
		"URL:http://example.com/#/item/" + taskKey + "\r\n",
		"DTSTART:" + strings.ReplaceAll(day(1), "-", "") + "T080000Z\r\n",
		"DTEND:" + strings.ReplaceAll(day(1), "-", "") + "T081500Z\r\n",
		"DTSTART:" + strings.ReplaceAll(day(3), "-", "") + "T080000Z\r\n",
		"-" + strings.ReplaceAll(day(1), "-", "") + "@example.com\r\n",
		"END:VCALENDAR\r\n",
	} {
		if !strings.Contains(compact, want) {
			t.Errorf("the feed lacks %q:\n%s", want, ics)
		}
	}
	if strings.Contains(compact, "DTSTART:"+strings.ReplaceAll(day(2), "-", "")+"T080000Z") {
		t.Error("a cancelled occurrence is in the feed")
	}
	if n := strings.Count(compact, "SUMMARY:"); n != 3 {
		t.Errorf("want the task and two occurrences, got %d entries:\n%s", n, ics)
	}
	for _, l := range strings.Split(ics, "\r\n") {
		if len(l) > 75 {
			t.Errorf("a line longer than 75 octets: %q", l)
		}
	}

	// A view only shows what its owner may read: carla holds no role.
	cv := w.carla.expect("POST", "/api/me/calendar-views", `{"name":"All","params":""}`, http.StatusCreated)
	cf := w.carla.expect("POST", "/api/me/calendar-views/"+field(cv, "view", "id").(string)+"/feed", "{}", http.StatusCreated)
	cu, _ := url.Parse(cf["url"].(string))
	if body := anon.do("GET", cu.Path, "").Body.String(); strings.Contains(body, "BEGIN:VEVENT") {
		t.Fatalf("carla's feed shows what she may not read:\n%s", body)
	}

	// Only who and status narrow: assigned to bert and not finished.
	b.expect("PUT", "/api/me/calendar-views/"+vid, `{"name":"Mine","params":"who=me&status=open"}`, http.StatusOK)
	if mine := strings.ReplaceAll(anon.do("GET", feedURL.Path, "").Body.String(), "\r\n ", ""); strings.Contains(mine, taskKey) || !strings.Contains(mine, "Standup") {
		t.Fatalf("who=me shows the task without an assignee, or not the meeting:\n%s", mine)
	}
	// A saved query that no longer reads is told.
	b.expect("PUT", "/api/me/calendar-views/"+vid, `{"name":"Broken","params":"q=state%20%3D%20("}`, http.StatusOK)
	if rec := anon.do("GET", feedURL.Path, ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("a broken query: %d", rec.Code)
	}

	// A new feed closes the old address; an ended feed opens nothing.
	f2 := b.expect("POST", "/api/me/calendar-views/"+vid+"/feed", "{}", http.StatusCreated)
	if anon.do("GET", feedURL.Path, "").Code != http.StatusNotFound {
		t.Fatal("the old address still opens the feed")
	}
	u2, _ := url.Parse(f2["url"].(string))
	b.expect("DELETE", "/api/me/calendar-views/"+vid+"/feed", "", http.StatusNoContent)
	if anon.do("GET", u2.Path, "").Code != http.StatusNotFound {
		t.Fatal("an ended feed still opens")
	}
	if anon.do("GET", "/calendar/nothing.ics", "").Code != http.StatusNotFound {
		t.Fatal("an unknown secret opens something")
	}
	b.expect("POST", "/api/me/calendar-views/"+vid+"/feed", "{}", http.StatusCreated)
	b.expect("DELETE", "/api/me/calendar-views/"+vid, "", http.StatusNoContent)
	b.expect("DELETE", "/api/me/calendar-views/"+vid, "", http.StatusNotFound)
	b.expect("PUT", "/api/me/calendar-views/"+vid, `{"name":"x","params":""}`, http.StatusNotFound)
	b.expect("DELETE", "/api/me/calendar-views/"+vid+"/feed", "", http.StatusNotFound)
	b.expect("PUT", "/api/me/calendar-views/x", `{"name":`, http.StatusBadRequest)
	b.expect("POST", "/api/me/calendar-views", `{"name":`, http.StatusBadRequest)
	anon.expect("GET", "/api/me/calendar-views", "", http.StatusUnauthorized)

	// Making and ending feeds is in the audit trail.
	trail := w.admin.expect("GET", "/api/audit?limit=200", "", http.StatusOK)
	var made, ended int
	for _, e := range trail["entries"].([]any) {
		switch e.(map[string]any)["event"] {
		case "calendar-feed.create":
			made++
		case "calendar-feed.revoke":
			ended++
		}
	}
	if made != 4 || ended != 2 {
		t.Errorf("audit: %d feeds made, %d ended", made, ended)
	}
}

// feedsOn serves calendar feeds (OT_CALENDAR_FEEDS=on).
func feedsOn(d *Deps) { d.CalendarFeeds = true }

// TestCalendarFeedsTurnedOff: feeds are off unless OT_CALENDAR_FEEDS=on;
// then no feed is made and a feed made before opens nothing; views stay,
// and an old feed can still be ended (ADR-0168).
func TestCalendarFeedsTurnedOff(t *testing.T) {
	plain := newWorld(t)
	pv := plain.bert.expect("POST", "/api/me/calendar-views", `{"name":"Mine","params":""}`, http.StatusCreated)
	plain.bert.expect("POST", "/api/me/calendar-views/"+field(pv, "view", "id").(string)+"/feed", "{}", http.StatusForbidden)
	if list := plain.bert.expect("GET", "/api/me/calendar-views", "", http.StatusOK); list["feeds"] != false {
		t.Fatalf("feeds are off by default: %v", list)
	}

	on := newWorld(t, feedsOn)
	v := on.bert.expect("POST", "/api/me/calendar-views", `{"name":"Mine","params":""}`, http.StatusCreated)
	vid := field(v, "view", "id").(string)
	f := on.bert.expect("POST", "/api/me/calendar-views/"+vid+"/feed", "{}", http.StatusCreated)
	feedURL, _ := url.Parse(f["url"].(string))
	if list := on.bert.expect("GET", "/api/me/calendar-views", "", http.StatusOK); list["feeds"] != true {
		t.Fatalf("feeds are on when turned on: %v", list)
	}

	// The same data directory, served with feeds turned off.
	off := on.deps
	off.CalendarFeeds = false
	h := Handler(Routes(off), off.DB.Accounts)
	bert := &client{t: t, h: h, cookie: on.bert.cookie}
	anon := &client{t: t, h: h}
	if rec := anon.do("GET", feedURL.Path, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("a feed opens with feeds off: %d", rec.Code)
	}
	list := bert.expect("GET", "/api/me/calendar-views", "", http.StatusOK)
	if list["feeds"] != false || len(list["views"].([]any)) != 1 {
		t.Fatalf("with feeds off: %v", list)
	}
	bert.expect("POST", "/api/me/calendar-views/"+vid+"/feed", "{}", http.StatusForbidden)
	bert.expect("DELETE", "/api/me/calendar-views/"+vid+"/feed", "", http.StatusNoContent)
}
