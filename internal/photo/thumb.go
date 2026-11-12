package photo

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"

	webpcodec "golang.org/x/image/webp"
)

var (
	// ErrUnsupported is returned for a format without a decoder in the
	// standard library, such as WebP.
	ErrUnsupported = errors.New("photo: no thumbnail for this format")
	// ErrTooLarge is returned when the header claims more pixels than allowed.
	ErrTooLarge = errors.New("photo: the picture has too many pixels")
)

// thumbQuality is the JPEG quality of thumbnails.
const thumbQuality = 82

// Thumbnail decodes a JPEG, PNG or GIF (its first frame), turns it upright by
// the EXIF orientation 1..8 (other values leave it as stored), scales it so
// its longer edge is at most edge pixels, never enlarging, by area averaging,
// and encodes it: as JPEG ("image/jpeg") when every pixel is opaque, else as
// PNG ("image/png") so that transparency survives.
//
// Before decoding, the size in the header is checked: a picture of more than
// maxPixels pixels is refused with ErrTooLarge. Formats without a decoder
// give ErrUnsupported.
func Thumbnail(r io.Reader, mediaType string, edge, orientation, maxPixels int) (data []byte, contentType string, err error) {
	if edge < 1 {
		return nil, "", fmt.Errorf("photo: edge (%d) must be positive", edge)
	}
	img, err := decodePicture(r, mediaType, maxPixels)
	if err != nil {
		return nil, "", err
	}
	b := img.Bounds()
	w, h := fit(b.Dx(), b.Dy(), edge)
	small := orient(scale(img, w, h), orientation)

	var buf bytes.Buffer
	if opaque(small) {
		contentType = "image/jpeg"
		err = jpeg.Encode(&buf, small, &jpeg.Options{Quality: thumbQuality})
	} else {
		contentType = "image/png"
		err = png.Encode(&buf, small)
	}
	if err != nil {
		return nil, "", fmt.Errorf("photo: %w", err)
	}
	return buf.Bytes(), contentType, nil
}

// decodePicture decodes a JPEG, PNG or GIF (its first frame, drawn on its whole
// screen) after checking the pixel count its header claims against
// maxPixels (ErrTooLarge); other formats give ErrUnsupported.
func decodePicture(r io.Reader, mediaType string, maxPixels int) (image.Image, error) {
	var decodeConfig func(io.Reader) (image.Config, error)
	var decodeImage func(io.Reader) (image.Image, error)
	switch normType(mediaType) {
	case "image/jpeg":
		decodeConfig, decodeImage = jpeg.DecodeConfig, jpeg.Decode
	case "image/png":
		decodeConfig, decodeImage = png.DecodeConfig, png.Decode
	case "image/gif":
		decodeConfig, decodeImage = gif.DecodeConfig, gif.Decode
	case "image/webp":
		// golang.org/x/image/webp, pure Go from the Go project
		// (ADR-draft-signed-links-to-a-shares-originals-and-previews-of-raw-and-webp).
		decodeConfig, decodeImage = webpcodec.DecodeConfig, webpcodec.Decode
	default:
		return nil, ErrUnsupported
	}
	if maxPixels < 1 {
		return nil, fmt.Errorf("photo: maxPixels (%d) must be positive", maxPixels)
	}
	if r == nil {
		return nil, errors.New("photo: no picture")
	}

	// The header is read through a copy, so the decoder reads it again
	// without reading the whole file first.
	var head bytes.Buffer
	cfg, err := decodeConfig(io.TeeReader(r, &head))
	if err != nil {
		return nil, fmt.Errorf("photo: %w", err)
	}
	if cfg.Width < 1 || cfg.Height < 1 {
		return nil, errors.New("photo: the picture has no pixels")
	}
	if int64(cfg.Width)*int64(cfg.Height) > int64(maxPixels) {
		return nil, ErrTooLarge
	}
	img, err := decodeImage(io.MultiReader(&head, r))
	if err != nil {
		return nil, fmt.Errorf("photo: %w", err)
	}
	if full := image.Rect(0, 0, cfg.Width, cfg.Height); img.Bounds() != full {
		// A GIF's first frame may cover part of its screen only.
		canvas := image.NewRGBA(full)
		draw.Draw(canvas, img.Bounds(), img, img.Bounds().Min, draw.Src)
		img = canvas
	}
	return img, nil
}

// fit is the size of w×h scaled so that its longer side is at most edge,
// never enlarged, each side at least one pixel.
func fit(w, h, edge int) (int, int) {
	if w <= edge && h <= edge {
		return w, h
	}
	long, short := int64(w), int64(h)
	if h > w {
		long, short = short, long
	}
	s := max(1, int((short*int64(edge)+long/2)/long))
	if w >= h {
		return edge, s
	}
	return s, edge
}

// spans maps each of n source pixels onto m ≤ n target pixels for area
// averaging: source pixel j covers the target index idx[j] with the share
// weight[j] of its area and the next target with the rest. Integer
// arithmetic keeps the indices exact.
func spans(n, m int) (idx []int32, weight []float32) {
	idx, weight = make([]int32, n), make([]float32, n)
	N, M := int64(n), int64(m)
	for j := range N {
		d := j * M / N
		idx[j] = int32(d)
		weight[j] = 1
		if b := (d + 1) * N; (j+1)*M > b { // straddles the boundary to d+1
			weight[j] = float32(b-j*M) / float32(M)
		}
	}
	return idx, weight
}

// scale reduces src to w×h (w and h no larger than src) by area averaging
// (a box filter with exact partial coverage). It converts one source row at
// a time, so a large picture never exists twice in full in memory.
// Averaging is done in premultiplied alpha, as image.RGBA stores it.
func scale(src image.Image, w, h int) *image.RGBA {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	xi, xw := spans(sw, w)
	yi, yw := spans(sh, h)
	row := image.NewRGBA(image.Rect(0, 0, sw, 1))
	hrow := make([]float32, 4*w)
	acc, next := make([]float32, 4*w), make([]float32, 4*w)
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	norm := float32(float64(w) * float64(h) / (float64(sw) * float64(sh)))
	cur := 0
	for y := range sh {
		draw.Draw(row, row.Rect, src, image.Pt(b.Min.X, b.Min.Y+y), draw.Src)
		clear(hrow)
		pix := row.Pix
		for x := range sw {
			p := pix[4*x : 4*x+4 : 4*x+4]
			d := 4 * int(xi[x])
			r, g, bl, a := float32(p[0]), float32(p[1]), float32(p[2]), float32(p[3])
			if wa := xw[x]; wa == 1 {
				o := hrow[d : d+4 : d+4]
				o[0] += r
				o[1] += g
				o[2] += bl
				o[3] += a
			} else {
				o := hrow[d : d+8 : d+8]
				wb := 1 - wa
				o[0] += r * wa
				o[1] += g * wa
				o[2] += bl * wa
				o[3] += a * wa
				o[4] += r * wb
				o[5] += g * wb
				o[6] += bl * wb
				o[7] += a * wb
			}
		}
		for d := int(yi[y]); d > cur; cur++ {
			emit(dst, cur, acc, norm)
			acc, next = next, acc
			clear(next)
		}
		wa := yw[y]
		for i, v := range hrow {
			acc[i] += v * wa
		}
		if wa != 1 {
			wb := 1 - wa
			for i, v := range hrow {
				next[i] += v * wb
			}
		}
	}
	for ; cur < h; cur++ {
		emit(dst, cur, acc, norm)
		acc, next = next, acc
		clear(next)
	}
	return dst
}

// emit writes the accumulated row y, scaled by norm, into dst, keeping every
// colour channel at most its alpha as premultiplied pixels require.
func emit(dst *image.RGBA, y int, acc []float32, norm float32) {
	out := dst.Pix[y*dst.Stride : y*dst.Stride+len(acc)]
	for i := 0; i < len(acc); i += 4 {
		a := to8(acc[i+3] * norm)
		out[i+3] = a
		out[i] = min(to8(acc[i]*norm), a)
		out[i+1] = min(to8(acc[i+1]*norm), a)
		out[i+2] = min(to8(acc[i+2]*norm), a)
	}
}

func to8(v float32) uint8 {
	v += 0.5
	switch {
	case v <= 0:
		return 0
	case v >= 255:
		return 255
	}
	return uint8(v)
}

// orient turns src upright for an EXIF orientation: 2 mirrors, 3 turns by
// 180°, 4 flips, 5 transposes, 6 turns 90° clockwise, 7 transverses and 8
// turns 90° counter-clockwise. Other values leave it as it is.
func orient(src *image.RGBA, o int) *image.RGBA {
	if o < 2 || o > 8 {
		return src
	}
	W, H := src.Rect.Dx(), src.Rect.Dy()
	dw, dh := W, H
	if o >= 5 {
		dw, dh = H, W
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := range H {
		for x := range W {
			var dx, dy int
			switch o {
			case 2:
				dx, dy = W-1-x, y
			case 3:
				dx, dy = W-1-x, H-1-y
			case 4:
				dx, dy = x, H-1-y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = H-1-y, x
			case 7:
				dx, dy = H-1-y, W-1-x
			case 8:
				dx, dy = y, W-1-x
			}
			s := y*src.Stride + 4*x
			d := dy*dst.Stride + 4*dx
			copy(dst.Pix[d:d+4], src.Pix[s:s+4])
		}
	}
	return dst
}

// opaque reports whether every pixel of m is fully opaque.
func opaque(m *image.RGBA) bool {
	for i := 3; i < len(m.Pix); i += 4 {
		if m.Pix[i] != 0xFF {
			return false
		}
	}
	return true
}
