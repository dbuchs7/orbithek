// Command adrnum numbers the architecture decision records in flight
// (ADR-0007, ADR-draft-adr-numbers-assigned-by-the-pull-requests-last-commit).
//
// A record is written on a branch as docs/adr/draft-<slug>.md, without a
// number, since two branches asking for "the next free number" at the same
// time get the same one. When the pull request is ready and up to date
// with main, its last commit numbers the drafts: this command renames each
// to NNNN-<slug>.md, rewrites its heading, appends its row to the index
// and rewrites every ADR-draft-<slug> citation in the tree.
//
// Usage:
//
//	go run ./docs/adr/cmd/adrnum [repository root]
//
// With no drafts it changes nothing.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/dbuchs7/orbithek/docs/adr"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, out, errOut io.Writer) int {
	root := "."
	switch len(args) {
	case 0:
	case 1:
		root = args[0]
	default:
		fmt.Fprintln(errOut, "usage: adrnum [repository root]")
		return 2
	}
	assigned, err := adr.AssignNumbers(root)
	if err != nil {
		fmt.Fprintln(errOut, "adrnum:", err)
		return 1
	}
	if len(assigned) == 0 {
		fmt.Fprintln(out, "adrnum: no drafts to number")
		return 0
	}
	for _, a := range assigned {
		fmt.Fprintf(out, "ADR-%04d  %s -> %s\n", a.Num, a.From, a.To)
	}
	fmt.Fprintf(out, "adrnum: numbered %d record(s); check the diff, run go test ./docs/adr, then commit.\n", len(assigned))
	return 0
}
