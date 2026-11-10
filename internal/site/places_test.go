package site

import "testing"

// TestThePlaceSearchIsOnUntilTurnedOff: administrators turn the place
// search of places set by hand off and on again; an unchanged setting is
// not journalled, and the journal gives it back (ADR-0180).
func TestThePlaceSearchIsOnUntilTurnedOff(t *testing.T) {
	var records [][]byte
	s, err := New(nil, func(r []byte) error { records = append(records, r); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if s.Places().SearchOff {
		t.Fatal("off by default")
	}
	if err := s.SetPlaces("admin", Places{SearchOff: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPlaces("admin", Places{SearchOff: true}); err != nil || len(records) != 1 {
		t.Fatalf("an unchanged setting is journalled: %d records, %v", len(records), err)
	}
	again, err := New(records, func([]byte) error { return nil })
	if err != nil || !again.Places().SearchOff {
		t.Fatalf("read again: %v", err)
	}
	if _, err := New([][]byte{[]byte(`{"v":1,"kind":"PlacesSet"}`)}, nil); err == nil {
		t.Fatal("a places event without its setting")
	}
}
