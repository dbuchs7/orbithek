package adr

// Numbering of records in flight (ADR-0007, ADR-0216).
//
// A record written on a branch carries no number: it lives at
// draft-<slug>.md, heads with `# ADR-DRAFT: Title`, has no row in the
// index and is cited as ADR-draft-<slug>. Two branches that each write a
// record touch different files and no shared line, so they cannot clash.
// AssignNumbers gives every draft the next free number once the branch is
// up to date with main, as the pull request's last commit, and rewrites
// the file name, the heading, the index row and every citation in one
// pass. A number, once on main, is never reassigned.

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	numberedFileRE = regexp.MustCompile(`^(\d{4})-[a-z0-9][a-z0-9.-]*\.md$`)
	draftFileRE    = regexp.MustCompile(`^draft-([a-z0-9]+(?:[a-z0-9.-]*[a-z0-9])?)\.md$`)
	draftHeadingRE = regexp.MustCompile(`(?m)^# ADR-DRAFT: (\S.*)$`)
	statusLineRE   = regexp.MustCompile(`(?m)^- \*\*Status:\*\* (.+)$`)
	implLineRE     = regexp.MustCompile(`(?m)^- \*\*Implementation:\*\* (.+)$`)
	// indexRowRE is a row of the index in README.md.
	indexRowRE = regexp.MustCompile(`^\| \[(\d{4})\]\(([^)]+)\) \| (.+?) \| (.+?) \| (.+?) \|$`)
	// citationRE finds a citation of a draft anywhere in the tree.
	citationRE = regexp.MustCompile(`ADR-draft-([a-z0-9]+(?:[a-z0-9.-]*[a-z0-9])?)`)
)

// Assignment is one draft becoming a numbered record.
type Assignment struct {
	Slug                          string
	Num                           int
	From, To                      string // draft-<slug>.md, NNNN-<slug>.md
	Title, Status, Implementation string
}

// draft is a record in flight as read from its file.
type draft struct {
	slug, file, title, status, impl string
}

// readDrafts lists the drafts in dir, sorted by slug, and the highest
// number a numbered record holds.
func readDrafts(dir string) ([]draft, int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, 0, err
	}
	var drafts []draft
	high := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			continue
		}
		if m := numberedFileRE.FindStringSubmatch(name); m != nil {
			if n, _ := strconv.Atoi(m[1]); n > high {
				high = n
			}
			continue
		}
		m := draftFileRE.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, 0, err
		}
		d := draft{slug: m[1], file: name}
		h := draftHeadingRE.FindSubmatch(body)
		s := statusLineRE.FindSubmatch(body)
		i := implLineRE.FindSubmatch(body)
		if h == nil || s == nil || i == nil {
			return nil, 0, fmt.Errorf("%s: a draft needs a `# ADR-DRAFT: Title` heading and Status and Implementation lines", name)
		}
		d.title, d.status, d.impl = strings.TrimSpace(string(h[1])), strings.TrimSpace(string(s[1])), strings.TrimSpace(string(i[1]))
		if strings.Contains(d.title+d.status+d.impl, "|") {
			return nil, 0, fmt.Errorf("%s: a | in the title, status or implementation would break the index table", name)
		}
		drafts = append(drafts, d)
	}
	// Slug order, not directory order: which number each draft gets must
	// not depend on how the file system lists them.
	sort.Slice(drafts, func(i, j int) bool { return drafts[i].slug < drafts[j].slug })
	return drafts, high, nil
}

// AssignNumbers numbers every draft under root/docs/adr: it renames the
// file, rewrites its heading, appends its row to the index and rewrites
// every ADR-draft-<slug> citation and draft-<slug>.md link in the tree.
// With no drafts it changes nothing. Every draft is checked before any is
// renamed, so a refusal leaves the tree as it was.
func AssignNumbers(root string) ([]Assignment, error) {
	dir := filepath.Join(root, "docs", "adr")
	drafts, next, err := readDrafts(dir)
	if err != nil || len(drafts) == 0 {
		return nil, err
	}
	var out []Assignment
	for _, d := range drafts {
		next++
		a := Assignment{Slug: d.slug, Num: next, From: d.file, To: fmt.Sprintf("%04d-%s.md", next, d.slug),
			Title: d.title, Status: d.status, Implementation: d.impl}
		path := filepath.Join(dir, a.From)
		body, err := os.ReadFile(path)
		if err != nil {
			return out, err
		}
		body = draftHeadingRE.ReplaceAll(body, []byte(fmt.Sprintf("# ADR-%04d: $1", a.Num)))
		// Rewrite in place, then rename: the other order leaves a moment in
		// which the decision is filed twice.
		if err := os.WriteFile(path, body, 0o644); err != nil {
			return out, err
		}
		if err := os.Rename(path, filepath.Join(dir, a.To)); err != nil {
			return out, err
		}
		out = append(out, a)
	}
	if err := rewriteCitations(root, out); err != nil {
		return out, err
	}
	return out, appendIndexRows(filepath.Join(dir, "README.md"), out)
}

// citable are the file types that may cite a record. An allowlist, as the
// pass rewrites files in place.
var citable = map[string]bool{
	".cjs": true, ".css": true, ".go": true, ".html": true, ".js": true, ".json": true,
	".md": true, ".mjs": true, ".sh": true, ".txt": true, ".yaml": true, ".yml": true,
}

// skipDirs never cite a record of this repository and are costly to walk.
var skipDirs = map[string]bool{".git": true, "node_modules": true, "vendor": true, "data": true}

// rewriteCitations moves every citation of a draft onto its number. A
// citation is matched as a whole token: the citation of a draft whose slug
// begins with another draft's slug is left to its own draft.
func rewriteCitations(root string, assigned []Assignment) error {
	bySlug := make(map[string]Assignment, len(assigned))
	for _, a := range assigned {
		bySlug[a.Slug] = a
	}
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if !citable[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out := citationRE.ReplaceAllFunc(body, func(c []byte) []byte {
			if a, ok := bySlug[string(c[len("ADR-draft-"):])]; ok {
				return []byte(fmt.Sprintf("ADR-%04d", a.Num))
			}
			return c
		})
		for _, a := range assigned {
			out = bytes.ReplaceAll(out, []byte(a.From), []byte(a.To))
		}
		if bytes.Equal(out, body) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return os.WriteFile(path, out, info.Mode().Perm())
	})
}

// appendIndexRows adds one row per numbered draft directly after the last
// row of the index: a blank line between rows would end the table.
func appendIndexRows(readme string, assigned []Assignment) error {
	body, err := os.ReadFile(readme)
	if err != nil {
		return err
	}
	lines := strings.Split(string(body), "\n")
	last := -1
	for i, l := range lines {
		if indexRowRE.MatchString(l) {
			last = i
		}
	}
	if last < 0 {
		return fmt.Errorf("%s: no index rows to append to", readme)
	}
	rows := make([]string, 0, len(assigned))
	for _, a := range assigned {
		rows = append(rows, fmt.Sprintf("| [%04d](%s) | %s | %s | %s |", a.Num, a.To, a.Title, a.Status, a.Implementation))
	}
	out := append(append(append([]string{}, lines[:last+1]...), rows...), lines[last+1:]...)
	return os.WriteFile(readme, []byte(strings.Join(out, "\n")), 0o644)
}
