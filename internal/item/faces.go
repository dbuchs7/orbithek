package item

import (
	"cmp"
	"math"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Face is a face found on a picture (ADR-0194): its box, as fractions of
// the picture's width and height from its top left corner, and the person
// someone named it, if anyone did. The box is all the journal keeps of a
// face; what the browser computed to recognise it is kept apart, beside
// the file, and can be forgotten (storage.FaceVectors).
type Face struct {
	X, Y, W, H float64
	Person     string `json:",omitempty"`
}

// MaxFaces bounds the faces of one picture; maxPersonRunes the length of
// a person's name.
const (
	MaxFaces       = 100
	maxPersonRunes = 80
)

// NormalPerson makes a person's name as it is kept: spaces collapsed and
// trimmed, at most 80 characters of text, no control characters. An empty
// name is no person.
func NormalPerson(name string) (string, error) {
	if !utf8.ValidString(name) {
		return "", invalid("a person's name must be UTF-8 text")
	}
	name = strings.Join(strings.Fields(name), " ")
	if utf8.RuneCountInString(name) > maxPersonRunes {
		return "", invalid("a person's name has at most %d characters", maxPersonRunes)
	}
	if strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return "", invalid("a person's name holds no control characters")
	}
	return name, nil
}

// checkFaces validates the faces of a picture and writes their names as
// they are kept.
func checkFaces(faces []Face) ([]Face, error) {
	if len(faces) > MaxFaces {
		return nil, invalid("a picture has at most %d faces", MaxFaces)
	}
	out := make([]Face, 0, len(faces))
	for _, f := range faces {
		for _, v := range []float64{f.X, f.Y, f.W, f.H} {
			if math.IsNaN(v) || v < 0 || v > 1 {
				return nil, invalid("a face's box lies within the picture, from 0 to 1")
			}
		}
		if f.W < 0.005 || f.H < 0.005 || f.X+f.W > 1.0001 || f.Y+f.H > 1.0001 {
			return nil, invalid("a face's box lies within the picture and has a size")
		}
		p, err := NormalPerson(f.Person)
		if err != nil {
			return nil, err
		}
		// Four decimal places are a tenth of a pixel on a picture of
		// 10,000 pixels; more only makes the journal longer.
		round := func(v float64) float64 { return math.Round(v*1e4) / 1e4 }
		out = append(out, Face{X: round(f.X), Y: round(f.Y), W: round(f.W), H: round(f.H), Person: p})
	}
	if len(out) == 0 {
		return nil, nil // as the journal reads it back
	}
	return out, nil
}

// SetPictureFaces sets the faces found on one version of a picture and the
// persons named on them (ADR-0194), and marks it as searched for faces,
// also when none were found. Whoever may edit the item may.
func (x *Session) SetPictureFaces(id, attachment ID, faces []Face) error {
	faces, err := checkFaces(faces)
	if err != nil {
		return err
	}
	if _, err := x.editable(id); err != nil {
		return err
	}
	it, _, err := x.stackOf(id, attachment)
	if err != nil {
		return err
	}
	if it.Presence != Live {
		return invalid("%s is %s", it.Key, it.Presence)
	}
	a, _ := it.attachment(attachment)
	if a.FacesSeen && slices.Equal(a.Faces, faces) {
		return nil
	}
	return x.s.record(x.who, Event{Kind: AttachmentFacesSet, Item: id, Attachment: &Attachment{ID: attachment, Faces: faces, FacesSeen: true}})
}

// Person is a name given to faces, with how many pictures the principal
// may read show it, and the newest of them with the face, for a cover.
type Person struct {
	Name     string
	Pictures int
	Cover    Picture
	Face     Face
}

// People lists the persons named on the pictures the principal may read,
// in space (every space when empty), by name (ADR-0194). A picture counts
// once for a person however often it shows them; only the newest version
// of a stack counts, as the album shows it.
func (x *Session) People(space string) []Person {
	by := map[string]*Person{}
	for _, p := range newestVersions(x.Pictures(space, func(a Attachment) bool { return len(a.Faces) > 0 })) {
		seen := map[string]bool{}
		for _, f := range p.Attachment.Faces {
			if f.Person == "" || seen[strings.ToLower(f.Person)] {
				continue
			}
			key := strings.ToLower(f.Person)
			seen[key] = true
			if by[key] == nil {
				by[key] = &Person{Name: f.Person, Cover: p, Face: f}
			}
			by[key].Pictures++
		}
	}
	out := make([]Person, 0, len(by))
	for _, p := range by {
		out = append(out, *p)
	}
	slices.SortFunc(out, func(a, b Person) int {
		return cmp.Or(cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)), cmp.Compare(a.Name, b.Name))
	})
	return out
}

// newestVersions keeps of each stack its newest version, the pictures
// being newest first as Pictures returns them.
func newestVersions(pics []Picture) []Picture {
	seen := map[[2]ID]bool{}
	out := pics[:0:0]
	for _, p := range pics {
		k := [2]ID{p.Item.ID, p.First}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, p)
	}
	return out
}

// Shows reports whether a picture shows a person by that name, whatever
// its case.
func (a Attachment) Shows(person string) bool {
	return slices.ContainsFunc(a.Faces, func(f Face) bool { return f.Person != "" && strings.EqualFold(f.Person, person) })
}

// UnnamedFaces lists the pictures in space (every space when empty) whose
// faces a name can be given to across the album (ADR-0199): of each stack
// its newest version, as the album shows it, that the principal may edit
// and that has a face no one named yet.
func (x *Session) UnnamedFaces(space string) []Picture {
	var out []Picture
	for _, p := range newestVersions(x.Pictures(space, func(Attachment) bool { return true })) {
		if p.Editable && slices.ContainsFunc(p.Attachment.Faces, func(f Face) bool { return f.Person == "" }) {
			out = append(out, p)
		}
	}
	return out
}
