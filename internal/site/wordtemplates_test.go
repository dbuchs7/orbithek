package site

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestWordTemplatesKeepEveryVersionAndAreReplayed(t *testing.T) {
	var records [][]byte
	j := func(r []byte) error { records = append(records, r); return nil }
	s, _ := New(nil, j)
	if v, err := s.AddWordTemplate("admin", "bericht", " Bericht ", `C:\Vorlagen\Bericht.dotx`, []byte("one")); err != nil || v != 1 {
		t.Fatalf("first version: %d, %v", v, err)
	}
	if v, err := s.AddWordTemplate("ada", "bericht", "Bericht 2026", "neu\n.dotx", []byte("two")); err != nil || v != 2 {
		t.Fatalf("second version: %d, %v", v, err)
	}
	if err := s.RenameWordTemplate("admin", "bericht", "Jahresbericht"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddWordTemplate("admin", "aktennotiz", "Aktennotiz", "a.dotx", []byte("memo")); err != nil {
		t.Fatal(err)
	}
	again, err := New(records, j)
	if err != nil {
		t.Fatal(err)
	}
	list := again.WordTemplates()
	if len(list) != 2 || list[0].ID != "aktennotiz" || list[1].Name != "Jahresbericht" {
		t.Fatalf("templates = %+v", list)
	}
	b := list[1]
	if len(b.Versions) != 2 || b.Versions[0].File != "Bericht.dotx" || b.Versions[1].File != "neu.dotx" || b.Versions[1].Actor != "ada" || b.Versions[1].Size != 3 || b.Versions[0].Tag == b.Versions[1].Tag {
		t.Errorf("versions = %+v", b.Versions)
	}
	if again.NewestWordTemplate("bericht") != 2 || again.NewestWordTemplate("none") != 0 {
		t.Error("newest version is wrong")
	}
	for v, want := range map[int]string{0: "two", 1: "one", 2: "two"} {
		meta, data, ok := again.WordTemplateFile("bericht", v)
		if !ok || string(data) != want || meta.Version == 0 {
			t.Errorf("version %d = %q, %v", v, data, ok)
		}
	}
	for _, v := range []int{-1, 3} {
		if _, _, ok := again.WordTemplateFile("bericht", v); ok {
			t.Errorf("version %d exists", v)
		}
	}
	if one, ok := again.WordTemplate("bericht"); !ok || one.Name != "Jahresbericht" {
		t.Error("WordTemplate does not find the template")
	}
	if err := again.RemoveWordTemplate("admin", "bericht"); err != nil {
		t.Fatal(err)
	}
	if _, ok := again.WordTemplate("bericht"); ok {
		t.Error("a removed template is still there")
	}
	if _, _, ok := again.WordTemplateFile("bericht", 0); ok {
		t.Error("a removed template's file is still there")
	}
}

func TestWordTemplatesRefuseWhatIsNotOne(t *testing.T) {
	s, _ := New(nil, func([]byte) error { return nil })
	for name, err := range map[string]error{
		"bad id":  func() error { _, err := s.AddWordTemplate("a", "Bericht", "B", "b", []byte("x")); return err }(),
		"no name": func() error { _, err := s.AddWordTemplate("a", "b", " ", "b", []byte("x")); return err }(),
		"long name": func() error {
			_, err := s.AddWordTemplate("a", "b", strings.Repeat("n", 81), "b", []byte("x"))
			return err
		}(),
		"empty file": func() error { _, err := s.AddWordTemplate("a", "b", "B", "b", nil); return err }(),
		"rename bad": s.RenameWordTemplate("a", "b", "x\ny"),
	} {
		if !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if err := s.RenameWordTemplate("a", "none", "N"); !errors.Is(err, ErrNotFound) {
		t.Errorf("rename a missing one: %v", err)
	}
	if err := s.RemoveWordTemplate("a", "none"); !errors.Is(err, ErrNotFound) {
		t.Errorf("remove a missing one: %v", err)
	}
	for i := range maxWordTemplates {
		if _, err := s.AddWordTemplate("a", fmt.Sprintf("t%d", i), "T", "t", []byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.AddWordTemplate("a", "more", "T", "t", []byte("x")); !errors.Is(err, ErrInvalid) {
		t.Errorf("one more than allowed: %v", err)
	}
	if _, err := s.AddWordTemplate("a", "t0", "T", "", []byte("y")); err != nil {
		t.Errorf("a new version of a kept one is refused: %v", err)
	}
	if got, _ := s.WordTemplate("t0"); got.Versions[1].File != "template.dotx" {
		t.Errorf("a file without a name = %q", got.Versions[1].File)
	}
	if _, err := New([][]byte{[]byte(`{"v":1,"kind":"WordTemplateSet"}`)}, nil); err == nil {
		t.Error("an event without a template is accepted")
	}
	if _, err := New([][]byte{[]byte(`{"v":1,"kind":"WordTemplateSet","wordTemplate":{"id":"x","name":"X"}}`)}, nil); err == nil {
		t.Error("a name before a file is accepted")
	}
}
