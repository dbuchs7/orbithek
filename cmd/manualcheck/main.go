// Command manualcheck is Check B of ADR-0185: a pull request that changes
// what people see changes every user manual, or says in its description
// why it need not. It is run by CI on pull requests:
//
//	go run ./cmd/manualcheck [-advisory] [-pr-body-file FILE] <base> <head>
//
// It compares head with the point where it left base, as a pull request's
// diff does. A pull request changes what people see when it adds or changes
// a value of locales/en.js, adds a route or a view to app.js, or adds a
// module of the UI; it passes when it also changes every manual/<lang>.md,
// or when its description holds a line "Manual: not needed — <reason>".
// See internal/manualcheck for the rules.
//
// With -advisory the result is reported but never fails: a missing manual
// becomes a warning annotation and the command exits 0. CI runs in this
// mode until the maintainer makes the check required.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/dbuchs7/orbithek/internal/manualcheck"
)

func main() {
	prBodyFile := flag.String("pr-body-file", "", "file holding the pull request description, which may say \"Manual: not needed — <reason>\"")
	advisory := flag.Bool("advisory", false, "report a missing manual as a warning and exit 0")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: manualcheck [-advisory] [-pr-body-file FILE] <base> <head>")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 2 {
		flag.Usage()
		os.Exit(2)
	}
	ok, err := run(os.Stdout, flag.Arg(0), flag.Arg(1), *prBodyFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "manualcheck:", err)
		os.Exit(2)
	}
	if ok {
		return
	}
	if *advisory {
		// A GitHub Actions workflow command: shown as a warning on the run
		// and the pull request, without failing the job.
		fmt.Println("::warning title=User manual not updated (advisory)::This pull request changes what people see but not every user manual. Update internal/web/static/assets/manual/*.md, or add a line \"Manual: not needed — <reason>\" to the description (ADR-0185).")
		return
	}
	os.Exit(1)
}

// run reads the change between base and head from git, applies the rules
// and writes the report to w. It reports whether the pull request passes.
func run(w io.Writer, base, head, prBodyFile string) (bool, error) {
	var c manualcheck.Change
	if prBodyFile != "" {
		body, err := os.ReadFile(prBodyFile)
		if err != nil {
			return false, err
		}
		c.Body = string(body)
	}
	mergeBase, err := git("merge-base", base, head)
	if err != nil {
		return false, err
	}
	mergeBase = strings.TrimSpace(mergeBase)

	status, err := git("diff", "--name-status", "-M", mergeBase, head)
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(status, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		// A rename lists the old and the new path; the new one is changed.
		p := fields[len(fields)-1]
		c.Changed = append(c.Changed, p)
		if fields[0] == "A" {
			c.Added = append(c.Added, p)
		}
	}
	if c.OldCatalogue, err = show(mergeBase, manualcheck.Catalogue); err != nil {
		return false, err
	}
	if c.NewCatalogue, err = show(head, manualcheck.Catalogue); err != nil {
		return false, err
	}
	if c.OldShell, err = show(mergeBase, manualcheck.Shell); err != nil {
		return false, err
	}
	if c.NewShell, err = show(head, manualcheck.Shell); err != nil {
		return false, err
	}
	files, err := git("ls-tree", "--name-only", head, manualcheck.Manuals)
	if err != nil {
		return false, err
	}
	for _, f := range strings.Fields(files) {
		if manualcheck.IsManual(f) {
			c.Manuals = append(c.Manuals, f)
		}
	}

	r, err := manualcheck.Evaluate(c)
	if err != nil {
		return false, err
	}
	report(w, r)
	return r.OK(), nil
}

// report writes what the pull request changes that people see, and the
// verdict.
func report(w io.Writer, r manualcheck.Report) {
	if !r.Visible() {
		fmt.Fprintln(w, "ok    the pull request changes nothing people see that the manual must follow")
		return
	}
	fmt.Fprintln(w, "The pull request changes what people see:")
	list(w, "catalogue values added or changed", r.Keys)
	list(w, "routes added", r.Routes)
	list(w, "views added", r.Views)
	list(w, "UI modules added", r.Modules)
	switch {
	case len(r.Missing) == 0:
		fmt.Fprintln(w, "ok    every user manual is changed too")
	case r.Reason != "":
		fmt.Fprintf(w, "ok    the description says the manual needs no change: %s\n", r.Reason)
	default:
		fmt.Fprintln(w, "FAIL  these user manuals are not changed:")
		for _, m := range r.Missing {
			fmt.Fprintf(w, "      - %s\n", m)
		}
		fmt.Fprintln(w, "      Describe the change in every manual, or add a line of its own to the pull request description:")
		fmt.Fprintln(w, "      Manual: not needed — <reason>")
	}
}

// list writes up to ten entries of a list under its title.
func list(w io.Writer, title string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(w, "  %s (%d):\n", title, len(items))
	for i, s := range items {
		if i == 10 {
			fmt.Fprintf(w, "    … and %d more\n", len(items)-10)
			break
		}
		fmt.Fprintf(w, "    %s\n", s)
	}
}

// show returns a file at a commit, or "" when the commit has no such file.
func show(commit, path string) (string, error) {
	if _, err := git("cat-file", "-e", commit+":"+path); err != nil {
		return "", nil
	}
	return git("show", commit+":"+path)
}

func git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}
