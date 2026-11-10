package site

import "testing"

// TestFaceRecognitionIsOffUntilTurnedOn: administrators turn the
// recognition of faces on and off again; an unchanged setting is not
// journalled, and the journal gives it back (ADR-0194).
func TestFaceRecognitionIsOffUntilTurnedOn(t *testing.T) {
	var records [][]byte
	s, err := New(nil, func(r []byte) error { records = append(records, r); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if s.Faces().Recognition {
		t.Fatal("on by default")
	}
	if err := s.SetFaces("admin", Faces{Recognition: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetFaces("admin", Faces{Recognition: true}); err != nil || len(records) != 1 {
		t.Fatalf("an unchanged setting is journalled: %d records, %v", len(records), err)
	}
	again, err := New(records, func([]byte) error { return nil })
	if err != nil || !again.Faces().Recognition {
		t.Fatalf("read again: %v", err)
	}
	if _, err := New([][]byte{[]byte(`{"v":1,"kind":"FacesSet"}`)}, nil); err == nil {
		t.Fatal("a faces event without its setting")
	}
}
