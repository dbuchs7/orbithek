package adr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// adrDir writes an ADR directory with the given files.
func adrDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const readmeA = "# ADRs\n\nIntro.\n\n## Index\n\n" + IndexBegin + "\n\nold\n\n" + IndexEnd + "\n\nAfter.\n"

func TestGenerate(t *testing.T) {
	dir := adrDir(t, map[string]string{
		"README.md":      readmeA,
		"template.md":    "# ADR-NNNN: Title\n",
		"0002-second.md": "# ADR-0002: Second | piped\n\n- **Status:** Proposed\n- **Implementation:** Not started\n\nBody.\n",
		"0001-first.md":  "# ADR-0001: First\n- **Status:** Accepted\n- **Implementation:** Landed\n",
		"notes.txt":      "not a record",
	})
	readme, changed, err := Generate(dir)
	if err != nil || !changed {
		t.Fatalf("Generate = changed %v, %v", changed, err)
	}
	want := "# ADRs\n\nIntro.\n\n## Index\n\n" + IndexBegin + "\n\n" +
		"| No. | Title | Status | Implementation |\n|---|---|---|---|\n" +
		"| [0001](0001-first.md) | First | Accepted | Landed |\n" +
		"| [0002](0002-second.md) | Second \\| piped | Proposed | Not started |\n" +
		"\n" + IndexEnd + "\n\nAfter.\n"
	if readme != want {
		t.Errorf("Generate =\n%s\nwant\n%s", readme, want)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, changed, err := Generate(dir); err != nil || changed {
		t.Errorf("a generated index is generated again: changed %v, %v", changed, err)
	}
}

func TestGenerateErrors(t *testing.T) {
	for name, files := range map[string]map[string]string{
		"no README":      {"0001-a.md": "# ADR-0001: A\n"},
		"no index block": {"README.md": "# ADRs\n", "0001-a.md": "# ADR-0001: A\n"},
		"two blocks":     {"README.md": readmeA + readmeA, "0001-a.md": "# ADR-0001: A\n"},
		"ends first":     {"README.md": IndexEnd + "\n" + IndexBegin + "\n", "0001-a.md": "# ADR-0001: A\n"},
		"bad heading":    {"README.md": readmeA, "0001-a.md": "# Title\n"},
		"empty record":   {"README.md": readmeA, "0001-a.md": ""},
		"twice a key":    {"README.md": readmeA, "0001-a.md": "# ADR-0001: A\n- **Status:** Accepted\n- **Status:** Proposed\n"},
	} {
		if _, _, err := Generate(adrDir(t, files)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if _, _, err := Generate(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("a missing directory: no error")
	}
	if _, err := ParseRecord(t.TempDir(), "0001-missing.md"); err == nil {
		t.Error("a missing record: no error")
	}
}

func TestIsRecordFile(t *testing.T) {
	for name, want := range map[string]bool{
		"0001-a.md": true, "0185-the-user-manual.md": true, "README.md": false,
		"template.md": false, "1-a.md": false, "0001-A.md": false, "0001-a.txt": false,
	} {
		if got := IsRecordFile(name); got != want {
			t.Errorf("IsRecordFile(%q) = %v", name, got)
		}
	}
	if !strings.Contains(IndexBegin, "go generate ./docs/adr") {
		t.Error("the index's first line does not say how it is made")
	}
}
