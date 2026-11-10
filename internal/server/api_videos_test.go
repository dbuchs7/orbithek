package server

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dbuchs7/orbithek/internal/budget"
)

// quickTime makes a QuickTime video as an iPhone writes it: when and
// where it was taken and how long it plays, then media data of pad bytes.
func quickTime(place string, pad int) []byte {
	box := func(kind string, c ...[]byte) []byte {
		b := bytes.Join(c, nil)
		return append(append(binary.BigEndian.AppendUint32(nil, uint32(8+len(b))), kind...), b...)
	}
	be := func(v uint32) []byte { return binary.BigEndian.AppendUint32(nil, v) }
	mvhd := box("mvhd", be(0), be(3797407330), be(3797407330), be(600), be(6000), make([]byte, 80))
	tkhd := box("tkhd", make([]byte, 40), be(1<<16), be(0), be(0), be(0), be(1<<16), be(0), be(0), be(0), be(1<<30), be(1920<<16), be(1080<<16))
	xyz := box("\xa9xyz", binary.BigEndian.AppendUint16(nil, uint16(len(place))), []byte{0x15, 0xc7}, []byte(place))
	return bytes.Join([][]byte{
		box("ftyp", []byte("qt  "), be(0), []byte("qt  ")),
		box("moov", mvhd, box("trak", tkhd), box("udta", xyz)),
		box("mdat", bytes.Repeat([]byte{9}, pad)),
	}, nil)
}

func (c *client) putBytes(path string, content []byte) *httptest.ResponseRecorder {
	c.t.Helper()
	req := httptest.NewRequest(http.MethodPut, path, bytes.NewReader(content))
	req.RemoteAddr = "192.0.2.1:4000"
	req.Header.Set("Content-Type", "application/octet-stream")
	if c.cookie != nil {
		req.AddCookie(c.cookie)
	}
	rec := httptest.NewRecorder()
	c.h.ServeHTTP(rec, req)
	return rec
}

// TestVideosInTheAlbum: a video is taken into a folder beyond the budget
// of other files, told by its own bytes, read for when and where it was
// taken and how long it plays, shown in the album with its poster, and
// given without a place its uploader keeps (ADR-0189).
func TestVideosInTheAlbum(t *testing.T) {
	w := newPhotoWorld(t)
	b := budget.Get()
	b.AttachmentBytes = 4000
	b.VideoBytes = 1 << 20
	budget.Set(b)
	t.Cleanup(func() { budget.Set(budget.Default()) })

	upload := func(name, query string, content []byte) *httptest.ResponseRecorder {
		return w.bert.upload("/api/items/"+w.folder+"/attachments?name="+name+query, "application/octet-stream", content)
	}
	mov := quickTime("+46.5584+007.8352+1200.000/", 10000) // larger than other files may be
	if r := upload("IMG_0001.MOV", "", mov); r.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("a large video without video=1: %d", r.Code)
	}
	if r := upload("big.bin", "&video=1", bytes.Repeat([]byte{1}, 10000)); r.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("a large file that is no video: %d %s", r.Code, r.Body)
	}
	r := upload("IMG_0001.MOV", "&video=1&place=private", mov)
	if r.Code != http.StatusCreated {
		t.Fatalf("video: %d %s", r.Code, r.Body)
	}
	var up map[string]map[string]any
	_ = json.Unmarshal(r.Body.Bytes(), &up)
	a := up["attachment"]
	id := a["id"].(string)
	p, _ := a["photo"].(map[string]any)
	if a["type"] != "video/quicktime" || p == nil || p["duration"] != 10.0 || p["taken"] != "2024-05-01T11:22:10" || p["width"] != 1920.0 || p["lat"] == nil {
		t.Fatalf("the video as recorded: %v", a)
	}

	// The album shows it as a video, and narrows to videos.
	page := field(w.vera.expect("GET", "/api/album?kind=video", "", http.StatusOK), "pictures").([]any)
	if len(page) != 1 || page[0].(map[string]any)["video"] != true || page[0].(map[string]any)["poster"] != nil {
		t.Fatalf("videos in the album: %v", page)
	}
	if got := names(w.vera, "?kind=image"); len(got) != 0 {
		t.Errorf("a video among the images: %v", got)
	}
	// A reader gets no place, from the photo data nor from the file.
	pv := field(w.vera.expect("GET", "/api/album?kind=video", "", http.StatusOK), "pictures").([]any)[0].(map[string]any)["attachment"].(map[string]any)["photo"].(map[string]any)
	if pv["lat"] != nil {
		t.Errorf("a reader sees the private place: %v", pv)
	}
	file := "/api/items/" + w.folder + "/attachments/" + id
	theirs := w.vera.do("GET", file, "")
	mine := w.bert.do("GET", file, "")
	if theirs.Code != http.StatusOK || len(theirs.Body.Bytes()) != len(mov) || bytes.Contains(theirs.Body.Bytes(), []byte("46.5584")) {
		t.Fatalf("the video for a reader: %d, %d bytes", theirs.Code, theirs.Body.Len())
	}
	if !bytes.Equal(mine.Body.Bytes(), mov) {
		t.Error("the uploader does not get the video as stored")
	}

	// Without a poster it has no thumbnail; with one, the poster's.
	w.vera.expect("GET", file+"/thumbnail", "", http.StatusNotFound)
	poster := exifJPEG(t, "2024:05:01 13:22:10", 0, 0, 64)
	if r := w.vera.putBytes(file+"/poster", poster); r.Code != http.StatusForbidden {
		t.Errorf("a reader set a poster: %d", r.Code)
	}
	if r := w.bert.putBytes(file+"/poster", []byte("not a picture at all")); r.Code != http.StatusUnsupportedMediaType {
		t.Errorf("a poster that is no picture: %d", r.Code)
	}
	if r := w.bert.putBytes(file+"/poster", poster); r.Code != http.StatusNoContent {
		t.Fatalf("poster: %d %s", r.Code, r.Body)
	}
	if rec := w.vera.do("GET", file+"/thumbnail", ""); rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Errorf("thumbnail of a video: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	// The album says it has a poster, so a browser asks for it only then.
	if page := field(w.vera.expect("GET", "/api/album?kind=video", "", http.StatusOK), "pictures").([]any); page[0].(map[string]any)["poster"] != true {
		t.Errorf("the album does not tell the poster: %v", page[0])
	}
	// A picture has no poster.
	pic := w.put(t, "lake.jpg", exifJPEG(t, "2024:05:01 13:22:10", 0, 0, 64))
	if r := w.bert.putBytes("/api/items/"+w.folder+"/attachments/"+pic["id"].(string)+"/poster", poster); r.Code != http.StatusBadRequest {
		t.Errorf("a poster for a picture: %d", r.Code)
	}

	// Taken without its place, the video is stored again without it.
	r = upload("IMG_0002.MOV", "&video=1&place=strip", quickTime("+46.5584+007.8352/", 100))
	if r.Code != http.StatusCreated {
		t.Fatalf("stripped: %d %s", r.Code, r.Body)
	}
	_ = json.Unmarshal(r.Body.Bytes(), &up)
	if up["attachment"]["photo"].(map[string]any)["lat"] != nil || up["attachment"]["name"] != "IMG_0002.MOV" {
		t.Errorf("a stripped video: %v", up["attachment"])
	}
	stripped := w.bert.do("GET", "/api/items/"+w.folder+"/attachments/"+up["attachment"]["id"].(string), "")
	if bytes.Contains(stripped.Body.Bytes(), []byte("46.5584")) {
		t.Error("the stripped video still holds its place")
	}
}
