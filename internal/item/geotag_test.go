package item

import (
	"errors"
	"strings"
	"testing"
)

// TestAPlaceSetByHand: who may edit a picture's item sets its place with a
// name or removes it; the height the file told goes, the place's privacy
// stays; a place its uploader keeps from the editor is not theirs to set;
// after a place set by hand, everyone but the uploader gets the file
// without the file's own place (ADR-0181).
func TestAPlaceSetByHand(t *testing.T) {
	s := setup(t)
	noErr(t, s.GrantRole("admin", "PLAT", "anna", Editor))
	noErr(t, s.GrantRole("admin", "PLAT", "paul", Editor))
	noErr(t, s.GrantRole("admin", "PLAT", "vera", Viewer))
	anna, paul, vera := s.As("anna"), s.As("paul"), s.As("vera")
	folder := mustItem(t)(anna.CreateFolder("PLAT", "", "Trips"))
	lat, lon, alt := 46.5584, 7.8352, 2061.0
	const lake, scan, home = "00112233445566778899aabbccddeef4", "00112233445566778899aabbccddeef5", "00112233445566778899aabbccddeef6"
	add := func(id, sha string, p *Photo) {
		t.Helper()
		_, err := anna.AddAttachment(folder.ID, Attachment{ID: ID(id), Name: id + ".jpg", Type: "image/jpeg", Size: 3, SHA256: strings.Repeat(sha, 32), Photo: p})
		noErr(t, err)
	}
	add(lake, "f4", &Photo{Taken: "2024-05-01T13:22:10", Lat: &lat, Lon: &lon, Alt: &alt})
	add(scan, "f5", nil)
	add(home, "f6", &Photo{Lat: &lat, Lon: &lon, PlacePrivate: true})

	if paul.PlaceHidden(folder.ID, lake) {
		t.Error("the file's own place counts before anything is set by hand")
	}
	zLat, zLon := 46.0207, 7.7491
	noErr(t, paul.SetPlace(folder.ID, lake, &zLat, &zLon, "  Zermatt  ", nil))
	got, _ := vera.Attachment(folder.ID, lake)
	if *got.Photo.Lat != zLat || got.Photo.PlaceName != "Zermatt" || !got.Photo.PlaceManual || got.Photo.Alt != nil || got.Photo.Taken != "2024-05-01T13:22:10" {
		t.Errorf("after setting: %+v", got.Photo)
	}
	if !vera.PlaceHidden(folder.ID, lake) || !paul.PlaceHidden(folder.ID, lake) || anna.PlaceHidden(folder.ID, lake) {
		t.Error("the file's own place must leave the file for all but the uploader")
	}
	// An older picture has its data read by the server recorded with it.
	noErr(t, paul.SetPlace(folder.ID, scan, &zLat, &zLon, "", &Photo{Taken: "1987-08-02T10:00:00"}))
	if got, _ := vera.Attachment(folder.ID, scan); got.Photo.Taken != "1987-08-02T10:00:00" || !got.Photo.HasPlace() {
		t.Errorf("the older picture: %+v", got.Photo)
	}
	// Removing it keeps the file's place out too.
	noErr(t, paul.SetPlace(folder.ID, lake, nil, nil, "ignored", nil))
	if got, _ := vera.Attachment(folder.ID, lake); got.Photo.HasPlace() || got.Photo.PlaceName != "" || !got.Photo.PlaceManual || !vera.PlaceHidden(folder.ID, lake) {
		t.Errorf("after removing: %+v", got.Photo)
	}
	// Who may only read may not; nor may an editor change a place kept from them.
	if err := vera.SetPlace(folder.ID, lake, &zLat, &zLon, "", nil); !errors.Is(err, ErrForbidden) {
		t.Errorf("a reader set a place: %v", err)
	}
	if err := paul.SetPlace(folder.ID, home, &zLat, &zLon, "", nil); !errors.Is(err, ErrForbidden) {
		t.Errorf("an editor changed a private place: %v", err)
	}
	noErr(t, anna.SetPlace(folder.ID, home, &zLat, &zLon, "Zermatt", nil))
	if got, _ := anna.Attachment(folder.ID, home); !got.Photo.PlacePrivate {
		t.Error("setting the place by hand lost its privacy")
	}
	if got, _ := paul.Attachment(folder.ID, home); got.Photo.HasPlace() || got.Photo.PlaceName != "" || got.Photo.PlaceManual {
		t.Errorf("a private place set by hand tells paul %+v", got.Photo)
	}
	for _, bad := range []struct {
		lat, lon *float64
		name     string
	}{{&zLat, nil, ""}, {nil, &zLon, ""}, {&zLat, &zLon, strings.Repeat("x", 201)}, {&zLat, &zLon, "a\nb"}} {
		if err := paul.SetPlace(folder.ID, scan, bad.lat, bad.lon, bad.name, nil); !errors.Is(err, ErrValidation) {
			t.Errorf("%v %v %q: %v", bad.lat, bad.lon, bad.name, err)
		}
	}
	if err := paul.SetPlace(folder.ID, "00000000000000000000000000000000", &zLat, &zLon, "", nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("no such file: %v", err)
	}
}

// TestACopyTakesAPlaceSetByHand: a copy carries the place set by hand on
// its original, as far as the copier may know it, and is accepted without
// the file's own place (ADR-0181).
func TestACopyTakesAPlaceSetByHand(t *testing.T) {
	s := setup(t)
	noErr(t, s.GrantRole("admin", "PLAT", "anna", Editor))
	noErr(t, s.GrantRole("admin", "PLAT", "paul", Editor))
	anna, paul := s.As("anna"), s.As("paul")
	from := mustItem(t)(anna.CreateFolder("PLAT", "", "Trips"))
	to := mustItem(t)(paul.CreateFolder("PLAT", "", "Mine"))
	lat, lon := 46.5584, 7.8352
	const lake = "00112233445566778899aabbccddeef4"
	_, err := anna.AddAttachment(from.ID, Attachment{ID: lake, Name: "lake.jpg", Type: "image/jpeg", Size: 3, SHA256: strings.Repeat("f4", 32), Photo: &Photo{Lat: &lat, Lon: &lon}})
	noErr(t, err)
	zLat, zLon := 46.0207, 7.7491
	noErr(t, anna.SetPlace(from.ID, lake, &zLat, &zLon, "Zermatt", nil))
	got, err := paul.AddAttachment(to.ID, Attachment{ID: "00112233445566778899aabbccddee11", Name: "lake.jpg", Type: "image/jpeg", Size: 3, SHA256: strings.Repeat("e9", 32),
		Source: &Source{Item: from.ID, Attachment: lake}, AsGiven: true, Photo: &Photo{Taken: "2024-05-01T13:22:10"}})
	noErr(t, err)
	if *got.Photo.Lat != zLat || got.Photo.PlaceName != "Zermatt" || !got.Photo.PlaceManual || got.Photo.Taken != "2024-05-01T13:22:10" {
		t.Errorf("the copy's photo data %+v", got.Photo)
	}
}
