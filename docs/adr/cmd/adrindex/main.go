// Command adrindex writes the index of the architecture decision records,
// the table in docs/adr/README.md, from the records' headings and front
// matter:
//
//	go generate ./docs/adr                         # from the repository's root
//	go run ./docs/adr/cmd/adrindex -dir docs/adr   # the same
//	go run ./docs/adr/cmd/adrindex -dir docs/adr -check
//
// With -check it changes nothing and exits 1 when the index is not the one
// the records make. A conflict in the index between two branches is
// resolved by running it after the merge.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dbuchs7/orbithek/docs/adr"
)

func main() {
	dir := flag.String("dir", ".", "the ADR directory, holding README.md and the records")
	check := flag.Bool("check", false, "change nothing; exit 1 when the index is out of date")
	flag.Parse()
	code, err := run(os.Stdout, *dir, *check)
	if err != nil {
		fmt.Fprintln(os.Stderr, "adrindex:", err)
		os.Exit(2)
	}
	os.Exit(code)
}

// run generates the index of dir and writes it, or with check only reports
// whether it is current; it returns the exit code.
func run(w io.Writer, dir string, check bool) (int, error) {
	readme, changed, err := adr.Generate(dir)
	if err != nil {
		return 0, err
	}
	path := filepath.Join(dir, "README.md")
	switch {
	case !changed:
		fmt.Fprintf(w, "%s: the index is current\n", path)
		return 0, nil
	case check:
		fmt.Fprintf(w, "%s: the index is out of date; run `go generate ./docs/adr`\n", path)
		return 1, nil
	}
	if err := os.WriteFile(path, []byte(readme), 0o644); err != nil {
		return 0, err
	}
	fmt.Fprintf(w, "%s: index written\n", path)
	return 0, nil
}
