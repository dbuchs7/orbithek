package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "adr")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	index := "| [0001](0001-a.md) | A | Accepted | Landed |\n"
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("README.md", index)
	write("0001-a.md", "# ADR-0001: A\n")

	var out, errOut bytes.Buffer
	if code := run([]string{root}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "no drafts") {
		t.Fatalf("no drafts: %d %q %q", code, out.String(), errOut.String())
	}
	write("draft-b.md", "# ADR-DRAFT: B\n- **Status:** Proposed\n- **Implementation:** Not started\n")
	out.Reset()
	if code := run([]string{root}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "ADR-0002  draft-b.md -> 0002-b.md") {
		t.Fatalf("one draft: %d %q %q", code, out.String(), errOut.String())
	}
	write("draft-c.md", "no heading\n")
	if code := run([]string{root}, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "draft-c.md") {
		t.Fatalf("a bad draft: %d %q", code, errOut.String())
	}
	if code := run([]string{"a", "b"}, &out, &errOut); code != 2 {
		t.Fatalf("two arguments: %d", code)
	}
	if code := run(nil, &out, &errOut); code != 1 {
		t.Fatalf("the working directory of the test has no docs/adr: %d", code)
	}
}
