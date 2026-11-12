package repoconfig

import (
	"bufio"
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// otherProducts are the products imports read from: the import engine
// (ADR-0023), the import of notes (ADR-0209) and the connector to a
// reading service (ADR-0210). Notion and Obsidian are matched as written,
// since "notion" is also an English word.
var otherProducts = regexp.MustCompile(`(?i:confluence|jira|readwise)|Notion|Obsidian`)

// manualDir holds the user manuals; there, a section about importing may
// name the sources it imports from.
const manualDir = "internal/web/static/assets/manual/"

// namingAllowed reports whether a file or a line of it may name another
// product: only where the subject is import (AGENTS.md, "Other products are
// named only for import"). heading is the heading the line stands under,
// in a manual.
func namingAllowed(path, line, heading string) bool {
	switch {
	// The import engine, the item model's import, the golden logs, which
	// record an imported item, and the import command.
	case strings.HasPrefix(path, "internal/migrate/"),
		path == "internal/item/import.go", path == "internal/item/import_test.go",
		strings.HasPrefix(path, "internal/item/testdata/"),
		path == "cmd/orbithek/import.go", path == "cmd/orbithek/import_test.go",
		// The connector that runs the import (ADR-0128).
		strings.HasPrefix(path, "internal/connector/atlassian"),
		// The import of notes (ADR-0209) and the connector to a reading
		// service (ADR-0210).
		strings.HasPrefix(path, "internal/noteimport/"),
		strings.HasPrefix(path, "internal/connector/readwise"):
		return true
	// A manual's section about importing (ADR-0209, ADR-0210).
	case strings.HasPrefix(path, manualDir) && importHeading.MatchString(heading):
		return true
	// The import record itself, and ADRs accepted before the rule
	// (2026-10-19), which keep their wording.
	case strings.HasPrefix(path, "docs/adr/0023-"), strings.HasPrefix(path, "docs/adr/0209-"), strings.HasPrefix(path, "docs/adr/0210-"):
		return true
	case strings.HasPrefix(path, "docs/adr/00") && path < "docs/adr/0035":
		return true
	}
	// A line about import: it refers to an import record, or builds an
	// imported item's origin.
	return strings.Contains(line, "0023") || strings.Contains(line, "0209") || strings.Contains(line, "0210") || strings.Contains(line, "Origin{")
}

// importHeading is a manual's heading of a section about importing, in
// the manual's languages.
var importHeading = regexp.MustCompile(`(?i)import`)

// binary files are not searched.
var binary = map[string]bool{".png": true, ".pdf": true, ".ico": true}

// TestOtherProductsAreNamedOnlyForImport enforces the maintainer's rule of
// 2026-10-19 on every text file of the repository.
func TestOtherProductsAreNamedOnlyForImport(t *testing.T) {
	root := "../.."
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if binary[filepath.Ext(p)] {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if rel == "internal/repoconfig/naming_test.go" {
			return nil // this test names them to find them
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		sc := bufio.NewScanner(bytes.NewReader(data))
		sc.Buffer(nil, 1<<24)
		heading := ""
		for n := 1; sc.Scan(); n++ {
			line := sc.Text()
			if strings.HasPrefix(rel, manualDir) && strings.HasPrefix(line, "#") {
				heading = line
			}
			if otherProducts.MatchString(line) && !namingAllowed(rel, line, heading) {
				t.Errorf("%s:%d names another product outside import; describe the product in its own terms (AGENTS.md)", rel, n)
			}
		}
		return sc.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
}

// The check itself finds what it must and lets through what it may.
func TestNamingAllowed(t *testing.T) {
	for _, c := range []struct {
		path, line, heading string
		want                bool
	}{
		{"internal/migrate/source.go", "a Jira reader", "", true},
		{"docs/adr/0001-unified-item-data-model.md", "like Confluence", "", true},
		{"docs/adr/0035-comments-are-part-of-their-item.md", "like Confluence", "", false},
		{"docs/adr/0040-something.md", "like Jira", "", false},
		{"AGENTS.md", "the import engine (ADR-0023)", "", true},
		{"AGENTS.md", "the Jira domain", "", false},
		{"internal/web/static/assets/app.js", `h("p", { text: "Like Jira" })`, "", false},
		{"README.md", "a Confluence alternative", "", false},
		{"internal/noteimport/plan.go", "a Notion export", "", true},
		{"internal/connector/readwise.go", "the Readwise export", "", true},
		{"internal/connector/sync.go", "like Readwise", "", false},
		{"docs/adr/0209-importing-notes.md", "an Obsidian vault", "", true},
		{"docs/adr/0211-a-theme.md", "like Obsidian", "", false},
		{"internal/site/connectors.go", `KindReadwise = "readwise" // ADR-0210`, "", true},
		{"internal/web/static/assets/manual/en.md", "a Notion export", "### Importing notes", true},
		{"internal/web/static/assets/manual/de.md", "ein Notion-Export", "### Notizen importieren", true},
		{"internal/web/static/assets/manual/en.md", "as in Notion", "### Reading an item", false},
		{"internal/web/static/assets/app.js", "a Notion export", "### Importing notes", false},
	} {
		if got := namingAllowed(c.path, c.line, c.heading); got != c.want {
			t.Errorf("%s %q: allowed %v, want %v", c.path, c.line, got, c.want)
		}
	}
	// "notion" as a word names no product.
	if otherProducts.MatchString("the same notion of responsible") || !otherProducts.MatchString("a Readwise token") {
		t.Error("the products are matched wrongly")
	}
}
