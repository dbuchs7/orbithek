package item

import (
	"errors"
	"slices"
	"testing"
)

// TestTheWebLinkTemplate: the ready-made type of web links (ADR-0206) has
// a body for one's notes and the fields a page says about itself, the
// address of kind url; it holds no access requests, so their tasks keep
// the task type.
func TestTheWebLinkTemplate(t *testing.T) {
	td, ok := TemplateType(WebLinkType)
	if !ok {
		t.Fatal("no ready-made web link type")
	}
	noErr(t, td.validate())
	if !td.Body || td.Workflow != nil {
		t.Errorf("a web link has a body and no workflow: %+v", td)
	}
	kinds := map[string]FieldKind{}
	for _, f := range td.Fields {
		kinds[f.Name] = f.Kind
	}
	want := map[string]FieldKind{"url": KindURL, "site": KindText, "description": KindText, "author": KindText, "published": KindDate}
	for name, kind := range want {
		if kinds[name] != kind {
			t.Errorf("field %s is %q, want %q", name, kinds[name], kind)
		}
	}
	if holdsRequests(td) {
		t.Error("a web link must not hold access requests")
	}
	if _, ok := TemplateType("nothing"); ok {
		t.Error("a name without a template has none")
	}
	var names []string
	for _, td := range TypeTemplates() {
		names = append(names, td.Name)
	}
	if want := []string{"page", "task", "decision", "topic", WebLinkType, "daily-note"}; !slices.Equal(names, want) {
		t.Errorf("the templates keep their order: %v, want %v", names, want)
	}
}

// TestCreatingFromATemplateDefinesItsType: one who may create in a room
// saves a web link before an administrator has added the type; the type is
// defined in the same batch as the item, and the next item uses it.
func TestCreatingFromATemplateDefinesItsType(t *testing.T) {
	s := newTestStore(t)
	noErr(t, s.CreateSpace("admin", "PLAT", "Platform"))
	noErr(t, s.GrantRole("root", "PLAT", "anna", Editor))
	noErr(t, s.GrantRole("root", "PLAT", "ben", Viewer))
	n := NewItem{Space: "PLAT", Type: WebLinkType, Title: "Example", Properties: Props{"url": "https://example.org/a"},
		Body: &Doc{Blocks: []Block{{Kind: Quote, Text: "A quote."}}}}

	// One who may not create in the room defines nothing either.
	if _, err := s.As("ben").CreateFromTemplate(n); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ben creates: %v", err)
	}
	if _, ok := s.latest(WebLinkType); ok {
		t.Fatal("a refused creation defined the type")
	}
	// A refused item leaves no type behind: the batch is one.
	bad := n
	bad.Properties = Props{"url": "ftp://example.org"}
	if _, err := s.As("anna").CreateFromTemplate(bad); !errors.Is(err, ErrValidation) {
		t.Fatalf("an invalid address: %v", err)
	}
	if _, ok := s.latest(WebLinkType); ok {
		t.Fatal("a refused item defined the type")
	}

	before := len(s.log)
	it := mustItem(t)(s.As("anna").CreateFromTemplate(n))
	if it.Type != WebLinkType || it.TypeVersion != 1 || it.Properties["url"] != "https://example.org/a" || len(it.Body.Blocks) != 1 {
		t.Fatalf("item %+v", it)
	}
	batch := s.log[before:]
	if batch[0].Kind != TypeDefined || batch[1].Kind != ItemCreated || batch[0].Batch == 0 || batch[0].Batch != batch[1].Batch {
		t.Fatalf("type and item are not one batch: %+v", batch)
	}
	if !s.As("anna").Watching(it.ID) {
		t.Error("the creator watches the new item")
	}
	// The second uses the type now defined, and defines nothing.
	before = len(s.log)
	n.Properties = Props{"url": "https://example.org/b"}
	mustItem(t)(s.As("anna").CreateFromTemplate(n))
	for _, e := range s.log[before:] {
		if e.Kind == TypeDefined {
			t.Fatal("the type was defined again")
		}
	}
	// A type of another name without a template is not found.
	if _, err := s.As("anna").CreateFromTemplate(NewItem{Space: "PLAT", Type: "nothing", Title: "x"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an unknown type: %v", err)
	}
}

// TestCreatingFromATemplateKeepsAnAdministratorsType: a type of the
// template's name that an administrator defined is used as it is.
func TestCreatingFromATemplateKeepsAnAdministratorsType(t *testing.T) {
	s := newTestStore(t)
	noErr(t, s.CreateSpace("admin", "PLAT", "Platform"))
	noErr(t, s.GrantRole("root", "PLAT", "anna", Editor))
	noErr(t, s.DefineType("admin", TypeDef{Name: WebLinkType, Body: false, Fields: []FieldDef{{Name: "url", Kind: KindURL}}}))
	it := mustItem(t)(s.As("anna").CreateFromTemplate(NewItem{Space: "PLAT", Type: WebLinkType, Title: "x", Properties: Props{"url": "https://example.org"}}))
	if it.TypeVersion != 1 {
		t.Fatalf("type version %d", it.TypeVersion)
	}
	if td, _ := s.latest(WebLinkType); td.Body || len(td.Fields) != 1 {
		t.Fatalf("the administrator's type changed: %+v", td)
	}
}
