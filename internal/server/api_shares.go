package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/dbuchs7/orbithek/internal/audit"
	"github.com/dbuchs7/orbithek/internal/item"
	"github.com/dbuchs7/orbithek/internal/share"
	"github.com/dbuchs7/orbithek/internal/site"
)

// Shares holds what the album's file shares need besides the site's
// settings (ADR-0197): the process environment, which names the
// directories administrators may bind and holds the keys of S3 shares.
type Shares struct {
	env func(string) string
}

// NewShares makes the shares' state over an environment.
func NewShares(env func(string) string) *Shares {
	return &Shares{env: env}
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
		{Method: http.MethodDelete, Pattern: "/api/shares/{id}", Summary: "Remove a file share; the folder stays (administrators)", Handler: http.HandlerFunc(h.removeShare)},
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
	})
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
		it, err := s.As(actor).Get(item.ID(sh.Folder))
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
	if err := h.d.DB.Site.RemoveShare(actor, sh.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "share not removed")
		return
	}
	h.record(audit.Entry{Event: audit.ShareRemoved, Actor: actor, Addr: clientAddr(r), Detail: map[string]string{"share": sh.ID, "folder": sh.Folder}})
	w.WriteHeader(http.StatusNoContent)
}
