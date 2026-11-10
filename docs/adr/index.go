package adr

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// The index in README.md lies between these two lines. Everything between
// them is generated from the records' headings and front matter; the text
// around them is written by hand.
const (
	IndexBegin = "<!-- BEGIN INDEX: generated from the records' headings by `go generate ./docs/adr`; do not edit by hand -->"
	IndexEnd   = "<!-- END INDEX -->"
)

var (
	fileNameRE    = regexp.MustCompile(`^(\d{4})-[a-z0-9]+(?:[a-z0-9.-]*[a-z0-9])?\.md$`)
	headingRE     = regexp.MustCompile(`^# ADR-(\d{4}): (\S.*)$`)
	frontMatterRE = regexp.MustCompile(`^- \*\*([A-Za-z ]+):\*\* ?(.*)$`)
)

// Record is one parsed ADR file: its heading and the front matter below it.
type Record struct {
	File        string
	Number      string
	Title       string
	FrontMatter map[string]string
}

// IsRecordFile reports whether a file name in the ADR directory names a
// record: NNNN-lowercase-slug.md.
func IsRecordFile(name string) bool { return fileNameRE.MatchString(name) }

// ParseRecord reads the heading and front matter of the record at
// dir/name.
func ParseRecord(dir, name string) (Record, error) {
	f, err := os.Open(filepath.Join(dir, name))
	if err != nil {
		return Record{}, err
	}
	defer f.Close()

	r := Record{File: name, FrontMatter: make(map[string]string)}
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return Record{}, fmt.Errorf("empty file")
	}
	h := headingRE.FindStringSubmatch(sc.Text())
	if h == nil {
		return Record{}, fmt.Errorf("first line must be '# ADR-NNNN: Title', got %q", sc.Text())
	}
	r.Number, r.Title = h[1], h[2]

	// The front matter is the bullet list directly below the heading.
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			if len(r.FrontMatter) > 0 {
				break
			}
			continue
		}
		m := frontMatterRE.FindStringSubmatch(line)
		if m == nil {
			break
		}
		if _, dup := r.FrontMatter[m[1]]; dup {
			return Record{}, fmt.Errorf("front matter key %q appears twice", m[1])
		}
		r.FrontMatter[m[1]] = strings.TrimSpace(m[2])
	}
	return r, sc.Err()
}

// Records reads every record of the ADR directory, in the order of their
// file names, which is the order of their numbers.
func Records(dir string) ([]Record, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []Record
	for _, e := range entries {
		if e.IsDir() || !IsRecordFile(e.Name()) {
			continue
		}
		r, err := ParseRecord(dir, e.Name())
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].File < out[j].File })
	return out, nil
}

// Table is the index of the records as a Markdown table: number and link,
// title, status and implementation, as each record's heading and front
// matter say.
func Table(records []Record) string {
	var b strings.Builder
	b.WriteString("| No. | Title | Status | Implementation |\n|---|---|---|---|\n")
	for _, r := range records {
		fmt.Fprintf(&b, "| [%s](%s) | %s | %s | %s |\n", r.Number, r.File, cell(r.Title), cell(r.FrontMatter["Status"]), cell(r.FrontMatter["Implementation"]))
	}
	return b.String()
}

// cell keeps a value from ending its table cell early.
func cell(s string) string { return strings.ReplaceAll(s, "|", `\|`) }

// WithIndex returns readme with the text between IndexBegin and IndexEnd
// replaced by table.
func WithIndex(readme, table string) (string, error) {
	begin := strings.Index(readme, IndexBegin)
	end := strings.Index(readme, IndexEnd)
	if begin < 0 || end < 0 || end < begin {
		return "", fmt.Errorf("README.md has no %q … %q block", IndexBegin, IndexEnd)
	}
	if strings.Count(readme, IndexBegin) > 1 || strings.Count(readme, IndexEnd) > 1 {
		return "", fmt.Errorf("README.md has more than one index block")
	}
	return readme[:begin+len(IndexBegin)] + "\n\n" + table + "\n" + readme[end:], nil
}

// Generate returns the README.md of the ADR directory with its index made
// from the records, and whether it differs from the file as it is.
func Generate(dir string) (readme string, changed bool, err error) {
	records, err := Records(dir)
	if err != nil {
		return "", false, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		return "", false, err
	}
	readme, err = WithIndex(string(data), Table(records))
	if err != nil {
		return "", false, err
	}
	return readme, readme != string(data), nil
}
