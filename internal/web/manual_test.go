package web

import (
	_ "embed"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// manualTerms is the list of bold text in the user manuals that is no
// label of the UI: keys, field contents, names of other products
// (ADR-0185). Each line is "<language> <term> — <reason>", "*" for every
// language; "#" begins a comment.
//
//go:embed manual_terms.txt
var manualTerms string

// boldRE finds the text a manual sets in bold. Bold never spans a line in
// the manuals, so a stray "**" cannot swallow a paragraph.
var boldRE = regexp.MustCompile(`\*\*([^*\n]+)\*\*`)

// pathSep separates the steps of a way through menus in one bold label,
// as in **Account and password → Connected engines**: each step is a label.
const pathSep = " → "

// normLabel is a label as the catalogue and the manual are compared: in
// lower case, without an ellipsis, trailing punctuation and surrounding
// space. The manual writes a label in a sentence ("the **logo**"), the UI
// at the start of a button ("Logo").
func normLabel(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	for {
		t := strings.TrimRight(s, " :.,;!?")
		t = strings.TrimSuffix(t, "…")
		t = strings.TrimSuffix(t, "...")
		t = strings.TrimSpace(t)
		if t == s {
			return s
		}
		s = t
	}
}

// labelSet holds the values of one catalogue, normalised by normLabel: the plain ones as a set,
// the ones with a placeholder as patterns in which the placeholder stands
// for any word.
type labelSet struct {
	plain    map[string]bool
	patterns []*regexp.Regexp
}

func (l labelSet) has(label string) bool {
	label = normLabel(label)
	if l.plain[label] {
		return true
	}
	for _, re := range l.patterns {
		if re.MatchString(label) {
			return true
		}
	}
	return false
}

// hintRE is a hint in brackets at the end of a catalogue value, such as the
// key in "Undo (Ctrl+Z)" or the count in "As a list ({n} labels)": the
// manual names the label without it.
// letterRE finds a letter of any script.
var letterRE = regexp.MustCompile(`\pL`)

var hintRE = regexp.MustCompile(`\s*\([^()]*\)$`)

func labelsOf(c map[string]any) labelSet {
	set := labelSet{plain: map[string]bool{}}
	var add func(v string)
	add = func(v string) {
		if short := hintRE.ReplaceAllString(v, ""); short != v && short != "" {
			add(short)
		}
		v = normLabel(v)
		if v == "" {
			return
		}
		if !placeholderRE.MatchString(v) {
			set.plain[v] = true
			return
		}
		if !letterRE.MatchString(placeholderRE.ReplaceAllString(v, "")) {
			// A value of placeholders only, such as "{n} %", names no label.
			return
		}
		var b strings.Builder
		b.WriteString("^")
		last := 0
		for _, loc := range placeholderRE.FindAllStringIndex(v, -1) {
			b.WriteString(regexp.QuoteMeta(v[last:loc[0]]))
			b.WriteString(`\S+`)
			last = loc[1]
		}
		b.WriteString(regexp.QuoteMeta(v[last:]))
		b.WriteString("$")
		set.patterns = append(set.patterns, regexp.MustCompile(b.String()))
	}
	for key, v := range c {
		if strings.HasPrefix(key, "$") {
			continue
		}
		switch m := v.(type) {
		case string:
			add(m)
		case map[string]any:
			for _, form := range m {
				if s, ok := form.(string); ok {
					add(s)
				}
			}
		}
	}
	return set
}

// parseManualTerms reads manual_terms.txt into the terms allowed per
// language; "*" holds the ones for every language. A line without a reason
// is an error: the list says why each term is no label.
func parseManualTerms(text string) (map[string]map[string]bool, []string) {
	terms := map[string]map[string]bool{}
	var problems []string
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lang, rest, ok := strings.Cut(line, " ")
		term, reason, hasReason := strings.Cut(rest, " — ")
		term = strings.TrimSpace(term)
		if !ok || term == "" || !hasReason || strings.TrimSpace(reason) == "" {
			problems = append(problems, fmt.Sprintf("manual_terms.txt:%d: want \"<language> <term> — <reason>\", got %q", i+1, line))
			continue
		}
		if terms[lang] == nil {
			terms[lang] = map[string]bool{}
		}
		terms[lang][normLabel(term)] = true
	}
	return terms, problems
}

// manualLabels lists the labels a manual names in bold, as it writes them,
// each step of a way through menus on its own, in the order they first
// appear; a label written twice, in any case, is listed once.
func manualLabels(text string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range boldRE.FindAllStringSubmatch(text, -1) {
		for _, part := range strings.Split(m[1], pathSep) {
			if n := normLabel(part); n != "" && !seen[n] {
				seen[n] = true
				out = append(out, strings.TrimSpace(part))
			}
		}
	}
	return out
}

// TestManualNamesExistingLabels is Check A of ADR-0185: every label a
// user manual names in bold is a value of the catalogue of its language,
// or a term listed in manual_terms.txt. A label renamed or removed in the
// catalogue fails here until the manual follows.
func TestManualNamesExistingLabels(t *testing.T) {
	terms, problems := parseManualTerms(manualTerms)
	for _, p := range problems {
		t.Error(p)
	}
	for _, lang := range Languages {
		data, err := fs.ReadFile(FS(), "assets/manual/"+lang+".md")
		if err != nil {
			t.Errorf("no manual in %s: %v", lang, err)
			continue
		}
		labels := labelsOf(catalogue(t, lang))
		var missing []string
		for _, l := range manualLabels(string(data)) {
			if n := normLabel(l); labels.has(l) || terms[lang][n] || terms["*"][n] {
				continue
			}
			missing = append(missing, l)
		}
		for _, l := range missing {
			t.Errorf("manual/%s.md names **%s**, which is no value of locales/%s.js: rename it in the manual as the UI now calls it, or, if it is no label, list it in internal/web/manual_terms.txt with a reason", lang, l, lang)
		}
	}
}

// TestManualTermsAreUsed: a term in manual_terms.txt that no manual sets in
// bold any more, or that has become a label of the catalogue, is removed
// from the list, so the list stays the exceptions it claims to be.
func TestManualTermsAreUsed(t *testing.T) {
	terms, _ := parseManualTerms(manualTerms)
	used := map[string]map[string]bool{"*": {}}
	for _, lang := range Languages {
		data, err := fs.ReadFile(FS(), "assets/manual/"+lang+".md")
		if err != nil {
			t.Fatal(err)
		}
		labels := labelsOf(catalogue(t, lang))
		used[lang] = map[string]bool{}
		for _, l := range manualLabels(string(data)) {
			n := normLabel(l)
			if labels.has(l) {
				if terms[lang][n] || terms["*"][n] {
					t.Errorf("manual_terms.txt lists %q for %s, but it is a label of locales/%s.js: remove the line", l, lang, lang)
				}
				continue
			}
			used[lang][n] = true
			used["*"][n] = true
		}
	}
	for lang, set := range terms {
		if lang != "*" && !slices.Contains(Languages, lang) {
			t.Errorf("manual_terms.txt names the language %q, which the UI has not", lang)
			continue
		}
		for term := range set {
			if !used[lang][term] {
				t.Errorf("manual_terms.txt lists %q for %s, which no manual sets in bold as a term: remove the line", term, lang)
			}
		}
	}
}

func TestNormLabel(t *testing.T) {
	for in, want := range map[string]string{
		"Complete…":      "complete",
		"Insert...":      "insert",
		" Rename: ":      "rename",
		"Done.":          "done",
		"Embed an item…": "embed an item",
		"Ask?":           "ask",
	} {
		if got := normLabel(in); got != want {
			t.Errorf("normLabel(%q) = %q, want %q", in, got, want)
		}
	}
	set := labelsOf(map[string]any{
		"a": "Present {n} pictures", "b": map[string]any{"one": "{n} file"}, "c": "Undo (Ctrl+Z)",
		"d": "As a list ({n} labels)", "e": "{n} %", "f": "{n} / {total}", "$name": "English",
	})
	for _, l := range []string{"Present N pictures", "1 file", "Undo", "undo (ctrl+z)", "As a list"} {
		if !set.has(l) {
			t.Errorf("%q is not found", l)
		}
	}
	for _, l := range []string{"Present pictures", "English", "Ctrl+Z", "Present two pictures now", "bell"} {
		if set.has(l) {
			t.Errorf("%q is found", l)
		}
	}
	if got := manualLabels("**A → B…** and **a** and\n**not\nbold**"); !slices.Equal(got, []string{"A", "B…"}) {
		t.Errorf("manualLabels = %q", got)
	}
	if _, p := parseManualTerms("en L — a key\nen Enter\n\n# comment\n* Excel — a product"); len(p) != 1 {
		t.Errorf("a line without a reason is not reported: %v", p)
	}
}
