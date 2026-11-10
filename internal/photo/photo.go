package photo

import (
	"encoding/binary"
	"image/gif"
	"io"
	"math"
	"strings"
	"time"
)

// Meta is what a picture tells about itself. It is persisted inside the event
// log, so its JSON names are part of the log format.
type Meta struct {
	// Taken is the wall-clock capture time "2006-01-02T15:04:05" from
	// DateTimeOriginal, else DateTimeDigitized, else IFD0's DateTime.
	Taken string `json:"taken,omitempty"`
	// Offset is the UTC offset ("+02:00") recorded for the time Taken came
	// from; empty when unknown.
	Offset string `json:"offset,omitempty"`
	// Lat and Lon are decimal degrees, south and west negative, set only
	// together, when both are valid and not (0, 0).
	Lat *float64 `json:"lat,omitempty"`
	Lon *float64 `json:"lon,omitempty"`
	// Alt is the altitude in metres, below sea level negative.
	Alt *float64 `json:"alt,omitempty"`
	// Make, Model and Lens are trimmed, printable and at most 64 runes.
	Make  string `json:"make,omitempty"`
	Model string `json:"model,omitempty"`
	Lens  string `json:"lens,omitempty"`
	// Exposure is the exposure time: "1/250" below a second, else "2.5".
	Exposure string  `json:"exposure,omitempty"`
	FNumber  float64 `json:"fNumber,omitempty"`
	ISO      int     `json:"iso,omitempty"`
	// Focal is the focal length in millimetres, Focal35 its 35 mm equivalent.
	Focal   float64 `json:"focal,omitempty"`
	Focal35 int     `json:"focal35,omitempty"`
	// Orientation is the EXIF orientation 1..8, 0 when absent or invalid.
	Orientation int `json:"orientation,omitempty"`
	// Width and Height are the pixel size as stored, before orientation.
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`
}

// Bounds on walking a file's structure.
const (
	maxSegments   = 10000   // JPEG markers before SOS
	maxFill       = 1 << 16 // JPEG fill bytes before a marker
	maxPNGChunks  = 1 << 17 // a PNG of 1 GiB in 8 KiB IDAT chunks
	maxWebPChunks = 1 << 14 // an animated WebP has a chunk per frame
	maxExifBytes  = 1 << 20 // EXIF read from a PNG or WebP chunk
)

// normType reduces a media type to its lower-case essence without parameters.
func normType(mediaType string) string {
	t, _, _ := strings.Cut(mediaType, ";")
	t = strings.ToLower(strings.TrimSpace(t))
	if t == "image/jpg" || t == "image/pjpeg" {
		t = "image/jpeg"
	}
	return t
}

// Read reads the metadata of a JPEG, PNG, GIF or WebP file. mediaType is the
// type detected from the content ("image/jpeg", "image/png", "image/gif",
// "image/webp"). Best effort: what cannot be read is left empty; it never
// panics. It reads only what it needs, seeking past the rest, starting at the
// beginning of r whatever r's position.
func Read(r io.ReadSeeker, mediaType string) (m Meta) {
	if r == nil {
		return Meta{}
	}
	defer func() {
		// A last line of defence: a reader that panics must not take the
		// upload down. The parsers themselves do not panic (FuzzRead).
		if recover() != nil {
			m = Meta{}
		}
	}()
	return read(r, mediaType)
}

func read(r io.ReadSeeker, mediaType string) Meta {
	var m Meta
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return m
	}
	switch normType(mediaType) {
	case "image/jpeg":
		readJPEG(r, &m)
	case "image/png":
		readPNG(r, &m)
	case "image/webp":
		readWebP(r, &m)
	case "image/gif":
		if c, err := gif.DecodeConfig(r); err == nil {
			m.setSize(uint64(c.Width), uint64(c.Height))
		}
	}
	return m
}

// setSize records a pixel size if both sides are positive and fit an int32.
func (m *Meta) setSize(w, h uint64) {
	if w > 0 && h > 0 && w <= math.MaxInt32 && h <= math.MaxInt32 {
		m.Width, m.Height = int(w), int(h)
	}
}

// readFull fills b from r.
func readFull(r io.Reader, b []byte) bool {
	_, err := io.ReadFull(r, b)
	return err == nil
}

// readUpTo reads n bytes, at most limit of them; less when r ends early.
func readUpTo(r io.Reader, n, limit uint64) []byte {
	b := make([]byte, min(n, limit))
	k, _ := io.ReadFull(r, b)
	return b[:k]
}

// skip moves n bytes forward.
func skip(r io.Seeker, n uint64) bool {
	if n > math.MaxInt64 {
		return false
	}
	_, err := r.Seek(int64(n), io.SeekCurrent)
	return err == nil
}

// isSOF reports whether a JPEG marker starts a frame (and holds its size).
func isSOF(c byte) bool {
	return c >= 0xC0 && c <= 0xCF && c != 0xC4 && c != 0xC8 && c != 0xCC
}

// readJPEG walks the markers from SOI to SOS, reading the first Exif APP1
// segment and the size from the first SOF.
func readJPEG(r io.ReadSeeker, m *Meta) {
	var b [6]byte
	if !readFull(r, b[:2]) || b[0] != 0xFF || b[1] != 0xD8 {
		return
	}
	haveExif, haveSize := false, false
	for range maxSegments {
		if !readFull(r, b[:1]) || b[0] != 0xFF {
			return
		}
		code := byte(0xFF)
		for n := 0; code == 0xFF; n++ {
			if n > maxFill || !readFull(r, b[:1]) {
				return
			}
			code = b[0]
		}
		switch {
		case code == 0xD9 || code == 0xDA || code == 0x00: // EOI, SOS, invalid
			return
		case code == 0x01 || (code >= 0xD0 && code <= 0xD7): // no length
			continue
		}
		if !readFull(r, b[:2]) {
			return
		}
		n := int(binary.BigEndian.Uint16(b[:2])) - 2
		if n < 0 {
			return
		}
		switch {
		case code == 0xE1 && !haveExif && n >= 6:
			if !readFull(r, b[:6]) {
				return
			}
			n -= 6
			if string(b[:5]) == "Exif\x00" {
				haveExif = true
				data := readUpTo(r, uint64(n), uint64(n))
				m.fromTIFF(data)
				if len(data) < n {
					return
				}
				n = 0
			}
		case isSOF(code) && !haveSize && n >= 5:
			if !readFull(r, b[:5]) {
				return
			}
			n -= 5
			haveSize = true
			m.setSize(uint64(binary.BigEndian.Uint16(b[3:5])), uint64(binary.BigEndian.Uint16(b[1:3])))
		}
		if haveExif && haveSize {
			return
		}
		if !skip(r, uint64(n)) {
			return
		}
	}
}

// readPNG walks the chunks up to IEND, reading the size from IHDR and the
// first eXIf chunk, wherever it is.
func readPNG(r io.ReadSeeker, m *Meta) {
	var b [8]byte
	if !readFull(r, b[:]) || string(b[:]) != "\x89PNG\r\n\x1a\n" {
		return
	}
	haveExif, haveSize := false, false
	for i := range maxPNGChunks {
		if !readFull(r, b[:]) {
			return
		}
		n := uint64(binary.BigEndian.Uint32(b[:4]))
		if n > math.MaxInt32 {
			return
		}
		switch string(b[4:8]) {
		case "IHDR":
			if i == 0 && n >= 8 {
				if !readFull(r, b[:]) {
					return
				}
				n -= 8
				haveSize = true
				m.setSize(uint64(binary.BigEndian.Uint32(b[:4])), uint64(binary.BigEndian.Uint32(b[4:8])))
			}
		case "eXIf":
			if !haveExif {
				haveExif = true
				data := readUpTo(r, n, maxExifBytes)
				m.fromTIFF(data)
				n -= uint64(len(data))
			}
		case "IEND":
			return
		}
		if haveExif && haveSize {
			return
		}
		if !skip(r, n+4) { // the rest and the CRC
			return
		}
	}
}

// readWebP walks the chunks of a RIFF WEBP container, reading the size from
// VP8X (the canvas), else VP8L or VP8, and the first EXIF chunk.
func readWebP(r io.ReadSeeker, m *Meta) {
	var b [12]byte
	if !readFull(r, b[:]) || string(b[:4]) != "RIFF" || string(b[8:12]) != "WEBP" {
		return
	}
	haveExif, haveSize := false, false
	for range maxWebPChunks {
		if !readFull(r, b[:8]) {
			return
		}
		size := uint64(binary.LittleEndian.Uint32(b[4:8]))
		n := size + size&1 // chunks are padded to an even size
		switch string(b[:4]) {
		case "VP8X":
			if !haveSize && size >= 10 {
				if !readFull(r, b[:10]) {
					return
				}
				n -= 10
				haveSize = true
				m.setSize(1+uint64(le24(b[4:7])), 1+uint64(le24(b[7:10])))
			}
		case "VP8L":
			if !haveSize && size >= 5 {
				if !readFull(r, b[:5]) {
					return
				}
				n -= 5
				bits := binary.LittleEndian.Uint32(b[1:5])
				if b[0] == 0x2F && bits>>29 == 0 {
					haveSize = true
					m.setSize(1+uint64(bits&0x3FFF), 1+uint64(bits>>14&0x3FFF))
				}
			}
		case "VP8 ":
			if !haveSize && size >= 10 {
				if !readFull(r, b[:10]) {
					return
				}
				n -= 10
				if b[0]&1 == 0 && b[3] == 0x9D && b[4] == 0x01 && b[5] == 0x2A { // key frame
					haveSize = true
					m.setSize(uint64(binary.LittleEndian.Uint16(b[6:8])&0x3FFF), uint64(binary.LittleEndian.Uint16(b[8:10])&0x3FFF))
				}
			}
		case "EXIF":
			if !haveExif {
				haveExif = true
				data := readUpTo(r, size, maxExifBytes)
				m.fromTIFF(data)
				n -= uint64(len(data))
			}
		}
		if haveExif && haveSize {
			return
		}
		if !skip(r, n) {
			return
		}
	}
}

func le24(b []byte) uint32 { return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 }

// Time is the capture time as a point in time: in the recorded offset when
// it is known (exact), else the wall-clock time taken as UTC. ok is false
// when Taken is empty or invalid.
func (m Meta) Time() (t time.Time, exact bool, ok bool) {
	w, err := time.ParseInLocation("2006-01-02T15:04:05", m.Taken, time.UTC)
	if err != nil || w.Year() < 1800 || w.Year() > 2200 {
		return time.Time{}, false, false
	}
	if _, secs, ok := parseOffset(m.Offset); ok {
		y, mo, d := w.Date()
		h, mi, s := w.Clock()
		return time.Date(y, mo, d, h, mi, s, 0, time.FixedZone("", secs)), true, true
	}
	return w, false, true
}

// Upright reports the displayed width and height: swapped for orientations
// 5 to 8, which turn the picture by 90 degrees.
func (m Meta) Upright() (w, h int) {
	if m.Orientation >= 5 && m.Orientation <= 8 {
		return m.Height, m.Width
	}
	return m.Width, m.Height
}
