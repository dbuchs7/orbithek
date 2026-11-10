package photosource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

const token = "B0aGWZuqDGKS7Ke" // partition "0a" = 36

// fakeICloud answers as the stream hosts and content hosts of iCloud do.
type fakeICloud struct {
	mu        sync.Mutex
	moveTo    string // the host a 330 names, in its body
	moveHdr   bool   // name it in the header instead
	photos    []map[string]any
	location  string // url_location handed out
	path      string // url_path handed out
	streamed  []string
	assetReqs [][]string
	downloads []string
	status    map[string]int // host+path answered with this status
	body      []byte
	redirect  string // the content host redirects here
}

func newFakeICloud() *fakeICloud {
	return &fakeICloud{moveTo: "p42-sharedstreams.icloud.com", location: "cvws.icloud-content.com", path: "/B/photo.jpg?o=1",
		status: map[string]int{}, body: []byte("APPLEJPEG"),
		photos: []map[string]any{
			{"photoGuid": "g1", "caption": "Lake", "dateCreated": "2024-07-01T08:00:00Z", "width": "4032", "height": "3024",
				"derivatives": map[string]any{
					"342":  map[string]any{"checksum": "small", "fileSize": "1000", "width": "342", "height": "256"},
					"2049": map[string]any{"checksum": "large", "fileSize": "900000", "width": "2049", "height": "1536"},
				}},
			{"photoGuid": "g2", "batchDateCreated": "2024-07-02T08:00:00Z", "mediaAssetType": "video", "width": 1920, "height": 1080,
				"derivatives": map[string]any{"PosterFrame": map[string]any{"checksum": "poster", "width": "1920", "height": "1080"},
					"720p": map[string]any{"checksum": "video720", "fileSize": "5000000", "width": "1280", "height": "720"},
					"360p": map[string]any{"checksum": "video360", "fileSize": "900000", "width": "640", "height": "360"}}},
			{"photoGuid": "g4", "mediaAssetType": "video",
				"derivatives": map[string]any{"PosterFrame": map[string]any{"checksum": "poster4", "width": "1920", "height": "1080"}}},
			{"photoGuid": "g3", "derivatives": map[string]any{}},
			{"photoGuid": "", "derivatives": map[string]any{"a": map[string]any{"checksum": "x"}}},
			{"photoGuid": "g1", "derivatives": map[string]any{"a": map[string]any{"checksum": "y"}}},
		}}
}

func (f *fakeICloud) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	host := hostOf(r)
	if st := f.status[host+r.URL.Path]; st != 0 {
		http.Error(w, "refused", st)
		return
	}
	switch {
	case strings.HasSuffix(host, "-sharedstreams.icloud.com"):
		if r.Method != http.MethodPost || r.Header.Get("Origin") != "https://www.icloud.com" {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		if host != f.moveTo {
			if f.moveHdr {
				w.Header().Set("X-Apple-MMe-Host", f.moveTo)
				w.WriteHeader(330)
				return
			}
			w.WriteHeader(330)
			fmt.Fprintf(w, `{"X-Apple-MMe-Host":%q,"X-Apple-MMe-Host-Num":"42"}`, f.moveTo)
			return
		}
		switch strings.TrimPrefix(r.URL.Path, "/"+token+"/sharedstreams/") {
		case "webstream":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body) != 1 || body["streamCtag"] != nil {
				http.Error(w, "bad", http.StatusBadRequest)
				return
			}
			f.streamed = append(f.streamed, host)
			json.NewEncoder(w).Encode(map[string]any{"streamName": "Holidays", "userFirstName": "Anna", "userLastName": "Muster",
				"streamCtag": "FT;123", "itemsReturned": fmt.Sprint(len(f.photos)), "photos": f.photos})
		case "webasseturls":
			var body struct{ PhotoGuids []string }
			json.NewDecoder(r.Body).Decode(&body)
			f.assetReqs = append(f.assetReqs, body.PhotoGuids)
			json.NewEncoder(w).Encode(map[string]any{"items": map[string]any{
				"large":    map[string]any{"url_expiry": "2026-11-10T13:00:00Z", "url_location": f.location, "url_path": f.path},
				"small":    map[string]any{"url_location": "cvws.icloud-content.com", "url_path": "/small.jpg"},
				"video720": map[string]any{"url_location": "cvws.icloud-content.com", "url_path": "/B/video.mp4"},
				"poster":   map[string]any{"url_location": "cvws.icloud-content.com", "url_path": "/B/poster.jpg"},
			}, "locations": map[string]any{}})
		default:
			http.NotFound(w, r)
		}
	case strings.HasSuffix(host, ".icloud-content.com"):
		f.downloads = append(f.downloads, host+r.URL.RequestURI())
		if f.redirect != "" && r.URL.Path != "/after" {
			http.Redirect(w, r, f.redirect, http.StatusFound)
			return
		}
		w.(http.Flusher).Flush()
		w.Write(f.body)
	default:
		http.NotFound(w, r)
	}
}

func newICloud(t *testing.T) (*ICloud, *fakeICloud) {
	f := newFakeICloud()
	return NewICloud(serve(t, f)), f
}

func TestAlbumToken(t *testing.T) {
	good := map[string]string{
		"https://www.icloud.com/sharedalbum/#B0aGWZuqDGKS7Ke":        "B0aGWZuqDGKS7Ke",
		" https://www.icloud.com/sharedalbum/de-de/#B0aGWZuqDGKS7Ke": "B0aGWZuqDGKS7Ke",
		"https://WWW.iCloud.com/sharedalbum/en-us#A0123456789":       "A0123456789",
		"https://www.icloud.com/sharedalbum#B0aGWZuqDGKS7Ke":         "B0aGWZuqDGKS7Ke",
		"https://www.icloud.com/sharedalbum/zh-Hant-tw/#B0aGWZuqDG":  "B0aGWZuqDG",
	}
	for raw, want := range good {
		if got, err := AlbumToken(raw); err != nil || got != want {
			t.Errorf("AlbumToken(%q) = %q, %v", raw, got, err)
		}
	}
	for _, raw := range []string{
		"http://www.icloud.com/sharedalbum/#B0aGWZuqDGKS7Ke",
		"https://icloud.com/sharedalbum/#B0aGWZuqDGKS7Ke",
		"https://www.icloud.com.evil.example/sharedalbum/#B0aGWZuqDGKS7Ke",
		"https://u@www.icloud.com/sharedalbum/#B0aGWZuqDGKS7Ke",
		"https://www.icloud.com/photos/#B0aGWZuqDGKS7Ke",
		"https://www.icloud.com/sharedalbum/de-de/x/#B0aGWZuqDGKS7Ke",
		"https://www.icloud.com/sharedalbum/#short",
		"https://www.icloud.com/sharedalbum/#B0aGWZuqDGKS7Ke;x",
		"https://www.icloud.com/sharedalbum/#" + strings.Repeat("a", 33),
		"https://www.icloud.com/sharedalbum/",
		"%zz",
	} {
		if _, err := AlbumToken(raw); !errors.Is(err, ErrInvalid) {
			t.Errorf("AlbumToken(%q) accepted, %v", raw, err)
		}
	}
}

func TestPartitionHost(t *testing.T) {
	for tok, want := range map[string]string{
		"B0aGWZuqDGKS7Ke": "p36-sharedstreams.icloud.com",
		"A1bcdefghij":     "p01-sharedstreams.icloud.com",
		"AZbcdefghij":     "p35-sharedstreams.icloud.com",
		"B01cdefghij":     "p01-sharedstreams.icloud.com",
		"B10cdefghij":     "p62-sharedstreams.icloud.com",
	} {
		if got := partitionHost(tok); got != want {
			t.Errorf("partitionHost(%q) = %q, want %q", tok, got, want)
		}
	}
	if c := NewICloud(nil); c.client != http.DefaultClient || c.Host(token) != "p36-sharedstreams.icloud.com" {
		t.Fatal("defaults")
	}
}

func TestAlbumFollows330AndReadsPhotos(t *testing.T) {
	c, f := newICloud(t)
	a, err := c.Album(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.streamed) != 1 || f.streamed[0] != "p42-sharedstreams.icloud.com" {
		t.Fatalf("streamed on %v", f.streamed)
	}
	if a.Name != "Holidays" || a.Owner != "Anna Muster" || len(a.Photos) != 3 {
		t.Fatalf("album %+v", a)
	}
	p := a.Photos[0]
	if p.GUID != "g1" || p.Caption != "Lake" || !p.Created.Equal(time.Date(2024, 7, 1, 8, 0, 0, 0, time.UTC)) ||
		p.Width != 4032 || p.Height != 3024 || p.Video || p.checksum != "large" || p.host != "p42-sharedstreams.icloud.com" {
		t.Fatalf("photo %+v", p)
	}
	v := a.Photos[1]
	if !v.Video || v.Width != 1920 || !v.Created.Equal(time.Date(2024, 7, 2, 8, 0, 0, 0, time.UTC)) || v.checksum != "video720" || v.poster != "poster" {
		t.Fatalf("video %+v", v)
	}

	rc, err := c.Download(context.Background(), token, p, 100)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil || string(data) != "APPLEJPEG" {
		t.Fatalf("download %q, %v", data, err)
	}
	if len(f.assetReqs) != 1 || len(f.assetReqs[0]) != 1 || f.assetReqs[0][0] != "g1" ||
		f.downloads[0] != "cvws.icloud-content.com/B/photo.jpg?o=1" {
		t.Fatalf("asked %v, downloaded %v", f.assetReqs, f.downloads)
	}

	// A video is its largest video, not its poster frame, which is its
	// poster (ADR-0192).
	if _, err = c.Download(context.Background(), token, v, 100); err != nil || f.downloads[1] != "cvws.icloud-content.com/B/video.mp4" {
		t.Fatalf("video downloaded %v, %v", f.downloads, err)
	}
	if _, err = c.Poster(context.Background(), token, v, 100); err != nil || f.downloads[2] != "cvws.icloud-content.com/B/poster.jpg" {
		t.Fatalf("poster downloaded %v, %v", f.downloads, err)
	}
	_, err = c.Poster(context.Background(), token, p, 100)
	wantErr(t, err, ErrNotFound)
	// A video Apple offers only a picture of is listed, not downloaded.
	only := a.Photos[2]
	if only.GUID != "g4" || !only.Video {
		t.Fatalf("poster only %+v", only)
	}
	_, err = c.Download(context.Background(), token, only, 100)
	wantErr(t, err, ErrNotFound)
}

func TestAlbumWithoutRedirectAndHeaderRedirect(t *testing.T) {
	c, f := newICloud(t)
	f.moveTo = "p36-sharedstreams.icloud.com"
	if _, err := c.Album(context.Background(), token); err != nil || f.streamed[0] != "p36-sharedstreams.icloud.com" {
		t.Fatalf("streamed %v, %v", f.streamed, err)
	}
	c, f = newICloud(t)
	f.moveHdr = true
	if _, err := c.Album(context.Background(), token); err != nil || f.streamed[0] != "p42-sharedstreams.icloud.com" {
		t.Fatalf("streamed %v, %v", f.streamed, err)
	}
	// A test may name the first host.
	c, f = newICloud(t)
	c.Host = func(string) string { return "p42-sharedstreams.icloud.com" }
	if _, err := c.Album(context.Background(), token); err != nil {
		t.Fatal(err)
	}
	c.Host = nil
	if _, err := c.Album(context.Background(), token); err != nil {
		t.Fatal(err)
	}
}

func TestAlbumRefusesRedirectsAwayFromICloud(t *testing.T) {
	for _, host := range []string{"evil.example", "icloud.com", "p1.icloud.com.evil.example", "p1.icloud.com:8443", "u@p1.icloud.com", "p1.icloud.com/x", ""} {
		c, f := newICloud(t)
		f.moveTo = host
		_, err := c.Album(context.Background(), token)
		wantErr(t, err, ErrUnreadable)
		if len(f.streamed) != 0 {
			t.Fatalf("%q: followed", host)
		}
	}
}

func TestAlbumSentOnTwice(t *testing.T) {
	// Every host answers 330: the second one is not followed.
	c := NewICloud(serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(330)
		w.Write([]byte(`{"X-Apple-MMe-Host":"p99-sharedstreams.icloud.com"}`))
	})))
	_, err := c.Album(context.Background(), token)
	wantErr(t, err, ErrUnreadable)
	if !strings.Contains(err.Error(), "again") {
		t.Fatalf("message %v", err)
	}
}

func TestAlbumRefusals(t *testing.T) {
	ctx := context.Background()
	c, f := newICloud(t)
	_, err := c.Album(ctx, "bad token")
	wantErr(t, err, ErrInvalid)

	f.status["p42-sharedstreams.icloud.com/"+token+"/sharedstreams/webstream"] = http.StatusNotFound
	_, err = c.Album(ctx, token)
	wantErr(t, err, ErrNotFound)

	f.status["p42-sharedstreams.icloud.com/"+token+"/sharedstreams/webstream"] = http.StatusServiceUnavailable
	_, err = c.Album(ctx, token)
	wantErr(t, err, ErrUnreadable)
	if !strings.Contains(err.Error(), "503") || strings.Contains(err.Error(), token) {
		t.Fatalf("message %v", err)
	}
	delete(f.status, "p42-sharedstreams.icloud.com/"+token+"/sharedstreams/webstream")

	f.photos = make([]map[string]any, maxPhotos+1)
	_, err = c.Album(ctx, token)
	wantErr(t, err, ErrTooMany)

	cctx, cancel := context.WithCancel(ctx)
	cancel()
	_, err = c.Album(cctx, token)
	wantErr(t, err, ErrUnreadable)

	for _, answer := range []string{"not json", ""} {
		c := NewICloud(serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(answer))
		})))
		_, err = c.Album(ctx, token)
		wantErr(t, err, ErrUnreadable)
	}

	c = NewICloud(serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(330)
		w.Write(make([]byte, drainBytes+1))
	})))
	_, err = c.Album(ctx, token)
	wantErr(t, err, ErrTooLarge)

	c = NewICloud(serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.(http.Flusher).Flush()
		w.Write(make([]byte, maxAlbumAnswer+1))
	})))
	_, err = c.Album(ctx, token)
	wantErr(t, err, ErrTooLarge)

	// A stream host that redirects elsewhere is not followed.
	c = NewICloud(serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://evil.example/x", http.StatusTemporaryRedirect)
	})))
	_, err = c.Album(ctx, token)
	wantErr(t, err, ErrUnreadable)
}

func TestBestDerivative(t *testing.T) {
	w := wirePhoto{Derivatives: map[string]wireDerivative{
		"a": {Checksum: "a", Width: 10, Height: 10, FileSize: 5},
		"b": {Checksum: "b", Width: 10, Height: 10, FileSize: 7},
		"c": {Checksum: "c", Width: 10, Height: 10, FileSize: 7},
		"d": {Width: 100, Height: 100},
		"e": {Checksum: "e", Width: 5, Height: 5, FileSize: 99},
	}}
	if got := w.best(); got != "b" {
		t.Fatalf("best %q", got)
	}
}

func TestDownloadChecksTheAssetAddress(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct{ location, path string }{
		{"evil.example", "/x.jpg"},
		{"icloud-content.com", "/x.jpg"},
		{"cvws.icloud-content.com.evil.example", "/x.jpg"},
		{"cvws.icloud-content.com", "@evil.example/x.jpg"},
		{"cvws.icloud-content.com", ".evil.example/x.jpg"},
		{"cvws.icloud-content.com:444", "/x.jpg"},
		{"cvws.icloud-content.com", "x.jpg"},
		{"cvws.icloud-content.com", "/%zz"},
	} {
		c, f := newICloud(t)
		f.location, f.path = tc.location, tc.path
		a, err := c.Album(ctx, token)
		if err != nil {
			t.Fatal(err)
		}
		_, err = c.Download(ctx, token, a.Photos[0], 100)
		wantErr(t, err, ErrUnreadable)
		if len(f.downloads) != 0 {
			t.Fatalf("%+v: downloaded", tc)
		}
	}
	// An asset on an icloud.com host is allowed.
	c, f := newICloud(t)
	f.location = "cvws.icloud-content.com"
	a, _ := c.Album(ctx, token)
	if rc, err := c.Download(ctx, token, a.Photos[0], 100); err != nil {
		t.Fatal(err)
	} else {
		rc.Close()
	}
}

func TestDownloadRefusals(t *testing.T) {
	ctx := context.Background()
	c, f := newICloud(t)
	a, _ := c.Album(ctx, token)
	p := a.Photos[0]

	_, err := c.Download(ctx, "bad", p, 100)
	wantErr(t, err, ErrInvalid)
	_, err = c.Download(ctx, token, p, 0)
	wantErr(t, err, ErrInvalid)
	_, err = c.Download(ctx, token, SharedPhoto{GUID: "g1"}, 100)
	wantErr(t, err, ErrInvalid)

	// A photo whose derivative iCloud no longer hands out.
	q := p
	q.checksum = "gone"
	_, err = c.Download(ctx, token, q, 100)
	wantErr(t, err, ErrNotFound)

	// Without the host Album found, the partition host is asked and its 330 followed.
	q = p
	q.host = ""
	rc, err := c.Download(ctx, token, q, 100)
	if err != nil {
		t.Fatal(err)
	}
	rc.Close()

	// Longer than the limit fails while reading.
	rc, err = c.Download(ctx, token, p, 3)
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.ReadAll(rc)
	rc.Close()
	wantErr(t, err, ErrTooLarge)

	f.status["cvws.icloud-content.com/B/photo.jpg"] = http.StatusNotFound
	_, err = c.Download(ctx, token, p, 100)
	wantErr(t, err, ErrNotFound)
	f.status["cvws.icloud-content.com/B/photo.jpg"] = http.StatusForbidden
	_, err = c.Download(ctx, token, p, 100)
	wantErr(t, err, ErrUnreadable)
	delete(f.status, "cvws.icloud-content.com/B/photo.jpg")

	f.status["p42-sharedstreams.icloud.com/"+token+"/sharedstreams/webasseturls"] = http.StatusNotFound
	_, err = c.Download(ctx, token, p, 100)
	wantErr(t, err, ErrNotFound)
	delete(f.status, "p42-sharedstreams.icloud.com/"+token+"/sharedstreams/webasseturls")

	// Redirects of the content host: to another of iCloud's hosts, followed; elsewhere, not.
	f.redirect = "https://cvws2.icloud-content.com/after"
	rc, err = c.Download(ctx, token, p, 100)
	if err != nil {
		t.Fatal(err)
	}
	rc.Close()
	f.redirect = "https://evil.example/after"
	_, err = c.Download(ctx, token, p, 100)
	wantErr(t, err, ErrUnreadable)
	f.redirect = "http://cvws2.icloud-content.com/after"
	_, err = c.Download(ctx, token, p, 100)
	wantErr(t, err, ErrUnreadable)
}

func TestDownloadUnreadableAssetAnswer(t *testing.T) {
	c := NewICloud(serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"items": 3}`))
	})))
	p := SharedPhoto{GUID: "g1", checksum: "large", host: "p42-sharedstreams.icloud.com"}
	_, err := c.Download(context.Background(), token, p, 100)
	wantErr(t, err, ErrUnreadable)
}

func TestDownloadCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	c := NewICloud(serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cancel()
		w.Write([]byte(`{"items":{"large":{"url_location":"cvws.icloud-content.com","url_path":"/x"}}}`))
	})))
	p := SharedPhoto{GUID: "g1", checksum: "large", host: "p42-sharedstreams.icloud.com"}
	_, err := c.Download(ctx, token, p, 100)
	wantErr(t, err, ErrUnreadable)
}
