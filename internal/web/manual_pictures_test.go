package web

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/dbuchs7/orbithek/internal/manualcheck"
)

// manualPicture finds a picture of the manual: a line of its own naming a
// file of its img folder (assets/manual.js).
var manualPicture = regexp.MustCompile(`(?m)^!\[[^\]\n]*\]\((img/[^ )]+)`)

// TestManualPicturesAreShown: every picture embedded for the manual is shown
// by a manual, so none is shipped in the binary for nothing
// (ADR-0253); that every picture a
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
		if strings.HasSuffix(path, ".txt") {
			return nil // sources.txt, the list of what each picture shows
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

// TestManualPictureSources keeps manual/img/sources.txt, the list of what
// each picture of the manual shows, true to the tree
// (ADR-0254): every picture
// has a line, every line names pictures that exist in every language, and
// every source it names exists, so the manual check cannot miss a picture
// by a name that has gone.
func TestManualPictureSources(t *testing.T) {
	text, err := fs.ReadFile(FS(), "assets/manual/img/sources.txt")
	if err != nil {
		t.Fatal(err)
	}
	pictures, err := manualcheck.ParseSources(string(text))
	if err != nil {
		t.Fatalf("sources.txt: %v", err)
	}
	catalogue, err := fs.ReadFile(FS(), "assets/locales/en.js")
	if err != nil {
		t.Fatal(err)
	}
	keys, err := manualcheck.ParseCatalogue(string(catalogue))
	if err != nil {
		t.Fatal(err)
	}
	const root = "internal/web/static/"
	listed := map[string]bool{}
	for _, p := range pictures {
		for _, f := range p.Files(Languages) {
			rel := strings.TrimPrefix(f, root)
			listed[rel] = true
			if _, err := fs.Stat(FS(), rel); err != nil {
				t.Errorf("sources.txt lists %s, but %s does not exist", p.Name, rel)
			}
		}
		for _, s := range p.Sources {
			if prefix, ok := strings.CutPrefix(s, "key:"); ok {
				found := false
				for k := range keys {
					found = found || strings.HasPrefix(k, prefix)
				}
				if !found {
					t.Errorf("%s: no catalogue key starts with %q", p.Name, prefix)
				}
				continue
			}
			data, err := fs.ReadFile(FS(), strings.TrimPrefix(manualcheck.SourceFile(s), root))
			if err != nil {
				t.Errorf("%s: %s: %v", p.Name, s, err)
				continue
			}
			if _, ok := manualcheck.Excerpt(string(data), s); !ok {
				t.Errorf("%s: %s names no top-level function or constant of its file", p.Name, s)
			}
		}
	}
	err = fs.WalkDir(FS(), "assets/manual/img", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.HasSuffix(path, ".txt") {
			return err
		}
		if !listed[path] {
			t.Errorf("%s has no line in sources.txt saying what it shows", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
