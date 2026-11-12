package item

import (
	"errors"
	"slices"
	"testing"
)

// gathered lists the names of the pictures a collection gathers for x.
func gathered(t *testing.T, x *Session, id ID) []string {
	t.Helper()
	_, rule, err := x.Collection(id)
	noErr(t, err)
	var out []string
	for _, p := range x.Pictures("", func(Attachment) bool { return true }) {
		if rule.Matches(p) {
			out = append(out, p.Attachment.Name)
		}
	}
	slices.Sort(out)
	return out
}

// A collection gathers the labelled pictures of every room, each reader
// seeing only those they may read (ADR-0175).
func TestACollectionGathersLabelledPicturesOfEveryRoom(t *testing.T) {
	s := permFixture(t)
	noErr(t, s.GrantRole("root", "OPS", "anna", Viewer))
	anna, ben, dora := s.As("anna"), s.As("ben"), s.As("dora")

	folder := mustItem(t)(anna.CreateFolder("PLAT", "", "Logos"))
	addPicture(t, anna, folder.ID, albumPic, "plat.png")
	noErr(t, anna.SetPictureLabels(folder.ID, albumPic, []string{"logo", "print"}))
	ops := mustItem(t)(dora.Create(NewItem{Space: "OPS", Type: "page", Title: "Ops"}))
	addPicture(t, dora, ops.ID, "11111111111111111111111111111111", "ops.png")
	noErr(t, dora.SetPictureLabels(ops.ID, "11111111111111111111111111111111", []string{"logo"}))
	page := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Brand"}))
	addPicture(t, anna, page.ID, "22222222222222222222222222222222", "page.png")
	noErr(t, anna.SetLabels(page.ID, []string{"logo"}))

	if _, err := ben.CreateCollection("PLAT", "", "Logos", CollectionRule{Labels: []string{"logo"}}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("a viewer made a collection: %v", err)
	}
	logos := mustItem(t)(anna.CreateCollection("PLAT", "", "Logos", CollectionRule{Labels: []string{"Logo"}}))
	if logos.Type != CollectionType || logos.Properties["rule"] != "logo" || logos.Properties["match"] != "any" || logos.Properties["source"] != "pictures" {
		t.Fatalf("collection %+v", logos)
	}
	if got := gathered(t, anna, logos.ID); !slices.Equal(got, []string{"ops.png", "plat.png"}) {
		t.Errorf("anna gathers %v", got)
	}
	// Ben reads PLAT only: the picture of OPS is not his to see (D10).
	if got := gathered(t, ben, logos.ID); !slices.Equal(got, []string{"plat.png"}) {
		t.Errorf("ben gathers %v", got)
	}
	if _, _, err := s.As("carl").Collection(logos.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("an outsider reads the collection: %v", err)
	}

	// The labels of a picture's item count when the rule says so.
	noErr(t, anna.SetCollectionRule(logos.ID, CollectionRule{Labels: []string{"logo"}, Items: true}))
	if got := gathered(t, anna, logos.ID); !slices.Equal(got, []string{"ops.png", "page.png", "plat.png"}) {
		t.Errorf("with item labels anna gathers %v", got)
	}
	// Every label, not any one.
	noErr(t, anna.SetCollectionRule(logos.ID, CollectionRule{Labels: []string{"print", "logo"}, All: true}))
	if got := gathered(t, anna, logos.ID); !slices.Equal(got, []string{"plat.png"}) {
		t.Errorf("with every label anna gathers %v", got)
	}
	it, err := anna.Get(logos.ID)
	noErr(t, err)
	if it.Properties["rule"] != "logo, print" || it.Properties["match"] != "all" || it.Properties["source"] != "pictures" {
		t.Errorf("rule kept as %v", it.Properties)
	}
	if err := ben.SetCollectionRule(logos.ID, CollectionRule{Labels: []string{"x"}}); !errors.Is(err, ErrForbidden) {
		t.Errorf("a viewer changed the rule: %v", err)
	}
	if err := anna.SetCollectionRule(folder.ID, CollectionRule{Labels: []string{"x"}}); !errors.Is(err, ErrNotFound) {
		t.Errorf("a folder took a rule: %v", err)
	}
	if _, _, err := anna.Collection(folder.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("a folder read as a collection: %v", err)
	}
}

func TestACollectionIsPlacedAsAFolderIs(t *testing.T) {
	s := permFixture(t)
	anna, ben := s.As("anna"), s.As("ben")
	rule := CollectionRule{Labels: []string{"logo"}}
	folder := mustItem(t)(anna.CreateFolder("PLAT", "", "Brand"))
	inside := mustItem(t)(anna.CreateCollection("OPS", folder.ID, "Logos", rule))
	if inside.Space != "PLAT" || inside.Parent != folder.ID {
		t.Errorf("a collection in a folder takes its room: %+v", inside)
	}
	mustItem(t)(anna.CreateCollection("PLAT", "", "archive", rule))
	if got := ben.Collections("PLAT", ""); len(got) != 1 || got[0].Item.Title != "archive" || got[0].Editable {
		t.Errorf("top collections for a viewer %+v", got)
	}
	if got := anna.Collections("", folder.ID); len(got) != 1 || got[0].Item.ID != inside.ID || !got[0].Editable || got[0].Rule.Labels[0] != "logo" {
		t.Errorf("collections in the folder %+v", got)
	}
	if got := ben.Folders("PLAT", ""); len(got) != 1 {
		t.Errorf("a collection lists as a folder: %+v", got)
	}
	if _, err := anna.CreateCollection("", inside.ID, "x", rule); !errors.Is(err, ErrValidation) {
		t.Errorf("a collection in a collection: %v", err)
	}
	if _, err := anna.CreateFolder("", inside.ID, "x"); !errors.Is(err, ErrValidation) {
		t.Errorf("a folder in a collection: %v", err)
	}
	for _, bad := range []CollectionRule{{}, {Labels: []string{"no good!"}}, {Labels: make([]string, maxPictureLabels+1)}} {
		if _, err := anna.CreateCollection("PLAT", "", "x", bad); !errors.Is(err, ErrValidation) {
			t.Errorf("rule %v: %v", bad, err)
		}
	}
	many := make([]string, maxPictureLabels+1)
	for i := range many {
		many[i] = "l" + string(rune('a'+i))
	}
	if _, err := anna.CreateCollection("PLAT", "", "x", CollectionRule{Labels: many}); !errors.Is(err, ErrValidation) {
		t.Errorf("too many labels: %v", err)
	}
}

func TestACollectionTypeOfAnotherShapeIsNotTaken(t *testing.T) {
	s := permFixture(t)
	noErr(t, s.DefineType("root", TypeDef{Name: CollectionType, Body: true}))
	if _, err := s.As("anna").CreateCollection("PLAT", "", "Logos", CollectionRule{Labels: []string{"logo"}}); !errors.Is(err, ErrValidation) {
		t.Fatalf("took a type of another shape: %v", err)
	}
	other := collectionType()
	other.Fields = append(other.Fields, FieldDef{Name: "owner", Kind: KindUser, Required: true})
	if isCollectionType(other) {
		t.Error("a type with another required field passes")
	}
	other = collectionType()
	other.Fields[1].Values = []string{"any"}
	if isCollectionType(other) {
		t.Error("a type with other match values passes")
	}
	other = collectionType()
	other.Fields = append(other.Fields, FieldDef{Name: "note", Kind: KindText})
	if !isCollectionType(other) {
		t.Error("a type with a further optional field is refused")
	}
}

// A rule typed on the collection's own page is read leniently: what is no
// label is passed over, and a rule without labels gathers nothing.
func TestARuleIsReadFromTheProperties(t *testing.T) {
	r := ruleOfProps(Props{"rule": "Print, logo,, no good!, logo", "match": "all", "source": "pictures and items"})
	if !slices.Equal(r.Labels, []string{"logo", "print"}) || !r.All || !r.Items {
		t.Errorf("rule %+v", r)
	}
	if (CollectionRule{}).Matches(Picture{Attachment: Attachment{Labels: []string{"logo"}}}) {
		t.Error("a rule without labels gathers")
	}
}

// A collection keeps the folder its new pictures go into, for everyone
// who adds to it; its type gains the field when one is first kept, and a
// folder a reader may not read is no folder for them (ADR-0221).
func TestACollectionKeepsTheFolderOfItsNewPictures(t *testing.T) {
	s := permFixture(t)
	noErr(t, s.GrantRole("root", "OPS", "anna", Editor))
	anna, ben := s.As("anna"), s.As("ben")
	logos := mustItem(t)(anna.CreateFolder("PLAT", "", "Logos"))
	secret := mustItem(t)(anna.CreateFolder("OPS", "", "Secret"))
	prints := mustItem(t)(anna.CreateCollection("PLAT", "", "Print", CollectionRule{Labels: []string{"print"}}))
	folderOf := func(x *Session) ID {
		t.Helper()
		for _, c := range x.Collections("PLAT", "") {
			if c.Item.ID == prints.ID {
				return c.Folder
			}
		}
		t.Fatalf("the collection is not listed")
		return ""
	}
	if folderOf(anna) != "" {
		t.Fatalf("a new collection keeps a folder")
	}
	before, _ := s.latest(CollectionType)
	if _, ok := before.field(fieldFolder); ok {
		t.Fatalf("the first type of collections has the folder field already")
	}

	if err := ben.SetCollectionFolder(prints.ID, logos.ID); !errors.Is(err, ErrForbidden) {
		t.Errorf("a viewer kept a folder: %v", err)
	}
	if err := anna.SetCollectionFolder(prints.ID, prints.ID); !errors.Is(err, ErrValidation) {
		t.Errorf("a collection kept itself as its folder: %v", err)
	}
	noErr(t, anna.SetCollectionFolder(prints.ID, logos.ID))
	after, _ := s.latest(CollectionType)
	if f, ok := after.field(fieldFolder); !ok || after.Version != before.Version+1 || f.Kind != KindItem || f.Required {
		t.Fatalf("the type gains the folder as its next version: %+v", after)
	}
	if !isCollectionType(after) {
		t.Fatalf("the type with the folder is still the album's collection")
	}
	if folderOf(anna) != logos.ID || folderOf(ben) != logos.ID {
		t.Errorf("everyone who reads the folder sees it kept: %q, %q", folderOf(anna), folderOf(ben))
	}
	// Another collection made now uses the type with the folder field.
	mustItem(t)(anna.CreateCollection("PLAT", "", "Logos", CollectionRule{Labels: []string{"logo"}}))
	// The same folder again changes nothing; another one replaces it.
	noErr(t, anna.SetCollectionFolder(prints.ID, logos.ID))
	noErr(t, anna.SetCollectionFolder(prints.ID, secret.ID))
	if folderOf(anna) != secret.ID {
		t.Errorf("anna sees %q", folderOf(anna))
	}
	// The kept folder and the type's new version survive a replay.
	re, err := Replay(s.Log())
	noErr(t, err)
	if got := re.As("anna").Collections("PLAT", ""); len(got) != 2 || got[1].Item.ID != prints.ID || got[1].Folder != secret.ID {
		t.Errorf("after a replay %+v", got)
	}
	// Ben may not read OPS: its folder does not exist for him (D10).
	if folderOf(ben) != "" {
		t.Errorf("ben sees a folder he may not read: %q", folderOf(ben))
	}
	noErr(t, anna.SetCollectionFolder(prints.ID, ""))
	if folderOf(anna) != "" {
		t.Errorf("a forgotten folder is still kept")
	}
	// A collection whose type lacks the field forgets nothing without
	// defining a version.
	other := mustItem(t)(anna.CreateCollection("PLAT", "", "Other", CollectionRule{Labels: []string{"x"}}))
	noErr(t, anna.SetCollectionFolder(other.ID, ""))
}
