package photo

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image/jpeg"
	"image/png"
	"testing"
)

// placeTIFF tells a time, a camera and a place in a byte order.
func placeTIFF(bo binary.ByteOrder) []byte {
	return buildTIFF(bo,
		[]tg{asciiTag(0x010F, "Maker")},
		[]tg{asciiTag(0x9003, "2024:05:01 13:22:10")},
		[]tg{asciiTag(tagGPSLatRef, "N"), ratTag(tagGPSLat, 46, 1, 33, 1, 30, 1), asciiTag(tagGPSLonRef, "E"), ratTag(tagGPSLon, 7, 1, 50, 1, 6, 1),
			rawTag(tagGPSAltRef, typeByte, []byte{0}), ratTag(tagGPSAlt, 2061, 1)})
}

const xmpPacket = `<x:xmpmeta><rdf:Description exif:GPSLatitude="46,33.5N"/></x:xmpmeta>`

// placeless reports that a file tells no place but still its time and
// camera.
func placeless(t *testing.T, data []byte, kind string) {
	t.Helper()
	m := Read(bytes.NewReader(data), kind)
	if m.Lat != nil || m.Lon != nil || m.Alt != nil {
		t.Errorf("%s still tells a place: %+v", kind, m)
	}
	if m.Taken != "2024-05-01T13:22:10" || m.Make != "Maker" {
		t.Errorf("%s lost what is no place: %+v", kind, m)
	}
	if bytes.Contains(data, []byte("GPSLatitude")) {
		t.Errorf("%s still holds its XMP", kind)
	}
}

// TestThePlaceLeavesAJPEG: the GPS directory is emptied in both byte
// orders, the XMP segments go, the picture still decodes and data is not
// changed (ADR-0177).
func TestThePlaceLeavesAJPEG(t *testing.T) {
	for _, bo := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
		in := jpegWith(encodeJPEG(t, testImage(32, 16)), exifAPP1(placeTIFF(bo)),
			segment(0xE1, append([]byte(xmpJPEG), xmpPacket...)), segment(0xE1, append([]byte(xmpExtJPEG), xmpPacket...)))
		keep := bytes.Clone(in)
		if Read(bytes.NewReader(in), "image/jpeg").Lat == nil {
			t.Fatal("the fixture tells no place")
		}
		out := StripPlace(in, "image/jpeg")
		if !bytes.Equal(in, keep) {
			t.Error("the input was changed")
		}
		placeless(t, out, "image/jpeg")
		if _, err := jpeg.Decode(bytes.NewReader(out)); err != nil {
			t.Errorf("the picture no longer decodes: %v", err)
		}
	}
}

// TestThePlaceLeavesAPNG: the eXIf chunk is emptied of its place with a
// valid checksum, the XMP chunk goes, the picture still decodes.
func TestThePlaceLeavesAPNG(t *testing.T) {
	in := pngWith(encodePNG(t, testImage(8, 8)), false, chunk("eXIf", placeTIFF(binary.BigEndian)),
		chunk("iTXt", append([]byte(xmpPNG), append([]byte{0, 0, 0, 0}, xmpPacket...)...)), chunk("tEXt", []byte("Comment\x00kept")))
	out := StripPlace(in, "image/png")
	placeless(t, out, "image/png")
	if !bytes.Contains(out, []byte("Comment\x00kept")) {
		t.Error("another chunk was dropped")
	}
	if _, err := png.Decode(bytes.NewReader(out)); err != nil {
		t.Errorf("the picture no longer decodes (checksum?): %v", err)
	}
	i := bytes.Index(out, []byte("eXIf"))
	n := int(binary.BigEndian.Uint32(out[i-4:]))
	if got := binary.BigEndian.Uint32(out[i+4+n:]); got != crc32.ChecksumIEEE(out[i:i+4+n]) {
		t.Error("the eXIf checksum is wrong")
	}
}

// TestThePlaceLeavesAWebP: the EXIF chunk is emptied of its place, the XMP
// chunk and its flag go, and the container's length is told again.
func TestThePlaceLeavesAWebP(t *testing.T) {
	head := vp8x(16, 8)
	head[0] |= 0x04 // XMP follows
	in := webp(riffChunk("VP8X", head), riffChunk("VP8L", vp8l(16, 8)), riffChunk("EXIF", placeTIFF(binary.LittleEndian)), riffChunk("XMP ", []byte(xmpPacket)))
	out := StripPlace(in, "image/webp")
	placeless(t, out, "image/webp")
	if got := binary.LittleEndian.Uint32(out[4:]); int(got) != len(out)-8 {
		t.Errorf("the container says %d bytes, holds %d", got, len(out)-8)
	}
	if out[20]&0x04 != 0 {
		t.Error("the XMP flag is still set")
	}
	if m := Read(bytes.NewReader(out), "image/webp"); m.Width != 16 || m.Height != 8 {
		t.Errorf("size %dx%d", m.Width, m.Height)
	}
}

// TestStripPlaceLeavesWhatItCannotWalk: other formats, a picture without a
// place and damaged files come back as they are, without a panic.
func TestStripPlaceLeavesWhatItCannotWalk(t *testing.T) {
	plain := encodeJPEG(t, testImage(4, 4))
	if !bytes.Equal(StripPlace(plain, "image/jpeg"), plain) {
		t.Error("a picture without metadata changed")
	}
	gif := encodeGIF(t, testImage(4, 4))
	if !bytes.Equal(StripPlace(gif, "image/gif"), gif) {
		t.Error("a GIF changed")
	}
	for _, bad := range [][]byte{nil, []byte("\xff\xd8"), []byte("\xff\xd8\xff\xe1\x00"), []byte("\x89PNG\r\n\x1a\n\x00\x00"), []byte("RIFF\x00\x00\x00\x00WEBPEXIF\xff\xff\xff\x7f"),
		append([]byte("\xff\xd8\xff\xe1\x00\x10Exif\x00\x00II*\x00"), 0xff, 0xff, 0xff, 0xff), []byte("text")} {
		for _, kind := range []string{"image/jpeg", "image/png", "image/webp"} {
			StripPlace(bad, kind)
		}
	}
	// A GPS directory pointing outside, or claiming too many entries, is left.
	tiff := buildTIFF(binary.LittleEndian, nil, nil, []tg{asciiTag(tagGPSLatRef, "N")})
	broken := bytes.Clone(tiff)
	blankGPS(broken[:9])
	blankGPS(append([]byte("MM\x00\x2a\x00\x00\x00\x08"), 0xff, 0xff))
	blankGPS([]byte("XX\x00\x2a\x00\x00\x00\x08"))
}

func FuzzStripPlace(f *testing.F) {
	f.Add(jpegWith(encodeJPEG(f, testImage(4, 4)), exifAPP1(placeTIFF(binary.LittleEndian))), "image/jpeg")
	f.Add(pngWith(encodePNG(f, testImage(4, 4)), true, chunk("eXIf", placeTIFF(binary.BigEndian))), "image/png")
	f.Add(webp(riffChunk("EXIF", placeTIFF(binary.LittleEndian))), "image/webp")
	f.Fuzz(func(t *testing.T, data []byte, kind string) {
		keep := bytes.Clone(data)
		out := StripPlace(data, kind)
		if !bytes.Equal(data, keep) {
			t.Fatal("the input was changed")
		}
		if m := Read(bytes.NewReader(out), kind); m.Lat != nil && Read(bytes.NewReader(data), kind).Lat == nil {
			t.Fatal("stripping made a place")
		}
	})
}
