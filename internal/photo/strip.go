package photo

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
)

// xmpJPEG and xmpExtJPEG start the APP1 segments of XMP metadata in a JPEG;
// xmpPNG is the keyword of its iTXt chunk in a PNG.
const (
	xmpJPEG    = "http://ns.adobe.com/xap/1.0/\x00"
	xmpExtJPEG = "http://ns.adobe.com/xmp/extension/\x00"
	xmpPNG     = "XML:com.adobe.xmp\x00"
)

// StripPlace removes from a picture where it was taken (ADR-0177): the GPS
// directory of its EXIF is emptied and every value it held overwritten with
// zeros, and XMP metadata, which may repeat the place, is dropped. It works
// on the whole file in memory and answers the file without them; data is
// not changed. A format it does not know, or a file it cannot walk, is
// answered as it is: the caller checks with Read that no place is left.
func StripPlace(data []byte, mediaType string) []byte {
	switch normType(mediaType) {
	case "image/jpeg":
		return stripJPEG(data)
	case "image/png":
		return stripPNG(data)
	case "image/webp":
		return stripWebP(data)
	}
	return data
}

// blankGPS empties the GPS directory of a TIFF structure in place, a
// leading "Exif\0\0" tolerated: its values and entries are overwritten with
// zeros and its count set to none, so no reader finds a place there.
func blankGPS(b []byte) {
	if len(b) >= 6 && string(b[:5]) == "Exif\x00" {
		b = b[6:]
	}
	if len(b) < 8 {
		return
	}
	var bo binary.ByteOrder
	switch string(b[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return
	}
	entries := func(off uint32) (int, bool) {
		if off < 8 || uint64(off)+2 > uint64(len(b)) {
			return 0, false
		}
		n := int(bo.Uint16(b[off:]))
		if fit := (len(b) - int(off) - 2) / 12; n > fit {
			n = fit
		}
		return n, n <= maxEntries
	}
	ifd0 := bo.Uint32(b[4:])
	n, ok := entries(ifd0)
	if !ok {
		return
	}
	for i := range n {
		e := b[int(ifd0)+2+12*i:][:12]
		if bo.Uint16(e) != tagGPSIFD {
			continue
		}
		gps := bo.Uint32(e[8:])
		m, ok := entries(gps)
		if !ok {
			return
		}
		for j := range m {
			g := b[int(gps)+2+12*j:][:12]
			if size := typeSize(bo.Uint16(g[2:])) * uint64(bo.Uint32(g[4:])); size > 4 {
				if o := uint64(bo.Uint32(g[8:])); o+size <= uint64(len(b)) {
					clear(b[o : o+size])
				}
			}
			clear(g)
		}
		bo.PutUint16(b[gps:], 0)
	}
}

// stripJPEG empties the GPS directory of the Exif segment and drops the
// XMP segments, up to the start of the image data, after which no metadata
// follows.
func stripJPEG(data []byte) []byte {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return data
	}
	out := bytes.Clone(data[:2])
	at := 2
	for range maxSegments {
		if at+4 > len(data) || data[at] != 0xFF {
			break
		}
		code := data[at+1]
		if code == 0xDA || code == 0xD9 || code == 0x01 || (code >= 0xD0 && code <= 0xD7) || code == 0xFF {
			break // the image data, or what needs no rewriting
		}
		end := at + 2 + int(binary.BigEndian.Uint16(data[at+2:]))
		if end > len(data) || end < at+4 {
			break
		}
		seg := bytes.Clone(data[at:end])
		body := seg[4:]
		switch {
		case code == 0xE1 && bytes.HasPrefix(body, []byte("Exif\x00")):
			blankGPS(body)
			out = append(out, seg...)
		case code == 0xE1 && (bytes.HasPrefix(body, []byte(xmpJPEG)) || bytes.HasPrefix(body, []byte(xmpExtJPEG))):
			// dropped
		default:
			out = append(out, seg...)
		}
		at = end
	}
	return append(out, data[at:]...)
}

// stripPNG empties the GPS directory of the eXIf chunk, with its checksum
// written again, and drops the XMP chunk.
func stripPNG(data []byte) []byte {
	const sig = "\x89PNG\r\n\x1a\n"
	if !bytes.HasPrefix(data, []byte(sig)) {
		return data
	}
	out := bytes.Clone(data[:8])
	at := 8
	for range maxPNGChunks {
		if at+12 > len(data) {
			break
		}
		n := int(binary.BigEndian.Uint32(data[at:]))
		end := at + 12 + n
		if n < 0 || end > len(data) || end < at {
			break
		}
		chunk := bytes.Clone(data[at:end])
		kind := string(chunk[4:8])
		switch {
		case kind == "eXIf":
			blankGPS(chunk[8 : 8+n])
			binary.BigEndian.PutUint32(chunk[8+n:], crc32.ChecksumIEEE(chunk[4:8+n]))
			out = append(out, chunk...)
		case kind == "iTXt" && bytes.HasPrefix(chunk[8:8+n], []byte(xmpPNG)):
			// dropped
		default:
			out = append(out, chunk...)
		}
		at = end
		if kind == "IEND" {
			break
		}
	}
	return append(out, data[at:]...)
}

// stripWebP empties the GPS directory of the EXIF chunk and drops the XMP
// chunk, its flag in VP8X and the length of the container told again.
func stripWebP(data []byte) []byte {
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return data
	}
	out := bytes.Clone(data[:12])
	at := 12
	dropped := false
	for range maxWebPChunks {
		if at+8 > len(data) {
			break
		}
		size := int(binary.LittleEndian.Uint32(data[at+4:]))
		end := at + 8 + size + size&1
		if size < 0 || end > len(data) || end < at {
			break
		}
		chunk := bytes.Clone(data[at:end])
		switch string(chunk[:4]) {
		case "EXIF":
			blankGPS(chunk[8 : 8+size])
			out = append(out, chunk...)
		case "XMP ":
			dropped = true
		default:
			out = append(out, chunk...)
		}
		at = end
	}
	out = append(out, data[at:]...)
	if dropped {
		binary.LittleEndian.PutUint32(out[4:], uint32(len(out)-8))
		if len(out) >= 21 && string(out[12:16]) == "VP8X" {
			out[20] &^= 0x04 // the flag that says XMP follows
		}
	}
	return out
}
