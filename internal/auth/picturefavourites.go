package auth

import (
	"fmt"
	"regexp"

	"github.com/dbuchs7/orbithek/internal/budget"
)

// PictureRef names a picture of the album as a person marks it a
// favourite (ADR-0177): its item and the first version of its file, so a
// picture stays a favourite through its later versions. It names, and
// holds no content.
type PictureRef struct {
	Item string `json:"item"`
	File string `json:"file"`
}

var (
	pictureItemRE = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	pictureFileRE = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

// PictureFavourites returns the pictures a user marked as favourites.
// Whether the user may still read each is the caller's concern.
func (a *Accounts) PictureFavourites(id UserID) map[PictureRef]bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := map[PictureRef]bool{}
	if u, ok := a.users[id]; ok {
		for p := range u.pictures {
			out[p] = true
		}
	}
	return out
}

// SetPictureFavourite marks a picture as one of a user's favourites, or
// unmarks it. Marking a favourite again, or unmarking one that is not,
// changes nothing. Each change is a record of its own, so a person with
// many favourites does not write them all again. A file keeps its ID when
// it is moved to another item (ADR-0188), so a favourite is the file
// whatever item it was marked in: marking it again in its new item
// changes nothing, and unmarking it removes it as it was marked.
func (a *Accounts) SetPictureFavourite(id UserID, p PictureRef, on bool) error {
	if !pictureItemRE.MatchString(p.Item) || !pictureFileRE.MatchString(p.File) {
		return fmt.Errorf("%w: no such picture", ErrInvalid)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	u, ok := a.users[id]
	if !ok {
		return ErrNotFound
	}
	for q := range u.pictures {
		if q.File == p.File && q != p {
			if on {
				return nil
			}
			p = q
			break
		}
	}
	if u.pictures[p] == on {
		return nil
	}
	if limit := budget.Get().PictureFavourites; on && len(u.pictures) >= limit {
		return fmt.Errorf("%w: at most %d favourite pictures", ErrInvalid, limit)
	}
	return a.record(event{Kind: pictureFavouriteSet, User: id, Picture: &p, On: on})
}

// applyPictureFavourite marks or unmarks one picture.
func (a *Accounts) applyPictureFavourite(e event) error {
	u, ok := a.users[e.User]
	if !ok || e.Picture == nil {
		return fmt.Errorf("picture favourite of unknown user %s", e.User)
	}
	if !e.On {
		delete(u.pictures, *e.Picture)
		return nil
	}
	if u.pictures == nil {
		u.pictures = map[PictureRef]bool{}
	}
	u.pictures[*e.Picture] = true
	return nil
}
