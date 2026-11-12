package noteimport

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/dbuchs7/orbithek/internal/item"
)

// Of several notes so named, a link finds the nearest; a folder's own
// note is the one named as the folder before index and README; what a
// note names that is not a note stays text and is reported.
func TestLinksFindTheNearestNote(t *testing.T) {
	a := open(t, zipOf(t,
		"a/Same.md", "In a.",
		"b/Same.md", "In b.",
		"b/From.md", "[[Same]] [[a/Same]] [[Same.md]] [x](Same.md#top) [pdf](doc.pdf) [gone](missing.md) [js](javascript:alert(1)) [mail](mailto:a@b.example) ![[gone.png]] ![web](https://img.example/a.png) ![local](nowhere.png) #bad!tag #ok_tag\n\n![[doc.pdf]]",
		"b/doc.pdf", "%PDF",
		"c/README.md", "Readme.",
		"c/index.md", "Index.",
		"c/c.md", "Own note.",
		"d/README.md", "Readme of d.",
		"d/index.md", "Index of d.",
	))
	p := Build(a, Options{PageType: pageType})
	from := pageOf(t, p, "md/b/From.md")
	got := hrefs(from)
	want := []string{NoteLink("md/b/Same.md"), NoteLink("md/a/Same.md"), NoteLink("md/b/Same.md"), NoteLink("md/b/Same.md"), "mailto:a@b.example", "https://img.example/a.png"}
	if !slices.Equal(got, want) {
		t.Errorf("links %v, want %v", got, want)
	}
	for _, s := range []string{"the link to the file b/doc.pdf", "the link to missing.md", "the picture gone.png", "the picture nowhere.png"} {
		if !hasProblem(p, s) {
			t.Errorf("no problem %q in %v", s, p.Problems)
		}
	}
	if !slices.Equal(from.Labels, []string{"bad", "ok_tag"}) {
		t.Errorf("labels %v", from.Labels)
	}
	if c := pageOf(t, p, "dir/c"); c.Blocks[0].Text != "Own note." {
		t.Errorf("c's text %+v", c.Blocks)
	}
	if d := pageOf(t, p, "dir/d"); d.Blocks[0].Text != "Index of d." {
		t.Errorf("d's text %+v", d.Blocks)
	}
	// The note that is its folder's text is no page of its own; the other
	// notes named like index notes are.
	for id, want := range map[string]bool{"md/c/c.md": false, "md/d/index.md": false, "md/c/README.md": true, "md/d/README.md": true} {
		if got := slices.ContainsFunc(p.Pages, func(pg Page) bool { return pg.ID == id }); got != want {
			t.Errorf("%s is a page: %v", id, got)
		}
	}
	// A tag that is no label even with its levels joined is reported.
	pl := &planner{}
	var pg Page
	pl.label(&pg, "#"+strings.Repeat("x", 70), "n.md")
	pl.label(&pg, " ", "n.md")
	if len(pg.Labels) != 0 || len(pl.plan.Problems) != 1 {
		t.Errorf("labels %v problems %v", pg.Labels, pl.plan.Problems)
	}
}

// A Notion export's CSV without an ID, or that is no table, is left out;
// a CSV outside a database's folder still makes a database page.
func TestBrokenDatabasesAreReported(t *testing.T) {
	a := open(t, zipOf(t,
		"Page "+idWiki+".md", "# Page",
		"Loose.csv", "a,b\n1,2\n",
		"Empty "+idChild+".csv", "",
		"Rows "+idTasks+".csv", "Name,Kind\nOne,a\nOne,b\n",
	))
	p := Build(a, Options{PageType: pageType, Lang: "fr"})
	if !hasProblem(p, "Loose.csv: a CSV file without a page ID") || !hasProblem(p, "Empty "+idChild+".csv is no table") {
		t.Errorf("problems %v", p.Problems)
	}
	rows := pageOf(t, p, "notion/"+idTasks)
	if rows.Title != "Rows" || rows.Path != "Rows "+idTasks+".csv" {
		t.Errorf("rows %+v", rows)
	}
	one, two := pageOf(t, p, "notion/"+idTasks+"/row/one"), pageOf(t, p, "notion/"+idTasks+"/row/one/2")
	if one.Blocks[0].Text != "Property\tValue\nKind\ta" || two.Blocks[0].Text != "Property\tValue\nKind\tb" {
		t.Errorf("rows %+v %+v", one.Blocks, two.Blocks)
	}
}

// A run reports what it cannot write: a page imported before as another
// type, pictures beyond a page's fifty, and its problems up to its bound.
func TestARunReportsWhatItCannotWrite(t *testing.T) {
	db := openDB(t)
	var pics []string
	var note strings.Builder
	for i := range maxPictures + 1 {
		pics = append(pics, fmt.Sprintf("p%d.png", i), string(append(pngData, byte(i))))
		fmt.Fprintf(&note, "![[p%d.png]]\n\n", i)
	}
	a := open(t, zipOf(t, append([]string{"Many.md", note.String()}, pics...)...))
	rep, err := Run(db, a, Build(a, Options{PageType: pageType}), Target{Space: "NOTES", Name: "many", Who: "pat"}, false)
	if err != nil || rep.Pictures != maxPictures || !slices.ContainsFunc(rep.Problems, func(s string) bool { return strings.Contains(s, "more than 50 pictures") }) {
		t.Fatalf("pictures %d %v %v", rep.Pictures, rep.Problems, err)
	}
	var types []item.TypeDef
	db.Read(func(s *item.Store) { types, _ = Types(s, "task") })
	rep, err = Run(db, a, Build(a, Options{PageType: types[0]}), Target{Space: "NOTES", Name: "many", Who: "pat"}, false)
	if err != nil || !slices.ContainsFunc(rep.Problems, func(s string) bool { return strings.Contains(s, "imported as page before, now task") }) {
		t.Fatalf("another type %v %v", rep.Problems, err)
	}
	var r Report
	for i := range problemsShown + 5 {
		r.problem("p%d", i)
	}
	if len(r.Problems) != problemsShown+1 || r.Problems[problemsShown] != "… and more" {
		t.Errorf("problems %d", len(r.Problems))
	}
}
