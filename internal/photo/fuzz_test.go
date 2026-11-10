package photo

import (
	"bytes"
	"encoding/binary"
	"testing"
)

var mediaTypes = []string{"image/jpeg", "image/png", "image/webp", "image/gif"}

// FuzzRead: no input makes Read panic, loop or invent sizes it cannot hold.
// The parsers are called without Read's recover, so a panic is found.
func FuzzRead(f *testing.F) {
	jpg := encodeJPEG(f, testImage(8, 8))
	tiffLE, tiffBE := fullTIFF(binary.LittleEndian), fullTIFF(binary.BigEndian)
	f.Add(jpegWith(jpg, exifAPP1(tiffLE)))
	f.Add(jpegWith(jpg, exifAPP1(tiffBE)))
	f.Add(pngWith(encodePNG(f, testImage(8, 8)), false, chunk("eXIf", tiffBE)))
	f.Add(webp(riffChunk("VP8X", vp8x(8, 8)), riffChunk("VP8L", vp8l(8, 8)), riffChunk("EXIF", tiffLE)))
	f.Add(webp(riffChunk("VP8 ", vp8(8, 8))))
	f.Add(encodeGIF(f, testImage(8, 8)))
	f.Add(tiffLE)
	f.Fuzz(func(t *testing.T, data []byte) {
		for _, mt := range mediaTypes {
			m := read(bytes.NewReader(data), mt)
			if m.Width < 0 || m.Height < 0 || (m.Width == 0) != (m.Height == 0) {
				t.Fatalf("%s: size %dx%d", mt, m.Width, m.Height)
			}
			if m.Orientation < 0 || m.Orientation > 8 {
				t.Fatalf("%s: orientation %d", mt, m.Orientation)
			}
			if (m.Lat == nil) != (m.Lon == nil) {
				t.Fatalf("%s: half a position", mt)
			}
			if m.Taken != "" {
				if _, _, ok := m.Time(); !ok {
					t.Fatalf("%s: taken %q is no time", mt, m.Taken)
				}
			}
		}
		// The TIFF parser on its own, as PNG and WebP hand it over.
		var m Meta
		m.fromTIFF(data)
	})
}

// FuzzThumbnail: no input makes Thumbnail panic; small limits keep it fast.
func FuzzThumbnail(f *testing.F) {
	f.Add(encodeJPEG(f, testImage(8, 8)), uint8(6))
	f.Add(encodePNG(f, testImage(8, 8)), uint8(3))
	f.Add(encodeGIF(f, testImage(8, 8)), uint8(8))
	f.Fuzz(func(t *testing.T, data []byte, o uint8) {
		for _, mt := range mediaTypes {
			out, ct, err := Thumbnail(bytes.NewReader(data), mt, 1+int(o)%16, int(o)%10, 1<<16)
			if err == nil && (len(out) == 0 || ct == "") {
				t.Fatalf("%s: empty thumbnail without error", mt)
			}
		}
	})
}
