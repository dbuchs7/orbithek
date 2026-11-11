package server

import (
	"net/http"

	"github.com/dbuchs7/orbithek/internal/item"
)

// Times set by hand (ADR-0198): when a photo was taken is set by its
// item's editors, or goes back to the time its file tells. Every file given
// out of a picture then carries that time in its EXIF (openFor); the stored
// file is never changed.

// setTaken sets when a photo was taken by hand.
func (h itemHandlers) setTaken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Taken  string `json:"taken"`
		Offset string `json:"offset"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Taken == "" {
		writeError(w, http.StatusBadRequest, "a time needs taken, of the form 2006-01-02T15:04:05; DELETE goes back to the file's own")
		return
	}
	h.takenByHand(w, r, req.Taken, req.Offset)
}

// resetTaken goes back to the time the file itself tells.
func (h itemHandlers) resetTaken(w http.ResponseWriter, r *http.Request) {
	h.takenByHand(w, r, "", "")
}

func (h itemHandlers) takenByHand(w http.ResponseWriter, r *http.Request, taken, offset string) {
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
	if !inlineTypes[a.Type] && !albumVideo(a) {
		writeError(w, http.StatusBadRequest, "only a picture or a video has a time it was taken")
		return
	}
	// What the file tells: its own time to go back to, and everything for
	// a picture stored before photo data was recorded.
	var file *item.Photo
	if a.Photo == nil || taken == "" {
		file = h.readPhoto(it.ID, a)
	}
	h.write(w, r, http.StatusOK, func(x *item.Session, _ *item.Store) (any, error) {
		if err := x.SetTaken(it.ID, a.ID, taken, offset, file); err != nil {
			return nil, err
		}
		got, err := x.Attachment(it.ID, a.ID)
		return map[string]attachmentJSON{"attachment": toAttachmentJSON(got)}, err
	})
}
