package manualcheck

import (
	"os"
	"slices"
	"strings"
	"testing"
)

const catA = `// comment
export default {
  "$name": "English",
  "common.close": "Close",
  "album.present": { "one": "Present {n} picture", "other": "Present {n} pictures" }
};
`

const shellA = `
async function route() {
  switch (section) {
      case "album": return albumView();
      case "governance": return arg ? await governanceView(arg) : await governanceOverview();
      default: return await homeView();
  }
}
function albumView() {}
async function governanceView(key) {}
function helper() { switch (x) { case "a": return 1; } }
`

func TestParseCatalogue(t *testing.T) {
	c, err := ParseCatalogue(catA)
	if err != nil || c["common.close"] != "Close" {
		t.Fatalf("ParseCatalogue = %v, %v", c, err)
	}
	if c, err := ParseCatalogue(""); err != nil || len(c) != 0 {
		t.Errorf("an empty file is not an empty catalogue: %v, %v", c, err)
	}
	if _, err := ParseCatalogue("const x = 1;"); err == nil {
		t.Error("a file without a default export is accepted")
	}
	if _, err := ParseCatalogue("export default { 'a': 1 };"); err == nil {
		t.Error("a catalogue that is no strict JSON is accepted")
	}
}

func TestRoutesAndViews(t *testing.T) {
	if got := Routes(shellA); !slices.Equal(got, []string{"album", "governance"}) {
		t.Errorf("Routes = %q", got)
	}
	if got := Views(shellA); !slices.Equal(got, []string{"albumView", "governanceView"}) {
		t.Errorf("Views = %q", got)
	}
}

// TestTheShellsConvention keeps the patterns in step with app.js itself:
// if the router or its views are written another way, Check B would see
// no new route, and this test says so.
func TestTheShellsConvention(t *testing.T) {
	data, err := os.ReadFile("../web/static/assets/app.js")
	if err != nil {
		t.Fatal(err)
	}
	routes, views := Routes(string(data)), Views(string(data))
	for _, r := range []string{"album", "present", "manual", "admin", "item", "governance"} {
		if !slices.Contains(routes, r) {
			t.Errorf("the route %q of app.js is not found; routes found: %q", r, routes)
		}
	}
	for _, v := range []string{"albumView", "manualView", "itemView"} {
		if !slices.Contains(views, v) {
			t.Errorf("the view %q of app.js is not found", v)
		}
	}
}

func TestIsModuleAndManual(t *testing.T) {
	for p, want := range map[string]bool{
		Assets + "present.js":      true,
		Assets + "locales/fr.js":   false,
		Assets + "style.css":       false,
		Assets + "palettes/x.json": false,
		"internal/web/web.go":      false,
		"e2e/smoke.cjs":            false,
	} {
		if got := IsModule(p); got != want {
			t.Errorf("IsModule(%q) = %v", p, got)
		}
	}
	if !IsManual(Manuals+"de.md") || IsManual(Manuals+"img/a.md") || IsManual(Assets+"de.md") {
		t.Error("IsManual is wrong")
	}
}

func TestNotNeeded(t *testing.T) {
	for body, want := range map[string]string{
		"Text\nManual: not needed — fixes a typo in a label\nmore": "fixes a typo in a label",
		"manual: Not needed - only an error message  ":             "only an error message",
		"Manual: not needed -- tests only\r\n":                     "tests only",
		"Manual: not needed – en dash":                             "en dash",
		"Manual: not needed —   ":                                  "",
		"Manual: not needed":                                       "",
		"See: Manual: not needed — inline is no line of its own":   "",
		"": "",
	} {
		if got := NotNeeded(body); got != want {
			t.Errorf("NotNeeded(%q) = %q, want %q", body, got, want)
		}
	}
}

func TestEvaluate(t *testing.T) {
	manuals := []string{Manuals + "de.md", Manuals + "en.md"}
	catWithMore := strings.Replace(catA, `"$name": "English",`, `"$name": "English", "album.rename": "Rename",`, 1)
	catB := strings.Replace(catA, `"Close"`, `"Close it"`, 1)
	catB = strings.Replace(catB, `"$name": "English",`, `"$name": "English (UK)", "album.rename": "Rename",`, 1)
	shellB := strings.Replace(shellA, `      default:`, "      case \"present\": return await presentScreen(params);\n      default:", 1) +
		"\nasync function presentScreen(params) {}\n"

	cases := []struct {
		name    string
		c       Change
		visible bool
		ok      bool
		missing int
	}{
		{"nothing people see", Change{OldCatalogue: catA, NewCatalogue: catA, OldShell: shellA, NewShell: shellA,
			Changed: []string{"internal/item/album.go"}, Manuals: manuals}, false, true, 2},
		{"a removed value", Change{OldCatalogue: catWithMore, NewCatalogue: catA, Manuals: manuals}, false, true, 2},
		{"a changed value without the manuals", Change{OldCatalogue: catA, NewCatalogue: catB, Manuals: manuals}, true, false, 2},
		{"a changed value with one manual", Change{OldCatalogue: catA, NewCatalogue: catB, Changed: []string{Manuals + "en.md"}, Manuals: manuals}, true, false, 1},
		{"a changed value with every manual", Change{OldCatalogue: catA, NewCatalogue: catB, Changed: manuals, Manuals: manuals}, true, true, 0},
		{"a changed value with a reason", Change{OldCatalogue: catA, NewCatalogue: catB, Manuals: manuals,
			Body: "Manual: not needed — fixes a typo"}, true, true, 2},
		{"a new route", Change{OldShell: shellA, NewShell: shellB, Manuals: manuals}, true, false, 2},
		{"a new module", Change{Added: []string{Assets + "present.js", Assets + "locales/fr.js"}, Manuals: manuals}, true, false, 2},
	}
	for _, tc := range cases {
		r, err := Evaluate(tc.c)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if r.Visible() != tc.visible || r.OK() != tc.ok || len(r.Missing) != tc.missing {
			t.Errorf("%s: visible %v, ok %v, missing %q", tc.name, r.Visible(), r.OK(), r.Missing)
		}
	}

	r, _ := Evaluate(Change{OldCatalogue: catA, NewCatalogue: catB, OldShell: shellA, NewShell: shellB,
		Added: []string{Assets + "present.js"}, Manuals: manuals})
	if !slices.Equal(r.Keys, []string{"album.rename", "common.close"}) {
		t.Errorf("Keys = %q; $name is no label", r.Keys)
	}
	if !slices.Equal(r.Routes, []string{"present"}) || !slices.Equal(r.Views, []string{"presentScreen"}) || !slices.Equal(r.Modules, []string{Assets + "present.js"}) {
		t.Errorf("Routes %q, Views %q, Modules %q", r.Routes, r.Views, r.Modules)
	}

	if _, err := Evaluate(Change{OldCatalogue: "nonsense"}); err == nil {
		t.Error("a broken base catalogue is not reported")
	}
	if _, err := Evaluate(Change{NewCatalogue: "nonsense"}); err == nil {
		t.Error("a broken head catalogue is not reported")
	}
}

func TestParseSources(t *testing.T) {
	ps, err := ParseSources("# comment\n\nhome app.js#homeView key:home.\nicons/shop shop.js\n")
	if err != nil || len(ps) != 2 || ps[0].Name != "home" || !slices.Equal(ps[0].Sources, []string{"app.js#homeView", "key:home."}) {
		t.Fatalf("ParseSources = %+v, %v", ps, err)
	}
	if got := ps[0].Files([]string{"en", "de"}); !slices.Equal(got, []string{Pictures + "en/home.png", Pictures + "de/home.png"}) {
		t.Errorf("Files = %q", got)
	}
	if got := ps[1].Files([]string{"en", "de"}); !slices.Equal(got, []string{Pictures + "icons/shop.png"}) {
		t.Errorf("a neutral picture's Files = %q", got)
	}
	for _, bad := range []string{"home\n", "Home app.js\n", "home app.js\nhome shop.js\n", "home ../x.js\n", "home app.js#\n", "home key:\n"} {
		if _, err := ParseSources(bad); err == nil {
			t.Errorf("ParseSources(%q) accepts it", bad)
		}
	}
}

func TestExcerpt(t *testing.T) {
	text := "const glyphs = {\n  a: 1,\n};\nexport async function homeView() {\n  if (x) {\n  }\n}\nfunction homeViewer() {\n}\n"
	if got, ok := Excerpt(text, "app.js#homeView"); !ok || got != "export async function homeView() {\n  if (x) {\n  }\n}" {
		t.Errorf("Excerpt of a function = %q, %v", got, ok)
	}
	if got, ok := Excerpt(text, "app.js#glyphs"); !ok || got != "const glyphs = {\n  a: 1,\n}" {
		t.Errorf("Excerpt of a constant = %q, %v", got, ok)
	}
	if _, ok := Excerpt(text, "app.js#missing"); ok {
		t.Error("Excerpt finds a function the text lacks")
	}
	if got, ok := Excerpt(text, "app.js"); !ok || got != text {
		t.Error("Excerpt of a file is not the whole file")
	}
	if SourceFile("key:home.") != "" || SourceFile("app.js#homeView") != Assets+"app.js" {
		t.Error("SourceFile")
	}
}

func TestPicturesNotNeeded(t *testing.T) {
	if got := PicturesNotNeeded("x\nPictures: not needed — a tooltip\n"); got != "a tooltip" {
		t.Errorf("PicturesNotNeeded = %q", got)
	}
	if got := PicturesNotNeeded("Pictures: not needed\nManual: not needed — x\n"); got != "" {
		t.Errorf("a line without a reason gives %q", got)
	}
}
