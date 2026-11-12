package adr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tree writes files under a temporary repository root.
func tree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func read(t *testing.T, root, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

const index = "# Index\n\n| No. | Title | Status | Implementation |\n|---|---|---|---|\n| [0001](0001-first.md) | First | Accepted | Landed |\n| [0002](0002-second.md) | Second | Proposed | Partial |\n\nAfter the table.\n"

func draftBody(title string) string {
	return "# ADR-DRAFT: " + title + "\n\n- **Status:** Proposed\n- **Implementation:** Not started\n- **Date:** 2026-11-12\n- **Deciders:** x\n\nBody.\n"
}

func TestDraftsAreNumberedInSlugOrderAndEveryCitationFollows(t *testing.T) {
	root := tree(t, map[string]string{
		"docs/adr/README.md":           index,
		"docs/adr/0001-first.md":       "# ADR-0001: First\n",
		"docs/adr/0002-second.md":      "# ADR-0002: Second\n",
		"docs/adr/draft-fixture-b.md":  draftBody("Bee"),
		"docs/adr/draft-fixture-ab.md": draftBody("Ay bee"),
		"internal/x/x.go":              "// see ADR-draft-fixture-ab and ADR-draft-fixture-b, not ADR-draft-fixture-bc\n",
		"docs/guide.md":                "[the record](adr/draft-fixture-b.md), ADR-draft-fixture-ab.\n",
		"internal/x/x.bin":             "ADR-draft-fixture-b\n",
		"node_modules/m/m.js":          "ADR-draft-fixture-b\n",
	})
	got, err := AssignNumbers(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Slug != "fixture-ab" || got[0].Num != 3 || got[1].Slug != "fixture-b" || got[1].Num != 4 {
		t.Fatalf("assigned %+v, want ab as 3 and b as 4", got)
	}
	if s := read(t, root, "docs/adr/0003-fixture-ab.md"); !strings.HasPrefix(s, "# ADR-0003: Ay bee\n") {
		t.Errorf("heading not rewritten: %q", s)
	}
	if _, err := os.Stat(filepath.Join(root, "docs/adr/draft-fixture-b.md")); !os.IsNotExist(err) {
		t.Errorf("the draft is still there: %v", err)
	}
	if s := read(t, root, "internal/x/x.go"); s != "// see ADR-0003 and ADR-0004, not ADR-draft-fixture-bc\n" {
		t.Errorf("citations in code: %q", s)
	}
	if s := read(t, root, "docs/guide.md"); s != "[the record](adr/0004-fixture-b.md), ADR-0003.\n" {
		t.Errorf("citations in a document: %q", s)
	}
	for _, f := range []string{"internal/x/x.bin", "node_modules/m/m.js"} {
		if s := read(t, root, f); s != "ADR-draft-fixture-b\n" {
			t.Errorf("%s was rewritten: %q", f, s)
		}
	}
	want := strings.Replace(index, "| Partial |\n", "| Partial |\n| [0003](0003-fixture-ab.md) | Ay bee | Proposed | Not started |\n| [0004](0004-fixture-b.md) | Bee | Proposed | Not started |\n", 1)
	if s := read(t, root, "docs/adr/README.md"); s != want {
		t.Errorf("index:\n%s\nwant:\n%s", s, want)
	}
	again, err := AssignNumbers(root)
	if err != nil || len(again) != 0 {
		t.Errorf("a second run numbered %v, %v; want nothing", again, err)
	}
}

func TestADraftThatCannotBeNumberedChangesNothing(t *testing.T) {
	for name, body := range map[string]string{
		"no heading":      "- **Status:** Proposed\n- **Implementation:** Not started\n",
		"no status":       "# ADR-DRAFT: T\n- **Implementation:** Not started\n",
		"a pipe in title": draftBody("A | B"),
	} {
		root := tree(t, map[string]string{
			"docs/adr/README.md":          index,
			"docs/adr/draft-fixture-a.md": draftBody("Fine"),
			"docs/adr/draft-fixture-b.md": body,
			"docs/adr/0002-second.md":     "# ADR-0002: Second\n",
		})
		if _, err := AssignNumbers(root); err == nil {
			t.Errorf("%s: numbered", name)
		}
		if _, err := os.Stat(filepath.Join(root, "docs/adr/draft-fixture-a.md")); err != nil {
			t.Errorf("%s: a draft was renamed before the refusal: %v", name, err)
		}
		if read(t, root, "docs/adr/README.md") != index {
			t.Errorf("%s: the index changed", name)
		}
	}
}

func TestAnIndexWithoutRowsIsRefused(t *testing.T) {
	root := tree(t, map[string]string{"docs/adr/README.md": "# Index\n", "docs/adr/draft-fixture-a.md": draftBody("A")})
	if _, err := AssignNumbers(root); err == nil {
		t.Fatal("numbered into an index without rows")
	}
}

func TestNoDirectoryIsAnError(t *testing.T) {
	if _, err := AssignNumbers(t.TempDir()); err == nil {
		t.Fatal("no docs/adr, no error")
	}
}
