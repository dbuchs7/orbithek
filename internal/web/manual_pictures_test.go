package web

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// manualPicture finds a picture of the manual: a line of its own naming a
// file of its img folder (assets/manual.js).
var manualPicture = regexp.MustCompile(`(?m)^!\[[^\]\n]*\]\((img/[^ )]+)`)

// TestManualPicturesAreShown: every picture embedded for the manual is shown
// by a manual, so none is shipped in the binary for nothing
// (ADR-draft-pictures-and-help-links-in-the-manual); that every picture a
// manual shows exists is internal/web/jstest/manual.test.mjs's.
func TestManualPicturesAreShown(t *testing.T) {
	shown := map[string]bool{}
	for _, lang := range Languages {
		data, err := fs.ReadFile(FS(), "assets/manual/"+lang+".md")
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range manualPicture.FindAllStringSubmatch(string(data), -1) {
			shown[m[1]] = true
		}
	}
	if len(shown) == 0 {
		t.Fatal("no manual shows a picture")
	}
	n := 0
	err := fs.WalkDir(FS(), "assets/manual/img", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		n++
		if rel := strings.TrimPrefix(path, "assets/manual/"); !shown[rel] {
			t.Errorf("%s is embedded but no manual shows it: show it or remove it", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("no picture is embedded for the manual")
	}
}
