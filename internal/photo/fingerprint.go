package photo

import (
	"image"
	"image/draw"
	"io"
	"math/bits"
	"strconv"
)

// A fingerprint tells pictures that look the same apart from others
// (ADR-0195): a difference hash of the picture turned upright. The picture
// is reduced by area averaging to a grid of 9×8 cells of grey, and each of
// the 64 bits says whether a cell is brighter than its right neighbour. It
// survives scaling, a new encoding and a removed place, all of which change
// the file's bytes; two pictures whose fingerprints differ in at most
// SimilarBits bits look the same.
//
// The album's browser code computes the same grid from the pixels it
// decodes (assets/fingerprint.js); the shared test vector in
// internal/web/jstest/fingerprint-vector.json holds both to it.
const (
	gridCols = 9
	gridRows = 8
	// SimilarBits is how many bits two fingerprints may differ in for
	// their pictures to look the same.
	SimilarBits = 5
	// minContrast is the least difference of grey, out of 255, between the
	// grid's brightest and darkest cell: a flatter picture (one colour, an
	// empty page) gets no fingerprint, since its bits are noise.
	minContrast = 3.0
)

// Fingerprint decodes a JPEG, PNG or GIF as Thumbnail does, refusing more
// than maxPixels pixels, and gives the fingerprint of the picture turned
// upright by the EXIF orientation 1..8. It gives "" without an error for a
// picture too small or too flat to have one.
func Fingerprint(r io.Reader, mediaType string, orientation, maxPixels int) (string, error) {
	img, err := decodePicture(r, mediaType, maxPixels)
	if err != nil {
		return "", err
	}
	return FingerprintOf(img, orientation), nil
}

// FingerprintOf is the fingerprint of a decoded picture turned upright by
// orientation, or "" when it is narrower or lower than the grid or flat.
func FingerprintOf(img image.Image, orientation int) string {
	cols, rows := gridCols, gridRows
	turned := orientation >= 5 && orientation <= 8
	if turned {
		cols, rows = rows, cols
	}
	b := img.Bounds()
	if b.Dx() < cols || b.Dy() < rows {
		return ""
	}
	g := greyGrid(img, cols, rows)
	if orientation >= 2 && orientation <= 8 {
		g = orientGrid(g, cols, rows, orientation)
	}
	return gridFingerprint(g)
}

// greyGrid reduces src to cols×rows cells of grey by area averaging, one
// source row at a time, in premultiplied alpha as image.RGBA holds it: a
// transparent pixel counts as black. Grey is 0.299 R + 0.587 G + 0.114 B.
func greyGrid(src image.Image, cols, rows int) []float64 {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	xi, xw := spans64(sw, cols)
	yi, yw := spans64(sh, rows)
	row := image.NewRGBA(image.Rect(0, 0, sw, 1))
	line := make([]float64, cols)
	g := make([]float64, cols*rows)
	for y := range sh {
		draw.Draw(row, row.Rect, src, image.Pt(b.Min.X, b.Min.Y+y), draw.Src)
		clear(line)
		for x := range sw {
			p := row.Pix[4*x : 4*x+3 : 4*x+3]
			v := 0.299*float64(p[0]) + 0.587*float64(p[1]) + 0.114*float64(p[2])
			d := xi[x]
			line[d] += v * xw[x]
			if xw[x] != 1 {
				line[d+1] += v * (1 - xw[x])
			}
		}
		d := yi[y]
		for c, v := range line {
			g[d*cols+c] += v * yw[y]
			if yw[y] != 1 {
				g[(d+1)*cols+c] += v * (1 - yw[y])
			}
		}
	}
	norm := float64(cols) * float64(rows) / (float64(sw) * float64(sh))
	for i := range g {
		g[i] *= norm
	}
	return g
}

// spans64 is spans with the shares in float64, which the browser's numbers
// are too.
func spans64(n, m int) (idx []int, weight []float64) {
	idx, weight = make([]int, n), make([]float64, n)
	N, M := int64(n), int64(m)
	for j := range N {
		d := j * M / N
		idx[j] = int(d)
		weight[j] = 1
		if b := (d + 1) * N; (j+1)*M > b {
			weight[j] = float64(b-j*M) / float64(M)
		}
	}
	return idx, weight
}

// orientGrid turns a W×H grid upright as orient turns a picture.
func orientGrid(g []float64, W, H, o int) []float64 {
	dw := W
	if o >= 5 {
		dw = H
	}
	out := make([]float64, len(g))
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
			out[dy*dw+dx] = g[y*W+x]
		}
	}
	return out
}

// gridFingerprint writes the bits of an upright 9×8 grid as 16 hexadecimal
// digits, the first cell's bit highest, or "" for a flat grid.
func gridFingerprint(g []float64) string {
	lo, hi := g[0], g[0]
	for _, v := range g {
		lo, hi = min(lo, v), max(hi, v)
	}
	if hi-lo < minContrast {
		return ""
	}
	var h uint64
	for r := range gridRows {
		for c := range gridCols - 1 {
			h <<= 1
			if g[r*gridCols+c] > g[r*gridCols+c+1] {
				h |= 1
			}
		}
	}
	s := strconv.FormatUint(h, 16)
	for len(s) < 16 {
		s = "0" + s
	}
	return s
}

// ValidFingerprint reports whether s has the form of a fingerprint.
func ValidFingerprint(s string) bool {
	if len(s) != 16 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// Distance is how many bits two fingerprints differ in, or -1 when either
// is no fingerprint.
func Distance(a, b string) int {
	if !ValidFingerprint(a) || !ValidFingerprint(b) {
		return -1
	}
	x, _ := strconv.ParseUint(a, 16, 64)
	y, _ := strconv.ParseUint(b, 16, 64)
	return bits.OnesCount64(x ^ y)
}

// LookAlike reports whether two fingerprints are those of pictures that
// look the same.
func LookAlike(a, b string) bool {
	d := Distance(a, b)
	return d >= 0 && d <= SimilarBits
}
