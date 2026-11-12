package server

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/dbuchs7/orbithek/internal/audit"
	"github.com/dbuchs7/orbithek/internal/budget"
	"github.com/dbuchs7/orbithek/internal/diskspace"
	"github.com/dbuchs7/orbithek/internal/item"
	"github.com/dbuchs7/orbithek/internal/noteimport"
	"github.com/dbuchs7/orbithek/internal/storage"
)

// noteImportRoutes import ZIP archives of notes into a room (ADR-0209):
// for its administrators, through their session.
func noteImportRoutes(h itemHandlers) []Route {
	return []Route{
		{Method: http.MethodGet, Pattern: "/api/spaces/{space}/imports", Summary: "The imports of notes into the room by name, with the pages of each the caller may read, so a re-run repeats a name (room administrators)",
			Handler: http.HandlerFunc(h.noteImports)},
		{Method: http.MethodPost, Pattern: "/api/spaces/{space}/imports/notes", Summary: "Import a ZIP archive of notes into the room: a folder of Markdown files or a workspace export of Markdown and CSV, as the body (application/octet-stream); " +
			"name names the import, which a re-run repeats to update what it made; type is the type of the pages (page when absent), rowType that of a database's rows; " +
			"dryRun=true only reports what a run would do; lang is the language of the words the import writes (room administrators)",
			Handler: http.HandlerFunc(h.importNotes)},
	}
}

// importTime is how long reading and writing an import may take: an
// archive of thousands of notes takes longer than the server's usual
// minute.
const importTime = 15 * time.Minute

func (h itemHandlers) noteImports(w http.ResponseWriter, r *http.Request) {
	h.read(w, r, func(x *item.Session) (any, error) {
		var out []noteimport.Import
		space := r.PathValue("space")
		if !x.MayAdminister(space) {
			return nil, fmt.Errorf("%w: room administrators only", item.ErrForbidden)
		}
		h.d.DB.Read(func(s *item.Store) { out = noteimport.Imports(s, who(r), space) })
		return map[string]any{"imports": out}, nil
	})
}

func (h itemHandlers) importNotes(w http.ResponseWriter, r *http.Request) {
	// Like application/json elsewhere, application/octet-stream is not a
	// type a cross-site form can send (ADR-0027).
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/octet-stream" {
		writeError(w, http.StatusUnsupportedMediaType, "the request body must be application/octet-stream")
		return
	}
	q := r.URL.Query()
	space := r.PathValue("space")
	dry, _ := strconv.ParseBool(q.Get("dryRun"))
	pageType := cmp.Or(q.Get("type"), "page")
	rowType := cmp.Or(q.Get("rowType"), pageType)
	if !dry && h.d.DB.Space() == diskspace.Full {
		h.fail(w, storage.ErrNoSpace)
		return
	}
	var types []item.TypeDef
	var check error
	h.d.DB.Read(func(s *item.Store) {
		switch x := s.As(who(r)); {
		case !s.HasSpace(space):
			check = fmt.Errorf("%w: space %q", item.ErrNotFound, space)
		case !x.MayAdminister(space):
			check = fmt.Errorf("%w: room administrators only", item.ErrForbidden)
		default:
			types, check = noteimport.Types(s, pageType, rowType)
		}
	})
	if errors.Is(check, noteimport.ErrType) {
		check = badInput("%v", check)
	}
	if check != nil {
		h.fail(w, check)
		return
	}
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Now().Add(importTime))
	_ = rc.SetWriteDeadline(time.Now().Add(importTime))
	b := budget.Get()
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, b.NoteImportBytes))
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &tooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("an archive may have at most %d bytes", b.NoteImportBytes))
		return
	case err != nil:
		badRequest(w, err)
		return
	}
	a, err := noteimport.Open(bytes.NewReader(data), int64(len(data)), noteimport.Limits{Files: b.NoteImportFiles, Bytes: b.NoteUnpackedBytes})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	name, err := noteimport.CleanName(cmp.Or(q.Get("name"), a.Root, "notes"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	plan := noteimport.Build(a, noteimport.Options{PageType: types[0], RowType: types[1], Lang: q.Get("lang")})
	rep, err := noteimport.Run(h.d.DB, a, plan, noteimport.Target{Space: space, Name: name, Who: who(r)}, dry)
	if err != nil {
		h.fail(w, err)
		return
	}
	if !dry {
		if h.d.Collab != nil {
			h.d.Collab.Revalidate()
		}
		h.d.DB.Audit.Record(audit.Entry{Event: audit.NotesImported, Actor: who(r), Subject: space, Detail: map[string]string{
			"name": name, "format": rep.Format, "new": strconv.Itoa(rep.New), "changed": strconv.Itoa(rep.Changed)}})
	}
	writeJSON(w, http.StatusOK, rep)
}
