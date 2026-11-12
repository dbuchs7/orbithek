package item

import (
	"errors"
	"slices"
	"testing"
)

// labelled makes an item of the type task in space carrying labels.
func labelled(t *testing.T, x *Session, space, title string, labels ...string) Item {
	t.Helper()
	it := mustItem(t)(x.Create(NewItem{Space: space, Type: "task", Title: title, Properties: Props{"priority": "low"}}))
	noErr(t, x.SetLabels(it.ID, labels))
	return it
}

// orderedTitles is the items of a topic as x reads them: those the order
// set by hand names, in its order, then the others as listed.
func orderedTitles(t *testing.T, x *Session, topic ID, label string) []string {
	t.Helper()
	order, err := x.ItemOrder(topic)
	noErr(t, err)
	rank := map[ID]int{}
	for i, id := range order {
		rank[id] = i
	}
	items := x.Find(Query{Label: label})
	slices.SortStableFunc(items, func(a, b Item) int {
		ra, oka := rank[a.ID]
		rb, okb := rank[b.ID]
		switch {
		case oka && okb:
			return ra - rb
		case oka:
			return -1
		case okb:
			return 1
		}
		return 0
	})
	var out []string
	for _, it := range items {
		out = append(out, it.Title)
	}
	return out
}

func TestTheTopicTemplateIsReadyMade(t *testing.T) {
	var found *TypeDef
	for _, td := range TypeTemplates() {
		if td.Name == TopicType {
			found = &td
		}
	}
	if found == nil {
		t.Fatal("no topic among the templates")
	}
	if err := found.validate(); err != nil {
		t.Fatal(err)
	}
	if !isTopicType(*found) {
		t.Fatal("the template is no topic")
	}
	f, _ := found.field(TopicLabelField)
	if f.Kind != KindText || !f.Unique || !f.Required || !found.Body || found.Workflow != nil {
		t.Errorf("template %+v", found)
	}
	if isTopicType(TypeDef{Name: "x", Body: true, Fields: []FieldDef{{Name: "label", Kind: KindNumber}}}) ||
		isTopicType(TypeDef{Name: "x", Fields: []FieldDef{{Name: "label", Kind: KindText}}}) {
		t.Error("a type that is no topic counts as one")
	}
	// A request's task is never made of the topic type.
	s := newTestStore(t)
	td, define, ok := s.requestType()
	if !ok || !define || td.Name != "task" {
		t.Errorf("request type %s %v %v", td.Name, define, ok)
	}
}

// Describing a label makes its page in a room: the type topic is defined
// on first use, the page holds the label, and the label's page is found
// by it in every readable room, the oldest first.
func TestALabelIsDescribedByATopicPage(t *testing.T) {
	s := permFixture(t)
	anna, ben, dora, carl := s.As("anna"), s.As("ben"), s.As("dora"), s.As("carl")
	if _, ok := anna.TopicOf("runbook"); ok {
		t.Fatal("a page before one was made")
	}
	if _, err := ben.DescribeLabel("PLAT", "runbook", ""); !errors.Is(err, ErrForbidden) {
		t.Fatalf("a reader described a label: %v", err)
	}
	if _, err := anna.DescribeLabel("PLAT", "no label!", ""); !errors.Is(err, ErrValidation) {
		t.Fatalf("an invalid label: %v", err)
	}
	page := mustItem(t)(anna.DescribeLabel("PLAT", " Runbook ", ""))
	if page.Type != TopicType || page.Title != "runbook" || page.Properties[TopicLabelField] != "runbook" {
		t.Fatalf("page %+v", page)
	}
	got, ok := ben.TopicOf("runbook")
	if !ok || got.ID != page.ID {
		t.Fatalf("ben finds %v %v", got.ID, ok)
	}
	if _, ok := carl.TopicOf("runbook"); ok {
		t.Error("one who reads no room found the page")
	}
	// The field is unique in a room.
	if _, err := anna.DescribeLabel("PLAT", "runbook", "Again"); !errors.Is(err, ErrValidation) {
		t.Errorf("a second page in one room: %v", err)
	}
	// Another room's page is newer: the older one is found first, and a
	// reader of only the other room finds that one.
	ops := mustItem(t)(dora.DescribeLabel("OPS", "runbook", "Runbooks of operations"))
	if got, _ := anna.TopicOf("runbook"); got.ID != page.ID {
		t.Errorf("anna finds %s", got.Key)
	}
	if got, _ := dora.TopicOf("runbook"); got.ID != ops.ID {
		t.Errorf("dora finds %s", got.Key)
	}
	// A restricted page is not found by those it excludes.
	noErr(t, s.As("pat").Restrict(page.ID, Restriction{Read: []string{"pat"}}))
	if got, ok := ben.TopicOf("runbook"); ok {
		t.Errorf("ben finds the restricted page %s", got.Key)
	}
	// A type of that name that is no topic is not taken over.
	s2 := setup(t)
	noErr(t, s2.GrantRole("admin", "PLAT", "anna", Editor))
	noErr(t, s2.DefineType("admin", TypeDef{Name: TopicType, Fields: []FieldDef{{Name: "x", Kind: KindText}}}))
	if _, err := s2.As("anna").DescribeLabel("PLAT", "runbook", ""); !errors.Is(err, ErrValidation) {
		t.Errorf("used a foreign type: %v", err)
	}
	s3 := setup(t)
	noErr(t, s3.GrantRole("admin", "PLAT", "anna", Editor))
	s3.SetNothingNew(func() error { return errors.New("the disk is full") })
	if _, err := s3.As("anna").DescribeLabel("PLAT", "runbook", ""); err == nil {
		t.Error("a topic was made while nothing new may be")
	}
	if _, ok := s3.latest(TopicType); ok {
		t.Error("the type was defined for a refused page")
	}
}

// The items of a topic are ordered by hand: moved before another or to
// the end; new items come after the ordered ones; items a reader does not
// read keep their places, unseen.
func TestTheItemsOfATopicAreOrderedByHand(t *testing.T) {
	s := permFixture(t)
	anna, ben, pat := s.As("anna"), s.As("ben"), s.As("pat")
	topic := mustItem(t)(anna.DescribeLabel("PLAT", "onboarding", "Onboarding"))
	a := labelled(t, anna, "PLAT", "A", "onboarding")
	b := labelled(t, anna, "PLAT", "B", "onboarding")
	c := labelled(t, anna, "PLAT", "C", "onboarding", "other")
	labelled(t, anna, "PLAT", "Elsewhere", "other")
	if got := orderedTitles(t, ben, topic.ID, "onboarding"); !slices.Equal(got, []string{"A", "B", "C"}) {
		t.Fatalf("as listed %v", got)
	}
	if err := ben.PlaceItems(topic.ID, []ID{c.ID}, a.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("a reader ordered: %v", err)
	}
	noErr(t, anna.PlaceItems(topic.ID, []ID{c.ID}, a.ID))
	if got := orderedTitles(t, ben, topic.ID, "onboarding"); !slices.Equal(got, []string{"C", "A", "B"}) {
		t.Fatalf("after moving C first %v", got)
	}
	noErr(t, anna.PlaceItems(topic.ID, []ID{a.ID}, ""))
	if got := orderedTitles(t, ben, topic.ID, "onboarding"); !slices.Equal(got, []string{"C", "B", "A"}) {
		t.Fatalf("after moving A to the end %v", got)
	}
	n := len(s.Log())
	noErr(t, anna.PlaceItems(topic.ID, []ID{a.ID}, ""))
	if len(s.Log()) != n {
		t.Error("an unchanged order was recorded")
	}
	// A new item comes after the ordered ones.
	d := labelled(t, anna, "PLAT", "D", "onboarding")
	if got := orderedTitles(t, ben, topic.ID, "onboarding"); !slices.Equal(got, []string{"C", "B", "A", "D"}) {
		t.Fatalf("with a new item %v", got)
	}
	// An item only pat reads: anna's moves keep its place.
	secret := labelled(t, pat, "PLAT", "Secret", "onboarding")
	noErr(t, pat.Restrict(secret.ID, Restriction{Read: []string{"pat"}}))
	noErr(t, pat.PlaceItems(topic.ID, []ID{secret.ID}, b.ID))
	if got := orderedTitles(t, pat, topic.ID, "onboarding"); !slices.Equal(got, []string{"C", "Secret", "B", "A", "D"}) {
		t.Fatalf("pat reads %v", got)
	}
	if err := anna.PlaceItems(topic.ID, []ID{secret.ID}, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("anna placed an item she does not read: %v", err)
	}
	order, err := anna.ItemOrder(topic.ID)
	noErr(t, err)
	if slices.Contains(order, secret.ID) {
		t.Fatal("anna was told of the secret item")
	}
	noErr(t, anna.PlaceItems(topic.ID, []ID{d.ID}, c.ID))
	if got := orderedTitles(t, pat, topic.ID, "onboarding"); !slices.Equal(got, []string{"D", "C", "Secret", "B", "A"}) {
		t.Fatalf("pat after anna's move %v", got)
	}
	// An item that no longer carries the label is forgotten with the next
	// change; a trashed one too.
	noErr(t, anna.SetLabels(c.ID, []string{"other"}))
	noErr(t, anna.Trash(b.ID))
	noErr(t, anna.PlaceItems(topic.ID, []ID{a.ID}, d.ID))
	if got := mustItem(t)(pat.Get(topic.ID)).ItemOrder; !slices.Equal(got, []ID{a.ID, d.ID, secret.ID}) {
		t.Errorf("order kept %v", got)
	}
	re, err := Replay(s.Log())
	noErr(t, err)
	if got := orderedTitles(t, re.As("pat"), topic.ID, "onboarding"); !slices.Equal(got, []string{"A", "D", "Secret"}) {
		t.Errorf("replayed %v", got)
	}
	noErr(t, anna.ClearItemOrder(topic.ID))
	if got := orderedTitles(t, ben, topic.ID, "onboarding"); !slices.Equal(got, []string{"A", "D"}) {
		t.Errorf("cleared %v", got)
	}
	n = len(s.Log())
	noErr(t, anna.ClearItemOrder(topic.ID))
	if len(s.Log()) != n {
		t.Error("clearing no order was recorded")
	}
}

func TestPlacingItemsNamesOnlyItemsOfTheTopic(t *testing.T) {
	s := permFixture(t)
	anna := s.As("anna")
	topic := mustItem(t)(anna.DescribeLabel("PLAT", "onboarding", ""))
	a := labelled(t, anna, "PLAT", "A", "onboarding")
	b := labelled(t, anna, "PLAT", "B", "onboarding")
	other := labelled(t, anna, "PLAT", "Other", "other")
	page := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Runbook"}))
	cases := map[string]struct {
		id     ID
		moved  []ID
		before ID
		want   error
	}{
		"nothing moved":     {topic.ID, nil, "", ErrValidation},
		"without the label": {topic.ID, []ID{other.ID}, "", ErrNotFound},
		"before another":    {topic.ID, []ID{a.ID}, other.ID, ErrNotFound},
		"named twice":       {topic.ID, []ID{a.ID, a.ID}, "", ErrValidation},
		"before itself":     {topic.ID, []ID{a.ID}, a.ID, ErrValidation},
		"the topic itself":  {topic.ID, []ID{topic.ID}, "", ErrNotFound},
		"too many":          {topic.ID, []ID{a.ID, b.ID, other.ID}, "", ErrValidation},
		"no topic":          {page.ID, []ID{a.ID}, "", ErrValidation},
	}
	for name, c := range cases {
		if err := anna.PlaceItems(c.id, c.moved, c.before); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", name, err, c.want)
		}
	}
	if err := anna.ClearItemOrder(page.ID); !errors.Is(err, ErrValidation) {
		t.Errorf("cleared the order of a page: %v", err)
	}
	if _, err := anna.ItemOrder(page.ID); !errors.Is(err, ErrValidation) {
		t.Errorf("read the order of a page: %v", err)
	}
	if err := s.As("carl").PlaceItems(topic.ID, []ID{a.ID}, ""); !errors.Is(err, ErrNotFound) {
		t.Errorf("one who does not read the topic: %v", err)
	}
	// A topic whose label is not a label orders nothing.
	noErr(t, anna.SetProperty(topic.ID, TopicLabelField, "not a label!"))
	if err := anna.PlaceItems(topic.ID, []ID{a.ID}, ""); !errors.Is(err, ErrValidation) {
		t.Errorf("a topic without a label: %v", err)
	}
	noErr(t, anna.Trash(topic.ID))
	if err := anna.ClearItemOrder(topic.ID); !errors.Is(err, ErrValidation) {
		t.Errorf("cleared a trashed topic: %v", err)
	}
}

// The topic page itself carries its label: it is no item of its own
// topic.
func TestATopicIsNotOneOfItsOwnItems(t *testing.T) {
	s := permFixture(t)
	anna := s.As("anna")
	topic := mustItem(t)(anna.DescribeLabel("PLAT", "onboarding", ""))
	noErr(t, anna.SetLabels(topic.ID, []string{"onboarding"}))
	a := labelled(t, anna, "PLAT", "A", "onboarding")
	noErr(t, anna.PlaceItems(topic.ID, []ID{a.ID}, ""))
	if got := mustItem(t)(anna.Get(topic.ID)).ItemOrder; !slices.Equal(got, []ID{a.ID}) {
		t.Errorf("order %v", got)
	}
}

func TestLastChangesTellWhenItemsChanged(t *testing.T) {
	s := permFixture(t)
	anna, carl := s.As("anna"), s.As("carl")
	a := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "A"}))
	b := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "B"}))
	noErr(t, anna.SetTitle(a.ID, "A2"))
	got := anna.LastChanges([]ID{a.ID, b.ID, "nothing"})
	if len(got) != 2 || !got[a.ID].After(got[b.ID]) {
		t.Errorf("changes %v", got)
	}
	if len(carl.LastChanges([]ID{a.ID})) != 0 {
		t.Error("one who does not read it was told")
	}
}
