package server

import (
	"testing"

	"github.com/dbuchs7/orbithek/internal/item"
)

// TestControlsAndExportsNameTheNewKinds: a Word template's controls and
// the page exports name a referenced item by its key and title, a file by
// its name, the values of a multi field joined and a rating of five
// (ADR-0200); a CSV row names the item by its key.
func TestControlsAndExportsNameTheNewKinds(t *testing.T) {
	s := item.NewStore()
	if err := s.CreateSpace("root", "PLAT", "Platform"); err != nil {
		t.Fatal(err)
	}
	td := item.TypeDef{Name: "card", Fields: []item.FieldDef{
		{Name: "About", Kind: item.KindItem}, {Name: "tags", Kind: item.KindMulti, Values: []string{"a", "b"}},
		{Name: "stars", Kind: item.KindRating}, {Name: "cover", Kind: item.KindFile},
	}}
	if err := s.DefineType("root", td); err != nil {
		t.Fatal(err)
	}
	target, err := s.Create("root", item.NewItem{Space: "PLAT", Type: "card", Title: "Target"})
	if err != nil {
		t.Fatal(err)
	}
	it := item.Item{Key: "PLAT-2", Properties: item.Props{"About": target.ID, "tags": []string{"a", "b"}, "stars": 3, "cover": "f1"},
		Attachments: []item.Attachment{{ID: "f1", Name: "cover.png"}}}
	got := controls(s, it, td, "Platform", nil)
	for k, want := range map[string]string{"about": "PLAT-1 Target", "tags": "a, b", "stars": "3/5", "cover": "cover.png"} {
		if got[k] != want {
			t.Errorf("%s = %q, want %q", k, got[k], want)
		}
	}
	if v := shownValue(s, it, item.KindItem, target.ID, false); v != "PLAT-1" {
		t.Errorf("a CSV cell names %v", v)
	}
	if v := shownValue(s, it, item.KindFile, "gone", false); v != nil {
		t.Errorf("a file that is not the item's reads %v", v)
	}
}
