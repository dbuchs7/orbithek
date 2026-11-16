package server

import (
	"net/http"

	"github.com/dbuchs7/orbithek/internal/item"
)

// Pictures turned by hand (ADR-0231): a picture
// or drawing is turned by quarter turns by its item's editors. Its
// thumbnails are made turned, and every JPEG, PNG and WebP given out of it
// carries the orientation in its EXIF (openFor); the stored file is never
// changed.

// turnPicture turns a picture by quarter turns as it is shown.
func (h itemHandlers) turnPicture(w http.ResponseWriter, r *http.Request) {
	var req struct {
		By int `json:"by"`
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
	switch {
	case !inlineTypes[a.Type] || albumVideo(a):
		writeError(w, http.StatusBadRequest, "only a picture or a drawing is turned")
		return
	case a.Linked():
		writeError(w, http.StatusBadRequest, "an entry of a file share is shown as its share holds it")
		return
	}
	// What the file tells: the orientation it is turned back to, and
	// everything for a picture stored before photo data was recorded.
	var file *item.Photo
	if a.Photo == nil || a.Photo.OrientationManual {
		file = h.readPhoto(it.ID, a)
	}
	h.write(w, r, http.StatusOK, func(x *item.Session, _ *item.Store) (any, error) {
		if err := x.Turn(it.ID, a.ID, req.By, file); err != nil {
			return nil, err
		}
		got, err := x.Attachment(it.ID, a.ID)
		return map[string]attachmentJSON{"attachment": toAttachmentJSON(got)}, err
	})
}
