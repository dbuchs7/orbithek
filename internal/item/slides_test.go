package item

import (
	"errors"
	"strings"
	"testing"
)

const (
	slide1 = "11111111111111111111111111111111"
	slide2 = "22222222222222222222222222222222"
	slide3 = "33333333333333333333333333333333"
)

// A slide deck in a text keeps its slides as files of its item, each
// named once, with the folder they came from and a plain caption
// (ADR-0260).
func TestASlideDeckKeepsFilesOfItsItemAndNamesItsSource(t *testing.T) {
	good := []Block{
		{Kind: Slides, Ref: "f", Files: []ID{slide1}},
		{Kind: Slides, Ref: "f", Files: []ID{slide1, slide2}, Text: "Board, 4 December"},
		{Kind: Slides, Files: []ID{slide1}}, // its source is gone
		{Kind: Slides, Ref: "f"},            // kept no slides yet (codec version 71)
	}
	for i, b := range good {
		if err := checkBlock(b, "self"); err != nil {
			t.Errorf("good %d: %v", i, err)
		}
	}
	many := make([]ID, MaxSlides+1)
	for i := range many {
		many[i] = ID(strings.Repeat("0", 28) + string(rune('a'+i%26)) + "000")
	}
	bad := map[string]struct {
		b    Block
		want string
	}{
		"nothing":             {Block{Kind: Slides}, "keeps its slides"},
		"itself as source":    {Block{Kind: Slides, Ref: "self", Files: []ID{slide1}}, "not its own item"},
		"marks":               {Block{Kind: Slides, Ref: "f", Files: []ID{slide1}, Text: "Board", Marks: []Mark{{Type: MarkBold, Start: 0, End: 5}}}, "plain caption"},
		"mentions":            {Block{Kind: Slides, Ref: "f", Files: []ID{slide1}, Mentions: []ID{"x"}}, "plain caption"},
		"aligned":             {Block{Kind: Slides, Ref: "f", Files: []ID{slide1}, Align: "center"}, "no alignment"},
		"empty list":          {Block{Kind: Slides, Ref: "f", Files: []ID{}}, "1 to"},
		"too many":            {Block{Kind: Slides, Ref: "f", Files: many}, "1 to"},
		"twice":               {Block{Kind: Slides, Ref: "f", Files: []ID{slide1, slide1}}, "each slide once"},
		"no file ID":          {Block{Kind: Slides, Ref: "f", Files: []ID{"x"}}, "each slide once"},
		"files elsewhere":     {Block{Kind: Image, Ref: slide1, Files: []ID{slide2}}, "only a slide deck"},
		"reference elsewhere": {Block{Kind: Paragraph, Text: "x", Ref: "f"}, "slide deck"},
	}
	for name, c := range bad {
		if err := checkBlock(c.b, "self"); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want an error with %q", name, err, c.want)
		}
	}
	if _, ok := (Block{Kind: Slides, Ref: "f"}).Embedded(); ok {
		t.Error("a slide deck's source is no embed")
	}
	if got := (Block{Kind: Slides, Files: []ID{slide1, slide2}}).shownFiles(); len(got) != 2 {
		t.Errorf("a deck shows its slides: %v", got)
	}
}

// A deck's slides are files of its item; a version of the text keeps the
// slides it showed. Its source is a folder or collection the writer
// reads, and no reference: the folder may be deleted while texts keep
// their slides (ADR-0260).
func TestASlideDeckKeepsItsSlidesAndItsSourceMayGo(t *testing.T) {
	s := setup(t)
	noErr(t, s.GrantRole("admin", "PLAT", "anna", Editor))
	noErr(t, s.GrantRole("admin", "PLAT", "bert", Editor))
	noErr(t, s.GrantRole("admin", "OPS", "anna", Editor))
	anna, bert := s.As("anna"), s.As("bert")
	page := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Review"}))
	folder := mustItem(t)(anna.CreateFolder("PLAT", "", "Talk"))
	coll := mustItem(t)(anna.CreateCollection("PLAT", "", "Logos", CollectionRule{Labels: []string{"logo"}}))
	other := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Other"}))
	hidden := mustItem(t)(anna.CreateFolder("OPS", "", "Secret talk"))
	for _, f := range []ID{slide1, slide2, slide3} {
		addPicture(t, anna, page.ID, f, string(f[:2])+".png")
	}

	deck := Doc{Blocks: []Block{{Kind: Slides, Ref: folder.ID, Files: []ID{slide2, slide1}, Text: "Board"}, {Kind: Slides, Ref: coll.ID, Files: []ID{slide3}}}}
	noErr(t, anna.CommitBody(page.ID, deck))
	for _, r := range s.Relations(page.ID, Outgoing) {
		if r.To == folder.ID || r.To == coll.ID {
			t.Errorf("a deck implies no relation to its source: %+v", r)
		}
	}
	// The slides are files the body shows: the album says they are used.
	if pics := mustPictures(t, anna, page.ID); !pics[slide1] || !pics[slide3] {
		t.Errorf("the slides are used by the body: %v", pics)
	}
	// Only files of the item, and a source that is a folder or collection.
	cases := map[string]struct {
		d    Doc
		want error
		text string
	}{
		"a file of another item": {Doc{Blocks: []Block{{Kind: Slides, Ref: folder.ID, Files: []ID{albumPic}}}}, ErrNotFound, ""},
		"a page as source":       {Doc{Blocks: []Block{{Kind: Slides, Ref: other.ID, Files: []ID{slide1}}}}, nil, "no album folder"},
		"an unknown source":      {Doc{Blocks: []Block{{Kind: Slides, Ref: "nothing", Files: []ID{slide1}}}}, ErrNotFound, ""},
		"no slides, newly":       {Doc{Blocks: []Block{{Kind: Slides, Ref: folder.ID}}}, nil, "copy them"},
	}
	for name, c := range cases {
		err := anna.CommitBody(page.ID, c.d)
		if err == nil || c.want != nil && !errors.Is(err, c.want) || c.text != "" && !strings.Contains(err.Error(), c.text) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if err := bert.CommitBody(page.ID, Doc{Blocks: []Block{{Kind: Slides, Ref: hidden.ID, Files: []ID{slide1}}}}); !errors.Is(err, ErrNotFound) {
		t.Errorf("a folder bert does not read is no source for him: %v", err)
	}
	// The folder goes; the deck keeps its slides and stays editable.
	noErr(t, anna.Trash(folder.ID))
	noErr(t, anna.CommitBody(page.ID, Doc{Blocks: []Block{{Kind: Slides, Ref: folder.ID, Files: []ID{slide2, slide1}, Text: "Board, again"}}}))
	if err := anna.CommitBody(page.ID, Doc{Blocks: []Block{{Kind: Slides, Ref: hidden.ID, Files: []ID{slide1}}, {Kind: Slides, Ref: folder.ID, Files: []ID{slide2}}}}); err != nil {
		t.Errorf("a source kept from the current body stays, even in the trash: %v", err)
	}
	// A deleted slide stays only while the current body shows it.
	noErr(t, anna.DeleteAttachment(page.ID, slide3))
	if err := anna.CommitBody(page.ID, Doc{Blocks: []Block{{Kind: Slides, Ref: folder.ID, Files: []ID{slide3}}}}); !errors.Is(err, ErrNotFound) {
		t.Errorf("a deleted slide is not newly shown: %v", err)
	}
	// A new item cannot start with a deck: its files come after it.
	if _, err := anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Copy", Body: &Doc{Blocks: []Block{{Kind: Slides, Ref: coll.ID, Files: []ID{slide1}}}}}); err == nil || !strings.Contains(err.Error(), "after creating") {
		t.Errorf("a new page with a deck: %v", err)
	}
	if bodyEmpty(Doc{Blocks: []Block{{Kind: Slides, Files: []ID{slide1}}}}) {
		t.Error("a body with a deck is not empty")
	}
}

// A deck written before slides were kept (codec version 71) stays while
// the text keeps it unchanged (ADR-0260).
func TestADeckWithoutSlidesStaysUnchanged(t *testing.T) {
	s := setup(t)
	noErr(t, s.GrantRole("admin", "PLAT", "anna", Editor))
	anna := s.As("anna")
	page := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Review"}))
	folder := mustItem(t)(anna.CreateFolder("PLAT", "", "Talk"))
	old := Block{Kind: Slides, Ref: folder.ID, Text: "Board"}
	// As an older binary wrote it.
	noErr(t, s.record("anna", Event{Kind: BodyCommitted, Item: page.ID, Body: &Doc{Blocks: []Block{old}}}))
	noErr(t, anna.CommitBody(page.ID, Doc{Blocks: []Block{{Kind: Paragraph, Text: "Intro"}, old}}))
	addPicture(t, anna, page.ID, slide1, "01.png")
	noErr(t, anna.CommitBody(page.ID, Doc{Blocks: []Block{{Kind: Slides, Ref: folder.ID, Files: []ID{slide1}, Text: "Board"}}}))
	if err := anna.CommitBody(page.ID, Doc{Blocks: []Block{old}}); err == nil {
		t.Error("a kept deck does not go back to keeping none")
	}
}

// The slides of a protected deck are protected files: whoever does not
// read the protection gets none of them (ADR-0160, ADR-0260).
func TestTheSlidesOfAProtectedDeckAreProtected(t *testing.T) {
	s := setup(t)
	noErr(t, s.GrantRole("admin", "PLAT", "anna", Editor))
	noErr(t, s.GrantRole("admin", "PLAT", "vera", Viewer))
	anna, vera := s.As("anna"), s.As("vera")
	page := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Review"}))
	folder := mustItem(t)(anna.CreateFolder("PLAT", "", "Talk"))
	addPicture(t, anna, page.ID, slide1, "01.png")
	addPicture(t, anna, page.ID, slide2, "02.png")
	noErr(t, anna.CommitBody(page.ID, Doc{Blocks: []Block{{Kind: Paragraph, Text: "Open"}, {Kind: Slides, Ref: folder.ID, Files: []ID{slide1, slide2}}}}))
	_, err := anna.ProtectBlocks(page.ID, 1, 1, nil)
	noErr(t, err)
	for _, f := range []ID{slide1, slide2} {
		if _, err := vera.Attachment(page.ID, f); err == nil {
			t.Errorf("vera reads the protected slide %s", f)
		}
		if _, err := anna.Attachment(page.ID, f); err != nil {
			t.Errorf("anna reads her slide %s: %v", f, err)
		}
	}
	// A slide may not show unprotected elsewhere in the text.
	body := mustItem(t)(anna.Get(page.ID)).Body
	body.Blocks = append(body.Blocks, Block{Kind: Image, Ref: slide1})
	if err := anna.CommitBody(page.ID, body); err == nil || !strings.Contains(err.Error(), "protected") {
		t.Errorf("a protected slide shown in the open: %v", err)
	}
}

// mustPictures lists which pictures of an item the album says its body
// uses, its slides among them.
func mustPictures(t *testing.T, x *Session, id ID) map[ID]bool {
	t.Helper()
	out := map[ID]bool{}
	for _, p := range x.PicturesWithSlides("", func(Attachment) bool { return true }, id) {
		if p.Item.ID == id {
			out[p.Attachment.ID] = p.Used
		}
	}
	return out
}

// The slides a deck keeps are left out of the album, also those only an
// earlier version of the text showed and new versions of a slide; the
// album lists them when it lists their own item (ADR-0260).
func TestTheAlbumLeavesOutTheSlidesADeckKeeps(t *testing.T) {
	s := setup(t)
	noErr(t, s.GrantRole("admin", "PLAT", "anna", Editor))
	anna := s.As("anna")
	page := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Review"}))
	folder := mustItem(t)(anna.CreateFolder("PLAT", "", "Talk"))
	addPicture(t, anna, folder.ID, albumPic, "01 Intro.png")
	addPicture(t, anna, page.ID, slide1, "01 Intro.png")
	addPicture(t, anna, page.ID, slide2, "02 Plan.png")
	addPicture(t, anna, page.ID, slide3, "A picture.png")
	noErr(t, anna.CommitBody(page.ID, Doc{Blocks: []Block{{Kind: Slides, Ref: folder.ID, Files: []ID{slide1}}, {Kind: Image, Ref: slide3}}}))
	// Updated: the second version keeps another copy; the first stays out.
	noErr(t, anna.CommitBody(page.ID, Doc{Blocks: []Block{{Kind: Slides, Ref: folder.ID, Files: []ID{slide2}}, {Kind: Image, Ref: slide3}}}))
	// A new version of a slide, as a drawing saved again, stays out too.
	v2 := ID("44444444444444444444444444444444")
	_, err := anna.AddAttachment(page.ID, Attachment{ID: v2, Name: "02 Plan.png", Type: "image/png", Size: 3, SHA256: strings.Repeat("cd", 32),
		Source: &Source{Item: page.ID, Attachment: slide2}})
	noErr(t, err)
	listed := func(pics []Picture) map[ID]bool {
		out := map[ID]bool{}
		for _, p := range pics {
			out[p.Attachment.ID] = true
		}
		return out
	}
	all := func(Attachment) bool { return true }
	album := listed(anna.Pictures("", all))
	if album[slide1] || album[slide2] || album[v2] {
		t.Errorf("the album shows slides: %v", album)
	}
	if !album[slide3] || !album[albumPic] {
		t.Errorf("the album shows the page's picture and the folder's: %v", album)
	}
	own := listed(anna.PicturesWithSlides("", all, page.ID))
	if !own[slide1] || !own[slide2] || !own[v2] || !own[slide3] {
		t.Errorf("the page's own listing shows its slides: %v", own)
	}
	// Replayed from the log, the slides stay out.
	r, err := Replay(s.Log())
	noErr(t, err)
	if again := listed(r.As("anna").Pictures("", all)); again[slide1] || again[slide2] {
		t.Errorf("after replay the album shows slides: %v", again)
	}
}
