package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dbuchs7/orbithek/docs/adr"
)

func TestRun(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("README.md", "# ADRs\n\n"+adr.IndexBegin+"\n"+adr.IndexEnd+"\n")
	write("0001-a.md", "# ADR-0001: A\n\n- **Status:** Accepted\n- **Implementation:** Landed\n")

	var out bytes.Buffer
	if code, err := run(&out, dir, true); err != nil || code != 1 || !strings.Contains(out.String(), "out of date") {
		t.Errorf("check of a stale index: %d, %v, %q", code, err, out.String())
	}
	out.Reset()
	if code, err := run(&out, dir, false); err != nil || code != 0 || !strings.Contains(out.String(), "index written") {
		t.Errorf("writing the index: %d, %v, %q", code, err, out.String())
	}
	data, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil || !strings.Contains(string(data), "| [0001](0001-a.md) | A | Accepted | Landed |") {
		t.Errorf("README.md after writing:\n%s", data)
	}
	out.Reset()
	if code, err := run(&out, dir, true); err != nil || code != 0 || !strings.Contains(out.String(), "is current") {
		t.Errorf("check of a current index: %d, %v, %q", code, err, out.String())
	}
	if _, err := run(&out, filepath.Join(dir, "missing"), false); err == nil {
		t.Error("a missing directory: no error")
	}
	// A README.md that cannot be written is an error.
	if err := os.Chmod(filepath.Join(dir, "README.md"), 0o400); err != nil {
		t.Fatal(err)
	}
	write("0002-b.md", "# ADR-0002: B\n")
	if os.Geteuid() != 0 {
		if _, err := run(&out, dir, false); err == nil {
			t.Error("a read-only README.md: no error")
		}
	}
}
