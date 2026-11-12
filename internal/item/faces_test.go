package item

import (
	"errors"
	"strings"
	"testing"
)

// TestFacesOnAPicture: faces are set on one version of a picture by whoever
// may edit it, validated, and listed as persons for whoever may read it,
// once per picture and only from the newest version of a stack (ADR-0194).
func TestFacesOnAPicture(t *testing.T) {
	s := setup(t)
	noErr(t, s.GrantRole("admin", "PLAT", "anna", Editor))
	noErr(t, s.GrantRole("admin", "PLAT", "ben", Viewer))
	anna, ben, carl := s.As("anna"), s.As("ben"), s.As("carl")
	folder := mustItem(t)(anna.CreateFolder("PLAT", "", "Family"))
	const lake, old, sketch = "0123456789abcdef0123456789abce31", "0123456789abcdef0123456789abce32", "0123456789abcdef0123456789abce33"
	add := func(id, name string, src *Source) {
		t.Helper()
		_, err := anna.AddAttachment(folder.ID, Attachment{ID: ID(id), Name: name, Type: "image/png", Size: 3, SHA256: strings.Repeat(id[30:], 32), Source: src})
		noErr(t, err)
	}
	add(lake, "Lake.png", nil)
	add(old, "Sketch.drawing.png", nil)
	add(sketch, "Sketch.drawing.png", &Source{Item: folder.ID, Attachment: old})

	for name, faces := range map[string][]Face{
		"outside":   {{X: 0.9, Y: 0.1, W: 0.2, H: 0.2}},
		"negative":  {{X: -0.1, Y: 0.1, W: 0.2, H: 0.2}},
		"no size":   {{X: 0.1, Y: 0.1, W: 0, H: 0.2}},
		"long name": {{X: 0.1, Y: 0.1, W: 0.2, H: 0.2, Person: strings.Repeat("a", 81)}},
		"control":   {{X: 0.1, Y: 0.1, W: 0.2, H: 0.2, Person: "a\x07b"}},
		"too many":  make([]Face, MaxFaces+1),
	} {
		if err := anna.SetPictureFaces(folder.ID, lake, faces); !errors.Is(err, ErrValidation) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if err := ben.SetPictureFaces(folder.ID, lake, nil); !errors.Is(err, ErrForbidden) {
		t.Errorf("a reader set faces: %v", err)
	}
	noErr(t, anna.SetPictureFaces(folder.ID, lake, []Face{
		{X: 0.1, Y: 0.2, W: 0.15, H: 0.2, Person: "  Anna   Muster "},
		{X: 0.5, Y: 0.2, W: 0.15, H: 0.2, Person: "anna muster"},
		{X: 0.3, Y: 0.6, W: 0.123456789, H: 0.2},
	}))
	got, _ := anna.Attachment(folder.ID, lake)
	if !got.FacesSeen || len(got.Faces) != 3 || got.Faces[0].Person != "Anna Muster" || got.Faces[2].W != 0.1235 {
		t.Fatalf("faces kept %+v", got)
	}
	if !got.Shows("ANNA MUSTER") || got.Shows("Ben") || got.Shows("") {
		t.Error("Shows")
	}
	// A picture searched without finding a face is marked searched.
	noErr(t, anna.SetPictureFaces(folder.ID, old, nil))
	if a, _ := anna.Attachment(folder.ID, old); !a.FacesSeen || len(a.Faces) != 0 {
		t.Errorf("searched without faces %+v", a)
	}
	n := len(s.Log())
	noErr(t, anna.SetPictureFaces(folder.ID, old, []Face{}))
	if len(s.Log()) != n {
		t.Error("setting the same faces again was recorded")
	}
	// An older version of a stack names nobody in the album.
	noErr(t, anna.SetPictureFaces(folder.ID, old, []Face{{X: 0.1, Y: 0.1, W: 0.2, H: 0.2, Person: "Old"}}))
	noErr(t, anna.SetPictureFaces(folder.ID, sketch, []Face{{X: 0.1, Y: 0.1, W: 0.2, H: 0.2, Person: "Ben"}}))

	people := ben.People("")
	if len(people) != 2 || people[0].Name != "Anna Muster" || people[0].Pictures != 1 || people[0].Cover.Attachment.ID != lake ||
		people[1].Name != "Ben" || people[1].Face.Person != "Ben" {
		t.Fatalf("people %+v", people)
	}
	if len(carl.People("")) != 0 || len(ben.People("OPS")) != 0 {
		t.Error("persons of pictures one may not read, or of another space")
	}
	// A deleted version forgets its faces; a moved file keeps them.
	noErr(t, anna.DeleteAttachment(folder.ID, sketch))
	if p := ben.People(""); len(p) != 2 || p[1].Name != "Old" {
		t.Errorf("after deleting the newest version %+v", p)
	}
	other := mustItem(t)(anna.CreateFolder("PLAT", "", "Other"))
	noErr(t, anna.MoveFile(folder.ID, lake, other.ID, []ID{lake}))
	if a, err := anna.Attachment(other.ID, lake); err != nil || len(a.Faces) != 3 {
		t.Errorf("a moved picture lost its faces: %+v %v", a, err)
	}
}

func TestNormalPerson(t *testing.T) {
	for in, want := range map[string]string{" Anna  Muster ": "Anna Muster", "": "", "Zoë": "Zoë"} {
		if got, err := NormalPerson(in); err != nil || got != want {
			t.Errorf("%q: %q %v", in, got, err)
		}
	}
	if _, err := NormalPerson("\xff"); err == nil {
		t.Error("invalid UTF-8 taken")
	}
}

// TestUnnamedFaces: the pictures a name can be suggested for across the
// album are the newest versions one may edit with a face nobody named
// (ADR-0199).
func TestUnnamedFaces(t *testing.T) {
	s := setup(t)
	noErr(t, s.GrantRole("admin", "PLAT", "anna", Editor))
	noErr(t, s.GrantRole("admin", "PLAT", "ben", Viewer))
	anna, ben := s.As("anna"), s.As("ben")
	folder := mustItem(t)(anna.CreateFolder("PLAT", "", "Family"))
	const named, open, old, newest = "0123456789abcdef0123456789abce41", "0123456789abcdef0123456789abce42", "0123456789abcdef0123456789abce43", "0123456789abcdef0123456789abce44"
	add := func(id, name string, src *Source) {
		t.Helper()
		_, err := anna.AddAttachment(folder.ID, Attachment{ID: ID(id), Name: name, Type: "image/png", Size: 3, SHA256: strings.Repeat(id[30:], 32), Source: src})
		noErr(t, err)
	}
	add(named, "Named.png", nil)
	add(open, "Open.png", nil)
	add(old, "Sketch.drawing.png", nil)
	add(newest, "Sketch.drawing.png", &Source{Item: folder.ID, Attachment: old})
	noErr(t, anna.SetPictureFaces(folder.ID, named, []Face{{X: 0.1, Y: 0.1, W: 0.2, H: 0.2, Person: "Anna"}}))
	noErr(t, anna.SetPictureFaces(folder.ID, open, []Face{{X: 0.1, Y: 0.1, W: 0.2, H: 0.2, Person: "Anna"}, {X: 0.5, Y: 0.1, W: 0.2, H: 0.2}}))
	// An older version of a stack is not offered: the album shows the newest.
	noErr(t, anna.SetPictureFaces(folder.ID, old, []Face{{X: 0.1, Y: 0.1, W: 0.2, H: 0.2}}))
	noErr(t, anna.SetPictureFaces(folder.ID, newest, []Face{{X: 0.1, Y: 0.1, W: 0.2, H: 0.2, Person: "Ben"}}))

	got := anna.UnnamedFaces("")
	if len(got) != 1 || got[0].Attachment.ID != open {
		t.Fatalf("unnamed faces %+v", got)
	}
	if len(ben.UnnamedFaces("")) != 0 || len(anna.UnnamedFaces("OPS")) != 0 {
		t.Error("pictures one may not edit, or of another space")
	}
}
