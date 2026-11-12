package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/dbuchs7/orbithek/internal/budget"
	"github.com/dbuchs7/orbithek/internal/item"
	"github.com/dbuchs7/orbithek/internal/photo"
	"github.com/dbuchs7/orbithek/internal/share"
	"github.com/dbuchs7/orbithek/internal/site"
	"github.com/dbuchs7/orbithek/internal/storage"
)

// A share's run (ADR-0197) lists the share, reads what is new or changed
// once — its hash, what it tells about itself, a preview — and records it
// in the bound folder as the trusted store, actor sync:share:<ID>. A file
// found at a new path with the content of an entry whose path is gone is
// that entry, moved; an entry whose file is gone is withdrawn, deleted
// after withdrawnDays. A share that cannot be read empties nothing, and a
// run that would withdraw more than half of a share's entries waits for
// an administrator's confirmation.

const (
	// shareEvery is how often the runner looks for shares that are due.
	shareEvery = 10 * time.Minute
	// shareHistory is how many runs of a share are kept in memory.
	shareHistory = 10
	// withdrawnDays is how long a withdrawn entry waits for its file.
	withdrawnDays = 30
	// previewEdge is the longer edge of an entry's preview, in pixels.
	previewEdge = 1600
	// shareProblems bounds the problems one run reports by name.
	shareProblems = 20
	// heldMin is the smallest number of withdrawals a run holds back.
	heldMin = 20
)

// ErrShareBusy is a run asked for while the share runs.
var ErrShareBusy = errors.New("the share is being read; try again when the run is over")

// ShareRun is what one run of a share did, or would do in a dry run.
type ShareRun struct {
	Share    string    `json:"share"`
	Started  time.Time `json:"started"`
	Finished time.Time `json:"finished"`
	DryRun   bool      `json:"dryRun,omitempty"`
	// Seen are the files the share lists that the album takes or skips;
	// More says it listed more than ShareFiles, of which the first were
	// taken.
	Seen int  `json:"seen"`
	More bool `json:"more,omitempty"`
	// What became of them.
	Unchanged int `json:"unchanged"`
	Linked    int `json:"linked"`
	Changed   int `json:"changed"`
	Moved     int `json:"moved"`
	Restored  int `json:"restored"`
	Withdrawn int `json:"withdrawn"`
	Deleted   int `json:"deleted"`
	// FoldersRemoved are folders of directories gone, removed empty.
	FoldersRemoved int `json:"foldersRemoved,omitempty"`
	// ToRead is, in a dry run, how many files a run would read.
	ToRead int `json:"toRead,omitempty"`
	// Held are the withdrawals not made because they were too many to
	// make unconfirmed.
	Held int `json:"held,omitempty"`
	// NoPreview are entries without a preview (a format the server does
	// not scale, a video); Skipped are files of other formats; TooLarge
	// files beyond AttachmentBytes (VideoBytes for a video).
	NoPreview int      `json:"noPreview"`
	Skipped   int      `json:"skipped"`
	TooLarge  int      `json:"tooLarge"`
	Problems  []string `json:"problems,omitempty"`
	Error     string   `json:"error,omitempty"`
}

func (r *ShareRun) problem(format string, args ...any) {
	if len(r.Problems) < shareProblems {
		r.Problems = append(r.Problems, fmt.Sprintf(format, args...))
	}
}

// ShareStatus is a share's state: whether it runs, and its last runs,
// newest first.
type ShareStatus struct {
	Running bool       `json:"running"`
	History []ShareRun `json:"history"`
}

// bind gives the runner the data directory it records into.
func (sh *Shares) bind(db *storage.DB, log *slog.Logger) {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	if log == nil {
		log = slog.Default()
	}
	if sh.db == nil {
		sh.db, sh.log = db, log
	}
}

// Status answers a share's state.
func (sh *Shares) Status(id string) ShareStatus {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	st := ShareStatus{Running: sh.running[id], History: []ShareRun{}}
	st.History = append(st.History, sh.history[id]...)
	return st
}

// Running reports whether a share runs now.
func (sh *Shares) Running(id string) bool {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	return sh.running[id]
}

// Loop runs every share that is switched on and due, until ctx ends.
func (sh *Shares) Loop(ctx context.Context) {
	t := time.NewTicker(shareEvery)
	defer t.Stop()
	for {
		sh.Tick(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Tick runs the shares that are due, one after the other.
func (sh *Shares) Tick(ctx context.Context) {
	if sh.db == nil {
		return
	}
	for _, s := range sh.db.Site.Shares() {
		sh.mu.Lock()
		due := s.Enabled && sh.now().Sub(sh.last[s.ID]) >= time.Duration(s.IntervalHours)*time.Hour-shareEvery
		sh.mu.Unlock()
		if !due {
			continue
		}
		if run, err := sh.Run(ctx, s.ID, false, false); err != nil && !errors.Is(err, ErrShareBusy) {
			sh.log.Warn("file share not read", "share", s.ID, "err", err, "error", run.Error)
		}
	}
}

// Run reads one share now. dryRun reads no file and records nothing;
// confirm makes withdrawals a run would otherwise hold back.
func (sh *Shares) Run(ctx context.Context, id string, dryRun, confirm bool) (ShareRun, error) {
	s, ok := sh.db.Site.Share(id)
	if !ok {
		return ShareRun{}, fmt.Errorf("no share %q", id)
	}
	sh.mu.Lock()
	if sh.running[id] {
		sh.mu.Unlock()
		return ShareRun{}, ErrShareBusy
	}
	sh.running[id] = true
	sh.mu.Unlock()
	run := ShareRun{Share: id, Started: sh.now().UTC(), DryRun: dryRun}
	err := sh.run(ctx, s, dryRun, confirm, &run)
	if err != nil {
		run.Error = err.Error()
	}
	run.Finished = sh.now().UTC()
	sh.mu.Lock()
	sh.running[id] = false
	if !dryRun {
		sh.last[id] = run.Started
	}
	sh.history[id] = append([]ShareRun{run}, sh.history[id]...)
	if len(sh.history[id]) > shareHistory {
		sh.history[id] = sh.history[id][:shareHistory]
	}
	sh.mu.Unlock()
	return run, err
}

// entryState is an entry of the share as the run found it, with the
// folder that holds it.
type entryState struct {
	a      item.Attachment
	folder item.ID
	used   bool // claimed by a file of this run
}

func contentKey(size int64, sum string) string { return fmt.Sprint(size, ":", sum) }

// ShareOrigin is the system of the origin of the folders a share's runs
// make for its directories (ADR-0197); the instance is the share's ID and
// the external ID the directory's path.
const ShareOrigin = "share"

// dirOf is the directory of a path below a share, "" for its top.
func dirOf(p string) string {
	if d := path.Dir(p); d != "." {
		return d
	}
	return ""
}

// shareFolders makes and finds the folders of a share's directories: the
// bound folder for its top, below it a folder per directory, matched by
// its origin, so a run finds again the folder it made, and a title
// changed by hand is set back.
type shareFolders struct {
	sh     *Shares
	s      site.Share
	actor  string
	space  string
	byDir  map[string]item.ID
	failed map[string]error
}

func (f *shareFolders) of(dir string) (item.ID, error) {
	if dir == "" {
		return item.ID(f.s.Folder), nil
	}
	if id, ok := f.byDir[dir]; ok {
		return id, nil
	}
	if err, ok := f.failed[dir]; ok {
		return "", err
	}
	parent, err := f.of(dirOf(dir))
	if err != nil {
		return "", err
	}
	u := item.Upsert{Origin: item.Origin{System: ShareOrigin, Instance: f.s.ID, ExternalID: dir}, Space: f.space, Type: item.FolderType, Parent: parent, Title: path.Base(dir)}
	var made item.Item
	// Every directory is upserted once a run: an unchanged folder writes
	// nothing, and a title changed by hand is set back.
	err = f.sh.db.Write(func(st *item.Store) error {
		var err error
		if made, _, err = st.Upsert(f.actor, u); err == nil {
			return nil
		}
		// A folder a run removed comes back with its directory.
		old, ok := folderOfOrigin(st, u.Origin)
		if !ok || old.Presence != item.Trashed {
			return err
		}
		if err := st.Restore(f.actor, old.ID); err != nil {
			return err
		}
		made, _, err = st.Upsert(f.actor, u)
		return err
	})
	if err != nil {
		f.failed[dir] = err
		return "", err
	}
	f.byDir[dir] = made.ID
	return made.ID, nil
}

// folderOfOrigin finds an item by the origin a run gave it.
func folderOfOrigin(st *item.Store, o item.Origin) (item.Item, bool) {
	for _, it := range st.Find(item.Query{Type: item.FolderType, IncludeDeleted: true}) {
		if it.Origin != nil && it.Origin.System == o.System && it.Origin.Instance == o.Instance && it.Origin.ExternalID == o.ExternalID {
			return it, true
		}
	}
	return item.Item{}, false
}

// shareItems answers the entries of a share wherever they lie, and the
// folders its runs made, live ones by directory.
func shareItems(st *item.Store, id string) (entries []*entryState, folders map[string]item.Item) {
	folders = map[string]item.Item{}
	for _, it := range st.Find(item.Query{Type: item.FolderType}) {
		if it.Origin != nil && it.Origin.System == ShareOrigin && it.Origin.Instance == id {
			folders[it.Origin.ExternalID] = it
		}
		for _, a := range it.Attachments {
			if !a.Deleted && a.Linked() && a.Share.Share == id {
				entries = append(entries, &entryState{a: a, folder: it.ID})
			}
		}
	}
	return entries, folders
}

func (sh *Shares) run(ctx context.Context, s site.Share, dryRun, confirm bool, run *ShareRun) error {
	addr, err := share.ParseAddress(s.Address)
	if err != nil {
		return err
	}
	folder := item.ID(s.Folder)
	var found bool
	var space string
	var entries []*entryState
	var made map[string]item.Item
	sh.db.Read(func(st *item.Store) {
		it, err := st.Get(folder)
		if err != nil || it.Presence != item.Live || it.Type != item.FolderType {
			return
		}
		found, space = true, it.Space
		entries, made = shareItems(st, s.ID)
	})
	if !found {
		return errors.New("the share's folder is gone or no album folder")
	}
	src, err := sh.open(addr, s.ID, s.Device, sh.env)
	if err != nil {
		return err
	}
	defer src.Close()
	objs, more, err := src.List(ctx, budget.Get().ShareFiles)
	if err != nil {
		return err
	}
	run.Seen, run.More = len(objs), more

	byPath := map[string]*entryState{}
	byContent := map[string][]*entryState{}
	for _, e := range entries {
		byPath[e.a.Share.Path] = e
		byContent[contentKey(e.a.Size, e.a.SHA256)] = append(byContent[contentKey(e.a.Size, e.a.SHA256)], e)
	}
	listed := map[string]bool{}
	dirs := map[string]bool{}
	var toRead []share.Object
	var reappeared, unchanged []*entryState
	for _, o := range objs {
		listed[o.Path] = true
		for d := dirOf(o.Path); d != ""; d = dirOf(d) {
			dirs[d] = true
		}
		e := byPath[o.Path]
		switch {
		case e != nil && e.a.Share.Tag == o.Tag && e.a.Size == o.Size:
			e.used = true
			run.Unchanged++
			unchanged = append(unchanged, e)
			if e.a.Withdrawn() {
				reappeared = append(reappeared, e)
			}
		default:
			toRead = append(toRead, o)
		}
	}
	// Entries beyond the last file listed cannot be judged when the share
	// lists more than a run takes.
	judged := func(p string) bool { return !more || len(objs) == 0 || p <= objs[len(objs)-1].Path }
	gone := func(e *entryState) bool {
		return !e.used && !e.a.Withdrawn() && !listed[e.a.Share.Path] && judged(e.a.Share.Path)
	}
	if dryRun {
		run.ToRead = len(toRead)
		n := 0
		for _, e := range entries {
			if gone(e) {
				n++
			}
		}
		sh.withdrawals(len(entries), n, confirm, run, func() {})
		return nil
	}
	actor := "sync:share:" + s.ID
	folders := &shareFolders{sh: sh, s: s, actor: actor, space: space, byDir: map[string]item.ID{}, failed: map[string]error{}}
	// Folders are made for every directory listed, in path order, so a
	// directory's title follows its name even before a file needs it.
	for _, o := range objs {
		if d := dirOf(o.Path); d != "" {
			if _, err := folders.of(d); err != nil {
				run.problem("%s: %v", d, err)
			}
		}
	}
	// place moves an entry to the folder of its file's directory.
	place := func(e *entryState, p string) error {
		to, err := folders.of(dirOf(p))
		if err != nil || to == e.folder {
			return err
		}
		var a item.Attachment
		sh.db.Read(func(st *item.Store) {
			if it, err := st.Get(e.folder); err == nil {
				for _, x := range it.Attachments {
					if x.ID == e.a.ID {
						a = x
					}
				}
			}
		})
		if err := sh.db.MoveAttachmentContent(e.folder, to, a); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := sh.db.Write(func(st *item.Store) error { return st.MoveLinkedFile(actor, e.folder, e.a.ID, to) }); err != nil {
			sh.db.DiscardAttachment(to, e.a.ID)
			return err
		}
		e.folder = to
		return nil
	}

	for _, e := range reappeared {
		if err := sh.db.Write(func(st *item.Store) error {
			return st.RestoreFile(actor, e.folder, e.a.ID, item.ShareFile{Path: e.a.Share.Path, Tag: e.a.Share.Tag})
		}); err != nil {
			run.problem("%s: %v", e.a.Share.Path, err)
			continue
		}
		run.Restored++
	}
	// An unchanged entry outside the folder of its directory, such as one
	// linked before the folders were made, goes there.
	for _, e := range unchanged {
		before := e.folder
		if err := place(e, e.a.Share.Path); err != nil {
			run.problem("%s: %v", e.a.Share.Path, err)
		} else if e.folder != before {
			run.Moved++
		}
	}

	for f := range sh.readAll(ctx, src, toRead) {
		if f.err != nil {
			switch {
			case errors.Is(f.err, errSkipped):
				run.Skipped++
			case errors.Is(f.err, errTooLarge):
				run.TooLarge++
			default:
				run.problem("%s: %v", f.obj.Path, f.err)
			}
			continue
		}
		at := &item.ShareFile{Share: s.ID, Path: f.obj.Path, Tag: f.obj.Tag}
		rec := item.Attachment{Name: path.Base(f.obj.Path), Type: f.typ, Size: f.obj.Size, SHA256: f.sum, Photo: f.photo, Fingerprint: f.fingerprint, Share: at}
		if checkName(rec.Name) != nil {
			run.Skipped++
			continue
		}
		if f.preview == nil {
			run.NoPreview++
		}
		// The same path with new content: the entry, read again.
		if e := byPath[f.obj.Path]; e != nil {
			e.used = true
			rec.ID = e.a.ID
			if err := sh.storePreview(e.folder, rec.ID, f.preview); err != nil {
				run.problem("%s: %v", f.obj.Path, err)
				continue
			}
			err := sh.db.Write(func(st *item.Store) error {
				if e.a.Withdrawn() {
					if err := st.RestoreFile(actor, e.folder, rec.ID, *at); err != nil {
						return err
					}
				}
				_, err := st.RelinkFile(actor, e.folder, rec)
				return err
			})
			if err != nil {
				run.problem("%s: %v", f.obj.Path, err)
				continue
			}
			run.Changed++
			continue
		}
		// A new path with the content of an entry whose file is gone or
		// withdrawn: that entry, moved, into the folder of its new
		// directory.
		if e := movedFrom(byContent[contentKey(f.obj.Size, f.sum)], listed); e != nil {
			e.used = true
			rec.ID = e.a.ID
			withdrawn := e.a.Withdrawn()
			// A withdrawn entry comes back first; either takes the file's
			// new path and name.
			err := sh.db.Write(func(st *item.Store) error {
				if withdrawn {
					if err := st.RestoreFile(actor, e.folder, rec.ID, *at); err != nil {
						return err
					}
				}
				_, err := st.RelinkFile(actor, e.folder, rec)
				return err
			})
			if err == nil {
				err = place(e, f.obj.Path)
			}
			if err != nil {
				run.problem("%s: %v", f.obj.Path, err)
				continue
			}
			if withdrawn {
				run.Restored++
			} else {
				run.Moved++
			}
			continue
		}
		to, err := folders.of(dirOf(f.obj.Path))
		if err != nil {
			run.problem("%s: %v", f.obj.Path, err)
			continue
		}
		rec.ID = newEntryID()
		if err := sh.storePreview(to, rec.ID, f.preview); err != nil {
			run.problem("%s: %v", f.obj.Path, err)
			continue
		}
		if err := sh.db.Write(func(st *item.Store) error { _, err := st.LinkFile(actor, to, rec); return err }); err != nil {
			sh.db.DiscardAttachment(to, rec.ID)
			run.problem("%s: %v", f.obj.Path, err)
			continue
		}
		run.Linked++
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	var goneEntries []*entryState
	for _, e := range entries {
		if gone(e) {
			goneEntries = append(goneEntries, e)
		}
	}
	sh.withdrawals(len(entries), len(goneEntries), confirm, run, func() {
		for _, e := range goneEntries {
			if err := sh.db.Write(func(st *item.Store) error { return st.WithdrawFile(actor, e.folder, e.a.ID) }); err != nil {
				run.problem("%s: %v", e.a.Share.Path, err)
				continue
			}
			run.Withdrawn++
		}
	})

	// Entries withdrawn long enough are deleted, with their previews.
	limit := sh.now().Add(-withdrawnDays * 24 * time.Hour).Unix()
	for _, e := range entries {
		if e.used || !e.a.Withdrawn() || e.a.Share.Withdrawn > limit {
			continue
		}
		if err := sh.db.Write(func(st *item.Store) error { return st.DeleteAttachment(actor, e.folder, e.a.ID) }); err != nil {
			run.problem("%s: %v", e.a.Share.Path, err)
			continue
		}
		run.Deleted++
	}

	// The folder of a directory gone is removed once it holds nothing:
	// no entry, withdrawn or not, and no folder (ADR-0197). Deeper ones
	// go first, so their parents are empty after them. Only when the
	// whole share was listed, and nothing was held back.
	if !more && run.Held == 0 {
		sh.db.Read(func(st *item.Store) { _, made = shareItems(st, s.ID) })
		var empty []string
		for dir := range made {
			if !dirs[dir] {
				empty = append(empty, dir)
			}
		}
		slices.SortFunc(empty, func(a, b string) int { return strings.Count(b, "/") - strings.Count(a, "/") })
		for _, dir := range empty {
			it := made[dir]
			err := sh.db.Write(func(st *item.Store) error {
				cur, err := st.Get(it.ID)
				if err != nil || cur.Presence != item.Live || hasLiveFiles(cur) || len(st.Find(item.Query{Parent: cur.ID})) > 0 {
					return err
				}
				return st.Trash(actor, cur.ID)
			})
			if err != nil {
				run.problem("%s: %v", dir, err)
				continue
			}
			run.FoldersRemoved++
		}
	}
	return nil
}

// withdrawals makes them, or holds them back when they are more than
// half of the share's live entries and more than heldMin, unconfirmed.
func (sh *Shares) withdrawals(entries, gone int, confirm bool, run *ShareRun, do func()) {
	if gone > heldMin && gone*2 > entries && !confirm {
		run.Held = gone
		run.problem("%d entries would be withdrawn, more than half of the share's; nothing was withdrawn — is the share complete? Run it with Confirm withdrawals to withdraw them", gone)
		return
	}
	if run.DryRun {
		run.Withdrawn = gone
		return
	}
	do()
}

// movedFrom answers an entry of the same content whose file is no longer
// listed, or is withdrawn, not yet claimed by another file of the run.
func movedFrom(candidates []*entryState, listed map[string]bool) *entryState {
	for _, e := range candidates {
		if !e.used && (e.a.Withdrawn() || !listed[e.a.Share.Path]) {
			return e
		}
	}
	return nil
}

func (sh *Shares) storePreview(folder, id item.ID, preview []byte) error {
	if preview == nil {
		return nil
	}
	_, err := sh.db.StorePreview(folder, id, bytes.NewReader(preview), budget.Get().AttachmentBytes)
	return err
}

func newEntryID() item.ID {
	var b [16]byte
	rand.Read(b[:])
	return item.ID(hex.EncodeToString(b[:]))
}

// checkName refuses a file name the item model would refuse.
func checkName(name string) error {
	if strings.TrimSpace(name) == "" || len([]rune(name)) > 255 || strings.ContainsAny(name, `/\`) {
		return errSkipped
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return errSkipped
		}
	}
	return nil
}

var (
	errSkipped  = errors.New("not a picture or video the album takes")
	errTooLarge = errors.New("too large")
)

// readFile is what a run read of one file.
type readFile struct {
	obj         share.Object
	typ, sum    string
	photo       *item.Photo
	preview     []byte
	fingerprint string
	err         error
}

// readAll reads the files with at most ShareWorkers at once across every
// run, and answers them in the order given.
func (sh *Shares) readAll(ctx context.Context, src share.Source, objs []share.Object) <-chan readFile {
	out := make(chan readFile)
	go func() {
		defer close(out)
		results := make([]chan readFile, len(objs))
		for i := range results {
			results[i] = make(chan readFile, 1)
		}
		go func() {
			var wg sync.WaitGroup
			for i, o := range objs {
				select {
				case sh.workers <- struct{}{}:
				case <-ctx.Done():
					results[i] <- readFile{obj: o, err: ctx.Err()}
					continue
				}
				wg.Add(1)
				go func(i int, o share.Object) {
					defer wg.Done()
					defer func() { <-sh.workers }()
					results[i] <- sh.read(ctx, src, o)
				}(i, o)
			}
			wg.Wait()
		}()
		for _, ch := range results {
			select {
			case f := <-ch:
				out <- f
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

// read reads one file: a picture whole, within AttachmentBytes, for its
// hash, its photo data, its fingerprint and its preview; a video
// streamed for its hash, within VideoBytes, and its header by ranges for
// its photo data.
func (sh *Shares) read(ctx context.Context, src share.Source, o share.Object) readFile {
	f := readFile{obj: o}
	head, err := readRange(ctx, src, o.Path, 0, 512)
	if err != nil {
		f.err = err
		return f
	}
	video := photo.VideoType(head)
	raw := rawType(head)
	switch {
	case video != "":
		f.typ = video
	case inlineTypes[http.DetectContentType(head)]:
		f.typ = http.DetectContentType(head)
	case raw != "":
		f.typ = raw
	default:
		f.err = errSkipped
		return f
	}
	b := budget.Get()
	// A video, a HEIC or a RAW is streamed for its hash, within
	// VideoBytes, since a camera's RAW often exceeds AttachmentBytes; the
	// server makes no preview of it, and the album shows a symbol.
	if video != "" || raw != "" {
		if o.Size > b.VideoBytes {
			f.err = errTooLarge
			return f
		}
		r, err := src.Open(ctx, o.Path)
		if err != nil {
			f.err = err
			return f
		}
		defer r.Close()
		h := sha256.New()
		n, err := io.Copy(h, io.LimitReader(r, b.VideoBytes+1))
		if err != nil {
			f.err = err
			return f
		}
		if n != o.Size {
			f.err = errors.New("the file changed while it was read")
			return f
		}
		f.sum = hex.EncodeToString(h.Sum(nil))
		if video != "" {
			f.photo = photoOf(photo.ReadVideo(&rangeReader{ctx: ctx, src: src, path: o.Path, size: o.Size}))
		}
		return f
	}
	if o.Size > b.AttachmentBytes {
		f.err = errTooLarge
		return f
	}
	r, err := src.Open(ctx, o.Path)
	if err != nil {
		f.err = err
		return f
	}
	data, err := io.ReadAll(io.LimitReader(r, b.AttachmentBytes+1))
	r.Close()
	if err != nil {
		f.err = err
		return f
	}
	if int64(len(data)) != o.Size {
		f.err = errors.New("the file changed while it was read")
		return f
	}
	sum := sha256.Sum256(data)
	f.sum = hex.EncodeToString(sum[:])
	f.photo = photoOf(photo.Read(bytes.NewReader(data), f.typ))
	orientation := 0
	if f.photo != nil {
		orientation = f.photo.Orientation
	}
	if p, _, err := photo.Thumbnail(bytes.NewReader(data), f.typ, previewEdge, orientation, b.ThumbnailPixels); err == nil {
		f.preview = p
	}
	if fp, err := photo.Fingerprint(bytes.NewReader(data), f.typ, orientation, b.ThumbnailPixels); err == nil {
		f.fingerprint = fp
	}
	return f
}

func readRange(ctx context.Context, src share.Source, p string, off, n int64) ([]byte, error) {
	r, err := src.OpenRange(ctx, p, off, n)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(io.LimitReader(r, n))
}

// rangeReader reads a file of a share by ranges, a chunk at a time, so a
// video's header is read in kilobytes however large the video is.
type rangeReader struct {
	ctx   context.Context
	src   share.Source
	path  string
	size  int64
	off   int64
	chunk []byte
	at    int64
}

const rangeChunk = 64 << 10

func (r *rangeReader) Read(p []byte) (int, error) {
	if r.off >= r.size {
		return 0, io.EOF
	}
	if r.chunk == nil || r.off < r.at || r.off >= r.at+int64(len(r.chunk)) {
		data, err := readRange(r.ctx, r.src, r.path, r.off, min(rangeChunk, r.size-r.off))
		if err != nil {
			return 0, err
		}
		if len(data) == 0 {
			return 0, io.ErrUnexpectedEOF
		}
		r.chunk, r.at = data, r.off
	}
	n := copy(p, r.chunk[r.off-r.at:])
	r.off += int64(n)
	return n, nil
}

func (r *rangeReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		offset += r.off
	case io.SeekEnd:
		offset += r.size
	default:
		return 0, errors.New("seek: whence")
	}
	if offset < 0 {
		return 0, errors.New("seek: negative position")
	}
	r.off = offset
	return offset, nil
}

// rawType tells a HEIC or a camera's RAW by its leading bytes, "" for
// anything else: the album lists them from a file share with a symbol, as
// the server decodes neither (ADR-0197).
func rawType(head []byte) string {
	switch {
	case len(head) >= 12 && string(head[4:8]) == "ftyp":
		switch string(head[8:12]) {
		case "heic", "heix", "hevc", "hevx", "heim", "heis", "hevm", "hevs", "mif1", "msf1":
			return "image/heic"
		case "avif", "avis":
			return "image/avif"
		case "crx ":
			return "image/x-canon-cr3"
		}
	case bytes.HasPrefix(head, []byte("FUJIFILMCCD-RAW")):
		return "image/x-fuji-raf"
	case bytes.HasPrefix(head, []byte("IIRO")), bytes.HasPrefix(head, []byte("IIRS")):
		return "image/x-olympus-orf"
	case bytes.HasPrefix(head, []byte("IIU\x00")):
		return "image/x-panasonic-rw2"
	// TIFF, which DNG and the RAW of Canon (CR2), Nikon (NEF), Sony (ARW)
	// and others are.
	case bytes.HasPrefix(head, []byte("II*\x00")), bytes.HasPrefix(head, []byte("MM\x00*")):
		return "image/tiff"
	}
	return ""
}
