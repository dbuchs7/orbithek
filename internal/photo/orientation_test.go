package photo

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/jpeg"
	"image/png"
	"testing"
)

// TestTurnedComposesQuarterTurns: a picture's orientation turned by quarter
// turns is the orientation that shows it upright and then turned that far,
// mirrored ones included; four turns, or none, give it back, and a picture
// that tells no orientation is read as upright (ADR-draft-pictures-turned-by-hand).
func TestTurnedComposesQuarterTurns(t *testing.T) {
	cases := []struct{ o, q, want int }{
		{0, 0, 1}, {0, 1, 6}, {1, 1, 6}, {6, 1, 3}, {3, 1, 8}, {8, 1, 1},
		{1, -1, 8}, {6, -1, 1}, {1, 2, 3}, {6, 2, 8}, {1, 3, 8}, {1, 4, 1}, {6, -5, 1},
		{2, 1, 7}, {7, 1, 4}, {4, 1, 5}, {5, 1, 2}, {2, -1, 5},
		{9, 1, 6}, // a value no EXIF writes is read as upright
	}
	for _, c := range cases {
		if got := Turned(c.o, c.q); got != c.want {
			t.Errorf("Turned(%d, %d) = %d, want %d", c.o, c.q, got, c.want)
		}
	}
}

// TestTurnedTurnsWhatIsShown: for every orientation, the picture turned
// upright by Turned(o, 1) is the picture turned upright by o and then a
// quarter turn clockwise, pixel for pixel.
func TestTurnedTurnsWhatIsShown(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 3, 2))
	for i := range src.Pix {
		src.Pix[i] = byte(i * 7)
	}
	for o := 1; o <= 8; o++ {
		want := orient(orient(src, o), 6)
		got := orient(src, Turned(o, 1))
		if !bytes.Equal(got.Pix, want.Pix) || got.Rect != want.Rect {
			t.Errorf("orientation %d turned once shows another picture", o)
		}
	}
}

// TestTheOrientationIsWrittenIntoAJPEG: the orientation replaces the one
// IFD0 holds, in both byte orders, and nothing else the file tells changes;
// the picture still decodes and data is not changed.
func TestTheOrientationIsWrittenIntoAJPEG(t *testing.T) {
	for _, bo := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
		in := jpegWith(encodeJPEG(t, testImage(32, 16)), exifAPP1(fullTIFF(bo)))
		keep := bytes.Clone(in)
		before := Read(bytes.NewReader(in), "image/jpeg")
		out, ok := SetOrientation(in, "image/jpeg", 8)
		if !ok {
			t.Fatal("not written")
		}
		if !bytes.Equal(in, keep) {
			t.Error("the input was changed")
		}
		after := Read(bytes.NewReader(out), "image/jpeg")
		if after.Orientation != 8 {
			t.Errorf("tells orientation %d, want 8", after.Orientation)
		}
		after.Orientation, before.Orientation = 0, 0
		check(t, after, before)
		if _, err := jpeg.Decode(bytes.NewReader(out)); err != nil {
			t.Errorf("the picture no longer decodes: %v", err)
		}
		// The time set by hand and the orientation are written one after
		// the other, each keeping the other.
		both, ok := SetTaken(out, "image/jpeg", "1987-06-05T04:03:02", "")
		if !ok {
			t.Fatal("the time is not written beside the orientation")
		}
		if m := Read(bytes.NewReader(both), "image/jpeg"); m.Orientation != 8 || m.Taken != "1987-06-05T04:03:02" {
			t.Errorf("tells orientation %d and time %q", m.Orientation, m.Taken)
		}
	}
}

// TestAPictureWithoutAnOrientationGetsOne: IFD0 without an orientation is
// written again with one, keeping what it held, and a picture without EXIF
// is given EXIF that tells only its orientation.
func TestAPictureWithoutAnOrientationGetsOne(t *testing.T) {
	ifd0 := buildTIFF(binary.BigEndian, []tg{asciiTag(tagMake, "Maker")}, []tg{asciiTag(tagOriginal, "2024:05:01 13:22:10")}, nil)
	in := jpegWith(encodeJPEG(t, testImage(8, 4)), exifAPP1(ifd0))
	out, ok := SetOrientation(in, "image/jpeg", 6)
	if !ok {
		t.Fatal("not written beside IFD0")
	}
	check(t, Read(bytes.NewReader(out), "image/jpeg"), Meta{Make: "Maker", Taken: "2024-05-01T13:22:10", Orientation: 6, Width: 8, Height: 4})

	plain := encodePNG(t, testImage(8, 8))
	out, ok = SetOrientation(plain, "image/png", 3)
	if !ok {
		t.Fatal("not written into a PNG without eXIf")
	}
	if m := Read(bytes.NewReader(out), "image/png"); m.Orientation != 3 {
		t.Errorf("the PNG tells orientation %d, want 3", m.Orientation)
	}
	if _, err := png.Decode(bytes.NewReader(out)); err != nil {
		t.Errorf("the PNG no longer decodes (checksum?): %v", err)
	}
}

// TestTheOrientationIsWrittenIntoAWebP: a simple WebP is given VP8X and an
// EXIF chunk that tells the orientation.
func TestTheOrientationIsWrittenIntoAWebP(t *testing.T) {
	in := webp(riffChunk("VP8L", vp8l(16, 8)))
	out, ok := SetOrientation(in, "image/webp", 5)
	if !ok {
		t.Fatal("not written")
	}
	if m := Read(bytes.NewReader(out), "image/webp"); m.Orientation != 5 || m.Width != 16 || m.Height != 8 {
		t.Errorf("the WebP tells orientation %d and %d × %d", m.Orientation, m.Width, m.Height)
	}
}

// TestAnOrientationThatCannotBeWritten: a value no EXIF writes, a format
// without EXIF (GIF) and a damaged file are given back as they are.
func TestAnOrientationThatCannotBeWritten(t *testing.T) {
	in := encodePNG(t, testImage(4, 4))
	for _, o := range []int{0, 9, -1} {
		if out, ok := SetOrientation(in, "image/png", o); ok || !bytes.Equal(out, in) {
			t.Errorf("orientation %d was written", o)
		}
	}
	gif := encodeGIF(t, testImage(4, 4))
	if out, ok := SetOrientation(gif, "image/gif", 6); ok || !bytes.Equal(out, gif) {
		t.Error("a GIF was changed")
	}
	broken := jpegWith(encodeJPEG(t, testImage(4, 4)), exifAPP1([]byte("II*\x00\xff\xff\xff\x00")))
	if out, ok := SetOrientation(broken, "image/jpeg", 6); ok || !bytes.Equal(out, broken) {
		t.Error("a file whose IFD0 lies outside its EXIF was changed")
	}
}
