package photo

import (
	"bytes"
	"encoding/binary"
	"image/jpeg"
	"image/png"
	"testing"
)

// taken reports that a file tells the time and offset written into it and
// still everything else it told.
func takenIs(t *testing.T, data []byte, kind, taken, offset string, want Meta) {
	t.Helper()
	m := Read(bytes.NewReader(data), kind)
	if m.Taken != taken || m.Offset != offset {
		t.Errorf("%s tells %q %q, want %q %q", kind, m.Taken, m.Offset, taken, offset)
	}
	m.Taken, m.Offset, want.Taken, want.Offset = "", "", "", ""
	check(t, m, want)
}

// TestTheTimeIsWrittenIntoAJPEG: the time and offset replace those the
// file told, in both byte orders, and nothing else it tells changes; the
// picture still decodes and data is not changed (ADR-0198).
func TestTheTimeIsWrittenIntoAJPEG(t *testing.T) {
	for _, bo := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
		in := jpegWith(encodeJPEG(t, testImage(32, 16)), exifAPP1(fullTIFF(bo)))
		keep := bytes.Clone(in)
		before := Read(bytes.NewReader(in), "image/jpeg")
		out, ok := SetTaken(in, "image/jpeg", "1987-06-05T04:03:02", "-03:30")
		if !ok {
			t.Fatal("not written")
		}
		if !bytes.Equal(in, keep) {
			t.Error("the input was changed")
		}
		takenIs(t, out, "image/jpeg", "1987-06-05T04:03:02", "-03:30", before)
		if _, err := jpeg.Decode(bytes.NewReader(out)); err != nil {
			t.Errorf("the picture no longer decodes: %v", err)
		}
		// Written again, the newest time counts, without an offset.
		again, ok := SetTaken(out, "image/jpeg", "2001-02-03T10:20:30", "")
		if !ok {
			t.Fatal("not written again")
		}
		takenIs(t, again, "image/jpeg", "2001-02-03T10:20:30", "", before)
	}
}

// TestAJPEGWithoutExifGetsIt: a picture without EXIF is given an Exif
// segment after its APP0 segment, and one whose EXIF has no Exif directory
// gets one beside the IFD0 it had.
func TestAJPEGWithoutExifGetsIt(t *testing.T) {
	jfif := segment(0xE0, []byte("JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00"))
	in := jpegWith(encodeJPEG(t, testImage(8, 4)), jfif)
	out, ok := SetTaken(in, "image/jpeg", "1950-12-24T18:00:00", "+01:00")
	if !ok {
		t.Fatal("not written")
	}
	takenIs(t, out, "image/jpeg", "1950-12-24T18:00:00", "+01:00", Meta{Width: 8, Height: 4})
	if !bytes.Equal(out[2:2+len(jfif)], jfif) || out[2+len(jfif)+1] != 0xE1 {
		t.Error("the Exif segment does not follow APP0")
	}
	if _, err := jpeg.Decode(bytes.NewReader(out)); err != nil {
		t.Errorf("the picture no longer decodes: %v", err)
	}

	ifd0Only := buildTIFF(binary.BigEndian, []tg{asciiTag(tagMake, "Maker"), shortTag(tagOrientation, 6), asciiTag(tagDateTime, "2020:01:01 00:00:00")}, nil, nil)
	in = jpegWith(encodeJPEG(t, testImage(8, 4)), exifAPP1(ifd0Only))
	out, ok = SetTaken(in, "image/jpeg", "1999-12-31T23:59:59", "")
	if !ok {
		t.Fatal("not written beside IFD0")
	}
	want := Meta{Make: "Maker", Orientation: 6, Width: 8, Height: 4}
	takenIs(t, out, "image/jpeg", "1999-12-31T23:59:59", "", want)
}

// TestAnOffsetLeavesWithoutOne: a time written without an offset removes
// the offset the file told for its time.
func TestAnOffsetLeavesWithoutOne(t *testing.T) {
	tiff := buildTIFF(binary.LittleEndian, nil, []tg{asciiTag(tagOriginal, "2024:05:01 13:22:10"), asciiTag(tagOffsetOrig, "+05:00"), asciiTag(tagDigitized, "2024:05:01 13:22:10")}, nil)
	in := jpegWith(encodeJPEG(t, testImage(4, 4)), exifAPP1(tiff))
	out, ok := SetTaken(in, "image/jpeg", "2024-05-02T08:00:00", "")
	if !ok {
		t.Fatal("not written")
	}
	takenIs(t, out, "image/jpeg", "2024-05-02T08:00:00", "", Meta{Width: 4, Height: 4})
}

// TestTheTimeIsWrittenIntoAPNG: into its eXIf chunk, or a new one after
// IHDR, with a valid checksum.
func TestTheTimeIsWrittenIntoAPNG(t *testing.T) {
	plain := encodePNG(t, testImage(8, 8))
	for name, in := range map[string][]byte{
		"with eXIf":    pngWith(plain, false, chunk("eXIf", fullTIFF(binary.BigEndian)), chunk("tEXt", []byte("Comment\x00kept"))),
		"late eXIf":    pngWith(plain, true, chunk("eXIf", fullTIFF(binary.LittleEndian))),
		"without eXIf": plain,
	} {
		before := Read(bytes.NewReader(in), "image/png")
		out, ok := SetTaken(in, "image/png", "2010-10-10T10:10:10", "+00:00")
		if !ok {
			t.Fatalf("%s: not written", name)
		}
		takenIs(t, out, "image/png", "2010-10-10T10:10:10", "+00:00", before)
		if _, err := png.Decode(bytes.NewReader(out)); err != nil {
			t.Errorf("%s: the picture no longer decodes (checksum?): %v", name, err)
		}
		if bytes.Contains(in, []byte("Comment")) && !bytes.Contains(out, []byte("Comment\x00kept")) {
			t.Errorf("%s: another chunk was dropped", name)
		}
	}
}

// TestTheTimeIsWrittenIntoAWebP: into its EXIF chunk, or a new one before
// XMP with its flag set; a simple WebP is given VP8X for its canvas; the
// container's length is told again.
func TestTheTimeIsWrittenIntoAWebP(t *testing.T) {
	alpha := vp8l(16, 8)
	alpha[4] |= 0x10 // alpha_is_used
	cases := map[string][]byte{
		"VP8X with EXIF":    webp(riffChunk("VP8X", vp8x(16, 8)), riffChunk("VP8L", vp8l(16, 8)), riffChunk("EXIF", fullTIFF(binary.LittleEndian))),
		"VP8X without EXIF": webp(riffChunk("VP8X", []byte{0x04, 0, 0, 0, 15, 0, 0, 7, 0, 0}), riffChunk("VP8L", vp8l(16, 8)), riffChunk("XMP ", []byte(xmpPacket))),
		"simple lossy":      webp(riffChunk("VP8 ", vp8(16, 8))),
		"simple lossless":   webp(riffChunk("VP8L", alpha)),
	}
	for name, in := range cases {
		before := Read(bytes.NewReader(in), "image/webp")
		out, ok := SetTaken(in, "image/webp", "2015-03-04T05:06:07", "+09:00")
		if !ok {
			t.Fatalf("%s: not written", name)
		}
		takenIs(t, out, "image/webp", "2015-03-04T05:06:07", "+09:00", before)
		if got := binary.LittleEndian.Uint32(out[4:]); int(got) != len(out)-8 {
			t.Errorf("%s: the container says %d bytes, holds %d", name, got, len(out)-8)
		}
		if string(out[12:16]) != "VP8X" || out[20]&0x08 == 0 {
			t.Errorf("%s: no VP8X with the EXIF flag", name)
		}
		if x, e := bytes.Index(out, []byte("XMP ")), bytes.Index(out, []byte("EXIF")); x >= 0 && e > x {
			t.Errorf("%s: EXIF follows XMP", name)
		}
	}
	out, _ := SetTaken(cases["simple lossless"], "image/webp", "2015-03-04T05:06:07", "")
	if out[20]&0x10 == 0 {
		t.Error("the alpha flag of a lossless picture with alpha is not set")
	}
	out, _ = SetTaken(cases["simple lossy"], "image/webp", "2015-03-04T05:06:07", "")
	if out[20]&0x10 != 0 {
		t.Error("the alpha flag of a lossy picture is set")
	}
}

// TestSetTakenLeavesWhatItCannotWrite: other formats, bad times and
// offsets, and damaged files come back as they are, without a panic.
func TestSetTakenLeavesWhatItCannotWrite(t *testing.T) {
	plain := encodeJPEG(t, testImage(4, 4))
	for _, c := range []struct{ taken, offset string }{
		{"", ""}, {"2024-02-30T10:00:00", ""}, {"2024-05-01 13:22:10", ""}, {"2024-05-01T13:22:10Z", ""},
		{"1700-01-01T00:00:00", ""}, {"2024-05-01T13:22:10", "+15:00"}, {"2024-05-01T13:22:10", "CET"},
	} {
		if out, ok := SetTaken(plain, "image/jpeg", c.taken, c.offset); ok || !bytes.Equal(out, plain) {
			t.Errorf("%q %q was written", c.taken, c.offset)
		}
	}
	gif := encodeGIF(t, testImage(4, 4))
	if _, ok := SetTaken(gif, "image/gif", "2024-05-01T13:22:10", ""); ok {
		t.Error("a GIF was written")
	}
	hugeIFD0 := rawTIFF(8, u16(maxEntries), make([]byte, 12*maxEntries), u32(0))
	for _, bad := range [][]byte{nil, []byte("\xff\xd8"), []byte("\xff\xd8\xff\xe1\x00"), []byte("\xff\xd8\xff\xe0\xff\xff"),
		[]byte("\x89PNG\r\n\x1a\n\x00\x00"), []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\x0dIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x02\x00\x00\x00\x00\x00\x00\x00"),
		[]byte("RIFF\x00\x00\x00\x00WEBPEXIF\xff\xff\xff\x7f"), webp(riffChunk("VP8X", []byte{0, 0})), webp(riffChunk("ALPH", []byte{1, 2})),
		webp(riffChunk("VP8 ", []byte{1, 2, 3})),
		append([]byte("\xff\xd8\xff\xe1\x00\x10Exif\x00\x00II*\x00"), 0xff, 0xff, 0xff, 0xff), exifAPP1(hugeIFD0), []byte("text")} {
		for _, kind := range []string{"image/jpeg", "image/png", "image/webp"} {
			keep := bytes.Clone(bad)
			if out, ok := SetTaken(bad, kind, "2024-05-01T13:22:10", ""); ok && kind == "image/jpeg" && bytes.HasPrefix(bad, []byte("\xff\xd8\xff\xe1")) {
				t.Errorf("a damaged %s was written: %x", kind, out)
			}
			if !bytes.Equal(bad, keep) {
				t.Errorf("the input %x was changed", bad)
			}
		}
	}
	// An IFD0 full of entries takes no pointer to an Exif directory more.
	if _, ok := timeTIFF(hugeIFD0, "2024:05:01 13:22:10", ""); ok {
		t.Error("an IFD0 with too many entries was written")
	}
	// An Exif segment that would grow beyond a segment is refused.
	big := buildTIFF(binary.LittleEndian, []tg{rawTag(0x9286, typeUndefined, make([]byte, maxAPP1-80))}, []tg{}, nil)
	if _, ok := SetTaken(jpegWith(plain, exifAPP1(big)), "image/jpeg", "2024-05-01T13:22:10", "+01:00"); ok {
		t.Error("an Exif segment too long was written")
	}
}

func FuzzSetTaken(f *testing.F) {
	f.Add(jpegWith(encodeJPEG(f, testImage(4, 4)), exifAPP1(fullTIFF(binary.LittleEndian))), "image/jpeg")
	f.Add(encodeJPEG(f, testImage(4, 4)), "image/jpeg")
	f.Add(pngWith(encodePNG(f, testImage(4, 4)), true, chunk("eXIf", fullTIFF(binary.BigEndian))), "image/png")
	f.Add(webp(riffChunk("VP8X", vp8x(4, 4)), riffChunk("EXIF", fullTIFF(binary.LittleEndian))), "image/webp")
	f.Add(webp(riffChunk("VP8L", vp8l(4, 4))), "image/webp")
	f.Fuzz(func(t *testing.T, data []byte, kind string) {
		keep := bytes.Clone(data)
		out, ok := SetTaken(data, kind, "2024-05-01T13:22:10", "+02:00")
		if !bytes.Equal(data, keep) {
			t.Fatal("the input was changed")
		}
		if !ok {
			if !bytes.Equal(out, data) {
				t.Fatal("a file not written was changed")
			}
			return
		}
		if m := Read(bytes.NewReader(out), kind); m.Taken != "2024-05-01T13:22:10" || m.Offset != "+02:00" {
			t.Fatalf("written, but read as %q %q", m.Taken, m.Offset)
		}
	})
}
