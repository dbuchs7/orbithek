package item

import (
	"errors"
	"strings"
	"testing"
)

// TestAPlaceKeptToItsUploader: the one who added a photo keeps its place
// to themselves; every other reader gets the photo data without it, in
// every listing, and is told nothing of it; the choice is changed only by
// its uploader and recorded for an older picture with what the server read
// (ADR-0178).
func TestAPlaceKeptToItsUploader(t *testing.T) {
	s := setup(t)
	noErr(t, s.GrantRole("admin", "PLAT", "anna", Editor))
	noErr(t, s.GrantRole("admin", "PLAT", "paul", Editor))
	anna, paul := s.As("anna"), s.As("paul")
	folder := mustItem(t)(anna.CreateFolder("PLAT", "", "Home"))
	lat, lon := 47.3769, 8.5417
	const garden, old, flat = "00112233445566778899aabbccddeef0", "00112233445566778899aabbccddeef1", "00112233445566778899aabbccddeef2"
	add := func(x *Session, id, sha string, p *Photo) {
		t.Helper()
		_, err := x.AddAttachment(folder.ID, Attachment{ID: ID(id), Name: id + ".jpg", Type: "image/jpeg", Size: 3, SHA256: strings.Repeat(sha, 32), Photo: p})
		noErr(t, err)
	}
	add(anna, garden, "e0", &Photo{Taken: "2026-06-01T18:00:00", Lat: &lat, Lon: &lon, PlacePrivate: true})
	add(anna, old, "e1", nil)
	add(anna, flat, "e2", &Photo{Taken: "2026-06-01T18:00:00"})

	mine, err := anna.Attachment(folder.ID, garden)
	noErr(t, err)
	if !mine.Photo.HasPlace() || !mine.Photo.PlacePrivate {
		t.Fatalf("the uploader lost the place: %+v", mine.Photo)
	}
	seen, err := paul.Attachment(folder.ID, garden)
	noErr(t, err)
	if seen.Photo.HasPlace() || seen.Photo.PlacePrivate || seen.Photo.Alt != nil || seen.Photo.Taken != "2026-06-01T18:00:00" {
		t.Errorf("another reader sees %+v", seen.Photo)
	}
	list, err := paul.Attachments(folder.ID)
	noErr(t, err)
	for _, a := range list {
		if a.Photo.HasPlace() {
			t.Errorf("listing: %s tells paul a place", a.Name)
		}
	}
	for _, p := range paul.Pictures("PLAT", func(Attachment) bool { return true }) {
		if p.Attachment.Photo.HasPlace() {
			t.Errorf("album: %s tells paul a place", p.Attachment.Name)
		}
	}
	if d, err := paul.Drawing(folder.ID, garden); err != nil || d.Newest.Photo.HasPlace() {
		t.Errorf("a stack tells paul a place: %v %+v", err, d.Newest.Photo)
	}
	if !paul.PlaceHidden(folder.ID, garden) || anna.PlaceHidden(folder.ID, garden) || paul.PlaceHidden(folder.ID, flat) || paul.PlaceHidden(folder.ID, "nope") {
		t.Error("PlaceHidden")
	}

	// Only the uploader decides; a picture without a place has nothing to keep.
	if err := paul.SetPlacePrivate(folder.ID, garden, false, nil); !errors.Is(err, ErrForbidden) {
		t.Errorf("paul decided: %v", err)
	}
	if err := anna.SetPlacePrivate(folder.ID, flat, true, nil); !errors.Is(err, ErrValidation) {
		t.Errorf("a picture without a place: %v", err)
	}
	if err := anna.SetPlacePrivate(folder.ID, "00000000000000000000000000000000", true, nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("no such file: %v", err)
	}
	noErr(t, anna.SetPlacePrivate(folder.ID, garden, false, nil))
	if seen, _ := paul.Attachment(folder.ID, garden); !seen.Photo.HasPlace() {
		t.Error("shown again, paul still sees no place")
	}
	n := len(s.Log())
	noErr(t, anna.SetPlacePrivate(folder.ID, garden, false, nil))
	if len(s.Log()) != n {
		t.Error("an unchanged choice was recorded again")
	}
	// An older picture's place, read by the server, is recorded with the choice.
	noErr(t, anna.SetPlacePrivate(folder.ID, old, true, &Photo{Lat: &lat, Lon: &lon}))
	if got, _ := anna.Attachment(folder.ID, old); !got.Photo.PlacePrivate || !got.Photo.HasPlace() {
		t.Errorf("the older picture: %+v", got.Photo)
	}
	if got, _ := paul.Attachment(folder.ID, old); got.Photo.HasPlace() {
		t.Error("the older picture tells paul its place")
	}
	if err := checkPhoto(&Photo{PlacePrivate: true}); !errors.Is(err, ErrValidation) {
		t.Errorf("a private place without a place: %v", err)
	}
}

// TestCopiesOfAPrivatePlace: the uploader's own copy keeps its place
// private; another reader copies only the content without the place, as
// the server makes it, never the file with it (ADR-0178).
func TestCopiesOfAPrivatePlace(t *testing.T) {
	s := setup(t)
	noErr(t, s.GrantRole("admin", "PLAT", "anna", Editor))
	noErr(t, s.GrantRole("admin", "PLAT", "paul", Editor))
	anna, paul := s.As("anna"), s.As("paul")
	from := mustItem(t)(anna.CreateFolder("PLAT", "", "Home"))
	to := mustItem(t)(paul.CreateFolder("PLAT", "", "Shared"))
	lat, lon := 47.3769, 8.5417
	const garden = "00112233445566778899aabbccddeef0"
	_, err := anna.AddAttachment(from.ID, Attachment{ID: garden, Name: "garden.jpg", Type: "image/jpeg", Size: 3, SHA256: strings.Repeat("e0", 32),
		Photo: &Photo{Lat: &lat, Lon: &lon, PlacePrivate: true}})
	noErr(t, err)
	src := &Source{Item: from.ID, Attachment: garden}

	got, err := anna.AddAttachment(to.ID, Attachment{ID: "00112233445566778899aabbccddee01", Name: "garden.jpg", Type: "image/jpeg", Size: 3, SHA256: strings.Repeat("e0", 32),
		Source: src, Photo: &Photo{Lat: &lat, Lon: &lon}})
	noErr(t, err)
	if !got.Photo.PlacePrivate {
		t.Error("the uploader's own copy shows its place to everyone")
	}
	if _, err := paul.AddAttachment(to.ID, Attachment{ID: "00112233445566778899aabbccddee02", Name: "garden.jpg", Type: "image/jpeg", Size: 3, SHA256: strings.Repeat("e9", 32), Source: src}); !errors.Is(err, ErrValidation) {
		t.Errorf("a copy of other content was taken: %v", err)
	}
	if _, err := paul.AddAttachment(to.ID, Attachment{ID: "00112233445566778899aabbccddee03", Name: "garden.jpg", Type: "image/jpeg", Size: 3, SHA256: strings.Repeat("e9", 32), Source: src, WithoutPlace: true}); err != nil {
		t.Errorf("the copy without the place was refused: %v", err)
	}
	// Without a hidden place, other content is still no copy.
	noErr(t, anna.SetPlacePrivate(from.ID, garden, false, nil))
	if _, err := paul.AddAttachment(to.ID, Attachment{ID: "00112233445566778899aabbccddee04", Name: "garden.jpg", Type: "image/jpeg", Size: 3, SHA256: strings.Repeat("e9", 32), Source: src, WithoutPlace: true}); !errors.Is(err, ErrValidation) {
		t.Errorf("WithoutPlace made a copy of other content: %v", err)
	}
}
