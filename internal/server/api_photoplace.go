package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"

	"github.com/dbuchs7/orbithek/internal/item"
	"github.com/dbuchs7/orbithek/internal/photo"
)

// Import choices of a photo's place (ADR-0177, ADR-0178): kept for every
// reader, kept to the one who adds the picture, or removed from the file.
const (
	placeKeep    = ""
	placePrivate = "private"
	placeStrip   = "strip"
)

// placeChoice reads an import's choice of the place; strip is the older
// way of asking to remove it.
func placeChoice(value string, strip bool) string {
	switch {
	case strip || value == placeStrip:
		return placeStrip
	case value == placePrivate:
		return placePrivate
	}
	return placeKeep
}

// keepPrivate marks photo data to keep its place to the one who adds the
// picture, when there is a place (ADR-0178).
func keepPrivate(p *item.Photo) *item.Photo {
	if !p.HasPlace() {
		return p
	}
	c := *p
	c.PlacePrivate = true
	return &c
}

// nopSeekCloser gives a picture held in memory as a stored file is given.
type nopSeekCloser struct{ *bytes.Reader }

func (nopSeekCloser) Close() error { return nil }

// videoCloser reads a video without its place and closes its file.
type videoCloser struct {
	io.ReadSeeker
	io.Closer
}

// openFor opens a stored file as the caller may have it: whole, or, when
// its uploader keeps the place a photo was taken from them, without it
// (ADR-0178); a picture whose place cannot be removed is not given. A
// picture whose time was set by hand carries that time in its EXIF
// (ADR-0196).
func (h itemHandlers) openFor(hidden bool, id item.ID, a item.Attachment) (io.ReadSeekCloser, error) {
	c, err := h.openWithoutPlace(hidden, id, a)
	if err != nil || !timeWritten(a) {
		return c, err
	}
	defer c.Close()
	data, err := io.ReadAll(io.LimitReader(c, a.Size+1))
	if err != nil {
		return nil, err
	}
	out, ok := photo.SetTaken(data, a.Type, a.Photo.Taken, a.Photo.Offset)
	if !ok {
		// The time stays in the photo data; the file is given as it is.
		h.log.Warn("the time set by hand could not be written into the file", "item", id, "attachment", a.ID)
	}
	return nopSeekCloser{bytes.NewReader(out)}, nil
}

// timeWritten reports whether a file is given with the time set by hand
// written into it (ADR-0196): a picture of a format whose EXIF the server
// writes. A video keeps its own.
func timeWritten(a item.Attachment) bool {
	return a.Photo != nil && a.Photo.TakenManual && inlineTypes[a.Type] && !albumVideo(a)
}

// openWithoutPlace opens a stored file whole, or without the place a photo
// was taken when hidden (ADR-0178).
func (h itemHandlers) openWithoutPlace(hidden bool, id item.ID, a item.Attachment) (io.ReadSeekCloser, error) {
	c, err := h.d.DB.OpenAttachment(id, a.ID)
	if err != nil || !hidden {
		return c, err
	}
	// A video is given as it is read, its place read as zeros, so it is
	// never held in memory (ADR-0189).
	if albumVideo(a) {
		out, ok := photo.VideoWithoutPlace(c)
		if !ok {
			c.Close()
			return nil, errPlaceStays
		}
		return videoCloser{out, c}, nil
	}
	defer c.Close()
	data, err := io.ReadAll(io.LimitReader(c, a.Size+1))
	if err != nil {
		return nil, err
	}
	out := photo.StripPlace(data, a.Type)
	if m := photo.Read(bytes.NewReader(out), a.Type); m.Lat != nil || m.Lon != nil {
		return nil, errPlaceStays
	}
	return nopSeekCloser{bytes.NewReader(out)}, nil
}

// givenSHA is the SHA-256 of a stored picture as the caller is given it:
// the content of a copy made by a reader from whom its uploader keeps the
// place (ADR-0178), or of a picture whose time was set by hand (ADR-0196).
func (h itemHandlers) givenSHA(hidden bool, id item.ID, a item.Attachment) (string, error) {
	c, err := h.openFor(hidden, id, a)
	if err != nil {
		return "", err
	}
	defer c.Close()
	sum := sha256.New()
	if _, err := io.Copy(sum, c); err != nil {
		return "", err
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

// markCopyAsGiven tells the item model that an uploaded copy is its
// source as the caller is given it — without the place its uploader keeps
// from them (ADR-0178), or with the time set by hand written into it
// (ADR-0196) — when the server finds the copy's content to be exactly that.
func (h itemHandlers) markCopyAsGiven(r *http.Request, id item.ID, a *item.Attachment) {
	if a.Source == nil || a.Source.Item == id {
		return
	}
	var hidden bool
	var src item.Attachment
	h.d.DB.Read(func(s *item.Store) {
		x := s.As(who(r))
		hidden = x.PlaceHidden(a.Source.Item, a.Source.Attachment)
		// Its name, type, size and photo data, to give it the way a
		// download does.
		src, _ = x.Attachment(a.Source.Item, a.Source.Attachment)
	})
	if src.ID == "" || src.SHA256 == a.SHA256 || (!hidden && !timeWritten(src)) {
		return
	}
	if sum, err := h.givenSHA(hidden, a.Source.Item, src); err == nil && sum == a.SHA256 {
		a.AsGiven = true
	}
}

// setPlacePrivate keeps where a photo was taken to the one who added it,
// or shows it to every reader again (ADR-0178). A picture stored before
// photo data was recorded has its data read from the file now and
// recorded with the choice.
func (h itemHandlers) setPlacePrivate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Private bool `json:"private"`
	}
	if !decode(w, r, &req) {
		return
	}
	var it item.Item
	var a item.Attachment
	var err error
	h.d.DB.Read(func(s *item.Store) {
		x := s.As(who(r))
		if it, err = resolveItem(x, r.PathValue("ref")); err == nil {
			a, err = x.Attachment(it.ID, item.ID(r.PathValue("id")))
		}
	})
	if err != nil {
		h.fail(w, err)
		return
	}
	var read *item.Photo
	if a.Photo == nil {
		read = h.readPhoto(it.ID, a)
	}
	h.write(w, r, http.StatusOK, func(x *item.Session, _ *item.Store) (any, error) {
		if err := x.SetPlacePrivate(it.ID, a.ID, req.Private, read); err != nil {
			return nil, err
		}
		got, err := x.Attachment(it.ID, a.ID)
		return map[string]attachmentJSON{"attachment": toAttachmentJSON(got)}, err
	})
}
