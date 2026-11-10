package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/dbuchs7/orbithek/internal/photo"
)

// fakePhoton answers as a Photon place search does; it refuses the
// language "it", as a search without Italian names would.
func fakePhoton(t *testing.T, asked *atomic.Int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked.Add(1)
		if r.URL.Query().Get("lang") == "it" {
			http.Error(w, "language not supported", http.StatusBadRequest)
			return
		}
		if !strings.HasPrefix(r.Header.Get("User-Agent"), "Orbithek/") {
			t.Errorf("user agent %q", r.Header.Get("User-Agent"))
		}
		switch r.URL.Path {
		case "/api/":
			w.Write([]byte(`{"type":"FeatureCollection","features":[
				{"geometry":{"type":"Point","coordinates":[7.7491,46.0207]},"properties":{"name":"Zermatt","state":"Valais","country":"Switzerland"}},
				{"geometry":{"type":"Point","coordinates":[7.7491,46.0207]},"properties":{"name":"Zermatt","state":"Valais","country":"Switzerland"}},
				{"geometry":{"type":"Point","coordinates":[7.75,46.02]},"properties":{"street":"Bahnhofstrasse","housenumber":"5","city":"Zermatt","country":"Switzerland"}},
				{"geometry":{"type":"Point","coordinates":[200,46]},"properties":{"name":"Nowhere"}},
				{"geometry":{"type":"Point","coordinates":[7,46]},"properties":{}}]}`))
		case "/reverse":
			w.Write([]byte(`{"features":[{"geometry":{"coordinates":[7.6585,45.9763]},"properties":{"name":"Matterhorn","state":"Valais","country":"Switzerland"}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestPlacesAreSuggestedFromThePlaceSearch: the first letters typed bring
// places named in the reader's language, each once, kept a while; a point
// chosen on a map is named; administrators turn the search off and on;
// without a search named none is asked (ADR-0181).
func TestPlacesAreSuggestedFromThePlaceSearch(t *testing.T) {
	var asked atomic.Int32
	srv := fakePhoton(t, &asked)
	photos := NewPhotos(func(k string) string { return map[string]string{GeocoderVar: srv.URL}[k] }, nil)
	photos.geoClient = srv.Client()
	w := newPhotoWorld(t, func(d *Deps) { d.Photos = photos })

	r := w.vera.expect("GET", "/api/places?q=zerm&lang=de", "", http.StatusOK)
	places := r["places"].([]any)
	if len(places) != 2 || r["credit"] != DefaultMapAttribution {
		t.Fatalf("places %v", r)
	}
	first := places[0].(map[string]any)
	if first["name"] != "Zermatt" || first["label"] != "Zermatt, Valais, Switzerland" || first["lat"] != 46.0207 || first["lon"] != 7.7491 {
		t.Errorf("first %v", first)
	}
	if second := places[1].(map[string]any); second["name"] != "Bahnhofstrasse 5" || second["label"] != "Bahnhofstrasse 5, Zermatt, Switzerland" {
		t.Errorf("second %v", second)
	}
	n := asked.Load()
	w.vera.expect("GET", "/api/places?q=zerm&lang=de", "", http.StatusOK)
	if asked.Load() != n {
		t.Error("the same search was asked again")
	}
	// A language the search does not know falls back to its own names.
	if got := w.vera.expect("GET", "/api/places?q=zerma&lang=it", "", http.StatusOK)["places"].([]any); len(got) != 2 {
		t.Errorf("fallback %v", got)
	}
	for _, bad := range []string{"", "z", strings.Repeat("x", 101)} {
		w.vera.expect("GET", "/api/places?q="+bad, "", http.StatusBadRequest)
	}
	if p := field(w.vera.expect("GET", "/api/places/reverse?lat=45.97&lon=7.65", "", http.StatusOK), "place").(map[string]any); p["name"] != "Matterhorn" {
		t.Errorf("reverse %v", p)
	}
	for _, bad := range []string{"lat=91&lon=0", "lat=0&lon=181", "lat=x&lon=0", "lat=0"} {
		w.vera.expect("GET", "/api/places/reverse?"+bad, "", http.StatusBadRequest)
	}
	if r := w.vera.expect("GET", "/api/album/imports", "", http.StatusOK); r["places"] != true {
		t.Errorf("imports %v", r)
	}

	// Administrators turn it off, and on again; nobody else may.
	w.vera.expect("PUT", "/api/settings/places", `{"search":false}`, http.StatusForbidden)
	w.vera.expect("GET", "/api/settings/places", "", http.StatusForbidden)
	if r := w.admin.expect("PUT", "/api/settings/places", `{"search":false}`, http.StatusOK); r["search"] != false || r["active"] != false || r["geocoder"] != srv.URL {
		t.Errorf("turned off: %v", r)
	}
	w.vera.expect("GET", "/api/places?q=zermatt", "", http.StatusNotFound)
	w.vera.expect("GET", "/api/places/reverse?lat=45&lon=7", "", http.StatusNotFound)
	if r := w.vera.expect("GET", "/api/album/imports", "", http.StatusOK); r["places"] != false {
		t.Errorf("imports while off %v", r)
	}
	if r := w.admin.expect("PUT", "/api/settings/places", `{"search":true}`, http.StatusOK); r["active"] != true {
		t.Errorf("turned on: %v", r)
	}
	w.vera.expect("GET", "/api/places?q=zermatt", "", http.StatusOK)

	// Without a search named, none is asked.
	off := newPhotoWorld(t, func(d *Deps) {
		d.Photos = NewPhotos(func(k string) string { return map[string]string{GeocoderVar: "off"}[k] }, nil)
	})
	off.vera.expect("GET", "/api/places?q=zermatt", "", http.StatusNotFound)
	if NewPhotos(func(string) string { return "" }, nil).GeocoderURL != DefaultGeocoder {
		t.Error("the default place search")
	}
}

// TestAPlaceSetByHandThroughTheAPI: an editor sets a photo's place with
// its name and removes it; readers then download the file without the
// file's own place, the uploader keeps the original; a reader may not set
// one, nor is a file that is no picture given a place (ADR-0181).
func TestAPlaceSetByHandThroughTheAPI(t *testing.T) {
	w := newPhotoWorld(t)
	lake := w.put(t, "lake.jpg", exifJPEG(t, "2024:05:01 13:22:10", 46.5584, 7.8352, 64))
	file := "/api/items/" + w.folder + "/attachments/" + lake["id"].(string)
	got := w.bert.expect("PUT", file+"/geotag", `{"lat":46.0207,"lon":7.7491,"name":"Zermatt"}`, http.StatusOK)
	p := field(got, "attachment", "photo").(map[string]any)
	if p["lat"] != 46.0207 || p["placeName"] != "Zermatt" || p["placeManual"] != true || p["taken"] != "2024-05-01T13:22:10" {
		t.Errorf("photo data %v", p)
	}
	if m := photo.Read(bytes.NewReader(w.vera.do("GET", file, "").Body.Bytes()), "image/jpeg"); m.Lat != nil {
		t.Error("vera's download carries the file's own place")
	}
	if m := photo.Read(bytes.NewReader(w.bert.do("GET", file, "").Body.Bytes()), "image/jpeg"); m.Lat == nil {
		t.Error("the uploader lost the original")
	}
	places := field(w.vera.expect("GET", "/api/album/places", "", http.StatusOK), "places").([]any)
	if len(places) != 1 || places[0].(map[string]any)["lat"] != 46.0207 {
		t.Errorf("the map shows %v", places)
	}
	w.vera.expect("PUT", file+"/geotag", `{"lat":1,"lon":1}`, http.StatusForbidden)
	w.bert.expect("PUT", file+"/geotag", `{"lat":1}`, http.StatusBadRequest)
	w.bert.expect("PUT", file+"/geotag", `{"lat":100,"lon":1}`, http.StatusBadRequest)
	got = w.bert.expect("DELETE", file+"/geotag", "", http.StatusOK)
	if p := field(got, "attachment", "photo").(map[string]any); p["lat"] != nil || p["placeManual"] != true {
		t.Errorf("removed: %v", p)
	}
	if places := field(w.vera.expect("GET", "/api/album/places", "", http.StatusOK), "places").([]any); len(places) != 0 {
		t.Errorf("the map still shows %v", places)
	}
	txt := w.put(t, "notes.txt", []byte("plain text"))
	w.bert.expect("PUT", "/api/items/"+w.folder+"/attachments/"+txt["id"].(string)+"/geotag", `{"lat":1,"lon":1}`, http.StatusBadRequest)
	// An older picture without recorded data gets them with the place.
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 100)...)
	scan := w.put(t, "scan.png", png)
	got = w.bert.expect("PUT", "/api/items/"+w.folder+"/attachments/"+scan["id"].(string)+"/geotag", `{"lat":46,"lon":7,"name":"Somewhere"}`, http.StatusOK)
	if p := field(got, "attachment", "photo").(map[string]any); p["placeName"] != "Somewhere" {
		t.Errorf("older picture %v", p)
	}
	var out map[string]any
	_ = json.Unmarshal(w.vera.do("GET", "/api/album/imports", "").Body.Bytes(), &out)
}

func TestPhotonAnswersAreReadCarefully(t *testing.T) {
	var a photonAnswer
	_ = json.Unmarshal([]byte(`{"features":[{"geometry":{"coordinates":[1]},"properties":{"name":"short"}},
		{"geometry":{"coordinates":[1,2]},"properties":{"name":"`+strings.Repeat("x", 300)+`"}}]}`), &a)
	got := a.places()
	if len(got) != 1 || len([]rune(got[0].Name)) != 200 {
		t.Errorf("places %v", got)
	}
}
