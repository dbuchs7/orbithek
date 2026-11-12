package photo

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math/rand/v2"
	"strings"
	"testing"
	"time"
)

var (
	red   = color.RGBA{255, 0, 0, 255}
	green = color.RGBA{0, 255, 0, 255}
	blue  = color.RGBA{0, 0, 255, 255}
	white = color.RGBA{255, 255, 255, 255}
)

// quadrants is a w×h picture whose quarters are red (top left), green (top
// right), blue (bottom left) and white (bottom right).
func quadrants(w, h int) *image.RGBA {
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			c := red
			switch {
			case x >= w/2 && y < h/2:
				c = green
			case x < w/2 && y >= h/2:
				c = blue
			case x >= w/2 && y >= h/2:
				c = white
			}
			m.SetRGBA(x, y, c)
		}
	}
	return m
}

func decode(t *testing.T, data []byte, contentType string) image.Image {
	t.Helper()
	var m image.Image
	var err error
	switch contentType {
	case "image/jpeg":
		m, err = jpeg.Decode(bytes.NewReader(data))
	case "image/png":
		m, err = png.Decode(bytes.NewReader(data))
	default:
		t.Fatalf("content type %q", contentType)
	}
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// near reports whether two colours differ by at most tol in every channel.
func near(a, b color.Color, tol int) bool {
	r1, g1, b1, a1 := a.RGBA()
	r2, g2, b2, a2 := b.RGBA()
	d := func(x, y uint32) bool { return abs(int(x>>8)-int(y>>8)) <= tol }
	return d(r1, r2) && d(g1, g2) && d(b1, b2) && d(a1, a2)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// TestThumbnailOrientations: for each EXIF orientation the stored corners
// land where the picture is upright, the sides swap for 5..8, and the longer
// edge is scaled to edge.
func TestThumbnailOrientations(t *testing.T) {
	src := encodePNG(t, quadrants(80, 40))
	// The colours at the upright picture's top left, top right, bottom left
	// and bottom right.
	want := map[int][4]color.RGBA{
		0: {red, green, blue, white}, // invalid: as stored
		1: {red, green, blue, white},
		2: {green, red, white, blue},
		3: {white, blue, green, red},
		4: {blue, white, red, green},
		5: {red, blue, green, white},
		6: {blue, red, white, green},
		7: {white, green, blue, red},
		8: {green, white, red, blue},
		9: {red, green, blue, white}, // invalid: as stored
	}
	for o, corners := range want {
		data, ct, err := Thumbnail(bytes.NewReader(src), "image/png", 20, o, 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		if ct != "image/jpeg" {
			t.Errorf("orientation %d: content type %q, want JPEG for an opaque picture", o, ct)
		}
		m := decode(t, data, ct)
		b := m.Bounds()
		ww, wh := 20, 10
		if o >= 5 && o <= 8 {
			ww, wh = 10, 20
		}
		if b.Dx() != ww || b.Dy() != wh {
			t.Fatalf("orientation %d: %dx%d, want %dx%d", o, b.Dx(), b.Dy(), ww, wh)
		}
		at := [4]image.Point{{1, 1}, {ww - 2, 1}, {1, wh - 2}, {ww - 2, wh - 2}}
		for i, p := range at {
			if got := m.At(p.X, p.Y); !near(got, corners[i], 40) {
				t.Errorf("orientation %d: corner %d is %v, want %v", o, i, got, corners[i])
			}
		}
	}
}

// TestThumbnailTransparency: a picture with transparent pixels stays PNG
// and keeps them transparent; colour is averaged in premultiplied alpha.
func TestThumbnailTransparency(t *testing.T) {
	m := image.NewNRGBA(image.Rect(0, 0, 40, 40))
	for y := range 40 {
		for x := range 40 {
			if x < 20 {
				m.SetNRGBA(x, y, color.NRGBA{255, 0, 0, 255})
			} else if (x+y)%2 == 0 {
				// Half transparent pixels with a colour that must not bleed.
				m.SetNRGBA(x, y, color.NRGBA{0, 255, 0, 0})
			} else {
				m.SetNRGBA(x, y, color.NRGBA{0, 0, 255, 255})
			}
		}
	}
	data, ct, err := Thumbnail(bytes.NewReader(encodePNG(t, m)), "image/png", 10, 1, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if ct != "image/png" {
		t.Fatalf("content type %q, want PNG", ct)
	}
	out := decode(t, data, ct)
	if c := color.NRGBAModel.Convert(out.At(1, 5)).(color.NRGBA); c != (color.NRGBA{255, 0, 0, 255}) {
		t.Errorf("opaque half: %v", c)
	}
	c := color.NRGBAModel.Convert(out.At(8, 5)).(color.NRGBA)
	if c.G > 2 || c.B < 250 || c.A < 120 || c.A > 135 {
		t.Errorf("half transparent blue: %v, want blue at alpha 128", c)
	}
}

// TestThumbnailGIF: a GIF's first frame, transparent where its palette
// says, placed on its screen.
func TestThumbnailGIF(t *testing.T) {
	pal := color.Palette{color.Transparent, red, green}
	frame := image.NewPaletted(image.Rect(10, 10, 30, 20), pal)
	for i := range frame.Pix {
		frame.Pix[i] = 1
	}
	second := image.NewPaletted(image.Rect(0, 0, 40, 40), pal)
	for i := range second.Pix {
		second.Pix[i] = 2
	}
	var b bytes.Buffer
	err := gif.EncodeAll(&b, &gif.GIF{
		Image:  []*image.Paletted{frame, second},
		Delay:  []int{0, 0},
		Config: image.Config{ColorModel: pal, Width: 40, Height: 40},
	})
	if err != nil {
		t.Fatal(err)
	}
	data, ct, err := Thumbnail(bytes.NewReader(b.Bytes()), "image/gif", 40, 1, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if ct != "image/png" {
		t.Fatalf("content type %q, want PNG", ct)
	}
	out := decode(t, data, ct)
	if out.Bounds().Dx() != 40 || out.Bounds().Dy() != 40 {
		t.Errorf("size %v, want the screen", out.Bounds())
	}
	if _, _, _, a := out.At(0, 0).RGBA(); a != 0 {
		t.Error("outside the first frame is not transparent")
	}
	if !near(out.At(15, 15), red, 0) {
		t.Errorf("inside the first frame: %v", out.At(15, 15))
	}

	// An opaque GIF becomes a JPEG.
	opaqueGIF := encodeGIF(t, quadrants(16, 16))
	if _, ct, err := Thumbnail(bytes.NewReader(opaqueGIF), "image/gif", 8, 1, 1<<20); err != nil || ct != "image/jpeg" {
		t.Errorf("opaque GIF: %q %v", ct, err)
	}
}

// TestThumbnailBoxFilter: area averaging turns a one-pixel checkerboard
// into grey at integer and fractional factors, and a picture smaller than
// edge keeps its size.
func TestThumbnailBoxFilter(t *testing.T) {
	m := image.NewGray(image.Rect(0, 0, 60, 30))
	for y := range 30 {
		for x := range 60 {
			if (x+y)%2 == 0 {
				m.SetGray(x, y, color.Gray{255})
			}
		}
	}
	src := encodePNG(t, m)
	// At factor 2 every target pixel averages two white and two black
	// pixels; at fractional factors the shares differ a little per pixel,
	// but the mean stays grey.
	for _, c := range []struct{ edge, tol int }{{30, 3}, {17, 12}, {40, 26}} {
		data, ct, err := Thumbnail(bytes.NewReader(src), "image/png", c.edge, 1, 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		out := decode(t, data, ct)
		b := out.Bounds()
		if b.Dx() != c.edge {
			t.Errorf("edge %d: width %d", c.edge, b.Dx())
		}
		sum, n := 0, 0
		for y := b.Min.Y + 1; y < b.Max.Y-1; y++ {
			for x := b.Min.X + 1; x < b.Max.X-1; x++ {
				if !near(out.At(x, y), color.Gray{128}, c.tol) {
					t.Fatalf("edge %d: (%d,%d) is %v, want grey", c.edge, x, y, out.At(x, y))
				}
				g := color.GrayModel.Convert(out.At(x, y)).(color.Gray)
				sum += int(g.Y)
				n++
			}
		}
		if mean := sum / n; abs(mean-128) > 3 {
			t.Errorf("edge %d: mean %d, want 128", c.edge, mean)
		}
	}
	data, ct, err := Thumbnail(bytes.NewReader(src), "image/png", 100, 6, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if b := decode(t, data, ct).Bounds(); b.Dx() != 30 || b.Dy() != 60 {
		t.Errorf("small picture: %v, want 30x60 upright and not enlarged", b)
	}
}

// TestScaleExact: the box filter itself is exact for a 3→2 reduction.
func TestScaleExact(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 3, 1))
	for x, v := range []uint8{0, 90, 180} {
		src.SetRGBA(x, 0, color.RGBA{v, v, v, 255})
	}
	out := scale(src, 2, 1)
	// Target 0 covers pixel 0 and half of pixel 1: (0 + 45) / 1.5 = 30.
	// Target 1 covers half of pixel 1 and pixel 2: (45 + 180) / 1.5 = 150.
	if got := [2]uint8{out.Pix[0], out.Pix[4]}; got != [2]uint8{30, 150} || out.Pix[3] != 255 {
		t.Errorf("got %v", out.Pix)
	}
	if w, h := fit(1000, 1, 10); w != 10 || h != 1 {
		t.Errorf("fit keeps at least one pixel: %dx%d", w, h)
	}
	if w, h := fit(1, 1000, 10); w != 1 || h != 10 {
		t.Errorf("fit keeps at least one pixel: %dx%d", w, h)
	}
}

// TestThumbnailFormats: JPEG in colour and grey, a sub-image's offset.
func TestThumbnailFormats(t *testing.T) {
	q := quadrants(64, 32)
	gray := image.NewGray(q.Bounds())
	draw.Draw(gray, gray.Bounds(), q, image.Point{}, draw.Src)
	pal := image.NewPaletted(q.Bounds(), palette.Plan9)
	draw.Draw(pal, pal.Bounds(), q, image.Point{}, draw.Src)
	for name, src := range map[string]struct {
		data []byte
		mt   string
	}{
		"jpeg colour": {encodeJPEG(t, q), "image/jpeg"},
		"jpeg grey":   {encodeJPEG(t, gray), "image/jpeg"},
		"png palette": {encodePNG(t, pal), "image/png"},
		"png 16 bit":  {encodePNG(t, image.NewRGBA64(image.Rect(0, 0, 9, 9))), "image/png"},
	} {
		t.Run(name, func(t *testing.T) {
			data, ct, err := Thumbnail(bytes.NewReader(src.data), src.mt, 16, 1, 1<<20)
			if err != nil {
				t.Fatal(err)
			}
			if len(data) == 0 || ct == "" {
				t.Fatal("empty thumbnail")
			}
			decode(t, data, ct)
		})
	}
}

// TestThumbnailRefusals: too many pixels before decoding, unsupported
// formats, bad arguments, broken and mislabelled input.
func TestThumbnailRefusals(t *testing.T) {
	png100 := encodePNG(t, testImage(100, 100))
	if _, _, err := Thumbnail(bytes.NewReader(png100), "image/png", 10, 1, 9999); !errors.Is(err, ErrTooLarge) {
		t.Errorf("too large: %v", err)
	}
	if _, _, err := Thumbnail(bytes.NewReader(png100), "image/png", 10, 1, 10000); err != nil {
		t.Errorf("exactly the limit: %v", err)
	}
	for _, mt := range []string{"image/bmp", "image/heic", ""} {
		if _, _, err := Thumbnail(bytes.NewReader(png100), mt, 10, 1, 1<<20); !errors.Is(err, ErrUnsupported) {
			t.Errorf("%q: %v", mt, err)
		}
	}
	for _, args := range [][2]int{{0, 100}, {-1, 100}, {10, 0}} {
		if _, _, err := Thumbnail(bytes.NewReader(png100), "image/png", args[0], 1, args[1]); err == nil {
			t.Errorf("edge %d, maxPixels %d: no error", args[0], args[1])
		}
	}
	if _, _, err := Thumbnail(nil, "image/png", 10, 1, 100); err == nil {
		t.Error("nil reader: no error")
	}
	for name, c := range map[string]struct {
		data []byte
		mt   string
	}{
		"garbage":        {[]byte("not a picture at all"), "image/jpeg"},
		"mislabelled":    {png100, "image/jpeg"},
		"cut after head": {png100[:100], "image/png"},
		"empty":          {nil, "image/gif"},
		"no pixels":      {zeroGIF(), "image/gif"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := Thumbnail(bytes.NewReader(c.data), c.mt, 10, 1, 1<<20); err == nil {
				t.Error("no error")
			}
		})
	}
}

// zeroGIF is a GIF header that claims a screen of 0×0 pixels.
func zeroGIF() []byte { return []byte("GIF89a\x00\x00\x00\x00\x00\x00\x00;") }

// TestThumbnailBomb: a small file that claims a huge picture is refused
// from its header, quickly and without allocating the picture.
func TestThumbnailBomb(t *testing.T) {
	// A PNG header claiming 10000×10000 pixels (larger claims the png
	// package itself refuses on 32-bit platforms).
	ihdr := []byte{0, 0, 0x27, 0x10, 0, 0, 0x27, 0x10, 8, 6, 0, 0, 0}
	bomb := append([]byte("\x89PNG\r\n\x1a\n"), chunk("IHDR", ihdr)...)
	bomb = append(bomb, chunk("IDAT", []byte(strings.Repeat("\x00", 64)))...)
	start := time.Now()
	if _, _, err := Thumbnail(bytes.NewReader(bomb), "image/png", 100, 1, 50_000_000); !errors.Is(err, ErrTooLarge) {
		t.Errorf("bomb: %v", err)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("refusing took %v", d)
	}
	// The same claim in a JPEG frame header (65535×65535).
	sof := segment(0xC0, []byte{8, 0xFF, 0xFF, 0xFF, 0xFF, 1, 1, 0x11, 0})
	jpg := append(append([]byte{0xFF, 0xD8}, sof...), segment(0xDA, []byte{1, 1, 0, 0, 63, 0})...)
	if _, _, err := Thumbnail(bytes.NewReader(jpg), "image/jpeg", 100, 1, 50_000_000); !errors.Is(err, ErrTooLarge) {
		t.Errorf("jpeg bomb: %v", err)
	}
}

// TestReadThenThumbnail: the orientation Read finds turns the thumbnail.
func TestReadThenThumbnail(t *testing.T) {
	tiff := buildTIFF(orders[1].bo, []tg{shortTag(tagOrientation, 8)}, nil, nil)
	f := jpegWith(encodeJPEG(t, quadrants(64, 32)), exifAPP1(tiff))
	m := Read(bytes.NewReader(f), "image/jpeg")
	if w, h := m.Upright(); w != 32 || h != 64 {
		t.Fatalf("upright %dx%d", w, h)
	}
	data, ct, err := Thumbnail(bytes.NewReader(f), "image/jpeg", 32, m.Orientation, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	out := decode(t, data, ct)
	if b := out.Bounds(); b.Dx() != 16 || b.Dy() != 32 {
		t.Errorf("size %v", b)
	}
	if !near(out.At(2, 2), green, 40) {
		t.Errorf("top left %v, want green", out.At(2, 2))
	}
}

// BenchmarkThumbnail: a 12 megapixel camera JPEG to a 400 pixel thumbnail,
// once smooth and once full of fine detail, which is slower to decode.
func BenchmarkThumbnail(b *testing.B) {
	noisy := testImage(4000, 3000)
	rng := rand.New(rand.NewPCG(3, 4))
	for i := range noisy.Pix {
		if i%4 != 3 {
			noisy.Pix[i] = uint8(int(noisy.Pix[i])/2 + rng.IntN(128))
		}
	}
	for name, img := range map[string]image.Image{"smooth": testImage(4000, 3000), "detailed": noisy} {
		src := encodeJPEG(b, img)
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(src)))
			for b.Loop() {
				if _, _, err := Thumbnail(bytes.NewReader(src), "image/jpeg", 400, 6, 50_000_000); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkScale: the box filter alone on the decoded 12 megapixel picture.
func BenchmarkScale(b *testing.B) {
	img, err := jpeg.Decode(bytes.NewReader(encodeJPEG(b, testImage(4000, 3000))))
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for b.Loop() {
		scale(img, 400, 300)
	}
}

// BenchmarkRead: the metadata of a camera JPEG.
func BenchmarkRead(b *testing.B) {
	f := jpegWith(encodeJPEG(b, testImage(64, 64)), exifAPP1(fullTIFF(orders[0].bo)))
	r := bytes.NewReader(f)
	for b.Loop() {
		Read(r, "image/jpeg")
	}
}

// tinyWebP is a WebP of one pixel, lossless.
const tinyWebP = "UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA=="

// TestThumbnailOfAWebP: a WebP is decoded and scaled as a JPEG or PNG is
// (ADR-0223); a broken one is refused.
func TestThumbnailOfAWebP(t *testing.T) {
	data, _ := base64.StdEncoding.DecodeString(tinyWebP)
	out, ct, err := Thumbnail(bytes.NewReader(data), "image/webp", 400, 1, 1<<20)
	if err != nil || (ct != "image/jpeg" && ct != "image/png") || len(out) == 0 {
		t.Fatalf("thumbnail of a WebP: %q %d bytes, %v", ct, len(out), err)
	}
	if _, _, err := Thumbnail(bytes.NewReader([]byte("RIFF....WEBP")), "image/webp", 400, 1, 1<<20); err == nil {
		t.Error("a broken WebP was scaled")
	}
}
