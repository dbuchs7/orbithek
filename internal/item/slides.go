package item

import (
	"maps"
	"slices"
)

// A slide deck in a text (ADR-0260) is a block of the kind slides that
// keeps the slides as they were when it was saved: Files names, in the
// order of the slides, files of the item itself — copies of the pictures
// of an album folder or collection, made in the browser with their
// speaker's notes when the deck is inserted or updated (ADR-0054,
// ADR-0055). Ref names that folder or collection by its ID, the source a
// deck is updated from, and Text is a plain caption.
//
// The slides are the item's own files, so they are read and protected
// with the item (D9, ADR-0160) and a version of the text shows the slides
// it showed. The source is no reference (D8): the folder may be deleted,
// and the deck keeps its slides; a deck whose source is gone has none.
//
// A deck written before the slides were kept (codec version 71) has a
// source and no files; it shows the folder as it is now, and a text may
// keep it unchanged until its next edit keeps its slides.

// MaxSlides bounds the slides a deck keeps.
const MaxSlides = 500

// Embedded names the item an embed shows and implies the relation embeds
// to (ADR-0020).
func (b Block) Embedded() (ID, bool) {
	if b.Kind == Embed && b.Ref != "" {
		return b.Ref, true
	}
	return "", false
}

// shownFiles names the files of its item a block shows: an image's, a
// video's, a sound's or a BPMN diagram's file, or a slide deck's slides.
func (b Block) shownFiles() []ID {
	switch {
	case b.Kind == Slides:
		return b.Files
	case b.Ref != "" && showsFile(b.Kind):
		return []ID{b.Ref}
	}
	return nil
}

// checkSlidesBlock validates a slide deck in a text on its own; that its
// files are the item's and its source a folder or collection is checked
// against the store (checkSlideDecks).
func checkSlidesBlock(b Block, self ID) error {
	switch {
	case b.Ref == self && self != "":
		return invalid("a slide deck's source is an album folder or collection, not its own item")
	case b.Ref == "" && len(b.Files) == 0:
		return invalid("a slide deck keeps its slides as files of its item, and names the album folder or collection they come from")
	case len(b.Marks) > 0 || len(b.Mentions) > 0:
		return invalid("a slide deck has a plain caption")
	case b.Files != nil && (len(b.Files) == 0 || len(b.Files) > MaxSlides):
		return invalid("a slide deck keeps 1 to %d slides", MaxSlides)
	}
	for i, f := range b.Files {
		if !ValidAttachmentID(string(f)) || slices.Contains(b.Files[:i], f) {
			return invalid("slide %d: a slide deck names each slide once, by the ID of a file of its item", i+1)
		}
	}
	return nil
}

// checkSlideDecks checks the slide decks of a body against the store and
// the item's current body (none for a new item): every slide is a file of
// the item, one deleted since only while the current body shows it in a
// deck; a source named anew is a live album folder or collection; and a
// deck that keeps no slides is one the current body holds unchanged.
func (s *Store) checkSlideDecks(it *Item, d Doc) error {
	var current []Block
	if it != nil {
		current = it.Body.Blocks
	}
	for i, b := range d.Blocks {
		if b.Kind != Slides {
			continue
		}
		if len(b.Files) == 0 && !slices.ContainsFunc(current, func(o Block) bool { return o.Kind == Slides && len(o.Files) == 0 && o.Ref == b.Ref }) {
			return wrapf(invalid("a slide deck keeps its slides as files of its item; copy them from the album folder first"), "block %d", i+1)
		}
		for _, f := range b.Files {
			var a *Attachment
			ok := false
			if it != nil {
				a, ok = it.attachment(f)
			}
			kept := slices.ContainsFunc(current, func(o Block) bool { return o.Kind == Slides && slices.Contains(o.Files, f) })
			if !ok || a.Deleted && !kept {
				return wrapf(notFound("attachment %s of the item", f), "block %d", i+1)
			}
		}
		if b.Ref == "" || slices.ContainsFunc(current, func(o Block) bool { return o.Kind == Slides && o.Ref == b.Ref }) {
			continue
		}
		target, ok := s.items[b.Ref]
		switch {
		case !ok:
			return wrapf(notFound("referenced item %s", b.Ref), "block %d", i+1)
		case target.Type != FolderType && target.Type != CollectionType:
			return wrapf(invalid("%s is no album folder or collection: a slide deck copies the pictures of one", target.Key), "block %d", i+1)
		case target.Presence != Live:
			return wrapf(invalid("item %s is %s and cannot be newly a slide deck's source", target.Key, target.Presence), "block %d", i+1)
		}
	}
	return nil
}

// slideSources names the sources of the slide decks of a body.
func slideSources(d Doc) []ID {
	var out []ID
	for _, b := range d.Blocks {
		if b.Kind == Slides && b.Ref != "" && !slices.Contains(out, b.Ref) {
			out = append(out, b.Ref)
		}
	}
	return out
}

// noteSlideFiles remembers the files a committed body shows as slides; it
// runs live and on replay. A file stays a slide once a version of the text
// showed it as one, so the copies an earlier version keeps stay out of the
// album too (ADR-0260).
func (it *Item) noteSlideFiles(d Doc) {
	for _, b := range d.Blocks {
		if b.Kind != Slides {
			continue
		}
		for _, f := range b.Files {
			if it.slideFiles[f] {
				continue
			}
			it.slideFiles = maps.Clone(it.slideFiles)
			if it.slideFiles == nil {
				it.slideFiles = map[ID]bool{}
			}
			it.slideFiles[f] = true
		}
	}
}

// isSlide reports whether a file, or the first version of its stack, is a
// slide a version of its item's text kept (ADR-0260).
func (it *Item) isSlide(a Attachment, first ID) bool {
	return it.slideFiles[a.ID] || it.slideFiles[first]
}

// SlidesLeftBehind names the files of the item that are slides (ADR-0260)
// and that no block of its text shows any more: the copies an earlier
// version of a deck kept, and newer versions of a slide. They are the
// text's own, out of the album, so a copy of the item or its return from a
// backup leaves them behind; there they would be pictures of the album.
func (it Item) SlidesLeftBehind() map[ID]bool {
	if len(it.slideFiles) == 0 {
		return nil
	}
	shown := map[ID]bool{}
	for _, b := range it.Body.Blocks {
		for _, f := range b.shownFiles() {
			shown[f] = true
		}
	}
	var out map[ID]bool
	for _, a := range it.Attachments {
		if shown[a.ID] || !it.isSlide(a, firstVersion(&it, a)) {
			continue
		}
		if out == nil {
			out = map[ID]bool{}
		}
		out[a.ID] = true
	}
	return out
}
