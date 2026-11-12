package photo

import (
	"bytes"
	"encoding/binary"
	"io"
)

// Sound recorded in the browser (ADR-0219):
// a recorder writes WebM or Ogg with Opus, or MP4 with AAC, and the
// standard detection names WebM and most MP4 files videos, whatever their
// tracks hold. AudioType reads the tracks, so a recording is told as sound
// and is not taken into the album as a video.

// The media types of sound alone in a container that also holds videos.
const (
	TypeAudioWebM = "audio/webm"
	TypeAudioOgg  = "audio/ogg"
	TypeAudioMP4  = "audio/mp4"
)

// Bounds on walking a WebM file's elements.
const (
	maxAudioElements = 10000
	maxAudioDepth    = 4
)

// The WebM (Matroska) elements read.
const (
	mkvSegment    = 0x18538067
	mkvTracks     = 0x1654AE6B
	mkvTrackEntry = 0xAE
	mkvTrackType  = 0x83
	mkvCluster    = 0x1F43B675
)

// AudioType tells whether a file is sound alone from its container: a
// WebM or MP4 file all of whose tracks are sound, or an Ogg stream of
// Opus or Vorbis; "" otherwise. MP3 and WAV, which the standard detection
// names already, answer "" too.
func AudioType(r io.ReadSeeker) string {
	size, err := r.Seek(0, io.SeekEnd)
	if err != nil || size < 12 {
		return ""
	}
	head := make([]byte, 64)
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return ""
	}
	n, _ := io.ReadFull(r, head)
	head = head[:n]
	switch {
	case bytes.HasPrefix(head, []byte("OggS")):
		// The first page holds the stream's first packet, which names
		// its codec.
		if bytes.Contains(head, []byte("OpusHead")) || bytes.Contains(head, []byte("\x01vorbis")) {
			return TypeAudioOgg
		}
	case VideoType(head) == TypeWebM:
		if soundOnly(webmTracks(r, size)) {
			return TypeAudioWebM
		}
	case string(head[4:8]) == "ftyp":
		if soundOnly(mp4Tracks(r, size)) {
			return TypeAudioMP4
		}
	}
	return ""
}

// soundOnly reports whether there are tracks and all are sound.
func soundOnly(kinds []string) bool {
	for _, k := range kinds {
		if k != "sound" {
			return false
		}
	}
	return len(kinds) > 0
}

// mp4Tracks names the kind of each track of an MP4 file by its handler:
// "sound", "video" or another handler's own name.
func mp4Tracks(r io.ReadSeeker, size int64) []string {
	var kinds []string
	w := &videoWalk{r: r, size: size}
	w.at = func(path string, b box) {
		if path != "moov/trak/mdia/hdlr" {
			return
		}
		c := w.content(b)
		if len(c) < 12 {
			kinds = append(kinds, "")
			return
		}
		switch k := string(c[8:12]); k {
		case "soun":
			kinds = append(kinds, "sound")
		case "vide":
			kinds = append(kinds, "video")
		default:
			kinds = append(kinds, k)
		}
	}
	w.children("", 0, size, 0)
	return kinds
}

// webmTracks names the kind of each track of a WebM file, read from its
// Tracks element; the walk stops at the first Cluster, where the media
// begin. A file whose tracks cannot be read has none.
func webmTracks(r io.ReadSeeker, size int64) []string {
	var kinds []string
	visits := 0
	ok := true
	var walk func(from, to int64, depth int, inEntry bool) bool
	// walk visits the elements between from and to; it answers false once
	// the walk is to end.
	walk = func(from, to int64, depth int, inEntry bool) bool {
		for off := from; off < to; {
			visits++
			if visits > maxAudioElements || depth > maxAudioDepth {
				ok = false
				return false
			}
			id, size, head, valid := ebmlElement(r, off)
			if !valid {
				ok = false
				return false
			}
			end := off + head + size
			if size < 0 { // unknown size: to the end of its parent
				end = to
			}
			if end > to {
				end = to
			}
			switch {
			case id == mkvCluster:
				return false
			case id == mkvSegment || id == mkvTracks:
				if !walk(off+head, end, depth+1, false) {
					return false
				}
			case id == mkvTrackEntry && !inEntry:
				before := len(kinds)
				if !walk(off+head, end, depth+1, true) {
					return false
				}
				if len(kinds) == before {
					kinds = append(kinds, "")
				}
			case id == mkvTrackType && inEntry:
				t := readUint(r, off+head, size)
				switch t {
				case 1:
					kinds = append(kinds, "video")
				case 2:
					kinds = append(kinds, "sound")
				default:
					kinds = append(kinds, "other")
				}
			}
			if size < 0 {
				return true
			}
			off = end
		}
		return true
	}
	walk(0, size, 0, false)
	if !ok {
		return nil
	}
	return kinds
}

// ebmlElement reads the head of the element at off: its ID (with its
// length marker, as specifications write it), its size (-1 when unknown)
// and the length of its head.
func ebmlElement(r io.ReadSeeker, off int64) (id uint32, size, head int64, ok bool) {
	if _, err := r.Seek(off, io.SeekStart); err != nil {
		return 0, 0, 0, false
	}
	var b [12]byte
	if !readFull(r, b[:1]) {
		return 0, 0, 0, false
	}
	idLen := vintLength(b[0])
	if idLen == 0 || idLen > 4 || !readFull(r, b[1:idLen]) {
		return 0, 0, 0, false
	}
	for _, c := range b[:idLen] {
		id = id<<8 | uint32(c)
	}
	if !readFull(r, b[idLen:idLen+1]) {
		return 0, 0, 0, false
	}
	sizeLen := vintLength(b[idLen])
	if sizeLen == 0 || !readFull(r, b[idLen+1:idLen+sizeLen]) {
		return 0, 0, 0, false
	}
	v := uint64(b[idLen]) & (0xFF >> sizeLen)
	unknown := v == 0xFF>>sizeLen
	for _, c := range b[idLen+1 : idLen+sizeLen] {
		v = v<<8 | uint64(c)
		unknown = unknown && c == 0xFF
	}
	if unknown {
		return id, -1, int64(idLen + sizeLen), true
	}
	if v > 1<<62 {
		return 0, 0, 0, false
	}
	return id, int64(v), int64(idLen + sizeLen), true
}

// vintLength is the length of a variable-length integer from its first
// byte, 0 when the byte starts none.
func vintLength(first byte) int {
	for n := 1; n <= 8; n++ {
		if first&(0x80>>(n-1)) != 0 {
			return n
		}
	}
	return 0
}

// readUint reads an unsigned integer of at most eight bytes.
func readUint(r io.ReadSeeker, off, size int64) uint64 {
	if size < 1 || size > 8 {
		return 0
	}
	if _, err := r.Seek(off, io.SeekStart); err != nil {
		return 0
	}
	var b [8]byte
	if !readFull(r, b[8-size:]) {
		return 0
	}
	return binary.BigEndian.Uint64(b[:])
}
