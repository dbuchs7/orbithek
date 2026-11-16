package item

import (
	"errors"
	"strings"
	"testing"
)

// TestAPictureTurnedByHand: who may edit a picture's item turns it by
// quarter turns, and the photo data say how it is shown; turned back to the
// orientation its file tells, it is no longer turned by hand; the rest of
// the photo data stays; who may only read may not turn it, and other turns
// are refused (ADR-draft-pictures-turned-by-hand).
func TestAPictureTurnedByHand(t *testing.T) {
	s := setup(t)
	noErr(t, s.GrantRole("admin", "PLAT", "anna", Editor))
	noErr(t, s.GrantRole("admin", "PLAT", "paul", Editor))
	noErr(t, s.GrantRole("admin", "PLAT", "vera", Viewer))
	anna, paul, vera := s.As("anna"), s.As("paul"), s.As("vera")
	folder := mustItem(t)(anna.CreateFolder("PLAT", "", "Scans"))
	const lake, scan = "00112233445566778899aabbccddeea4", "00112233445566778899aabbccddeea5"
	file := &Photo{Taken: "2024-05-01T13:22:10", TakenManual: false, Make: "Canon", Orientation: 6, Width: 40, Height: 30}
	_, err := anna.AddAttachment(folder.ID, Attachment{ID: lake, Name: "lake.jpg", Type: "image/jpeg", Size: 3, SHA256: strings.Repeat("a4", 32), Photo: file})
	noErr(t, err)
	_, err = anna.AddAttachment(folder.ID, Attachment{ID: scan, Name: "scan.png", Type: "image/png", Size: 3, SHA256: strings.Repeat("a5", 32)})
	noErr(t, err)

	noErr(t, paul.Turn(folder.ID, lake, 1, file))
	got, _ := vera.Attachment(folder.ID, lake)
	if p := got.Photo; p.Orientation != 3 || !p.OrientationManual || p.Make != "Canon" || p.Taken != "2024-05-01T13:22:10" || p.Width != 40 {
		t.Errorf("after a quarter turn: %+v", p)
	}
	noErr(t, paul.Turn(folder.ID, lake, 2, file))
	if got, _ := vera.Attachment(folder.ID, lake); got.Photo.Orientation != 1 || !got.Photo.OrientationManual {
		t.Errorf("after turning upside down: %+v", got.Photo)
	}
	// Back where the file tells, the picture is no longer turned by hand.
	noErr(t, paul.Turn(folder.ID, lake, 1, file))
	if got, _ := vera.Attachment(folder.ID, lake); got.Photo.Orientation != 6 || got.Photo.OrientationManual {
		t.Errorf("turned back: %+v", got.Photo)
	}

	// A picture stored before photo data was recorded, or one that tells
	// no orientation, is read as upright.
	noErr(t, paul.Turn(folder.ID, scan, -1, nil))
	if got, _ := vera.Attachment(folder.ID, scan); got.Photo.Orientation != 8 || !got.Photo.OrientationManual {
		t.Errorf("the scan turned counter-clockwise: %+v", got.Photo)
	}
	noErr(t, paul.Turn(folder.ID, scan, 1, &Photo{Width: 10, Height: 20}))
	if got, _ := vera.Attachment(folder.ID, scan); got.Photo.Orientation != 0 || got.Photo.OrientationManual {
		t.Errorf("the scan turned back: %+v", got.Photo)
	}

	if err := vera.Turn(folder.ID, lake, 1, file); !errors.Is(err, ErrForbidden) {
		t.Errorf("a reader turned a picture: %v", err)
	}
	for _, q := range []int{0, 3, -2, 4} {
		if err := paul.Turn(folder.ID, lake, q, file); !errors.Is(err, ErrValidation) {
			t.Errorf("turned by %d: %v", q, err)
		}
	}
	if err := paul.Turn(folder.ID, "00000000000000000000000000000000", 1, nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("no such file: %v", err)
	}
	if ValidPhoto(&Photo{OrientationManual: true}) {
		t.Error("a picture turned by hand without an orientation is valid")
	}
}

// TestFacesTurnWithTheirPicture: the boxes of the faces found on a picture
// are turned with it, so they lie on the same faces; their names stay.
func TestFacesTurnWithTheirPicture(t *testing.T) {
	s := setup(t)
	noErr(t, s.GrantRole("admin", "PLAT", "anna", Editor))
	anna := s.As("anna")
	folder := mustItem(t)(anna.CreateFolder("PLAT", "", "Family"))
	const pic = "00112233445566778899aabbccddeea6"
	_, err := anna.AddAttachment(folder.ID, Attachment{ID: pic, Name: "family.jpg", Type: "image/jpeg", Size: 3, SHA256: strings.Repeat("a6", 32)})
	noErr(t, err)
	noErr(t, anna.SetPictureFaces(folder.ID, pic, []Face{{X: 0.1, Y: 0.2, W: 0.3, H: 0.4, Person: "Ben"}}))

	noErr(t, anna.Turn(folder.ID, pic, 1, nil))
	got, _ := anna.Attachment(folder.ID, pic)
	if want := (Face{X: 0.4, Y: 0.1, W: 0.4, H: 0.3, Person: "Ben"}); len(got.Faces) != 1 || got.Faces[0] != want || !got.FacesSeen {
		t.Errorf("after a quarter turn the faces are %+v, want %+v", got.Faces, want)
	}
	noErr(t, anna.Turn(folder.ID, pic, -1, nil))
	got, _ = anna.Attachment(folder.ID, pic)
	if want := (Face{X: 0.1, Y: 0.2, W: 0.3, H: 0.4, Person: "Ben"}); len(got.Faces) != 1 || got.Faces[0] != want {
		t.Errorf("turned back the faces are %+v, want %+v", got.Faces, want)
	}
	noErr(t, anna.Turn(folder.ID, pic, 2, nil))
	got, _ = anna.Attachment(folder.ID, pic)
	if want := (Face{X: 0.6, Y: 0.4, W: 0.3, H: 0.4, Person: "Ben"}); len(got.Faces) != 1 || got.Faces[0] != want {
		t.Errorf("upside down the faces are %+v, want %+v", got.Faces, want)
	}
}

// TestACopyTakesATurnByHand: a copy is shown turned as its original is,
// whether its content is the original's as stored or as the server gave
// it, with the orientation written into it.
func TestACopyTakesATurnByHand(t *testing.T) {
	s := setup(t)
	noErr(t, s.GrantRole("admin", "PLAT", "anna", Editor))
	noErr(t, s.GrantRole("admin", "PLAT", "paul", Editor))
	anna, paul := s.As("anna"), s.As("paul")
	from := mustItem(t)(anna.CreateFolder("PLAT", "", "Scans"))
	to := mustItem(t)(paul.CreateFolder("PLAT", "", "Mine"))
	const scan = "00112233445566778899aabbccddeea7"
	_, err := anna.AddAttachment(from.ID, Attachment{ID: scan, Name: "scan.gif", Type: "image/gif", Size: 3, SHA256: strings.Repeat("a7", 32), Photo: &Photo{Width: 4, Height: 2}})
	noErr(t, err)
	noErr(t, anna.Turn(from.ID, scan, 1, nil))
	src := &Source{Item: from.ID, Attachment: scan}
	// A GIF is given as stored: its copy has the original's content.
	got, err := paul.AddAttachment(to.ID, Attachment{ID: "00112233445566778899aabbccddee20", Name: "scan.gif", Type: "image/gif", Size: 3, SHA256: strings.Repeat("a7", 32), Source: src, Photo: &Photo{Width: 4, Height: 2}})
	noErr(t, err)
	if p := got.Photo; p.Orientation != 6 || !p.OrientationManual || p.Width != 4 {
		t.Errorf("the copy as stored: %+v", p)
	}
	// A picture given with its orientation written is taken as given.
	got, err = paul.AddAttachment(to.ID, Attachment{ID: "00112233445566778899aabbccddee21", Name: "scan.gif", Type: "image/gif", Size: 3, SHA256: strings.Repeat("e1", 32), Source: src, AsGiven: true, Photo: &Photo{Orientation: 6}})
	noErr(t, err)
	if p := got.Photo; p.Orientation != 6 || !p.OrientationManual {
		t.Errorf("the copy as given: %+v", p)
	}
}
