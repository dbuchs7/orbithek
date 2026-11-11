package server

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/dbuchs7/orbithek/internal/item"
)

// faceVector is a vector of 128 numbers around seed, written as JSON.
func faceVector(seed float64) string {
	parts := make([]string, 128)
	for i := range parts {
		parts[i] = fmt.Sprintf("%.4f", seed+float64(i%7)/100)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// TestFacesAndPersonAlbums: whoever may edit a picture sets its faces and
// names them; every reader sees the names, the persons and their album.
// Vectors are kept only while recognition is on, suggest names from
// pictures one may read, and are forgotten when it is turned off
// (ADR-0194).
func TestFacesAndPersonAlbums(t *testing.T) {
	w := newPhotoWorld(t)
	lake := w.put(t, "lake.jpg", exifJPEG(t, "2024:05:01 13:22:10", 0, 0, 64))
	town := w.put(t, "town.jpg", exifJPEG(t, "2024:05:02 13:22:10", 0, 0, 66))
	w.put(t, "empty.jpg", exifJPEG(t, "2024:05:03 13:22:10", 0, 0, 68))
	faces := func(pic map[string]any) string {
		return "/api/items/" + w.folder + "/attachments/" + pic["id"].(string) + "/faces"
	}

	w.vera.expect("PUT", faces(lake), `{"faces":[]}`, http.StatusForbidden)
	w.bert.expect("PUT", faces(lake), `{"faces":[{"x":0.9,"y":0.1,"w":0.3,"h":0.2}]}`, http.StatusBadRequest)
	w.bert.expect("PUT", faces(lake), `{"faces":[{"x":0.1,"y":0.1,"w":0.2,"h":0.2,"vector":[1,2]}]}`, http.StatusBadRequest)
	got := w.bert.expect("PUT", faces(lake), `{"faces":[{"x":0.1,"y":0.1,"w":0.2,"h":0.2,"person":"Anna","vector":`+faceVector(0.1)+`},{"x":0.5,"y":0.1,"w":0.2,"h":0.2}]}`, http.StatusOK)
	if got["facesSeen"] != true || len(got["faces"].([]any)) != 2 {
		t.Fatalf("faces %v", got)
	}
	// Without recognition no vector is kept.
	if _, err := w.db.FaceVectors(item.ID(w.folder), item.ID(lake["id"].(string))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a vector kept without recognition: %v", err)
	}
	w.bert.expect("GET", faces(lake)+"/suggestions", "", http.StatusNotFound)

	// Every reader sees the names, the persons and their album.
	page := field(w.vera.expect("GET", "/api/album?folder="+w.folder, "", http.StatusOK), "pictures").([]any)
	var seen map[string]any
	for _, p := range page {
		if a := p.(map[string]any)["attachment"].(map[string]any); a["name"] == "lake.jpg" {
			seen = a
		}
	}
	if seen == nil || seen["facesSeen"] != true || seen["faces"].([]any)[0].(map[string]any)["person"] != "Anna" || seen["faces"].([]any)[0].(map[string]any)["vector"] != nil {
		t.Fatalf("the album's picture %v", seen)
	}
	people := w.vera.expect("GET", "/api/album/people", "", http.StatusOK)["people"].([]any)
	if len(people) != 1 || people[0].(map[string]any)["name"] != "Anna" || people[0].(map[string]any)["pictures"] != 1.0 {
		t.Fatalf("people %v", people)
	}
	if got := names(w.vera, "?person=anna"); len(got) != 1 || got[0] != "lake.jpg" {
		t.Errorf("the album of Anna %v", got)
	}
	if got := names(w.vera, "?q=anna"); len(got) != 1 {
		t.Errorf("searched by a person %v", got)
	}
	if got := w.carla.expect("GET", "/api/album/people", "", http.StatusOK)["people"].([]any); len(got) != 0 {
		t.Errorf("persons of pictures one may not read %v", got)
	}

	// Administrators turn recognition on; vectors are kept and suggest.
	w.vera.expect("PUT", "/api/settings/faces", `{"recognition":true}`, http.StatusForbidden)
	if r := w.admin.expect("PUT", "/api/settings/faces", `{"recognition":true}`, http.StatusOK); r["recognition"] != true {
		t.Fatalf("turned on %v", r)
	}
	if r := w.bert.expect("GET", "/api/album/imports", "", http.StatusOK); r["faces"] != true {
		t.Errorf("imports %v", r)
	}
	w.bert.expect("PUT", faces(lake), `{"faces":[{"x":0.1,"y":0.1,"w":0.2,"h":0.2,"person":"Anna","vector":`+faceVector(0.1)+`},{"x":0.5,"y":0.1,"w":0.2,"h":0.2,"vector":`+faceVector(0.9)+`}]}`, http.StatusOK)
	w.bert.expect("PUT", faces(town), `{"faces":[{"x":0.2,"y":0.2,"w":0.2,"h":0.2,"vector":`+faceVector(0.11)+`},{"x":0.6,"y":0.2,"w":0.2,"h":0.2,"vector":`+faceVector(-0.9)+`}]}`, http.StatusOK)
	s := w.bert.expect("GET", faces(town)+"/suggestions", "", http.StatusOK)["suggestions"].([]any)
	if len(s) != 1 || s[0].(map[string]any)["index"] != 0.0 || s[0].(map[string]any)["person"] != "Anna" {
		t.Fatalf("suggestions %v", s)
	}
	w.vera.expect("GET", faces(town)+"/suggestions", "", http.StatusForbidden)
	// Naming a face keeps its vector: the face of the same box keeps it.
	w.bert.expect("PUT", faces(lake), `{"faces":[{"x":0.1,"y":0.1,"w":0.2,"h":0.2,"person":"Anna"},{"x":0.5,"y":0.1,"w":0.2,"h":0.2,"person":"Ben"}]}`, http.StatusOK)
	v, err := w.db.FaceVectors(item.ID(w.folder), item.ID(lake["id"].(string)))
	if err != nil || len(v) != 2 || len(v[1]) != 128 {
		t.Fatalf("vectors after naming %d %v", len(v), err)
	}
	s = w.bert.expect("GET", faces(town)+"/suggestions", "", http.StatusOK)["suggestions"].([]any)
	if len(s) != 1 {
		t.Errorf("suggestions after naming %v", s)
	}

	// Turned off, every vector is forgotten.
	r := w.admin.expect("PUT", "/api/settings/faces", `{"recognition":false}`, http.StatusOK)
	if r["recognition"] != false || r["forgotten"] != 2.0 {
		t.Fatalf("turned off %v", r)
	}
	if _, err := w.db.FaceVectors(item.ID(w.folder), item.ID(town["id"].(string))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a vector stays after recognition was turned off: %v", err)
	}
	w.bert.expect("GET", faces(town)+"/suggestions", "", http.StatusNotFound)
	if r := w.admin.expect("GET", "/api/settings/faces", "", http.StatusOK); r["recognition"] != false {
		t.Errorf("setting %v", r)
	}
	// The names stay: they are no biometric data.
	if got := names(w.vera, "?person=Ben"); len(got) != 1 {
		t.Errorf("the album of Ben %v", got)
	}
}

// TestFacesSuggestedForAPerson: across the album, the unnamed faces that
// resemble a person's named faces more than anyone else's are suggested
// for that person, most alike first, on the pictures one may edit, and
// only while recognition is on (ADR-0199).
func TestFacesSuggestedForAPerson(t *testing.T) {
	w := newPhotoWorld(t)
	lake := w.put(t, "lake.jpg", exifJPEG(t, "2024:05:01 13:22:10", 0, 0, 64))
	town := w.put(t, "town.jpg", exifJPEG(t, "2024:05:02 13:22:10", 0, 0, 66))
	forest := w.put(t, "forest.jpg", exifJPEG(t, "2024:05:03 13:22:10", 0, 0, 68))
	hill := w.put(t, "hill.jpg", exifJPEG(t, "2024:05:04 13:22:10", 0, 0, 70))
	faces := func(pic map[string]any) string {
		return "/api/items/" + w.folder + "/attachments/" + pic["id"].(string) + "/faces"
	}
	const suggest = "/api/album/people/suggestions?person="
	w.bert.expect("GET", suggest+"Anna", "", http.StatusNotFound)
	w.admin.expect("PUT", "/api/settings/faces", `{"recognition":true}`, http.StatusOK)

	w.bert.expect("PUT", faces(lake), `{"faces":[{"x":0.1,"y":0.1,"w":0.1,"h":0.1,"person":"Anna","vector":`+faceVector(0.1)+`},{"x":0.3,"y":0.1,"w":0.1,"h":0.1,"person":"Ben","vector":`+faceVector(0.3)+`},{"x":0.5,"y":0.1,"w":0.1,"h":0.1,"person":"Cleo","vector":`+faceVector(0.16)+`}]}`, http.StatusOK)
	// Town: the first face is Anna's, the second nobody's, the third Ben's.
	w.bert.expect("PUT", faces(town), `{"faces":[{"x":0.1,"y":0.1,"w":0.1,"h":0.1,"vector":`+faceVector(0.11)+`},{"x":0.3,"y":0.1,"w":0.1,"h":0.1,"vector":`+faceVector(0.9)+`},{"x":0.5,"y":0.1,"w":0.1,"h":0.1,"vector":`+faceVector(0.29)+`}]}`, http.StatusOK)
	// Forest: Anna's, less alike; and a face already named stays out.
	w.bert.expect("PUT", faces(forest), `{"faces":[{"x":0.1,"y":0.1,"w":0.1,"h":0.1,"person":"Dora","vector":`+faceVector(0.1)+`},{"x":0.3,"y":0.1,"w":0.1,"h":0.1,"vector":`+faceVector(0.12)+`}]}`, http.StatusOK)
	// Hill: near enough to Anna, but nearer to Cleo; and one without vector.
	w.bert.expect("PUT", faces(hill), `{"faces":[{"x":0.1,"y":0.1,"w":0.1,"h":0.1,"vector":`+faceVector(0.135)+`},{"x":0.3,"y":0.1,"w":0.1,"h":0.1}]}`, http.StatusOK)

	w.bert.expect("GET", suggest, "", http.StatusBadRequest)
	w.bert.expect("GET", suggest+"Anna&limit=0", "", http.StatusBadRequest)
	r := w.bert.expect("GET", suggest+"anna", "", http.StatusOK)
	got := r["suggestions"].([]any)
	if r["total"] != 2.0 || len(got) != 2 {
		t.Fatalf("suggestions for Anna %v", r)
	}
	first, second := got[0].(map[string]any), got[1].(map[string]any)
	if first["attachment"].(map[string]any)["id"] != town["id"] || second["attachment"].(map[string]any)["id"] != forest["id"] {
		t.Fatalf("not most alike first: %v", got)
	}
	if m := first["matches"].([]any); len(m) != 1 || m[0].(map[string]any)["index"] != 0.0 {
		t.Errorf("matches on town %v", m)
	}
	if m := second["matches"].([]any); len(m) != 1 || m[0].(map[string]any)["index"] != 1.0 {
		t.Errorf("matches on forest %v", m)
	}
	if f := first["faces"].([]any); len(f) != 3 || f[0].(map[string]any)["vector"] != nil {
		t.Errorf("faces of town %v", f)
	}
	if r := w.bert.expect("GET", suggest+"Anna&limit=1", "", http.StatusOK); len(r["suggestions"].([]any)) != 1 || r["total"] != 2.0 {
		t.Errorf("limited %v", r)
	}
	cleo := w.bert.expect("GET", suggest+"Cleo", "", http.StatusOK)["suggestions"].([]any)
	if len(cleo) != 1 || cleo[0].(map[string]any)["attachment"].(map[string]any)["id"] != hill["id"] {
		t.Errorf("suggestions for Cleo %v", cleo)
	}
	if got := w.bert.expect("GET", suggest+"Nobody", "", http.StatusOK)["suggestions"].([]any); len(got) != 0 {
		t.Errorf("suggestions for no one named %v", got)
	}
	// Only pictures one may edit are offered, and only from what one reads.
	if got := w.vera.expect("GET", suggest+"Anna", "", http.StatusOK)["suggestions"].([]any); len(got) != 0 {
		t.Errorf("a reader got suggestions %v", got)
	}
	if got := w.carla.expect("GET", suggest+"Anna", "", http.StatusOK)["suggestions"].([]any); len(got) != 0 {
		t.Errorf("an outsider got suggestions %v", got)
	}
	if got := w.bert.expect("GET", suggest+"Anna&space=OPS", "", http.StatusOK)["suggestions"].([]any); len(got) != 0 {
		t.Errorf("suggestions of another space %v", got)
	}

	// Confirmed, the face is named and keeps its vector; it is no longer
	// suggested.
	w.bert.expect("PUT", faces(town), `{"faces":[{"x":0.1,"y":0.1,"w":0.1,"h":0.1,"person":"Anna"},{"x":0.3,"y":0.1,"w":0.1,"h":0.1},{"x":0.5,"y":0.1,"w":0.1,"h":0.1}]}`, http.StatusOK)
	if r := w.bert.expect("GET", suggest+"Anna", "", http.StatusOK); r["total"] != 1.0 {
		t.Errorf("after confirming %v", r)
	}
	if v, err := w.db.FaceVectors(item.ID(w.folder), item.ID(town["id"].(string))); err != nil || len(v) != 3 || len(v[0]) != 128 {
		t.Errorf("vectors after confirming %d %v", len(v), err)
	}
}
