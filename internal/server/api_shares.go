package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dbuchs7/orbithek/internal/budget"
	"github.com/dbuchs7/orbithek/internal/storage"

	"github.com/dbuchs7/orbithek/internal/audit"
	"github.com/dbuchs7/orbithek/internal/item"
	"github.com/dbuchs7/orbithek/internal/share"
	"github.com/dbuchs7/orbithek/internal/site"
)

// Shares holds what the album's file shares need besides the site's
// settings (ADR-0197): the process environment, which names the
// directories administrators may bind and holds the keys of S3 shares.
type Shares struct {
	env  func(string) string
	now  func() time.Time
	open func(share.Address, string, uint64, share.Env) (share.Source, error)
	// workers bounds the files runs read at once (ShareWorkers).
	workers chan struct{}

	mu      sync.Mutex
	db      *storage.DB
	log     *slog.Logger
	running map[string]bool
	last    map[string]time.Time
	history map[string][]ShareRun
}

// NewShares makes the shares' state over an environment; Routes binds it
// to the data directory, and the caller runs its Loop.
func NewShares(env func(string) string) *Shares {
	return &Shares{env: env, now: time.Now, open: func(a share.Address, id string, dev uint64, env share.Env) (share.Source, error) {
		return share.Open(a, id, dev, env)
	},
		workers: make(chan struct{}, max(1, budget.Get().ShareWorkers)),
		running: map[string]bool{}, last: map[string]time.Time{}, history: map[string][]ShareRun{}}
}

// shareRoutes bind album folders to file shares (ADR-0197). They are for
// administrators and are not offered to agents (ADR-0012): binding a
// share discloses everything its credential reads to the folder's
// readers.
func shareRoutes(d Deps) []Route {
	h := authHandlers{d: d}
	return []Route{
		{Method: http.MethodGet, Pattern: "/api/shares", Summary: "The file shares album folders show, with the directories the operator allows and whether each S3 share's keys are set (administrators)", Handler: http.HandlerFunc(h.shares)},
		{Method: http.MethodPost, Pattern: "/api/shares", Summary: "Bind an album folder to a file share: ID, name, address (a directory of the server or s3://bucket/folder?endpoint=…&region=…), folder, link template, schedule (administrators)", Handler: http.HandlerFunc(h.addShare)},
		{Method: http.MethodPut, Pattern: "/api/shares/{id}", Summary: "Change a file share's name, address, link template or schedule; its ID and folder stay (administrators)", Handler: http.HandlerFunc(h.setShare)},
		{Method: http.MethodDelete, Pattern: "/api/shares/{id}", Summary: "Remove a file share with its entries and their previews; the folder stays (administrators)", Handler: http.HandlerFunc(h.removeShare)},
		{Method: http.MethodPost, Pattern: "/api/shares/{id}/run", Summary: "Read a file share now, in the background; dryRun reads no file and records nothing; confirm makes withdrawals a run holds back (administrators)", Handler: http.HandlerFunc(h.runShare)},
	}
}

// shareJSON is a share with what the page shows about it.
type shareJSON struct {
	site.Share
	Kind share.Kind `json:"kind"`
	// AccessKeyVar and SecretKeyVar name the environment variables of an
	// S3 share's keys; KeysSet says whether both are set, never their
	// values.
	AccessKeyVar string `json:"accessKeyVar,omitempty"`
	SecretKeyVar string `json:"secretKeyVar,omitempty"`
	KeysSet      bool   `json:"keysSet,omitempty"`
	FolderTitle  string `json:"folderTitle"`
	// Entries counts the share's entries in its folder, Withdrawn those
	// whose file is gone for now.
	Entries   int         `json:"entries"`
	Withdrawn int         `json:"withdrawn"`
	Status    ShareStatus `json:"status"`
}

func (h authHandlers) shareJSON(sh site.Share) shareJSON {
	out := shareJSON{Share: sh, Kind: sh.Kind()}
	if out.Kind == share.S3 {
		out.AccessKeyVar, out.SecretKeyVar = share.KeyVars(sh.ID)
		env := h.d.Shares.env
		out.KeysSet = strings.TrimSpace(env(out.AccessKeyVar)) != "" && strings.TrimSpace(env(out.SecretKeyVar)) != ""
	}
	h.d.DB.Read(func(s *item.Store) {
		if it, err := s.Get(item.ID(sh.Folder)); err == nil {
			out.FolderTitle = it.Title
		}
		entries, _ := shareItems(s, sh.ID)
		out.Entries = len(entries)
		for _, e := range entries {
			if e.a.Withdrawn() {
				out.Withdrawn++
			}
		}
	})
	out.Status = h.d.Shares.Status(sh.ID)
	return out
}

func (h authHandlers) shares(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.adminOnly(w, r); !ok {
		return
	}
	list := []shareJSON{}
	for _, sh := range h.d.DB.Site.Shares() {
		list = append(list, h.shareJSON(sh))
	}
	roots := share.Roots(h.d.Shares.env)
	if roots == nil {
		roots = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"shares":       list,
		"roots":        roots,
		"rootsVar":     share.RootsVar,
		"allowPrivate": h.d.Shares.env(share.AllowPrivateVar) == "on",
		"intervalMin":  site.ShareIntervalMin,
		"intervalMax":  site.ShareIntervalMax,
	})
}

// checkShare checks what the settings cannot know: that a directory lies
// where the operator allows and records the device it lies on, and that
// the folder is an album folder the administrator may edit. A folder that
// is bound for the first time must hold no pictures of its own.
func (h authHandlers) checkShare(w http.ResponseWriter, actor string, sh *site.Share, adding bool) bool {
	*sh = sh.Normalize()
	if err := sh.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return false
	}
	a, _ := share.ParseAddress(sh.Address)
	if a.Kind == share.Dir {
		if err := share.CheckDir(a.Dir, share.Roots(h.d.Shares.env), h.d.DB.Dir()); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return false
		}
		dev, err := share.DeviceOf(a.Dir)
		if err != nil {
			writeError(w, http.StatusBadRequest, "the server cannot read the directory "+a.Dir+"; is the share mounted?")
			return false
		}
		sh.Device = dev
	}
	var msg string
	h.d.DB.Read(func(s *item.Store) {
		// The folder may be named by its key, as people see it.
		it, err := resolveItem(s.As(actor), sh.Folder)
		if err == nil {
			sh.Folder = string(it.ID)
		}
		switch {
		case err != nil || it.Type != item.FolderType:
			msg = "the share fills an album folder; this is none"
		case !s.As(actor).CanEdit(it.ID):
			msg = "you may not edit the folder"
		case adding && hasLiveFiles(it):
			msg = "the folder holds pictures of its own; a share fills an empty folder"
		}
	})
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return false
	}
	return true
}

func hasLiveFiles(it item.Item) bool {
	for _, a := range it.Attachments {
		if !a.Deleted {
			return true
		}
	}
	return false
}

func (h authHandlers) saveShare(w http.ResponseWriter, r *http.Request, actor string, sh site.Share, event string, status int) {
	kept, err := h.d.DB.Site.SetShare(actor, sh)
	if errors.Is(err, site.ErrInvalid) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "share not saved")
		return
	}
	h.record(audit.Entry{Event: event, Actor: actor, Addr: clientAddr(r), Detail: map[string]string{
		"share": kept.ID, "kind": string(kept.Kind()), "folder": kept.Folder,
	}})
	writeJSON(w, status, h.shareJSON(kept))
}

func (h authHandlers) addShare(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.adminOnly(w, r)
	if !ok {
		return
	}
	var sh site.Share
	if err := readJSON(w, r, &sh); err != nil {
		badRequest(w, err)
		return
	}
	if _, exists := h.d.DB.Site.Share(strings.TrimSpace(sh.ID)); exists {
		writeError(w, http.StatusConflict, "a share "+strings.TrimSpace(sh.ID)+" exists")
		return
	}
	if !h.checkShare(w, actor, &sh, true) {
		return
	}
	h.saveShare(w, r, actor, sh, audit.ShareAdded, http.StatusCreated)
}

func (h authHandlers) setShare(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.adminOnly(w, r)
	if !ok {
		return
	}
	old, exists := h.d.DB.Site.Share(r.PathValue("id"))
	if !exists {
		writeError(w, http.StatusNotFound, "no such share")
		return
	}
	var sh site.Share
	if err := readJSON(w, r, &sh); err != nil {
		badRequest(w, err)
		return
	}
	sh.ID = old.ID
	if strings.TrimSpace(sh.Folder) == "" {
		sh.Folder = old.Folder
	}
	if !h.checkShare(w, actor, &sh, false) {
		return
	}
	h.saveShare(w, r, actor, sh, audit.ShareChanged, http.StatusOK)
}

func (h authHandlers) removeShare(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.adminOnly(w, r)
	if !ok {
		return
	}
	sh, exists := h.d.DB.Site.Share(r.PathValue("id"))
	if !exists {
		writeError(w, http.StatusNotFound, "no such share")
		return
	}
	if h.d.Shares.Running(sh.ID) {
		writeError(w, http.StatusConflict, ErrShareBusy.Error())
		return
	}
	// Its entries go first, with their previews and what was set on
	// them; the folder stays (ADR-0197).
	removed := 0
	var gone []*entryState
	var made map[string]item.Item
	h.d.DB.Read(func(s *item.Store) { gone, made = shareItems(s, sh.ID) })
	for _, e := range gone {
		if err := h.d.DB.Write(func(s *item.Store) error { return s.DeleteAttachment(actor, e.folder, e.a.ID) }); err != nil {
			writeError(w, http.StatusInternalServerError, "the share's entries were not all removed; try again")
			return
		}
		removed++
	}
	// The folders its runs made go to the trash, deeper ones first; one
	// that holds something people put there stays.
	dirs := make([]string, 0, len(made))
	for d := range made {
		dirs = append(dirs, d)
	}
	slices.SortFunc(dirs, func(a, b string) int { return strings.Count(b, "/") - strings.Count(a, "/") })
	for _, d := range dirs {
		_ = h.d.DB.Write(func(s *item.Store) error {
			cur, err := s.Get(made[d].ID)
			if err != nil || cur.Presence != item.Live || hasLiveFiles(cur) || len(s.Find(item.Query{Parent: cur.ID})) > 0 {
				return err
			}
			return s.Trash(actor, cur.ID)
		})
	}
	if err := h.d.DB.Site.RemoveShare(actor, sh.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "share not removed")
		return
	}
	h.record(audit.Entry{Event: audit.ShareRemoved, Actor: actor, Addr: clientAddr(r), Detail: map[string]string{"share": sh.ID, "folder": sh.Folder, "entries": strconv.Itoa(removed)}})
	w.WriteHeader(http.StatusNoContent)
}

// boundFolder refuses to add a file to a folder that shows a file share:
// the folder shows its share and nothing else, and the share is the
// master (ADR-0197). It answers whether the caller may go on.
func boundFolder(w http.ResponseWriter, d Deps, id item.ID) bool {
	var sh site.Share
	var ok bool
	d.DB.Read(func(s *item.Store) { sh, ok = shareTree(s, d.DB.Site, id) })
	if ok {
		writeError(w, http.StatusConflict, "the folder shows the file share "+sh.Name+"; add the file to the share instead")
		return false
	}
	return true
}

// shareTree answers the share whose bound folder is id or holds it: the
// folders of a share's directories are its runs' alone (ADR-0197).
func shareTree(s *item.Store, sites *site.Settings, id item.ID) (site.Share, bool) {
	for depth := 0; id != "" && depth < 256; depth++ {
		if sh, ok := sites.ShareOfFolder(string(id)); ok {
			return sh, true
		}
		it, err := s.Get(id)
		if err != nil {
			return site.Share{}, false
		}
		id = it.Parent
	}
	return site.Share{}, false
}

// treeRefusal refuses to change the folders of a share by hand.
func treeRefusal(sh site.Share) error {
	return fmt.Errorf("%w: %w: the folder shows the file share %s, which is the master; its folders follow the share's directories", item.ErrValidation, item.ErrReferenced, sh.Name)
}

// runShare reads a share now, in the background (ADR-0197).
func (h authHandlers) runShare(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.adminOnly(w, r)
	if !ok {
		return
	}
	var req struct {
		DryRun  bool `json:"dryRun"`
		Confirm bool `json:"confirm"`
	}
	if err := readJSON(w, r, &req); err != nil {
		badRequest(w, err)
		return
	}
	sh, exists := h.d.DB.Site.Share(r.PathValue("id"))
	if !exists {
		writeError(w, http.StatusNotFound, "no such share")
		return
	}
	if h.d.Shares.Running(sh.ID) {
		writeError(w, http.StatusConflict, ErrShareBusy.Error())
		return
	}
	if req.Confirm && !req.DryRun {
		h.record(audit.Entry{Event: audit.ShareConfirmed, Actor: actor, Addr: clientAddr(r), Detail: map[string]string{"share": sh.ID}})
	}
	go func() {
		if _, err := h.d.Shares.Run(context.Background(), sh.ID, req.DryRun, req.Confirm); err != nil && !errors.Is(err, ErrShareBusy) {
			h.d.Log.Warn("file share not read", "share", sh.ID, "err", err)
		}
	}()
	w.WriteHeader(http.StatusAccepted)
}
