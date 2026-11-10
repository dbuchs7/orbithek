package server

import (
	"archive/zip"
	"bytes"
	"cmp"
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dbuchs7/orbithek/internal/audit"
	"github.com/dbuchs7/orbithek/internal/auth"
	"github.com/dbuchs7/orbithek/internal/budget"
	"github.com/dbuchs7/orbithek/internal/item"
	"github.com/dbuchs7/orbithek/internal/photo"
	"github.com/dbuchs7/orbithek/internal/photosource"
	"github.com/dbuchs7/orbithek/internal/storage"
)

// photoRoutes are the routes of the album's photo management (ADR-0177).
// None is an MCP tool: thumbnails, tiles and archives are for browsers,
// imports act with a person's sign-in at another service, and favourites
// are a person's own.
func photoRoutes(h itemHandlers) []Route {
	return []Route{
		{Method: http.MethodGet, Pattern: "/api/items/{ref}/attachments/{id}/thumbnail", Summary: "A picture scaled to edge (400 or 1600) pixels on its longer edge and turned upright, with the access of the file; a picture that cannot be scaled is answered whole", Handler: http.HandlerFunc(h.thumbnail)},
		{Method: http.MethodGet, Pattern: "/api/items/{ref}/attachments/{id}/photo", Summary: "What a picture tells about itself: capture time, place, camera and exposure; recorded says whether it was recorded when the file was stored, else it is read from the file now", Handler: http.HandlerFunc(h.photoData)},
		{Method: http.MethodGet, Pattern: "/api/album/months", Summary: "The pictures of the album counted per month they were taken in, newest first, narrowed as the album is (space, folder, label, favourites, place)", Handler: http.HandlerFunc(h.albumMonths)},
		{Method: http.MethodGet, Pattern: "/api/album/places", Summary: "The readable pictures that tell where they were taken, at most the budget AlbumPlaces, narrowed as the album is; tiles says whether the map has tiles", Handler: http.HandlerFunc(h.albumPlaces)},
		{Method: http.MethodPut, Pattern: "/api/album/favourites", Summary: "Mark a picture as one's own favourite, or unmark it: item, file, favourite; a favourite stays through later versions of the file", Handler: http.HandlerFunc(h.setPictureFavourite)},
		{Method: http.MethodPost, Pattern: "/api/album/zip", Summary: "A ZIP archive of chosen pictures (pictures: item and file each), each read with the caller's rights; at most the budgets AlbumZipPictures and AlbumZipBytes", Handler: http.HandlerFunc(h.albumZip)},
		{Method: http.MethodGet, Pattern: "/api/map/tiles/{z}/{x}/{y}", Summary: "A tile of the album's map, fetched by the server from the tile service in OT_MAP_TILES and kept a while; nothing when none is named", Handler: http.HandlerFunc(h.mapTile)},
		{Method: http.MethodGet, Pattern: "/api/settings/places", Summary: "Whether places set by hand are suggested from the place search: search (the administrators' setting), geocoder (the search the operator names in OT_GEOCODER, empty for none) and active (administrators)", Handler: http.HandlerFunc(h.placeSettings)},
		{Method: http.MethodPut, Pattern: "/api/settings/places", Summary: "Turn the place search of places set by hand off or on: search (administrators)", Handler: http.HandlerFunc(h.setPlaceSettings)},
		{Method: http.MethodGet, Pattern: "/api/places", Summary: "Places suggested for the first letters of a name (q, 2 to 100 characters; lang de, en, fr or it), from the place search in OT_GEOCODER: name, label, lat and lon", Handler: http.HandlerFunc(h.searchPlaces)},
		{Method: http.MethodGet, Pattern: "/api/places/reverse", Summary: "The place at a point chosen on a map (lat, lon; lang), from the place search in OT_GEOCODER", Handler: http.HandlerFunc(h.reversePlace)},
		{Method: http.MethodPut, Pattern: "/api/items/{ref}/attachments/{id}/geotag", Summary: "Set where a photo was taken by hand: lat, lon and name; who may edit its item may, unless its uploader keeps the place from them. Everyone but the uploader then gets the file without the file's own place", Handler: http.HandlerFunc(h.setGeotag)},
		{Method: http.MethodDelete, Pattern: "/api/items/{ref}/attachments/{id}/geotag", Summary: "Remove where a photo was taken by hand; everyone but the uploader then gets the file without the file's own place", Handler: http.HandlerFunc(h.removeGeotag)},
		{Method: http.MethodGet, Pattern: "/api/album/imports", Summary: "Which imports the server offers: google (Google Photos through its picker; connected says whether one is signed in at Google now) and icloud (shared iCloud albums)", Handler: http.HandlerFunc(h.photoImports)},
		{Method: http.MethodGet, Pattern: "/api/album/google/connect", Summary: "Sign in at Google to bring photos into the album; redirects to Google, which asks to let this server read the photos one chooses, and back to the folder (folder)", Handler: http.HandlerFunc(h.googleConnect)},
		{Method: http.MethodGet, Pattern: GooglePhotosCallback, Summary: "Google's redirect back after one allowed or refused an import; bound to the browser that began it, it goes on to the album", Access: Public, Handler: http.HandlerFunc(h.googlePhotosCallback)},
		{Method: http.MethodPost, Pattern: "/api/album/google/sessions", Summary: "Open a picker at Google: answers its id and the address where one chooses photos", Handler: http.HandlerFunc(h.googleSession)},
		{Method: http.MethodGet, Pattern: "/api/album/google/sessions/{id}", Summary: "Whether the photos are chosen in a picker; once they are, the chosen media (id, name, type, taken, video)", Handler: http.HandlerFunc(h.googlePoll)},
		{Method: http.MethodPost, Pattern: "/api/album/google/sessions/{id}/import", Summary: "Bring one chosen photo (media) into a folder (folder) as one's own upload; skipped when the folder holds the same picture; place is private (kept to oneself) or strip (removed)", Handler: http.HandlerFunc(h.googleImport)},
		{Method: http.MethodDelete, Pattern: "/api/album/google/sessions/{id}", Summary: "Close a picker at Google", Handler: http.HandlerFunc(h.googleClose)},
		{Method: http.MethodPost, Pattern: "/api/album/icloud/albums", Summary: "Read a shared iCloud album by its public address (url): its name and photos (guid, caption, created, video)", Handler: http.HandlerFunc(h.icloudAlbum)},
		{Method: http.MethodPost, Pattern: "/api/album/icloud/import", Summary: "Bring one photo (photo, its guid) of a shared iCloud album read before (url) into a folder (folder) as one's own upload; skipped when the folder holds the same picture; place is private (kept to oneself) or strip (removed)", Handler: http.HandlerFunc(h.icloudImport)},
	}
}

// photos is the album's photo management, made when the server is set up
// without one, so every handler finds it.
func (h itemHandlers) photos() *Photos {
	if h.d.Photos != nil {
		return h.d.Photos
	}
	return fallbackPhotos
}

var fallbackPhotos = NewPhotos(func(string) string { return "" }, nil)

// --- photo data -----------------------------------------------------------

// photoOf turns what a file tells into the attachment's photo data, nil
// when it tells nothing.
func photoOf(m photo.Meta) *item.Photo {
	p := &item.Photo{Taken: m.Taken, Offset: m.Offset, Lat: m.Lat, Lon: m.Lon, Alt: m.Alt, Make: m.Make, Model: m.Model, Lens: m.Lens,
		Exposure: m.Exposure, FNumber: m.FNumber, ISO: m.ISO, Focal: m.Focal, Focal35: m.Focal35, Orientation: m.Orientation, Width: m.Width, Height: m.Height}
	if p.Lat == nil {
		p.Alt = nil // a height without a place says nothing the album uses
	}
	if *p == (item.Photo{}) || !item.ValidPhoto(p) {
		return nil
	}
	return p
}

// readPhoto reads the photo data of a stored picture, nil for any other
// file or one that tells nothing. Data the model would refuse is left out
// rather than refusing the file.
func (h itemHandlers) readPhoto(id item.ID, a item.Attachment) *item.Photo {
	if !inlineTypes[a.Type] {
		return nil
	}
	c, err := h.d.DB.OpenAttachment(id, a.ID)
	if err != nil {
		return nil
	}
	defer c.Close()
	return photoOf(photo.Read(c, a.Type))
}

// errPlaceStays refuses a picture whose place could not be removed.
var errPlaceStays = errors.New("the place this picture was taken could not be removed from it, so it was not stored")

// withoutPlace reads a picture whole, up to limit bytes (one more tells it
// is too large), and answers it without where it was taken (ADR-0177): the
// GPS data of its EXIF emptied and its XMP dropped. What is no picture is
// answered as it is; a picture that still tells a place afterwards is
// refused, never stored with it.
func withoutPlace(body io.Reader, limit int64) (io.Reader, error) {
	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, storage.ErrAttachmentTooLarge
	}
	kind := http.DetectContentType(data)
	if !inlineTypes[kind] {
		return bytes.NewReader(data), nil
	}
	out := photo.StripPlace(data, kind)
	if m := photo.Read(bytes.NewReader(out), kind); m.Lat != nil || m.Lon != nil {
		return nil, errPlaceStays
	}
	return bytes.NewReader(out), nil
}

func (h itemHandlers) photoData(w http.ResponseWriter, r *http.Request) {
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
	p, recorded := a.Photo, a.Photo != nil
	if !recorded {
		p = h.readPhoto(it.ID, a)
	}
	writeJSON(w, http.StatusOK, map[string]any{"photo": p, "recorded": recorded})
}

// --- thumbnails -----------------------------------------------------------

// thumbnailEdges are the sizes thumbnails are made in: the album's grid
// and the viewer.
var thumbnailEdges = map[string]int{"400": 400, "1600": 1600}

func (h itemHandlers) thumbnail(w http.ResponseWriter, r *http.Request) {
	edge, ok := thumbnailEdges[cmp.Or(r.URL.Query().Get("edge"), "400")]
	if !ok {
		writeError(w, http.StatusBadRequest, "edge is 400 or 1600")
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
	if !inlineTypes[a.Type] {
		writeError(w, http.StatusNotFound, "the file is no picture")
		return
	}
	etag := strconv.Quote(a.SHA256 + "-" + strconv.Itoa(edge))
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	original := fmt.Sprintf("/api/items/%s/attachments/%s", it.ID, a.ID)
	p := h.photos()
	key := string(a.ID) + ":" + strconv.Itoa(edge)
	data, kind, ok := p.thumbs.get(key)
	if !ok {
		data, kind, err = h.makeThumbnail(r.Context(), p, it.ID, a, edge)
		switch {
		case errors.Is(err, photo.ErrUnsupported), errors.Is(err, photo.ErrTooLarge):
			http.Redirect(w, r, original, http.StatusFound)
			return
		case err != nil:
			h.log.Warn("thumbnail not made", "item", it.ID, "attachment", a.ID, "err", err)
			http.Redirect(w, r, original, http.StatusFound)
			return
		}
		p.thumbs.put(key, kind, data)
	}
	hd := w.Header()
	hd.Set("Content-Type", kind)
	hd.Set("X-Content-Type-Options", "nosniff")
	hd.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	hd.Set("Cache-Control", "private, max-age=86400")
	hd.Set("ETag", etag)
	hd.Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// makeThumbnail scales a picture down, waiting for a free worker.
func (h itemHandlers) makeThumbnail(ctx context.Context, p *Photos, id item.ID, a item.Attachment, edge int) ([]byte, string, error) {
	select {
	case p.working <- struct{}{}:
		defer func() { <-p.working }()
	case <-ctx.Done():
		return nil, "", ctx.Err()
	}
	c, err := h.d.DB.OpenAttachment(id, a.ID)
	if err != nil {
		return nil, "", err
	}
	defer c.Close()
	orientation := 0
	if a.Photo != nil {
		orientation = a.Photo.Orientation
	} else {
		orientation = photo.Read(c, a.Type).Orientation
		if _, err := c.Seek(0, io.SeekStart); err != nil {
			return nil, "", err
		}
	}
	return photo.Thumbnail(c, a.Type, edge, orientation, budget.Get().ThumbnailPixels)
}

// --- the album's views ----------------------------------------------------

// albumFilter narrows the album (ADR-0054, ADR-0090, ADR-0177).
type albumFilter struct {
	space, kind, text, label, day, from, to string
	folder                                  item.ID
	taken                                   bool
	place                                   bool
	// favourites are the caller's, by the first version of the file: a
	// file moved to another item keeps its ID and stays a favourite
	// (ADR-0188). only is whether the album shows them alone.
	favourites map[item.ID]bool
	only       bool
	// inRule is a collection's rule (ADR-0175), nil for none.
	inRule func(item.Picture) bool
}

// withCollection narrows the filter to what a collection gathers, when
// ref names one.
func withCollection(x *item.Session, f *albumFilter, ref string) error {
	if ref == "" {
		return nil
	}
	c, err := resolveItem(x, ref)
	if err != nil {
		return err
	}
	_, rule, err := x.Collection(c.ID)
	if err != nil {
		return err
	}
	f.inRule = rule.Matches
	return nil
}

var (
	dayRE  = regexp.MustCompile(`^(0[1-9]|1[0-2])-(0[1-9]|[12]\d|3[01])$`)
	dateRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

// albumFilterOf reads the album's narrowing from a request.
func (h itemHandlers) albumFilterOf(r *http.Request) (albumFilter, error) {
	q := r.URL.Query()
	f := albumFilter{space: q.Get("space"), kind: q.Get("kind"), text: strings.ToLower(strings.TrimSpace(q.Get("q"))), folder: item.ID(q.Get("folder")),
		day: q.Get("day"), from: q.Get("from"), to: q.Get("to"), taken: q.Get("order") == "taken", place: q.Get("place") == "on", only: q.Get("favourites") == "on"}
	f.label, _ = item.NormalLabel(q.Get("label"))
	switch {
	case f.kind != "" && f.kind != "image" && f.kind != "drawing":
		return f, errors.New("kind is image or drawing")
	case q.Get("order") != "" && q.Get("order") != "added" && q.Get("order") != "taken":
		return f, errors.New("order is added or taken")
	case f.day != "" && !dayRE.MatchString(f.day):
		return f, errors.New("day is a month and day, MM-DD")
	case f.from != "" && !dateRE.MatchString(f.from), f.to != "" && !dateRE.MatchString(f.to):
		return f, errors.New("from and to are days, YYYY-MM-DD")
	}
	f.favourites = map[item.ID]bool{}
	for p := range h.d.DB.Accounts.PictureFavourites(auth.UserID(who(r))) {
		f.favourites[item.ID(p.File)] = true
	}
	return f, nil
}

// albumStacks lists the stacks of versions the filter keeps (ADR-0103):
// the versions of one file, newest on top. A stack shows when any of its
// versions matches the words, labels and folder, and when its top matches
// the time, place and favourites. Stacks come newest first: by the time
// their top was taken, or by when it was added.
func albumStacks(x *item.Session, f albumFilter) [][]item.Picture {
	keep := func(a item.Attachment) bool {
		drawing := strings.HasSuffix(a.Name, DrawingSuffix)
		return inlineTypes[a.Type] && (f.kind == "" || (f.kind == "drawing") == drawing)
	}
	match := func(p item.Picture) bool {
		return (f.inRule == nil || f.inRule(p)) && (f.folder == "" || p.Item.ID == f.folder) && (f.label == "" || slices.Contains(p.Attachment.Labels, f.label)) &&
			(f.text == "" || strings.Contains(strings.ToLower(p.Attachment.Name+" "+strings.Join(p.Attachment.Labels, " ")+" "+p.Item.Key+" "+p.Item.Title+" "+cameraText(p.Attachment.Photo)), f.text))
	}
	type stackKey struct{ item, first item.ID }
	var stacks [][]item.Picture
	at := map[stackKey]int{}
	matched := map[int]bool{}
	for _, p := range x.Pictures(f.space, keep) {
		k := stackKey{p.Item.ID, p.First}
		i, ok := at[k]
		if !ok {
			i = len(stacks)
			at[k] = i
			stacks = append(stacks, nil)
		}
		stacks[i] = append(stacks[i], p)
		if match(p) {
			matched[i] = true
		}
	}
	var out [][]item.Picture
	for i, st := range stacks {
		top := st[0]
		day := top.Attachment.Photo.Day(top.Attachment.Created)
		switch {
		case !matched[i],
			f.only && !f.favourites[top.First],
			f.place && !top.Attachment.Photo.HasPlace(),
			f.day != "" && day[5:] != f.day,
			f.from != "" && day < f.from,
			f.to != "" && day > f.to:
			continue
		}
		out = append(out, st)
	}
	if f.taken {
		// A stable sort keeps stacks taken at the same moment in the order
		// they were added, newest first.
		slices.SortStableFunc(out, func(a, b []item.Picture) int {
			ta, _ := a[0].Attachment.Photo.At(a[0].Attachment.Created)
			tb, _ := b[0].Attachment.Photo.At(b[0].Attachment.Created)
			return tb.Compare(ta)
		})
	}
	return out
}

// cameraText is the camera of a photo, searched with the album's words.
func cameraText(p *item.Photo) string {
	if p == nil {
		return ""
	}
	return p.Make + " " + p.Model + " " + p.Lens
}

// takenJSON is when a picture was taken as the album tells it: the day,
// the moment, and whether the picture told it (exact) or the album took
// the day it was added.
type takenJSON struct {
	Day   string    `json:"day"`
	At    time.Time `json:"at"`
	Exact bool      `json:"exact"`
}

func takenOf(a item.Attachment) takenJSON {
	at, exact := a.Photo.At(a.Created)
	told := a.Photo != nil && a.Photo.Taken != ""
	return takenJSON{Day: a.Photo.Day(a.Created), At: at, Exact: exact || told}
}

func (h itemHandlers) albumMonths(w http.ResponseWriter, r *http.Request) {
	f, err := h.albumFilterOf(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.read(w, r, func(x *item.Session) (any, error) {
		if err := withCollection(x, &f, r.URL.Query().Get("collection")); err != nil {
			return nil, err
		}
		count := map[string]int{}
		for _, st := range albumStacks(x, f) {
			count[st[0].Attachment.Photo.Day(st[0].Attachment.Created)[:7]]++
		}
		out := []map[string]any{}
		for _, m := range slices.Sorted(func(yield func(string) bool) {
			for k := range count {
				if !yield(k) {
					return
				}
			}
		}) {
			out = append(out, map[string]any{"month": m, "count": count[m]})
		}
		slices.Reverse(out)
		return map[string]any{"months": out}, nil
	})
}

func (h itemHandlers) albumPlaces(w http.ResponseWriter, r *http.Request) {
	f, err := h.albumFilterOf(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	f.place = true
	limit := budget.Get().AlbumPlaces
	h.read(w, r, func(x *item.Session) (any, error) {
		if err := withCollection(x, &f, r.URL.Query().Get("collection")); err != nil {
			return nil, err
		}
		all := albumStacks(x, f)
		out := []map[string]any{}
		for _, st := range all[:min(len(all), limit)] {
			p := st[0]
			out = append(out, map[string]any{"item": p.Item.ID, "key": p.Item.Key, "file": p.Attachment.ID, "first": p.First, "name": p.Attachment.Name,
				"lat": *p.Attachment.Photo.Lat, "lon": *p.Attachment.Photo.Lon, "taken": takenOf(p.Attachment)})
		}
		return map[string]any{"places": out, "total": len(all), "truncated": len(all) > limit, "tiles": h.photos().TileURL != "", "credit": h.photos().TileCredit}, nil
	})
}

func (h itemHandlers) setPictureFavourite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Item      item.ID `json:"item"`
		File      item.ID `json:"file"`
		Favourite bool    `json:"favourite"`
	}
	if !decode(w, r, &req) {
		return
	}
	var first item.ID
	var err error
	h.d.DB.Read(func(s *item.Store) {
		var d item.Drawing
		if d, err = s.As(who(r)).Drawing(req.Item, req.File); err == nil {
			first = d.First
		}
	})
	if err != nil {
		h.fail(w, err)
		return
	}
	if err := h.d.DB.Accounts.SetPictureFavourite(auth.UserID(who(r)), auth.PictureRef{Item: string(req.Item), File: string(first)}, req.Favourite); err != nil {
		if errors.Is(err, auth.ErrInvalid) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.log.Warn("favourite not kept", "err", err)
		writeError(w, http.StatusInternalServerError, "the favourite was not kept")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": req.Item, "first": first, "favourite": req.Favourite})
}

// --- ZIP archives ---------------------------------------------------------

func (h itemHandlers) albumZip(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Pictures []struct {
			Item item.ID `json:"item"`
			File item.ID `json:"file"`
		} `json:"pictures"`
	}
	if !decode(w, r, &req) {
		return
	}
	b := budget.Get()
	if len(req.Pictures) == 0 || len(req.Pictures) > b.AlbumZipPictures {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("choose 1 to %d pictures", b.AlbumZipPictures))
		return
	}
	type entry struct {
		item item.ID
		a    item.Attachment
		// hidden: its uploader keeps the place from the caller (ADR-0178).
		hidden bool
	}
	var entries []entry
	var total int64
	var err error
	h.d.DB.Read(func(s *item.Store) {
		x := s.As(who(r))
		for _, p := range req.Pictures {
			var a item.Attachment
			if a, err = x.Attachment(p.Item, p.File); err != nil {
				return
			}
			entries = append(entries, entry{p.Item, a, x.PlaceHidden(p.Item, p.File)})
			total += a.Size
		}
	})
	if err != nil {
		h.fail(w, err)
		return
	}
	if total > b.AlbumZipBytes {
		writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("the chosen pictures have more than %d bytes together; choose fewer", b.AlbumZipBytes))
		return
	}
	hd := w.Header()
	hd.Set("Content-Type", "application/zip")
	// Quoted, as the UI's download reads the name (assets/api.js).
	hd.Set("Content-Disposition", `attachment; filename="album.zip"`)
	hd.Set("X-Content-Type-Options", "nosniff")
	zw := zip.NewWriter(w)
	names := map[string]bool{}
	for _, e := range entries {
		name := uniqueName(names, e.a.Name)
		// Pictures are compressed already: they are stored as they are.
		fw, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store, Modified: time.Unix(e.a.Created, 0).UTC()})
		if err != nil {
			return
		}
		c, err := h.openFor(e.hidden, e.item, e.a)
		if err != nil {
			h.log.Error("attachment content not given", "item", e.item, "attachment", e.a.ID, "err", err)
			continue
		}
		_, err = io.Copy(fw, c)
		c.Close()
		if err != nil {
			// The status is sent; a broken archive shows as a failed download.
			h.log.Warn("album archive cut off", "err", err)
			return
		}
	}
	zw.Close()
}

// uniqueName gives a file name not yet in the archive: "name (2).jpg".
func uniqueName(names map[string]bool, name string) string {
	out := name
	ext := ""
	if i := strings.LastIndex(name, "."); i > 0 {
		name, ext = name[:i], name[i:]
	}
	for n := 2; names[out]; n++ {
		out = fmt.Sprintf("%s (%d)%s", name, n, ext)
	}
	names[out] = true
	return out
}

// --- map tiles ------------------------------------------------------------

// tileTypes are the formats a tile may come in.
var tileTypes = map[string]bool{"image/png": true, "image/jpeg": true, "image/webp": true}

// maxTileBytes bounds one tile the server fetches; tiles are small, and a
// larger answer is not a tile.
const maxTileBytes = 1 << 20

// maxTileZoom is the deepest zoom of the album's map.
const maxTileZoom = 19

func (h itemHandlers) mapTile(w http.ResponseWriter, r *http.Request) {
	p := h.photos()
	if p.TileURL == "" {
		writeError(w, http.StatusNotFound, "the map draws no tiles here; "+MapTilesVar+" is off")
		return
	}
	z, errZ := strconv.Atoi(r.PathValue("z"))
	x, errX := strconv.Atoi(r.PathValue("x"))
	y, errY := strconv.Atoi(strings.TrimSuffix(r.PathValue("y"), ".png"))
	if errZ != nil || errX != nil || errY != nil || z < 0 || z > maxTileZoom || x < 0 || y < 0 || x >= 1<<z || y >= 1<<z {
		writeError(w, http.StatusBadRequest, "no such tile")
		return
	}
	key := fmt.Sprintf("%d/%d/%d", z, x, y)
	data, kind, ok := p.tiles.get(key)
	if !ok {
		var err error
		data, kind, err = p.fetchTile(r.Context(), z, x, y)
		if err != nil {
			h.log.Warn("map tile not fetched", "tile", key, "err", err)
			writeError(w, http.StatusBadGateway, "the tile service did not answer")
			return
		}
		p.tiles.put(key, kind, data)
	}
	hd := w.Header()
	hd.Set("Content-Type", kind)
	hd.Set("X-Content-Type-Options", "nosniff")
	hd.Set("Cache-Control", "private, max-age=604800") // a week, as tile services ask
	w.Write(data)
}

// fetchTile reads one tile from the tile service, naming this server as
// tile services ask.
func (p *Photos) fetchTile(ctx context.Context, z, x, y int) ([]byte, string, error) {
	u := strings.NewReplacer("{z}", strconv.Itoa(z), "{x}", strconv.Itoa(x), "{y}", strconv.Itoa(y)).Replace(p.TileURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", p.tileAgent)
	resp, err := p.tileClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	kind, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if resp.StatusCode != http.StatusOK || !tileTypes[kind] {
		return nil, "", fmt.Errorf("status %d, type %q", resp.StatusCode, kind)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxTileBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) > maxTileBytes {
		return nil, "", errors.New("the tile is too large")
	}
	return data, kind, nil
}

// --- imports --------------------------------------------------------------

func (h itemHandlers) photoImports(w http.ResponseWriter, r *http.Request) {
	p := h.photos()
	writeJSON(w, http.StatusOK, map[string]any{
		"google":    p.Google != nil,
		"connected": p.Google != nil && p.Google.Connected(who(r)),
		"icloud":    p.ICloud != nil,
		"tiles":     p.TileURL != "",
		"places":    h.placeSearch(),
		"credit":    p.TileCredit,
	})
}

// importInto stores a picture brought from another service into a folder,
// as the caller's upload: only a picture the album shows, not one the
// folder holds already. fallback is what the service tells when the file
// does not: the time it was taken.
func (h itemHandlers) importInto(w http.ResponseWriter, r *http.Request, folder, name string, body io.Reader, taken time.Time, place string) {
	var id item.ID
	var check error
	h.d.DB.Read(func(s *item.Store) {
		x := s.As(who(r))
		it, err := resolveItem(x, folder)
		if err == nil {
			id, err = it.ID, x.CanAttach(it.ID)
		}
		check = err
	})
	if check != nil {
		h.fail(w, check)
		return
	}
	limit := budget.Get().AttachmentBytes
	if place == placeStrip {
		var err error
		if body, err = withoutPlace(body, limit); err != nil {
			h.storeFail(w, err, limit)
			return
		}
	}
	a, err := h.d.DB.StoreAttachment(id, body, limit)
	switch {
	case errors.Is(err, errPlaceStays):
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	case errors.Is(err, storage.ErrAttachmentTooLarge), errors.Is(err, photosource.ErrTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("a file may have at most %d bytes", limit))
		return
	case errors.Is(err, photosource.ErrUnreadable), errors.Is(err, photosource.ErrNotConnected):
		writeError(w, http.StatusBadGateway, "the photo could not be read from the service")
		return
	case err != nil:
		h.fail(w, err)
		return
	}
	if !inlineTypes[a.Type] {
		h.d.DB.DiscardAttachment(id, a.ID)
		writeError(w, http.StatusUnsupportedMediaType, "the album takes JPEG, PNG, GIF and WebP; export HEIC photos as JPEG")
		return
	}
	a.Name = importName(name, a.Type)
	a.Photo = h.readPhoto(id, a)
	if (a.Photo == nil || a.Photo.Taken == "") && !taken.IsZero() {
		if a.Photo == nil {
			a.Photo = &item.Photo{}
		}
		a.Photo.Taken, a.Photo.Offset = taken.UTC().Format("2006-01-02T15:04:05"), "+00:00"
	}
	if place == placePrivate {
		a.Photo = keepPrivate(a.Photo)
	}
	h.write(w, r, http.StatusCreated, func(x *item.Session, _ *item.Store) (any, error) {
		have, err := x.Attachments(id)
		if err != nil {
			h.d.DB.DiscardAttachment(id, a.ID)
			return nil, err
		}
		if i := slices.IndexFunc(have, func(b item.Attachment) bool { return b.SHA256 == a.SHA256 }); i >= 0 {
			h.d.DB.DiscardAttachment(id, a.ID)
			return map[string]any{"skipped": true, "attachment": toAttachmentJSON(have[i])}, nil
		}
		got, err := x.AddAttachment(id, a)
		if err != nil {
			h.d.DB.DiscardAttachment(id, a.ID)
			return nil, err
		}
		return map[string]any{"skipped": false, "attachment": toAttachmentJSON(got)}, nil
	})
}

// storeFail answers a picture that could not be read to remove its place.
func (h itemHandlers) storeFail(w http.ResponseWriter, err error, limit int64) {
	var tooLarge *http.MaxBytesError
	switch {
	case errors.Is(err, errPlaceStays):
		writeError(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, storage.ErrAttachmentTooLarge), errors.Is(err, photosource.ErrTooLarge), errors.As(err, &tooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("a file may have at most %d bytes", limit))
	case errors.Is(err, photosource.ErrUnreadable), errors.Is(err, photosource.ErrNotConnected):
		writeError(w, http.StatusBadGateway, "the photo could not be read from the service")
	default:
		h.fail(w, err)
	}
}

// importName is a file name the item model takes: the service's name
// without slashes or control characters, or one made from the type.
func importName(name, kind string) string {
	name = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r < 0x20 || r == 0x7f {
			return '_'
		}
		return r
	}, strings.TrimSpace(name))
	if r := []rune(name); len(r) > 200 {
		name = string(r[:200])
	}
	if name == "" {
		name = "photo"
	}
	if !strings.Contains(name, ".") {
		switch kind {
		case "image/jpeg":
			name += ".jpg"
		case "image/png":
			name += ".png"
		case "image/gif":
			name += ".gif"
		case "image/webp":
			name += ".webp"
		}
	}
	return name
}

// photosFail answers an error of another photo service.
func (h itemHandlers) photosFail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, photosource.ErrInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, photosource.ErrNotFound):
		writeError(w, http.StatusNotFound, "the service does not know it, or it is not public")
	case errors.Is(err, photosource.ErrNotConnected):
		writeError(w, http.StatusUnauthorized, "sign in at Google again to bring photos in")
	case errors.Is(err, photosource.ErrTooMany):
		writeError(w, http.StatusTooManyRequests, err.Error())
	case errors.Is(err, photosource.ErrVideo):
		writeError(w, http.StatusUnsupportedMediaType, "the album takes photos, not videos")
	case errors.Is(err, photosource.ErrTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, err.Error())
	case errors.Is(err, context.Canceled):
		writeError(w, http.StatusRequestTimeout, "the request was cancelled")
	default:
		h.log.Warn("photo service", "err", err)
		writeError(w, http.StatusBadGateway, "the photo service did not answer as expected")
	}
}

// googlePhotosCookie binds a begun sign-in at Google to the browser that
// began it; googleReturnCookie names the folder to go back to.
const (
	googlePhotosCookie = "ot_photos_state"
	googleReturnCookie = "ot_photos_folder"
)

var folderKeyRE = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func (h itemHandlers) google(w http.ResponseWriter) *photosource.Google {
	g := h.photos().Google
	if g == nil {
		writeError(w, http.StatusNotFound, "imports from Google Photos are not set up on this server")
	}
	return g
}

func (h itemHandlers) googleConnect(w http.ResponseWriter, r *http.Request) {
	g := h.google(w)
	if g == nil {
		return
	}
	authURL, state, err := g.Begin(who(r))
	if err != nil {
		h.photosFail(w, err)
		return
	}
	cookie := func(name, value string) {
		http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/api/album/google/", MaxAge: int((10 * time.Minute).Seconds()),
			HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: isSecure(r)})
	}
	cookie(googlePhotosCookie, state)
	if f := r.URL.Query().Get("folder"); folderKeyRE.MatchString(f) {
		cookie(googleReturnCookie, f)
	}
	http.Redirect(w, r, authURL, http.StatusSeeOther)
}

// toAlbum goes on to the album, at the folder a sign-in began in, telling
// how the sign-in ended.
func toAlbum(w http.ResponseWriter, r *http.Request, folder, outcome string) {
	q := "photos=" + outcome
	if folderKeyRE.MatchString(folder) {
		q = "folder=" + folder + "&" + q
	}
	http.Redirect(w, r, "/#/album?"+q, http.StatusSeeOther)
}

func (h itemHandlers) googlePhotosCallback(w http.ResponseWriter, r *http.Request) {
	folder := ""
	if c, err := r.Cookie(googleReturnCookie); err == nil {
		folder = c.Value
	}
	for _, name := range []string{googlePhotosCookie, googleReturnCookie} {
		http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/api/album/google/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: isSecure(r)})
	}
	q := r.URL.Query()
	state := q.Get("state")
	c, err := r.Cookie(googlePhotosCookie)
	g := h.photos().Google
	if err != nil || state == "" || g == nil || subtle.ConstantTimeCompare([]byte(c.Value), []byte(state)) != 1 {
		toAlbum(w, r, folder, "google-error")
		return
	}
	user, ok := g.Pending(state)
	if !ok {
		toAlbum(w, r, folder, "google-error")
		return
	}
	if q.Get("error") != "" {
		g.Cancel(state)
		toAlbum(w, r, folder, "google-denied")
		return
	}
	if _, err := g.Finish(r.Context(), state, q.Get("code")); err != nil {
		h.log.Warn("signing in at Google for an import failed", "err", err)
		toAlbum(w, r, folder, "google-error")
		return
	}
	h.d.DB.Audit.Record(audit.Entry{Event: audit.PhotosGoogleConnected, Actor: user, Addr: clientAddr(r)})
	toAlbum(w, r, folder, "google")
}

func (h itemHandlers) googleSession(w http.ResponseWriter, r *http.Request) {
	g := h.google(w)
	if g == nil {
		return
	}
	s, err := g.NewSession(r.Context(), who(r))
	if err != nil {
		h.photosFail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": s.ID, "pickerUri": s.PickerURI, "pollInterval": s.PollInterval.Milliseconds(), "timeout": s.Timeout.Milliseconds()})
}

func (h itemHandlers) googlePoll(w http.ResponseWriter, r *http.Request) {
	g := h.google(w)
	if g == nil {
		return
	}
	s, err := g.Poll(r.Context(), who(r), r.PathValue("id"))
	if err != nil {
		h.photosFail(w, err)
		return
	}
	out := map[string]any{"id": s.ID, "picked": s.Picked, "pollInterval": s.PollInterval.Milliseconds()}
	if s.Picked {
		media, err := g.Items(r.Context(), who(r), s.ID)
		if err != nil {
			h.photosFail(w, err)
			return
		}
		items := []map[string]any{}
		for _, m := range media {
			items = append(items, map[string]any{"id": m.ID, "name": m.Name, "type": m.Type, "taken": m.Taken, "video": m.Video, "width": m.Width, "height": m.Height})
		}
		out["items"] = items
	}
	writeJSON(w, http.StatusOK, out)
}

func (h itemHandlers) googleImport(w http.ResponseWriter, r *http.Request) {
	g := h.google(w)
	if g == nil {
		return
	}
	var req struct {
		Media  string `json:"media"`
		Folder string `json:"folder"`
		// Place keeps where the photo was taken (empty), keeps it to the
		// caller (private, ADR-0178) or removes it (strip, ADR-0177);
		// Strip is the older way of asking to remove it.
		Place string `json:"place"`
		Strip bool   `json:"strip"`
	}
	if !decode(w, r, &req) {
		return
	}
	if !h.mayImportInto(w, r, req.Folder) {
		return
	}
	body, m, err := g.Download(r.Context(), who(r), r.PathValue("id"), req.Media, budget.Get().AttachmentBytes)
	if err != nil {
		h.photosFail(w, err)
		return
	}
	defer body.Close()
	h.importInto(w, r, req.Folder, m.Name, body, m.Taken, placeChoice(req.Place, req.Strip))
}

// mayImportInto refuses an import into an item the caller may not add
// files to before anything is fetched from another service.
func (h itemHandlers) mayImportInto(w http.ResponseWriter, r *http.Request, folder string) bool {
	var err error
	h.d.DB.Read(func(s *item.Store) {
		x := s.As(who(r))
		var it item.Item
		if it, err = resolveItem(x, folder); err == nil {
			err = x.CanAttach(it.ID)
		}
	})
	if err != nil {
		h.fail(w, err)
		return false
	}
	return true
}

func (h itemHandlers) googleClose(w http.ResponseWriter, r *http.Request) {
	g := h.google(w)
	if g == nil {
		return
	}
	g.Close(r.Context(), who(r), r.PathValue("id"))
	w.WriteHeader(http.StatusNoContent)
}

func (h itemHandlers) icloud(w http.ResponseWriter) *photosource.ICloud {
	c := h.photos().ICloud
	if c == nil {
		writeError(w, http.StatusNotFound, "imports from shared iCloud albums are not set up on this server")
	}
	return c
}

// readSharedAlbum is a shared album by its address: the one read before,
// while it is at hand, else read now.
func (h itemHandlers) readSharedAlbum(r *http.Request, c *photosource.ICloud, address string) (string, photosource.Album, error) {
	token, err := photosource.AlbumToken(address)
	if err != nil {
		return "", photosource.Album{}, err
	}
	p := h.photos()
	if a, ok := p.album(who(r), token); ok {
		return token, a, nil
	}
	a, err := c.Album(r.Context(), token)
	if err != nil {
		return "", photosource.Album{}, err
	}
	p.keepAlbum(who(r), token, a)
	return token, a, nil
}

func (h itemHandlers) icloudAlbum(w http.ResponseWriter, r *http.Request) {
	c := h.icloud(w)
	if c == nil {
		return
	}
	var req struct {
		URL string `json:"url"`
	}
	if !decode(w, r, &req) {
		return
	}
	token, err := photosource.AlbumToken(req.URL)
	if err != nil {
		h.photosFail(w, err)
		return
	}
	// Giving the address again reads the album again: that is updating.
	a, err := c.Album(r.Context(), token)
	if err != nil {
		h.photosFail(w, err)
		return
	}
	h.photos().keepAlbum(who(r), token, a)
	photos := []map[string]any{}
	for _, p := range a.Photos {
		photos = append(photos, map[string]any{"guid": p.GUID, "caption": p.Caption, "created": p.Created, "video": p.Video, "width": p.Width, "height": p.Height})
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": a.Name, "owner": a.Owner, "photos": photos})
}

func (h itemHandlers) icloudImport(w http.ResponseWriter, r *http.Request) {
	c := h.icloud(w)
	if c == nil {
		return
	}
	var req struct {
		URL    string `json:"url"`
		Photo  string `json:"photo"`
		Folder string `json:"folder"`
		// Place keeps where the photo was taken (empty), keeps it to the
		// caller (private, ADR-0178) or removes it (strip, ADR-0177);
		// Strip is the older way of asking to remove it.
		Place string `json:"place"`
		Strip bool   `json:"strip"`
	}
	if !decode(w, r, &req) {
		return
	}
	if !h.mayImportInto(w, r, req.Folder) {
		return
	}
	token, a, err := h.readSharedAlbum(r, c, req.URL)
	if err != nil {
		h.photosFail(w, err)
		return
	}
	i := slices.IndexFunc(a.Photos, func(p photosource.SharedPhoto) bool { return p.GUID == req.Photo })
	if i < 0 {
		writeError(w, http.StatusNotFound, "the album holds no such photo")
		return
	}
	p := a.Photos[i]
	body, err := c.Download(r.Context(), token, p, budget.Get().AttachmentBytes)
	if err != nil {
		h.photosFail(w, err)
		return
	}
	defer body.Close()
	name := p.GUID + ".jpg"
	if p.Caption != "" {
		name = strings.TrimSpace(p.Caption)
	}
	h.importInto(w, r, req.Folder, name, body, p.Created, placeChoice(req.Place, req.Strip))
}
