package adr

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

var (
	supersededRE = regexp.MustCompile(`^Superseded by ADR-(\d{4})$`)
	monthRE      = regexp.MustCompile(`^\d{4}-\d{2}$`)
)

var validImplementations = map[string]bool{
	"Not started": true,
	"Partial":     true,
	"Landed":      true,
	"Superseded":  true,
}

// questionMaxAge is how long an open question may go unchecked. A record
// that rests on an unanswered question must be looked at again at least
// once a year, so the gap cannot quietly become settled truth.
const questionMaxAge = 12 * 30 * 24 * time.Hour

func loadRecords(t *testing.T) map[string]Record {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading ADR directory: %v", err)
	}
	records := make(map[string]Record)
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") || name == "README.md" || name == "template.md" {
			continue
		}
		m := fileNameRE.FindStringSubmatch(name)
		if m == nil {
			t.Errorf("%s: file name must match NNNN-lowercase-slug.md", name)
			continue
		}
		r, err := ParseRecord(".", name)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if r.Number != m[1] {
			t.Errorf("%s: heading says ADR-%s, file name says %s", name, r.Number, m[1])
		}
		if prev, ok := records[m[1]]; ok {
			t.Errorf("ADR number %s used twice: %s and %s", m[1], prev.File, name)
			continue
		}
		records[m[1]] = r
	}
	return records
}

func TestRecordsAreNumberedWithoutGaps(t *testing.T) {
	records := loadRecords(t)
	if len(records) == 0 {
		t.Fatal("no ADRs found")
	}
	numbers := make([]string, 0, len(records))
	for n := range records {
		numbers = append(numbers, n)
	}
	sort.Strings(numbers)
	for i, n := range numbers {
		if want := fmt.Sprintf("%04d", i+1); n != want {
			t.Errorf("ADR numbers must run 0001, 0002, … without gaps: expected %s, found %s", want, n)
			break
		}
	}
}

func TestFrontMatter(t *testing.T) {
	records := loadRecords(t)
	now := time.Now()
	for _, r := range records {
		fm := r.FrontMatter
		for _, key := range []string{"Status", "Implementation", "Date", "Deciders"} {
			if fm[key] == "" {
				t.Errorf("%s: front matter needs a non-empty %q line", r.File, key)
			}
		}

		status := fm["Status"]
		switch status {
		case "Proposed", "Accepted", "Deprecated", "":
		default:
			m := supersededRE.FindStringSubmatch(status)
			if m == nil {
				t.Errorf("%s: Status %q is not one of Proposed, Accepted, Deprecated, Superseded by ADR-NNNN", r.File, status)
			} else if _, ok := records[m[1]]; !ok {
				t.Errorf("%s: superseded by ADR-%s, which does not exist", r.File, m[1])
			}
		}

		impl := fm["Implementation"]
		if impl != "" && !validImplementations[impl] {
			t.Errorf("%s: Implementation %q is not one of Not started, Partial, Landed, Superseded", r.File, impl)
		}
		if impl == "Landed" && status != "Accepted" {
			t.Errorf("%s: Implementation Landed requires Status Accepted, got %q", r.File, status)
		}

		if d := fm["Date"]; d != "" {
			if _, err := time.Parse("2006-01-02", d); err != nil {
				t.Errorf("%s: Date %q is not YYYY-MM-DD", r.File, d)
			}
		}

		q, hasQuestion := fm["Open question"]
		checked, hasChecked := fm["Question checked"]
		switch {
		case hasQuestion && q == "":
			t.Errorf("%s: Open question is empty; delete the line when nothing is open", r.File)
		case hasQuestion && !hasChecked:
			t.Errorf("%s: an Open question needs a Question checked month (YYYY-MM)", r.File)
		case !hasQuestion && hasChecked:
			t.Errorf("%s: Question checked without an Open question", r.File)
		case hasChecked:
			if !monthRE.MatchString(checked) {
				t.Errorf("%s: Question checked %q is not YYYY-MM", r.File, checked)
				break
			}
			when, err := time.Parse("2006-01", checked)
			if err != nil {
				t.Errorf("%s: Question checked %q: %v", r.File, checked, err)
				break
			}
			if when.After(now) {
				t.Errorf("%s: Question checked %s lies in the future", r.File, checked)
			} else if now.Sub(when) > questionMaxAge {
				t.Errorf("%s: the open question was last checked %s; look at it again and update Question checked", r.File, checked)
			}
		}
	}
}

// TestIndexIsGenerated: the index in README.md is the one the records'
// headings and front matter make, so a record and its row cannot drift
// apart, and a conflict in the index is resolved by generating it again.
func TestIndexIsGenerated(t *testing.T) {
	loadRecords(t) // reports malformed records by name
	_, changed, err := Generate(".")
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("README.md: the index is not the one the records make; run `go generate ./docs/adr` and commit README.md")
	}
}
