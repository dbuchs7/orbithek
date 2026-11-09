package item

import (
	"errors"
	"testing"
)

func TestATypeNamesAWordTemplateAndAStartText(t *testing.T) {
	s := setup(t)
	ok := TypeDef{Name: "report", Body: true, Template: "bericht", Start: &Doc{Blocks: []Block{
		{Kind: Heading, Level: 1, Text: "Ausgangslage"},
		{Kind: Paragraph, Text: "Siehe Quelle", Marks: []Mark{{Type: MarkLink, Start: 6, End: 12, Href: "https://example.org"}}},
		{Kind: Table, Text: "A\tB\n1\t2"}, {Kind: TOC}, {Kind: Children}, {Kind: Divider},
	}}}
	noErr(t, s.DefineType("admin", ok))
	if got := s.Type("report"); got.Template != "bericht" || len(got.Start.Blocks) != 6 {
		t.Fatalf("type = %+v", got)
	}
	bad := map[string]TypeDef{
		"template ID":        {Name: "x", Body: true, Template: "Bericht 1"},
		"start without body": {Name: "x", Start: &Doc{}},
		"a picture":          {Name: "x", Body: true, Start: &Doc{Blocks: []Block{{Kind: Image, Ref: "att1"}}}},
		"an embed":           {Name: "x", Body: true, Start: &Doc{Blocks: []Block{{Kind: Embed, Ref: "id001"}}}},
		"a process":          {Name: "x", Body: true, Start: &Doc{Blocks: []Block{{Kind: Process, Engine: "atlas", Process: "p"}}}},
		"a mention":          {Name: "x", Body: true, Start: &Doc{Blocks: []Block{{Kind: Paragraph, Text: "@a", Mentions: []ID{"id001"}}}}},
		"a protection":       {Name: "x", Body: true, Start: &Doc{Blocks: []Block{{Kind: Paragraph, Text: "s", Secret: "sec1"}}}},
		"a broken block":     {Name: "x", Body: true, Start: &Doc{Blocks: []Block{{Kind: TOC, Text: "words"}}}},
	}
	for name, td := range bad {
		if err := s.DefineType("admin", td); !errors.Is(err, ErrValidation) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if ValidTemplateID("") || !ValidTemplateID("a-1") {
		t.Error("ValidTemplateID")
	}
}

func TestAPageRemembersItsTemplateVersion(t *testing.T) {
	s := setup(t)
	noErr(t, s.DefineType("admin", TypeDef{Name: "report", Body: true, Template: "bericht"}))
	// Without the site's versions, a page is made without one.
	plain := mustItem(t)(s.Create("anna", NewItem{Space: "PLAT", Type: "report", Title: "Ohne"}))
	if plain.Template != nil {
		t.Errorf("a page without known versions has %+v", plain.Template)
	}
	newest := 0
	s.SetTemplateVersions(func(id string) int {
		if id == "bericht" {
			return newest
		}
		return 7
	})
	if it := mustItem(t)(s.Create("anna", NewItem{Space: "PLAT", Type: "report", Title: "Kein File"})); it.Template != nil {
		t.Errorf("a template without a file gave %+v", it.Template)
	}
	newest = 2
	it := mustItem(t)(s.Create("anna", NewItem{Space: "PLAT", Type: "report", Title: "Quartal"}))
	if it.Template == nil || *it.Template != (TemplateRef{"bericht", 2}) {
		t.Fatalf("template = %+v", it.Template)
	}
	// A copy keeps its original's version; another template's is ignored.
	kept := mustItem(t)(s.Create("anna", NewItem{Space: "PLAT", Type: "report", Title: "Kopie", Template: &TemplateRef{"bericht", 1}}))
	other := mustItem(t)(s.Create("anna", NewItem{Space: "PLAT", Type: "report", Title: "Fremd", Template: &TemplateRef{"memo", 1}}))
	if *kept.Template != (TemplateRef{"bericht", 1}) || *other.Template != (TemplateRef{"bericht", 2}) {
		t.Errorf("kept %+v, other %+v", kept.Template, other.Template)
	}
	// A page of a type without a template has none, whatever is asked.
	page := mustItem(t)(s.Create("anna", NewItem{Space: "PLAT", Type: "page", Title: "Seite", Template: &TemplateRef{"bericht", 1}}))
	if page.Template != nil {
		t.Errorf("a page of a type without a template has %+v", page.Template)
	}
	// The template moves on; the page stays until moved.
	newest = 3
	if got, _ := s.Get(it.ID); *got.Template != (TemplateRef{"bericht", 2}) {
		t.Errorf("the page moved by itself: %+v", got.Template)
	}
	noErr(t, s.GrantRole("admin", "PLAT", "anna", Editor))
	noErr(t, s.GrantRole("admin", "PLAT", "vera", Viewer))
	if err := s.As("vera").SetTemplate(it.ID, TemplateRef{"bericht", 3}); err == nil {
		t.Error("a viewer moved the page")
	}
	noErr(t, s.As("anna").SetTemplate(it.ID, TemplateRef{"bericht", 3}))
	got, _ := s.Get(it.ID)
	if *got.Template != (TemplateRef{"bericht", 3}) || got.Version != it.Version+1 {
		t.Errorf("after the move: %+v, version %d", got.Template, got.Version)
	}
	before := len(s.Log())
	noErr(t, s.SetTemplate("anna", it.ID, TemplateRef{"bericht", 3}))
	if len(s.Log()) != before {
		t.Error("moving to the same version recorded an event")
	}
	for name, ref := range map[string]TemplateRef{"another template": {"memo", 1}, "no version": {"bericht", 0}} {
		if err := s.SetTemplate("anna", it.ID, ref); !errors.Is(err, ErrValidation) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if err := s.SetTemplate("anna", page.ID, TemplateRef{"bericht", 1}); !errors.Is(err, ErrValidation) {
		t.Errorf("a page of a type without a template: err = %v", err)
	}
	if err := s.SetTemplate("anna", "missing", TemplateRef{"bericht", 1}); err == nil {
		t.Error("a missing item was moved")
	}
	// The returned item is a copy.
	got.Template.Version = 99
	if again, _ := s.Get(it.ID); again.Template.Version != 3 {
		t.Error("changing a returned item changed the store")
	}
}
