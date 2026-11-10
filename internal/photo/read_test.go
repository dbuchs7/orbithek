package photo

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"
	"time"
)

func ptr(v float64) *float64 { return &v }

// fullTags is a camera's complete set of tags; fullMeta is what it tells.
func fullTags() (ifd0, exif, gps []tg) {
	ifd0 = []tg{
		asciiTag(tagMake, "Canon\x00\x00  "),
		asciiTag(tagModel, "  Canon EOS   R5 "),
		shortTag(tagOrientation, 6),
		asciiTag(tagDateTime, "2024:01:01 00:00:00"),
	}
	exif = []tg{
		ratTag(tagExposure, 1, 250),
		ratTag(tagFNumber, 28, 10),
		shortTag(tagISO, 400),
		asciiTag(tagOriginal, "2023:07:14 18:30:05"),
		asciiTag(tagDigitized, "2023:07:14 18:30:06"),
		asciiTag(tagOffsetOrig, "+02:00"),
		asciiTag(tagOffsetDigit, "+03:00"),
		asciiTag(tagOffset, "+04:00"),
		ratTag(tagFocal, 1053, 10),
		shortTag(tagFocal35, 105),
		asciiTag(tagLensModel, "RF24-105mm F4 L IS USM"),
	}
	gps = []tg{
		asciiTag(tagGPSLatRef, "N"),
		ratTag(tagGPSLat, 47, 1, 22, 1, 3600, 100),
		asciiTag(tagGPSLonRef, "W"),
		ratTag(tagGPSLon, 8, 1, 32, 1, 24, 1),
		rawTag(tagGPSAltRef, typeByte, []byte{1}),
		ratTag(tagGPSAlt, 4085, 10),
	}
	return
}

var fullMeta = Meta{
	Taken:       "2023-07-14T18:30:05",
	Offset:      "+02:00",
	Lat:         ptr(47.3766667),
	Lon:         ptr(-8.54),
	Alt:         ptr(-408.5),
	Make:        "Canon",
	Model:       "Canon EOS R5",
	Lens:        "RF24-105mm F4 L IS USM",
	Exposure:    "1/250",
	FNumber:     2.8,
	ISO:         400,
	Focal:       105.3,
	Focal35:     105,
	Orientation: 6,
}

var orders = []struct {
	name string
	bo   binary.ByteOrder
}{{"II", binary.LittleEndian}, {"MM", binary.BigEndian}}

func fullTIFF(bo binary.ByteOrder) []byte {
	ifd0, exif, gps := fullTags()
	return buildTIFF(bo, ifd0, exif, gps)
}

// sized is fullMeta with a pixel size.
func sized(w, h int) Meta {
	m := fullMeta
	m.Width, m.Height = w, h
	return m
}

func check(t *testing.T, got, want Meta) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		g, _ := json.Marshal(got)
		w, _ := json.Marshal(want)
		t.Errorf("got  %s\nwant %s", g, w)
	}
}

// TestReadJPEG: every field of an Exif APP1 segment is read in both byte
// orders, and the size comes from the frame header.
func TestReadJPEG(t *testing.T) {
	jpg := encodeJPEG(t, testImage(16, 8))
	for _, o := range orders {
		t.Run(o.name, func(t *testing.T) {
			f := jpegWith(jpg, exifAPP1(fullTIFF(o.bo)))
			check(t, Read(bytes.NewReader(f), "image/jpeg"), sized(16, 8))
		})
	}
}

// TestReadJPEGMarkers: other APP1 segments (XMP), fill bytes and markers
// without a length are walked past; the walk stops at SOS, at EOI and at
// anything that is not a marker.
func TestReadJPEGMarkers(t *testing.T) {
	jpg := encodeJPEG(t, testImage(16, 8))
	xmp := segment(0xE1, []byte("http://ns.adobe.com/xap/1.0/\x00<x/>"))
	app0 := segment(0xE0, []byte("JFIF\x00\x01\x02"))
	f := jpegWith(jpg, []byte{0xFF, 0x01}, app0, xmp, []byte{0xFF, 0xFF, 0xFF}, exifAPP1(fullTIFF(binary.BigEndian)))
	check(t, Read(bytes.NewReader(f), "image/jpeg"), sized(16, 8))

	// Without EXIF only the size is read, the walk reaching SOS.
	check(t, Read(bytes.NewReader(jpg), "image/jpeg"), Meta{Width: 16, Height: 8})

	// A second Exif segment does not override the first.
	other := buildTIFF(binary.LittleEndian, []tg{asciiTag(tagMake, "Other")}, nil, nil)
	got := Read(bytes.NewReader(jpegWith(jpg, exifAPP1(fullTIFF(binary.LittleEndian)), exifAPP1(other))), "image/jpeg")
	if got.Make != "Canon" {
		t.Errorf("make = %q, want the first segment's", got.Make)
	}

	for name, f := range map[string][]byte{
		"EOI first":       {0xFF, 0xD8, 0xFF, 0xD9},
		"SOS first":       {0xFF, 0xD8, 0xFF, 0xDA, 0, 2},
		"no marker":       {0xFF, 0xD8, 0x12, 0x34},
		"short length":    {0xFF, 0xD8, 0xFF, 0xE0, 0, 1, 0, 0},
		"marker 00":       {0xFF, 0xD8, 0xFF, 0x00},
		"no SOI":          {0xFF, 0xD9},
		"cut in length":   {0xFF, 0xD8, 0xFF, 0xE0, 0},
		"cut in marker":   {0xFF, 0xD8, 0xFF},
		"cut after SOI":   {0xFF, 0xD8},
		"cut in exif":     {0xFF, 0xD8, 0xFF, 0xE1, 0, 20, 'E', 'x'},
		"cut in SOF":      {0xFF, 0xD8, 0xFF, 0xC0, 0, 17, 8, 0},
		"endless fill":    append([]byte{0xFF, 0xD8}, bytes.Repeat([]byte{0xFF}, maxFill+10)...),
		"many segments":   append([]byte{0xFF, 0xD8}, bytes.Repeat([]byte{0xFF, 0xFE, 0, 2}, maxSegments+10)...),
		"truncated exif":  append([]byte{0xFF, 0xD8}, exifAPP1(fullTIFF(binary.LittleEndian))[:60]...),
		"SOF with size 0": {0xFF, 0xD8, 0xFF, 0xC0, 0, 17, 8, 0, 0, 0, 0},
	} {
		t.Run(name, func(t *testing.T) {
			m := Read(bytes.NewReader(f), "image/jpeg")
			if m.Width != 0 || m.Height != 0 || m.Taken != "" {
				t.Errorf("got %+v, want nothing", m)
			}
		})
	}
}

// TestReadJPEGTruncatedExif: a segment cut by the end of the file still
// gives what lies in the part that is there.
func TestReadJPEGTruncatedExif(t *testing.T) {
	tiff := buildTIFF(binary.LittleEndian, []tg{shortTag(tagOrientation, 3)}, nil, nil)
	seg := exifAPP1(append(tiff, make([]byte, 100)...))
	f := append([]byte{0xFF, 0xD8}, seg[:len(seg)-50]...)
	if m := Read(bytes.NewReader(f), "image/jpeg"); m.Orientation != 3 {
		t.Errorf("orientation = %d, want 3", m.Orientation)
	}
}

// TestReadPNG: an eXIf chunk with or without the "Exif\0\0" prefix, before
// or after the image data, in both byte orders; the size comes from IHDR.
func TestReadPNG(t *testing.T) {
	p := encodePNG(t, testImage(12, 5))
	for _, o := range orders {
		for _, prefix := range []string{"", "Exif\x00\x00"} {
			for _, late := range []bool{false, true} {
				f := pngWith(p, late, chunk("tEXt", []byte("a\x00b")), chunk("eXIf", append([]byte(prefix), fullTIFF(o.bo)...)))
				check(t, Read(bytes.NewReader(f), "image/png"), sized(12, 5))
			}
		}
	}
	check(t, Read(bytes.NewReader(p), "image/png"), Meta{Width: 12, Height: 5})

	// IHDR not first gives no size; a chunk longer than PNG allows ends the walk.
	bad := append([]byte("\x89PNG\r\n\x1a\n"), chunk("tEXt", nil)...)
	bad = append(bad, p[8:]...)
	check(t, Read(bytes.NewReader(bad), "image/png"), Meta{})
	huge := append([]byte("\x89PNG\r\n\x1a\n"), 0x80, 0, 0, 0, 'I', 'D', 'A', 'T')
	check(t, Read(bytes.NewReader(huge), "image/png"), Meta{})
	check(t, Read(bytes.NewReader(p[:20]), "image/png"), Meta{})
	check(t, Read(bytes.NewReader([]byte("GIF89a")), "image/png"), Meta{})
}

// TestReadWebP: the canvas size of VP8X, the sizes of VP8L and VP8, an EXIF
// chunk after the image data, with or without prefix, odd chunks padded.
func TestReadWebP(t *testing.T) {
	for _, o := range orders {
		tiff := fullTIFF(o.bo)
		f := webp(riffChunk("VP8X", vp8x(3000, 2000)), riffChunk("ICCP", []byte{1, 2, 3}), riffChunk("VP8L", vp8l(10, 10)), riffChunk("EXIF", tiff))
		check(t, Read(bytes.NewReader(f), "image/webp"), sized(3000, 2000))
		f = webp(riffChunk("VP8X", vp8x(1, 16777216)), riffChunk("EXIF", append([]byte("Exif\x00\x00"), tiff...)))
		check(t, Read(bytes.NewReader(f), "image/webp"), sized(1, 16777216))
	}
	check(t, Read(bytes.NewReader(webp(riffChunk("VP8L", vp8l(16383, 1)))), "image/webp"), Meta{Width: 16383, Height: 1})
	check(t, Read(bytes.NewReader(webp(riffChunk("VP8 ", vp8(640, 480)))), "image/webp"), Meta{Width: 640, Height: 480})

	interframe := vp8(640, 480)
	interframe[0] |= 1
	badL := vp8l(5, 5)
	badL[0] = 0
	for name, f := range map[string][]byte{
		"inter frame":   webp(riffChunk("VP8 ", interframe)),
		"bad signature": webp(riffChunk("VP8L", badL)),
		"short VP8X":    webp(riffChunk("VP8X", []byte{0, 0, 0})),
		"not RIFF":      []byte("RIFX\x00\x00\x00\x00WEBP"),
		"not WEBP":      []byte("RIFF\x00\x00\x00\x00WAVE"),
		"cut VP8X":      webp(riffChunk("VP8X", vp8x(5, 5)))[:24],
		"cut VP8L":      webp(riffChunk("VP8L", vp8l(5, 5)))[:24],
		"cut VP8":       webp(riffChunk("VP8 ", vp8(5, 5)))[:24],
		"huge chunk":    webp([]byte("JUNK\xff\xff\xff\xff")),
	} {
		t.Run(name, func(t *testing.T) { check(t, Read(bytes.NewReader(f), "image/webp"), Meta{}) })
	}
}

// TestReadGIF: a GIF tells its size and nothing else.
func TestReadGIF(t *testing.T) {
	check(t, Read(bytes.NewReader(encodeGIF(t, testImage(7, 9))), "image/gif"), Meta{Width: 7, Height: 9})
	check(t, Read(bytes.NewReader([]byte("GIF89a")), "image/gif"), Meta{})
}

// TestReadMediaTypes: parameters and case are ignored, aliases of JPEG are
// known, and other types give nothing.
func TestReadMediaTypes(t *testing.T) {
	jpg := encodeJPEG(t, testImage(4, 4))
	for _, mt := range []string{"image/JPEG; foo=bar", "image/jpg", " image/pjpeg "} {
		if m := Read(bytes.NewReader(jpg), mt); m.Width != 4 {
			t.Errorf("%q: width %d", mt, m.Width)
		}
	}
	check(t, Read(bytes.NewReader(jpg), "image/bmp"), Meta{})
	check(t, Read(nil, "image/jpeg"), Meta{})
}

type failingSeeker struct{ io.Reader }

func (failingSeeker) Seek(int64, int) (int64, error) { return 0, errors.New("no seeking") }

type panickingReader struct{}

func (panickingReader) Read([]byte) (int, error)       { panic("boom") }
func (panickingReader) Seek(int64, int) (int64, error) { return 0, nil }

// seekOnce allows the seek to the start and refuses every later one.
type seekOnce struct {
	*bytes.Reader
	n int
}

func (s *seekOnce) Seek(off int64, whence int) (int64, error) {
	if s.n++; s.n > 1 {
		return 0, errors.New("no more seeking")
	}
	return s.Reader.Seek(off, whence)
}

// TestReadReaderFailures: a reader that cannot seek or panics gives nothing;
// Read starts at the beginning whatever the reader's position.
func TestReadReaderFailures(t *testing.T) {
	jpg := jpegWith(encodeJPEG(t, testImage(4, 4)), segment(0xFE, []byte("comment")))
	check(t, Read(failingSeeker{bytes.NewReader(jpg)}, "image/jpeg"), Meta{})
	check(t, Read(panickingReader{}, "image/jpeg"), Meta{})
	for _, mt := range []string{"image/jpeg", "image/png", "image/webp"} {
		var f []byte
		switch mt {
		case "image/jpeg":
			f = jpg
		case "image/png":
			f = pngWith(encodePNG(t, testImage(2, 2)), false, chunk("tEXt", []byte("x")))
		case "image/webp":
			f = webp(riffChunk("ICCP", []byte{1}), riffChunk("VP8L", vp8l(2, 2)))
		}
		// PNG has read its size from IHDR before the first skip.
		if m := Read(&seekOnce{Reader: bytes.NewReader(f)}, mt); m.Width != 0 && mt != "image/png" {
			t.Errorf("%s: read past a failed seek: %+v", mt, m)
		}
	}
	r := bytes.NewReader(encodeJPEG(t, testImage(4, 4)))
	r.Seek(10, io.SeekStart)
	if m := Read(r, "image/jpeg"); m.Width != 4 {
		t.Errorf("width = %d, want 4 from the start of the file", m.Width)
	}
}

func metaOf(ifd0, exif, gps []tg) Meta {
	var m Meta
	m.fromTIFF(buildTIFF(binary.LittleEndian, ifd0, exif, gps))
	return m
}

// TestTakenFallbacks: DateTimeOriginal, else DateTimeDigitized, else
// DateTime, each with its own offset; invalid times are passed over.
func TestTakenFallbacks(t *testing.T) {
	offsets := []tg{asciiTag(tagOffsetOrig, "+01:00"), asciiTag(tagOffsetDigit, "-05:30"), asciiTag(tagOffset, "-00:00")}
	cases := []struct {
		name         string
		ifd0, exif   []tg
		taken, ofset string
	}{
		{"original", []tg{asciiTag(tagDateTime, "2001:01:01 01:01:01")},
			[]tg{asciiTag(tagOriginal, "2003:03:03 03:03:03"), asciiTag(tagDigitized, "2002:02:02 02:02:02")}, "2003-03-03T03:03:03", "+01:00"},
		{"digitized", []tg{asciiTag(tagDateTime, "2001:01:01 01:01:01")},
			[]tg{asciiTag(tagOriginal, "2003:02:29 03:03:03"), asciiTag(tagDigitized, "2002:02:02 02:02:02")}, "2002-02-02T02:02:02", "-05:30"},
		{"datetime", []tg{asciiTag(tagDateTime, "2001-01-01T01:01:01.123+02:00")},
			[]tg{asciiTag(tagOriginal, "    :  :     :  :  "), asciiTag(tagDigitized, "0000:00:00 00:00:00")}, "2001-01-01T01:01:01", "+00:00"},
		{"none valid", []tg{asciiTag(tagDateTime, "1799:12:31 23:59:59")},
			[]tg{asciiTag(tagOriginal, "2201:01:01 00:00:00")}, "", ""},
		{"offset in IFD0", []tg{asciiTag(tagDateTime, "2001:01:01 01:01:01"), asciiTag(tagOffset, "+0930")}, nil, "2001-01-01T01:01:01", "+09:30"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			exif := c.exif
			if c.name != "offset in IFD0" {
				exif = append(append([]tg(nil), c.exif...), offsets...)
			}
			m := metaOf(c.ifd0, exif, nil)
			if m.Taken != c.taken || m.Offset != c.ofset {
				t.Errorf("got %q %q, want %q %q", m.Taken, m.Offset, c.taken, c.ofset)
			}
		})
	}
	// An invalid offset is unknown, the time still taken.
	m := metaOf(nil, []tg{asciiTag(tagOriginal, "2003:03:03 03:03:03"), asciiTag(tagOffsetOrig, "+25:00")}, nil)
	if m.Taken != "2003-03-03T03:03:03" || m.Offset != "" {
		t.Errorf("got %q %q", m.Taken, m.Offset)
	}
}

func TestParseExifTime(t *testing.T) {
	for in, want := range map[string]string{
		"2023:07:14 18:30:05":       "2023-07-14T18:30:05",
		" 2023/07/14 18:30:05 ":     "2023-07-14T18:30:05",
		"2024:02:29 00:00:00":       "2024-02-29T00:00:00",
		"2000:02:29 23:59:59":       "2000-02-29T23:59:59",
		"1800:01:01 00:00:00":       "1800-01-01T00:00:00",
		"2200:12:31 23:59:59":       "2200-12-31T23:59:59",
		"1900:02:29 00:00:00":       "",
		"2023:02:29 00:00:00":       "",
		"2023:04:31 00:00:00":       "",
		"2023:13:01 00:00:00":       "",
		"2023:00:01 00:00:00":       "",
		"2023:01:00 00:00:00":       "",
		"2023:01:01 24:00:00":       "",
		"2023:01:01 23:60:00":       "",
		"2023:01:01 23:59:60":       "",
		"2023:01:01 23:59":          "",
		"2023:01:01_23:59:59":       "",
		"2023:01:01 23-59-59":       "",
		"2023.01.01 23:59:59":       "",
		"2O23:01:01 23:59:59":       "",
		"":                          "",
		"2023:01:01 23:59:59 extra": "2023-01-01T23:59:59",
	} {
		got, ok := parseExifTime([]byte(in))
		if got != want || ok != (want != "") {
			t.Errorf("%q: got %q %v, want %q", in, got, ok, want)
		}
	}
}

func TestParseOffset(t *testing.T) {
	for in, want := range map[string]struct {
		s    string
		secs int
	}{
		"+02:00":   {"+02:00", 7200},
		"-05:30":   {"-05:30", -19800},
		"+0545":    {"+05:45", 20700},
		"-00:00":   {"+00:00", 0},
		" +14:00 ": {"+14:00", 50400},
		"+15:00":   {},
		"+02:60":   {},
		"02:00":    {},
		"+2:00":    {},
		"+02-00":   {},
		"+0a:00":   {},
		"Z":        {},
		"":         {},
	} {
		s, secs, ok := parseOffset(in)
		if s != want.s || secs != want.secs || ok != (want.s != "") {
			t.Errorf("%q: got %q %d %v, want %+v", in, s, secs, ok, want)
		}
	}
}

// TestGPS: hemispheres give the sign; altitude below sea level is negative;
// zero denominators, negative or out-of-range values, a missing or unknown
// reference and (0, 0) give no position.
func TestGPS(t *testing.T) {
	gps := func(latRef string, lat []uint32, lonRef string, lon []uint32, extra ...tg) []tg {
		g := []tg{asciiTag(tagGPSLatRef, latRef), ratTag(tagGPSLat, lat...), asciiTag(tagGPSLonRef, lonRef), ratTag(tagGPSLon, lon...)}
		return append(g, extra...)
	}
	dms := []uint32{33, 1, 51, 1, 54, 1} // 33.865
	cases := []struct {
		name     string
		tags     []tg
		lat, lon *float64
		alt      *float64
	}{
		{"south east", gps("S", dms, "E", []uint32{151, 1, 12, 1, 36, 1}), ptr(-33.865), ptr(151.21), nil},
		{"lower case", gps("s", dms, "w", dms), ptr(-33.865), ptr(-33.865), nil},
		{"degrees only", gps("N", []uint32{4512345, 100000}, "E", []uint32{712, 100}), ptr(45.12345), ptr(7.12), nil},
		{"degrees and minutes", gps("N", []uint32{45, 1, 3015, 100}, "E", []uint32{7, 1}), ptr(45.5025), ptr(7), nil},
		{"zero denominator", gps("N", []uint32{45, 0, 0, 1, 0, 1}, "E", dms), nil, nil, nil},
		{"out of range", gps("N", []uint32{91, 1}, "E", dms), nil, nil, nil},
		{"lon out of range", gps("N", dms, "E", []uint32{180, 1, 1, 1}), nil, nil, nil},
		{"null island", gps("N", []uint32{0, 1, 0, 1, 0, 1}, "E", []uint32{0, 1}), nil, nil, nil},
		{"no reference", gps("", dms, "E", dms), nil, nil, nil},
		{"bad reference", gps("X", dms, "E", dms), nil, nil, nil},
		{"lat only", []tg{asciiTag(tagGPSLatRef, "N"), ratTag(tagGPSLat, dms...)}, nil, nil, nil},
		{"empty value", gps("N", nil, "E", dms), nil, nil, nil},
		{"negative", []tg{asciiTag(tagGPSLatRef, "N"), sratTag(tagGPSLat, -10, 1), asciiTag(tagGPSLonRef, "E"), ratTag(tagGPSLon, dms...)}, nil, nil, nil},
		{"signed positive", []tg{asciiTag(tagGPSLatRef, "N"), sratTag(tagGPSLat, 10, 1, 30, 1), asciiTag(tagGPSLonRef, "E"), ratTag(tagGPSLon, dms...)}, ptr(10.5), ptr(33.865), nil},
		{"wrong type", []tg{asciiTag(tagGPSLatRef, "N"), shortTag(tagGPSLat, 10), asciiTag(tagGPSLonRef, "E"), ratTag(tagGPSLon, dms...)}, nil, nil, nil},
		{"altitude above", []tg{rawTag(tagGPSAltRef, typeByte, []byte{0}), ratTag(tagGPSAlt, 12345, 100)}, nil, nil, ptr(123.45)},
		{"altitude without ref", []tg{ratTag(tagGPSAlt, 8848, 1)}, nil, nil, ptr(8848)},
		{"altitude zero denominator", []tg{ratTag(tagGPSAlt, 1, 0)}, nil, nil, nil},
		{"altitude out of range", []tg{ratTag(tagGPSAlt, 200000, 1)}, nil, nil, nil},
		{"altitude too deep", []tg{rawTag(tagGPSAltRef, typeByte, []byte{1}), ratTag(tagGPSAlt, 20000, 1)}, nil, nil, nil},
		{"signed overflow", []tg{sratTag(tagGPSAlt, -2147483648, -1)}, nil, nil, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := metaOf(nil, nil, c.tags)
			if !reflect.DeepEqual(m.Lat, c.lat) || !reflect.DeepEqual(m.Lon, c.lon) || !reflect.DeepEqual(m.Alt, c.alt) {
				t.Errorf("got %v %v %v, want %v %v %v", deref(m.Lat), deref(m.Lon), deref(m.Alt), deref(c.lat), deref(c.lon), deref(c.alt))
			}
		})
	}
}

func deref(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func TestFormatExposure(t *testing.T) {
	for v, want := range map[float64]string{
		1.0 / 250:  "1/250",
		0.0041:     "1/244",
		1.0 / 3:    "1/3",
		0.5:        "1/2",
		0.8:        "0.8",
		0.7:        "0.7", // 1/1.43 would round to 1/1, which no camera shows
		1:          "1",
		2.5:        "2.5",
		30:         "30",
		1.0 / 8000: "1/8000",
		0:          "",
		-1:         "",
		1e7:        "",
	} {
		if got := formatExposure(v); got != want {
			t.Errorf("%v: got %q, want %q", v, got, want)
		}
	}
	m := metaOf(nil, []tg{ratTag(tagExposure, 10, 2500)}, nil)
	if m.Exposure != "1/250" {
		t.Errorf("exposure = %q", m.Exposure)
	}
}

// TestCameraFields: ISO from ISOSpeed when the short field overflows, values
// out of range and wrong types are dropped, orientation only 1..8.
func TestCameraFields(t *testing.T) {
	m := metaOf([]tg{shortTag(tagOrientation, 9)}, []tg{shortTag(tagISO, 65535), longTag(tagISOSpeed, 102400), ratTag(tagFNumber, 0, 1), ratTag(tagFocal, 1, 0), shortTag(tagFocal35, 0)}, nil)
	if m.ISO != 102400 || m.Orientation != 0 || m.FNumber != 0 || m.Focal != 0 || m.Focal35 != 0 {
		t.Errorf("got %+v", m)
	}
	m = metaOf([]tg{asciiTag(tagOrientation, "6")}, []tg{longTag(tagISO, 20000000), rawTag(tagLensModel, typeUndefined, []byte("Lens\x00junk"))}, nil)
	if m.ISO != 0 || m.Orientation != 0 || m.Lens != "Lens" {
		t.Errorf("got %+v", m)
	}
	m = metaOf([]tg{rawTag(tagOrientation, typeByte, []byte{8}), rawTag(tagMake, typeShort, []byte("ab"))}, []tg{rawTag(tagISO, typeByte, []byte{100})}, nil)
	if m.ISO != 100 || m.Orientation != 8 || m.Make != "" {
		t.Errorf("got %+v", m)
	}
}

func TestClean(t *testing.T) {
	long := strings.Repeat("ab", 40)
	for in, want := range map[string]string{
		"Canon\x00\x00":                "Canon",
		"\x00\x00NIKON CORPORATION":    "NIKON CORPORATION",
		"  Apple \t\n iPhone  15 ":     "Apple iPhone 15",
		"Bell\x07\x1b[31mred":          "Bell[31mred",
		"Zei\xdf":                      "Zeiß", // Latin-1
		"Zeiß":                         "Zeiß",
		long:                           long[:64],
		strings.Repeat("a", 63) + " b": strings.Repeat("a", 63),
		strings.Repeat("é", 70):        strings.Repeat("é", 64),
		" x y":                         "x y",
		"":                             "",
		"\x01\x02":                     "",
	} {
		if got := clean([]byte(in)); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}

// rawTIFF builds a TIFF by hand: little-endian header pointing at ifdOff,
// then the bytes given.
func rawTIFF(ifdOff uint32, rest ...[]byte) []byte {
	b := []byte{'I', 'I', 42, 0}
	b = binary.LittleEndian.AppendUint32(b, ifdOff)
	for _, r := range rest {
		b = append(b, r...)
	}
	return b
}

// rawEntry is one 12-byte IFD entry, little-endian.
func rawEntry(tag, typ uint16, count, value uint32) []byte {
	b := binary.LittleEndian.AppendUint16(nil, tag)
	b = binary.LittleEndian.AppendUint16(b, typ)
	b = binary.LittleEndian.AppendUint32(b, count)
	return binary.LittleEndian.AppendUint32(b, value)
}

func u16(v uint16) []byte { return binary.LittleEndian.AppendUint16(nil, v) }
func u32(v uint32) []byte { return binary.LittleEndian.AppendUint32(nil, v) }

// TestHostileTIFF: loops, offsets outside the data, huge counts, too many
// entries and broken headers end the parse without harm.
func TestHostileTIFF(t *testing.T) {
	orientation6 := rawEntry(tagOrientation, typeShort, 1, 6)
	cases := []struct {
		name string
		tiff []byte
		want int // orientation
	}{
		{"pointers to itself", rawTIFF(8, u16(3), orientation6, rawEntry(tagExifIFD, typeLong, 1, 8), rawEntry(tagGPSIFD, typeLong, 1, 8), u32(8)), 6},
		{"exif and gps share an IFD", rawTIFF(8, u16(3), rawEntry(tagExifIFD, typeLong, 1, 50), rawEntry(tagGPSIFD, typeLong, 1, 50), orientation6, u32(0), u16(1), rawEntry(tagISO, typeShort, 1, 100), u32(0)), 6},
		{"pointer outside", rawTIFF(8, u16(2), orientation6, rawEntry(tagExifIFD, typeLong, 1, 0xFFFFFFF0), u32(0)), 6},
		{"pointer of type IFD", rawTIFF(8, u16(2), orientation6, rawEntry(tagExifIFD, typeIFD, 1, 7), u32(0)), 6},
		{"too many entries", rawTIFF(8, u16(maxEntries+1), orientation6), 0},
		{"count beyond the data", rawTIFF(8, u16(500), orientation6), 6},
		{"huge value count", rawTIFF(8, u16(2), rawEntry(tagMake, typeRational, 0xFFFFFFFF, 8), orientation6), 6},
		{"value outside", rawTIFF(8, u16(2), rawEntry(tagMake, typeASCII, 100, 0xFFFFFF00), orientation6), 6},
		{"unknown type", rawTIFF(8, u16(2), rawEntry(tagOrientation, 99, 1, 3), orientation6), 6},
		{"duplicate tag", rawTIFF(8, u16(2), orientation6, rawEntry(tagOrientation, typeShort, 1, 3)), 6},
		{"IFD0 in the header", rawTIFF(4, u16(1), orientation6), 0},
		{"IFD0 outside", rawTIFF(1000, u16(1), orientation6), 0},
		{"IFD0 at the end", rawTIFF(8, []byte{1}), 0},
		{"bad magic", append([]byte("II\x2b\x00"), rawTIFF(8, u16(1), orientation6)[4:]...), 0},
		{"bad order", append([]byte("XX\x2a\x00"), rawTIFF(8, u16(1), orientation6)[4:]...), 0},
		{"short", []byte("II*\x00"), 0},
		{"empty", nil, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var m Meta
			m.fromTIFF(c.tiff)
			if m.Orientation != c.want {
				t.Errorf("orientation = %d, want %d", m.Orientation, c.want)
			}
			// The same structure inside every container.
			Read(bytes.NewReader(jpegWith([]byte{0xFF, 0xD8, 0xFF, 0xD9}, exifAPP1(c.tiff))), "image/jpeg")
		})
	}
	// The shared IFD is read once, as the Exif IFD.
	var m Meta
	m.fromTIFF(cases[1].tiff)
	if m.ISO != 100 {
		t.Errorf("iso = %d, want 100 from the shared IFD", m.ISO)
	}
	// A rational index beyond the count is no value.
	d := ifd{bo: binary.LittleEndian, m: map[uint16]entry{1: {typ: typeRational, n: 1, val: make([]byte, 8)}}}
	if _, ok := d.rat(1, 1); ok {
		t.Error("rat beyond the count")
	}
	if _, ok := d.rat(1, -1); ok {
		t.Error("rat at a negative index")
	}
}

// TestTruncationsAndGarbage: every prefix of a valid file and random bytes
// are read without panicking, and a prefix never invents a field.
func TestTruncationsAndGarbage(t *testing.T) {
	tiff := fullTIFF(binary.BigEndian)
	files := map[string][]byte{
		"image/jpeg": jpegWith(encodeJPEG(t, testImage(8, 8)), exifAPP1(tiff)),
		"image/png":  pngWith(encodePNG(t, testImage(8, 8)), true, chunk("eXIf", tiff)),
		"image/webp": webp(riffChunk("VP8X", vp8x(8, 8)), riffChunk("VP8L", vp8l(8, 8)), riffChunk("EXIF", tiff)),
		"image/gif":  encodeGIF(t, testImage(8, 8)),
	}
	for mt, f := range files {
		full := Read(bytes.NewReader(f), mt)
		for n := range len(f) {
			m := Read(bytes.NewReader(f[:n]), mt)
			// A cut value falls back to the next time, never to anything else.
			taken := map[string]bool{"": true, full.Taken: true, "2023-07-14T18:30:06": true, "2024-01-01T00:00:00": true}
			if m.Make != "" && m.Make != full.Make || !taken[m.Taken] || m.Width != 0 && m.Width != full.Width {
				t.Fatalf("%s cut at %d invents %+v", mt, n, m)
			}
		}
	}
	rng := rand.New(rand.NewPCG(1, 2))
	for range 2000 {
		b := make([]byte, rng.IntN(512))
		for i := range b {
			b[i] = byte(rng.UintN(256))
		}
		for mt, f := range files {
			Read(bytes.NewReader(b), mt)
			// A valid start followed by garbage.
			k := min(len(f), rng.IntN(64))
			Read(bytes.NewReader(append(append([]byte(nil), f[:k]...), b...)), mt)
		}
	}
}

// TestTime: with an offset the time is exact; without, it is the wall
// clock taken as UTC; an empty or invalid Taken is no time.
func TestTime(t *testing.T) {
	tm, exact, ok := Meta{Taken: "2023-07-14T18:30:05", Offset: "+02:00"}.Time()
	if !ok || !exact || !tm.Equal(time.Date(2023, 7, 14, 16, 30, 5, 0, time.UTC)) {
		t.Errorf("got %v %v %v", tm, exact, ok)
	}
	if _, off := tm.Zone(); off != 7200 {
		t.Errorf("zone offset = %d", off)
	}
	tm, exact, ok = Meta{Taken: "2023-07-14T18:30:05", Offset: "bogus"}.Time()
	if !ok || exact || !tm.Equal(time.Date(2023, 7, 14, 18, 30, 5, 0, time.UTC)) || tm.Location() != time.UTC {
		t.Errorf("got %v %v %v", tm, exact, ok)
	}
	for _, s := range []string{"", "2023-02-30T00:00:00", "1799-12-31T23:59:59", "2023:07:14 18:30:05", "2201-01-01T00:00:00"} {
		if _, _, ok := (Meta{Taken: s, Offset: "+01:00"}).Time(); ok {
			t.Errorf("%q gives a time", s)
		}
	}
}

func TestUpright(t *testing.T) {
	for o := range 10 {
		w, h := Meta{Width: 40, Height: 30, Orientation: o}.Upright()
		swapped := o >= 5 && o <= 8
		if swapped && (w != 30 || h != 40) || !swapped && (w != 40 || h != 30) {
			t.Errorf("orientation %d: %dx%d", o, w, h)
		}
	}
}

// TestMetaJSON: the names in the event log; empty fields are left out.
func TestMetaJSON(t *testing.T) {
	b, _ := json.Marshal(Meta{})
	if string(b) != "{}" {
		t.Errorf("empty meta = %s", b)
	}
	b, _ = json.Marshal(sized(4, 3))
	const want = `{"taken":"2023-07-14T18:30:05","offset":"+02:00","lat":47.3766667,"lon":-8.54,"alt":-408.5,"make":"Canon","model":"Canon EOS R5","lens":"RF24-105mm F4 L IS USM","exposure":"1/250","fNumber":2.8,"iso":400,"focal":105.3,"focal35":105,"orientation":6,"width":4,"height":3}`
	if string(b) != want {
		t.Errorf("got  %s\nwant %s", b, want)
	}
	var back Meta
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	check(t, back, sized(4, 3))
}
