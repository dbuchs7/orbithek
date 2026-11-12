package item

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// ShareFile names where the original of a linked entry lies (ADR-0197):
// an album folder bound to a file share holds one entry per file of the
// share, with no content of its own but a preview beside it. Share is the
// share's ID, Path the file's path below the share's address, Tag what
// changes with the file's content (an ETag, a modification time). The
// share is the master: people label, name and place such an entry, but
// never rename, move, version or delete it; the share's runs do. It is
// content, sealed with the rest of the attachment.
type ShareFile struct {
	Share string `json:"share"`
	Path  string `json:"path"`
	Tag   string `json:"tag,omitempty"`
	// Withdrawn is when the file was found gone from the share, Unix
	// seconds, 0 while it is there. It comes from the event that withdrew
	// it and is never persisted.
	Withdrawn int64 `json:"-"`
}

var shareIDRE = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{0,39}$`)

// maxSharePath and maxShareTag bound what a linked entry names.
const (
	maxSharePath = 1024
	maxShareTag  = 128
)

func checkShareFile(f *ShareFile) error {
	if f == nil {
		return invalid("a linked entry names its share and path")
	}
	if !shareIDRE.MatchString(f.Share) {
		return invalid("share %q", f.Share)
	}
	if f.Path == "" || len(f.Path) > maxSharePath || !utf8.ValidString(f.Path) || strings.ContainsAny(f.Path, "\\\x00") {
		return invalid("share path %q", f.Path)
	}
	for _, seg := range strings.Split(f.Path, "/") {
		if seg == "" || seg == "." || seg == ".." || strings.IndexFunc(seg, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
			return invalid("share path %q", f.Path)
		}
	}
	if len(f.Tag) > maxShareTag || strings.IndexFunc(f.Tag, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		return invalid("share tag of %q", f.Path)
	}
	return nil
}

// Linked reports whether the file is an entry of a file share.
func (a Attachment) Linked() bool { return a.Share != nil }

// Withdrawn reports whether the file is an entry of a file share whose
// file is gone from the share for now.
func (a Attachment) Withdrawn() bool { return a.Share != nil && a.Share.Withdrawn != 0 }

// onShare refuses to change a file of a share as people change files: the
// share is the master (ADR-0197).
func onShare(a *Attachment) error {
	if a != nil && a.Share != nil {
		return invalid("%s lies on the file share %s, which is the master; change it there", a.Name, a.Share.Share)
	}
	return nil
}

// linkedEntry returns a live linked entry of a live item.
func (s *Store) linkedEntry(id, attachment ID) (*Item, *Attachment, error) {
	it, err := s.live(id)
	if err != nil {
		return nil, nil, err
	}
	a, ok := it.attachment(attachment)
	if !ok || a.Deleted || a.Share == nil {
		return nil, nil, notFound("linked entry %s of item %s", attachment, id)
	}
	return it, a, nil
}

// checkLinkedContent checks what a run records of a file it read: like an
// upload's, without content of its own here.
func checkLinkedContent(a Attachment) error {
	switch {
	case !ValidAttachmentID(string(a.ID)):
		return invalid("attachment ID %q", a.ID)
	case a.Size < 0 || !sha256RE.MatchString(a.SHA256) || strings.TrimSpace(a.Type) == "":
		return invalid("attachment %s: size, type and SHA-256 are required", a.ID)
	}
	if err := checkAttachmentName(a.Name); err != nil {
		return err
	}
	if err := checkPhoto(a.Photo); err != nil {
		return err
	}
	if a.Fingerprint != "" && !fingerprintRE.MatchString(a.Fingerprint) {
		return invalid("attachment %s: the fingerprint must be 16 hexadecimal digits", a.ID)
	}
	return checkShareFile(a.Share)
}

// LinkFile records a file of a share as an entry of the folder that holds
// its directory (ADR-0197). Only a share's run calls it, as the trusted
// store; the caller has stored the entry's preview under a.ID.
func (s *Store) LinkFile(actor string, id ID, a Attachment) (Attachment, error) {
	it, err := s.live(id)
	if err != nil {
		return Attachment{}, err
	}
	if err := checkLinkedContent(a); err != nil {
		return Attachment{}, err
	}
	if _, dup := it.attachment(a.ID); dup {
		return Attachment{}, invalid("attachment %s exists", a.ID)
	}
	f := *a.Share
	f.Withdrawn = 0
	rec := Attachment{ID: a.ID, Name: a.Name, Type: a.Type, Size: a.Size, SHA256: a.SHA256, Photo: a.Photo, Fingerprint: a.Fingerprint, Share: &f}
	if err := s.record(actor, Event{Kind: AttachmentLinked, Item: id, Attachment: &rec}); err != nil {
		return Attachment{}, err
	}
	got, _ := s.items[id].attachment(a.ID)
	return *got, nil
}

// RelinkFile records what a run found of a linked entry's file again: a
// changed content (size, type, hash, photo data, fingerprint, tag) or a
// new path within the same folder. What people set on the entry stays:
// labels, notes, faces, places and times set by hand.
func (s *Store) RelinkFile(actor string, id ID, a Attachment) (Attachment, error) {
	_, old, err := s.linkedEntry(id, a.ID)
	if err != nil {
		return Attachment{}, err
	}
	if err := checkLinkedContent(a); err != nil {
		return Attachment{}, err
	}
	if a.Share.Share != old.Share.Share {
		return Attachment{}, invalid("entry %s belongs to the share %s", a.ID, old.Share.Share)
	}
	f := *a.Share
	f.Withdrawn = 0
	rec := Attachment{ID: a.ID, Name: a.Name, Type: a.Type, Size: a.Size, SHA256: a.SHA256, Photo: a.Photo, Fingerprint: a.Fingerprint, Share: &f}
	if err := s.record(actor, Event{Kind: AttachmentRelinked, Item: id, Attachment: &rec}); err != nil {
		return Attachment{}, err
	}
	got, _ := s.items[id].attachment(a.ID)
	return *got, nil
}

// WithdrawFile hides a linked entry whose file is gone from the share; its
// preview and what people set on it stay until it comes back or is
// deleted.
func (s *Store) WithdrawFile(actor string, id, attachment ID) error {
	_, a, err := s.linkedEntry(id, attachment)
	if err != nil {
		return err
	}
	if a.Share.Withdrawn != 0 {
		return invalid("entry %s is withdrawn already", attachment)
	}
	return s.record(actor, Event{Kind: AttachmentWithdrawn, Item: id, Attachment: &Attachment{ID: attachment}})
}

// RestoreFile shows a withdrawn entry again, whose file came back to the
// share, at the path and tag given.
func (s *Store) RestoreFile(actor string, id, attachment ID, at ShareFile) error {
	_, a, err := s.linkedEntry(id, attachment)
	if err != nil {
		return err
	}
	if a.Share.Withdrawn == 0 {
		return invalid("entry %s is not withdrawn", attachment)
	}
	at.Share, at.Withdrawn = a.Share.Share, 0
	if err := checkShareFile(&at); err != nil {
		return err
	}
	return s.record(actor, Event{Kind: AttachmentRestored, Item: id, Attachment: &Attachment{ID: attachment, Share: &at}})
}

// applyShareFile applies the kinds of linked entries.
func (s *Store) applyShareFile(it *Item, e Event) {
	switch e.Kind {
	case AttachmentLinked:
		a := *e.Attachment
		f := *a.Share
		a.Share, a.Author, a.Created = &f, e.Actor, e.At.Unix()
		it.Attachments = append(it.Attachments, a)
	case AttachmentRelinked:
		if a, ok := it.attachment(e.Attachment.ID); ok && a.Share != nil {
			n := e.Attachment
			f := *n.Share
			a.Name, a.Type, a.Size, a.SHA256, a.Fingerprint, a.Share = n.Name, n.Type, n.Size, n.SHA256, n.Fingerprint, &f
			a.Photo = relinkedPhoto(a.Photo, n.Photo)
		}
	case AttachmentWithdrawn:
		if a, ok := it.attachment(e.Attachment.ID); ok && a.Share != nil {
			f := *a.Share
			f.Withdrawn = e.At.Unix()
			a.Share = &f
		}
	case AttachmentRestored:
		if a, ok := it.attachment(e.Attachment.ID); ok && a.Share != nil && e.Attachment.Share != nil {
			f := *e.Attachment.Share
			f.Share, f.Withdrawn = a.Share.Share, 0
			a.Share = &f
		}
	}
}

// relinkedPhoto takes what a file tells again, keeping a place or time set
// by hand and whether its place is private (ADR-0178, ADR-0181,
// ADR-0198).
func relinkedPhoto(old, read *Photo) *Photo {
	if old == nil {
		return read
	}
	var p Photo
	if read != nil {
		p = *read
	}
	if old.PlaceManual {
		p.Lat, p.Lon, p.Alt, p.PlaceName, p.PlaceManual = old.Lat, old.Lon, nil, old.PlaceName, true
	}
	if old.TakenManual {
		p.Taken, p.Offset, p.TakenManual = old.Taken, old.Offset, true
	}
	p.PlacePrivate = old.PlacePrivate
	if read == nil && !old.PlaceManual && !old.TakenManual && !old.PlacePrivate {
		return nil
	}
	return &p
}
