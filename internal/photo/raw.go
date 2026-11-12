package photo

import (
	"bytes"
	"encoding/binary"
	"image/jpeg"
)

// maxEmbeddedCandidates bounds the starts of a JPEG looked at in one RAW.
const maxEmbeddedCandidates = 512

// EmbeddedJPEG finds the largest JPEG a camera embedded in its RAW that
// the standard library decodes, within maxPixels, and the orientation the
// RAW tells, for a preview of it
// (ADR-draft-signed-links-to-a-shares-originals-and-previews-of-raw-and-webp). A TIFF-based RAW
// (DNG, CR2, NEF, ARW and others) holds it in an IFD, CR3 in a box of its
// own, RAF at an offset of its header; every one of them starts with the
// markers of a JPEG, which is what is looked for. The RAW data themselves,
// often lossless JPEG, are not decoded and so not taken. It answers nil
// when there is none.
func EmbeddedJPEG(data []byte, maxPixels int) (picture []byte, orientation int) {
	best := int64(0)
	start := 0
	for n := 0; n < maxEmbeddedCandidates; n++ {
		i := bytes.Index(data[start:], []byte{0xFF, 0xD8, 0xFF})
		if i < 0 {
			break
		}
		at := start + i
		start = at + 3
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(data[at:]))
		if err != nil || cfg.Width < 1 || cfg.Height < 1 {
			continue
		}
		area := int64(cfg.Width) * int64(cfg.Height)
		if area > int64(maxPixels) || area <= best {
			continue
		}
		best, picture = area, data[at:]
	}
	if picture == nil {
		return nil, 0
	}
	return picture, tiffOrientation(data)
}

// tiffOrientation reads the orientation (tag 0x0112) of a TIFF's first
// IFD, 0 when it is no TIFF or tells none. A RAW's embedded JPEG is
// stored as the sensor saw it, turned by this.
func tiffOrientation(data []byte) int {
	if len(data) < 8 {
		return 0
	}
	var order binary.ByteOrder
	switch string(data[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 0
	}
	ifd := int(order.Uint32(data[4:8]))
	if ifd < 8 || ifd+2 > len(data) {
		return 0
	}
	n := int(order.Uint16(data[ifd:]))
	for k := 0; k < n; k++ {
		e := ifd + 2 + 12*k
		if e+12 > len(data) {
			return 0
		}
		if order.Uint16(data[e:]) == 0x0112 && order.Uint16(data[e+2:]) == 3 {
			if v := int(order.Uint16(data[e+8:])); v >= 1 && v <= 8 {
				return v
			}
		}
	}
	return 0
}
