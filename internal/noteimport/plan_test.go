package noteimport

import (
	"archive/zip"
	"bytes"
	"errors"
	"io/fs"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/dbuchs7/orbithek/internal/item"
)

// pngData is the smallest PNG picture.
var pngData = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89\x00\x00\x00\rIDATx\x9cc\xf8\xff\xff?\x00\x05\xfe\x02\xfe\xa7\x35\x81\x84\x00\x00\x00\x00IEND\xaeB`\x82")

// zipOf makes a ZIP archive of files by name, in the order given.
func zipOf(t *testing.T, files ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for i := 0; i+1 < len(files); i += 2 {
		f, err := w.Create(files[i])
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(files[i+1])); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

var roomy = Limits{Files: 100, Bytes: 10 << 20}

func open(t *testing.T, data []byte) *Archive {
	t.Helper()
	a, err := Open(bytes.NewReader(data), int64(len(data)), roomy)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

var pageType = item.TypeDef{Name: "page", Body: true, Fields: []item.FieldDef{{Name: "due", Kind: item.KindDate}, {Name: "owner", Kind: item.KindUser}}}

var taskType = item.TypeDef{Name: "task", Body: true, Fields: []item.FieldDef{{Name: "estimate", Kind: item.KindNumber}},
	Workflow: &item.Workflow{Initial: "Open", States: []string{"Open", "Done"}, Transitions: []item.Transition{{Name: "finish", From: []string{"Open"}, To: "Done"}}}}

func pageOf(t *testing.T, p Plan, id string) Page {
	t.Helper()
	for _, pg := range p.Pages {
		if pg.ID == id {
			return pg
		}
	}
	var ids []string
	for _, pg := range p.Pages {
		ids = append(ids, pg.ID)
	}
	t.Fatalf("no page %s among %v", id, ids)
	return Page{}
}

func hrefs(pg Page) []string {
	var out []string
	for _, b := range pg.Blocks {
		for _, m := range b.Marks {
			if m.Type == item.MarkLink && !strings.HasPrefix(m.Href, fileHost) {
				out = append(out, m.Href)
			}
		}
	}
	return out
}

func hasProblem(p Plan, part string) bool {
	return slices.ContainsFunc(p.Problems, func(s string) bool { return strings.Contains(s, part) })
}

// A vault: folders become pages, wiki links and relative links between
// notes the addresses of their pages, front matter and tags labels,
// fields and a table, pictures files to attach; what cannot be converted
// is reported (ADR-0209).
func TestAVaultBecomesATreeOfPages(t *testing.T) {
	a := open(t, zipOf(t,
		"Vault/Home.md", "---\ntitle: Start here\ntags: [a, project/x]\naliases: Start\ndue: 2026-01-05\nrating: 3\nstatus: draft\nowner: anna\n---\n"+
			"See [[Projects/Alpha|the *alpha*]] and [[Beta#Plan]] and [[Missing]]. #inline-tag #123\n\n"+
			"![[pic.png|300]]\n\n![[doc.pdf]]\n\n![[Beta]]\n\nKeep `[[NotALink]] #nottag` as code. A block ^ref1\n\n```\n[[fenced]] #fenced\n```\n",
		"Vault/Projects/Alpha.md", "# Alpha title\n\nBack [home](../Home.md), to [[Start]] and [[#Local]], [web](https://example.org).\n\n![shot](../attachments/pic.png)\n",
		"Vault/Projects/Projects.md", "The projects.\n",
		"Vault/Beta.md", "---\naliases: [B]\n---\nPlain beta. [[B]]\n",
		"Vault/Deep/Er/Note.md", "Deep.\n",
		"Vault/Deep/nothing.txt", "no note",
		"Vault/attachments/pic.png", string(pngData),
		"Vault/.obsidian/app.json", "{}",
		"Vault/data.csv", "a,b\n",
	))
	p := Build(a, Options{PageType: pageType, Lang: "en"})
	if p.Format != FormatFolder || p.Root != "Vault" {
		t.Fatalf("format %s root %s", p.Format, p.Root)
	}
	var ids []string
	for _, pg := range p.Pages {
		ids = append(ids, pg.ID)
	}
	want := []string{"dir/Deep", "dir/Deep/Er", "dir/Projects", "md/Beta.md", "md/Deep/Er/Note.md", "md/Home.md", "md/Projects/Alpha.md"}
	if !slices.Equal(ids, want) {
		t.Fatalf("pages %v, want %v", ids, want)
	}
	home := pageOf(t, p, "md/Home.md")
	if home.Title != "Start here" || home.Parent != "" || home.Type != "page" {
		t.Fatalf("home %+v", home)
	}
	if !slices.Equal(home.Labels, []string{"a", "inline-tag", "project.x"}) {
		t.Errorf("labels %v", home.Labels)
	}
	if d, _ := home.Props["due"].(time.Time); !d.Equal(time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)) || len(home.Props) != 1 {
		t.Errorf("props %v", home.Props)
	}
	if b := home.Blocks[0]; b.Kind != item.Table || b.Text != "Property\tValue\nrating\t3\nstatus\tdraft\nowner\tanna" {
		t.Errorf("table %q", b.Text)
	}
	if got := hrefs(home); !slices.Equal(got, []string{NoteLink("md/Projects/Alpha.md"), NoteLink("md/Beta.md"), NoteLink("md/Beta.md")}) {
		t.Errorf("links %v", got)
	}
	text := home.Blocks[1].Text
	if !strings.HasPrefix(text, "See the *alpha* and Beta › Plan and Missing.") || strings.Contains(home.Raw, "\r") || !strings.HasPrefix(home.Raw, "---\ntitle") {
		t.Errorf("text %q", text)
	}
	if len(home.Pictures) != 1 || home.Pictures[0].File != "attachments/pic.png" || home.Blocks[home.Pictures[0].Block].Text != "pic.png" {
		t.Errorf("pictures %+v", home.Pictures)
	}
	all := ""
	for _, b := range home.Blocks {
		all += b.Text + "\n"
	}
	for _, s := range []string{"[[NotALink]] #nottag", "[[fenced]] #fenced", "doc.pdf"} {
		if !strings.Contains(all, s) {
			t.Errorf("%q is lost from %q", s, all)
		}
	}
	if strings.Contains(all, "^ref1") {
		t.Errorf("a block's anchor stays: %q", all)
	}
	for _, s := range []string{"[[Missing]] names no note", "embedded file doc.pdf", "embedded note Beta", "data.csv: a CSV file"} {
		if !hasProblem(p, s) {
			t.Errorf("no problem %q in %v", s, p.Problems)
		}
	}
	alpha := pageOf(t, p, "md/Projects/Alpha.md")
	if alpha.Title != "Alpha title" || alpha.Parent != "dir/Projects" {
		t.Errorf("alpha %+v", alpha)
	}
	if got := hrefs(alpha); !slices.Equal(got, []string{NoteLink("md/Home.md"), NoteLink("md/Home.md"), "https://example.org"}) {
		t.Errorf("alpha links %v", got)
	}
	if len(alpha.Pictures) != 1 || alpha.Pictures[0].File != "attachments/pic.png" || alpha.Pictures[0].Caption != "shot" {
		t.Errorf("alpha pictures %+v", alpha.Pictures)
	}
	// The folder's note is its page's text, above the list of its pages.
	projects := pageOf(t, p, "dir/Projects")
	if projects.Title != "Projects" || projects.Blocks[0].Text != "The projects." || projects.Blocks[len(projects.Blocks)-1].Kind != item.Children {
		t.Errorf("projects %+v", projects)
	}
	deep := pageOf(t, p, "dir/Deep/Er")
	if deep.Parent != "dir/Deep" || len(deep.Blocks) != 1 || deep.Blocks[0].Kind != item.Children || deep.Raw == "" {
		t.Errorf("deep %+v", deep)
	}
	// A note's link to itself stays text.
	if beta := pageOf(t, p, "md/Beta.md"); len(hrefs(beta)) != 0 {
		t.Errorf("beta links itself: %v", hrefs(beta))
	}
}

const (
	idWiki  = "0123456789abcdef0123456789abcdef"
	idChild = "11111111111111111111111111111111"
	idTasks = "22222222222222222222222222222222"
	idFirst = "33333333333333333333333333333333"
)

// A Notion export: IDs in the names are stripped and identify the pages,
// links name pages by them, and a database's CSV makes items of the
// rows' type with fields and states from its columns (ADR-0209).
func TestANotionExportBecomesPagesAndItems(t *testing.T) {
	csvText := "\xef\xbb\xbfName,Status,Estimate,Tags,Owner\nFirst,Done,3,\"x, y\",anna\nSecond,Open,many,,\n,,,,\n"
	a := open(t, zipOf(t,
		"Wiki "+idWiki+".md", "# Wiki\n\nSee [Child](Wiki%20"+idWiki+"/Child%20"+idChild+".md) and [Tasks](Wiki%20"+idWiki+"/Tasks%20"+idTasks+".csv).\n\n![](Wiki%20"+idWiki+"/image.png)\n",
		"Wiki "+idWiki+"/Child "+idChild+".md", "# Child\n\nBack to [Wiki](https://www.notion.so/Wiki-"+idWiki+"?pvs=21).\n",
		"Wiki "+idWiki+"/Tasks "+idTasks+".csv", "Name\nignored\n",
		"Wiki "+idWiki+"/Tasks "+idTasks+"_all.csv", csvText,
		"Wiki "+idWiki+"/Tasks "+idTasks+"/First "+idFirst+".md", "# First\n\nStatus: Done\nEstimate: 3\n\nBody of first.\n",
		"Wiki "+idWiki+"/image.png", string(pngData),
	))
	p := Build(a, Options{PageType: pageType, RowType: taskType, Lang: "de"})
	if p.Format != FormatNotion || p.Root != "" {
		t.Fatalf("format %s root %q", p.Format, p.Root)
	}
	wiki := pageOf(t, p, "notion/"+idWiki)
	if wiki.Title != "Wiki" || wiki.Parent != "" {
		t.Fatalf("wiki %+v", wiki)
	}
	if got := hrefs(wiki); !slices.Equal(got, []string{NoteLink("notion/" + idChild), NoteLink("notion/" + idTasks)}) {
		t.Errorf("wiki links %v", got)
	}
	if len(wiki.Pictures) != 1 || wiki.Pictures[0].File != "Wiki "+idWiki+"/image.png" {
		t.Errorf("wiki pictures %+v", wiki.Pictures)
	}
	child := pageOf(t, p, "notion/"+idChild)
	if child.Parent != wiki.ID || child.Title != "Child" || !slices.Equal(hrefs(child), []string{NoteLink(wiki.ID)}) {
		t.Errorf("child %+v", child)
	}
	db := pageOf(t, p, "notion/"+idTasks)
	if db.Title != "Tasks" || db.Parent != wiki.ID || len(db.Blocks) != 1 || db.Blocks[0].Kind != item.Children || !strings.HasPrefix(db.Raw, "database:") {
		t.Errorf("database %+v", db)
	}
	first := pageOf(t, p, "notion/"+idFirst)
	if first.Type != "task" || first.Parent != db.ID || first.State != "Done" || first.Props["estimate"] != 3.0 || !slices.Equal(first.Labels, []string{"x", "y"}) {
		t.Errorf("first %+v", first)
	}
	if b := first.Blocks[0]; b.Kind != item.Table || b.Text != "Eigenschaft\tWert\nOwner\tanna" {
		t.Errorf("first's table %q", b.Text)
	}
	for _, b := range first.Blocks {
		if strings.Contains(b.Text, "Status: Done") {
			t.Errorf("the row's columns stay in its text: %+v", first.Blocks)
		}
	}
	second := pageOf(t, p, "notion/"+idTasks+"/row/second")
	if second.Type != "task" || second.State != "Open" || second.Title != "Second" || second.Parent != db.ID {
		t.Errorf("second %+v", second)
	}
	if len(p.Pages) != 5 {
		t.Errorf("pages %d", len(p.Pages))
	}
	for _, s := range []string{"the column Owner has no field in task", "the value of Estimate does not fit"} {
		if !hasProblem(p, s) {
			t.Errorf("no problem %q in %v", s, p.Problems)
		}
	}
}

// An archive is read only within itself and its limits: names that leave
// it are refused, too many files or too many bytes unpacked stop it.
func TestAnArchiveIsReadSafely(t *testing.T) {
	data := zipOf(t, "../evil.md", "x", "/abs.md", "x", "C:/win.md", "x", "ok.md", "fine", "dir\\win.md", "back", "big.md", strings.Repeat("a", 5000))
	a, err := Open(bytes.NewReader(data), int64(len(data)), Limits{Files: 6, Bytes: 5006})
	if err != nil {
		t.Fatal(err)
	}
	if got := a.Paths(); !slices.Equal(got, []string{"big.md", "dir/win.md", "ok.md"}) {
		t.Errorf("paths %v", got)
	}
	if len(a.Skipped) != 3 {
		t.Errorf("skipped %v", a.Skipped)
	}
	if _, err := a.Read("big.md", 100); !errors.Is(err, ErrTooLarge) {
		t.Errorf("a file beyond its limit: %v", err)
	}
	if _, err := a.Read("big.md", 10000); err != nil {
		t.Errorf("big.md: %v", err)
	}
	if _, err := a.Read("ok.md", 100); err != nil {
		t.Errorf("ok.md: %v", err)
	}
	if _, err := a.Read("dir/win.md", 100); !errors.Is(err, ErrArchive) {
		t.Errorf("beyond the archive's bytes: %v", err)
	}
	if _, err := a.Read("missing.md", 100); err == nil {
		t.Error("a missing file is read")
	}
	if _, err := Open(bytes.NewReader(data), int64(len(data)), Limits{Files: 5, Bytes: 6000}); !errors.Is(err, ErrArchive) {
		t.Errorf("too many files: %v", err)
	}
	if _, err := Open(strings.NewReader("no zip"), 6, roomy); !errors.Is(err, ErrArchive) {
		t.Errorf("not a ZIP archive: %v", err)
	}
	// A link and a name twice are not read.
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	h := &zip.FileHeader{Name: "link.md"}
	h.SetMode(fs.ModeSymlink | 0o777)
	f, _ := w.CreateHeader(h)
	_, _ = f.Write([]byte("/etc/passwd"))
	for range 2 {
		f, _ = w.Create("twice.md")
		_, _ = f.Write([]byte("x"))
	}
	_ = w.Close()
	a = open(t, buf.Bytes())
	if got := a.Paths(); !slices.Equal(got, []string{"twice.md"}) || len(a.Skipped) != 2 {
		t.Errorf("paths %v, skipped %v", got, a.Skipped)
	}
}

// What a lying directory says of a file's size does not let more be read.
func TestALyingSizeIsNotTrusted(t *testing.T) {
	data := zipOf(t, "a.md", strings.Repeat("a", 1000))
	a := open(t, data)
	a.files["a.md"].UncompressedSize64 = 10
	if _, err := a.Read("a.md", 100); err == nil {
		t.Errorf("read beyond the limit: %v", err)
	}
}

func TestWikiLinksBecomeCommonMark(t *testing.T) {
	for _, c := range wikiCases {
		if got := prepare(c.in).text; got != c.out {
			t.Errorf("%q: got %q, want %q", c.in, got, c.out)
		}
	}
	p := prepare("#one and #two/sub, (#three) #4 x#no `#code`")
	if !slices.Equal(p.tags, []string{"one", "two/sub", "three"}) {
		t.Errorf("tags %v", p.tags)
	}
}

var wikiCases = []struct{ in, out string }{
	{"[[Note]]", "[Note](<wiki:Note>)"},
	{"[[Note|An *alias*]]", `[An \*alias\*](<wiki:Note>)`},
	{"[[Folder/Note.md#Part]]", "[Note › Part](<wiki:Folder/Note.md#Part>)"},
	{"[[Note#^block]]", "[Note](<wiki:Note#^block>)"},
	{"[[#Local]]", "[Local](<wiki:#Local>)"},
	{"![[a b.png|200]]", "![a b\\.png](<wiki:a b.png>)"},
	{"![[x.pdf]]", `x\.pdf`},
	{"![[Other]]", "[Other](<wiki:Other>)"},
	{"`[[code]]` [[x]]", "`[[code]]` [x](<wiki:x>)"},
	{"``a`[[b]]`` c", "``a`[[b]]`` c"},
	{"open ` tick [[x]]", "open ` tick [x](<wiki:x>)"},
	{"~~~\n[[x]]\n~~~\n[[y]]", "~~~\n[[x]]\n~~~\n[y](<wiki:y>)"},
	{"text %%hidden%% shown", "text  shown"},
	{"[[a<b>]]", `[a\<b\>](<wiki:a\<b\>>)`},
	{"[[ ]]", "[[ ]]"},
}
