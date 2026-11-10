package auth

import (
	"errors"
	"testing"

	"github.com/dbuchs7/orbithek/internal/budget"
)

// TestPictureFavouritesAreKeptPerUserAndSurviveReload: a picture is marked
// and unmarked with one record each, only for its user, and a reload of
// the journal gives the marks back (ADR-0175).
func TestPictureFavouritesAreKeptPerUserAndSurviveReload(t *testing.T) {
	a, j, _ := newAccounts(t)
	anna, err := a.CreateUser("anna", "anna's password")
	noErr(t, err)
	paul, err := a.CreateUser("paul", "paul's password")
	noErr(t, err)
	lake := PictureRef{Item: "id001", File: "00112233445566778899aabbccddeeff"}
	hill := PictureRef{Item: "id002", File: "ffeeddccbbaa99887766554433221100"}
	noErr(t, a.SetPictureFavourite(anna.ID, lake, true))
	noErr(t, a.SetPictureFavourite(anna.ID, hill, true))
	n := len(j.records)
	noErr(t, a.SetPictureFavourite(anna.ID, lake, true))  // already: nothing recorded
	noErr(t, a.SetPictureFavourite(paul.ID, lake, false)) // not one: nothing recorded
	if len(j.records) != n {
		t.Error("an unchanged mark was recorded again")
	}
	if got := a.PictureFavourites(anna.ID); len(got) != 2 || !got[lake] || !got[hill] {
		t.Fatalf("anna's favourites %v", got)
	}
	if got := a.PictureFavourites(paul.ID); len(got) != 0 {
		t.Fatalf("paul has anna's favourites: %v", got)
	}
	for _, bad := range []PictureRef{{}, {Item: "id001"}, {Item: "a/b", File: lake.File}, {Item: "id001", File: "XYZ"}} {
		if err := a.SetPictureFavourite(anna.ID, bad, true); !errors.Is(err, ErrInvalid) {
			t.Errorf("%v: %v", bad, err)
		}
	}
	if err := a.SetPictureFavourite("nobody", lake, true); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown user: %v", err)
	}
	if got := a.PictureFavourites("nobody"); len(got) != 0 {
		t.Errorf("unknown user's favourites: %v", got)
	}

	b, err := New(j.records, j.append, WithIterations(1000))
	noErr(t, err)
	if got := b.PictureFavourites(anna.ID); len(got) != 2 || !got[lake] || !got[hill] {
		t.Fatalf("after reload: %v", got)
	}
	noErr(t, b.SetPictureFavourite(anna.ID, lake, false))
	c, err := New(j.records, j.append, WithIterations(1000))
	noErr(t, err)
	if got := c.PictureFavourites(anna.ID); len(got) != 1 || !got[hill] {
		t.Fatalf("one unmarked, after reload: %v", got)
	}
}

// TestPictureFavouritesAreBounded: a user marks at most PictureFavourites
// pictures; unmarking one is always possible.
func TestPictureFavouritesAreBounded(t *testing.T) {
	b := budget.Default()
	b.PictureFavourites = 2
	budget.Set(b)
	t.Cleanup(func() { budget.Set(budget.Default()) })
	a, _, _ := newAccounts(t)
	u, err := a.CreateUser("anna", "anna's password")
	noErr(t, err)
	refs := []PictureRef{{"i1", "00000000000000000000000000000001"}, {"i1", "00000000000000000000000000000002"}, {"i1", "00000000000000000000000000000003"}}
	noErr(t, a.SetPictureFavourite(u.ID, refs[0], true))
	noErr(t, a.SetPictureFavourite(u.ID, refs[1], true))
	if err := a.SetPictureFavourite(u.ID, refs[2], true); !errors.Is(err, ErrInvalid) {
		t.Errorf("past the bound: %v", err)
	}
	noErr(t, a.SetPictureFavourite(u.ID, refs[0], false))
	noErr(t, a.SetPictureFavourite(u.ID, refs[2], true))
}

// TestPictureFavouritesOfAnUnknownUserAreRefusedOnLoad: a journal that
// names no such user, or no picture, does not load.
func TestPictureFavouritesOfAnUnknownUserAreRefusedOnLoad(t *testing.T) {
	for _, rec := range []string{
		`{"v":1,"kind":"PictureFavouriteSet","user":"ghost","picture":{"item":"i1","file":"00000000000000000000000000000001"},"on":true}`,
	} {
		if _, err := New([][]byte{[]byte(rec)}, func([]byte) error { return nil }, WithIterations(1000)); err == nil {
			t.Errorf("%s was loaded", rec)
		}
	}
}
