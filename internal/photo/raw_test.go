package photo

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/jpeg"
	"testing"
)

func jpegOf(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewGray(image.Rect(0, 0, w, h)), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// rawOf is a little-endian TIFF whose first IFD tells an orientation,
// followed by the bytes given, as a camera's RAW holds its JPEGs.
func rawOf(orientation uint16, rest ...[]byte) []byte {
	var b bytes.Buffer
	b.WriteString("II*\x00")
	binary.Write(&b, binary.LittleEndian, uint32(8))
	binary.Write(&b, binary.LittleEndian, uint16(1))
	for _, v := range []any{uint16(0x0112), uint16(3), uint32(1), orientation, uint16(0)} {
		binary.Write(&b, binary.LittleEndian, v)
	}
	binary.Write(&b, binary.LittleEndian, uint32(0))
	for _, r := range rest {
		b.Write(r)
	}
	return b.Bytes()
}

func TestEmbeddedJPEG(t *testing.T) {
	small, large := jpegOf(t, 16, 12), jpegOf(t, 160, 120)
	raw := rawOf(6, []byte("\xFF\xD8\xFF garbage that is no JPEG"), small, []byte{1, 2, 3}, large, []byte("raw sensor data"))
	pic, orientation := EmbeddedJPEG(raw, 1<<20)
	if orientation != 6 {
		t.Errorf("orientation %d", orientation)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(pic))
	if err != nil || cfg.Width != 160 {
		t.Fatalf("the largest JPEG: %+v %v", cfg, err)
	}
	// Within maxPixels the smaller one is taken.
	pic, _ = EmbeddedJPEG(raw, 16*12)
	if cfg, _ := jpeg.DecodeConfig(bytes.NewReader(pic)); cfg.Width != 16 {
		t.Errorf("within the bound: %+v", cfg)
	}
	// A RAF, no TIFF: its JPEG without an orientation.
	pic, orientation = EmbeddedJPEG(append([]byte("FUJIFILMCCD-RAW 0201"), large...), 1<<20)
	if pic == nil || orientation != 0 {
		t.Errorf("RAF: %d bytes, orientation %d", len(pic), orientation)
	}
	if pic, _ := EmbeddedJPEG(rawOf(1, []byte("no picture at all")), 1<<20); pic != nil {
		t.Error("a JPEG found where there is none")
	}
	// Broken TIFFs tell no orientation.
	for _, bad := range [][]byte{nil, []byte("II*\x00\x02\x00\x00\x00"), []byte("II*\x00\x08\x00\x00\x00\x05\x00"), []byte("MM\x00*\x00\x00\x00\x08\x00\x01")} {
		if o := tiffOrientation(bad); o != 0 {
			t.Errorf("tiffOrientation(%q) = %d", bad, o)
		}
	}
}
