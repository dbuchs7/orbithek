package photo

import (
	"bytes"
	"encoding/binary"
)

// An EXIF orientation says how a picture as stored is turned upright: by
// mirroring it or not, then turning it by quarter turns clockwise. turns
// gives that pair for the orientations 1 to 8, as orient applies them.
var turns = [9]struct{ mirror, quarters int }{
	{0, 0}, // not told: upright
	{0, 0}, // 1
	{1, 0}, // 2 mirrors
	{0, 2}, // 3 turns by 180°
	{1, 2}, // 4 flips: mirrors, then turns by 180°
	{1, 3}, // 5 transposes: mirrors, then turns 90° counter-clockwise
	{0, 1}, // 6 turns 90° clockwise
	{1, 1}, // 7 transverses: mirrors, then turns 90° clockwise
	{0, 3}, // 8 turns 90° counter-clockwise
}

// orientationOf is the orientation of a mirror and quarter turns.
var orientationOf = [2][4]int{{1, 6, 3, 8}, {2, 7, 4, 5}}

// Turned is the EXIF orientation that shows a picture of orientation o as
// it is shown upright, turned by quarters quarter turns clockwise, or
// counter-clockwise when negative (ADR-draft-pictures-turned-by-hand). A picture that tells
// no orientation, or one no EXIF writes, is read as upright.
func Turned(o, quarters int) int {
	if o < 1 || o > 8 {
		o = 1
	}
	t := turns[o]
	return orientationOf[t.mirror][((t.quarters+quarters)%4+4)%4]
}

// SetOrientation writes a picture's EXIF orientation, 1 to 8, into IFD0
// (ADR-draft-pictures-turned-by-hand): an orientation IFD0 holds is set where it is, and an
// IFD0 without one is written again after the TIFF structure with it; a
// picture without EXIF is given EXIF that tells its orientation. It works
// as SetTaken does, on the whole file in memory, in JPEG, PNG and WebP,
// and answers data as it is with false when it cannot write.
func SetOrientation(data []byte, mediaType string, o int) ([]byte, bool) {
	if o < 1 || o > 8 {
		return data, false
	}
	return editExif(data, mediaType, func(tiff []byte) ([]byte, bool) { return orientationTIFF(tiff, o) })
}

// orientationTIFF answers the TIFF structure b, nil for none, with o as
// the orientation of its IFD0. Every byte stays where it was but those of
// the entry, so every offset into b still holds.
func orientationTIFF(b []byte, o int) ([]byte, bool) {
	if len(b) == 0 {
		b = emptyTIFF()
	}
	if len(b) < 8 {
		return nil, false
	}
	var bo binary.ByteOrder
	switch string(b[:4]) {
	case "II*\x00":
		bo = binary.LittleEndian
	case "MM\x00*":
		bo = binary.BigEndian
	default:
		return nil, false
	}
	ifd0At := bo.Uint32(b[4:])
	ifd0, ifd0Next, ok := dirEntries(b, bo, ifd0At)
	if !ok {
		return nil, false
	}
	entry := newEntry(bo, tagOrientation, typeShort, 1, 0)
	bo.PutUint16(entry[8:], uint16(o))
	out := bytes.Clone(b)
	for i, e := range ifd0 {
		if e.tag(bo) == tagOrientation {
			at := int(ifd0At) + 2 + 12*i
			copy(out[at:at+12], entry[:])
			return out, true
		}
	}
	if len(ifd0)+1 > maxEntries {
		return nil, false
	}
	out, at := appendIFD(out, bo, append(ifd0, entry), ifd0Next)
	bo.PutUint32(out[4:], at)
	return out, true
}
