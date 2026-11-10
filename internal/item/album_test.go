package item

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

const albumPic = "0123456789abcdef0123456789abcdef"

func addPicture(t *testing.T, x *Session, id ID, aid ID, name string) {
	t.Helper()
	_, err := x.AddAttachment(id, Attachment{ID: aid, Name: name, Type: "image/png", Size: 3, SHA256: strings.Repeat("ab", 32)})
	noErr(t, err)
}

// Folders of the album are items: made where one may create, nested by
// the parent, read as their space allows (ADR-0090).
func TestFoldersOfTheAlbum(t *testing.T) {
	s := permFixture(t)
	anna, ben := s.As("anna"), s.As("ben")
	if _, err := ben.CreateFolder("PLAT", "", "Logos"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("a viewer made a folder: %v", err)
	}
	logos := mustItem(t)(anna.CreateFolder("PLAT", "", "Logos"))
	brand := mustItem(t)(anna.CreateFolder("", logos.ID, "Brand"))
	mustItem(t)(anna.CreateFolder("PLAT", "", "archive"))
	if brand.Space != "PLAT" || brand.Parent != logos.ID || logos.Type != FolderType {
		t.Fatalf("folders %+v %+v", logos, brand)
	}
	if got := ben.Folders("PLAT", ""); len(got) != 2 || got[0].Title != "archive" || got[1].Title != "Logos" {
		t.Errorf("top folders %+v", got)
	}
	if got := ben.Folders("", logos.ID); len(got) != 1 || got[0].ID != brand.ID {
		t.Errorf("sub folders %+v", got)
	}
	if got := s.As("carl").Folders("", ""); len(got) != 0 {
		t.Errorf("one without a role sees %+v", got)
	}
	page := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Runbook"}))
	if _, err := anna.CreateFolder("", page.ID, "x"); !errors.Is(err, ErrValidation) {
		t.Errorf("a folder in a page: %v", err)
	}
	if _, err := ben.CreateFolder("", logos.ID, "x"); !errors.Is(err, ErrForbidden) {
		t.Errorf("a viewer made a subfolder: %v", err)
	}
}

func TestAFolderTypeOfAnotherShapeIsNotTaken(t *testing.T) {
	s := permFixture(t)
	noErr(t, s.DefineType("root", TypeDef{Name: FolderType, Body: true}))
	if _, err := s.As("anna").CreateFolder("PLAT", "", "Logos"); !errors.Is(err, ErrValidation) {
		t.Fatalf("took a type of another shape: %v", err)
	}
}

// A picture's labels are its own: they are not the item's, so no kanban
// or label board shows them (ADR-0090).
func TestPicturesAreLabelled(t *testing.T) {
	s := permFixture(t)
	anna, ben := s.As("anna"), s.As("ben")
	folder := mustItem(t)(anna.CreateFolder("PLAT", "", "Logos"))
	addPicture(t, anna, folder.ID, albumPic, "logo.png")
	noErr(t, anna.SetPictureLabels(folder.ID, albumPic, []string{"Print", "brand", "print"}))
	noErr(t, anna.SetPictureLabels(folder.ID, albumPic, []string{"brand", "print"})) // no change
	if err := ben.SetPictureLabels(folder.ID, albumPic, []string{"x"}); !errors.Is(err, ErrForbidden) {
		t.Errorf("a viewer labelled: %v", err)
	}
	if err := anna.SetPictureLabels(folder.ID, albumPic, []string{"no good!"}); !errors.Is(err, ErrValidation) {
		t.Errorf("a bad label: %v", err)
	}
	if err := anna.SetPictureLabels(folder.ID, "fedcba9876543210fedcba9876543210", nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("an unknown picture: %v", err)
	}
	many := make([]string, maxPictureLabels+1)
	for i := range many {
		many[i] = "l" + strings.Repeat("x", i+1)
	}
	if err := anna.SetPictureLabels(folder.ID, albumPic, many); !errors.Is(err, ErrValidation) {
		t.Errorf("too many labels: %v", err)
	}
	got := mustItem(t)(ben.Get(folder.ID))
	if !slices.Equal(got.Attachments[0].Labels, []string{"brand", "print"}) || len(got.Labels) != 0 {
		t.Fatalf("labels %v, item labels %v", got.Attachments[0].Labels, got.Labels)
	}
	if labels := ben.Labels(""); len(labels) != 0 {
		t.Errorf("a picture's label counts as an item's: %v", labels)
	}
	re, err := Replay(s.Log())
	noErr(t, err)
	if r := mustItem(t)(re.As("ben").Get(folder.ID)); !slices.Equal(r.Attachments[0].Labels, []string{"brand", "print"}) {
		t.Errorf("replayed %v", r.Attachments[0].Labels)
	}
	noErr(t, anna.DeleteAttachment(folder.ID, albumPic))
	if err := anna.SetPictureLabels(folder.ID, albumPic, []string{"x"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("labelled a deleted picture: %v", err)
	}
}

// The album tells whether a picture is in its item's text and whether the
// reader may change it.
func TestPicturesSayWhetherTheyAreUsed(t *testing.T) {
	s := permFixture(t)
	anna := s.As("anna")
	page := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Plans"}))
	addPicture(t, anna, page.ID, albumPic, "used.png")
	const spare = "fedcba9876543210fedcba9876543210"
	addPicture(t, anna, page.ID, spare, "spare.png")
	noErr(t, anna.CommitBody(page.ID, Doc{Blocks: []Block{{Kind: Image, Ref: albumPic, Text: "Plan"}}}))
	all := func(Attachment) bool { return true }
	for who, editable := range map[string]bool{"anna": true, "ben": false} {
		pics := s.As(who).Pictures("PLAT", all)
		if len(pics) != 2 {
			t.Fatalf("%s sees %d pictures", who, len(pics))
		}
		for _, p := range pics {
			if p.Used != (p.Attachment.ID == albumPic) || p.Editable != editable || p.Item.Parent != "" {
				t.Errorf("%s: %s used %v editable %v", who, p.Attachment.Name, p.Used, p.Editable)
			}
		}
	}
}

// A picture's speaker's notes are kept on its first version: every
// version of a drawing shows them, whichever version set them, and they
// survive the deletion of the first version (ADR-0176).
func TestPictureNotesFollowTheStack(t *testing.T) {
	s := permFixture(t)
	anna, ben := s.As("anna"), s.As("ben")
	folder := mustItem(t)(anna.CreateFolder("PLAT", "", "Talk"))
	const v1, v2 = "0123456789abcdef0123456789abcd01", "0123456789abcdef0123456789abcd02"
	addPicture(t, anna, folder.ID, v1, "01 Intro.drawing.png")
	_, err := anna.AddAttachment(folder.ID, Attachment{ID: v2, Name: "01 Intro.drawing.png", Type: "image/png", Size: 3,
		SHA256: strings.Repeat("cd", 32), Source: &Source{Item: folder.ID, Attachment: v1}})
	noErr(t, err)

	noErr(t, anna.SetPictureNotes(folder.ID, v2, "  Welcome.\r\nAsk who used it.\r  "))
	n := len(s.Log())
	noErr(t, anna.SetPictureNotes(folder.ID, v1, "Welcome.\nAsk who used it.")) // no change
	if len(s.Log()) != n {
		t.Error("unchanged notes were recorded")
	}
	if err := ben.SetPictureNotes(folder.ID, v2, "x"); !errors.Is(err, ErrForbidden) {
		t.Errorf("a viewer wrote notes: %v", err)
	}
	if err := anna.SetPictureNotes(folder.ID, "fedcba9876543210fedcba9876543210", "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("notes of an unknown picture: %v", err)
	}
	if err := anna.SetPictureNotes(folder.ID, v2, "\xff"); !errors.Is(err, ErrValidation) {
		t.Errorf("notes that are no UTF-8: %v", err)
	}
	all := func(Attachment) bool { return true }
	notes := func(x *Session) map[ID]string {
		out := map[ID]string{}
		for _, p := range x.Pictures("PLAT", all) {
			out[p.Attachment.ID] = p.Notes
		}
		return out
	}
	const want = "Welcome.\nAsk who used it."
	if got := notes(ben); got[v1] != want || got[v2] != want {
		t.Fatalf("notes %q", got)
	}
	re, err := Replay(s.Log())
	noErr(t, err)
	if got := notes(re.As("ben")); got[v2] != want {
		t.Errorf("replayed %q", got)
	}
	noErr(t, anna.DeleteAttachment(folder.ID, v1))
	if got := notes(ben); got[v2] != want {
		t.Errorf("after the first version was deleted %q", got)
	}
	noErr(t, anna.SetPictureNotes(folder.ID, v2, ""))
	if got := notes(ben); got[v2] != "" {
		t.Errorf("cleared notes %q", got)
	}
}

// Notes are content: sensitive data in them restricts the item at once,
// and purging the item erases them (ADR-0026, ADR-0040).
func TestPictureNotesAreContent(t *testing.T) {
	s := permFixture(t)
	anna, ben := s.As("anna"), s.As("ben")
	folder := mustItem(t)(anna.CreateFolder("PLAT", "", "Talk"))
	addPicture(t, anna, folder.ID, albumPic, "slide.png")
	noErr(t, anna.SetPictureNotes(folder.ID, albumPic, "password: Sommer2026!"))
	if _, err := ben.Get(folder.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("sensitive notes left the folder readable: %v", err)
	}
	e := s.Log()[len(s.Log())-2]
	if e.Kind != AttachmentNotesSet || !e.HasContent() {
		t.Fatalf("event %+v", e)
	}
	e.erase()
	if e.Attachment != nil || !e.Erased {
		t.Errorf("erased %+v", e)
	}
}

// Renaming a picture renames every version of it and keeps its extension,
// and a drawing its marker; a copy in another item keeps its name
// (ADR-0180).
func TestRenamingAPictureRenamesItsVersions(t *testing.T) {
	s := permFixture(t)
	anna, ben := s.As("anna"), s.As("ben")
	folder := mustItem(t)(anna.CreateFolder("PLAT", "", "Plans"))
	const v1, v2, photo = "0123456789abcdef0123456789abcf01", "0123456789abcdef0123456789abcf02", "0123456789abcdef0123456789abcf03"
	addPicture(t, anna, folder.ID, v1, "Sketch.drawing.png")
	_, err := anna.AddAttachment(folder.ID, Attachment{ID: v2, Name: "Sketch.drawing.png", Type: "image/png", Size: 3,
		SHA256: strings.Repeat("cd", 32), Source: &Source{Item: folder.ID, Attachment: v1}})
	noErr(t, err)
	addPicture(t, anna, folder.ID, photo, "IMG_0042.JPG")
	page := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Copy"}))
	const copied = "0123456789abcdef0123456789abcf04"
	_, err = anna.AddAttachment(page.ID, Attachment{ID: copied, Name: "Sketch.drawing.png", Type: "image/png", Size: 3,
		SHA256: strings.Repeat("cd", 32), Source: &Source{Item: folder.ID, Attachment: v2}})
	noErr(t, err)

	names := func(x *Session, id ID) map[ID]string {
		out := map[ID]string{}
		atts, err := x.Attachments(id)
		noErr(t, err)
		for _, a := range atts {
			out[a.ID] = a.Name
		}
		return out
	}
	noErr(t, anna.RenameFile(folder.ID, v1, "  01 Floor plan  "))
	if got := names(ben, folder.ID); got[v1] != "01 Floor plan.drawing.png" || got[v2] != "01 Floor plan.drawing.png" {
		t.Fatalf("names after renaming %v", got)
	}
	if got := names(ben, page.ID); got[copied] != "Sketch.drawing.png" {
		t.Errorf("the copy was renamed: %v", got)
	}
	noErr(t, anna.RenameFile(folder.ID, photo, "Lake at dawn.jpg")) // the extension typed is not doubled
	if got := names(ben, folder.ID)[photo]; got != "Lake at dawn.JPG" {
		t.Errorf("photo renamed to %q", got)
	}
	n := len(s.Log())
	noErr(t, anna.RenameFile(folder.ID, v2, "01 Floor plan")) // no change
	if len(s.Log()) != n {
		t.Error("an unchanged name was recorded")
	}
	for name, err := range map[string]error{
		"a viewer":      ben.RenameFile(folder.ID, v1, "x"),
		"an empty name": anna.RenameFile(folder.ID, v1, " .drawing.png "),
		"a slash":       anna.RenameFile(folder.ID, v1, "a/b"),
		"too long":      anna.RenameFile(folder.ID, v1, strings.Repeat("x", 250)),
		"an unknown":    anna.RenameFile(folder.ID, "fedcba9876543210fedcba9876543210", "x"),
	} {
		want := ErrValidation
		switch name {
		case "a viewer":
			want = ErrForbidden
		case "an unknown":
			want = ErrNotFound
		}
		if !errors.Is(err, want) {
			t.Errorf("%s: %v, want %v", name, err, want)
		}
	}
	re, err := Replay(s.Log())
	noErr(t, err)
	if got := names(re.As("ben"), folder.ID); got[v2] != "01 Floor plan.drawing.png" {
		t.Errorf("replayed %v", got)
	}
	// A later version of the renamed drawing is found under the new name
	// as a stack: the album groups by first version, not by name.
	noErr(t, anna.DeleteAttachment(folder.ID, v1))
	noErr(t, anna.RenameFile(folder.ID, v2, "Floor plan"))
	if got := names(ben, folder.ID)[v2]; got != "Floor plan.drawing.png" {
		t.Errorf("renamed after deleting the first version: %q", got)
	}
}

// A name is content: sensitive data in it restricts the item at once,
// and purging erases it (ADR-0026, ADR-0040).
func TestAPictureNameIsContent(t *testing.T) {
	s := permFixture(t)
	anna, ben := s.As("anna"), s.As("ben")
	folder := mustItem(t)(anna.CreateFolder("PLAT", "", "Plans"))
	addPicture(t, anna, folder.ID, albumPic, "plan.png")
	noErr(t, anna.RenameFile(folder.ID, albumPic, "password: Sommer2026!"))
	if _, err := ben.Get(folder.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a sensitive name left the folder readable: %v", err)
	}
	e := s.Log()[len(s.Log())-2]
	if e.Kind != AttachmentRenamed || !e.HasContent() {
		t.Fatalf("event %+v", e)
	}
	e.erase()
	if e.Attachment != nil || !e.Erased {
		t.Errorf("erased %+v", e)
	}
}

// Any file of an item is renamed as a picture is, from its file list:
// its extension stays, and a name without one stays without (ADR-0181).
func TestAnyFileIsRenamed(t *testing.T) {
	s := permFixture(t)
	anna, ben := s.As("anna"), s.As("ben")
	page := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Report"}))
	const pdf, plain = "0123456789abcdef0123456789abcf11", "0123456789abcdef0123456789abcf12"
	for id, name := range map[ID]string{pdf: "scan_0001.pdf", plain: "README"} {
		_, err := anna.AddAttachment(page.ID, Attachment{ID: id, Name: name, Type: "application/octet-stream", Size: 3, SHA256: strings.Repeat("ef", 32)})
		noErr(t, err)
	}
	noErr(t, anna.RenameFile(page.ID, pdf, "Annual report 2026"))
	noErr(t, anna.RenameFile(page.ID, plain, "Read me first"))
	atts, err := ben.Attachments(page.ID)
	noErr(t, err)
	got := map[ID]string{}
	for _, a := range atts {
		got[a.ID] = a.Name
	}
	if got[pdf] != "Annual report 2026.pdf" || got[plain] != "Read me first" {
		t.Fatalf("names %v", got)
	}
	if err := ben.RenameFile(page.ID, pdf, "x"); !errors.Is(err, ErrForbidden) {
		t.Errorf("a viewer renamed a file: %v", err)
	}
}
