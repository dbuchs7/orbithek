package server

import (
	"container/list"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dbuchs7/orbithek/internal/budget"
	"github.com/dbuchs7/orbithek/internal/buildinfo"
	"github.com/dbuchs7/orbithek/internal/connector"
	"github.com/dbuchs7/orbithek/internal/photosource"
)

// The variables that turn the album's photo services on (ADR-0177). Each
// makes the server reach another service on people's behalf, so each is
// the operator's decision and off unless set.
const (
	// PhotosGoogleVar, set to "on", lets people bring photos in from
	// Google Photos through its picker, with the Google client of single
	// sign-on.
	PhotosGoogleVar = "OT_PHOTOS_GOOGLE"
	// PhotosICloudVar, set to "on", lets people bring photos in from a
	// shared iCloud album with a public website.
	PhotosICloudVar = "OT_PHOTOS_ICLOUD"
	// MapTilesVar names the tile service the album's map is drawn on, as
	// an address with {z}, {x} and {y}; the server fetches the tiles, so
	// readers' browsers reach no other site. Unset, OpenStreetMap's is
	// used (ADR-0179); "off" draws no tiles.
	MapTilesVar = "OT_MAP_TILES"
	// MapAttributionVar is the credit the map shows for its tiles, as the
	// tile service asks for it; unset with the default service, it is
	// OpenStreetMap's.
	MapAttributionVar = "OT_MAP_ATTRIBUTION"
	// MapAllowPrivateVar, set to "on", lets the tile service be in the
	// server's own network.
	MapAllowPrivateVar = "OT_MAP_ALLOW_PRIVATE"
	// GeocoderVar names the place search the album suggests places from
	// when a place is set by hand (ADR-0181): the address of a Photon
	// service; unset, the public one on OpenStreetMap's data; "off", none.
	GeocoderVar = "OT_GEOCODER"
)

// GooglePhotosCallback is the path Google sends a person back to after
// they allowed an import; the public address and this path must be among
// the redirect addresses of the Google client (ADR-0177).
const GooglePhotosCallback = "/api/album/google/callback"

// The tile service the album's map uses unless the operator names another
// or none (ADR-0179), and the credit its use asks for.
const (
	DefaultMapTiles       = "https://tile.openstreetmap.org/{z}/{x}/{y}.png"
	DefaultMapAttribution = "© OpenStreetMap contributors"
	DefaultGeocoder       = "https://photon.komoot.io"
)

// Photos holds what the album's photo management keeps in memory
// (ADR-0177): thumbnails and map tiles made or fetched before, and the
// imports from Google Photos and shared iCloud albums in progress.
type Photos struct {
	thumbs, tiles *byteCache
	// working bounds the thumbnails made at once.
	working chan struct{}
	// TileURL is the tile service's address template, empty for none;
	// TileCredit the credit shown beneath the map; tileAgent names this
	// server to the tile service, with its public address when known.
	TileURL    string
	TileCredit string
	tileClient *http.Client
	tileAgent  string
	// GeocoderURL is the place search's address, empty for none
	// (ADR-0181); its answers are kept a while in places.
	GeocoderURL string
	geoClient   *http.Client
	places      *byteCache
	// Google and ICloud are nil while turned off.
	Google *photosource.Google
	ICloud *photosource.ICloud

	now    func() time.Time
	mu     sync.Mutex
	albums map[string]sharedAlbum
}

// sharedAlbum is a shared iCloud album one person read, kept while they
// bring its photos in.
type sharedAlbum struct {
	album   photosource.Album
	expires time.Time
}

// albumLifetime is how long a shared album read stays at hand.
const albumLifetime = 30 * time.Minute

// PhotoClient makes the client the album reaches other services with:
// through the environment's proxy, with timeouts, and kept to public
// addresses unless allow names a variable set to "on".
func PhotoClient(getenv func(string) string, allow string) *http.Client {
	var transport http.RoundTripper = &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		MaxIdleConnsPerHost:   4,
		IdleConnTimeout:       90 * time.Second,
	}
	if allow == "" || getenv(allow) != "on" {
		transport = connector.PublicOnlyUnless(transport, allow)
	}
	return &http.Client{Timeout: 2 * time.Minute, Transport: transport, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || req.URL.Scheme != "https" {
			return http.ErrUseLastResponse
		}
		return nil
	}}
}

// NewPhotos sets the album's photo management up from the environment;
// google is the picker client when Google Photos is turned on and set
// up, else nil.
func NewPhotos(getenv func(string) string, google *photosource.Google) *Photos {
	b := budget.Get()
	p := &Photos{thumbs: newByteCache(b.ThumbnailCacheBytes), tiles: newByteCache(b.MapTileCacheBytes),
		working: make(chan struct{}, max(1, b.ThumbnailWorkers)), now: time.Now, albums: map[string]sharedAlbum{}}
	switch u := strings.TrimSpace(getenv(MapTilesVar)); {
	case u == "":
		p.TileURL, p.TileCredit = DefaultMapTiles, DefaultMapAttribution
	case validTileURL(u):
		p.TileURL = u
	}
	if credit := strings.TrimSpace(getenv(MapAttributionVar)); credit != "" && len(credit) <= 300 {
		p.TileCredit = credit
	}
	if p.TileURL != "" {
		p.tileClient = PhotoClient(getenv, MapAllowPrivateVar)
		p.tileAgent = "Orbithek/" + buildinfo.Get().Version + " (album map)"
		if public := strings.TrimSpace(getenv("OT_PUBLIC_URL")); strings.HasPrefix(public, "https://") || strings.HasPrefix(public, "http://") {
			p.tileAgent = "Orbithek/" + buildinfo.Get().Version + " (album map; +" + public + ")"
		}
	}
	switch u := strings.TrimRight(strings.TrimSpace(getenv(GeocoderVar)), "/"); {
	case u == "":
		p.GeocoderURL = DefaultGeocoder
	case strings.HasPrefix(u, "https://"):
		p.GeocoderURL = u
	}
	if p.GeocoderURL != "" {
		p.geoClient = PhotoClient(getenv, MapAllowPrivateVar)
		p.places = newByteCache(b.PlaceSearchCacheBytes)
		if p.tileAgent == "" {
			p.tileAgent = "Orbithek/" + buildinfo.Get().Version + " (album map)"
		}
	}
	if getenv(PhotosGoogleVar) == "on" {
		p.Google = google
	}
	if getenv(PhotosICloudVar) == "on" {
		p.ICloud = photosource.NewICloud(PhotoClient(getenv, ""))
	}
	return p
}

// validTileURL reports whether a tile address names https and all three
// of {z}, {x} and {y}.
func validTileURL(u string) bool {
	return strings.HasPrefix(u, "https://") && strings.Contains(u, "{z}") && strings.Contains(u, "{x}") && strings.Contains(u, "{y}")
}

// keepAlbum keeps a shared album one person read; at most PhotoImports
// albums are kept, the oldest given up first.
func (p *Photos) keepAlbum(user, token string, a photosource.Album) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.now()
	var oldest string
	for k, v := range p.albums {
		if !now.Before(v.expires) {
			delete(p.albums, k)
		} else if oldest == "" || v.expires.Before(p.albums[oldest].expires) {
			oldest = k
		}
	}
	if len(p.albums) >= max(1, budget.Get().PhotoImports) && oldest != "" {
		delete(p.albums, oldest)
	}
	p.albums[user+"\x00"+token] = sharedAlbum{album: a, expires: now.Add(albumLifetime)}
}

// album is a shared album the person read before, while it is at hand.
func (p *Photos) album(user, token string) (photosource.Album, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	a, ok := p.albums[user+"\x00"+token]
	if !ok || !p.now().Before(a.expires) {
		return photosource.Album{}, false
	}
	return a.album, true
}

// byteCache keeps answers in memory up to a number of bytes, dropping the
// least recently used first.
type byteCache struct {
	mu    sync.Mutex
	limit int64
	size  int64
	order *list.List
	at    map[string]*list.Element
}

type cached struct {
	key, kind string
	data      []byte
}

func newByteCache(limit int64) *byteCache {
	return &byteCache{limit: limit, order: list.New(), at: map[string]*list.Element{}}
}

func (c *byteCache) get(key string) ([]byte, string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.at[key]
	if !ok {
		return nil, "", false
	}
	c.order.MoveToFront(e)
	v := e.Value.(cached)
	return v.data, v.kind, true
}

// drop forgets every answer whose key starts with prefix.
func (c *byteCache) drop(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, e := range c.at {
		if strings.HasPrefix(key, prefix) {
			c.size -= int64(len(e.Value.(cached).data))
			c.order.Remove(e)
			delete(c.at, key)
		}
	}
}

func (c *byteCache) put(key, kind string, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if int64(len(data)) > c.limit/4 {
		return // one answer never takes more than a quarter
	}
	if e, ok := c.at[key]; ok {
		c.size -= int64(len(e.Value.(cached).data))
		c.order.Remove(e)
	}
	c.at[key] = c.order.PushFront(cached{key: key, kind: kind, data: data})
	c.size += int64(len(data))
	for c.size > c.limit {
		last := c.order.Back()
		v := last.Value.(cached)
		c.order.Remove(last)
		delete(c.at, v.key)
		c.size -= int64(len(v.data))
	}
}
