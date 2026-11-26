package item

// A slide deck in a text (ADR-0260) is a block of
// the kind slides that names an album folder or collection by its ID in
// Ref and has a plain caption in Text. Its slides are the pictures the
// album shows of that folder or collection, in its order of slides
// (ADR-0176, ADR-0195), read with each reader's rights when the text is
// shown: the body holds no picture and no copy of one (D4), and a reader
// sees only the pictures they read (D10). Like an embed, the block implies
// the relation embeds to the folder or collection, so the folder is not
// deleted while a text shows it (D8), and the impact view and the context
// of the folder name the text.

// Embedded names the item a block embeds and implies the relation embeds
// to: an embed's item, or the folder or collection a slide deck shows.
func (b Block) Embedded() (ID, bool) {
	if (b.Kind == Embed || b.Kind == Slides) && b.Ref != "" {
		return b.Ref, true
	}
	return "", false
}

// checkSlidesBlock validates a slide deck in a text on its own; that its
// reference names a folder or collection is checked against the store
// (checkSlideDecks).
func checkSlidesBlock(b Block, self ID) error {
	switch {
	case b.Ref == "" || b.Ref == self:
		return invalid("a slide deck names an album folder or collection by its ID")
	case len(b.Marks) > 0 || len(b.Mentions) > 0:
		return invalid("a slide deck has a plain caption")
	}
	return nil
}

// checkSlideDecks refuses a slide deck that names an item that is not an
// album folder or collection. The type of an item never changes, so a
// body that passed once keeps passing.
func (s *Store) checkSlideDecks(d Doc) error {
	for i, b := range d.Blocks {
		if b.Kind != Slides {
			continue
		}
		target, ok := s.items[b.Ref]
		if !ok {
			return wrapf(notFound("referenced item %s", b.Ref), "block %d", i+1)
		}
		if target.Type != FolderType && target.Type != CollectionType {
			return wrapf(invalid("%s is no album folder or collection: a slide deck shows the pictures of one", target.Key), "block %d", i+1)
		}
	}
	return nil
}
