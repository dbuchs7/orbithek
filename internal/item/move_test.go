package item

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// A file moved to another item keeps its identity with all its versions,
// labels, notes, rule, place and who added it; its copies find it there,
// and it leaves its former item (ADR-0186).
func TestAMovedFileKeepsItsIdentity(t *testing.T) {
	s := permFixture(t)
	anna, ben := s.As("anna"), s.As("ben")
	noErr(t, s.GrantRole("root", "OPS", "anna", Editor))
	from := mustItem(t)(anna.CreateFolder("PLAT", "", "Inbox"))
	to := mustItem(t)(anna.CreateFolder("OPS", "", "Holidays"))
	const v1, v2, v3, other = "0123456789abcdef0123456789abcd01", "0123456789abcdef0123456789abcd02", "0123456789abcdef0123456789abcd03", "0123456789abcdef0123456789abcd04"
	addPicture(t, anna, from.ID, v1, "Lake.drawing.png")
	for _, v := range []struct{ id, prev ID }{{v2, v1}, {v3, v2}} {
		_, err := anna.AddAttachment(from.ID, Attachment{ID: v.id, Name: "Lake.drawing.png", Type: "image/png", Size: 3,
			SHA256: strings.Repeat("cd", 32), Source: &Source{Item: from.ID, Attachment: v.prev}})
		noErr(t, err)
	}
	addPicture(t, anna, from.ID, other, "Other.png")
	noErr(t, anna.SetPictureLabels(from.ID, v3, []string{"lake"}))
	noErr(t, anna.SetPictureNotes(from.ID, v3, "Say: the lake"))
	noErr(t, anna.SetPictureRetention(from.ID, v3, &Retention{Keep: 5}))
	noErr(t, anna.DeleteAttachment(from.ID, v1)) // a deleted version still links the stack
	page := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Trip"}))
	const copied = "0123456789abcdef0123456789abcd05"
	_, err := anna.AddAttachment(page.ID, Attachment{ID: copied, Name: "Lake.drawing.png", Type: "image/png", Size: 3,
		SHA256: strings.Repeat("cd", 32), Source: &Source{Item: from.ID, Attachment: v2}})
	noErr(t, err)

	first, live, err := anna.PlanMove(from.ID, v3, to.ID)
	noErr(t, err)
	ids := func(as []Attachment) []ID {
		var out []ID
		for _, a := range as {
			out = append(out, a.ID)
		}
		slices.Sort(out)
		return out
	}
	if first != v1 || !slices.Equal(ids(live), []ID{v2, v3}) {
		t.Fatalf("plan: first %s, live %v", first, ids(live))
	}
	noErr(t, anna.MoveFile(from.ID, v3, to.ID, []ID{v3, v2}))

	check := func(s *Store) {
		t.Helper()
		x := s.As("anna")
		left, err := x.Attachments(from.ID)
		noErr(t, err)
		if !slices.Equal(ids(left), []ID{other}) {
			t.Errorf("left in the inbox: %v", ids(left))
		}
		it := s.items[from.ID]
		if slices.ContainsFunc(it.Attachments, func(a Attachment) bool { return a.ID == v1 }) {
			t.Error("the deleted version stayed behind")
		}
		got, err := x.Attachments(to.ID)
		noErr(t, err)
		if !slices.Equal(ids(got), []ID{v2, v3}) {
			t.Fatalf("in the folder: %v", ids(got))
		}
		dst := s.items[to.ID]
		top, _ := dst.attachment(v3)
		if top.Author != "anna" || top.Created == 0 || top.Name != "Lake.drawing.png" || !slices.Equal(top.Labels, []string{"lake"}) || top.Move != nil {
			t.Errorf("moved version %+v", *top)
		}
		if top.Source == nil || top.Source.Item != to.ID || top.Source.Attachment != v2 {
			t.Errorf("the version's source %+v", top.Source)
		}
		if f := firstVersion(dst, *top); f != v1 {
			t.Errorf("first version in the new item %s", f)
		}
		if notesOf(dst, v1) != "Say: the lake" || ruleOf(dst, v1) == nil || ruleOf(dst, v1).Keep != 5 {
			t.Errorf("notes %q, rule %v", notesOf(dst, v1), ruleOf(dst, v1))
		}
		if gone, _ := dst.attachment(v1); gone == nil || !gone.Deleted {
			t.Errorf("the deleted first version did not come along deleted: %+v", gone)
		}
		// The copy finds its original where it went.
		o, ok, err := x.FileOrigin(page.ID, copied)
		noErr(t, err)
		if !ok || !o.Readable || o.Item.ID != to.ID || o.Latest == nil || o.Latest.ID != v3 {
			t.Errorf("origin of the copy %+v %v", o, ok)
		}
	}
	check(s)
	re, err := Replay(s.Log())
	noErr(t, err)
	check(re)
	// One who may not read the new item learns nothing of it: ben reads
	// PLAT only.
	if o, ok, err := ben.FileOrigin(page.ID, copied); err != nil || !ok || o.Readable {
		t.Errorf("origin for a reader of the copy only %+v %v %v", o, ok, err)
	}
	// It moves back, as any file.
	noErr(t, anna.MoveFile(to.ID, v2, from.ID, []ID{v2, v3}))
	if got, _ := anna.Attachments(from.ID); !slices.Equal(ids(got), []ID{v2, v3, other}) {
		t.Errorf("moved back: %v", ids(got))
	}
}

// What a move may not do: leave a text, a protection or comments behind,
// act without the right to edit both items, or record versions whose
// content was not stored (ADR-0186).
func TestAMoveIsRefused(t *testing.T) {
	s := permFixture(t)
	anna, ben := s.As("anna"), s.As("ben")
	from := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Plan"}))
	to := mustItem(t)(anna.CreateFolder("PLAT", "", "Pictures"))
	const shown, drawing, plain = "0123456789abcdef0123456789abce01", "0123456789abcdef0123456789abce02", "0123456789abcdef0123456789abce03"
	addPicture(t, anna, from.ID, shown, "shown.png")
	addPicture(t, anna, from.ID, drawing, "board.drawing.png")
	addPicture(t, anna, from.ID, plain, "plain.png")
	noErr(t, anna.CommitBody(from.ID, Doc{Blocks: []Block{{Kind: Image, Ref: shown, Text: "Shown"}}}))
	_, err := s.Comment("anna", from.ID, NewComment{Text: "?", Anchor: &Anchor{Drawing: drawing, Element: "el_1"}})
	noErr(t, err)
	n := len(s.Log())
	for name, c := range map[string]struct {
		err  error
		want error
	}{
		"shown in the text":   {anna.MoveFile(from.ID, shown, to.ID, []ID{shown}), ErrReferenced},
		"commented drawing":   {anna.MoveFile(from.ID, drawing, to.ID, []ID{drawing}), ErrReferenced},
		"a viewer":            {ben.MoveFile(from.ID, plain, to.ID, []ID{plain}), ErrForbidden},
		"into its own item":   {anna.MoveFile(from.ID, plain, from.ID, []ID{plain}), ErrValidation},
		"content not stored":  {anna.MoveFile(from.ID, plain, to.ID, nil), ErrValidation},
		"an unknown file":     {anna.MoveFile(from.ID, "fedcba9876543210fedcba9876543210", to.ID, nil), ErrNotFound},
		"an unknown target":   {anna.MoveFile(from.ID, plain, "nope", []ID{plain}), ErrNotFound},
		"a plan for a viewer": {planErr(ben.PlanMove(from.ID, plain, to.ID)), ErrForbidden},
	} {
		if !errors.Is(c.err, c.want) {
			t.Errorf("%s: %v, want %v", name, c.err, c.want)
		}
	}
	if len(s.Log()) != n {
		t.Error("a refused move was recorded")
	}
	// A viewer of the target may not move a file into it either.
	noErr(t, s.GrantRole("root", "OPS", "anna", Viewer))
	other := mustItem(t)(s.Create("root", NewItem{Space: "OPS", Type: "page", Title: "Ops"}))
	if err := anna.MoveFile(from.ID, plain, other.ID, []ID{plain}); !errors.Is(err, ErrForbidden) {
		t.Errorf("into an item one may only read: %v", err)
	}
}

func planErr(_ ID, _ []Attachment, err error) error { return err }

// A moved version's names and labels are content of its new item: sealed
// there, scanned there, and erased with it (ADR-0026, ADR-0040, ADR-0186).
func TestAMovedFileIsContentOfItsNewItem(t *testing.T) {
	s := permFixture(t)
	anna, ben := s.As("anna"), s.As("ben")
	from := mustItem(t)(anna.CreateFolder("PLAT", "", "Inbox"))
	to := mustItem(t)(anna.CreateFolder("PLAT", "", "Archive"))
	addPicture(t, anna, from.ID, albumPic, "plan.png")
	noErr(t, anna.SetPictureLabels(from.ID, albumPic, []string{"plan"}))
	noErr(t, anna.MoveFile(from.ID, albumPic, to.ID, []ID{albumPic}))
	log := s.Log()
	in, out := log[len(log)-2], log[len(log)-1]
	if in.Kind != AttachmentMovedIn || in.Item != to.ID || !in.HasContent() || in.Attachment.Move.From != from.ID || in.Batch == 0 {
		t.Fatalf("moved in %+v", in)
	}
	if out.Kind != AttachmentMovedOut || out.Item != from.ID || out.HasContent() || out.Attachment.Move.To != to.ID || out.Batch != in.Batch {
		t.Fatalf("moved out %+v", out)
	}
	in.erase()
	if in.Attachment != nil || !in.Erased {
		t.Errorf("erased %+v", in)
	}
	if _, err := ben.Attachment(to.ID, albumPic); err != nil {
		t.Errorf("a reader of the new item: %v", err)
	}
}

// A place its uploader keeps to themselves stays theirs after a move by
// someone else: the version keeps who added it (ADR-0178, ADR-0186).
func TestAMovedPhotoKeepsItsPrivatePlace(t *testing.T) {
	s := permFixture(t)
	noErr(t, s.GrantRole("root", "PLAT", "dora", Editor))
	anna, dora, ben := s.As("anna"), s.As("dora"), s.As("ben")
	from := mustItem(t)(anna.CreateFolder("PLAT", "", "Inbox"))
	to := mustItem(t)(anna.CreateFolder("PLAT", "", "Trips"))
	lat, lon, alt := 46.5584, 7.8352, 1200.0
	_, err := anna.AddAttachment(from.ID, Attachment{ID: albumPic, Name: "lake.jpg", Type: "image/jpeg", Size: 3, SHA256: strings.Repeat("ab", 32),
		Photo: &Photo{Taken: "2024-05-01T13:22:10", Lat: &lat, Lon: &lon, Alt: &alt, PlacePrivate: true}})
	noErr(t, err)
	noErr(t, dora.MoveFile(from.ID, albumPic, to.ID, []ID{albumPic}))
	mine, err := anna.Attachment(to.ID, albumPic)
	noErr(t, err)
	theirs, err := ben.Attachment(to.ID, albumPic)
	noErr(t, err)
	if !mine.Photo.HasPlace() || *mine.Photo.Lat != lat || mine.Author != "anna" {
		t.Errorf("the uploader's view %+v", mine)
	}
	if theirs.Photo.HasPlace() {
		t.Errorf("another reader sees the private place %+v", theirs.Photo)
	}
	// The moved version's photo data share nothing with the original's.
	c := clonePhoto(&Photo{Lat: &lat, Lon: &lon, Alt: &alt})
	if c.Lat == &lat || c.Lon == &lon || c.Alt == &alt || *c.Lat != lat || clonePhoto(nil) != nil {
		t.Errorf("clone %+v", c)
	}
}
