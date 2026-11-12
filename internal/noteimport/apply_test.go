package noteimport

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/dbuchs7/orbithek/internal/auth"
	"github.com/dbuchs7/orbithek/internal/item"
	"github.com/dbuchs7/orbithek/internal/storage"
)

func openDB(t *testing.T) *storage.DB {
	t.Helper()
	db, _, err := storage.OpenWith(t.TempDir(), storage.Options{Auth: []auth.Option{auth.WithIterations(1000)}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	err = db.Write(func(s *item.Store) error {
		for _, step := range []func() error{
			func() error { return s.CreateSpace("admin", "NOTES", "Notes") },
			func() error { return s.GrantRole("admin", "NOTES", "pat", item.SpaceAdmin) },
			func() error { return s.GrantRole("admin", "NOTES", "anna", item.Editor) },
			func() error { return s.DefineType("admin", pageType) },
			func() error { return s.DefineType("admin", taskType) },
		} {
			if err := step(); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func vault(t *testing.T, home string, more ...string) *Archive {
	return open(t, zipOf(t, append([]string{
		"Vault/Home.md", home,
		"Vault/Projects/Alpha.md", "# Alpha\n\nBack to [[Home]].\n",
		"Vault/pic.png", string(pngData),
		"Vault/drawing.svg", "<svg/>",
	}, more...)...))
}

const homeNote = "---\ntags: [start]\ndue: 2026-03-01\n---\n# Home\n\nSee [[Alpha]] and [[Zeta]].\n\n![[pic.png]]\n\n![[drawing.svg]]\n"

func importAs(t *testing.T, db *storage.DB, a *Archive, who string, dry bool) Report {
	t.Helper()
	var types []item.TypeDef
	db.Read(func(s *item.Store) { types, _ = Types(s, "page", "task") })
	rep, err := Run(db, a, Build(a, Options{PageType: types[0], RowType: types[1], Lang: "en"}), Target{Space: "NOTES", Name: "vault", Who: who}, dry)
	if err != nil {
		t.Fatal(err)
	}
	return rep
}

func byOrigin(t *testing.T, db *storage.DB, id string) item.Item {
	t.Helper()
	var it item.Item
	var ok bool
	db.Read(func(s *item.Store) { it, ok = s.FindByOrigin(System, Instance("NOTES", "vault"), id) })
	if !ok {
		t.Fatalf("no page %s", id)
	}
	return it
}

// A dry run writes nothing and says what a run would make; the run makes
// it, with mentions between the pages, pictures attached and labels and
// fields set; a re-run changes nothing; a changed archive changes what
// changed, keeps what people wrote and never deletes (ADR-0211).
func TestAnImportIsRehearsedRunAndRepeated(t *testing.T) {
	db := openDB(t)
	a := vault(t, homeNote, "Vault/Zeta.md", "Last.\n")
	dry := importAs(t, db, a, "pat", true)
	if !dry.DryRun || dry.New != 4 || dry.Pictures != 2 || dry.Format != FormatFolder {
		t.Fatalf("dry run %+v", dry)
	}
	var count int
	db.Read(func(s *item.Store) { count = len(s.FindBySystem(System)) })
	if count != 0 {
		t.Fatalf("a dry run wrote %d pages", count)
	}

	rep := importAs(t, db, vault(t, homeNote, "Vault/Zeta.md", "Last.\n"), "pat", false)
	if rep.New != 4 || rep.Pictures != 1 || rep.Changed+rep.Unchanged != 0 {
		t.Fatalf("run %+v", rep)
	}
	if !slices.ContainsFunc(rep.Problems, func(p string) bool { return strings.Contains(p, "drawing.svg is no PNG") }) {
		t.Errorf("problems %v", rep.Problems)
	}
	home := byOrigin(t, db, "md/Home.md")
	alpha := byOrigin(t, db, "md/Projects/Alpha.md")
	zeta := byOrigin(t, db, "md/Zeta.md")
	folder := byOrigin(t, db, "dir/Projects")
	if home.Title != "Home" || !slices.Equal(home.Labels, []string{"start"}) || home.Properties["due"] == nil || alpha.Parent != folder.ID {
		t.Fatalf("home %+v alpha %+v", home, alpha)
	}
	// Alpha's link to Home and Home's to Zeta, made after it, are mentions.
	if !slices.Contains(home.Body.Blocks[0].Mentions, alpha.ID) || !slices.Contains(home.Body.Blocks[0].Mentions, zeta.ID) {
		t.Errorf("home mentions %+v", home.Body.Blocks[0])
	}
	if !slices.Contains(alpha.Body.Blocks[0].Mentions, home.ID) {
		t.Errorf("alpha mentions %+v", alpha.Body.Blocks[0])
	}
	var images int
	for _, b := range home.Body.Blocks {
		if b.Kind == item.Image {
			images++
		}
		for _, m := range b.Marks {
			if strings.Contains(m.Href, ".invalid/") {
				t.Errorf("an address of the import stays: %+v", m)
			}
		}
	}
	if images != 1 || len(home.Attachments) != 1 || home.Attachments[0].Name != "pic.png" {
		t.Errorf("pictures: %d images, %+v", images, home.Attachments)
	}

	// A re-run of the same archive changes nothing.
	again := importAs(t, db, vault(t, homeNote, "Vault/Zeta.md", "Last.\n"), "pat", false)
	if again.Unchanged != 4 || again.New+again.Changed != 0 {
		t.Fatalf("re-run %+v", again)
	}
	if byOrigin(t, db, "md/Home.md").Version != home.Version {
		t.Error("an unchanged page was written")
	}

	// People edit Alpha and label Home; the archive changes Home, drops
	// Zeta and adds Omega.
	err := db.Write(func(s *item.Store) error {
		if err := s.As("anna").CommitBody(alpha.ID, item.Doc{Blocks: []item.Block{{Kind: item.Paragraph, Text: "Edited here."}}}); err != nil {
			return err
		}
		return s.As("anna").SetLabels(home.ID, []string{"start", "mine"})
	})
	if err != nil {
		t.Fatal(err)
	}
	changed := importAs(t, db, vault(t, strings.Replace(homeNote, "See", "Read", 1), "Vault/Omega.md", "New."), "pat", false)
	if changed.New != 1 || changed.Changed != 1 || changed.Kept != 1 || changed.Unchanged != 2 {
		t.Fatalf("changed run %+v", changed)
	}
	home = byOrigin(t, db, "md/Home.md")
	if !strings.HasPrefix(home.Body.Blocks[0].Text, "Read") || !slices.Equal(home.Labels, []string{"mine", "start"}) || len(home.Attachments) != 1 {
		t.Errorf("home after the change %+v", home)
	}
	if alpha = byOrigin(t, db, "md/Projects/Alpha.md"); alpha.Body.Blocks[0].Text != "Edited here." {
		t.Errorf("what people wrote is lost: %+v", alpha.Body)
	}
	if z := byOrigin(t, db, "md/Zeta.md"); z.Presence != item.Live {
		t.Errorf("a page no longer in the archive is not kept: %+v", z)
	}
	var imports []Import
	db.Read(func(s *item.Store) { imports = Imports(s, "anna", "NOTES") })
	if len(imports) != 1 || imports[0].Name != "vault" || imports[0].Pages != 5 {
		t.Errorf("imports %+v", imports)
	}
}

// Only a room administrator imports.
func TestOnlyARoomAdministratorImports(t *testing.T) {
	db := openDB(t)
	a := vault(t, homeNote)
	_, err := Run(db, a, Build(a, Options{PageType: pageType}), Target{Space: "NOTES", Name: "vault", Who: "anna"}, true)
	if !errors.Is(err, item.ErrForbidden) {
		t.Fatalf("an editor imports: %v", err)
	}
	var err2 error
	db.Read(func(s *item.Store) { _, err2 = Types(s, "page", "missing") })
	if !errors.Is(err2, ErrType) {
		t.Errorf("a missing type: %v", err2)
	}
	if _, err := CleanName("  "); !errors.Is(err, ErrName) {
		t.Errorf("an empty name: %v", err)
	}
	if n, err := CleanName(" my\nvault "); err != nil || n != "my vault" {
		t.Errorf("name %q %v", n, err)
	}
}
