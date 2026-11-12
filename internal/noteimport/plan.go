package noteimport

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/dbuchs7/orbithek/internal/item"
	"github.com/dbuchs7/orbithek/internal/markdown"
)

// The formats of an archive.
const (
	// FormatFolder is a folder of Markdown files: an Obsidian vault, or
	// the Markdown export of any notes application that writes Markdown
	// files with front matter.
	FormatFolder = "folder"
	// FormatNotion is a Notion workspace export of Markdown and CSV, whose
	// file and folder names end in the 32 hexadecimal digits of the page's
	// ID.
	FormatNotion = "notion"
)

// maxNote bounds a Markdown or CSV file a page is written from, as for a
// file of a repository (ADR-0138).
const maxNote = 2 << 20

// Options say what an archive's pages become.
type Options struct {
	// PageType is the type of the notes' pages; it must have a text.
	PageType item.TypeDef
	// RowType is the type of the rows of a database (CSV); empty is
	// PageType.
	RowType item.TypeDef
	// Lang is the language of the words the import writes itself, such as
	// the head of the properties table.
	Lang string
}

// Page is one page the archive makes.
type Page struct {
	// ID identifies the page in the archive and in its item's origin:
	// "md/<path>" for a note, "dir/<path>" for a folder, "notion/<id>"
	// for a page of a Notion export.
	ID string
	// Path is the file or folder in the archive it is written from.
	Path string
	// Parent is the ID of the page that holds it; empty is the top.
	Parent string
	Type   string
	Title  string
	// Raw is the source the page is written from, kept in its origin, so
	// a re-run writes the page only when its source changed.
	Raw    string
	Blocks []item.Block
	Labels []string
	Props  item.Props
	State  string
	// Pictures are files of the archive the page shows: each replaces the
	// block at its index with an image once it is attached.
	Pictures []Picture
}

// Picture is a file of the archive a page shows.
type Picture struct {
	Block   int
	File    string
	Caption string
}

// Plan is what an archive makes: its pages, parents before children, and
// what cannot be converted.
type Plan struct {
	Format   string
	Root     string
	Pages    []Page
	Problems []string
}

// noteHost and fileHost head the addresses the conversion gives the links
// to notes and the pictures of the archive until the pages exist: the
// reserved domain invalid never names a real site (RFC 2606).
const (
	noteHost = "https://note.invalid/"
	fileHost = "https://file.invalid/"
)

// NoteLink is the address a link to the page id has until the page exists.
func NoteLink(id string) string { return noteHost + url.PathEscape(id) }

// LinkedNote is the ID of the page an address of NoteLink names.
func LinkedNote(href string) (string, bool) {
	rest, ok := strings.CutPrefix(href, noteHost)
	if !ok {
		return "", false
	}
	id, err := url.PathUnescape(rest)
	return id, err == nil
}

// notionIDRE is the ID a Notion export appends to a file's or folder's
// name; a database's CSV of every view ends in _all.
var notionIDRE = regexp.MustCompile(`^(.*?) ?([0-9a-f]{32})(_all)?$`)

// notionName splits a name without its extension into the name people
// gave it and its ID.
func notionName(base string) (string, string, bool) {
	m := notionIDRE.FindStringSubmatch(base)
	if m == nil {
		return base, "", false
	}
	return strings.TrimSpace(m[1]), m[2], true
}

func isNote(p string) bool {
	ext := strings.ToLower(path.Ext(p))
	return ext == ".md" || ext == ".markdown"
}

func isCSV(p string) bool { return strings.EqualFold(path.Ext(p), ".csv") }

func stem(p string) string { return strings.TrimSuffix(path.Base(p), path.Ext(p)) }

func dirOf(p string) string {
	if d := path.Dir(p); d != "." {
		return d
	}
	return ""
}

// planner builds a plan.
type planner struct {
	a      *Archive
	opt    Options
	words  words
	format string
	plan   Plan
	// folders map a folder of the archive to the ID of the page that
	// holds what is in it.
	folders map[string]string
	// byPath maps a file of the archive to the ID of its page; byName
	// maps a note's name, lower case and without extension, and its
	// aliases to the paths of the notes so named.
	byPath map[string]string
	byName map[string][]string
	// byNotion maps a Notion ID to its page's ID.
	byNotion map[string]string
	notes    map[string]*note
	index    map[string]string // folder → its index note
	rows     map[string]row    // note → the database row it is
	pages    map[string]*Page
	// told are the problems reported once per import.
	told map[string]bool
}

// note is a Markdown file read, with its front matter.
type note struct {
	path string
	name string
	// raw is the file as read, text the note without its front matter.
	raw   string
	text  string
	front map[string]any
	order []string
}

// Build reads an archive's notes and plans their pages.
func Build(a *Archive, opt Options) Plan {
	if opt.RowType.Name == "" {
		opt.RowType = opt.PageType
	}
	p := &planner{a: a, opt: opt, words: wordsIn(opt.Lang), folders: map[string]string{}, byPath: map[string]string{},
		byName: map[string][]string{}, byNotion: map[string]string{}, notes: map[string]*note{}, index: map[string]string{},
		rows: map[string]row{}, pages: map[string]*Page{}}
	p.plan.Root = a.Root
	p.plan.Problems = append(p.plan.Problems, a.Skipped...)
	p.format = FormatFolder
	for _, f := range a.Paths() {
		if _, _, ok := notionName(stem(f)); ok && (isNote(f) || isCSV(f)) {
			p.format = FormatNotion
			break
		}
	}
	p.plan.Format = p.format
	p.read()
	p.tree()
	p.databases()
	p.convert()
	return p.plan
}

func (p *planner) problem(format string, args ...any) {
	p.plan.Problems = append(p.plan.Problems, fmt.Sprintf(format, args...))
}

// read reads every note with its front matter, so links can name a note
// by an alias before the note is converted.
func (p *planner) read() {
	for _, f := range p.a.Paths() {
		if !isNote(f) {
			continue
		}
		data, err := p.a.Read(f, maxNote)
		if err != nil {
			p.problem("%s is left out: %v", f, err)
			continue
		}
		n := &note{path: f, raw: strings.ReplaceAll(strings.ToValidUTF8(string(data), "�"), "\r\n", "\n")}
		n.name = stem(f)
		if p.format == FormatNotion {
			n.name, _, _ = notionName(n.name)
		}
		n.front, n.order, n.text = frontMatter(n.raw, func(err error) { p.problem("%s: its front matter is not read: %v", f, err) })
		p.notes[f] = n
		key := strings.ToLower(n.name)
		p.byName[key] = append(p.byName[key], f)
		for _, alias := range append(list(n.front["aliases"]), list(n.front["alias"])...) {
			key := strings.ToLower(alias)
			if !slices.Contains(p.byName[key], f) {
				p.byName[key] = append(p.byName[key], f)
			}
		}
	}
}

// indexNames are the notes whose text is their folder's page, besides
// one named as the folder: the hosts show README.md, generators of
// documentation sites read index.md and _index.md.
var indexNames = []string{"index", "_index", "readme"}

// tree gives every note and every folder that holds notes its page.
func (p *planner) tree() {
	// The folders that hold notes or databases, with those above them.
	needed := map[string]bool{}
	for _, f := range p.a.Paths() {
		if _, ok := p.notes[f]; !ok && !(p.format == FormatNotion && isCSV(f)) {
			continue
		}
		for d := dirOf(f); d != "" && !needed[d]; d = dirOf(d) {
			needed[d] = true
		}
	}
	dirs := slices.Sorted(func(yield func(string) bool) {
		for d := range needed {
			if !yield(d) {
				return
			}
		}
	})
	// A folder's page: in a Notion export the page whose file has the
	// folder's name, else a page of its own, its text an index note.
	for _, d := range dirs {
		base := path.Base(d)
		if p.format == FormatNotion {
			if _, id, ok := notionName(base); ok {
				p.folders[d] = "notion/" + id
				continue
			}
		}
		p.folders[d] = "dir/" + d
		for _, f := range p.notesIn(d) {
			n := strings.ToLower(stem(f))
			if slices.Contains(indexNames, n) || n == strings.ToLower(base) {
				if old, ok := p.index[d]; !ok || indexRank(old, base) > indexRank(f, base) {
					p.index[d] = f
				}
			}
		}
	}
	for _, d := range dirs {
		id := p.folders[d]
		if _, made := p.pages[id]; made {
			continue
		}
		pg := &Page{ID: id, Path: d, Parent: p.folders[dirOf(d)], Type: p.opt.PageType.Name, Title: path.Base(d)}
		if p.format == FormatNotion {
			pg.Title, _, _ = notionName(pg.Title)
		}
		if f, ok := p.index[d]; ok {
			p.byPath[f] = id
		}
		p.add(pg)
	}
	for _, f := range p.a.Paths() {
		n, ok := p.notes[f]
		if !ok || p.index[dirOf(f)] == f {
			continue
		}
		id := "md/" + f
		if p.format == FormatNotion {
			if _, nid, ok := notionName(stem(f)); ok {
				id = "notion/" + nid
			}
		}
		p.byPath[f] = id
		if pg, made := p.pages[id]; made {
			// The page of a Notion folder: its file gives its text.
			pg.Path, pg.Title = f, n.name
			continue
		}
		p.add(&Page{ID: id, Path: f, Parent: p.folders[dirOf(f)], Type: p.opt.PageType.Name, Title: n.name})
	}
	for _, id := range p.byPath {
		if strings.HasPrefix(id, "notion/") {
			p.byNotion[strings.TrimPrefix(id, "notion/")] = id
		}
	}
	for _, id := range p.folders {
		if strings.HasPrefix(id, "notion/") {
			p.byNotion[strings.TrimPrefix(id, "notion/")] = id
		}
	}
}

// indexRank orders the index notes of a folder: the note named as the
// folder first, then index, _index and README.
func indexRank(f, folder string) int {
	n := strings.ToLower(stem(f))
	if n == strings.ToLower(folder) {
		return 0
	}
	return 1 + slices.Index(indexNames, n)
}

// notesIn are the notes right in a folder.
func (p *planner) notesIn(d string) []string {
	var out []string
	for f := range p.notes {
		if dirOf(f) == d {
			out = append(out, f)
		}
	}
	slices.Sort(out)
	return out
}

// add plans a page; parents come before their children, as the archive's
// paths are sorted and a folder's path sorts before what is in it.
func (p *planner) add(pg *Page) {
	p.pages[pg.ID] = pg
	p.plan.Pages = append(p.plan.Pages, Page{ID: pg.ID})
}

// convert writes every planned page.
func (p *planner) convert() {
	out := make([]Page, 0, len(p.plan.Pages))
	for _, ref := range p.plan.Pages {
		pg := p.pages[ref.ID]
		switch {
		case p.index[pg.Path] != "":
			p.fill(pg, p.notes[p.index[pg.Path]], true)
		case p.notes[pg.Path] != nil:
			p.fill(pg, p.notes[pg.Path], false)
		case pg.Raw == "" && len(pg.Blocks) == 0:
			// A folder without a note of its own lists its pages.
			pg.Blocks = []item.Block{{Kind: item.Children}}
			pg.Raw = "folder:" + pg.Path
		}
		if pg.Title = oneLine(pg.Title); pg.Title == "" {
			pg.Title = path.Base(pg.Path)
		}
		slices.Sort(pg.Labels)
		pg.Labels = slices.Compact(pg.Labels)
		out = append(out, *pg)
	}
	p.plan.Pages = out
}

// fill writes a page from a note: its title, text, labels, fields and
// pictures. A folder's page keeps the list of its pages below its text.
func (p *planner) fill(pg *Page, n *note, folder bool) {
	r, isRow := p.rows[n.path]
	td := p.opt.PageType
	if isRow {
		td = p.opt.RowType
		pg.Type = td.Name
	}
	text := n.text
	if isRow {
		text = withoutProperties(text, r.columns)
	}
	prep := prepare(text)
	src := markdown.Source{Name: n.name + ".md", Link: p.linkFrom(n, pg.ID), Picture: p.pictureFrom(n)}
	d := markdown.Convert([]byte(prep.text), src)
	pg.Title = d.Title
	if t, ok := n.front["title"].(string); ok && strings.TrimSpace(t) != "" {
		pg.Title = t
	}
	pg.Raw = n.raw
	for _, e := range prep.embeds {
		p.problem("%s: the embedded file %s is not shown; it stays its name", n.path, e)
	}
	for _, e := range prep.notes {
		p.problem("%s: the embedded note %s becomes a mention of its page", n.path, e)
	}
	// Front matter and tags: labels, fields, the state, and a table of
	// what fits no field.
	values := n.front
	order := n.order
	if isRow {
		values, order = r.values, r.columns
		pg.Raw += "\n" + r.raw
	}
	table := p.properties(pg, td, values, order, n.path, isRow)
	for _, tag := range prep.tags {
		p.label(pg, tag, n.path)
	}
	var blocks []item.Block
	if table.Text != "" {
		blocks = append(blocks, table)
	}
	shift := len(blocks)
	blocks = append(blocks, d.Blocks...)
	if folder {
		blocks = append(blocks, item.Block{Kind: item.Children})
	}
	pg.Blocks = blocks
	for _, pic := range d.Pictures {
		if f, ok := strings.CutPrefix(pic.Src, fileHost); ok {
			if file, err := url.PathUnescape(f); err == nil {
				pg.Pictures = append(pg.Pictures, Picture{Block: pic.Block + shift, File: file, Caption: oneLine(pic.Alt)})
			}
		}
	}
}

// label adds a tag as a label; a tag's levels, a/b, are joined by dots.
func (p *planner) label(pg *Page, tag, where string) {
	tag = strings.TrimPrefix(strings.TrimSpace(tag), "#")
	if tag == "" {
		return
	}
	l, ok := item.NormalLabel(strings.ReplaceAll(tag, "/", "."))
	if !ok {
		p.problem("%s: the tag %q is no label: letters and digits, joined by -, _ or ., up to 64 characters", where, tag)
		return
	}
	pg.Labels = append(pg.Labels, l)
}

// target finds the note a link names: a path relative to the note, a
// path in the archive, or a name or alias, nearest first.
func (p *planner) target(from *note, name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", false
	}
	candidates := []string{path.Join(dirOf(from.path), name), name}
	if !isNote(name) {
		candidates = append(candidates, path.Join(dirOf(from.path), name+".md"), name+".md")
	}
	for _, c := range candidates {
		if _, ok := p.byPath[c]; ok {
			return c, true
		}
	}
	key := strings.ToLower(strings.TrimSuffix(path.Base(name), path.Ext(name)))
	if !isNote(name) {
		key = strings.ToLower(path.Base(name))
	}
	found := p.byName[key]
	if len(found) == 0 {
		return "", false
	}
	// Of several notes so named, the one whose path ends as the link's
	// does, then the one in the same folder, then the shortest path.
	best := slices.Clone(found)
	slices.SortFunc(best, func(x, y string) int {
		rank := func(f string) int {
			switch {
			case strings.Contains(name, "/") && strings.HasSuffix(strings.TrimSuffix(f, path.Ext(f)), strings.TrimSuffix(name, path.Ext(name))):
				return 0
			case dirOf(f) == dirOf(from.path):
				return 1
			}
			return 2
		}
		if rx, ry := rank(x), rank(y); rx != ry {
			return rx - ry
		}
		if len(x) != len(y) {
			return len(x) - len(y)
		}
		return strings.Compare(x, y)
	})
	if _, ok := p.byPath[best[0]]; !ok {
		return "", false
	}
	return best[0], true
}

// file finds a file of the archive a note names: relative to the note,
// in the archive, or by its name anywhere, nearest first.
func (p *planner) file(from *note, name string) (string, bool) {
	for _, c := range []string{path.Join(dirOf(from.path), name), path.Clean(name)} {
		if c, ok := clean(c); ok && p.a.Has(c) {
			return c, true
		}
	}
	base := path.Base(name)
	var found []string
	for _, f := range p.a.Paths() {
		if path.Base(f) == base {
			found = append(found, f)
		}
	}
	if len(found) == 0 {
		return "", false
	}
	slices.SortFunc(found, func(x, y string) int {
		if (dirOf(x) == dirOf(from.path)) != (dirOf(y) == dirOf(from.path)) {
			if dirOf(x) == dirOf(from.path) {
				return -1
			}
			return 1
		}
		return len(x) - len(y)
	})
	return found[0], true
}

// linkFrom resolves the links of a note: a wiki link or a relative link
// to another note becomes the address of its page, which a run turns
// into a mention (D2); a link to a page of a Notion export too; web
// addresses stay.
func (p *planner) linkFrom(n *note, self string) func(string) string {
	return func(target string) string {
		if t, ok := strings.CutPrefix(target, wikiScheme); ok {
			name, _, _ := strings.Cut(t, "#")
			if name == "" {
				return ""
			}
			f, ok := p.target(n, name)
			if !ok {
				p.problem("%s: [[%s]] names no note of the archive; it stays text", n.path, name)
				return ""
			}
			return p.noteLink(f, self)
		}
		u, err := url.Parse(target)
		if err != nil || target == "" || strings.HasPrefix(target, "#") {
			return ""
		}
		if u.Scheme != "" || u.Host != "" {
			if id := p.notionLink(u); id != "" {
				return NoteLink(id)
			}
			if item.SafeHref(target) {
				return target
			}
			return ""
		}
		name, err := url.PathUnescape(u.Path)
		if err != nil {
			name = u.Path
		}
		if f, ok := p.target(n, name); ok {
			return p.noteLink(f, self)
		}
		if f, ok := p.file(n, name); ok {
			p.problem("%s: the link to the file %s is not imported; it stays text", n.path, f)
			return ""
		}
		p.problem("%s: the link to %s names nothing in the archive; it stays text", n.path, name)
		return ""
	}
}

// noteLink is the address of a note's page, or nothing for the page
// itself.
func (p *planner) noteLink(f, self string) string {
	id := p.byPath[f]
	if id == self {
		return ""
	}
	return NoteLink(id)
}

// notionLink is the page a link into a Notion workspace names, when the
// export holds it: its address ends in the page's ID.
func (p *planner) notionLink(u *url.URL) string {
	if p.format != FormatNotion || !(strings.HasSuffix(u.Hostname(), "notion.so") || strings.HasSuffix(u.Hostname(), "notion.site")) {
		return ""
	}
	last := path.Base(u.Path)
	if len(last) < 32 {
		return ""
	}
	return p.byNotion[last[len(last)-32:]]
}

// pictureFrom resolves the pictures of a note: a file of the archive
// becomes an address the run attaches; a web address stays a link, as
// the server fetches nothing for an import.
func (p *planner) pictureFrom(n *note) func(string) string {
	return func(src string) string {
		name, wiki := strings.CutPrefix(src, wikiScheme)
		if !wiki {
			u, err := url.Parse(src)
			if err != nil || src == "" {
				return ""
			}
			if u.Scheme == "https" || u.Scheme == "http" {
				return src
			}
			if u.Scheme != "" || u.Host != "" {
				return ""
			}
			if name, err = url.PathUnescape(u.Path); err != nil {
				name = u.Path
			}
		}
		f, ok := p.file(n, name)
		if !ok {
			p.problem("%s: the picture %s is not in the archive; it stays its name", n.path, name)
			return ""
		}
		return fileHost + url.PathEscape(f)
	}
}

// oneLine makes a text one line.
func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }
