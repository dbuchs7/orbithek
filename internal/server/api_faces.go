package server

import (
	"errors"
	"math"
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/dbuchs7/orbithek/internal/audit"
	"github.com/dbuchs7/orbithek/internal/item"
	"github.com/dbuchs7/orbithek/internal/site"
	"github.com/dbuchs7/orbithek/internal/storage"
)

// Faces of the album (ADR-0194): the browser finds faces on a picture and
// computes, where administrators turned recognition on, a vector of each
// to recognise it; whoever may edit the picture names them. The journal
// keeps the boxes and names, which every reader of the picture sees;
// the vectors, biometric data, are kept apart beside the file, never
// leave the server again and are forgotten when recognition is turned
// off. The server compares them to suggest names.

// faceRoutes are the routes of faces and persons.
func faceRoutes(h itemHandlers) []Route {
	return []Route{
		{Method: http.MethodPut, Pattern: "/api/items/{ref}/attachments/{id}/faces", Summary: "Set the faces found on one version of a picture (faces: x, y, w and h as fractions of the picture from its top left corner, person, and vector, 128 numbers computed to recognise it, kept only while recognition is on) and mark it as searched; who may edit the item may (ADR-0194)", Handler: http.HandlerFunc(h.setFaces)},
		{Method: http.MethodGet, Pattern: "/api/items/{ref}/attachments/{id}/faces/suggestions", Summary: "Names suggested for the unnamed faces of a picture, from the named faces of the pictures one may read that resemble them (index, person, distance); only while recognition is on, for who may edit the item (ADR-0194)", Handler: http.HandlerFunc(h.faceSuggestions)},
		{Method: http.MethodGet, Pattern: "/api/album/people", Summary: "The persons named on the pictures one may read, by name: name, pictures (how many show them) and the newest picture with the face as a cover (space)", Handler: http.HandlerFunc(h.albumPeople)},
		{Method: http.MethodGet, Pattern: "/api/settings/faces", Summary: "Whether faces are recognised: recognition (administrators)", Handler: http.HandlerFunc(h.faceSettings)},
		{Method: http.MethodPut, Pattern: "/api/settings/faces", Summary: "Turn the recognition of faces on or off (recognition); turning it off forgets every vector computed of a face (administrators)", Handler: http.HandlerFunc(h.setFaceSettings)},
	}
}

// faceJSON is a face as the API takes and gives it.
type faceJSON struct {
	X      float64   `json:"x"`
	Y      float64   `json:"y"`
	W      float64   `json:"w"`
	H      float64   `json:"h"`
	Person string    `json:"person,omitempty"`
	Vector []float32 `json:"vector,omitempty"`
}

func toFacesJSON(faces []item.Face) []faceJSON {
	if len(faces) == 0 {
		return nil
	}
	out := make([]faceJSON, len(faces))
	for i, f := range faces {
		out[i] = faceJSON{X: f.X, Y: f.Y, W: f.W, H: f.H, Person: f.Person}
	}
	return out
}

// recognition reports whether administrators turned face recognition on.
func (h itemHandlers) recognition() bool {
	return h.d.DB != nil && h.d.DB.Site.Faces().Recognition
}

// setFaces sets the faces of a picture. Vectors are kept only while
// recognition is on; a face sent without one keeps the vector of the face
// of the same box it had, so naming a face keeps what recognises it.
func (h itemHandlers) setFaces(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Faces []faceJSON `json:"faces"`
	}
	if !decode(w, r, &req) {
		return
	}
	faces := make([]item.Face, len(req.Faces))
	for i, f := range req.Faces {
		if len(f.Vector) != 0 && len(f.Vector) != storage.FaceVectorLen {
			writeError(w, http.StatusBadRequest, "a face's vector has 128 numbers")
			return
		}
		faces[i] = item.Face{X: f.X, Y: f.Y, W: f.W, H: f.H, Person: f.Person}
	}
	picture := item.ID(r.PathValue("id"))
	var before, after item.Attachment
	var id item.ID
	done := false
	h.write(w, r, http.StatusOK, func(x *item.Session, _ *item.Store) (any, error) {
		it, err := resolveItem(x, r.PathValue("ref"))
		if err != nil {
			return nil, err
		}
		id = it.ID
		if before, err = x.Attachment(it.ID, picture); err != nil {
			return nil, err
		}
		if err := x.SetPictureFaces(it.ID, picture, faces); err != nil {
			return nil, err
		}
		if after, err = x.Attachment(it.ID, picture); err != nil {
			return nil, err
		}
		done = true
		return map[string]any{"faces": toFacesJSON(after.Faces), "facesSeen": after.FacesSeen}, nil
	})
	// The vectors are files of storage, written once the store is free.
	if done {
		h.keepVectors(id, picture, before.Faces, after.Faces, req.Faces)
	}
}

// keepVectors stores the vectors of a picture's faces as they are now
// (kept), from those sent, or else those of the face of the same box
// before; without recognition it removes them.
func (h itemHandlers) keepVectors(id, picture item.ID, before, kept []item.Face, sent []faceJSON) {
	p := h.photos()
	defer p.forgetVectors(picture)
	if !h.recognition() {
		h.d.DB.DiscardFaceVectors(id, picture)
		return
	}
	old, _ := h.d.DB.FaceVectors(id, picture)
	vectors := make([][]float32, len(kept))
	for i := range kept {
		if i < len(sent) && len(sent[i].Vector) > 0 {
			vectors[i] = sent[i].Vector
			continue
		}
		if j := slices.IndexFunc(before, func(f item.Face) bool { return sameBox(f, kept[i]) }); j >= 0 && j < len(old) {
			vectors[i] = old[j]
		}
	}
	if err := h.d.DB.StoreFaceVectors(id, picture, vectors); err != nil {
		h.log.Warn("face vectors not kept", "err", err)
	}
}

func sameBox(a, b item.Face) bool { return a.X == b.X && a.Y == b.Y && a.W == b.W && a.H == b.H }

// forgetVectors drops what is kept in memory of a picture's vectors, or
// of every picture's for "".
func (p *Photos) forgetVectors(picture item.ID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if picture == "" {
		clear(p.vectors)
		return
	}
	delete(p.vectors, picture)
}

// maxKeptVectors bounds the pictures whose vectors are kept in memory.
const maxKeptVectors = 20000

// vectorsOf reads the vectors of a picture's faces, kept in memory once
// read.
func (h itemHandlers) vectorsOf(id, picture item.ID) [][]float32 {
	p := h.photos()
	p.mu.Lock()
	v, ok := p.vectors[picture]
	p.mu.Unlock()
	if ok {
		return v
	}
	v, err := h.d.DB.FaceVectors(id, picture)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		h.log.Warn("face vectors not read", "err", err)
	}
	p.mu.Lock()
	if len(p.vectors) < maxKeptVectors {
		p.vectors[picture] = v
	}
	p.mu.Unlock()
	return v
}

// faceMatch is the distance below which two faces count as one person:
// the vendored network places faces of one person within about 0.6 of
// each other; a little less suggests fewer wrong names.
const faceMatch = 0.55

// suggestionJSON is a name suggested for one face of a picture.
type suggestionJSON struct {
	Index    int     `json:"index"`
	Person   string  `json:"person"`
	Distance float64 `json:"distance"`
}

// faceSuggestions suggests names for the unnamed faces of a picture from
// the named faces of the pictures the caller may read.
func (h itemHandlers) faceSuggestions(w http.ResponseWriter, r *http.Request) {
	if !h.recognition() {
		writeError(w, http.StatusNotFound, "faces are not recognised here: administrators have not turned recognition on")
		return
	}
	picture := item.ID(r.PathValue("id"))
	var it item.Item
	var a item.Attachment
	var named []item.Picture
	var err error
	h.d.DB.Read(func(s *item.Store) {
		x := s.As(who(r))
		if it, err = resolveItem(x, r.PathValue("ref")); err != nil {
			return
		}
		if !x.MayEdit(it.ID) {
			err = item.ErrForbidden
			return
		}
		if a, err = x.Attachment(it.ID, picture); err != nil {
			return
		}
		named = x.Pictures("", func(b item.Attachment) bool {
			return b.ID != picture && slices.ContainsFunc(b.Faces, func(f item.Face) bool { return f.Person != "" })
		})
	})
	if err != nil {
		h.fail(w, err)
		return
	}
	mine := h.vectorsOf(it.ID, picture)
	out := []suggestionJSON{}
	best := make([]suggestionJSON, len(a.Faces))
	for i := range best {
		best[i] = suggestionJSON{Index: i, Distance: math.Inf(1)}
	}
	for _, p := range named {
		theirs := h.vectorsOf(p.Item.ID, p.Attachment.ID)
		for j, f := range p.Attachment.Faces {
			if f.Person == "" || j >= len(theirs) || len(theirs[j]) == 0 {
				continue
			}
			for i, g := range a.Faces {
				if g.Person != "" || i >= len(mine) || len(mine[i]) == 0 {
					continue
				}
				if d := distance(mine[i], theirs[j]); d < best[i].Distance {
					best[i].Person, best[i].Distance = f.Person, d
				}
			}
		}
	}
	for _, b := range best {
		if b.Person != "" && b.Distance < faceMatch {
			b.Distance = math.Round(b.Distance*1000) / 1000
			out = append(out, b)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"suggestions": out})
}

// distance is the Euclidean distance of two vectors of one length.
func distance(a, b []float32) float64 {
	if len(a) != len(b) {
		return math.Inf(1)
	}
	var sum float64
	for i := range a {
		d := float64(a[i] - b[i])
		sum += d * d
	}
	return math.Sqrt(sum)
}

// albumPeople lists the persons named on the pictures the caller may read.
func (h itemHandlers) albumPeople(w http.ResponseWriter, r *http.Request) {
	space := r.URL.Query().Get("space")
	h.read(w, r, func(x *item.Session) (any, error) {
		out := []map[string]any{}
		for _, p := range x.People(space) {
			c := p.Cover
			out = append(out, map[string]any{"name": p.Name, "pictures": p.Pictures,
				"cover": map[string]any{"item": map[string]any{"id": c.Item.ID, "key": c.Item.Key}, "attachment": map[string]any{"id": c.Attachment.ID, "name": c.Attachment.Name, "type": c.Attachment.Type},
					"video": albumVideo(c.Attachment), "poster": albumVideo(c.Attachment) && h.d.DB.HasPoster(c.Item.ID, c.Attachment.ID)},
				"face": faceJSON{X: p.Face.X, Y: p.Face.Y, W: p.Face.W, H: p.Face.H}})
		}
		return map[string]any{"people": out}, nil
	})
}

// faceSettings answers whether faces are recognised (administrators).
func (h itemHandlers) faceSettings(w http.ResponseWriter, r *http.Request) {
	if !h.adminOnly(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"recognition": h.recognition()})
}

// setFaceSettings turns face recognition on or off (administrators).
// Turning it off forgets every vector computed of a face.
func (h itemHandlers) setFaceSettings(w http.ResponseWriter, r *http.Request) {
	if !h.adminOnly(w, r) {
		return
	}
	var req struct {
		Recognition bool `json:"recognition"`
	}
	if !decode(w, r, &req) {
		return
	}
	before := h.recognition()
	if err := h.d.DB.Site.SetFaces(who(r), site.Faces{Recognition: req.Recognition}); err != nil {
		writeError(w, http.StatusInternalServerError, "setting not saved")
		return
	}
	forgotten := 0
	if !req.Recognition {
		n, err := h.d.DB.ForgetFaceVectors()
		if err != nil {
			h.log.Warn("face vectors not all forgotten", "err", err)
		}
		forgotten = n
		h.photos().forgetVectors("")
	}
	if before != req.Recognition {
		state := map[bool]string{true: "on", false: "off"}[req.Recognition]
		h.d.DB.Audit.Record(audit.Entry{Event: audit.FacesChanged, Actor: who(r), Addr: clientAddr(r), Detail: map[string]string{"recognition": state}})
	}
	writeJSON(w, http.StatusOK, map[string]any{"recognition": h.recognition(), "forgotten": forgotten})
}

// personText is the persons on a picture, searched with the album's words.
func personText(faces []item.Face) string {
	var b strings.Builder
	for _, f := range faces {
		if f.Person != "" {
			b.WriteString(" " + f.Person)
		}
	}
	return b.String()
}
