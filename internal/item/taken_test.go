package item

import (
	"errors"
	"strings"
	"testing"
)

// TestATimeSetByHand: who may edit a picture's item sets when it was
// taken, with or without an offset, and goes back to the file's own; the
// rest of the photo data, a private place above all, stays; who may only
// read may not; bad times are refused (ADR-0196).
func TestATimeSetByHand(t *testing.T) {
	s := setup(t)
	noErr(t, s.GrantRole("admin", "PLAT", "anna", Editor))
	noErr(t, s.GrantRole("admin", "PLAT", "paul", Editor))
	noErr(t, s.GrantRole("admin", "PLAT", "vera", Viewer))
	anna, paul, vera := s.As("anna"), s.As("paul"), s.As("vera")
	folder := mustItem(t)(anna.CreateFolder("PLAT", "", "Scans"))
	lat, lon := 46.5584, 7.8352
	const lake, scan = "00112233445566778899aabbccddeef4", "00112233445566778899aabbccddeef5"
	_, err := anna.AddAttachment(folder.ID, Attachment{ID: lake, Name: "lake.jpg", Type: "image/jpeg", Size: 3, SHA256: strings.Repeat("f4", 32),
		Photo: &Photo{Taken: "2024-05-01T13:22:10", Offset: "+02:00", Lat: &lat, Lon: &lon, PlacePrivate: true, Make: "Canon"}})
	noErr(t, err)
	_, err = anna.AddAttachment(folder.ID, Attachment{ID: scan, Name: "scan.jpg", Type: "image/jpeg", Size: 3, SHA256: strings.Repeat("f5", 32)})
	noErr(t, err)

	noErr(t, paul.SetTaken(folder.ID, lake, "1987-06-05T04:03:02", "-03:30", nil))
	got, _ := anna.Attachment(folder.ID, lake)
	if p := got.Photo; p.Taken != "1987-06-05T04:03:02" || p.Offset != "-03:30" || !p.TakenManual || !p.HasPlace() || !p.PlacePrivate || p.Make != "Canon" {
		t.Errorf("after setting: %+v", p)
	}
	if got, _ := paul.Attachment(folder.ID, lake); got.Photo.HasPlace() || got.Photo.Taken != "1987-06-05T04:03:02" {
		t.Errorf("paul reads %+v", got.Photo)
	}
	if day := got.Photo.Day(0); day != "1987-06-05" {
		t.Errorf("the timeline puts it on %s", day)
	}
	// Without an offset, the offset the file told goes too.
	noErr(t, paul.SetTaken(folder.ID, lake, "1987-06-06T12:00:00", "", nil))
	if got, _ := anna.Attachment(folder.ID, lake); got.Photo.Offset != "" || got.Photo.Taken != "1987-06-06T12:00:00" {
		t.Errorf("without an offset: %+v", got.Photo)
	}
	// The same time again records nothing.
	n := len(s.Log())
	noErr(t, paul.SetTaken(folder.ID, lake, "1987-06-06T12:00:00", "", nil))
	if len(s.Log()) != n {
		t.Error("an unchanged time was recorded")
	}
	// Back to the file's own, as the server read it.
	noErr(t, paul.SetTaken(folder.ID, lake, "", "", &Photo{Taken: "2024-05-01T13:22:10", Offset: "+02:00"}))
	if got, _ := anna.Attachment(folder.ID, lake); got.Photo.TakenManual || got.Photo.Taken != "2024-05-01T13:22:10" || got.Photo.Offset != "+02:00" || !got.Photo.HasPlace() {
		t.Errorf("back to the file's own: %+v", got.Photo)
	}
	n = len(s.Log())
	noErr(t, paul.SetTaken(folder.ID, lake, "", "", nil))
	noErr(t, paul.SetTaken(folder.ID, scan, "", "", nil))
	if len(s.Log()) != n {
		t.Error("going back to the file's own time twice was recorded")
	}

	// A picture stored before photo data was recorded has the data read
	// from its file recorded with the time.
	noErr(t, paul.SetTaken(folder.ID, scan, "1950-12-24T18:00:00", "", &Photo{Make: "Scanner", Width: 10, Height: 20}))
	if got, _ := vera.Attachment(folder.ID, scan); got.Photo.Taken != "1950-12-24T18:00:00" || got.Photo.Make != "Scanner" || got.Photo.Width != 10 || !got.Photo.TakenManual {
		t.Errorf("the older picture: %+v", got.Photo)
	}

	if err := vera.SetTaken(folder.ID, lake, "1987-06-05T04:03:02", "", nil); !errors.Is(err, ErrForbidden) {
		t.Errorf("a reader set a time: %v", err)
	}
	for _, bad := range []struct{ taken, offset string }{
		{"1987-06-05", ""}, {"1987-06-05 04:03:02", ""}, {"1987-02-30T04:03:02", ""}, {"1700-01-01T00:00:00", ""}, {"2300-01-01T00:00:00", ""},
		{"1987-06-05T04:03:02", "+2"}, {"1987-06-05T04:03:02", "+15:00"}, {"1987-06-05T04:03:02", "Z"},
	} {
		if err := paul.SetTaken(folder.ID, scan, bad.taken, bad.offset, nil); !errors.Is(err, ErrValidation) {
			t.Errorf("%q %q: %v", bad.taken, bad.offset, err)
		}
	}
	if err := paul.SetTaken(folder.ID, "00000000000000000000000000000000", "1987-06-05T04:03:02", "", nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("no such file: %v", err)
	}
	if ValidPhoto(&Photo{TakenManual: true}) {
		t.Error("a time set by hand without a time is valid")
	}
}

// TestACopyTakesATimeSetByHand: a copy carries the time set by hand on
// its original and is accepted with its content as the server gave it,
// which differs from the original's; a copy not marked so is not
// (ADR-0196).
func TestACopyTakesATimeSetByHand(t *testing.T) {
	s := setup(t)
	noErr(t, s.GrantRole("admin", "PLAT", "anna", Editor))
	noErr(t, s.GrantRole("admin", "PLAT", "paul", Editor))
	anna, paul := s.As("anna"), s.As("paul")
	from := mustItem(t)(anna.CreateFolder("PLAT", "", "Scans"))
	to := mustItem(t)(paul.CreateFolder("PLAT", "", "Mine"))
	const scan = "00112233445566778899aabbccddeef4"
	_, err := anna.AddAttachment(from.ID, Attachment{ID: scan, Name: "scan.jpg", Type: "image/jpeg", Size: 3, SHA256: strings.Repeat("f4", 32), Photo: &Photo{Make: "Scanner"}})
	noErr(t, err)
	src := &Source{Item: from.ID, Attachment: scan}
	// Before a time is set by hand, a copy must have the original's content.
	if _, err := paul.AddAttachment(to.ID, Attachment{ID: "00112233445566778899aabbccddee10", Name: "scan.jpg", Type: "image/jpeg", Size: 3, SHA256: strings.Repeat("e9", 32), Source: src, AsGiven: true}); !errors.Is(err, ErrValidation) {
		t.Errorf("a copy with other content was taken: %v", err)
	}
	noErr(t, anna.SetTaken(from.ID, scan, "1962-07-14T15:30:00", "+01:00", nil))
	got, err := paul.AddAttachment(to.ID, Attachment{ID: "00112233445566778899aabbccddee11", Name: "scan.jpg", Type: "image/jpeg", Size: 3, SHA256: strings.Repeat("e9", 32),
		Source: src, AsGiven: true, Photo: &Photo{Make: "Scanner"}})
	noErr(t, err)
	if p := got.Photo; p.Taken != "1962-07-14T15:30:00" || p.Offset != "+01:00" || !p.TakenManual || p.Make != "Scanner" {
		t.Errorf("the copy's photo data %+v", p)
	}
	if _, err := paul.AddAttachment(to.ID, Attachment{ID: "00112233445566778899aabbccddee12", Name: "scan.jpg", Type: "image/jpeg", Size: 3, SHA256: strings.Repeat("e9", 32), Source: src}); !errors.Is(err, ErrValidation) {
		t.Errorf("a copy with other content, not made by the server, was taken: %v", err)
	}
}
