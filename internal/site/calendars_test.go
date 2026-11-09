package site

import "testing"

func TestCalendarsOfOtherServicesAreOffUntilTurnedOn(t *testing.T) {
	var records [][]byte
	s, err := New(nil, func(r []byte) error { records = append(records, r); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if s.Calendars().External {
		t.Fatal("on by default")
	}
	if err := s.SetCalendars("admin", Calendars{External: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCalendars("admin", Calendars{External: true}); err != nil || len(records) != 1 {
		t.Fatalf("an unchanged setting is journalled: %d records, %v", len(records), err)
	}
	again, err := New(records, func([]byte) error { return nil })
	if err != nil || !again.Calendars().External {
		t.Fatalf("read again: %v", err)
	}
	if _, err := New([][]byte{[]byte(`{"v":1,"kind":"CalendarsSet"}`)}, nil); err == nil {
		t.Fatal("a calendars event without its setting")
	}
}
