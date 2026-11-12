package photo

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
)

// scene is a w×h picture with soft shapes of light and shade, as a photo
// has, drawn from seed.
func scene(w, h int, seed uint64) *image.RGBA {
	r := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	type wave struct{ fx, fy, phase, amp float64 }
	var waves [4]wave
	for i := range waves {
		waves[i] = wave{r.Float64()*6 + 1, r.Float64()*6 + 1, r.Float64() * 2 * math.Pi, r.Float64()*50 + 20}
	}
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			v := 128.0
			for _, wv := range waves {
				v += wv.amp * math.Sin(wv.fx*float64(x)/float64(w)*math.Pi+wv.fy*float64(y)/float64(h)*math.Pi+wv.phase)
			}
			c := uint8(max(0, min(255, v)))
			m.SetRGBA(x, y, color.RGBA{c, uint8(int(c) * 3 / 4), 255 - c, 255})
		}
	}
	return m
}

func jpegAt(t *testing.T, m image.Image, quality int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, m, &jpeg.Options{Quality: quality}); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func mustFingerprint(t *testing.T, data []byte, typ string, orientation int) string {
	t.Helper()
	fp, err := Fingerprint(bytes.NewReader(data), typ, orientation, 1<<24)
	if err != nil {
		t.Fatal(err)
	}
	if !ValidFingerprint(fp) {
		t.Fatalf("fingerprint %q is not one", fp)
	}
	return fp
}

func TestFingerprintSurvivesANewEncodingAndScaling(t *testing.T) {
	big := scene(640, 480, 1)
	orig := mustFingerprint(t, jpegAt(t, big, 95), "image/jpeg", 1)
	again := mustFingerprint(t, jpegAt(t, big, 40), "image/jpeg", 1)
	small := mustFingerprint(t, jpegAt(t, scale(big, 160, 120), 80), "image/jpeg", 1)
	var p bytes.Buffer
	if err := png.Encode(&p, big); err != nil {
		t.Fatal(err)
	}
	asPNG := mustFingerprint(t, p.Bytes(), "image/png", 1)
	for name, fp := range map[string]string{"quality 40": again, "a quarter of the size": small, "PNG": asPNG} {
		if d := Distance(orig, fp); d > SimilarBits {
			t.Errorf("%s: %d bits apart (%s, %s), want at most %d", name, d, orig, fp, SimilarBits)
		}
	}
}

func TestFingerprintTellsOtherPicturesApart(t *testing.T) {
	a := mustFingerprint(t, jpegAt(t, scene(320, 240, 1), 90), "image/jpeg", 1)
	for seed := uint64(2); seed < 12; seed++ {
		b := mustFingerprint(t, jpegAt(t, scene(320, 240, seed), 90), "image/jpeg", 1)
		if LookAlike(a, b) {
			t.Errorf("scene %d looks like scene 1: %s, %s", seed, a, b)
		}
	}
	// The same picture turned or mirrored looks different.
	if b := FingerprintOf(scene(320, 240, 1), 2); LookAlike(a, b) {
		t.Errorf("a mirrored picture looks the same: %s, %s", a, b)
	}
}

func TestFingerprintIsOfThePictureTurnedUpright(t *testing.T) {
	stored := scene(300, 200, 7)
	for o := 2; o <= 8; o++ {
		got := FingerprintOf(stored, o)
		want := FingerprintOf(orient(stored, o), 1)
		if got != want {
			t.Errorf("orientation %d: %s, the upright picture %s", o, got, want)
		}
	}
	if FingerprintOf(stored, 9) != FingerprintOf(stored, 1) {
		t.Error("an unknown orientation should leave the picture as stored")
	}
}

func TestFlatAndTinyPicturesHaveNoFingerprint(t *testing.T) {
	flat := image.NewRGBA(image.Rect(0, 0, 100, 80))
	for i := range flat.Pix {
		flat.Pix[i] = 200
	}
	if fp := FingerprintOf(flat, 1); fp != "" {
		t.Errorf("a flat picture: %q, want none", fp)
	}
	if fp := FingerprintOf(scene(8, 40, 3), 1); fp != "" {
		t.Errorf("a picture narrower than the grid: %q, want none", fp)
	}
	if fp := FingerprintOf(scene(40, 8, 3), 6); fp != "" {
		t.Errorf("a picture lower than the turned grid: %q, want none", fp)
	}
	if fp := FingerprintOf(scene(9, 8, 3), 1); !ValidFingerprint(fp) {
		t.Errorf("a picture of the grid's size: %q, want one", fp)
	}
}

func TestFingerprintRefusesWhatItCannotDecode(t *testing.T) {
	if _, err := Fingerprint(bytes.NewReader([]byte("RIFF")), "image/webp", 1, 100); err == nil {
		t.Error("broken WebP: no error")
	}
	if _, err := Fingerprint(bytes.NewReader([]byte("no picture")), "image/heic", 1, 100); !errors.Is(err, ErrUnsupported) {
		t.Errorf("HEIC: %v, want ErrUnsupported", err)
	}
	if _, err := Fingerprint(bytes.NewReader(jpegAt(t, scene(100, 100, 1), 80)), "image/jpeg", 1, 99); !errors.Is(err, ErrTooLarge) {
		t.Errorf("too many pixels: %v, want ErrTooLarge", err)
	}
	if _, err := Fingerprint(bytes.NewReader([]byte("no picture")), "image/png", 1, 100); err == nil {
		t.Error("broken PNG: no error")
	}
}

func TestDistance(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0000000000000000", "0000000000000000", 0},
		{"0000000000000000", "ffffffffffffffff", 64},
		{"00000000000000ff", "0000000000000001", 7},
		{"", "0000000000000000", -1},
		{"000000000000000g", "0000000000000000", -1},
		{"0000000000000000", "00000000000000000", -1},
		{"ABCDEF0000000000", "abcdef0000000000", -1},
	}
	for _, c := range cases {
		if got := Distance(c.a, c.b); got != c.want {
			t.Errorf("Distance(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
	if !LookAlike("0000000000000000", "000000000000001f") || LookAlike("0000000000000000", "000000000000003f") {
		t.Errorf("LookAlike must allow %d bits and no more", SimilarBits)
	}
}

// vector is the test vector the browser's fingerprint is held to: an
// opaque picture as RGBA bytes and its fingerprint.
type vector struct {
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	RGBA        string `json:"rgba"`
	Fingerprint string `json:"fingerprint"`
}

// TestTheBrowserSharesTheFingerprint pins the vector that
// internal/web/jstest/fingerprint.test.mjs checks assets/fingerprint.js
// against; OT_UPDATE_VECTOR=1 writes it anew.
func TestTheBrowserSharesTheFingerprint(t *testing.T) {
	m := scene(45, 37, 42)
	path := filepath.Join("..", "web", "jstest", "fingerprint-vector.json")
	got := vector{Width: 45, Height: 37, RGBA: base64.StdEncoding.EncodeToString(m.Pix), Fingerprint: FingerprintOf(m, 1)}
	if os.Getenv("OT_UPDATE_VECTOR") == "1" {
		data, _ := json.MarshalIndent(got, "", "  ")
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with OT_UPDATE_VECTOR=1 to write it)", err)
	}
	var want vector
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("the fingerprint's vector changed: got %s, the file holds %s", got.Fingerprint, want.Fingerprint)
	}
}
