package photo

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"
)

// mbox makes a box of an ISO base media file.
func mbox(kind string, content ...[]byte) []byte {
	c := bytes.Join(content, nil)
	b := binary.BigEndian.AppendUint32(nil, uint32(8+len(c)))
	return append(append(b, kind...), c...)
}

func be32(v uint32) []byte { return binary.BigEndian.AppendUint32(nil, v) }

// mvhd is a movie header, version 0: created (seconds since 1904), scale
// and duration.
func mvhd(created, scale, duration uint32) []byte {
	c := append(be32(0), be32(created)...)
	c = append(c, be32(created)...)
	c = append(c, be32(scale)...)
	c = append(c, be32(duration)...)
	return mbox("mvhd", c, make([]byte, 80))
}

// tkhd is a track header, version 0, with a matrix of a, b, c, d and a size.
func tkhd(a, b, c, d int32, w, h uint32) []byte {
	head := make([]byte, 40)
	mx := make([]byte, 36)
	for i, v := range []int32{a, b, 0, c, d, 0, 0, 0, 1 << 30} {
		binary.BigEndian.PutUint32(mx[i*4:], uint32(v))
	}
	return mbox("tkhd", head, mx, be32(w<<16), be32(h<<16))
}

func xyz(place string) []byte {
	c := binary.BigEndian.AppendUint16(nil, uint16(len(place)))
	return mbox("\xa9xyz", c, []byte{0x15, 0xc7}, []byte(place))
}

// appleMeta is Apple's metadata: keys and their values.
func appleMeta(kv ...string) []byte {
	keys := append(be32(0), be32(uint32(len(kv)/2))...)
	var items []byte
	for i := 0; i < len(kv); i += 2 {
		keys = append(keys, mbox("mdta", []byte(kv[i]))...)
		data := mbox("data", be32(1), be32(0), []byte(kv[i+1]))
		items = append(items, mbox(string(be32(uint32(i/2+1))), data)...)
	}
	hdlr := mbox("hdlr", be32(0), be32(0), []byte("mdta"), make([]byte, 13))
	return mbox("meta", hdlr, mbox("keys", keys), mbox("ilst", items))
}

func ftyp(major string, compat ...string) []byte {
	c := append([]byte(major), be32(0)...)
	for _, b := range compat {
		c = append(c, b...)
	}
	return mbox("ftyp", c)
}

func TestTheKindOfAVideoIsToldByItsBrands(t *testing.T) {
	webm := append([]byte{0x1A, 0x45, 0xDF, 0xA3, 0x9F, 0x42, 0x86, 0x81, 0x01}, []byte("\x42\x82\x84webm")...)
	for name, c := range map[string]struct {
		head []byte
		want string
	}{
		"iPhone MOV":         {ftyp("qt  ", "qt  "), TypeQuickTime},
		"isom with mp41":     {ftyp("isom", "isom", "iso2", "avc1", "mp41"), TypeMP4},
		"isom alone":         {ftyp("isom", "isom"), TypeMP4},
		"mp42":               {ftyp("mp42", "mp42", "isom"), TypeMP4},
		"an old QuickTime":   {mbox("moov", mvhd(0, 600, 600)), TypeQuickTime},
		"WebM":               {webm, TypeWebM},
		"M4A sound":          {ftyp("M4A ", "M4A ", "mp42"[:0]), ""},
		"HEIF still picture": {ftyp("heic", "mif1", "heic"), ""},
		"JPEG":               {[]byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00"), ""},
		"too short":          {[]byte("ftyp"), ""},
	} {
		if got := VideoType(c.head); got != c.want {
			t.Errorf("%s: %q, want %q", name, got, c.want)
		}
	}
	if !IsVideo("video/quicktime") || !IsVideo("Video/MP4; codecs=avc1") || IsVideo("image/png") {
		t.Error("IsVideo")
	}
}

// What a video tells about itself is read from its movie header, also
// when the header follows gigabytes of media data, as a camera writes it.
func TestAVideoTellsWhenWhereAndHowLong(t *testing.T) {
	const created = 3797407330 // 2024-05-01 11:22:10 UTC, seconds since 1904
	android := bytes.Join([][]byte{
		ftyp("isom", "isom", "mp42"),
		mbox("mdat", make([]byte, 4096)),
		mbox("moov", mvhd(created, 1000, 12345), mbox("trak", tkhd(0, 1<<16, -1<<16, 0, 1920, 1080)),
			mbox("udta", xyz("+46.5584+007.8352+1200.000/"))),
	}, nil)
	m := ReadVideo(bytes.NewReader(android))
	if m.Taken != "2024-05-01T11:22:10" || m.Offset != "+00:00" || m.Duration != 12.345 || m.Width != 1920 || m.Height != 1080 || m.Orientation != 6 {
		t.Fatalf("android %+v", m)
	}
	if m.Lat == nil || *m.Lat != 46.5584 || *m.Lon != 7.8352 || m.Alt == nil || *m.Alt != 1200 {
		t.Fatalf("place %v %v %v", m.Lat, m.Lon, m.Alt)
	}

	iphone := bytes.Join([][]byte{
		ftyp("qt  ", "qt  "),
		mbox("wide"),
		mbox("moov", mvhd(created, 600, 3000), mbox("trak", tkhd(1<<16, 0, 0, 1<<16, 3840, 2160)),
			mbox("trak", tkhd(1<<16, 0, 0, 1<<16, 0, 0)),
			appleMeta("com.apple.quicktime.location.ISO6709", "-33.8688+151.2093+012.345/",
				"com.apple.quicktime.make", "Apple", "com.apple.quicktime.model", "iPhone 15",
				"com.apple.quicktime.creationdate", "2024-05-01T13:22:10+0200")),
		mbox("mdat", make([]byte, 100)),
	}, nil)
	m = ReadVideo(bytes.NewReader(iphone))
	if m.Taken != "2024-05-01T13:22:10" || m.Offset != "+02:00" || m.Duration != 5 || m.Width != 3840 || m.Orientation != 0 || m.Make != "Apple" || m.Model != "iPhone 15" {
		t.Fatalf("iphone %+v", m)
	}
	if m.Lat == nil || *m.Lat != -33.8688 || *m.Lon != 151.2093 {
		t.Fatalf("iphone place %v %v", m.Lat, m.Lon)
	}

	// Damaged files are read as far as they go, never panic.
	for _, b := range [][]byte{nil, android[:10], android[:len(android)-7], mbox("moov", []byte{0, 0, 0, 1, 'x', 'x', 'x', 'x'})} {
		_ = ReadVideo(bytes.NewReader(b))
	}
}

// A video is given without its place by reading the texts that hold it as
// zeros: the size stays, the rest is unchanged, and every part reads so.
func TestAVideoWithoutItsPlace(t *testing.T) {
	video := bytes.Join([][]byte{
		ftyp("qt  ", "qt  "),
		mbox("moov", mvhd(3797407330, 600, 600), mbox("udta", xyz("+46.5584+007.8352/")),
			appleMeta("com.apple.quicktime.location.ISO6709", "+46.5584+007.8352+1200.000/")),
		mbox("mdat", bytes.Repeat([]byte{7}, 50)),
	}, nil)
	r, ok := VideoWithoutPlace(bytes.NewReader(video))
	if !ok {
		t.Fatal("refused")
	}
	got, err := io.ReadAll(r)
	if err != nil || len(got) != len(video) {
		t.Fatalf("read %d of %d: %v", len(got), len(video), err)
	}
	if bytes.Contains(got, []byte("46.5584")) || !bytes.HasSuffix(got, bytes.Repeat([]byte{7}, 50)) {
		t.Fatal("the place is still in the video, or the media changed")
	}
	if m := ReadVideo(bytes.NewReader(got)); m.Lat != nil || m.Taken == "" {
		t.Errorf("read again %+v", m)
	}
	// A part read from the middle is blanked as well.
	i := int64(bytes.Index(video, []byte("+46.5584+007.8352+1200")))
	if _, err := r.Seek(i-2, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	part := make([]byte, 10)
	io.ReadFull(r, part)
	if bytes.Contains(part, []byte("46.5")) {
		t.Errorf("a part %q", part)
	}
}

func FuzzReadVideo(f *testing.F) {
	f.Add(mbox("moov", mvhd(1, 1, 1), mbox("udta", xyz("+1+1/")), appleMeta("com.apple.quicktime.make", "x")))
	f.Fuzz(func(t *testing.T, b []byte) {
		_ = ReadVideo(bytes.NewReader(b))
		_, _ = VideoWithoutPlace(bytes.NewReader(b))
		_ = VideoType(b)
	})
}
