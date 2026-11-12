package photo

import (
	"bytes"
	"testing"
)

// ebml makes an EBML element: its ID as written, then its size as a
// vint of eight bytes, then its content.
func ebml(id []byte, content ...[]byte) []byte {
	c := bytes.Join(content, nil)
	size := []byte{0x01, 0, 0, 0, 0, 0, 0, 0}
	n := uint64(len(c))
	for i := 7; i > 0; i-- {
		size[i] = byte(n)
		n >>= 8
	}
	return append(append(append([]byte{}, id...), size...), c...)
}

// unknownSize makes an EBML element whose size is unknown, as a recorder
// writes its Segment and Clusters while it records.
func unknownSize(id []byte, content ...[]byte) []byte {
	return append(append(append([]byte{}, id...), 0x01, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF), bytes.Join(content, nil)...)
}

var (
	idEBML       = []byte{0x1A, 0x45, 0xDF, 0xA3}
	idDocType    = []byte{0x42, 0x82}
	idSegment    = []byte{0x18, 0x53, 0x80, 0x67}
	idInfo       = []byte{0x15, 0x49, 0xA9, 0x66}
	idTracks     = []byte{0x16, 0x54, 0xAE, 0x6B}
	idTrackEntry = []byte{0xAE}
	idTrackType  = []byte{0x83}
	idCodecID    = []byte{0x86}
	idCluster    = []byte{0x1F, 0x43, 0xB6, 0x75}
)

// webm makes a WebM file whose tracks are of the given types (1 video,
// 2 audio), the Segment of unknown size as a recorder writes it.
func webm(types ...byte) []byte {
	var entries [][]byte
	for _, t := range types {
		codec := "A_OPUS"
		if t == 1 {
			codec = "V_VP8"
		}
		entries = append(entries, ebml(idTrackEntry, ebml(idTrackType, []byte{t}), ebml(idCodecID, []byte(codec))))
	}
	return append(ebml(idEBML, ebml(idDocType, []byte("webm"))),
		unknownSize(idSegment, ebml(idInfo, make([]byte, 20)), ebml(idTracks, entries...), unknownSize(idCluster, make([]byte, 64)))...)
}

// hdlr is a handler box of the kind given ("soun", "vide").
func hdlr(kind string) []byte { return mbox("hdlr", be32(0), be32(0), []byte(kind), make([]byte, 13)) }

// mp4 makes an MP4 file of the brands given whose tracks have the
// handlers given.
func mp4(brands []string, handlers ...string) []byte {
	ftyp := []byte(brands[0])
	ftyp = append(ftyp, be32(0)...)
	for _, b := range brands {
		ftyp = append(ftyp, b...)
	}
	var traks [][]byte
	for _, h := range handlers {
		traks = append(traks, mbox("trak", tkhd(1<<16, 0, 0, 1<<16, 0, 0), mbox("mdia", mbox("mdhd", make([]byte, 24)), hdlr(h))))
	}
	return append(append(mbox("ftyp", ftyp), mbox("moov", append([][]byte{mvhd(0, 1000, 5000)}, traks...)...)...), mbox("mdat", make([]byte, 32))...)
}

// ogg makes the first page of an Ogg stream whose first packet starts
// with id.
func ogg(id string) []byte {
	page := append([]byte("OggS"), 0, 2)
	page = append(page, make([]byte, 20)...)
	page = append(page, 1, byte(len(id)+11))
	return append(append(page, id...), make([]byte, 11)...)
}

func TestAudioType(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"a WebM recording of sound", webm(2), TypeAudioWebM},
		{"a WebM of two sound tracks", webm(2, 2), TypeAudioWebM},
		{"a WebM video with sound", webm(1, 2), ""},
		{"a WebM video without sound", webm(1), ""},
		{"a WebM without tracks", webm(), ""},
		{"an M4A file", mp4([]string{"M4A ", "mp42", "isom"}, "soun"), TypeAudioMP4},
		{"an MP4 recording of sound", mp4([]string{"iso5", "iso6", "mp41"}, "soun"), TypeAudioMP4},
		{"an MP4 video with sound", mp4([]string{"isom", "mp42"}, "vide", "soun"), ""},
		{"an MP4 without tracks", mp4([]string{"isom"}), ""},
		{"an Ogg stream of Opus", ogg("OpusHead"), TypeAudioOgg},
		{"an Ogg stream of Vorbis", ogg("\x01vorbis"), TypeAudioOgg},
		{"an Ogg stream of Theora", ogg("\x80theora"), ""},
		{"an MP3 file, named already by the standard detection", append([]byte("ID3\x04\x00"), make([]byte, 64)...), ""},
		{"a picture", append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 64)...), ""},
		{"nothing", nil, ""},
	}
	for _, c := range cases {
		if got := AudioType(bytes.NewReader(c.data)); got != c.want {
			t.Errorf("%s: AudioType = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestAudioTypeIsNoVideo(t *testing.T) {
	// What the recorder makes is not taken into the album as a video.
	for _, mt := range []string{TypeAudioWebM, TypeAudioOgg, TypeAudioMP4} {
		if IsVideo(mt) {
			t.Errorf("IsVideo(%q) = true", mt)
		}
	}
}

func TestAudioTypeOfBrokenFiles(t *testing.T) {
	// Cut short anywhere, a file is told as sound only where what was read
	// says so, and never makes the reader fail.
	for _, data := range [][]byte{webm(2), webm(1, 2), mp4([]string{"M4A "}, "soun"), mp4([]string{"isom"}, "vide")} {
		for n := range len(data) {
			_ = AudioType(bytes.NewReader(data[:n]))
		}
	}
	// A track of sound found after the first cluster is not looked for:
	// the reader stops where the media begin.
	late := append(ebml(idEBML, ebml(idDocType, []byte("webm"))),
		unknownSize(idSegment, unknownSize(idCluster, make([]byte, 16)))...)
	if got := AudioType(bytes.NewReader(late)); got != "" {
		t.Errorf("a WebM without tracks before its media: %q", got)
	}
	// A size beyond the file ends the walk.
	huge := append(ebml(idEBML, ebml(idDocType, []byte("webm"))), append(idSegment, 0x01, 0, 0, 0, 0xFF, 0xFF, 0xFF, 0xF0)...)
	if got := AudioType(bytes.NewReader(huge)); got != "" {
		t.Errorf("a WebM whose segment is larger than the file: %q", got)
	}
}
