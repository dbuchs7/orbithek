package photo

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"regexp"
	"strconv"
	"time"
)

// Videos of the album (ADR-0189): MP4 and QuickTime files are ISO base
// media files, a tree of boxes. What a video tells about itself is read
// from a few small boxes of its movie header, seeking past the media
// data, so a video of gigabytes is read in kilobytes: when it was taken
// (mvhd, or Apple's creationdate), how long it plays (mvhd), its size and
// turn (tkhd), the camera (Apple's make and model) and where it was taken
// (©xyz, or Apple's location). WebM is recognised but not read.

// The media types of a video the album takes.
const (
	TypeMP4       = "video/mp4"
	TypeQuickTime = "video/quicktime"
	TypeWebM      = "video/webm"
)

// Bounds on walking a video's boxes; each box read whole is a small one.
const (
	maxVideoBoxes   = 10000   // boxes visited in one video
	maxVideoDepth   = 8       // nesting of boxes
	maxVideoBoxRead = 1 << 16 // bytes of one box read whole (mvhd, tkhd, keys, an item)
)

// mp4Brands are the ftyp brands of a video a browser plays as MP4.
var mp4Brands = map[string]bool{
	"isom": true, "iso2": true, "iso3": true, "iso4": true, "iso5": true, "iso6": true, "iso8": true, "iso9": true,
	"mp41": true, "mp42": true, "avc1": true, "M4V ": true, "M4VP": true, "M4VH": true, "dash": true, "msnv": true,
	"3gp4": true, "3gp5": true, "3gp6": true, "3g2a": true, "MSNV": true, "XAVC": true, "hvc1": true,
}

// VideoType tells what kind of video a file is from its first bytes, ""
// when it is none the album takes. A file of the brand "qt  " is
// QuickTime; MP4 is told by its brands, so an audio file (M4A) or a still
// image in the same container (HEIF) is no video.
func VideoType(head []byte) string {
	if bytes.HasPrefix(head, []byte{0x1A, 0x45, 0xDF, 0xA3}) && bytes.Contains(head[:min(len(head), 64)], []byte("webm")) {
		return TypeWebM
	}
	if len(head) < 12 {
		return ""
	}
	size := int(binary.BigEndian.Uint32(head))
	switch string(head[4:8]) {
	case "ftyp":
		if size < 16 || size > len(head) {
			size = min(len(head), 64)
		}
		brands := []string{string(head[8:12])}
		for i := 16; i+4 <= size; i += 4 {
			brands = append(brands, string(head[i:i+4]))
		}
		if brands[0] == "qt  " {
			return TypeQuickTime
		}
		for _, b := range brands {
			if mp4Brands[b] {
				return TypeMP4
			}
		}
		for _, b := range brands {
			if b == "qt  " {
				return TypeQuickTime
			}
		}
	case "moov", "mdat", "wide", "pnot":
		return TypeQuickTime // a QuickTime file written before ftyp existed
	}
	return ""
}

// IsVideo reports whether a media type is a video the album takes.
func IsVideo(mediaType string) bool {
	switch normType(mediaType) {
	case TypeMP4, TypeQuickTime, TypeWebM:
		return true
	}
	return false
}

// box is a box of the file: its type, where its content starts and how
// long it is.
type box struct {
	kind       string
	start, len int64
}

// videoWalk visits the boxes of an MP4 or QuickTime file that hold what
// the album reads, calling at for each with its path ("moov/udta/©xyz").
type videoWalk struct {
	r      io.ReadSeeker
	size   int64
	visits int
	at     func(path string, b box)
}

func (w *videoWalk) children(path string, from, to int64, depth int) {
	if depth > maxVideoDepth {
		return
	}
	for off := from; off+8 <= to; {
		w.visits++
		if w.visits > maxVideoBoxes {
			return
		}
		var h [16]byte
		if _, err := w.r.Seek(off, io.SeekStart); err != nil || !readFull(w.r, h[:8]) {
			return
		}
		n := int64(binary.BigEndian.Uint32(h[:4]))
		kind := string(h[4:8])
		head := int64(8)
		switch n {
		case 0:
			n = to - off
		case 1:
			if !readFull(w.r, h[8:16]) {
				return
			}
			big := binary.BigEndian.Uint64(h[8:16])
			if big > math.MaxInt64 {
				return
			}
			n, head = int64(big), 16
		}
		if n < head || off+n > to {
			return
		}
		b := box{kind: kind, start: off + head, len: n - head}
		p := kind
		if path != "" {
			p = path + "/" + kind
		}
		w.at(p, b)
		switch kind {
		case "moov", "trak", "udta", "mdia":
			// mdia holds a track's handler, which tells sound from
			// pictures (AudioType).
			w.children(p, b.start, b.start+b.len, depth+1)
		case "meta":
			// QuickTime's meta holds its children at once; ISO's has a
			// version and flags first.
			start := b.start
			var peek [8]byte
			if _, err := w.r.Seek(start, io.SeekStart); err == nil && readFull(w.r, peek[:]) && string(peek[4:8]) != "hdlr" && string(peek[4:8]) != "keys" {
				start += 4
			}
			w.children(p, start, b.start+b.len, depth+1)
		case "ilst":
			w.children(p, b.start, b.start+b.len, depth+1)
		}
		off += n
	}
}

// content reads a box's content whole, if it is small.
func (w *videoWalk) content(b box) []byte {
	if b.len > maxVideoBoxRead {
		return nil
	}
	if _, err := w.r.Seek(b.start, io.SeekStart); err != nil {
		return nil
	}
	return readUpTo(w.r, uint64(b.len), maxVideoBoxRead)
}

// quickTimeEpoch is where the times of a movie header count from.
var quickTimeEpoch = time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC)

// iso6709 is a place as ISO 6709 writes it in decimal degrees,
// "+46.5584+007.8352+1200.000/".
var iso6709 = regexp.MustCompile(`^([+-]\d{1,2}(?:\.\d+)?)([+-]\d{1,3}(?:\.\d+)?)([+-]\d+(?:\.\d+)?)?`)

// placeOf reads an ISO 6709 place.
func placeOf(s string, m *Meta) {
	g := iso6709.FindStringSubmatch(s)
	if g == nil {
		return
	}
	lat, err1 := strconv.ParseFloat(g[1], 64)
	lon, err2 := strconv.ParseFloat(g[2], 64)
	if err1 != nil || err2 != nil || math.Abs(lat) > 90 || math.Abs(lon) > 180 || (lat == 0 && lon == 0) {
		return
	}
	lat, lon = round(lat, 1e7), round(lon, 1e7)
	m.Lat, m.Lon = &lat, &lon
	if g[3] != "" {
		if alt, err := strconv.ParseFloat(g[3], 64); err == nil && math.Abs(alt) < 100000 {
			alt = round(alt, 100)
			m.Alt = &alt
		}
	}
}

// appleTime reads Apple's creationdate, "2024-05-01T13:22:10+0200".
func appleTime(s string, m *Meta) bool {
	if len(s) < 19 {
		return false
	}
	t, err := time.Parse("2006-01-02T15:04:05", s[:19])
	if err != nil {
		return false
	}
	m.Taken, m.Offset = t.Format(takenLayout), ""
	if off, _, ok := parseOffset(s[19:]); ok {
		m.Offset = off
	}
	return true
}

// takenLayout is how Meta.Taken writes a time.
const takenLayout = "2006-01-02T15:04:05"

// placeValue is where a video holds its place: the bytes of the text,
// which removing the place empties.
type placeValue struct{ off, len int64 }

// readVideo reads what an MP4 or QuickTime file tells about itself, and
// where it holds its place.
func readVideo(r io.ReadSeeker) (Meta, []placeValue) {
	var m Meta
	size, err := r.Seek(0, io.SeekEnd)
	if err != nil || size < 8 {
		return m, nil
	}
	var places []placeValue
	var keys []string
	type item struct {
		index uint32
		data  box
	}
	var items []item
	w := &videoWalk{r: r, size: size}
	w.at = func(path string, b box) {
		switch path {
		case "moov/mvhd":
			c := w.content(b)
			if len(c) < 20 {
				return
			}
			var created uint64
			var scale uint32
			var dur uint64
			if c[0] == 1 {
				if len(c) < 32 {
					return
				}
				created, scale, dur = binary.BigEndian.Uint64(c[4:12]), binary.BigEndian.Uint32(c[20:24]), binary.BigEndian.Uint64(c[24:32])
			} else {
				created, scale, dur = uint64(binary.BigEndian.Uint32(c[4:8])), binary.BigEndian.Uint32(c[12:16]), uint64(binary.BigEndian.Uint32(c[16:20]))
			}
			if scale > 0 && dur > 0 && dur != math.MaxUint32 && dur != math.MaxUint64 {
				m.Duration = round(float64(dur)/float64(scale), 1000)
			}
			// A movie header's time is UTC; 1 January 1970 or earlier is
			// a camera that did not know the time.
			if created > 66*365*86400 && created < 300*365*86400 && m.Taken == "" {
				t := quickTimeEpoch.Add(time.Duration(created) * time.Second)
				m.Taken, m.Offset = t.Format(takenLayout), "+00:00"
			}
		case "moov/trak/tkhd":
			if m.Width > 0 {
				return
			}
			c := w.content(b)
			at := 76 // version 0: matrix at 40, width at 76
			if len(c) > 0 && c[0] == 1 {
				at = 88
			}
			if len(c) < at+8 {
				return
			}
			width, height := binary.BigEndian.Uint32(c[at:at+4])>>16, binary.BigEndian.Uint32(c[at+4:at+8])>>16
			if width == 0 || height == 0 {
				return // a sound track
			}
			m.setSize(uint64(width), uint64(height))
			mx := c[at-36 : at]
			a, bb := int32(binary.BigEndian.Uint32(mx[0:4])), int32(binary.BigEndian.Uint32(mx[4:8]))
			switch {
			case a == 0 && bb > 0:
				m.Orientation = 6 // turned clockwise
			case a == 0 && bb < 0:
				m.Orientation = 8
			case a < 0:
				m.Orientation = 3
			}
		case "moov/udta/\xa9xyz", "moov/trak/udta/\xa9xyz":
			c := w.content(b)
			if len(c) < 4 {
				return
			}
			n := int(binary.BigEndian.Uint16(c[:2]))
			if n > len(c)-4 {
				n = len(c) - 4
			}
			if m.Lat == nil {
				placeOf(string(c[4:4+n]), &m)
			}
			places = append(places, placeValue{b.start + 4, int64(n)})
		case "moov/meta/keys":
			c := w.content(b)
			if len(c) < 8 {
				return
			}
			count := binary.BigEndian.Uint32(c[4:8])
			for p := 8; p+8 <= len(c) && uint32(len(keys)) < count; {
				n := int(binary.BigEndian.Uint32(c[p : p+4]))
				if n < 8 || p+n > len(c) {
					break
				}
				keys = append(keys, string(c[p+8:p+n]))
				p += n
			}
		default:
			// An item of Apple's metadata: moov/meta/ilst/<index>/data.
			if len(path) == 19 && path[:15] == "moov/meta/ilst/" {
				w.children(path, b.start, b.start+b.len, maxVideoDepth)
			}
			if len(path) == 24 && path[:15] == "moov/meta/ilst/" && b.kind == "data" {
				idx := binary.BigEndian.Uint32([]byte(path[15:19]))
				items = append(items, item{idx, b})
			}
		}
	}
	w.children("", 0, size, 0)
	for _, it := range items {
		if it.index == 0 || int(it.index) > len(keys) || it.data.len < 8 {
			continue
		}
		value := box{start: it.data.start + 8, len: it.data.len - 8} // type and locale first
		text := string(w.content(value))
		switch keys[it.index-1] {
		case "com.apple.quicktime.location.ISO6709":
			var p Meta
			placeOf(text, &p)
			if p.Lat != nil {
				m.Lat, m.Lon, m.Alt = p.Lat, p.Lon, p.Alt
			}
			places = append(places, placeValue{value.start, value.len})
		case "com.apple.quicktime.creationdate":
			appleTime(text, &m)
		case "com.apple.quicktime.make":
			m.Make = clean([]byte(text))
		case "com.apple.quicktime.model":
			m.Model = clean([]byte(text))
		}
	}
	return m, places
}

// ReadVideo reads what an MP4 or QuickTime video tells about itself:
// when it was taken, how long it plays, its size and turn, the camera and
// where it was taken. Best effort, as Read is: what cannot be read is
// left empty, and it never panics.
func ReadVideo(r io.ReadSeeker) (m Meta) {
	if r == nil {
		return Meta{}
	}
	defer func() {
		if recover() != nil {
			m = Meta{}
		}
	}()
	m, _ = readVideo(r)
	return m
}

// VideoWithoutPlace answers a video read without where it was taken: the
// texts that hold its place read as zeros, the rest as it is, so the
// video keeps its size and plays as before, and a part is read from any
// position as a player asks for it (ADR-0189). ok is false when the place
// could not be told apart, in which case the video must not be given.
func VideoWithoutPlace(r io.ReadSeeker) (out io.ReadSeeker, ok bool) {
	var places []placeValue
	var m Meta
	func() {
		defer func() {
			if recover() != nil {
				places, m = nil, Meta{Lat: new(float64)}
			}
		}()
		m, places = readVideo(r)
	}()
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, false
	}
	if m.Lat != nil && len(places) == 0 {
		return nil, false
	}
	return &blanked{r: r, places: places}, true
}

// blanked reads a file with some of its bytes as zeros.
type blanked struct {
	r      io.ReadSeeker
	pos    int64
	places []placeValue
}

func (b *blanked) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)
	for _, pl := range b.places {
		from, to := max(pl.off, b.pos), min(pl.off+pl.len, b.pos+int64(n))
		for i := from; i < to; i++ {
			p[i-b.pos] = 0
		}
	}
	b.pos += int64(n)
	return n, err
}

func (b *blanked) Seek(offset int64, whence int) (int64, error) {
	pos, err := b.r.Seek(offset, whence)
	if err == nil {
		b.pos = pos
	}
	return pos, err
}
