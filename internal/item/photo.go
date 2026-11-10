package item

import (
	"math"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Photo is what a picture tells about itself (ADR-0176): when it was
// taken, where, and with what. The server reads it from the file when the
// file is stored (EXIF), or takes it from the service a picture is
// imported from. It is content, sealed with the rest of the attachment and
// erased with it: a place where a photo was taken can be as personal as
// the photo.
type Photo struct {
	// Taken is the wall-clock time of capture, "2006-01-02T15:04:05";
	// Offset its offset from UTC, "+02:00", when the picture tells it.
	Taken  string `json:"taken,omitempty"`
	Offset string `json:"offset,omitempty"`
	// Lat and Lon are the place in decimal degrees, Alt the height in
	// metres; absent when the picture tells no place.
	Lat *float64 `json:"lat,omitempty"`
	Lon *float64 `json:"lon,omitempty"`
	Alt *float64 `json:"alt,omitempty"`
	// Make, Model and Lens name the camera.
	Make  string `json:"make,omitempty"`
	Model string `json:"model,omitempty"`
	Lens  string `json:"lens,omitempty"`
	// Exposure ("1/250"), FNumber, ISO, Focal (mm) and Focal35 (mm on
	// 35 mm film) are the exposure.
	Exposure string  `json:"exposure,omitempty"`
	FNumber  float64 `json:"fNumber,omitempty"`
	ISO      int     `json:"iso,omitempty"`
	Focal    float64 `json:"focal,omitempty"`
	Focal35  int     `json:"focal35,omitempty"`
	// Orientation is the EXIF orientation, 1 to 8; Width and Height the
	// pixel size as stored, before it is turned upright.
	Orientation int `json:"orientation,omitempty"`
	Width       int `json:"width,omitempty"`
	Height      int `json:"height,omitempty"`
}

const takenLayout = "2006-01-02T15:04:05"

var offsetRE = regexp.MustCompile(`^[+-](0\d|1[0-4]):[0-5]\d$`)

// photoText reports whether s is short plain text a picture may carry.
func photoText(s string) bool {
	return utf8.RuneCountInString(s) <= 64 && utf8.ValidString(s) && strings.IndexFunc(s, unicode.IsControl) < 0
}

// finite reports whether every value is a number.
func finite(vs ...float64) bool {
	for _, v := range vs {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}

// checkPhoto refuses photo data that no picture could tell.
func checkPhoto(p *Photo) error {
	if p == nil {
		return nil
	}
	if p.Taken != "" {
		if _, err := time.Parse(takenLayout, p.Taken); err != nil {
			return invalid("photo: taken %q is no time of the form 2006-01-02T15:04:05", p.Taken)
		}
	}
	if p.Offset != "" && (p.Taken == "" || !offsetRE.MatchString(p.Offset)) {
		return invalid("photo: offset %q", p.Offset)
	}
	if (p.Lat == nil) != (p.Lon == nil) {
		return invalid("photo: a place needs latitude and longitude")
	}
	if p.Lat != nil && (!finite(*p.Lat, *p.Lon) || math.Abs(*p.Lat) > 90 || math.Abs(*p.Lon) > 180) {
		return invalid("photo: the place is out of range")
	}
	if p.Alt != nil && (p.Lat == nil || !finite(*p.Alt) || math.Abs(*p.Alt) > 100000) {
		return invalid("photo: the height is out of range")
	}
	for _, s := range []string{p.Make, p.Model, p.Lens, p.Exposure} {
		if !photoText(s) {
			return invalid("photo: camera texts are plain text of at most 64 characters")
		}
	}
	if !finite(p.FNumber, p.Focal) || p.FNumber < 0 || p.FNumber > 1000 || p.Focal < 0 || p.Focal > 100000 ||
		p.ISO < 0 || p.ISO > 10000000 || p.Focal35 < 0 || p.Focal35 > 100000 {
		return invalid("photo: the exposure is out of range")
	}
	if p.Orientation < 0 || p.Orientation > 8 || p.Width < 0 || p.Height < 0 || p.Width > 1<<20 || p.Height > 1<<20 {
		return invalid("photo: orientation or size out of range")
	}
	return nil
}

// ValidPhoto reports whether the item model takes photo data as it is.
func ValidPhoto(p *Photo) bool { return checkPhoto(p) == nil }

// At is the moment a picture was taken: with its offset when it tells
// one, else its wall-clock time read as UTC, which keeps its day. A
// picture that tells no time was taken, as far as the album knows, when
// it was added (created, Unix seconds); exact is then false as well.
func (p *Photo) At(created int64) (at time.Time, exact bool) {
	if p == nil || p.Taken == "" {
		return time.Unix(created, 0).UTC(), false
	}
	t, err := time.Parse(takenLayout, p.Taken)
	if err != nil {
		return time.Unix(created, 0).UTC(), false
	}
	if p.Offset == "" {
		return t, false
	}
	if o, err := time.Parse("-07:00", p.Offset); err == nil {
		_, secs := o.Zone()
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.FixedZone("", secs)), true
	}
	return t, false
}

// Day is the calendar day a picture was taken on where it was taken,
// "2006-01-02", or the day it was added when it tells no time.
func (p *Photo) Day(created int64) string {
	at, _ := p.At(created)
	return at.Format(time.DateOnly)
}

// HasPlace reports whether the picture tells where it was taken.
func (p *Photo) HasPlace() bool { return p != nil && p.Lat != nil && p.Lon != nil }
