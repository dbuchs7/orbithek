package item

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

// TestPhotoDataIsChecked: the model takes photo data a picture could
// tell and refuses the rest (ADR-0177).
func TestPhotoDataIsChecked(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	good := []*Photo{nil, {}, {Taken: "2024-05-01T13:22:10", Offset: "+02:00", Lat: f(46.5), Lon: f(-7.8), Alt: f(-30), Make: "Maker", Exposure: "1/250", FNumber: 2.8, ISO: 100, Focal: 35, Focal35: 52, Orientation: 8, Width: 10, Height: 5}}
	for _, p := range good {
		if !ValidPhoto(p) {
			t.Errorf("refused %+v: %v", p, checkPhoto(p))
		}
	}
	bad := []Photo{
		{Taken: "yesterday"}, {Offset: "+02:00"}, {Taken: "2024-05-01T13:22:10", Offset: "02:00"}, {Taken: "2024-05-01T13:22:10", Offset: "+15:00"},
		{Lat: f(1)}, {Lat: f(91), Lon: f(0)}, {Lat: f(0), Lon: f(181)}, {Lat: f(math.NaN()), Lon: f(0)}, {Alt: f(1)}, {Lat: f(1), Lon: f(1), Alt: f(math.Inf(1))},
		{Make: strings.Repeat("x", 65)}, {Model: "a\nb"}, {Lens: "\xff"}, {FNumber: -1}, {ISO: -1}, {Focal: math.NaN()}, {Focal35: -1},
		{Orientation: 9}, {Width: -1}, {Height: 1 << 21},
	}
	for _, p := range bad {
		if err := checkPhoto(&p); !errors.Is(err, ErrValidation) {
			t.Errorf("took %+v", p)
		}
	}
	s := setup(t)
	noErr(t, s.GrantRole("admin", "PLAT", "anna", Editor))
	anna := s.As("anna")
	folder := mustItem(t)(anna.CreateFolder("PLAT", "", "Holidays"))
	if _, err := anna.AddAttachment(folder.ID, Attachment{ID: "00112233445566778899aabbccddeeff", Name: "a.jpg", Type: "image/jpeg", Size: 1, SHA256: strings.Repeat("ab", 32), Photo: &Photo{Orientation: 12}}); !errors.Is(err, ErrValidation) {
		t.Errorf("a picture with impossible photo data was taken: %v", err)
	}
}

// TestWhenAPhotoWasTaken: with its offset a photo's time is a moment;
// without, its wall-clock time keeps its day; without a time, the day it
// was added counts.
func TestWhenAPhotoWasTaken(t *testing.T) {
	added := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC).Unix()
	at, exact := (&Photo{Taken: "2024-05-01T23:30:00", Offset: "+02:00"}).At(added)
	if !exact || !at.Equal(time.Date(2024, 5, 1, 21, 30, 0, 0, time.UTC)) || (&Photo{Taken: "2024-05-01T23:30:00", Offset: "+02:00"}).Day(added) != "2024-05-01" {
		t.Errorf("with an offset: %v %v", at, exact)
	}
	if at, exact := (&Photo{Taken: "2024-05-01T23:30:00"}).At(added); exact || at.Format(time.DateTime) != "2024-05-01 23:30:00" {
		t.Errorf("without an offset: %v %v", at, exact)
	}
	var none *Photo
	if none.Day(added) != "2026-01-02" || (&Photo{}).Day(added) != "2026-01-02" || (&Photo{Taken: "bad"}).Day(added) != "2026-01-02" {
		t.Error("without a time, the day it was added")
	}
	if at, _ := (&Photo{Taken: "2024-05-01T23:30:00", Offset: "bad"}).At(added); at.Format(time.DateTime) != "2024-05-01 23:30:00" {
		t.Errorf("a broken offset: %v", at)
	}
	lat := 1.0
	if none.HasPlace() || (&Photo{Lat: &lat}).HasPlace() || !(&Photo{Lat: &lat, Lon: &lat}).HasPlace() {
		t.Error("HasPlace")
	}
}
