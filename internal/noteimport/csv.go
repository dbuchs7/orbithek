package noteimport

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/dbuchs7/orbithek/internal/item"
)

// maxTable bounds a database's CSV file.
const maxTable = 8 << 20

// row is a row of a database: its columns besides the first, which is
// its title, with their values, and the row as read.
type row struct {
	columns []string
	values  map[string]any
	raw     string
}

// databases reads the databases of a Notion export: each CSV file is a
// page that lists its rows, each row an item of the rows' type below it,
// its columns mapped to fields by name. A row's page in the export gives
// the item's text; a row without one is written from the CSV alone.
func (p *planner) databases() {
	if p.format != FormatNotion {
		for _, f := range p.a.Paths() {
			if isCSV(f) {
				p.problem("%s: a CSV file is read as a database only in a Notion export; it is left out", f)
			}
		}
		return
	}
	for _, f := range p.a.Paths() {
		if !isCSV(f) {
			continue
		}
		name, id, ok := notionName(stem(f))
		if !ok {
			p.problem("%s: a CSV file without a page ID is left out", f)
			continue
		}
		// The CSV of every view ends in _all; the other is then left, and
		// links to it name the database too.
		dbID := "notion/" + id
		if !strings.HasSuffix(stem(f), "_all") && p.a.Has(strings.TrimSuffix(f, path.Ext(f))+"_all"+path.Ext(f)) {
			p.byPath[f] = dbID
			continue
		}
		data, err := p.a.Read(f, maxTable)
		if err != nil {
			p.problem("%s is left out: %v", f, err)
			continue
		}
		records, err := readCSV(data)
		if err != nil || len(records) == 0 || len(records[0]) == 0 {
			p.problem("%s is no table of rows and is left out: %v", f, err)
			continue
		}
		p.byPath[f] = dbID
		p.byNotion[id] = dbID
		db, made := p.pages[dbID]
		if !made {
			db = &Page{ID: dbID, Parent: p.folders[dirOf(f)], Type: p.opt.PageType.Name}
			p.add(db)
		}
		db.Title, db.Raw = name, "database:"+strings.Join(records[0], "\t")
		if p.notes[db.Path] == nil {
			db.Path = f
			db.Blocks = []item.Block{{Kind: item.Children}}
		}
		p.rowsOf(f, dbID, records)
	}
}

// rowsOf matches a database's rows with the pages of its folder by their
// titles and plans those without a page.
func (p *planner) rowsOf(f, dbID string, records [][]string) {
	header := records[0]
	folder := strings.TrimSuffix(strings.TrimSuffix(f, path.Ext(f)), "_all")
	pages := map[string][]string{}
	for _, n := range p.notesIn(folder) {
		key := strings.ToLower(oneLine(p.notes[n].name))
		pages[key] = append(pages[key], n)
		if h := firstHeading(p.notes[n].text); h != "" && !strings.EqualFold(h, p.notes[n].name) {
			pages[strings.ToLower(h)] = append(pages[strings.ToLower(h)], n)
		}
	}
	used := map[string]bool{}
	for i, rec := range records[1:] {
		title := oneLine(rec[0])
		if title == "" && len(slices.DeleteFunc(slices.Clone(rec), func(s string) bool { return strings.TrimSpace(s) == "" })) == 0 {
			continue
		}
		r := row{columns: header[1:], values: map[string]any{}, raw: strings.Join(rec, "\t")}
		for j, col := range header[1:] {
			if j+1 < len(rec) {
				r.values[col] = rec[j+1]
			}
		}
		var match string
		for _, n := range pages[strings.ToLower(title)] {
			if !used[n] {
				match = n
				break
			}
		}
		if match != "" {
			used[match] = true
			p.rows[match] = r
			continue
		}
		// A row without a page of its own.
		if title == "" {
			title = "—"
		}
		synthetic := fmt.Sprintf("%s#%d", f, i+2) // the row's line in the file
		p.notes[synthetic] = &note{path: synthetic, name: title}
		p.rows[synthetic] = r
		id := dbID + "/row/" + strings.ToLower(title)
		for n := 2; p.pages[id] != nil; n++ {
			id = fmt.Sprintf("%s/row/%s/%d", dbID, strings.ToLower(title), n)
		}
		p.add(&Page{ID: id, Path: synthetic, Parent: dbID, Type: p.opt.RowType.Name, Title: title})
	}
}

// readCSV reads a CSV file as a Notion export writes it, with or without
// a byte order mark.
func readCSV(data []byte) ([][]string, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord, r.LazyQuotes = -1, true
	return r.ReadAll()
}

var headingRE = regexp.MustCompile(`(?m)^# +(.+?)\s*$`)

// firstHeading is the text of a note's first heading of level 1.
func firstHeading(text string) string {
	if m := headingRE.FindStringSubmatch(text); m != nil {
		return oneLine(m[1])
	}
	return ""
}

// withoutProperties drops the lines "Column: value" a Notion export
// writes below a row's title, as the columns become fields or the table
// at the top of the page.
func withoutProperties(text string, columns []string) string {
	lines := strings.Split(text, "\n")
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i < len(lines) && strings.HasPrefix(lines[i], "# ") {
		i++
	}
	start := i
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	end := i
	for end < len(lines) {
		key, _, ok := strings.Cut(lines[end], ":")
		if !ok || !slices.Contains(columns, strings.TrimSpace(key)) {
			break
		}
		end++
	}
	if end == i {
		return text
	}
	return strings.Join(append(lines[:start:start], lines[end:]...), "\n")
}
