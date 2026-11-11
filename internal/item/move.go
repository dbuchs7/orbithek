package item

import "slices"

// A file is moved to another item with all its versions, keeping its
// identity (ADR-0188): its ID, every version, its labels, notes, place,
// rule of versions and who added it stay, so favourites and copies of it
// follow it. Its content is sealed again with the new item's key by the
// storage layer, which asks PlanMove first and records the move with
// MoveFile once the content is stored for the new item.

// PlanMove tells which versions of a file move to another item, so their
// content can be stored for that item before the move is recorded. file
// names any version; the result is the file's first version, which names
// it in the album, and every version that is not deleted, whose content
// moves. It changes nothing.
//
// The file is moved only when the principal may edit both items and read
// the file, and when nothing in its item depends on it: a file the item's
// text shows stays, so the text is not left with a gap; a protected file
// stays under its protection (ADR-0160); a drawing that comments are on
// stays with them (ADR-0171).
func (x *Session) PlanMove(from, file, to ID) (first ID, live []Attachment, err error) {
	it, _, records, err := x.planMove(from, file, to)
	if err != nil {
		return "", nil, err
	}
	for _, a := range records {
		if !a.Deleted {
			live = append(live, a)
		}
	}
	return firstVersion(it, records[0]), live, nil
}

func (x *Session) planMove(from, file, to ID) (*Item, *Item, []Attachment, error) {
	src, err := x.editable(from)
	if err != nil {
		return nil, nil, nil, err
	}
	if src.Presence != Live {
		return nil, nil, nil, invalid("%s is %s", src.Key, src.Presence)
	}
	it, first, err := x.stackOf(from, file)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := x.CanAttach(to); err != nil {
		return nil, nil, nil, err
	}
	dst := x.s.items[to]
	if dst.ID == it.ID {
		return nil, nil, nil, invalid("the file is in %s already", dst.Key)
	}
	var records []Attachment
	for _, a := range it.Attachments {
		if firstVersion(it, a) != first {
			continue
		}
		if !a.Deleted && inBody(it.Body, a.ID) {
			return nil, nil, nil, referenced("the text of %s shows the file; it stays with the text", it.Key)
		}
		if _, ok := it.fileSecret(a.ID); ok {
			return nil, nil, nil, forbidden("the file is protected in %s; it stays under its protection", it.Key)
		}
		if _, dup := dst.attachment(a.ID); dup {
			return nil, nil, nil, invalid("attachment %s exists in %s", a.ID, dst.Key)
		}
		records = append(records, a)
	}
	for _, c := range it.Comments {
		if !c.Deleted && c.Anchor != nil && c.Anchor.Drawing != "" && slices.ContainsFunc(records, func(a Attachment) bool { return a.ID == c.Anchor.Drawing }) {
			return nil, nil, nil, referenced("comments of %s are on the drawing; it stays with them", it.Key)
		}
	}
	return it, dst, records, nil
}

// MoveFile moves a file with all its versions from one item to another
// (ADR-0188), once the content of the versions PlanMove named is stored
// for the new item. stored names those versions; if the file has changed
// since, the move is refused and nothing happens, so no version is moved
// without its content. Versions made in the item point to each other in
// the new item as in the old; a stack copied from a third item keeps
// naming its original.
func (x *Session) MoveFile(from, file, to ID, stored []ID) error {
	it, dst, records, err := x.planMove(from, file, to)
	if err != nil {
		return err
	}
	var live []ID
	for _, a := range records {
		if !a.Deleted {
			live = append(live, a.ID)
		}
	}
	if !sameIDs(live, stored) {
		return invalid("the file changed while it was moved; try again")
	}
	in := make([]Event, 0, len(records))
	out := make([]Event, 0, len(records))
	for _, a := range records {
		rec := a
		rec.Labels, rec.Photo, rec.Faces = slices.Clone(a.Labels), clonePhoto(a.Photo), slices.Clone(a.Faces)
		if a.Retention != nil {
			r := *a.Retention
			rec.Retention = &r
		}
		if a.Source != nil {
			src := *a.Source
			if src.Item == it.ID {
				src.Item = dst.ID
			}
			rec.Source = &src
		}
		rec.Move = &Move{From: it.ID, Author: a.Author, Created: a.Created, Deleted: a.Deleted}
		rec.Author, rec.Created, rec.Deleted, rec.AsGiven = "", 0, false, false
		in = append(in, Event{Kind: AttachmentMovedIn, Item: dst.ID, Attachment: &rec})
		out = append(out, Event{Kind: AttachmentMovedOut, Item: it.ID, Attachment: &Attachment{ID: a.ID, Move: &Move{To: dst.ID}}})
	}
	return x.s.recordBatch(x.who, append(in, out...)...)
}

// clonePhoto copies photo data, so the moved version shares nothing
// with the one it leaves.
func clonePhoto(p *Photo) *Photo {
	if p == nil {
		return nil
	}
	c := *p
	for _, f := range []**float64{&c.Lat, &c.Lon, &c.Alt} {
		if *f != nil {
			v := **f
			*f = &v
		}
	}
	return &c
}

// sameIDs reports whether two lists hold the same IDs, in any order.
func sameIDs(a, b []ID) bool {
	if len(a) != len(b) {
		return false
	}
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

// sourceItem is the item that holds the file a source names now: where
// it went when it was moved out of the item named (ADR-0188).
func (s *Store) sourceItem(src Source) ID {
	if to, ok := s.moved[src.Attachment]; ok {
		if it := s.items[src.Item]; it == nil || !hasFile(it, src.Attachment) {
			return to
		}
	}
	return src.Item
}

func hasFile(it *Item, file ID) bool {
	_, ok := it.attachment(file)
	return ok
}
