package photo

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// maxEntries bounds the entries of one IFD; real files have a few dozen.
const maxEntries = 1000

// Tags read from the IFDs.
const (
	tagMake         = 0x010F
	tagModel        = 0x0110
	tagOrientation  = 0x0112
	tagDateTime     = 0x0132
	tagExifIFD      = 0x8769
	tagGPSIFD       = 0x8825
	tagExposure     = 0x829A
	tagFNumber      = 0x829D
	tagISO          = 0x8827
	tagISOSpeed     = 0x8833
	tagOriginal     = 0x9003
	tagDigitized    = 0x9004
	tagOffset       = 0x9010
	tagOffsetOrig   = 0x9011
	tagOffsetDigit  = 0x9012
	tagFocal        = 0x920A
	tagFocal35      = 0xA405
	tagLensModel    = 0xA434
	tagGPSLatRef    = 0x0001
	tagGPSLat       = 0x0002
	tagGPSLonRef    = 0x0003
	tagGPSLon       = 0x0004
	tagGPSAltRef    = 0x0005
	tagGPSAlt       = 0x0006
	typeByte        = 1
	typeASCII       = 2
	typeShort       = 3
	typeLong        = 4
	typeRational    = 5
	typeUndefined   = 7
	typeSRational   = 10
	typeIFD         = 13
	maxStringRunes  = 64
	maxOffsetHours  = 14
	maxAltitude     = 100000 // metres; anything higher is not a photo taken on earth
	minAltitude     = -12000 // metres; below the deepest trench
	maxExposureSecs = 1e6
)

// entry is one IFD entry: its type, its count and its value bytes, which lie
// inside the TIFF data.
type entry struct {
	typ uint16
	n   uint32
	val []byte
}

// ifd is one image file directory; the zero ifd has no entries.
type ifd struct {
	bo binary.ByteOrder
	m  map[uint16]entry
}

// typeSize is the size of one value of a TIFF type, 0 for unknown types.
func typeSize(t uint16) uint64 {
	switch t {
	case 1, 2, 6, 7: // BYTE, ASCII, SBYTE, UNDEFINED
		return 1
	case 3, 8: // SHORT, SSHORT
		return 2
	case 4, 9, 11, typeIFD: // LONG, SLONG, FLOAT, IFD
		return 4
	case 5, 10, 12: // RATIONAL, SRATIONAL, DOUBLE
		return 8
	}
	return 0
}

// parseTIFF reads IFD0 and the Exif and GPS IFDs it points to from a TIFF
// structure, tolerating a leading "Exif\0\0". What is invalid is left empty.
func parseTIFF(b []byte) (ifd0, exif, gps ifd) {
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
	if bo.Uint16(b[2:]) != 42 {
		return
	}
	seen := map[uint32]bool{}
	ifd0 = readIFD(b, bo, bo.Uint32(b[4:]), seen)
	if off, ok := ifd0.uint(tagExifIFD); ok {
		exif = readIFD(b, bo, off, seen)
	}
	if off, ok := ifd0.uint(tagGPSIFD); ok {
		gps = readIFD(b, bo, off, seen)
	}
	return
}

// readIFD reads the IFD at off. An offset already read (a loop) or outside
// the data gives the zero ifd; entries that do not fit are dropped.
func readIFD(b []byte, bo binary.ByteOrder, off uint32, seen map[uint32]bool) ifd {
	d := ifd{bo: bo}
	if off < 8 || seen[off] || uint64(off)+2 > uint64(len(b)) {
		return d
	}
	seen[off] = true
	n := int(bo.Uint16(b[off:]))
	if n > maxEntries {
		return d
	}
	if fit := (len(b) - int(off) - 2) / 12; n > fit {
		n = fit
	}
	d.m = make(map[uint16]entry, n)
	for i := 0; i < n; i++ {
		e := b[int(off)+2+12*i:][:12]
		tag, typ, count := bo.Uint16(e), bo.Uint16(e[2:]), bo.Uint32(e[4:])
		size := typeSize(typ) * uint64(count)
		if size == 0 {
			continue
		}
		var val []byte
		if size <= 4 {
			val = e[8 : 8+size]
		} else {
			o := uint64(bo.Uint32(e[8:]))
			if o+size > uint64(len(b)) {
				continue
			}
			val = b[o : o+size]
		}
		if _, dup := d.m[tag]; !dup {
			d.m[tag] = entry{typ: typ, n: count, val: val}
		}
	}
	return d
}

// uint is the first value of an unsigned integer entry.
func (d ifd) uint(tag uint16) (uint32, bool) {
	e, ok := d.m[tag]
	if !ok {
		return 0, false
	}
	switch e.typ {
	case typeByte:
		return uint32(e.val[0]), true
	case typeShort:
		return uint32(d.bo.Uint16(e.val)), true
	case typeLong, typeIFD:
		return d.bo.Uint32(e.val), true
	}
	return 0, false
}

// rat is the i-th value of a rational entry; a zero denominator is no value.
func (d ifd) rat(tag uint16, i int) (float64, bool) {
	e, ok := d.m[tag]
	if !ok || i < 0 || uint64(i) >= uint64(e.n) {
		return 0, false
	}
	v := e.val[8*i:]
	var num, den float64
	switch e.typ {
	case typeRational:
		num, den = float64(d.bo.Uint32(v)), float64(d.bo.Uint32(v[4:]))
	case typeSRational:
		num, den = float64(int32(d.bo.Uint32(v))), float64(int32(d.bo.Uint32(v[4:])))
	default:
		return 0, false
	}
	if den == 0 {
		return 0, false
	}
	return num / den, true // a ratio of 32-bit integers is always finite
}

// str is the text of an ASCII entry (also BYTE and UNDEFINED, which some
// writers use): leading NULs skipped, cut at the next NUL.
func (d ifd) str(tag uint16) []byte {
	e, ok := d.m[tag]
	if !ok || (e.typ != typeASCII && e.typ != typeByte && e.typ != typeUndefined) {
		return nil
	}
	s := bytes.TrimLeft(e.val, "\x00")
	if i := bytes.IndexByte(s, 0); i >= 0 {
		s = s[:i]
	}
	return s
}

// fromTIFF fills m with what the TIFF structure b tells.
func (m *Meta) fromTIFF(b []byte) {
	ifd0, exif, gps := parseTIFF(b)
	m.Make = clean(ifd0.str(tagMake))
	m.Model = clean(ifd0.str(tagModel))
	m.Lens = clean(exif.str(tagLensModel))
	if o, ok := ifd0.uint(tagOrientation); ok && o >= 1 && o <= 8 {
		m.Orientation = int(o)
	}

	times := []struct {
		d           ifd
		tag, offset uint16
	}{
		{exif, tagOriginal, tagOffsetOrig},
		{exif, tagDigitized, tagOffsetDigit},
		{ifd0, tagDateTime, tagOffset},
	}
	for _, c := range times {
		if t, ok := parseExifTime(c.d.str(c.tag)); ok {
			m.Taken = t
			off := exif.str(c.offset)
			if off == nil {
				off = ifd0.str(c.offset)
			}
			m.Offset, _, _ = parseOffset(string(off))
			break
		}
	}

	if v, ok := exif.rat(tagExposure, 0); ok {
		m.Exposure = formatExposure(v)
	}
	if v, ok := exif.rat(tagFNumber, 0); ok && v > 0 && v < 1000 {
		m.FNumber = math.Round(v*10) / 10
	}
	if v, ok := exif.rat(tagFocal, 0); ok && v > 0 && v < 100000 {
		m.Focal = math.Round(v*10) / 10
	}
	if v, ok := exif.uint(tagFocal35); ok && v > 0 && v < 100000 {
		m.Focal35 = int(v)
	}
	iso, _ := exif.uint(tagISO)
	if iso == 0 || iso == 65535 { // 65535: the real value is in ISOSpeed (EXIF 2.3)
		if v, ok := exif.uint(tagISOSpeed); ok && v > 0 && v <= 10000000 {
			iso = v
		}
	}
	if iso > 0 && iso <= 10000000 {
		m.ISO = int(iso)
	}

	lat, okLat := coordinate(gps, tagGPSLatRef, tagGPSLat, 'N', 'S', 90)
	lon, okLon := coordinate(gps, tagGPSLonRef, tagGPSLon, 'E', 'W', 180)
	if okLat && okLon && (lat != 0 || lon != 0) {
		lat, lon = round(lat, 1e7), round(lon, 1e7)
		m.Lat, m.Lon = &lat, &lon
	}
	if alt, ok := gps.rat(tagGPSAlt, 0); ok {
		if ref, _ := gps.uint(tagGPSAltRef); ref == 1 {
			alt = -alt
		}
		if alt >= minAltitude && alt <= maxAltitude {
			alt = round(alt, 100)
			m.Alt = &alt
		}
	}
}

func round(v, scale float64) float64 { return math.Round(v*scale) / scale }

// coordinate reads a GPS latitude or longitude: degrees, minutes and seconds
// (fewer values are taken as the leading ones) and a reference that gives the
// sign. Without a valid reference the sign is unknown, so there is no value.
func coordinate(gps ifd, refTag, valTag uint16, pos, neg byte, limit float64) (float64, bool) {
	ref := gps.str(refTag)
	if len(ref) == 0 {
		return 0, false
	}
	var sign float64
	switch ref[0] | 0x20 { // lower case
	case pos | 0x20:
		sign = 1
	case neg | 0x20:
		sign = -1
	default:
		return 0, false
	}
	e, ok := gps.m[valTag]
	if !ok || e.n == 0 {
		return 0, false
	}
	n := int(min(e.n, 3))
	var v float64
	for i, div := range []float64{1, 60, 3600}[:n] {
		x, ok := gps.rat(valTag, i)
		if !ok || x < 0 {
			return 0, false
		}
		v += x / div
	}
	if v > limit {
		return 0, false
	}
	return sign * v, true
}

// formatExposure writes an exposure time in seconds as photographers do:
// "1/250" below a second, else "2.5" or "30".
func formatExposure(v float64) string {
	if !(v > 0) || v >= maxExposureSecs {
		return ""
	}
	if v < 1 {
		if d := math.Round(1 / v); d >= 2 {
			return "1/" + strconv.FormatFloat(d, 'f', 0, 64)
		}
	}
	return strconv.FormatFloat(round(v, 10), 'f', -1, 64)
}

// clean makes a printable, trimmed string of at most maxStringRunes runes.
// Bytes that are not UTF-8 are taken as Latin-1; whitespace runs become one
// space; other unprintable runes are dropped.
func clean(b []byte) string {
	out := make([]rune, 0, maxStringRunes)
	space := false
loop:
	for len(b) > 0 && len(out) < maxStringRunes {
		r, size := utf8.DecodeRune(b)
		if r == utf8.RuneError && size == 1 {
			r = rune(b[0])
		}
		b = b[size:]
		switch {
		case unicode.IsSpace(r):
			space = len(out) > 0
		case unicode.IsPrint(r):
			if space {
				out = append(out, ' ')
				space = false
				if len(out) == maxStringRunes {
					break loop
				}
			}
			out = append(out, r)
		}
	}
	return strings.TrimRight(string(out), " ")
}

// parseExifTime reads "2006:01:02 15:04:05" (also with "-" or "/" in the
// date and "T" before the time; anything after the seconds is ignored) and
// returns it as "2006-01-02T15:04:05". Impossible dates and years outside
// 1800..2200 are refused, as are the blanks and zeros cameras write when the
// clock was not set.
func parseExifTime(b []byte) (string, bool) {
	s := strings.TrimSpace(string(b))
	if len(s) < 19 {
		return "", false
	}
	num := func(i, n int) int {
		v := 0
		for _, c := range []byte(s[i : i+n]) {
			if c < '0' || c > '9' {
				return -1
			}
			v = v*10 + int(c-'0')
		}
		return v
	}
	dateSep := func(c byte) bool { return c == ':' || c == '-' || c == '/' }
	if !dateSep(s[4]) || !dateSep(s[7]) || (s[10] != ' ' && s[10] != 'T') || s[13] != ':' || s[16] != ':' {
		return "", false
	}
	y, mo, d, h, mi, se := num(0, 4), num(5, 2), num(8, 2), num(11, 2), num(14, 2), num(17, 2)
	if y < 1800 || y > 2200 || mo < 1 || mo > 12 || d < 1 || h < 0 || h > 23 || mi < 0 || mi > 59 || se < 0 || se > 59 {
		return "", false
	}
	if d > daysIn(y, mo) {
		return "", false
	}
	return fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02d", y, mo, d, h, mi, se), true
}

// daysIn is the number of days of a month of the proleptic Gregorian calendar.
func daysIn(y, m int) int {
	switch m {
	case 2:
		if y%4 == 0 && (y%100 != 0 || y%400 == 0) {
			return 29
		}
		return 28
	case 4, 6, 9, 11:
		return 30
	}
	return 31
}

// parseOffset reads a UTC offset "+02:00" (also "+0200") and returns it as
// "+02:00" with its seconds east of UTC; "-00:00" becomes "+00:00".
func parseOffset(s string) (string, int, bool) {
	s = strings.TrimSpace(s)
	if len(s) == 5 {
		s = s[:3] + ":" + s[3:]
	}
	if len(s) != 6 || (s[0] != '+' && s[0] != '-') || s[3] != ':' {
		return "", 0, false
	}
	h, err1 := strconv.ParseUint(s[1:3], 10, 8)
	m, err2 := strconv.ParseUint(s[4:6], 10, 8)
	if err1 != nil || err2 != nil || h > maxOffsetHours || m > 59 {
		return "", 0, false
	}
	secs := int(h*3600 + m*60)
	sign := s[0]
	if secs == 0 {
		sign = '+'
	}
	if sign == '-' {
		secs = -secs
	}
	return fmt.Sprintf("%c%02d:%02d", sign, h, m), secs, true
}
