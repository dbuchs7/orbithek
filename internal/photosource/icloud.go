package photosource

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Bounds of reading a shared album: how long its answer may be and how
// many photos it may hold (Apple allows 5,000 in a shared album).
const (
	maxAlbumAnswer = 16 << 20
	maxPhotos      = 5000
)

// streamHosts are the hosts a shared album's stream may be read on;
// assetHosts those its photos may be downloaded from.
var (
	streamHosts = []string{".icloud.com"}
	assetHosts  = []string{".icloud-content.com", ".icloud.com"}
)

// albumToken is the token of a shared album, the part of its address after
// the "#".
var albumToken = regexp.MustCompile(`^[0-9A-Za-z]{10,32}$`)

// albumPath is the path of a shared album's address, with or without a
// locale such as de-de.
var albumPath = regexp.MustCompile(`^/sharedalbum(/[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8})*)?/?$`)

const base62 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// ICloud reads public iCloud shared albums: those whose owner turned on
// "Public Website", with an address such as
// https://www.icloud.com/sharedalbum/#B0aGWZuqDGKS7Ke.
//
// Apple has no public photo API. ICloud speaks the protocol the album's
// web page speaks — a "webstream" listing the photos and "webasseturls"
// handing out their download addresses — as open-source projects
// describe it. It is undocumented and may change or break at any time;
// what Apple answers is checked, never trusted.
//
// ICloud keeps no state and is safe for concurrent use.
type ICloud struct {
	// Host answers the host an album's stream is read on first; Apple may
	// name another. It is the album's partition host unless a test sets
	// it.
	Host func(token string) string

	client *http.Client
}

// NewICloud sets up reading shared albums with the client every request
// goes through. A nil client is http.DefaultClient.
func NewICloud(client *http.Client) *ICloud {
	if client == nil {
		client = http.DefaultClient
	}
	return &ICloud{Host: partitionHost, client: client}
}

// partitionHost is the host of an album's partition, which its token
// tells in base 62: the second character if the token starts with "A",
// else the second and third.
func partitionHost(token string) string {
	digits := token[1:3]
	if token[0] == 'A' {
		digits = token[1:2]
	}
	n := 0
	for _, c := range digits {
		n = n*62 + strings.IndexRune(base62, c)
	}
	return fmt.Sprintf("p%02d-sharedstreams.icloud.com", n)
}

// AlbumToken answers the token of a shared album's address: https on
// www.icloud.com, the path /sharedalbum/ with or without a locale, and the
// token as its fragment. Anything else is ErrInvalid.
func AlbumToken(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.User != nil || !strings.EqualFold(u.Host, "www.icloud.com") ||
		!albumPath.MatchString(u.Path) || !albumToken.MatchString(u.Fragment) {
		return "", fmt.Errorf("%w: not the address of a public iCloud shared album", ErrInvalid)
	}
	return u.Fragment, nil
}

// SharedPhoto is a photo or video of a shared album. Its best derivative,
// the largest Apple offers, is the one downloaded.
type SharedPhoto struct {
	GUID, Caption string
	Created       time.Time
	Width, Height int
	Video         bool

	checksum string
	host     string
}

// Album is a shared album: its name, its owner's name and its photos.
type Album struct {
	Name, Owner string
	Photos      []SharedPhoto
}

// wireDerivative is one size of a photo Apple offers; Apple writes the
// numbers as strings.
type wireDerivative struct {
	Checksum string `json:"checksum"`
	FileSize number `json:"fileSize"`
	Width    number `json:"width"`
	Height   number `json:"height"`
}

type wirePhoto struct {
	PhotoGUID        string                    `json:"photoGuid"`
	Caption          string                    `json:"caption"`
	DateCreated      string                    `json:"dateCreated"`
	BatchDateCreated string                    `json:"batchDateCreated"`
	Width            number                    `json:"width"`
	Height           number                    `json:"height"`
	MediaAssetType   string                    `json:"mediaAssetType"`
	Derivatives      map[string]wireDerivative `json:"derivatives"`
}

// best answers the checksum of the largest derivative, by its pixels and
// then its bytes, ties broken by the checksum so the choice is stable.
func (w wirePhoto) best() string {
	var best wireDerivative
	for _, d := range w.Derivatives {
		if d.Checksum == "" {
			continue
		}
		pd, pb := int64(d.Width)*int64(d.Height), int64(best.Width)*int64(best.Height)
		if best.Checksum == "" || pd > pb || pd == pb && (d.FileSize > best.FileSize ||
			d.FileSize == best.FileSize && d.Checksum < best.Checksum) {
			best = d
		}
	}
	return best.Checksum
}

func (c *ICloud) host(token string) string {
	if c.Host != nil {
		return c.Host(token)
	}
	return partitionHost(token)
}

// Album reads a shared album's stream. Apple may answer 330 with the host
// the album lives on, which is asked once instead, if it is one of
// iCloud's. An album that is gone or no longer public is ErrNotFound.
func (c *ICloud) Album(ctx context.Context, token string) (Album, error) {
	if !albumToken.MatchString(token) {
		return Album{}, fmt.Errorf("%w: not the token of a shared album", ErrInvalid)
	}
	data, host, err := c.post(ctx, c.host(token), token, "webstream", map[string]any{"streamCtag": nil})
	if err != nil {
		return Album{}, err
	}
	var answer struct {
		StreamName    string      `json:"streamName"`
		UserFirstName string      `json:"userFirstName"`
		UserLastName  string      `json:"userLastName"`
		Photos        []wirePhoto `json:"photos"`
	}
	if err := json.Unmarshal(data, &answer); err != nil {
		return Album{}, fmt.Errorf("%w: iCloud's answer cannot be read", ErrUnreadable)
	}
	if len(answer.Photos) > maxPhotos {
		return Album{}, fmt.Errorf("%w: the album holds more than %d photos", ErrTooMany, maxPhotos)
	}
	album := Album{Name: answer.StreamName,
		Owner: strings.TrimSpace(answer.UserFirstName + " " + answer.UserLastName)}
	seen := map[string]bool{}
	for _, w := range answer.Photos {
		sum := w.best()
		if w.PhotoGUID == "" || sum == "" || seen[w.PhotoGUID] {
			continue
		}
		seen[w.PhotoGUID] = true
		created, err := time.Parse(time.RFC3339, w.DateCreated)
		if err != nil {
			created, _ = time.Parse(time.RFC3339, w.BatchDateCreated)
		}
		album.Photos = append(album.Photos, SharedPhoto{GUID: w.PhotoGUID, Caption: w.Caption, Created: created,
			Width: w.Width.int(), Height: w.Height.int(), Video: w.MediaAssetType == "video",
			checksum: sum, host: host})
	}
	return album, nil
}

// post sends one request of the album protocol to a stream host and reads
// its answer, following one 330 to the host Apple names in its answer or
// a header. It answers the host that answered.
func (c *ICloud) post(ctx context.Context, host, token, endpoint string, body any) ([]byte, string, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	client := guarded(c.client, func(next, _ *url.URL) bool { return allowed(next, streamHosts...) })
	for try := 0; ; try++ {
		addr := "https://" + host + "/" + token + "/sharedstreams/" + endpoint
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, addr, bytes.NewReader(payload))
		if err != nil {
			return nil, "", fmt.Errorf("%w: %s", ErrUnreadable, reason(err))
		}
		req.Header.Set("Content-Type", "text/plain")
		req.Header.Set("Accept", "*/*")
		req.Header.Set("Origin", "https://www.icloud.com")
		req.Header.Set("Referer", "https://www.icloud.com/sharedalbum/")
		resp, err := client.Do(req)
		if err != nil {
			return nil, "", fmt.Errorf("%w: %s", ErrUnreadable, reason(err))
		}
		switch {
		case resp.StatusCode == 330:
			next, err := moved(resp)
			if err != nil {
				return nil, "", err
			}
			if try > 0 {
				return nil, "", fmt.Errorf("%w: iCloud sends the album on again", ErrUnreadable)
			}
			host = next
			continue
		case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
			drain(resp)
			return nil, "", fmt.Errorf("%w: no public shared album (%d)", ErrNotFound, resp.StatusCode)
		case resp.StatusCode < 200 || resp.StatusCode > 299:
			drain(resp)
			return nil, "", fmt.Errorf("%w: iCloud answers %d", ErrUnreadable, resp.StatusCode)
		}
		data, err := readBounded(resp.Body, maxAlbumAnswer)
		resp.Body.Close()
		return data, host, err
	}
}

// moved reads the host of a 330 answer, from its JSON field
// "X-Apple-MMe-Host" or else the header of that name, and checks it is
// one of iCloud's.
func moved(resp *http.Response) (string, error) {
	defer resp.Body.Close()
	data, err := readBounded(resp.Body, drainBytes)
	if err != nil {
		return "", err
	}
	var answer struct {
		Host string `json:"X-Apple-MMe-Host"`
	}
	json.Unmarshal(data, &answer)
	host := answer.Host
	if host == "" {
		host = resp.Header.Get("X-Apple-MMe-Host")
	}
	if u, err := url.Parse("https://" + host + "/"); host == "" || err != nil || u.Host != host || !allowed(u, streamHosts...) {
		return "", fmt.Errorf("%w: iCloud sends the album to a host that is not iCloud's", ErrUnreadable)
	}
	return host, nil
}

// assetURL asks the stream host for the download address of one photo's
// best derivative and checks it is one of iCloud's.
func (c *ICloud) assetURL(ctx context.Context, token string, p SharedPhoto) (string, error) {
	host := p.host
	if host == "" {
		host = c.host(token)
	}
	data, _, err := c.post(ctx, host, token, "webasseturls", map[string]any{"photoGuids": []string{p.GUID}})
	if err != nil {
		return "", err
	}
	var answer struct {
		Items map[string]struct {
			Location string `json:"url_location"`
			Path     string `json:"url_path"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &answer); err != nil {
		return "", fmt.Errorf("%w: iCloud's answer cannot be read", ErrUnreadable)
	}
	it, ok := answer.Items[p.checksum]
	if !ok {
		return "", fmt.Errorf("%w: iCloud hands out no address for the photo", ErrNotFound)
	}
	addr := "https://" + it.Location + it.Path
	u, err := url.Parse(addr)
	if err != nil || !strings.HasPrefix(it.Path, "/") || u.Host != it.Location || !allowed(u, assetHosts...) {
		return "", fmt.Errorf("%w: iCloud hands out an address that is not iCloud's", ErrUnreadable)
	}
	return addr, nil
}

// Download reads one photo of a shared album, its best derivative, at most
// limit bytes (more fails with ErrTooLarge while reading). The photo must
// come from Album; a video is ErrVideo. Its download address is asked for
// anew, as Apple's addresses expire, and redirects are followed only to
// iCloud's hosts.
func (c *ICloud) Download(ctx context.Context, token string, p SharedPhoto, limit int64) (io.ReadCloser, error) {
	switch {
	case !albumToken.MatchString(token):
		return nil, fmt.Errorf("%w: not the token of a shared album", ErrInvalid)
	case limit <= 0:
		return nil, fmt.Errorf("%w: no limit", ErrInvalid)
	case p.Video:
		return nil, ErrVideo
	case p.GUID == "" || p.checksum == "":
		return nil, fmt.Errorf("%w: a photo not read from its album", ErrInvalid)
	}
	addr, err := c.assetURL(ctx, token, p)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, addr, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnreadable, reason(err))
	}
	client := guarded(c.client, func(next, _ *url.URL) bool { return allowed(next, assetHosts...) })
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnreadable, reason(err))
	}
	return download(resp, limit)
}
