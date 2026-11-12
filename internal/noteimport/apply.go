package noteimport

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"path"
	"reflect"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/dbuchs7/orbithek/internal/budget"
	"github.com/dbuchs7/orbithek/internal/item"
	"github.com/dbuchs7/orbithek/internal/storage"
)

// System names the source in the origin of every page a notes import
// writes; the import's name and room are its instance (ADR-0209).
const System = item.ImportSystemPrefix + "notes"

// maxPictures bounds how many pictures one page keeps, as for a page
// written from a repository (ADR-0138).
const maxPictures = 50

// pictureTypes are the pictures a page keeps: those it shows inline
// (ADR-0045). An SVG can carry script and stays its name.
var pictureTypes = []string{"image/png", "image/jpeg", "image/gif", "image/webp"}

// Target says where an import writes: the room, the import's name, which
// a re-run repeats to update what the run before made, and who imports.
type Target struct {
	Space string
	Name  string
	Who   string
}

// Instance names an import in its pages' origins: its room and its name.
func Instance(space, name string) string { return space + "/" + name }

// ErrName is a name no import can have.
var ErrName = errors.New("noteimport: the import's name is a single line of 1 to 80 characters")

// CleanName is an import's name as kept: one line, trimmed.
func CleanName(name string) (string, error) {
	name = oneLine(name)
	if name == "" || utf8.RuneCountInString(name) > 80 {
		return "", ErrName
	}
	return name, nil
}

// Change is one page a run made, changed or kept.
type Change struct {
	Kind  string `json:"kind"` // new, changed, kept
	Key   string `json:"key,omitempty"`
	Title string `json:"title"`
	Path  string `json:"path"`
}

// Report is what a run did, or as a dry run would do.
type Report struct {
	DryRun    bool   `json:"dryRun"`
	Format    string `json:"format"`
	Name      string `json:"name"`
	Pages     int    `json:"pages"`
	New       int    `json:"new"`
	Changed   int    `json:"changed"`
	Unchanged int    `json:"unchanged"`
	// Pictures counts the pictures attached, or to be attached.
	Pictures int `json:"pictures"`
	// Kept counts the pages an earlier run of this import made that the
	// archive no longer holds: they stay as they are, never deleted (D8).
	Kept     int      `json:"kept"`
	Problems []string `json:"problems"`
	// Examples are the first pages made, changed or kept, to judge a dry
	// run by.
	Examples []Change `json:"examples"`
}

// examplesShown and problemsShown bound the lists of a report.
const (
	examplesShown = 50
	problemsShown = 200
)

func (r *Report) problem(format string, args ...any) {
	if len(r.Problems) < problemsShown {
		r.Problems = append(r.Problems, fmt.Sprintf(format, args...))
	} else if len(r.Problems) == problemsShown {
		r.Problems = append(r.Problems, "… and more")
	}
}

func (r *Report) count(kind, key, title, p string) {
	switch kind {
	case "new":
		r.New++
	case "changed":
		r.Changed++
	case "kept":
		r.Kept++
	}
	if len(r.Examples) < examplesShown {
		r.Examples = append(r.Examples, Change{Kind: kind, Key: key, Title: title, Path: p})
	}
}

// ErrType is a type an import cannot write: missing, or without a text.
var ErrType = errors.New("noteimport: type")

// Types checks the types an import writes as who sees them: each must
// exist and have a text.
func Types(s *item.Store, names ...string) ([]item.TypeDef, error) {
	var out []item.TypeDef
	for _, n := range names {
		td := s.Type(n)
		switch {
		case td.Name == "":
			return nil, fmt.Errorf("%w: %q does not exist", ErrType, n)
		case !td.Body:
			return nil, fmt.Errorf("%w: %q has no text", ErrType, n)
		}
		out = append(out, td)
	}
	return out, nil
}

// run is one import's writing.
type run struct {
	db   *storage.DB
	a    *Archive
	t    Target
	dry  bool
	rep  Report
	ids  map[string]item.ID
	full bool // the storage refused something new
}

// Run writes a plan into a room as who, through who's session (D10): a
// page for every note, folder and row the archive has, below the page of
// its folder, updating what an earlier run of the same import made; a
// page whose source is unchanged keeps what people wrote in it. Links
// between notes become mentions once every page exists. With dry, it
// writes nothing and reports what it would do.
func Run(db *storage.DB, a *Archive, p Plan, t Target, dry bool) (Report, error) {
	r := &run{db: db, a: a, t: t, dry: dry, ids: map[string]item.ID{},
		rep: Report{DryRun: dry, Format: p.Format, Name: t.Name, Pages: len(p.Pages), Problems: []string{}, Examples: []Change{}}}
	for _, s := range p.Problems {
		r.rep.problem("%s", s)
	}
	instance := Instance(t.Space, t.Name)
	existing := map[string]item.Item{}
	var err error
	db.Read(func(s *item.Store) {
		x := s.As(t.Who)
		if !x.MayAdminister(t.Space) {
			err = fmt.Errorf("%w: %s may not import into %s", item.ErrForbidden, t.Who, t.Space)
			return
		}
		for _, it := range s.FindBySystem(System) {
			if it.Origin.Instance == instance && it.Space == t.Space {
				existing[it.Origin.ExternalID] = it
			}
		}
	})
	if err != nil {
		return r.rep, err
	}
	for id, it := range existing {
		r.ids[id] = it.ID
	}
	var later []pending
	planned := map[string]bool{}
	for _, pg := range p.Pages {
		planned[pg.ID] = true
		old, had := existing[pg.ID]
		if pend, ok := r.page(pg, old, had); ok {
			later = append(later, pend)
		}
	}
	// A page that links to pages made after it names them now.
	for _, pend := range later {
		blocks, _ := r.resolve(pend.blocks, pend.self, true)
		if sameBlocks(blocks, pend.written) {
			continue
		}
		err := db.Write(func(s *item.Store) error { return s.As(t.Who).CommitBody(r.ids[pend.self], item.Doc{Blocks: blocks}) })
		if err != nil {
			r.rep.problem("%s: its links are not written: %v", pend.path, err)
		}
	}
	for _, id := range slices.Sorted(maps.Keys(existing)) {
		if !planned[id] {
			it := existing[id]
			r.count("kept", it.Key, it.Title, it.Origin.ExternalKey)
		}
	}
	if r.rep.Kept > 0 {
		r.rep.problem("%d pages of an earlier run are no longer in the archive; they are kept as they are", r.rep.Kept)
	}
	return r.rep, nil
}

func (r *run) count(kind, key, title, p string) { r.rep.count(kind, key, title, p) }

// pending is a page whose links name pages not made yet when it was
// written.
type pending struct {
	self, path      string
	blocks, written []item.Block
}

// page writes one page, and reports it when its links wait for pages
// made after it.
func (r *run) page(pg Page, old item.Item, had bool) (pending, bool) {
	if had && old.Type != pg.Type {
		r.rep.problem("%s: imported as %s before, now %s; it is left as it is", pg.Path, old.Type, pg.Type)
		return pending{}, false
	}
	u := item.Upsert{Origin: item.Origin{System: System, Instance: Instance(r.t.Space, r.t.Name), ExternalID: pg.ID, ExternalKey: pg.Path},
		Space: r.t.Space, Type: pg.Type, Parent: r.ids[pg.Parent], Title: pg.Title, State: pg.State}
	if pg.Parent != "" && u.Parent == "" && !r.dry {
		r.rep.problem("%s: the page that holds it was not written; it goes to the top of the room", pg.Path)
	}
	labels := slices.Clone(pg.Labels)
	if had {
		u.Parent = old.Parent
		u.Properties = maps.Clone(old.Properties)
		labels = append(labels, old.Labels...)
		u.Origin.Raw = old.Origin.Raw
		if u.State == "" {
			u.State = old.State
		}
	}
	for k, v := range pg.Props {
		if u.Properties == nil {
			u.Properties = item.Props{}
		}
		u.Properties[k] = v
	}
	slices.Sort(labels)
	labels = slices.Compact(labels)
	write := !had || old.Origin.Raw != pg.Raw
	newLabels := !slices.Equal(labels, old.Labels)
	kind := "new"
	if had {
		kind = "changed"
		if !write && !newLabels && old.Title == u.Title && old.State == u.State && reflect.DeepEqual(nonEmpty(old.Properties), nonEmpty(u.Properties)) {
			r.rep.Unchanged++
			return pending{}, false
		}
	}
	if r.dry {
		if write {
			r.rep.Pictures += min(len(pg.Pictures), maxPictures)
		}
		r.count(kind, old.Key, pg.Title, pg.Path)
		if !had {
			r.ids[pg.ID] = item.ID("new:" + pg.ID)
		}
		return pending{}, false
	}
	if !had && r.full {
		return pending{}, false
	}
	x := func(s *item.Store) *item.Session { return s.As(r.t.Who) }
	if !write {
		err := r.db.Write(func(s *item.Store) error {
			it, _, err := x(s).Import(u)
			if err == nil && newLabels {
				err = x(s).SetLabels(it.ID, labels)
			}
			return err
		})
		if err != nil {
			r.rep.problem("%s: %v", pg.Path, err)
			return pending{}, false
		}
		r.count(kind, old.Key, pg.Title, pg.Path)
		return pending{}, false
	}
	it := old
	if !had {
		// The page is made first, so its pictures can be stored for it.
		err := r.db.Write(func(s *item.Store) error {
			var err error
			it, _, err = x(s).Import(u)
			return err
		})
		if errors.Is(err, storage.ErrNothingNew) {
			r.full = true
			r.rep.problem("the storage is almost full: %s and the pages after it are not made", pg.Path)
			return pending{}, false
		}
		if err != nil {
			r.rep.problem("%s: %v", pg.Path, err)
			return pending{}, false
		}
		r.ids[pg.ID] = it.ID
	}
	blocks, added := r.pictures(pg, it)
	written, all := r.resolve(blocks, pg.ID, false)
	u.Origin.Raw = pg.Raw
	recorded := 0
	err := r.db.Write(func(s *item.Store) error {
		got, _, err := x(s).Import(u)
		if err != nil {
			return err
		}
		for _, a := range added {
			if _, err := x(s).AddAttachment(got.ID, a); err != nil {
				return err
			}
			recorded++
		}
		if err := x(s).CommitBody(got.ID, item.Doc{Blocks: written}); err != nil {
			return err
		}
		if newLabels {
			return x(s).SetLabels(got.ID, labels)
		}
		return nil
	})
	// A stored file that was never recorded goes at once; the next start
	// would remove it too.
	for _, a := range added[recorded:] {
		r.db.DiscardAttachment(it.ID, a.ID)
	}
	if err != nil {
		r.rep.problem("%s: %v", pg.Path, err)
		return pending{}, false
	}
	r.rep.Pictures += len(added)
	r.count(kind, it.Key, pg.Title, pg.Path)
	if all {
		return pending{}, false
	}
	return pending{self: pg.ID, path: pg.Path, blocks: blocks, written: written}, true
}

// pictures stores the pictures a page shows and turns their blocks into
// images; a picture the page already has with the same content is not
// stored again. One that is not a picture a page shows, too large or
// beyond the first maxPictures stays its name.
func (r *run) pictures(pg Page, it item.Item) ([]item.Block, []item.Attachment) {
	blocks := slices.Clone(pg.Blocks)
	var added []item.Attachment
	limit := budget.Get().AttachmentBytes
	for n, pic := range pg.Pictures {
		if n == maxPictures {
			r.rep.problem("%s shows more than %d pictures; the rest stay their names", pg.Path, maxPictures)
			break
		}
		data, err := r.a.Read(pic.File, limit)
		if err != nil {
			r.rep.problem("%s: the picture %s is not attached: %v", pg.Path, pic.File, err)
			continue
		}
		if !slices.Contains(pictureTypes, http.DetectContentType(data)) {
			r.rep.problem("%s: %s is no PNG, JPEG, GIF or WebP picture; it stays its name", pg.Path, pic.File)
			continue
		}
		sum := sha256.Sum256(data)
		var ref item.ID
		for _, a := range it.Attachments {
			if !a.Deleted && a.SHA256 == hex.EncodeToString(sum[:]) {
				ref = a.ID
			}
		}
		for _, a := range added {
			if a.SHA256 == hex.EncodeToString(sum[:]) {
				ref = a.ID
			}
		}
		if ref == "" {
			a, err := r.db.StoreAttachment(it.ID, bytes.NewReader(data), limit)
			if err != nil {
				r.rep.problem("%s: the picture %s is not attached: %v", pg.Path, pic.File, err)
				continue
			}
			a.Name = path.Base(pic.File)
			added = append(added, a)
			ref = a.ID
		}
		blocks[pic.Block] = item.Block{Kind: item.Image, Ref: ref, Text: pic.Caption}
	}
	return blocks, added
}

// resolve turns the links of blocks to other notes into mentions of
// their pages (D2) and reports whether every such page exists. A link
// whose page does not exist yet, and a picture's link to a file of the
// archive, leave their text without a link; with final, a link whose
// page was never made is reported.
func (r *run) resolve(blocks []item.Block, self string, final bool) ([]item.Block, bool) {
	all := true
	out := make([]item.Block, len(blocks))
	for i, b := range blocks {
		var marks []item.Mark
		b.Mentions = slices.Clone(b.Mentions)
		for _, m := range b.Marks {
			if m.Type == item.MarkLink && strings.HasPrefix(m.Href, fileHost) {
				continue
			}
			if id, ok := LinkedNote(m.Href); ok && m.Type == item.MarkLink {
				ref, made := r.ids[id]
				switch {
				case !made:
					all = false
					if final {
						r.rep.problem("a link to %s stays text: its page was not written", id)
					}
				case id != self:
					marks = append(marks, item.Mark{Type: item.MarkMention, Start: m.Start, End: m.End, Ref: ref})
					if !slices.Contains(b.Mentions, ref) {
						b.Mentions = append(b.Mentions, ref)
					}
				}
				continue
			}
			marks = append(marks, m)
		}
		b.Marks = marks
		out[i] = b
	}
	return out, all
}

// sameBlocks reports whether two texts are the same.
func sameBlocks(a, b []item.Block) bool { return reflect.DeepEqual(a, b) }

// nonEmpty treats no properties and an empty set of them alike.
func nonEmpty(p item.Props) item.Props {
	if len(p) == 0 {
		return nil
	}
	return p
}

// Import is an earlier import into a room: its name and how many pages
// it made that the caller may read.
type Import struct {
	Name  string `json:"name"`
	Pages int    `json:"pages"`
}

// Imports lists the imports into a room whose pages who may read, by
// name, so a re-run can repeat one's name.
func Imports(s *item.Store, who, space string) []Import {
	x := s.As(who)
	counts := map[string]int{}
	for _, it := range s.FindBySystem(System) {
		name, ok := strings.CutPrefix(it.Origin.Instance, space+"/")
		if !ok || it.Space != space {
			continue
		}
		if _, err := x.Get(it.ID); err == nil {
			counts[name]++
		}
	}
	out := []Import{}
	for _, n := range slices.Sorted(maps.Keys(counts)) {
		out = append(out, Import{Name: n, Pages: counts[n]})
	}
	return out
}
