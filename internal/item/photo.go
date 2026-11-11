package item

import (
	"math"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Photo is what a picture tells about itself (ADR-0177): when it was
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
	// TakenManual says the time was set by hand (ADR-0196), so it, not the
	// file's own, counts: every file given out carries it in its EXIF.
	TakenManual bool `json:"takenManual,omitempty"`
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
	// PlacePrivate keeps the place to the one who added the picture
	// (ADR-0178): everyone else reads the photo data without it and gets
	// the file without it.
	PlacePrivate bool `json:"placePrivate,omitempty"`
	// PlaceName names the place, as it was chosen by hand; PlaceManual
	// says the place was set or removed by hand (ADR-0181), so it, not the
	// file's own, counts: everyone but the uploader gets the file without
	// the file's place.
	PlaceName   string `json:"placeName,omitempty"`
	PlaceManual bool   `json:"placeManual,omitempty"`
	// Duration is how long a video plays, in seconds (ADR-0189).
	Duration float64 `json:"duration,omitempty"`
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
	if p.TakenManual && p.Taken == "" {
		return invalid("photo: a time set by hand needs a time")
	}
	if (p.Lat == nil) != (p.Lon == nil) {
		return invalid("photo: a place needs latitude and longitude")
	}
	if p.Lat != nil && (!finite(*p.Lat, *p.Lon) || math.Abs(*p.Lat) > 90 || math.Abs(*p.Lon) > 180) {
		return invalid("photo: the place is out of range")
	}
	if p.PlacePrivate && p.Lat == nil {
		return invalid("photo: only a place can be kept private")
	}
	if p.PlaceName != "" && (p.Lat == nil || utf8.RuneCountInString(p.PlaceName) > 200 || !utf8.ValidString(p.PlaceName) || strings.IndexFunc(p.PlaceName, unicode.IsControl) >= 0) {
		return invalid("photo: a place's name is plain text of at most 200 characters, for a place")
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
	if !finite(p.Duration) || p.Duration < 0 || p.Duration > 1e7 {
		return invalid("photo: the duration is out of range")
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

// placeHidden reports whether a file's place is kept from who: the photo
// tells a place, and the one who added it keeps it to themselves
// (ADR-0178).
func placeHidden(who string, a Attachment) bool {
	return a.Photo.HasPlace() && a.Photo.PlacePrivate && a.Author != who
}

// fileWithoutPlace reports whether who gets a file without the place the
// file itself tells: its uploader keeps the place to themselves
// (ADR-0178), or the place was set or removed by hand, so the file's own
// is not the photo's (ADR-0181). The uploader always gets the original.
func fileWithoutPlace(who string, a Attachment) bool {
	return placeHidden(who, a) || (a.Photo != nil && a.Photo.PlaceManual && a.Author != who)
}

// timeWritten reports whether every file given out of a picture carries
// a time set by hand in its EXIF rather than the file's own (ADR-0196).
func timeWritten(a Attachment) bool { return a.Photo != nil && a.Photo.TakenManual }

// fileAsGiven reports whether who gets a file other than as it is stored:
// without its place (ADR-0178, ADR-0181) or with a time set by hand
// (ADR-0196).
func fileAsGiven(who string, a Attachment) bool { return fileWithoutPlace(who, a) || timeWritten(a) }

// seenBy is a file as who may know it: without a place its uploader keeps
// to themselves, and without saying that there is one.
func seenBy(who string, a Attachment) Attachment {
	if !placeHidden(who, a) {
		return a
	}
	p := *a.Photo
	p.Lat, p.Lon, p.Alt, p.PlacePrivate, p.PlaceName, p.PlaceManual = nil, nil, nil, false, "", false
	a.Photo = &p
	return a
}

// PlaceHidden reports whether the file of a readable item is given to the
// principal only without the place it tells: its uploader keeps the place
// from them (ADR-0178), or a place set by hand replaces it (ADR-0181).
func (x *Session) PlaceHidden(id, attachment ID) bool {
	it, err := x.readable(id)
	if err != nil {
		return false
	}
	a, ok := it.attachment(attachment)
	return ok && !a.Deleted && x.s.readsFile(x.who, it, attachment) && fileWithoutPlace(x.who, *a)
}

// SetPlace sets where a photo was taken by hand, with the place's name,
// or removes its place when lat and lon are nil (ADR-0181). Who may edit
// the picture's item may, unless its uploader keeps its place from them.
// read is the photo data the server read from the file, for a picture
// stored before photo data was recorded. The height the file told goes;
// whether the place is private stays.
func (x *Session) SetPlace(id, attachment ID, lat, lon *float64, name string, read *Photo) error {
	it, err := x.editable(id)
	if err != nil {
		return err
	}
	if it.Presence != Live {
		return invalid("%s is %s", it.Key, it.Presence)
	}
	a, ok := it.attachment(attachment)
	if !ok || a.Deleted || !x.s.readsFile(x.who, it, attachment) {
		return notFound("attachment %s of item %s", attachment, it.Key)
	}
	if placeHidden(x.who, *a) {
		return forbidden("the one who added the picture keeps its place to themselves")
	}
	if (lat == nil) != (lon == nil) {
		return invalid("a place needs latitude and longitude")
	}
	var next Photo
	switch {
	case a.Photo != nil:
		next = *a.Photo
	case read != nil:
		next = *read
	}
	next.Lat, next.Lon, next.Alt, next.PlaceName, next.PlaceManual = lat, lon, nil, strings.TrimSpace(name), true
	if lat == nil {
		next.PlaceName, next.PlacePrivate = "", false
	}
	if err := checkPhoto(&next); err != nil {
		return err
	}
	return x.s.record(x.who, Event{Kind: AttachmentPlaceSet, Item: id, Attachment: &Attachment{ID: attachment, Photo: &next}})
}

// SetPlacePrivate keeps where a photo was taken to the one who added it,
// or shows it to every reader again (ADR-0178). Only the one who added the
// picture decides, while they may read it. read is the photo data the
// server read from the file, for a picture stored before photo data was
// recorded; it is recorded with the choice.
func (x *Session) SetPlacePrivate(id, attachment ID, private bool, read *Photo) error {
	it, err := x.readable(id)
	if err != nil {
		return err
	}
	if it.Presence != Live {
		return invalid("%s is %s", it.Key, it.Presence)
	}
	a, ok := it.attachment(attachment)
	if !ok || a.Deleted || !x.s.readsFile(x.who, it, attachment) {
		return notFound("attachment %s of item %s", attachment, it.Key)
	}
	if a.Author != x.who {
		return forbidden("only the one who added the picture decides who sees where it was taken")
	}
	p := a.Photo
	if p == nil {
		p = read
	}
	if !p.HasPlace() {
		return invalid("the picture tells no place")
	}
	if a.Photo != nil && p.PlacePrivate == private {
		return nil
	}
	next := *p
	next.PlacePrivate = private
	if err := checkPhoto(&next); err != nil {
		return err
	}
	return x.s.record(x.who, Event{Kind: AttachmentPlaceSet, Item: id, Attachment: &Attachment{ID: attachment, Photo: &next}})
}

// Years a time set by hand may lie in: those the EXIF reader takes
// (ADR-0177).
const minTakenYear, maxTakenYear = 1800, 2200

// SetTaken sets when a photo was taken by hand (ADR-0196): taken is the
// wall-clock time "2006-01-02T15:04:05", offset its offset from UTC,
// "+02:00", or empty when it is not known. An empty taken goes back to the
// time the file itself tells. Who may edit the picture's item may. file is
// the photo data the server read from the stored file: the time it tells,
// and everything else for a picture stored before photo data was recorded.
// The rest of the photo data, its place above all, stays as it is.
func (x *Session) SetTaken(id, attachment ID, taken, offset string, file *Photo) error {
	it, err := x.editable(id)
	if err != nil {
		return err
	}
	if it.Presence != Live {
		return invalid("%s is %s", it.Key, it.Presence)
	}
	a, ok := it.attachment(attachment)
	if !ok || a.Deleted || !x.s.readsFile(x.who, it, attachment) {
		return notFound("attachment %s of item %s", attachment, it.Key)
	}
	var next Photo
	switch {
	case a.Photo != nil:
		next = *a.Photo
	case file != nil:
		next = *file
	}
	if taken == "" {
		if a.Photo == nil || !a.Photo.TakenManual {
			return nil // the file's own time counts already
		}
		next.Taken, next.Offset, next.TakenManual = "", "", false
		if file != nil {
			next.Taken, next.Offset = file.Taken, file.Offset
		}
	} else {
		t, err := time.Parse(takenLayout, taken)
		if err != nil || t.Year() < minTakenYear || t.Year() > maxTakenYear {
			return invalid("a time a photo was taken is of the form 2006-01-02T15:04:05, in the years %d to %d", minTakenYear, maxTakenYear)
		}
		if offset != "" && !offsetRE.MatchString(offset) {
			return invalid("an offset from UTC is of the form +02:00")
		}
		next.Taken, next.Offset, next.TakenManual = taken, offset, true
	}
	if err := checkPhoto(&next); err != nil {
		return err
	}
	if p := a.Photo; p != nil && p.Taken == next.Taken && p.Offset == next.Offset && p.TakenManual == next.TakenManual {
		return nil
	}
	return x.s.record(x.who, Event{Kind: AttachmentTakenSet, Item: id, Attachment: &Attachment{ID: attachment, Photo: &next}})
}
