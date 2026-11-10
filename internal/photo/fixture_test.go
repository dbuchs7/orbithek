package photo

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"
)

// tg is one IFD entry of a test fixture; enc gives its value bytes in a byte
// order, n its count.
type tg struct {
	id, typ uint16
	n       uint32
	enc     func(binary.ByteOrder) []byte
}

func asciiTag(id uint16, s string) tg {
	b := append([]byte(s), 0)
	return tg{id, typeASCII, uint32(len(b)), func(binary.ByteOrder) []byte { return b }}
}

// rawTag has the given type and bytes, its count the number of bytes.
func rawTag(id, typ uint16, b []byte) tg {
	return tg{id, typ, uint32(len(b)), func(binary.ByteOrder) []byte { return b }}
}

func shortTag(id uint16, vs ...uint16) tg {
	return tg{id, typeShort, uint32(len(vs)), func(bo binary.ByteOrder) []byte {
		b := make([]byte, 2*len(vs))
		for i, v := range vs {
			bo.PutUint16(b[2*i:], v)
		}
		return b
	}}
}

func longTag(id uint16, vs ...uint32) tg {
	return tg{id, typeLong, uint32(len(vs)), func(bo binary.ByteOrder) []byte {
		b := make([]byte, 4*len(vs))
		for i, v := range vs {
			bo.PutUint32(b[4*i:], v)
		}
		return b
	}}
}

// ratTag holds rationals given as numerator, denominator pairs.
func ratTag(id uint16, vs ...uint32) tg {
	t := longTag(id, vs...)
	t.typ, t.n = typeRational, uint32(len(vs)/2)
	return t
}

func sratTag(id uint16, vs ...int32) tg {
	u := make([]uint32, len(vs))
	for i, v := range vs {
		u[i] = uint32(v)
	}
	t := ratTag(id, u...)
	t.typ = typeSRational
	return t
}

// buildTIFF lays out a TIFF structure: header, IFD0 (with pointers to the
// Exif and GPS IFDs when those are not nil), the Exif IFD, the GPS IFD, then
// the values that do not fit an entry.
func buildTIFF(bo binary.ByteOrder, ifd0, exif, gps []tg) []byte {
	size := func(tags []tg) int { return 2 + 12*len(tags) + 4 }
	n0 := len(ifd0)
	if exif != nil {
		n0++
	}
	if gps != nil {
		n0++
	}
	off0 := 8
	offE := off0 + 2 + 12*n0 + 4
	offG := offE
	if exif != nil {
		offG += size(exif)
	}
	end := offG
	if gps != nil {
		end += size(gps)
	}
	all := append([]tg(nil), ifd0...)
	if exif != nil {
		all = append(all, longTag(tagExifIFD, uint32(offE)))
	}
	if gps != nil {
		all = append(all, longTag(tagGPSIFD, uint32(offG)))
	}

	buf := make([]byte, end)
	if bo == binary.ByteOrder(binary.LittleEndian) {
		copy(buf, "II")
	} else {
		copy(buf, "MM")
	}
	bo.PutUint16(buf[2:], 42)
	bo.PutUint32(buf[4:], uint32(off0))
	write := func(at int, tags []tg) {
		bo.PutUint16(buf[at:], uint16(len(tags)))
		for i, t := range tags {
			e := buf[at+2+12*i:]
			bo.PutUint16(e, t.id)
			bo.PutUint16(e[2:], t.typ)
			bo.PutUint32(e[4:], t.n)
			v := t.enc(bo)
			if len(v) <= 4 {
				copy(e[8:12], v)
				continue
			}
			bo.PutUint32(e[8:], uint32(len(buf)))
			buf = append(buf, v...)
			if len(buf)%2 == 1 {
				buf = append(buf, 0)
			}
		}
	}
	write(off0, all)
	if exif != nil {
		write(offE, exif)
	}
	if gps != nil {
		write(offG, gps)
	}
	return buf
}

// testImage is a small opaque picture with a gradient.
func testImage(w, h int) *image.RGBA {
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			m.Set(x, y, color.RGBA{uint8(x * 255 / max(1, w-1)), uint8(y * 255 / max(1, h-1)), 128, 255})
		}
	}
	return m
}

func encodeJPEG(t testing.TB, m image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, m, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func encodePNG(t testing.TB, m image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, m); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func encodeGIF(t testing.TB, m image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := gif.Encode(&b, m, nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// segment makes a JPEG marker segment.
func segment(code byte, payload []byte) []byte {
	b := []byte{0xFF, code, 0, 0}
	binary.BigEndian.PutUint16(b[2:], uint16(len(payload)+2))
	return append(b, payload...)
}

// jpegWith puts segments right after the SOI of a JPEG.
func jpegWith(jpg []byte, segs ...[]byte) []byte {
	out := append([]byte(nil), jpg[:2]...)
	for _, s := range segs {
		out = append(out, s...)
	}
	return append(out, jpg[2:]...)
}

// exifAPP1 is an APP1 segment holding a TIFF structure.
func exifAPP1(tiff []byte) []byte {
	return segment(0xE1, append([]byte("Exif\x00\x00"), tiff...))
}

// chunk makes a PNG chunk with its CRC.
func chunk(typ string, data []byte) []byte {
	b := binary.BigEndian.AppendUint32(nil, uint32(len(data)))
	b = append(b, typ...)
	b = append(b, data...)
	return binary.BigEndian.AppendUint32(b, crc32.ChecksumIEEE(b[4:]))
}

// pngWith puts chunks after IHDR, or before IEND when late is set.
func pngWith(p []byte, late bool, chunks ...[]byte) []byte {
	at := 8 + 25 // signature and IHDR
	if late {
		at = len(p) - 12
	}
	out := append([]byte(nil), p[:at]...)
	for _, c := range chunks {
		out = append(out, c...)
	}
	return append(out, p[at:]...)
}

// riffChunk makes a WebP chunk, padded to an even size.
func riffChunk(fourcc string, data []byte) []byte {
	b := append([]byte(fourcc), binary.LittleEndian.AppendUint32(nil, uint32(len(data)))...)
	b = append(b, data...)
	if len(data)%2 == 1 {
		b = append(b, 0)
	}
	return b
}

// webp makes a RIFF WEBP container of chunks.
func webp(chunks ...[]byte) []byte {
	var body []byte
	for _, c := range chunks {
		body = append(body, c...)
	}
	b := append([]byte("RIFF"), binary.LittleEndian.AppendUint32(nil, uint32(4+len(body)))...)
	b = append(b, "WEBP"...)
	return append(b, body...)
}

// vp8x is the extended header of a canvas w×h with the EXIF flag.
func vp8x(w, h int) []byte {
	b := []byte{0x08, 0, 0, 0}
	b = append(b, byte(w-1), byte((w-1)>>8), byte((w-1)>>16))
	return append(b, byte(h-1), byte((h-1)>>8), byte((h-1)>>16))
}

// vp8l is the start of a lossless bitstream of w×h.
func vp8l(w, h int) []byte {
	bits := uint32(w-1) | uint32(h-1)<<14
	return append([]byte{0x2F}, binary.LittleEndian.AppendUint32(nil, bits)...)
}

// vp8 is the start of a lossy key frame of w×h.
func vp8(w, h int) []byte {
	b := []byte{0x10, 0x02, 0x00, 0x9D, 0x01, 0x2A}
	b = binary.LittleEndian.AppendUint16(b, uint16(w))
	return binary.LittleEndian.AppendUint16(b, uint16(h))
}
