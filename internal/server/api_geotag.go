package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dbuchs7/orbithek/internal/audit"
	"github.com/dbuchs7/orbithek/internal/item"
	"github.com/dbuchs7/orbithek/internal/site"
)

// Places set by hand (ADR-0181): suggestions from a Photon place search,
// the name of a point chosen on a map, and setting or removing a photo's
// place. The server asks the search, so readers' browsers reach no other
// site, and keeps its answers a while.

// maxPlaceAnswer bounds one answer of the place search; it lists a few
// places, and a larger answer is not one.
const maxPlaceAnswer = 1 << 20

// placeLanguages are the languages the place search names places in;
// others get its default names.
var placeLanguages = map[string]bool{"de": true, "en": true, "fr": true, "it": true}

// placeJSON is a place as the album suggests it.
type placeJSON struct {
	Name  string  `json:"name"`
	Label string  `json:"label"`
	Lat   float64 `json:"lat"`
	Lon   float64 `json:"lon"`
}

// photonAnswer is the part of a Photon answer the album reads: GeoJSON
// points with their names.
type photonAnswer struct {
	Features []struct {
		Geometry struct {
			Coordinates []float64 `json:"coordinates"`
		} `json:"geometry"`
		Properties struct {
			Name    string `json:"name"`
			Street  string `json:"street"`
			Number  string `json:"housenumber"`
			City    string `json:"city"`
			Town    string `json:"town"`
			Village string `json:"village"`
			State   string `json:"state"`
			Country string `json:"country"`
		} `json:"properties"`
	} `json:"features"`
}

// places turns a Photon answer into the album's places: those with a
// point in range, named by their name or street, labelled with where they
// lie, each label once.
func (a photonAnswer) places() []placeJSON {
	out := []placeJSON{}
	seen := map[string]bool{}
	for _, f := range a.Features {
		c := f.Geometry.Coordinates
		if len(c) < 2 || math.IsNaN(c[0]) || math.IsNaN(c[1]) || math.Abs(c[1]) > 90 || math.Abs(c[0]) > 180 {
			continue
		}
		p := f.Properties
		name := strings.TrimSpace(p.Name)
		if name == "" {
			name = strings.TrimSpace(strings.TrimSpace(p.Street + " " + p.Number))
		}
		if name == "" {
			continue
		}
		parts := []string{name}
		for _, part := range []string{p.City, p.Town, p.Village, p.State, p.Country} {
			if part = strings.TrimSpace(part); part != "" && !containsFold(parts, part) {
				parts = append(parts, part)
			}
		}
		label := clip(strings.Join(parts, ", "), 200)
		if seen[label] {
			continue
		}
		seen[label] = true
		out = append(out, placeJSON{Name: clip(name, 200), Label: label, Lat: c[1], Lon: c[0]})
	}
	return out
}

func containsFold(list []string, s string) bool {
	for _, l := range list {
		if strings.EqualFold(l, s) {
			return true
		}
	}
	return false
}

// clip shortens a text to at most n characters.
func clip(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// askPlaces asks the place search at path with q, in lang when it names
// places in it, and keeps the answer.
func (p *Photos) askPlaces(ctx context.Context, path string, q url.Values, lang string) ([]placeJSON, error) {
	if placeLanguages[lang] {
		q.Set("lang", lang)
	}
	u := p.GeocoderURL + path + "?" + q.Encode()
	if data, _, ok := p.places.get(u); ok {
		var out []placeJSON
		if json.Unmarshal(data, &out) == nil {
			return out, nil
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", p.tileAgent)
	resp, err := p.geoClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusBadRequest && q.Get("lang") != "" {
		// A search that does not name places in the language answers in
		// its default names.
		resp.Body.Close()
		q.Del("lang")
		return p.askPlaces(ctx, path, q, "")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxPlaceAnswer+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxPlaceAnswer {
		return nil, errors.New("the answer is too large")
	}
	var a photonAnswer
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, errors.New("the answer cannot be read")
	}
	out := a.places()
	if kept, err := json.Marshal(out); err == nil {
		p.places.put(u, "application/json", kept)
	}
	return out, nil
}

// placeSearch reports whether places are searched: the operator names a
// search (OT_GEOCODER) and administrators have not turned it off.
func (h itemHandlers) placeSearch() bool {
	return h.photos().GeocoderURL != "" && (h.d.DB == nil || !h.d.DB.Site.Places().SearchOff)
}

// noPlaces answers that places are not searched here.
func noPlaces(w http.ResponseWriter) {
	writeError(w, http.StatusNotFound, "places are not searched here: "+GeocoderVar+" is off, or administrators turned the place search off")
}

// placeSettings answers whether places are searched, the search the
// operator names, and the administrators' setting (ADR-0181).
func (h itemHandlers) placeSettings(w http.ResponseWriter, r *http.Request) {
	if !h.adminOnly(w, r) {
		return
	}
	p := h.photos()
	writeJSON(w, http.StatusOK, map[string]any{"search": !h.d.DB.Site.Places().SearchOff, "geocoder": p.GeocoderURL, "active": h.placeSearch()})
}

// setPlaceSettings turns the place search off or on (administrators).
func (h itemHandlers) setPlaceSettings(w http.ResponseWriter, r *http.Request) {
	if !h.adminOnly(w, r) {
		return
	}
	var req struct {
		Search bool `json:"search"`
	}
	if !decode(w, r, &req) {
		return
	}
	before := !h.d.DB.Site.Places().SearchOff
	if err := h.d.DB.Site.SetPlaces(who(r), site.Places{SearchOff: !req.Search}); err != nil {
		writeError(w, http.StatusInternalServerError, "setting not saved")
		return
	}
	if before != req.Search {
		state := "off"
		if req.Search {
			state = "on"
		}
		h.d.DB.Audit.Record(audit.Entry{Event: audit.PlacesChanged, Actor: who(r), Addr: clientAddr(r), Detail: map[string]string{"search": state}})
	}
	h.placeSettings(w, r)
}

// searchPlaces suggests places for the first letters typed (ADR-0181).
func (h itemHandlers) searchPlaces(w http.ResponseWriter, r *http.Request) {
	p := h.photos()
	if !h.placeSearch() {
		noPlaces(w)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if n := utf8.RuneCountInString(q); n < 2 || n > 100 {
		writeError(w, http.StatusBadRequest, "q is 2 to 100 characters")
		return
	}
	out, err := p.askPlaces(r.Context(), "/api/", url.Values{"q": {q}, "limit": {"8"}}, r.URL.Query().Get("lang"))
	if err != nil {
		h.log.Warn("place search", "err", err)
		writeError(w, http.StatusBadGateway, "the place search did not answer")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"places": out, "credit": DefaultMapAttribution})
}

// coordinate reads a latitude or longitude from a request.
func coordinate(s string, limit float64) (float64, bool) {
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil && !math.IsNaN(v) && !math.IsInf(v, 0) && math.Abs(v) <= limit
}

// reversePlace names the place at a point chosen on a map (ADR-0181).
func (h itemHandlers) reversePlace(w http.ResponseWriter, r *http.Request) {
	p := h.photos()
	if !h.placeSearch() {
		noPlaces(w)
		return
	}
	q := r.URL.Query()
	lat, okLat := coordinate(q.Get("lat"), 90)
	lon, okLon := coordinate(q.Get("lon"), 180)
	if !okLat || !okLon {
		writeError(w, http.StatusBadRequest, "lat is -90 to 90, lon -180 to 180")
		return
	}
	// Points are named to about ten metres, so nearby clicks share an
	// answer.
	f := func(v float64) string { return strconv.FormatFloat(v, 'f', 4, 64) }
	out, err := p.askPlaces(r.Context(), "/reverse", url.Values{"lat": {f(lat)}, "lon": {f(lon)}, "limit": {"1"}}, q.Get("lang"))
	if err != nil {
		h.log.Warn("place search", "err", err)
		writeError(w, http.StatusBadGateway, "the place search did not answer")
		return
	}
	var place *placeJSON
	if len(out) > 0 {
		place = &out[0]
	}
	writeJSON(w, http.StatusOK, map[string]any{"place": place})
}

// setGeotag sets where a photo was taken by hand, with the place's name
// (ADR-0181).
func (h itemHandlers) setGeotag(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Lat  *float64 `json:"lat"`
		Lon  *float64 `json:"lon"`
		Name string   `json:"name"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Lat == nil || req.Lon == nil {
		writeError(w, http.StatusBadRequest, "a place needs lat and lon; DELETE removes the place")
		return
	}
	h.placeByHand(w, r, req.Lat, req.Lon, req.Name)
}

// removeGeotag removes a photo's place by hand (ADR-0181).
func (h itemHandlers) removeGeotag(w http.ResponseWriter, r *http.Request) {
	h.placeByHand(w, r, nil, nil, "")
}

func (h itemHandlers) placeByHand(w http.ResponseWriter, r *http.Request, lat, lon *float64, name string) {
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
	if !inlineTypes[a.Type] {
		writeError(w, http.StatusBadRequest, "only a picture has a place")
		return
	}
	var read *item.Photo
	if a.Photo == nil {
		read = h.readPhoto(it.ID, a)
	}
	h.write(w, r, http.StatusOK, func(x *item.Session, _ *item.Store) (any, error) {
		if err := x.SetPlace(it.ID, a.ID, lat, lon, name, read); err != nil {
			return nil, err
		}
		got, err := x.Attachment(it.ID, a.ID)
		return map[string]attachmentJSON{"attachment": toAttachmentJSON(got)}, err
	})
}
