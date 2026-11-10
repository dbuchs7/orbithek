package server

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"slices"
	"time"

	"github.com/dbuchs7/orbithek/internal/budget"
	"github.com/dbuchs7/orbithek/internal/item"
	"github.com/dbuchs7/orbithek/internal/photo"
	"github.com/dbuchs7/orbithek/internal/storage"
)

// Videos of the album (ADR-0189): MP4, WebM and QuickTime files are taken
// into a folder as pictures are, up to the budget VideoBytes, and shown,
// sorted, labelled, moved and deleted as pictures are. The server reads
// when and where a video was taken from its header; the browser takes a
// picture of it when it uploads it, which the server keeps as its poster.

// albumVideo reports whether a file is a video the album shows.
func albumVideo(a item.Attachment) bool { return photo.IsVideo(a.Type) }

// longTransfer gives a request carrying or answering a video the time the
// budget VideoTransferSeconds allows, beyond the server's usual minute.
func longTransfer(w http.ResponseWriter, read bool) {
	rc := http.NewResponseController(w)
	until := time.Now().Add(time.Duration(budget.Get().VideoTransferSeconds) * time.Second)
	if read {
		_ = rc.SetReadDeadline(until)
	}
	_ = rc.SetWriteDeadline(until)
}

// videoType tells a stored file's video type from its first bytes, which
// the standard detection leaves as an octet stream for QuickTime and for
// some MP4 brands; "" for no video.
func (h itemHandlers) videoType(id item.ID, a item.Attachment) string {
	c, err := h.d.DB.OpenAttachment(id, a.ID)
	if err != nil {
		return ""
	}
	defer c.Close()
	head := make([]byte, 64)
	n, _ := io.ReadFull(c, head)
	return photo.VideoType(head[:n])
}

// readVideo reads what a stored video tells about itself, nil when it
// tells nothing the model takes.
func (h itemHandlers) readVideo(id item.ID, a item.Attachment) *item.Photo {
	if a.Type == photo.TypeWebM {
		return nil // WebM is recognised, not read
	}
	c, err := h.d.DB.OpenAttachment(id, a.ID)
	if err != nil {
		return nil
	}
	defer c.Close()
	return photoOf(photo.ReadVideo(c))
}

// videoWithoutPlace stores a stored video again without where it was
// taken, under a new ID, and discards the first (ADR-0177, ADR-0189). A
// video whose place cannot be told apart is refused.
func (h itemHandlers) videoWithoutPlace(id item.ID, a item.Attachment) (item.Attachment, error) {
	c, err := h.d.DB.OpenAttachment(id, a.ID)
	if err != nil {
		return item.Attachment{}, err
	}
	defer c.Close()
	defer h.d.DB.DiscardAttachment(id, a.ID)
	out, ok := photo.VideoWithoutPlace(c)
	if !ok {
		return item.Attachment{}, errPlaceStays
	}
	b, err := h.d.DB.StoreAttachment(id, out, a.Size)
	if err != nil {
		return item.Attachment{}, err
	}
	b.Type = a.Type
	if m := h.readVideo(id, b); m != nil && m.Lat != nil {
		h.d.DB.DiscardAttachment(id, b.ID)
		return item.Attachment{}, errPlaceStays
	}
	return b, nil
}

// octetStream reports whether a request's body is
// application/octet-stream, a type a cross-site form cannot send.
func octetStream(r *http.Request) bool {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return err == nil && mt == "application/octet-stream"
}

// posterTypes are the pictures a poster may be.
var posterTypes = []string{"image/jpeg", "image/png", "image/webp"}

// setPoster keeps a picture the browser took from a video as its poster
// (ADR-0189), for whoever may edit the video's item. The body is the
// picture, application/octet-stream, at most AttachmentBytes.
func (h itemHandlers) setPoster(w http.ResponseWriter, r *http.Request) {
	if !octetStream(r) {
		writeError(w, http.StatusUnsupportedMediaType, "the request body must be application/octet-stream")
		return
	}
	var id item.ID
	var a item.Attachment
	var check error
	h.d.DB.Read(func(s *item.Store) {
		x := s.As(who(r))
		it, err := resolveItem(x, r.PathValue("ref"))
		if err == nil {
			a, err = x.Attachment(it.ID, item.ID(r.PathValue("id")))
		}
		if err == nil {
			err = x.CanAttach(it.ID)
		}
		id, check = it.ID, err
	})
	if check != nil {
		h.fail(w, check)
		return
	}
	if !albumVideo(a) {
		writeError(w, http.StatusBadRequest, "only a video has a poster")
		return
	}
	limit := budget.Get().AttachmentBytes
	p, err := h.d.DB.StorePoster(id, a.ID, http.MaxBytesReader(w, r.Body, limit+1), limit)
	var tooLarge *http.MaxBytesError
	switch {
	case errors.Is(err, storage.ErrAttachmentTooLarge) || errors.As(err, &tooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("a poster may have at most %d bytes", limit))
		return
	case err != nil:
		h.fail(w, err)
		return
	}
	if !slices.Contains(posterTypes, p.Type) {
		h.d.DB.DiscardPoster(id, a.ID)
		writeError(w, http.StatusUnsupportedMediaType, "a poster is a JPEG, PNG or WebP picture")
		return
	}
	h.photos().thumbs.drop(string(a.ID))
	w.WriteHeader(http.StatusNoContent)
}
