package noteimport

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/dbuchs7/orbithek/internal/item"
)

// words are what the import writes itself, by language; English is the
// source.
type words struct{ property, value string }

var allWords = map[string]words{
	"en": {property: "Property", value: "Value"},
	"de": {property: "Eigenschaft", value: "Wert"},
}

func wordsIn(lang string) words {
	if w, ok := allWords[lang]; ok {
		return w
	}
	return allWords["en"]
}

// frontMatter cuts a YAML front matter, a block between lines of "---" at
// the very start, off a note and reads it: its values by key, the keys
// in their order, and the note's text without it. A front matter that is
// no YAML mapping is cut off all the same and reported.
func frontMatter(text string, fail func(error)) (map[string]any, []string, string) {
	rest, ok := strings.CutPrefix(text, "---\n")
	if !ok {
		return nil, nil, text
	}
	var block []string
	for n := 0; n < 1000; n++ {
		line, after, found := strings.Cut(rest, "\n")
		if l := strings.TrimRight(line, " "); l == "---" || l == "..." {
			return parseFront(strings.Join(block, "\n"), fail), keysOf(strings.Join(block, "\n")), after
		}
		block = append(block, line)
		if !found {
			break
		}
		rest = after
	}
	return nil, nil, text
}

func parseFront(src string, fail func(error)) map[string]any {
	var m map[string]any
	if err := yaml.Unmarshal([]byte(src), &m); err != nil {
		fail(err)
		return nil
	}
	return m
}

// keysOf lists the keys of a YAML mapping in the order written.
func keysOf(src string) []string {
	var doc yaml.Node
	if yaml.Unmarshal([]byte(src), &doc) != nil || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil
	}
	var out []string
	for i := 0; i+1 < len(doc.Content[0].Content); i += 2 {
		out = append(out, doc.Content[0].Content[i].Value)
	}
	return out
}

// list is a value as a list of texts: a list's items, or one text.
func list(v any) []string {
	switch v := v.(type) {
	case nil:
		return nil
	case []any:
		var out []string
		for _, x := range v {
			if s := strings.TrimSpace(text(x)); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		if s := strings.TrimSpace(text(v)); s != "" {
			return []string{s}
		}
	}
	return nil
}

// tagsOf are the tags a value names: a list's items, or a text's parts
// split at commas and, unless the text comes from a table, at spaces.
func tagsOf(v any, table bool) []string {
	var out []string
	for _, s := range list(v) {
		out = append(out, strings.FieldsFunc(s, func(r rune) bool { return r == ',' || !table && (r == ' ' || r == '\t') })...)
	}
	return out
}

// text writes a value as text: lists joined by commas, days as dates,
// anything else as JSON.
func text(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case string:
		return v
	case time.Time:
		if v.Hour() == 0 && v.Minute() == 0 && v.Second() == 0 {
			return v.Format(time.DateOnly)
		}
		return v.Format(time.RFC3339)
	case bool, int, int64, uint64, float64:
		return fmt.Sprint(v)
	case []any:
		parts := make([]string, 0, len(v))
		for _, x := range v {
			parts = append(parts, text(x))
		}
		return strings.Join(parts, ", ")
	}
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(data)
}

// dateLayouts are how notes write days: ISO 8601, and as a Notion export
// writes them.
var dateLayouts = []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02T15:04", time.DateOnly, "2006/01/02", "02.01.2006",
	"January 2, 2006 3:04 PM", "January 2, 2006 15:04", "January 2, 2006"}

func parseTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	// A span, "January 2, 2026 → January 5, 2026", starts on its first day.
	if a, _, ok := strings.Cut(s, " → "); ok {
		s = a
	}
	for _, l := range dateLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// fieldValue converts a value of a note to one of a field's kind; an
// empty value is none.
func fieldValue(f item.FieldDef, v any) (any, error) {
	s := strings.TrimSpace(text(v))
	if s == "" {
		return nil, nil
	}
	switch f.Kind {
	case item.KindText:
		return s, nil
	case item.KindNumber:
		var n float64
		switch v := v.(type) {
		case int:
			n = float64(v)
		case float64:
			n = v
		default:
			var err error
			if n, err = strconv.ParseFloat(strings.ReplaceAll(s, "'", ""), 64); err != nil {
				return nil, fmt.Errorf("%q is not a number", s)
			}
		}
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return nil, fmt.Errorf("%q is not a number", s)
		}
		return n, nil
	case item.KindBool:
		if b, ok := v.(bool); ok {
			return b, nil
		}
		switch strings.ToLower(s) {
		case "true", "yes", "ja", "1", "x", "✓":
			return true, nil
		case "false", "no", "nein", "0":
			return false, nil
		}
		return nil, fmt.Errorf("%q is neither yes nor no", s)
	case item.KindDate:
		t, ok := v.(time.Time)
		if !ok {
			if t, ok = parseTime(s); !ok {
				return nil, fmt.Errorf("%q is not a date", s)
			}
		}
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
	case item.KindDateTime:
		t, ok := v.(time.Time)
		if !ok {
			var err error
			if t, err = time.Parse(time.RFC3339, s); err != nil {
				return nil, fmt.Errorf("%q is not a point in time with an offset", s)
			}
		}
		return t.UTC().Truncate(time.Second), nil
	case item.KindEnum:
		if i := slices.IndexFunc(f.Values, func(x string) bool { return strings.EqualFold(x, s) }); i >= 0 {
			return f.Values[i], nil
		}
		return nil, fmt.Errorf("%q is not one of its values", s)
	case item.KindURL:
		if (strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://")) && item.SafeHref(s) {
			return s, nil
		}
		return nil, fmt.Errorf("%q is not a web address", s)
	}
	return nil, fmt.Errorf("a %s field is not filled by an import", f.Kind)
}

// fieldOf finds the field a key names: by its name, or by its name in
// any case.
func fieldOf(td item.TypeDef, key string) (item.FieldDef, bool) {
	for _, f := range td.Fields {
		if f.Name == key {
			return f, true
		}
	}
	for _, f := range td.Fields {
		if strings.EqualFold(f.Name, key) {
			return f, true
		}
	}
	return item.FieldDef{}, false
}

// properties takes a note's front matter, or a database row's columns,
// onto its page: tags become labels, a status the type's workflow has
// the state, a key that names a field of a fitting kind that field, and
// the rest a table at the top of the page.
func (p *planner) properties(pg *Page, td item.TypeDef, values map[string]any, order []string, where string, table bool) item.Block {
	var rows []string
	for _, k := range order {
		v, ok := values[k]
		if !ok {
			continue
		}
		lk := strings.ToLower(strings.TrimSpace(k))
		switch {
		case lk == "title" || lk == "aliases" || lk == "alias" || lk == "cssclasses" || lk == "cssclass":
			continue
		case lk == "tags" || lk == "tag":
			for _, tag := range tagsOf(v, table) {
				p.label(pg, tag, where)
			}
			continue
		case (lk == "status" || lk == "state") && td.Workflow != nil:
			s := strings.TrimSpace(text(v))
			if i := slices.IndexFunc(td.Workflow.States, func(x string) bool { return strings.EqualFold(x, s) }); i >= 0 {
				pg.State = td.Workflow.States[i]
				continue
			}
		}
		if f, ok := fieldOf(td, k); ok {
			fv, err := fieldValue(f, v)
			if err == nil {
				if fv != nil {
					if pg.Props == nil {
						pg.Props = item.Props{}
					}
					pg.Props[f.Name] = fv
				}
				continue
			}
			p.once("value:"+td.Name+":"+f.Name, "%s: the value of %s does not fit the field %s of %s (%v); it is kept in the table at the top of the page", where, k, f.Name, td.Name, err)
		} else if table {
			p.once("column:"+td.Name+":"+k, "%s: the column %s has no field in %s; it is kept in the table at the top of each page", where, k, td.Name)
		}
		if s := oneLine(text(v)); s != "" {
			rows = append(rows, oneLine(strings.ReplaceAll(k, "\t", " "))+"\t"+strings.ReplaceAll(s, "\t", " "))
		}
	}
	if len(rows) == 0 {
		return item.Block{}
	}
	return item.Block{Kind: item.Table, Text: p.words.property + "\t" + p.words.value + "\n" + strings.Join(rows, "\n")}
}

// once reports a problem once per key.
func (p *planner) once(key, format string, args ...any) {
	if p.told == nil {
		p.told = map[string]bool{}
	}
	if !p.told[key] {
		p.told[key] = true
		p.problem(format, args...)
	}
}
