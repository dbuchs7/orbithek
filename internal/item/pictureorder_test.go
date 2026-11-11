package item

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// slideNames is the order of the slides of a folder or collection as x
// reads them: the order set by hand, then the rest by name (ADR-0195).
func slideNames(t *testing.T, x *Session, id ID) []string {
	t.Helper()
	c, err := x.slideContainer(id)
	noErr(t, err)
	ranks, err := x.SlideRanks(id)
	noErr(t, err)
	stacks := x.slideStacks(c)
	slices.SortFunc(stacks, func(a, b slideStack) int { return ranks.Compare(a.key, b.key) })
	var out []string
	for _, st := range stacks {
		out = append(out, SlideCaption(st.key.Name))
	}
	return out
}

func pic(n int) ID {
	return ID("0123456789abcdef0123456789ab" + strings.Repeat("0", 4-len(itoa(n))) + itoa(n))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}

func TestPictureNamesCompareByTheValueOfTheirNumbers(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"2 Plan", "10 Outlook", -1},
		{"Slide9", "Slide10", -1},
		{"folie 3", "Folie 3", 1}, // equal but for case: as written
		{"a", "B", -1},
		{"Intro", "intro 2", -1},
		{"007", "7", 1},
		{"99999999999999999999 b", "100000000000000000000 a", -1},
		{"x", "x", 0},
	}
	for _, c := range cases {
		got := ComparePictureNames(c.a, c.b)
		if (got < 0) != (c.want < 0) || (got > 0) != (c.want > 0) {
			t.Errorf("ComparePictureNames(%q, %q) = %d, want sign %d", c.a, c.b, got, c.want)
		}
	}
	if got := SlideCaption("1 Map.drawing.png"); got != "1 Map" {
		t.Errorf("caption of a drawing %q", got)
	}
	if got := SlideCaption("Intro.final.png"); got != "Intro.final" {
		t.Errorf("caption %q", got)
	}
}

// Pictures of a folder are moved by hand; pictures added later come
// after them, by name; clearing goes back to the order by name.
func TestTheSlidesOfAFolderAreOrderedByHand(t *testing.T) {
	s := permFixture(t)
	anna, ben := s.As("anna"), s.As("ben")
	talk := mustItem(t)(anna.CreateFolder("PLAT", "", "Talk"))
	for i, name := range []string{"10 End.png", "2 Goal.png", "1 Start.png"} {
		addPicture(t, anna, talk.ID, pic(i+1), name)
	}
	if got := slideNames(t, ben, talk.ID); !slices.Equal(got, []string{"1 Start", "2 Goal", "10 End"}) {
		t.Fatalf("by name %v", got)
	}
	if err := ben.PlacePictures(talk.ID, []ID{pic(1)}, pic(3)); !errors.Is(err, ErrForbidden) {
		t.Fatalf("a reader ordered the slides: %v", err)
	}
	noErr(t, anna.PlacePictures(talk.ID, []ID{pic(1)}, pic(3)))
	if got := slideNames(t, ben, talk.ID); !slices.Equal(got, []string{"10 End", "1 Start", "2 Goal"}) {
		t.Fatalf("after moving the end first %v", got)
	}
	noErr(t, anna.PlacePictures(talk.ID, []ID{pic(3), pic(2)}, ""))
	if got := slideNames(t, ben, talk.ID); !slices.Equal(got, []string{"10 End", "1 Start", "2 Goal"}) {
		t.Fatalf("moving two to the end %v", got)
	}
	noErr(t, anna.PlacePictures(talk.ID, []ID{pic(2), pic(3)}, ""))
	if got := slideNames(t, ben, talk.ID); !slices.Equal(got, []string{"10 End", "2 Goal", "1 Start"}) {
		t.Fatalf("moving two in their given order %v", got)
	}
	// The same order again records nothing.
	n := len(s.Log())
	noErr(t, anna.PlacePictures(talk.ID, []ID{pic(3)}, ""))
	if len(s.Log()) != n {
		t.Errorf("an unchanged order was recorded")
	}
	// New pictures come after the ordered ones, by name.
	addPicture(t, anna, talk.ID, pic(5), "B.png")
	addPicture(t, anna, talk.ID, pic(4), "A.png")
	if got := slideNames(t, ben, talk.ID); !slices.Equal(got, []string{"10 End", "2 Goal", "1 Start", "A", "B"}) {
		t.Fatalf("with new pictures %v", got)
	}
	// A new version of a picture keeps its place.
	_, err := anna.AddAttachment(talk.ID, Attachment{ID: pic(6), Name: "2 Goal.png", Type: "image/png", Size: 3, SHA256: strings.Repeat("cd", 32),
		Source: &Source{Item: talk.ID, Attachment: pic(2)}})
	noErr(t, err)
	if got := slideNames(t, ben, talk.ID); !slices.Equal(got, []string{"10 End", "2 Goal", "1 Start", "A", "B"}) {
		t.Fatalf("after a new version %v", got)
	}
	re, err := Replay(s.Log())
	noErr(t, err)
	if got := slideNames(t, re.As("ben"), talk.ID); !slices.Equal(got, []string{"10 End", "2 Goal", "1 Start", "A", "B"}) {
		t.Errorf("replayed %v", got)
	}
	// A deleted picture is forgotten with the next change.
	noErr(t, anna.DeleteAttachment(talk.ID, pic(1)))
	noErr(t, anna.PlacePictures(talk.ID, []ID{pic(4)}, pic(2)))
	if got := mustItem(t)(ben.Get(talk.ID)).PictureOrder; slices.Contains(got, pic(1)) || !slices.Equal(got, []ID{pic(4), pic(2), pic(3), pic(5)}) {
		t.Errorf("order kept %v", got)
	}
	noErr(t, anna.ClearPictureOrder(talk.ID))
	if got := slideNames(t, ben, talk.ID); !slices.Equal(got, []string{"1 Start", "2 Goal", "A", "B"}) {
		t.Fatalf("cleared %v", got)
	}
	n = len(s.Log())
	noErr(t, anna.ClearPictureOrder(talk.ID))
	if len(s.Log()) != n {
		t.Errorf("clearing no order was recorded")
	}
}

func TestPlacingPicturesNamesOnlyPicturesOfTheFolder(t *testing.T) {
	s := permFixture(t)
	anna := s.As("anna")
	talk := mustItem(t)(anna.CreateFolder("PLAT", "", "Talk"))
	other := mustItem(t)(anna.CreateFolder("PLAT", "", "Other"))
	addPicture(t, anna, talk.ID, pic(1), "a.png")
	addPicture(t, anna, talk.ID, pic(2), "b.png")
	addPicture(t, anna, other.ID, pic(3), "c.png")
	page := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Runbook"}))
	cases := map[string]struct {
		id     ID
		moved  []ID
		before ID
		want   error
	}{
		"nothing moved":         {talk.ID, nil, "", ErrValidation},
		"of another folder":     {talk.ID, []ID{pic(3)}, "", ErrNotFound},
		"before another folder": {talk.ID, []ID{pic(1)}, pic(3), ErrNotFound},
		"named twice":           {talk.ID, []ID{pic(1), pic(1)}, "", ErrValidation},
		"before itself":         {talk.ID, []ID{pic(1)}, pic(1), ErrValidation},
		"too many":              {talk.ID, []ID{pic(1), pic(2), pic(3)}, "", ErrValidation},
		"no folder":             {page.ID, []ID{pic(1)}, "", ErrValidation},
	}
	for name, c := range cases {
		if err := anna.PlacePictures(c.id, c.moved, c.before); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", name, err, c.want)
		}
	}
	if err := anna.ClearPictureOrder(page.ID); !errors.Is(err, ErrValidation) {
		t.Errorf("cleared the order of a page: %v", err)
	}
	if err := s.As("carl").PlacePictures(talk.ID, []ID{pic(1)}, ""); !errors.Is(err, ErrNotFound) {
		t.Errorf("one who does not read the folder: %v", err)
	}
}

// A collection gathers pictures of several rooms. Its order names
// pictures some readers do not read: they keep their places when such a
// reader moves the others, and the reader sees only what they read.
func TestTheSlidesOfACollectionKeepThePlacesOfPicturesOthersDoNotRead(t *testing.T) {
	s := permFixture(t)
	anna, pat := s.As("anna"), s.As("pat")
	open := mustItem(t)(anna.CreateFolder("PLAT", "", "Open"))
	closed := mustItem(t)(pat.CreateFolder("PLAT", "", "Closed"))
	noErr(t, pat.Restrict(closed.ID, Restriction{Read: []string{"pat"}}))
	for i, name := range []string{"a.png", "b.png", "c.png"} {
		addPicture(t, anna, open.ID, pic(i+1), name)
		noErr(t, anna.SetPictureLabels(open.ID, pic(i+1), []string{"talk"}))
	}
	addPicture(t, pat, closed.ID, pic(9), "secret.png")
	noErr(t, pat.SetPictureLabels(closed.ID, pic(9), []string{"talk"}))
	talks := mustItem(t)(anna.CreateCollection("PLAT", "", "Talks", CollectionRule{Labels: []string{"talk"}}))

	if err := anna.PlacePictures(talks.ID, []ID{pic(9)}, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("placed a picture one does not read: %v", err)
	}
	noErr(t, pat.PlacePictures(talks.ID, []ID{pic(9)}, pic(2)))
	if got := slideNames(t, pat, talks.ID); !slices.Equal(got, []string{"a", "secret", "b", "c"}) {
		t.Fatalf("pat reads %v", got)
	}
	if got := slideNames(t, anna, talks.ID); !slices.Equal(got, []string{"a", "b", "c"}) {
		t.Fatalf("anna reads %v", got)
	}
	noErr(t, anna.PlacePictures(talks.ID, []ID{pic(3)}, pic(1)))
	if got := slideNames(t, anna, talks.ID); !slices.Equal(got, []string{"c", "a", "b"}) {
		t.Fatalf("anna moved c first: %v", got)
	}
	if got := slideNames(t, pat, talks.ID); !slices.Equal(got, []string{"c", "a", "secret", "b"}) {
		t.Fatalf("pat reads after anna's move %v", got)
	}
	// A picture moved to another folder with its identity keeps its place
	// in the collection (ADR-0188).
	other := mustItem(t)(anna.CreateFolder("PLAT", "", "Other"))
	noErr(t, anna.MoveFile(open.ID, pic(1), other.ID, []ID{pic(1)}))
	if got := slideNames(t, pat, talks.ID); !slices.Equal(got, []string{"c", "a", "secret", "b"}) {
		t.Fatalf("after moving a to another folder %v", got)
	}
}
