package photo

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"slices"
)

// maxTIFFBytes bounds the TIFF structure SetTaken writes into a PNG or WebP
// chunk: what Read takes from such a chunk. A JPEG's APP1 segment holds
// less.
const maxTIFFBytes = maxExifBytes

// maxAPP1 is the most a JPEG segment holds after its length.
const maxAPP1 = 0xFFFF - 2

// SetTaken writes when a picture was taken into its EXIF (ADR-0196): taken
// is "2006-01-02T15:04:05", offset its offset from UTC, "+02:00", or
// empty. The Exif directory's DateTimeOriginal is set and its
// OffsetTimeOriginal set or, without an offset, removed; a picture without
// EXIF, or without an Exif directory, is given one. Nothing the file holds
// is moved: a changed directory is written again after the TIFF structure,
// and what pointed to the old one points to the new. It works on the whole
// file in memory and answers the file with the time and true; data is not
// changed. A format it does not know, a file it cannot walk, or a time it
// cannot write, is answered as it is with false.
func SetTaken(data []byte, mediaType, taken, offset string) ([]byte, bool) {
	stamp, ok := parseExifTime([]byte(taken))
	if !ok || len(taken) != len("2006-01-02T15:04:05") || taken[4] != '-' || taken[7] != '-' || taken[10] != 'T' {
		return data, false
	}
	// "2006-01-02T15:04:05" as EXIF writes it: "2006:01:02 15:04:05".
	b := []byte(stamp)
	b[4], b[7], b[10] = ':', ':', ' '
	stamp = string(b)
	if offset != "" {
		if offset, _, ok = parseOffset(offset); !ok {
			return data, false
		}
	}
	var out []byte
	ok = false
	switch normType(mediaType) {
	case "image/jpeg":
		out, ok = takenJPEG(data, stamp, offset)
	case "image/png":
		out, ok = takenPNG(data, stamp, offset)
	case "image/webp":
		out, ok = takenWebP(data, stamp, offset)
	}
	if !ok {
		return data, false
	}
	return out, true
}

// dirEntry is one IFD entry as its 12 bytes, read from or written into a
// TIFF structure.
type dirEntry [12]byte

// dirEntries reads the entries of the IFD at off and the offset of the next
// one; ok is false when the IFD does not lie inside b.
func dirEntries(b []byte, bo binary.ByteOrder, off uint32) (entries []dirEntry, next uint32, ok bool) {
	if off < 8 || uint64(off)+2 > uint64(len(b)) {
		return nil, 0, false
	}
	n := int(bo.Uint16(b[off:]))
	end := uint64(off) + 2 + 12*uint64(n)
	if n > maxEntries || end+4 > uint64(len(b)) {
		return nil, 0, false
	}
	entries = make([]dirEntry, n)
	for i := range entries {
		copy(entries[i][:], b[int(off)+2+12*i:])
	}
	return entries, bo.Uint32(b[end:]), true
}

// tag is the tag of an entry.
func (e dirEntry) tag(bo binary.ByteOrder) uint16 { return bo.Uint16(e[:]) }

// newEntry is an entry of a type and count whose value is the 4 bytes of
// val, or lies at the offset val.
func newEntry(bo binary.ByteOrder, tag, typ uint16, n, val uint32) dirEntry {
	var e dirEntry
	bo.PutUint16(e[0:], tag)
	bo.PutUint16(e[2:], typ)
	bo.PutUint32(e[4:], n)
	bo.PutUint32(e[8:], val)
	return e
}

// appendIFD writes an IFD with its entries, sorted by tag as TIFF wants,
// and the offset of the next one at the end of b, on an even offset, and
// answers b and where the IFD starts.
func appendIFD(b []byte, bo binary.ByteOrder, entries []dirEntry, next uint32) ([]byte, uint32) {
	slices.SortStableFunc(entries, func(x, y dirEntry) int { return int(x.tag(bo)) - int(y.tag(bo)) })
	if len(b)%2 == 1 {
		b = append(b, 0)
	}
	at := uint32(len(b))
	var n [4]byte
	bo.PutUint16(n[:], uint16(len(entries)))
	b = append(b, n[:2]...)
	for _, e := range entries {
		b = append(b, e[:]...)
	}
	bo.PutUint32(n[:], next)
	return append(b, n[:]...), at
}

// emptyTIFF is a TIFF structure with an empty IFD0, for a picture without
// EXIF.
func emptyTIFF() []byte {
	return []byte{'M', 'M', 0, 42, 0, 0, 0, 8, 0, 0, 0, 0, 0, 0}
}

// timeTIFF answers the TIFF structure b, nil for none, with stamp
// ("2006:01:02 15:04:05") as DateTimeOriginal of its Exif directory and
// offset as OffsetTimeOriginal, or none without an offset. The Exif
// directory is written again at the end, and IFD0 too when it had none;
// every other byte stays where it was, so every offset into b still holds.
func timeTIFF(b []byte, stamp, offset string) ([]byte, bool) {
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
	out := bytes.Clone(b)
	if len(out)%2 == 1 {
		out = append(out, 0)
	}
	stampAt := uint32(len(out))
	out = append(append(out, stamp...), 0)
	offsetAt := uint32(len(out))
	if offset != "" {
		out = append(append(out, offset...), 0)
	}

	// The Exif directory as it was, if IFD0 points to one that lies inside
	// the structure.
	pointer := -1
	var exif []dirEntry
	var exifNext uint32
	for i, e := range ifd0 {
		if e.tag(bo) != tagExifIFD {
			continue
		}
		pointer = i
		if old, next, ok := dirEntries(b, bo, bo.Uint32(e[8:])); ok {
			exif, exifNext = old, next
		}
		break
	}
	exif = slices.DeleteFunc(exif, func(e dirEntry) bool {
		return e.tag(bo) == tagOriginal || e.tag(bo) == tagOffsetOrig
	})
	exif = append(exif, newEntry(bo, tagOriginal, typeASCII, uint32(len(stamp)+1), stampAt))
	if offset != "" {
		exif = append(exif, newEntry(bo, tagOffsetOrig, typeASCII, uint32(len(offset)+1), offsetAt))
	}
	if len(exif) > maxEntries {
		return nil, false
	}
	out, exifAt := appendIFD(out, bo, exif, exifNext)

	if pointer >= 0 {
		// IFD0 stays where it is; its pointer to the Exif directory is set.
		at := int(ifd0At) + 2 + 12*pointer
		bo.PutUint16(out[at+2:], typeLong)
		bo.PutUint32(out[at+4:], 1)
		bo.PutUint32(out[at+8:], exifAt)
		return out, true
	}
	if len(ifd0)+1 > maxEntries {
		return nil, false
	}
	ifd0 = append(ifd0, newEntry(bo, tagExifIFD, typeLong, 1, exifAt))
	out, at := appendIFD(out, bo, ifd0, ifd0Next)
	bo.PutUint32(out[4:], at)
	return out, true
}

// takenJPEG writes the time into the first Exif segment, or into a new one
// after the SOI and the APP0 segments that follow it, before the image
// data.
func takenJPEG(data []byte, stamp, offset string) ([]byte, bool) {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return nil, false
	}
	insert := 2
	at := 2
	app0 := true
	for range maxSegments {
		if at+4 > len(data) || data[at] != 0xFF {
			return nil, false
		}
		code := data[at+1]
		if code == 0xDA || code == 0xD9 || code == 0x01 || (code >= 0xD0 && code <= 0xD7) || code == 0xFF {
			break // the image data, or what has no length: no metadata after it is read
		}
		end := at + 2 + int(binary.BigEndian.Uint16(data[at+2:]))
		if end > len(data) || end < at+4 {
			return nil, false
		}
		body := data[at+4 : end]
		if code == 0xE1 && len(body) >= 6 && bytes.HasPrefix(body, []byte("Exif\x00")) {
			tiff, ok := timeTIFF(body[6:], stamp, offset)
			if !ok {
				return nil, false
			}
			seg, ok := exifSegment(body[:6], tiff)
			if !ok {
				return nil, false
			}
			return slices.Concat(data[:at], seg, data[end:]), true
		}
		if app0 && code == 0xE0 {
			insert = end
		} else {
			app0 = false
		}
		at = end
	}
	tiff, _ := timeTIFF(nil, stamp, offset)
	seg, _ := exifSegment([]byte("Exif\x00\x00"), tiff)
	return slices.Concat(data[:insert], seg, data[insert:]), true
}

// exifSegment is an APP1 segment of head ("Exif\0\0") and a TIFF structure,
// unless it is too long for a segment.
func exifSegment(head, tiff []byte) ([]byte, bool) {
	n := len(head) + len(tiff)
	if n > maxAPP1 {
		return nil, false
	}
	seg := []byte{0xFF, 0xE1, byte((n + 2) >> 8), byte(n + 2)}
	return append(append(seg, head...), tiff...), true
}

// pngChunk is a PNG chunk of a type and data with its CRC.
func pngChunk(kind string, data []byte) []byte {
	b := binary.BigEndian.AppendUint32(nil, uint32(len(data)))
	b = append(append(b, kind...), data...)
	return binary.BigEndian.AppendUint32(b, crc32.ChecksumIEEE(b[4:]))
}

// takenPNG writes the time into the eXIf chunk, or into a new one right
// after IHDR.
func takenPNG(data []byte, stamp, offset string) ([]byte, bool) {
	const sig = "\x89PNG\r\n\x1a\n"
	if !bytes.HasPrefix(data, []byte(sig)) || len(data) < 8+12 || string(data[12:16]) != "IHDR" {
		return nil, false
	}
	afterIHDR := -1
	at := 8
	for range maxPNGChunks {
		if at+12 > len(data) {
			return nil, false
		}
		n := uint64(binary.BigEndian.Uint32(data[at:]))
		end := uint64(at) + 12 + n
		if end > uint64(len(data)) {
			return nil, false
		}
		kind := string(data[at+4 : at+8])
		if afterIHDR < 0 {
			afterIHDR = int(end)
		}
		switch kind {
		case "eXIf":
			tiff, ok := timeTIFF(data[at+8:at+8+int(n)], stamp, offset)
			if !ok || len(tiff) > maxTIFFBytes {
				return nil, false
			}
			return slices.Concat(data[:at], pngChunk("eXIf", tiff), data[end:]), true
		case "IEND":
			tiff, _ := timeTIFF(nil, stamp, offset)
			return slices.Concat(data[:afterIHDR], pngChunk("eXIf", tiff), data[afterIHDR:]), true
		}
		at = int(end)
	}
	return nil, false
}

// webpChunk is a WebP chunk of a FourCC and data, padded to an even size.
func webpChunk(fourcc string, data []byte) []byte {
	b := append([]byte(fourcc), binary.LittleEndian.AppendUint32(nil, uint32(len(data)))...)
	b = append(b, data...)
	if len(data)%2 == 1 {
		b = append(b, 0)
	}
	return b
}

// takenWebP writes the time into the EXIF chunk, or into a new one after
// the image and before XMP, with the flag in VP8X that says EXIF follows.
// A simple WebP, which has no VP8X, is given one for its canvas.
func takenWebP(data []byte, stamp, offset string) ([]byte, bool) {
	if len(data) < 20 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return nil, false
	}
	type span struct {
		fourcc     string
		start, end int
	}
	var chunks []span
	at := 12
	for at < len(data) {
		if len(chunks) >= maxWebPChunks || at+8 > len(data) {
			return nil, false
		}
		size := uint64(binary.LittleEndian.Uint32(data[at+4:]))
		end := uint64(at) + 8 + size + size&1
		if end > uint64(len(data)) {
			return nil, false
		}
		chunks = append(chunks, span{string(data[at : at+4]), at, int(end)})
		at = int(end)
	}
	if len(chunks) == 0 {
		return nil, false
	}
	var tiff []byte
	exifAt := -1
	for i, c := range chunks {
		if c.fourcc == "EXIF" {
			exifAt = i
			tiff = data[c.start+8 : c.start+8+int(binary.LittleEndian.Uint32(data[c.start+4:]))]
			break
		}
	}
	tiff, ok := timeTIFF(tiff, stamp, offset)
	if !ok || len(tiff) > maxTIFFBytes {
		return nil, false
	}
	exif := webpChunk("EXIF", tiff)

	var out []byte
	switch {
	case chunks[0].fourcc == "VP8X":
		if chunks[0].end-chunks[0].start < 8+10 {
			return nil, false
		}
		out = bytes.Clone(data[:12])
		placed := false
		for i, c := range chunks {
			switch {
			case i == exifAt:
				out, placed = append(out, exif...), true
			case c.fourcc == "EXIF":
				// a second EXIF chunk is left out: only the first is read
			case c.fourcc == "XMP " && !placed:
				out = append(append(out, exif...), data[c.start:c.end]...)
				placed = true
			default:
				out = append(out, data[c.start:c.end]...)
			}
		}
		if !placed {
			out = append(out, exif...)
		}
		out[20] |= 0x08 // the flag that says EXIF follows
	case chunks[0].fourcc == "VP8 " || chunks[0].fourcc == "VP8L":
		m := read(bytes.NewReader(data), "image/webp")
		if m.Width < 1 || m.Height < 1 || m.Width > 1<<24 || m.Height > 1<<24 {
			return nil, false
		}
		flags := byte(0x08)
		if c := chunks[0]; c.fourcc == "VP8L" && c.end-c.start >= 8+5 && binary.LittleEndian.Uint32(data[c.start+9:])>>28&1 == 1 {
			flags |= 0x10 // the lossless image uses its alpha channel
		}
		w, h := m.Width-1, m.Height-1
		head := []byte{flags, 0, 0, 0, byte(w), byte(w >> 8), byte(w >> 16), byte(h), byte(h >> 8), byte(h >> 16)}
		out = slices.Concat(data[:12], webpChunk("VP8X", head), data[12:at], exif)
	default:
		return nil, false
	}
	binary.LittleEndian.PutUint32(out[4:], uint32(len(out)-8))
	return out, true
}
