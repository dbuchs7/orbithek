package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/dbuchs7/orbithek/internal/budget"
	"github.com/dbuchs7/orbithek/internal/item"
	"github.com/dbuchs7/orbithek/internal/photo"
	"github.com/dbuchs7/orbithek/internal/photosource"
)

// exifJPEG is a small JPEG photo that tells when (EXIF DateTimeOriginal,
// "2006:01:02 15:04:05") and, unless lat and lon are both 0, where it was
// taken; size makes photos of the same time differ.
func exifJPEG(t *testing.T, taken string, lat, lon float64, size int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, size, size/2))
	for i := range img.Pix {
		img.Pix[i] = byte(i * 7)
	}
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	var plain bytes.Buffer
	if err := jpeg.Encode(&plain, img, nil); err != nil {
		t.Fatal(err)
	}
	le := binary.LittleEndian
	tiff := []byte("II*\x00\x08\x00\x00\x00")
	entry := func(b []byte, tag, typ uint16, count, value uint32) []byte {
		b = le.AppendUint16(b, tag)
		b = le.AppendUint16(b, typ)
		b = le.AppendUint32(b, count)
		return le.AppendUint32(b, value)
	}
	// IFD0 at 8: the pointers to the EXIF and GPS IFDs.
	tiff = le.AppendUint16(tiff, 2)
	tiff = entry(tiff, 0x8769, 4, 1, 38)
	tiff = entry(tiff, 0x8825, 4, 1, 76)
	tiff = le.AppendUint32(tiff, 0)
	// EXIF IFD at 38: DateTimeOriginal, its text at 56.
	tiff = le.AppendUint16(tiff, 1)
	tiff = entry(tiff, 0x9003, 2, 20, 56)
	tiff = le.AppendUint32(tiff, 0)
	tiff = append(tiff, []byte(taken+"\x00")...)
	// GPS IFD at 76: references inline, degrees as rationals at 130 and 154.
	ref := func(s string) uint32 { return uint32(s[0]) }
	ns, ew := "N", "E"
	if lat < 0 {
		ns, lat = "S", -lat
	}
	if lon < 0 {
		ew, lon = "W", -lon
	}
	n := uint16(4)
	if lat == 0 && lon == 0 {
		n = 0
	}
	tiff = le.AppendUint16(tiff, n)
	if n > 0 {
		tiff = entry(tiff, 1, 2, 2, ref(ns))
		tiff = entry(tiff, 2, 5, 3, 130)
		tiff = entry(tiff, 3, 2, 2, ref(ew))
		tiff = entry(tiff, 4, 5, 3, 154)
	}
	tiff = le.AppendUint32(tiff, 0)
	degrees := func(b []byte, v float64) []byte {
		b = le.AppendUint32(b, uint32(v*1e6))
		b = le.AppendUint32(b, 1e6)
		for range 2 {
			b = le.AppendUint32(b, 0)
			b = le.AppendUint32(b, 1)
		}
		return b
	}
	if n > 0 {
		tiff = degrees(tiff, lat)
		tiff = degrees(tiff, lon)
	}
	app1 := append([]byte("Exif\x00\x00"), tiff...)
	seg := []byte{0xff, 0xe1, byte((len(app1) + 2) >> 8), byte(len(app1) + 2)}
	out := append([]byte{0xff, 0xd8}, seg...)
	out = append(out, app1...)
	return append(out, plain.Bytes()[2:]...)
}

// photoWorld is a world with a folder of photos bert made.
type photoWorld struct {
	world
	folder, folderKey string
}

func newPhotoWorld(t *testing.T, opts ...func(*Deps)) photoWorld {
	w := newWorld(t, opts...)
	f := w.bert.expect("POST", "/api/album/folders", `{"space":"PLAT","name":"Holidays"}`, http.StatusCreated)
	return photoWorld{world: w, folder: field(f, "item", "id").(string), folderKey: field(f, "item", "key").(string)}
}

func (w photoWorld) put(t *testing.T, name string, content []byte) map[string]any {
	t.Helper()
	r := w.bert.upload("/api/items/"+w.folder+"/attachments?name="+name, "application/octet-stream", content)
	if r.Code != http.StatusCreated {
		t.Fatalf("upload %s: %d %s", name, r.Code, r.Body)
	}
	var out map[string]any
	_ = json.Unmarshal(r.Body.Bytes(), &out)
	return out["attachment"].(map[string]any)
}

func names(c *client, query string) []string {
	var out []string
	for _, p := range field(c.expect("GET", "/api/album"+query, "", http.StatusOK), "pictures").([]any) {
		out = append(out, p.(map[string]any)["attachment"].(map[string]any)["name"].(string))
	}
	return out
}

// TestAPhotoTellsWhenAndWhereItWasTaken: an upload records the photo's
// capture time and place; the album sorts by them and narrows to a day, a
// span of days, a day in every year and pictures with a place; months
// and places are counted and listed for readers only (ADR-0177).
func TestAPhotoTellsWhenAndWhereItWasTaken(t *testing.T) {
	w := newPhotoWorld(t)
	lake := w.put(t, "lake.jpg", exifJPEG(t, "2024:05:01 13:22:10", 46.5584, 7.8352, 64))
	w.put(t, "old.jpg", exifJPEG(t, "2019:05:01 08:00:00", 0, 0, 66))
	w.put(t, "town.jpg", exifJPEG(t, "2023:12:24 18:00:00", -33.8688, 151.2093, 68))
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 100)...)
	w.put(t, "drawn.png", png) // tells nothing: sorted by the day it was added

	photo := lake["photo"].(map[string]any)
	if photo["taken"] != "2024-05-01T13:22:10" || photo["lat"].(float64) < 46.55 || photo["lon"].(float64) > 7.84 || photo["width"].(float64) != 64 {
		t.Fatalf("photo data %v", photo)
	}
	if got := strings.Join(names(w.vera, "?order=taken"), " "); got != "drawn.png lake.jpg town.jpg old.jpg" {
		t.Errorf("by the time taken: %s", got)
	}
	if got := strings.Join(names(w.vera, "?order=taken&day=05-01"), " "); got != "lake.jpg old.jpg" {
		t.Errorf("on the first of May: %s", got)
	}
	if got := strings.Join(names(w.vera, "?order=taken&day=05-01&to=2023-12-31"), " "); got != "old.jpg" {
		t.Errorf("on the first of May in earlier years: %s", got)
	}
	if got := strings.Join(names(w.vera, "?from=2023-01-01&to=2024-12-31&order=taken"), " "); got != "lake.jpg town.jpg" {
		t.Errorf("2023 and 2024: %s", got)
	}
	if got := strings.Join(names(w.vera, "?place=on&order=taken"), " "); got != "lake.jpg town.jpg" {
		t.Errorf("with a place: %s", got)
	}
	page := w.vera.expect("GET", "/api/album?order=taken&day=05-01", "", http.StatusOK)
	first := page["pictures"].([]any)[0].(map[string]any)
	if taken := first["taken"].(map[string]any); taken["day"] != "2024-05-01" || taken["exact"] != true || first["first"] != lake["id"] {
		t.Errorf("taken %v, first %v", taken, first["first"])
	}
	months := field(w.vera.expect("GET", "/api/album/months", "", http.StatusOK), "months").([]any)
	if len(months) != 4 || months[1].(map[string]any)["month"] != "2024-05" || months[3].(map[string]any)["month"] != "2019-05" {
		t.Errorf("months %v", months)
	}
	places := w.vera.expect("GET", "/api/album/places", "", http.StatusOK)
	if len(places["places"].([]any)) != 2 || places["tiles"] != true || places["truncated"] != false {
		t.Errorf("places %v", places)
	}
	if got := field(w.carla.expect("GET", "/api/album/places", "", http.StatusOK), "places").([]any); len(got) != 0 {
		t.Errorf("an outsider sees places %v", got)
	}
	// What a picture tells is answered, recorded or read now.
	if r := w.vera.expect("GET", "/api/items/"+w.folder+"/attachments/"+lake["id"].(string)+"/photo", "", http.StatusOK); r["recorded"] != true {
		t.Errorf("photo route %v", r)
	}
	w.carla.expect("GET", "/api/items/"+w.folder+"/attachments/"+lake["id"].(string)+"/photo", "", http.StatusNotFound)
	for _, bad := range []string{"?day=13-01", "?from=2024", "?order=random", "?kind=video"} {
		w.vera.expect("GET", "/api/album"+bad, "", http.StatusBadRequest)
		w.vera.expect("GET", "/api/album/months"+bad, "", http.StatusBadRequest)
		w.vera.expect("GET", "/api/album/places"+bad, "", http.StatusBadRequest)
	}
	// The words of the album find a picture by its camera, too.
	if got := names(w.vera, "?q=lake"); len(got) != 1 {
		t.Errorf("search: %v", got)
	}
}

// TestThumbnailsAreMadeWithTheFilesAccess: a thumbnail is a JPEG no
// larger than its edge, for readers of the file only, answered again from
// memory and by its tag; a format that cannot be scaled leads to the file
// itself (ADR-0177).
func TestThumbnailsAreMadeWithTheFilesAccess(t *testing.T) {
	w := newPhotoWorld(t)
	a := w.put(t, "wide.jpg", exifJPEG(t, "2024:05:01 13:22:10", 0, 0, 900))
	path := "/api/items/" + w.folder + "/attachments/" + a["id"].(string) + "/thumbnail"
	r := w.vera.do("GET", path, "")
	if r.Code != http.StatusOK || r.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("thumbnail: %d %s", r.Code, r.Header())
	}
	img, err := jpeg.Decode(r.Body)
	if err != nil || img.Bounds().Dx() != 400 || img.Bounds().Dy() != 200 {
		t.Fatalf("thumbnail %v, %v", img.Bounds(), err)
	}
	if r := w.vera.do("GET", path+"?edge=1600", ""); r.Code != http.StatusOK {
		t.Errorf("a large thumbnail: %d", r.Code)
	}
	etag := r.Header().Get("ETag")
	again := httptest.NewRequest("GET", path, nil)
	again.AddCookie(w.vera.cookie)
	again.Header.Set("If-None-Match", etag)
	rec := httptest.NewRecorder()
	w.vera.h.ServeHTTP(rec, again)
	if rec.Code != http.StatusNotModified {
		t.Errorf("by its tag: %d", rec.Code)
	}
	if r := w.vera.do("GET", path, ""); r.Code != http.StatusOK || r.Body.Len() == 0 {
		t.Errorf("from memory: %d", r.Code)
	}
	w.vera.expect("GET", path+"?edge=12", "", http.StatusBadRequest)
	w.carla.expect("GET", path, "", http.StatusNotFound)
	webp := append([]byte("RIFF\x24\x00\x00\x00WEBPVP8 "), bytes.Repeat([]byte{0}, 40)...)
	b := w.put(t, "pic.webp", webp)
	if r := w.vera.do("GET", "/api/items/"+w.folder+"/attachments/"+b["id"].(string)+"/thumbnail", ""); r.Code != http.StatusFound {
		t.Errorf("a WebP picture: %d", r.Code)
	}
	txt := w.put(t, "notes.txt", []byte("plain text"))
	w.vera.expect("GET", "/api/items/"+w.folder+"/attachments/"+txt["id"].(string)+"/thumbnail", "", http.StatusNotFound)
}

// TestFavouritesArePersonal: a favourite is one person's, stays through
// a new version of its file, narrows the album to it, and is set only on
// a picture one may read (ADR-0177).
func TestFavouritesArePersonal(t *testing.T) {
	w := newPhotoWorld(t)
	a := w.put(t, "lake.jpg", exifJPEG(t, "2024:05:01 13:22:10", 0, 0, 64))
	w.put(t, "hill.jpg", exifJPEG(t, "2024:05:02 13:22:10", 0, 0, 66))
	id := a["id"].(string)
	r := w.vera.expect("PUT", "/api/album/favourites", `{"item":"`+w.folder+`","file":"`+id+`","favourite":true}`, http.StatusOK)
	if r["first"] != id {
		t.Errorf("favourite %v", r)
	}
	if got := names(w.vera, "?favourites=on"); len(got) != 1 || got[0] != "lake.jpg" {
		t.Errorf("vera's favourites %v", got)
	}
	if got := names(w.bert, "?favourites=on"); len(got) != 0 {
		t.Errorf("bert has vera's favourites: %v", got)
	}
	// A new version of the file keeps the mark.
	r2 := w.bert.upload("/api/items/"+w.folder+"/attachments?name=lake.jpg&sourceItem="+w.folder+"&sourceAttachment="+id, "application/octet-stream", exifJPEG(t, "2024:05:01 13:22:11", 0, 0, 70))
	if r2.Code != http.StatusCreated {
		t.Fatalf("new version: %d %s", r2.Code, r2.Body)
	}
	page := field(w.vera.expect("GET", "/api/album?favourites=on", "", http.StatusOK), "pictures").([]any)
	if len(page) != 1 || page[0].(map[string]any)["favourite"] != true || len(page[0].(map[string]any)["versions"].([]any)) != 1 {
		t.Errorf("after a new version: %v", page)
	}
	w.vera.expect("PUT", "/api/album/favourites", `{"item":"`+w.folder+`","file":"`+id+`","favourite":false}`, http.StatusOK)
	if got := names(w.vera, "?favourites=on"); len(got) != 0 {
		t.Errorf("unmarked: %v", got)
	}
	w.carla.expect("PUT", "/api/album/favourites", `{"item":"`+w.folder+`","file":"`+id+`","favourite":true}`, http.StatusNotFound)
	w.vera.expect("PUT", "/api/album/favourites", `{"item":"`+w.folder+`","file":"nope","favourite":true}`, http.StatusNotFound)
}

// TestChosenPicturesDownloadAsOneArchive: the archive holds each chosen
// picture under a name of its own, read with the caller's rights, within
// the budgets (ADR-0177).
func TestChosenPicturesDownloadAsOneArchive(t *testing.T) {
	w := newPhotoWorld(t)
	one := exifJPEG(t, "2024:05:01 13:22:10", 0, 0, 64)
	a := w.put(t, "lake.jpg", one)
	b := w.put(t, "lake.jpg", exifJPEG(t, "2024:05:01 13:22:10", 0, 0, 66))
	body := `{"pictures":[{"item":"` + w.folder + `","file":"` + a["id"].(string) + `"},{"item":"` + w.folder + `","file":"` + b["id"].(string) + `"}]}`
	r := w.vera.do("POST", "/api/album/zip", body)
	if r.Code != http.StatusOK || r.Header().Get("Content-Type") != "application/zip" || r.Header().Get("Content-Disposition") != `attachment; filename="album.zip"` {
		t.Fatalf("zip: %d %s", r.Code, r.Body)
	}
	zr, err := zip.NewReader(bytes.NewReader(r.Body.Bytes()), int64(r.Body.Len()))
	if err != nil || len(zr.File) != 2 || zr.File[0].Name != "lake.jpg" || zr.File[1].Name != "lake (2).jpg" {
		t.Fatalf("archive %v %v", zr, err)
	}
	f, _ := zr.File[0].Open()
	got, _ := io.ReadAll(f)
	if !bytes.Equal(got, one) {
		t.Error("the archive does not hold the picture as stored")
	}
	w.carla.expect("POST", "/api/album/zip", body, http.StatusNotFound)
	w.vera.expect("POST", "/api/album/zip", `{"pictures":[]}`, http.StatusBadRequest)
	bud := budget.Get()
	bud.AlbumZipBytes = 10
	budget.Set(bud)
	t.Cleanup(func() { budget.Set(budget.Default()) })
	w.vera.expect("POST", "/api/album/zip", body, http.StatusRequestEntityTooLarge)
}

func TestUniqueNamesInAnArchive(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range []struct{ in, want string }{{"a.jpg", "a.jpg"}, {"a.jpg", "a (2).jpg"}, {"a.jpg", "a (3).jpg"}, {"noext", "noext"}, {"noext", "noext (2)"}} {
		if got := uniqueName(seen, c.in); got != c.want {
			t.Errorf("%s: %s, want %s", c.in, got, c.want)
		}
	}
}

// TestMapTilesComeThroughTheServer: a tile is fetched from the named tile
// service once and kept; without one, or for a tile that does not exist,
// nothing is fetched (ADR-0177).
func TestMapTilesComeThroughTheServer(t *testing.T) {
	fetched := 0
	tilesrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetched++
		if r.URL.Path == "/broken/1/0/0.png" {
			http.Error(w, "no", http.StatusInternalServerError)
			return
		}
		if !strings.HasPrefix(r.Header.Get("User-Agent"), "Orbithek/") {
			t.Errorf("user agent %q", r.Header.Get("User-Agent"))
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("\x89PNG tile"))
	}))
	t.Cleanup(tilesrv.Close)
	photos := NewPhotos(func(k string) string {
		return map[string]string{MapTilesVar: tilesrv.URL + "/{z}/{x}/{y}.png"}[k]
	}, nil)
	photos.tileClient = tilesrv.Client()
	w := newWorld(t, func(d *Deps) { d.Photos = photos })
	for range 2 {
		if r := w.vera.do("GET", "/api/map/tiles/3/2/1", ""); r.Code != http.StatusOK || r.Header().Get("Content-Type") != "image/png" {
			t.Fatalf("tile: %d %s", r.Code, r.Body)
		}
	}
	if fetched != 1 {
		t.Errorf("fetched %d times", fetched)
	}
	for _, bad := range []string{"3/8/0", "20/0/0", "x/0/0", "-1/0/0"} {
		w.vera.expect("GET", "/api/map/tiles/"+bad, "", http.StatusBadRequest)
	}
	photos.TileURL = tilesrv.URL + "/broken/{z}/{x}/{y}.png"
	w.vera.expect("GET", "/api/map/tiles/1/0/0", "", http.StatusBadGateway)
	photos.TileURL = ""
	w.vera.expect("GET", "/api/map/tiles/1/0/0", "", http.StatusNotFound)
	if validTileURL("http://tiles/{z}/{x}/{y}") || validTileURL("https://tiles/{z}/{x}") || !validTileURL("https://t/{z}/{x}/{y}.png") {
		t.Error("tile addresses")
	}
}

// anyHost is a client that reaches a TLS test server whatever host an
// address names, so the services' own addresses are used unchanged.
func anyHost(t *testing.T, h http.Handler) *http.Client {
	t.Helper()
	srv := httptest.NewTLSServer(h)
	t.Cleanup(srv.Close)
	base := srv.Client().Transport.(*http.Transport).Clone()
	base.TLSClientConfig.ServerName = "example.com"
	addr := srv.Listener.Addr().String()
	base.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, network, addr)
	}
	return &http.Client{Transport: base}
}

// TestImportsAreOffTillTheOperatorTurnsThemOn: without the variables the
// album offers no import and refuses every import route.
func TestImportsAreOffTillTheOperatorTurnsThemOn(t *testing.T) {
	w := newPhotoWorld(t, func(d *Deps) { d.Photos = NewPhotos(func(string) string { return "" }, nil) })
	r := w.vera.expect("GET", "/api/album/imports", "", http.StatusOK)
	// The map has OpenStreetMap's tiles unless the operator names others
	// or none (ADR-0179).
	if r["google"] != false || r["icloud"] != false || r["tiles"] != true {
		t.Errorf("imports %v", r)
	}
	w.bert.expect("GET", "/api/album/google/connect", "", http.StatusNotFound)
	w.bert.expect("POST", "/api/album/google/sessions", "", http.StatusNotFound)
	w.bert.expect("GET", "/api/album/google/sessions/x", "", http.StatusNotFound)
	w.bert.expect("POST", "/api/album/google/sessions/x/import", `{"media":"m","folder":"`+w.folder+`"}`, http.StatusNotFound)
	w.bert.expect("DELETE", "/api/album/google/sessions/x", "", http.StatusNotFound)
	w.bert.expect("POST", "/api/album/icloud/albums", `{"url":"x"}`, http.StatusNotFound)
	w.bert.expect("POST", "/api/album/icloud/import", `{"url":"x"}`, http.StatusNotFound)
	if r := w.bert.do("GET", GooglePhotosCallback+"?state=s&code=c", ""); r.Code != http.StatusSeeOther || !strings.Contains(r.Header().Get("Location"), "photos=google-error") {
		t.Errorf("callback without Google: %d %s", r.Code, r.Header())
	}
}

// fakeICloud answers as Apple's shared album interface does for one album
// of two photos and a video.
func fakeICloud(t *testing.T, photo []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/sharedstreams/webstream"):
			w.Write([]byte(`{"streamName":"Summer","userFirstName":"Anna","userLastName":"Muster","photos":[
				{"photoGuid":"G1","caption":"Lake","dateCreated":"2024-05-01T11:22:10Z","width":"64","height":"32","derivatives":{"1":{"checksum":"c1","fileSize":"100","width":"64","height":"32"}}},
				{"photoGuid":"G2","caption":"","dateCreated":"2024-05-02T11:22:10Z","width":"64","height":"32","derivatives":{"1":{"checksum":"c2","fileSize":"100","width":"64","height":"32"}}},
				{"photoGuid":"G3","mediaAssetType":"video","dateCreated":"2024-05-03T11:22:10Z","derivatives":{"1":{"checksum":"c3","fileSize":"100","width":"64","height":"32"}}}]}`))
		case strings.HasSuffix(r.URL.Path, "/sharedstreams/webasseturls"):
			var req struct {
				PhotoGuids []string `json:"photoGuids"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			items := map[string]any{}
			for _, g := range req.PhotoGuids {
				items["c"+g[1:]] = map[string]string{"url_location": "cvws.icloud-content.com", "url_path": "/asset/" + g}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
		case strings.HasPrefix(r.URL.Path, "/asset/G1"):
			w.Write(photo)
		case strings.HasPrefix(r.URL.Path, "/asset/G2"):
			w.Write([]byte("not a picture at all"))
		default:
			http.NotFound(w, r)
		}
	})
}

// TestASharedICloudAlbumComesIntoAFolder: the album is read by its
// address; each photo comes in as the caller's upload with the time Apple
// tells, again only once, and what is no picture is refused (ADR-0177).
func TestASharedICloudAlbumComesIntoAFolder(t *testing.T) {
	// The photo without EXIF: its time comes from Apple.
	var raw bytes.Buffer
	_ = jpeg.Encode(&raw, image.NewRGBA(image.Rect(0, 0, 20, 10)), nil)
	photos := NewPhotos(func(k string) string { return map[string]string{PhotosICloudVar: "on"}[k] }, nil)
	photos.ICloud = photosource.NewICloud(anyHost(t, fakeICloud(t, raw.Bytes())))
	w := newPhotoWorld(t, func(d *Deps) { d.Photos = photos })
	if r := w.vera.expect("GET", "/api/album/imports", "", http.StatusOK); r["icloud"] != true {
		t.Errorf("imports %v", r)
	}
	address := "https://www.icloud.com/sharedalbum/#B0aGWZuqDGKS7Ke"
	album := w.bert.expect("POST", "/api/album/icloud/albums", `{"url":"`+address+`"}`, http.StatusOK)
	if album["name"] != "Summer" || len(album["photos"].([]any)) != 3 {
		t.Fatalf("album %v", album)
	}
	w.bert.expect("POST", "/api/album/icloud/albums", `{"url":"https://example.org/x"}`, http.StatusBadRequest)
	imp := func(c *client, guid string, status int) map[string]any {
		return c.expect("POST", "/api/album/icloud/import", `{"url":"`+address+`","photo":"`+guid+`","folder":"`+w.folderKey+`"}`, status)
	}
	got := imp(w.bert, "G1", http.StatusCreated)
	a := got["attachment"].(map[string]any)
	if got["skipped"] != false || a["name"] != "Lake.jpg" || a["photo"].(map[string]any)["taken"] != "2024-05-01T11:22:10" {
		t.Errorf("imported %v", got)
	}
	if again := imp(w.bert, "G1", http.StatusCreated); again["skipped"] != true {
		t.Errorf("again %v", again)
	}
	imp(w.bert, "G2", http.StatusUnsupportedMediaType)
	imp(w.bert, "G3", http.StatusUnsupportedMediaType)
	imp(w.bert, "G9", http.StatusNotFound)
	imp(w.vera, "G1", http.StatusForbidden)
	if got := names(w.vera, "?folder="+w.folder); len(got) != 1 {
		t.Errorf("the folder holds %v", got)
	}
}

// fakeGoogle answers as Google's sign-in and Photos Picker do for one
// chosen photo and one video.
func fakeGoogle(t *testing.T, photo []byte) http.Handler {
	polled := 0
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Host == "oauth2.googleapis.com" || r.Host == "oauth2.googleapis.com" || r.URL.Path == "/token":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"access_token":"tok","token_type":"Bearer","expires_in":3600}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/sessions":
			w.Write([]byte(`{"id":"S1","pickerUri":"https://photos.google.com/picker/S1","pollingConfig":{"pollInterval":"1s","timeoutIn":"600s"},"mediaItemsSet":false}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/sessions/S1":
			polled++
			w.Write([]byte(`{"id":"S1","pickerUri":"https://photos.google.com/picker/S1","pollingConfig":{"pollInterval":"1s","timeoutIn":"600s"},"mediaItemsSet":` + map[bool]string{true: "true", false: "false"}[polled > 1] + `}`))
		case r.Method == http.MethodDelete:
			w.Write([]byte(`{}`))
		case r.URL.Path == "/v1/mediaItems":
			w.Write([]byte(`{"mediaItems":[
				{"id":"M1","createTime":"2024-05-01T11:22:10Z","type":"PHOTO","mediaFile":{"baseUrl":"https://lh3.googleusercontent.com/M1","mimeType":"image/jpeg","filename":"IMG_1.jpg","mediaFileMetadata":{"width":64,"height":32}}},
				{"id":"M2","createTime":"2024-05-02T11:22:10Z","type":"VIDEO","mediaFile":{"baseUrl":"https://lh3.googleusercontent.com/M2","mimeType":"video/mp4","filename":"VID.mp4"}}]}`))
		case strings.HasPrefix(r.URL.Path, "/M1"):
			if r.Header.Get("Authorization") != "Bearer tok" {
				http.Error(w, "no", http.StatusUnauthorized)
				return
			}
			w.Write(photo)
		default:
			http.NotFound(w, r)
		}
	})
}

// TestPhotosChosenInGooglePhotosComeIntoAFolder: one signs in at Google,
// bound to the browser that began it, opens a picker, waits until the
// photos are chosen and brings each into the folder; videos are refused
// and the picker is closed (ADR-0177).
func TestPhotosChosenInGooglePhotosComeIntoAFolder(t *testing.T) {
	photo := exifJPEG(t, "2024:05:01 13:22:10", 46.5, 7.8, 64)
	client := anyHost(t, fakeGoogle(t, photo))
	g := photosource.NewGoogle("client", "secret", "https://wiki.example.org"+GooglePhotosCallback, client)
	photos := NewPhotos(func(k string) string { return map[string]string{PhotosGoogleVar: "on"}[k] }, g)
	w := newPhotoWorld(t, func(d *Deps) { d.Photos = photos })
	if r := w.bert.expect("GET", "/api/album/imports", "", http.StatusOK); r["google"] != true || r["connected"] != false {
		t.Errorf("imports %v", r)
	}
	r := w.bert.do("GET", "/api/album/google/connect?folder="+w.folderKey, "")
	if r.Code != http.StatusSeeOther {
		t.Fatalf("connect: %d %s", r.Code, r.Body)
	}
	to, _ := url.Parse(r.Header().Get("Location"))
	state := to.Query().Get("state")
	if state == "" || !strings.Contains(to.Query().Get("scope"), "photospicker.mediaitems.readonly") {
		t.Fatalf("to Google: %s", to)
	}
	var cookies []*http.Cookie
	for _, c := range r.Result().Cookies() {
		cookies = append(cookies, c)
	}
	back := func(query string, with []*http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", GooglePhotosCallback+"?"+query, nil)
		for _, c := range with {
			req.AddCookie(c)
		}
		rec := httptest.NewRecorder()
		w.bert.h.ServeHTTP(rec, req)
		return rec
	}
	// Another browser, without the cookie, is refused.
	if rec := back("state="+state+"&code=c", nil); !strings.Contains(rec.Header().Get("Location"), "photos=google-error") {
		t.Errorf("without the cookie: %s", rec.Header())
	}
	rec := back("state="+state+"&code=c", cookies)
	if loc := rec.Header().Get("Location"); loc != "/#/album?folder="+w.folderKey+"&photos=google" {
		t.Fatalf("back: %d %s", rec.Code, loc)
	}
	if r := w.bert.expect("GET", "/api/album/imports", "", http.StatusOK); r["connected"] != true {
		t.Errorf("not connected: %v", r)
	}
	s := w.bert.expect("POST", "/api/album/google/sessions", "", http.StatusCreated)
	if s["id"] != "S1" || s["pickerUri"] != "https://photos.google.com/picker/S1" {
		t.Fatalf("session %v", s)
	}
	if p := w.bert.expect("GET", "/api/album/google/sessions/S1", "", http.StatusOK); p["picked"] != false {
		t.Errorf("first poll %v", p)
	}
	p := w.bert.expect("GET", "/api/album/google/sessions/S1", "", http.StatusOK)
	if p["picked"] != true || len(p["items"].([]any)) != 2 {
		t.Fatalf("picked %v", p)
	}
	w.vera.expect("GET", "/api/album/google/sessions/S1", "", http.StatusNotFound) // another's picker is no picker
	got := w.bert.expect("POST", "/api/album/google/sessions/S1/import", `{"media":"M1","folder":"`+w.folder+`","strip":true}`, http.StatusCreated)
	if got["attachment"].(map[string]any)["photo"].(map[string]any)["lat"] != nil {
		t.Errorf("the place stayed: %v", got)
	}
	if a := got["attachment"].(map[string]any); a["name"] != "IMG_1.jpg" || a["photo"].(map[string]any)["taken"] != "2024-05-01T13:22:10" {
		t.Errorf("imported %v", got)
	}
	w.bert.expect("POST", "/api/album/google/sessions/S1/import", `{"media":"M2","folder":"`+w.folder+`"}`, http.StatusUnsupportedMediaType)
	w.vera.expect("POST", "/api/album/google/sessions/S1/import", `{"media":"M1","folder":"`+w.folder+`"}`, http.StatusForbidden)
	w.bert.expect("DELETE", "/api/album/google/sessions/S1", "", http.StatusNoContent)
	// A refusal at Google comes back as one.
	r = w.bert.do("GET", "/api/album/google/connect", "")
	to, _ = url.Parse(r.Header().Get("Location"))
	if rec := back("state="+to.Query().Get("state")+"&error=access_denied", r.Result().Cookies()); !strings.Contains(rec.Header().Get("Location"), "photos=google-denied") {
		t.Errorf("refused: %s", rec.Header())
	}
}

func TestImportNames(t *testing.T) {
	for _, c := range []struct{ in, kind, want string }{
		{"IMG_1.jpg", "image/jpeg", "IMG_1.jpg"}, {"a/b", "image/png", "a_b.png"}, {"", "image/gif", "photo.gif"}, {"Lake", "image/webp", "Lake.webp"},
		{strings.Repeat("x", 300), "image/jpeg", strings.Repeat("x", 200) + ".jpg"},
	} {
		if got := importName(c.in, c.kind); got != c.want {
			t.Errorf("%q: %q, want %q", c.in, got, c.want)
		}
	}
}

func TestByteCacheKeepsTheNewest(t *testing.T) {
	c := newByteCache(40)
	c.put("a", "x", make([]byte, 10))
	c.put("b", "x", make([]byte, 10))
	c.get("a")
	c.put("c", "x", make([]byte, 10))
	c.put("d", "x", make([]byte, 10))
	c.put("e", "x", make([]byte, 20)) // more than a quarter: not kept
	if _, _, ok := c.get("e"); ok {
		t.Error("a large answer was kept")
	}
	c.put("a", "x", make([]byte, 10)) // replaced
	c.put("f", "x", make([]byte, 10))
	if _, _, ok := c.get("b"); ok {
		t.Error("the least recently used was kept")
	}
	if _, _, ok := c.get("f"); !ok {
		t.Error("the newest was dropped")
	}
}

// TestThePlaceIsLeftBehindOnRequest: with place=strip an upload keeps
// neither the place in its photo data nor in the stored file, and keeps
// what is no place; without it the place stays; what is no picture is
// stored as it is (ADR-0177).
func TestThePlaceIsLeftBehindOnRequest(t *testing.T) {
	w := newPhotoWorld(t)
	lake := exifJPEG(t, "2024:05:01 13:22:10", 46.5584, 7.8352, 64)
	r := w.bert.upload("/api/items/"+w.folder+"/attachments?name=lake.jpg&place=strip", "application/octet-stream", lake)
	if r.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", r.Code, r.Body)
	}
	var out map[string]any
	_ = json.Unmarshal(r.Body.Bytes(), &out)
	a := out["attachment"].(map[string]any)
	p := a["photo"].(map[string]any)
	if p["lat"] != nil || p["lon"] != nil || p["taken"] != "2024-05-01T13:22:10" {
		t.Errorf("photo data %v", p)
	}
	stored := w.bert.do("GET", "/api/items/"+w.folder+"/attachments/"+a["id"].(string), "")
	if m := photo.Read(bytes.NewReader(stored.Body.Bytes()), "image/jpeg"); m.Lat != nil || m.Taken == "" {
		t.Errorf("the stored file still tells %+v", m)
	}
	if _, err := jpeg.Decode(bytes.NewReader(stored.Body.Bytes())); err != nil {
		t.Errorf("the stored picture no longer decodes: %v", err)
	}
	if got := w.put(t, "kept.jpg", exifJPEG(t, "2024:05:01 13:22:10", 46.5584, 7.8352, 66)); got["photo"].(map[string]any)["lat"] == nil {
		t.Error("without the option the place went")
	}
	if r := w.bert.upload("/api/items/"+w.folder+"/attachments?name=notes.txt&place=strip", "application/octet-stream", []byte("plain text")); r.Code != http.StatusCreated {
		t.Errorf("a text file: %d", r.Code)
	}
	bud := budget.Get()
	bud.AttachmentBytes = 50
	budget.Set(bud)
	t.Cleanup(func() { budget.Set(budget.Default()) })
	if r := w.bert.upload("/api/items/"+w.folder+"/attachments?name=big.jpg&place=strip", "application/octet-stream", lake); r.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("too large: %d", r.Code)
	}
}

func TestStoreFailures(t *testing.T) {
	w := newPhotoWorld(t)
	h := itemHandlers{d: w.deps, log: slog.New(slog.DiscardHandler)}
	for err, status := range map[error]int{errPlaceStays: 422, photosource.ErrTooLarge: 413, photosource.ErrUnreadable: 502, io.ErrUnexpectedEOF: 500} {
		rec := httptest.NewRecorder()
		h.storeFail(rec, err, 10)
		if rec.Code != status {
			t.Errorf("%v: %d, want %d", err, rec.Code, status)
		}
	}
}

// TestTheMapHasTilesUnlessTurnedOff: without a tile service named the map
// draws OpenStreetMap's, with its credit; "off" draws none; another
// service has the credit the operator gives (ADR-0179).
func TestTheMapHasTilesUnlessTurnedOff(t *testing.T) {
	env := func(vars map[string]string) func(string) string { return func(k string) string { return vars[k] } }
	p := NewPhotos(env(nil), nil)
	if p.TileURL != DefaultMapTiles || p.TileCredit != DefaultMapAttribution || !strings.HasPrefix(p.tileAgent, "Orbithek/") {
		t.Errorf("default: %q %q %q", p.TileURL, p.TileCredit, p.tileAgent)
	}
	if p := NewPhotos(env(map[string]string{MapTilesVar: "off"}), nil); p.TileURL != "" || p.TileCredit != "" {
		t.Errorf("off: %q %q", p.TileURL, p.TileCredit)
	}
	p = NewPhotos(env(map[string]string{MapTilesVar: "https://tiles.example/{z}/{x}/{y}.png", MapAttributionVar: "© Example", "OT_PUBLIC_URL": "https://wiki.example.org"}), nil)
	if p.TileURL != "https://tiles.example/{z}/{x}/{y}.png" || p.TileCredit != "© Example" || !strings.Contains(p.tileAgent, "+https://wiki.example.org") {
		t.Errorf("own service: %q %q %q", p.TileURL, p.TileCredit, p.tileAgent)
	}
	if p := NewPhotos(env(map[string]string{MapTilesVar: "https://tiles.example/{z}/{x}/{y}.png"}), nil); p.TileCredit != "" {
		t.Errorf("another service took OpenStreetMap's credit: %q", p.TileCredit)
	}
	w := newPhotoWorld(t, func(d *Deps) { d.Photos = NewPhotos(env(nil), nil) })
	if r := w.vera.expect("GET", "/api/album/places", "", http.StatusOK); r["tiles"] != true || r["credit"] != DefaultMapAttribution {
		t.Errorf("places %v", r)
	}
}

// TestAPlaceKeptToItsUploaderLeavesTheFile: with place=private the one
// who adds a photo keeps its place; every other reader reads the photo
// data and downloads the file, alone or in an archive, without it, and may
// copy it only so; the uploader shows it again, nobody else may (ADR-0178).
func TestAPlaceKeptToItsUploaderLeavesTheFile(t *testing.T) {
	w := newPhotoWorld(t)
	lake := exifJPEG(t, "2024:05:01 13:22:10", 46.5584, 7.8352, 64)
	r := w.bert.upload("/api/items/"+w.folder+"/attachments?name=lake.jpg&place=private", "application/octet-stream", lake)
	if r.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", r.Code, r.Body)
	}
	var out map[string]any
	_ = json.Unmarshal(r.Body.Bytes(), &out)
	a := out["attachment"].(map[string]any)
	id := a["id"].(string)
	if p := a["photo"].(map[string]any); p["lat"] == nil || p["placePrivate"] != true {
		t.Fatalf("the uploader's photo data %v", p)
	}
	file := "/api/items/" + w.folder + "/attachments/" + id
	seen := field(w.vera.expect("GET", file+"/photo", "", http.StatusOK), "photo").(map[string]any)
	if seen["lat"] != nil || seen["placePrivate"] != nil || seen["taken"] != "2024-05-01T13:22:10" {
		t.Errorf("vera reads %v", seen)
	}
	got := w.vera.do("GET", file, "")
	if m := photo.Read(bytes.NewReader(got.Body.Bytes()), "image/jpeg"); got.Code != http.StatusOK || m.Lat != nil || m.Taken == "" || !strings.HasSuffix(got.Header().Get("ETag"), `-noplace"`) {
		t.Errorf("vera's download: %d %+v %s", got.Code, m, got.Header().Get("ETag"))
	}
	if m := photo.Read(bytes.NewReader(w.bert.do("GET", file, "").Body.Bytes()), "image/jpeg"); m.Lat == nil {
		t.Error("the uploader's download lost the place")
	}
	if places := field(w.vera.expect("GET", "/api/album/places", "", http.StatusOK), "places").([]any); len(places) != 0 {
		t.Errorf("vera's map shows %v", places)
	}
	if places := field(w.bert.expect("GET", "/api/album/places", "", http.StatusOK), "places").([]any); len(places) != 1 {
		t.Errorf("bert's map shows %v", places)
	}
	zr := w.vera.do("POST", "/api/album/zip", `{"pictures":[{"item":"`+w.folder+`","file":"`+id+`"}]}`)
	zf, err := zip.NewReader(bytes.NewReader(zr.Body.Bytes()), int64(zr.Body.Len()))
	if err != nil || len(zf.File) != 1 {
		t.Fatalf("archive: %v", err)
	}
	f, _ := zf.File[0].Open()
	inZip, _ := io.ReadAll(f)
	if m := photo.Read(bytes.NewReader(inZip), "image/jpeg"); m.Lat != nil {
		t.Error("vera's archive tells the place")
	}

	// The administrator copies it: only without the place.
	other := w.admin.expect("POST", "/api/album/folders", `{"space":"PLAT","name":"Mine"}`, http.StatusCreated)
	mine := field(other, "item", "id").(string)
	src := "&sourceItem=" + w.folder + "&sourceAttachment=" + id
	stripped := w.admin.do("GET", file, "").Body.Bytes()
	if r := w.admin.upload("/api/items/"+mine+"/attachments?name=lake.jpg"+src, "application/octet-stream", stripped); r.Code != http.StatusCreated {
		t.Errorf("a copy without the place: %d %s", r.Code, r.Body)
	}
	// Who holds the original already copies it as it is; the copy keeps
	// its place from everyone else, as its original does.
	r = w.admin.upload("/api/items/"+mine+"/attachments?name=lake.jpg"+src, "application/octet-stream", lake)
	_ = json.Unmarshal(r.Body.Bytes(), &out)
	if r.Code != http.StatusCreated || field(out, "attachment", "photo", "placePrivate") != true {
		t.Errorf("a copy of the original: %d %s", r.Code, r.Body)
	}
	if r := w.admin.upload("/api/items/"+mine+"/attachments?name=lake.jpg"+src, "application/octet-stream", append(stripped, 0)); r.Code != http.StatusBadRequest {
		t.Errorf("other content as a copy: %d %s", r.Code, r.Body)
	}

	// Only the uploader decides.
	w.vera.expect("PUT", file+"/place", `{"private":false}`, http.StatusForbidden)
	w.carla.expect("PUT", file+"/place", `{"private":false}`, http.StatusNotFound)
	if r := w.bert.expect("PUT", file+"/place", `{"private":false}`, http.StatusOK); field(r, "attachment", "photo", "placePrivate") != nil {
		t.Errorf("shown again: %v", r)
	}
	if field(w.vera.expect("GET", file+"/photo", "", http.StatusOK), "photo", "lat") == nil {
		t.Error("shown again, vera still reads no place")
	}
	// A picture without a place has none to keep.
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 100)...)
	drawn := w.put(t, "drawn.png", png)
	w.bert.expect("PUT", "/api/items/"+w.folder+"/attachments/"+drawn["id"].(string)+"/place", `{"private":true}`, http.StatusBadRequest)
}

func TestPlaceChoices(t *testing.T) {
	for _, c := range []struct {
		value string
		strip bool
		want  string
	}{{"", false, placeKeep}, {"private", false, placePrivate}, {"strip", false, placeStrip}, {"", true, placeStrip}, {"private", true, placeStrip}, {"other", false, placeKeep}} {
		if got := placeChoice(c.value, c.strip); got != c.want {
			t.Errorf("%q %v: %q, want %q", c.value, c.strip, got, c.want)
		}
	}
	if keepPrivate(nil) != nil || keepPrivate(&item.Photo{Taken: "x"}).PlacePrivate {
		t.Error("keepPrivate without a place")
	}
}

// TestAMovedPictureKeepsItsIdentity: a picture moved to another folder
// keeps its ID, its versions and the favourites marked on it; its content
// is read from the new folder; one who may only read may not move it
// (ADR-0186).
func TestAMovedPictureKeepsItsIdentity(t *testing.T) {
	w := newPhotoWorld(t)
	a := w.put(t, "lake.jpg", exifJPEG(t, "2024:05:01 13:22:10", 0, 0, 64))
	id := a["id"].(string)
	second := exifJPEG(t, "2024:05:01 13:22:11", 0, 0, 70)
	r := w.bert.upload("/api/items/"+w.folder+"/attachments?name=lake.jpg&sourceItem="+w.folder+"&sourceAttachment="+id, "application/octet-stream", second)
	if r.Code != http.StatusCreated {
		t.Fatalf("new version: %d %s", r.Code, r.Body)
	}
	var up map[string]map[string]any
	_ = json.Unmarshal(r.Body.Bytes(), &up)
	v2 := up["attachment"]["id"].(string)
	w.vera.expect("PUT", "/api/album/favourites", `{"item":"`+w.folder+`","file":"`+id+`","favourite":true}`, http.StatusOK)
	f := w.bert.expect("POST", "/api/album/folders", `{"space":"PLAT","name":"Archive"}`, http.StatusCreated)
	archive := field(f, "item", "id").(string)

	move := "/api/items/" + w.folder + "/attachments/" + v2 + "/move"
	w.vera.expect("POST", move, `{"to":"`+archive+`"}`, http.StatusForbidden)
	w.carla.expect("POST", move, `{"to":"`+archive+`"}`, http.StatusNotFound)
	w.bert.expect("POST", move, `{"to":"`+w.folder+`"}`, http.StatusBadRequest)
	got := w.bert.expect("POST", move, `{"to":"`+archive+`"}`, http.StatusOK)
	if field(got, "attachment", "id") != v2 {
		t.Fatalf("moved %v", got)
	}
	if got := names(w.bert, "?folder="+w.folder); len(got) != 0 {
		t.Errorf("left in the former folder: %v", got)
	}
	page := field(w.bert.expect("GET", "/api/album?folder="+archive, "", http.StatusOK), "pictures").([]any)
	if len(page) != 1 || page[0].(map[string]any)["first"] != id || len(page[0].(map[string]any)["versions"].([]any)) != 1 {
		t.Fatalf("the stack in the new folder: %v", page)
	}
	if rec := w.bert.do("GET", "/api/items/"+archive+"/attachments/"+v2, ""); rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), second) {
		t.Errorf("content from the new folder: %d", rec.Code)
	}
	w.bert.expect("GET", "/api/items/"+w.folder+"/attachments/"+v2, "", http.StatusNotFound)
	// The favourite went with the picture, and is unmarked where it is now.
	fav := field(w.vera.expect("GET", "/api/album?favourites=on", "", http.StatusOK), "pictures").([]any)
	if len(fav) != 1 || fav[0].(map[string]any)["favourite"] != true {
		t.Fatalf("favourites after the move: %v", fav)
	}
	w.vera.expect("PUT", "/api/album/favourites", `{"item":"`+archive+`","file":"`+v2+`","favourite":true}`, http.StatusOK)
	w.vera.expect("PUT", "/api/album/favourites", `{"item":"`+archive+`","file":"`+v2+`","favourite":false}`, http.StatusOK)
	if got := names(w.vera, "?favourites=on"); len(got) != 0 {
		t.Errorf("unmarked after the move: %v", got)
	}
}
