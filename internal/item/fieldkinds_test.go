package item

import (
	"errors"
	"reflect"
	"slices"
	"testing"
)

// storyType has a field of every kind of ADR-0202.
func storyType() TypeDef {
	return TypeDef{Name: "story", Body: true, Fields: []FieldDef{
		{Name: "epic", Kind: KindItem, Targets: []string{"page", "story"}},
		{Name: "anything", Kind: KindItem},
		{Name: "platforms", Kind: KindMulti, Values: []string{"web", "ios", "android"}},
		{Name: "rating", Kind: KindRating},
		{Name: "cover", Kind: KindFile},
	}}
}

func storyFixture(t *testing.T) *Store {
	t.Helper()
	s := permFixture(t)
	noErr(t, s.DefineType("root", storyType()))
	return s
}

func TestTheNewFieldKindsAreDefinedWithTheirOptions(t *testing.T) {
	s := setup(t)
	bad := map[string]FieldDef{
		"multi without values":  {Name: "m", Kind: KindMulti},
		"multi value twice":     {Name: "m", Kind: KindMulti, Values: []string{"a", "a"}},
		"multi empty value":     {Name: "m", Kind: KindMulti, Values: []string{"a", " "}},
		"targets on a text":     {Name: "x", Kind: KindText, Targets: []string{"page"}},
		"values on a rating":    {Name: "x", Kind: KindRating, Values: []string{"1"}},
		"target named twice":    {Name: "x", Kind: KindItem, Targets: []string{"page", "page"}},
		"unique item reference": {Name: "x", Kind: KindItem, Unique: true},
	}
	for name, f := range bad {
		if err := s.DefineType("admin", TypeDef{Name: "t", Fields: []FieldDef{f}}); !errors.Is(err, ErrValidation) {
			t.Errorf("%s: err = %v, want ErrValidation", name, err)
		}
	}
	// A field may refer to its own type.
	noErr(t, s.DefineType("admin", TypeDef{Name: "epic", Fields: []FieldDef{{Name: "parent-epic", Kind: KindItem, Targets: []string{"epic"}}}}))
}

func TestValuesOfTheNewFieldKinds(t *testing.T) {
	s := storyFixture(t)
	anna := s.As("anna")
	page := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Epic"}))
	task := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "task", Title: "Task", Properties: Props{"priority": "low"}}))

	st := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "story", Title: "Login", Properties: Props{
		"epic": page.ID, "platforms": []string{"android", "web"}, "rating": 4,
	}}))
	if got := st.Properties["platforms"]; !reflect.DeepEqual(got, []string{"web", "android"}) {
		t.Fatalf("platforms = %#v, want them in the order of the values", got)
	}
	if st.Properties["epic"] != page.ID || st.Properties["rating"] != 4 {
		t.Fatalf("properties %#v", st.Properties)
	}

	bad := map[string]struct {
		field string
		value any
	}{
		"rating too high":      {"rating", 6},
		"rating zero":          {"rating", 0},
		"rating not whole":     {"rating", 2.5},
		"unknown platform":     {"platforms", []string{"web", "palm"}},
		"multi as a text":      {"platforms", "web"},
		"reference as text":    {"epic", string(page.ID)},
		"target type":          {"epic", task.ID},
		"unknown target":       {"anything", ID("nope")},
		"itself":               {"anything", st.ID},
		"file it does not own": {"cover", fileA},
		"file not an ID":       {"cover", "../x"},
	}
	for name, c := range bad {
		if err := anna.SetProperty(st.ID, c.field, c.value); err == nil {
			t.Errorf("%s: set", name)
		}
	}
	// An empty choice unsets a multi field.
	noErr(t, anna.SetProperty(st.ID, "platforms", []string{}))
	if got := mustItem(t)(anna.Get(st.ID)); got.Properties["platforms"] != nil {
		t.Fatalf("platforms = %#v after an empty choice", got.Properties["platforms"])
	}
	// A file field names a file of the item, set once it has one.
	if _, err := anna.Create(NewItem{Space: "PLAT", Type: "story", Title: "x", Properties: Props{"cover": fileA}}); !errors.Is(err, ErrValidation) {
		t.Fatalf("a new item names a file: %v", err)
	}
	_, err := anna.AddAttachment(st.ID, testFile(fileA, "cover.png"))
	noErr(t, err)
	noErr(t, anna.SetProperty(st.ID, "cover", fileA))
	noErr(t, anna.DeleteAttachment(st.ID, ID(fileA)))
	if err := anna.SetProperty(st.ID, "rating", 5); err != nil {
		t.Fatalf("another field, while the cover's file is deleted: %v", err)
	}
	if err := anna.SetProperty(st.ID, "cover", fileA); err != nil {
		t.Fatalf("setting the deleted file it names already: %v", err)
	}
	if FormatValue([]string{"web", "ios"}) != "web, ios" || FormatValue(page.ID) != string(page.ID) {
		t.Fatal("FormatValue")
	}
}

func TestAnItemFieldIsARelationEverywhere(t *testing.T) {
	s := storyFixture(t)
	anna := s.As("anna")
	page := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Epic"}))
	other := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Other"}))
	st := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "story", Title: "Login", Properties: Props{"epic": page.ID}}))

	in, err := anna.Relations(page.ID, Incoming)
	noErr(t, err)
	if len(in) != 1 || in[0].From != st.ID || in[0].Type != "epic" || !in[0].Derived || in[0].Field() != "epic" {
		t.Fatalf("incoming %+v", in)
	}
	// It protects its target from deletion (D8).
	if err := anna.Trash(page.ID); !errors.Is(err, ErrReferenced) {
		t.Fatalf("trashed a referenced item: %v", err)
	}
	// Committing a body keeps it: it follows the property, not the body.
	noErr(t, anna.CommitBody(st.ID, Doc{Blocks: []Block{{Kind: Paragraph, Text: "see", Mentions: []ID{other.ID}, Marks: []Mark{{Type: MarkMention, Start: 0, End: 3, Ref: other.ID}}}}}))
	out, _ := anna.Relations(st.ID, Outgoing)
	if len(out) != 2 {
		t.Fatalf("outgoing after a body %+v", out)
	}
	// The starmap and the impact view show it as a relation named by the
	// field.
	sm, err := anna.Starmap("PLAT", 100)
	noErr(t, err)
	if !slices.Contains(sm.Edges, StarEdge{From: string(st.ID), To: string(page.ID), Kind: StarRelation, Type: "epic"}) {
		t.Fatalf("starmap edges %+v", sm.Edges)
	}
	imp, err := anna.Impact(string(page.ID), ImpactAffected, 1, 100)
	noErr(t, err)
	if !slices.ContainsFunc(imp.Nodes, func(n ImpactNode) bool { return n.ID == string(st.ID) }) {
		t.Fatalf("impact %+v", imp.Nodes)
	}
	// Changing the value moves the relation; clearing it frees the target.
	noErr(t, anna.SetProperty(st.ID, "epic", other.ID))
	if in, _ := anna.Relations(page.ID, Incoming); len(in) != 0 {
		t.Fatalf("old target still referenced: %+v", in)
	}
	noErr(t, anna.Trash(page.ID))
	if err := anna.SetProperty(st.ID, "anything", page.ID); !errors.Is(err, ErrValidation) {
		t.Fatalf("referred to a trashed item: %v", err)
	}
	noErr(t, anna.SetProperty(st.ID, "epic", nil))
	if in, _ := anna.Relations(other.ID, Incoming); len(in) != 1 || in[0].Type != RelMentions {
		t.Fatalf("after clearing, other is related by %+v", in)
	}
	// A purged referrer frees its target.
	keep := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Kept"}))
	noErr(t, anna.SetProperty(st.ID, "anything", keep.ID))
	noErr(t, anna.Trash(st.ID))
	noErr(t, s.As("pat").Purge(st.ID))
	noErr(t, anna.Trash(keep.ID))
}

func TestAnUnreadableTargetIsNotRevealed(t *testing.T) {
	s := storyFixture(t)
	anna, ben := s.As("anna"), s.As("ben")
	secret := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Secret"}))
	noErr(t, anna.Restrict(secret.ID, Restriction{Read: []string{"anna"}}))
	st := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "story", Title: "Login", Properties: Props{"anything": secret.ID, "rating": 3}}))

	got := mustItem(t)(ben.Get(st.ID))
	if _, ok := got.Properties["anything"]; ok || got.Properties["rating"] != 3 {
		t.Fatalf("ben reads %#v", got.Properties)
	}
	if found := ben.Find(Query{Type: "story"}); len(found) != 1 || found[0].Properties["anything"] != nil {
		t.Fatalf("ben finds %#v", found)
	}
	history, err := ben.History(st.ID)
	noErr(t, err)
	for _, e := range history {
		if e.Props["anything"] != nil {
			t.Fatalf("the history names the target: %+v", e)
		}
	}
	if out, _ := ben.Relations(st.ID, Outgoing); len(out) != 0 {
		t.Fatalf("ben sees relations %+v", out)
	}
	// One may not refer to an item one does not read: it does not exist.
	other := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "story", Title: "Other"}))
	noErr(t, s.GrantRole("root", "PLAT", "ben", Editor))
	if err := ben.SetProperty(other.ID, "anything", secret.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ben refers to the secret: %v", err)
	}
	if _, err := ben.Create(NewItem{Space: "PLAT", Type: "story", Title: "x", Properties: Props{"anything": secret.ID}}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ben creates with the secret: %v", err)
	}
	// What the item holds already may stay while another field changes.
	noErr(t, ben.SetProperty(st.ID, "rating", 5))
}

func TestQueriesOfTheNewFieldKinds(t *testing.T) {
	s := storyFixture(t)
	anna := s.As("anna")
	page := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Epic"}))
	a := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "story", Title: "A", Properties: Props{"platforms": []string{"web", "ios"}, "rating": 2, "epic": page.ID}}))
	b := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "story", Title: "B", Properties: Props{"platforms": []string{"android"}, "rating": 5}}))
	keys := func(q string) []string {
		t.Helper()
		items, err := anna.Select(Query{}, q, QueryEnv{})
		noErr(t, err)
		var out []string
		for _, it := range items {
			out = append(out, it.Key)
		}
		return out
	}
	for q, want := range map[string][]string{
		"platforms = web":                   {a.Key},
		"platforms = WEB":                   {a.Key},
		"platforms IN (android, ios)":       {a.Key, b.Key},
		"platforms != web AND type=story":   {b.Key},
		"rating >= 3":                       {b.Key},
		"rating = 2":                        {a.Key},
		"type = story ORDER BY rating DESC": {b.Key, a.Key},
		"epic = " + page.Key:                {a.Key},
		"epic = " + string(page.ID):         {a.Key},
		"epic IS EMPTY AND type = story":    {b.Key},
		"type = story ORDER BY epic":        {a.Key, b.Key},
	} {
		if got := keys(q); !slices.Equal(got, want) {
			t.Errorf("%s: got %v, want %v", q, got, want)
		}
	}
	for _, q := range []string{"platforms = palm", "rating = 7", "rating ~ x", "platforms > web"} {
		if _, err := anna.Select(Query{}, q, QueryEnv{}); !errors.Is(err, ErrValidation) {
			t.Errorf("%s: err = %v", q, err)
		}
	}
}
