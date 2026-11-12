package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/dbuchs7/orbithek/internal/budget"
	"github.com/dbuchs7/orbithek/internal/collab"
	"github.com/dbuchs7/orbithek/internal/diskspace"
	"github.com/dbuchs7/orbithek/internal/item"
	"github.com/dbuchs7/orbithek/internal/storage"
	"github.com/dbuchs7/orbithek/internal/whiteboard"
)

// Live whiteboards (ADR-0169, ADR-0170): a drawing edited by several
// people and agents at once. Browsers follow a session as server-sent
// events and send operations with plain POSTs, as for text (ADR-0010);
// agents read and change it with single requests.

// whiteboardBackend is the item store as a whiteboard.Hub sees it.
type whiteboardBackend struct {
	db     *storage.DB
	collab *collab.Hub
}

func (b whiteboardBackend) Access(k whiteboard.Key, principal string) (read, edit bool) {
	b.db.Read(func(s *item.Store) {
		x := s.As(principal)
		if _, err := x.Drawing(k.Item, k.First); err != nil {
			return
		}
		read, edit = true, x.MayEdit(k.Item)
	})
	return read, edit
}

func (b whiteboardBackend) Load(k whiteboard.Key) ([]byte, error) {
	var d item.Drawing
	var err error
	b.db.Read(func(s *item.Store) { d, err = s.Drawing(k.Item, k.First) })
	if err != nil {
		return nil, err
	}
	f, err := b.db.OpenAttachment(k.Item, d.Newest.ID)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, budget.Get().AttachmentBytes+1))
}

// Save stores a picture as the drawing's newest version and points the
// image blocks of the body that show any of its versions at it, unless a
// live text session holds locks on the body (ADR-0010); then the next
// checkpoint does.
func (b whiteboardBackend) Save(k whiteboard.Key, principal string, png []byte) (item.ID, error) {
	if b.db.Space() >= diskspace.Critical {
		return "", storage.ErrNoSpace
	}
	limit := budget.Get().AttachmentBytes
	a, err := b.db.StoreAttachment(k.Item, bytes.NewReader(png), limit)
	if err != nil {
		return "", err
	}
	busy := b.collab != nil && b.collab.Busy(k.Item)
	err = b.db.Write(func(s *item.Store) error {
		x := s.As(principal)
		d, err := x.Drawing(k.Item, k.First)
		if err != nil {
			return err
		}
		a.Name, a.Source = d.Newest.Name, &item.Source{Item: k.Item, Attachment: d.Newest.ID}
		if a, err = x.AddAttachment(k.Item, a); err != nil {
			return err
		}
		if busy {
			return nil
		}
		it, err := x.Get(k.Item)
		if err != nil {
			return err
		}
		doc, changed := item.Doc{Blocks: slices.Clone(it.Body.Blocks)}, false
		for i, bl := range doc.Blocks {
			if bl.Kind == item.Image && slices.Contains(d.Versions, bl.Ref) {
				doc.Blocks[i].Ref, changed = a.ID, true
			}
		}
		if changed {
			// A body that cannot be committed (one holding passages the
			// keeper may not read) keeps showing an older version; the
			// version itself is saved.
			_ = x.CommitBody(k.Item, doc)
		}
		return nil
	})
	if err != nil {
		b.db.DiscardAttachment(k.Item, a.ID)
		return "", err
	}
	return a.ID, nil
}

// NewWhiteboards returns a whiteboard hub over the data directory; text
// sessions on the same items are respected when a checkpoint points the
// body at a new version.
func NewWhiteboards(db *storage.DB, live *collab.Hub, log *slog.Logger) *whiteboard.Hub {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return whiteboard.NewHub(whiteboardBackend{db: db, collab: live}, whiteboard.WithLog(log))
}

func whiteboardRoutes(h itemHandlers) []Route {
	get, post := http.MethodGet, http.MethodPost
	return []Route{
		{Method: get, Pattern: "/api/whiteboards", Summary: "The whiteboards with a live session one may read: item, drawing (its first version) and how many take part", Handler: http.HandlerFunc(h.liveWhiteboards)},
		{Method: post, Pattern: "/api/items/{ref}/whiteboards", Summary: "Make a whiteboard in an item one may edit: an empty drawing (title names it) at the end of the body; answers its file", Handler: http.HandlerFunc(h.createWhiteboard)},
		{Method: get, Pattern: "/api/items/{ref}/whiteboards/{file}", Summary: "Read a whiteboard (any version of a drawing names it) as agents need it: its elements in plain terms, who holds them, the participants, timer, vote and presentation", Handler: http.HandlerFunc(h.readWhiteboard)},
		{Method: post, Pattern: "/api/items/{ref}/whiteboards/{file}/changes", Summary: "Change a whiteboard as an agent: notes, shapes, texts, frames, arrows, new text, moves, colours, deletions and votes, all or none; answers the element each change made or changed", Handler: http.HandlerFunc(h.changeWhiteboard)},
		{Method: get, Pattern: "/api/items/{ref}/whiteboards/{file}/live", Summary: "Join a whiteboard's live session: a stream of server-sent events, starting with a snapshot", Handler: http.HandlerFunc(h.whiteboardEvents)},
		{Method: post, Pattern: "/api/items/{ref}/whiteboards/{file}/live", Summary: "Send operations in a whiteboard's live session: put, file, lock, release, pointer, background, timer, vote, present, frame, view, comments, save", Handler: http.HandlerFunc(h.whiteboardOps)},
		{Method: post, Pattern: "/api/items/{ref}/whiteboards/{file}/checkpoints", Summary: "Store the picture the keeper drew for a checkpoint (client, token; the body is the PNG, application/octet-stream); the server writes the scene into it", Handler: http.HandlerFunc(h.whiteboardCheckpoint)},
	}
}

// whiteboardKey resolves an item and any version of its drawing to the
// whiteboard, as the caller may read it.
func (h itemHandlers) whiteboardKey(r *http.Request) (whiteboard.Key, error) {
	var k whiteboard.Key
	var err error
	h.d.DB.Read(func(s *item.Store) {
		x := s.As(who(r))
		var it item.Item
		if it, err = resolveItem(x, r.PathValue("ref")); err != nil {
			return
		}
		var d item.Drawing
		if d, err = x.Drawing(it.ID, item.ID(r.PathValue("file"))); err == nil {
			k = whiteboard.Key{Item: it.ID, First: d.First}
		}
	})
	return k, err
}

func (h itemHandlers) whiteboardFail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, whiteboard.ErrLocked), errors.Is(err, whiteboard.ErrNotJoined), errors.Is(err, whiteboard.ErrStale):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, whiteboard.ErrForbidden):
		writeError(w, http.StatusForbidden, "you may not change this whiteboard")
	case errors.Is(err, whiteboard.ErrInvalid), errors.Is(err, whiteboard.ErrNotPNG), errors.Is(err, whiteboard.ErrNoScene):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, whiteboard.ErrBusy), errors.Is(err, whiteboard.ErrTooLarge):
		writeError(w, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, storage.ErrAttachmentTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("a file may have at most %d bytes", budget.Get().AttachmentBytes))
	default:
		h.fail(w, err)
	}
}

type liveWhiteboardJSON struct {
	Item         string  `json:"item"`
	Key          string  `json:"key"`
	Title        string  `json:"title"`
	Space        string  `json:"space"`
	File         item.ID `json:"file"`
	Participants int     `json:"participants"`
	// People are the participants with a browser: while there are any,
	// the drawing is not edited alone (ADR-0172).
	People int `json:"people"`
	// Versions are the IDs of every version of the drawing, so a page
	// that shows one of them knows it is in use (ADR-0172).
	Versions []item.ID `json:"versions"`
}

func (h itemHandlers) liveWhiteboards(w http.ResponseWriter, r *http.Request) {
	out := []liveWhiteboardJSON{}
	infos := h.d.Whiteboards.Sessions()
	h.d.DB.Read(func(s *item.Store) {
		x := s.As(who(r))
		for _, in := range infos {
			d, err := x.Drawing(in.Key.Item, in.Key.First)
			if err != nil {
				continue
			}
			if it, err := x.Get(in.Key.Item); err == nil {
				out = append(out, liveWhiteboardJSON{Item: string(it.ID), Key: it.Key, Title: it.Title, Space: it.Space, File: in.Key.First,
					Participants: in.Participants, People: in.People, Versions: d.Versions})
			}
		}
	})
	slices.SortFunc(out, func(a, b liveWhiteboardJSON) int { return strings.Compare(a.Key+string(a.File), b.Key+string(b.File)) })
	writeJSON(w, http.StatusOK, map[string]any{"whiteboards": out})
}

// whiteboardName makes the file name of a new whiteboard from its title.
func whiteboardName(title string) string {
	base := strings.Join(strings.Fields(strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r < ' ' {
			return ' '
		}
		return r
	}, title)), " ")
	if base == "" {
		base = "Whiteboard"
	}
	if r := []rune(base); len(r) > 80 {
		base = string(r[:80])
	}
	return base + DrawingSuffix
}

func (h itemHandlers) createWhiteboard(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title string `json:"title"`
	}
	if !decode(w, r, &req) {
		return
	}
	var id item.ID
	var check error
	h.d.DB.Read(func(s *item.Store) {
		x := s.As(who(r))
		it, err := resolveItem(x, r.PathValue("ref"))
		if err == nil {
			id, err = it.ID, x.CanAttach(it.ID)
		}
		check = err
	})
	if check != nil {
		h.fail(w, check)
		return
	}
	if !boundFolder(w, h.d, id) {
		return
	}
	if h.d.Collab != nil && h.d.Collab.Busy(id) {
		writeError(w, http.StatusConflict, "someone is editing this item's text in a live session; add the whiteboard when they are done")
		return
	}
	// A new whiteboard is an upload: refused from the critical level on
	// (ADR-0163).
	if h.d.DB.Space() >= diskspace.Critical {
		h.fail(w, storage.ErrNoSpace)
		return
	}
	p, err := whiteboard.WriteScene(whiteboard.Placeholder(), whiteboard.Scene{}, true)
	if err != nil {
		h.fail(w, err)
		return
	}
	a, err := h.d.DB.StoreAttachment(id, bytes.NewReader(p), budget.Get().AttachmentBytes)
	if err != nil {
		h.fail(w, err)
		return
	}
	a.Name = whiteboardName(req.Title)
	h.write(w, r, http.StatusCreated, func(x *item.Session, _ *item.Store) (any, error) {
		got, err := x.AddAttachment(id, a)
		if err == nil {
			var it item.Item
			if it, err = x.Get(id); err == nil {
				doc := item.Doc{Blocks: append(slices.Clone(it.Body.Blocks), item.Block{Kind: item.Image, Ref: got.ID, Text: strings.TrimSpace(req.Title)})}
				err = x.CommitBody(id, doc)
			}
		}
		if err != nil {
			h.d.DB.DiscardAttachment(id, a.ID)
			return nil, err
		}
		return map[string]any{"file": got.ID, "attachment": toAttachmentJSON(got)}, nil
	})
}

func (h itemHandlers) readWhiteboard(w http.ResponseWriter, r *http.Request) {
	k, err := h.whiteboardKey(r)
	if err != nil {
		h.fail(w, err)
		return
	}
	v, err := h.d.Whiteboards.Read(k, who(r))
	if err != nil {
		h.whiteboardFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h itemHandlers) changeWhiteboard(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Changes []whiteboard.Change `json:"changes"`
	}
	if err := readJSONUpTo(w, r, &req, budget.Get().WhiteboardRequestBytes); err != nil {
		badRequest(w, err)
		return
	}
	k, err := h.whiteboardKey(r)
	if err != nil {
		h.fail(w, err)
		return
	}
	p, _ := PrincipalFrom(r.Context())
	ids, err := h.d.Whiteboards.Change(k, string(p.User), p.Username, req.Changes)
	if err != nil {
		h.whiteboardFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"elements": ids})
}

func (h itemHandlers) whiteboardEvents(w http.ResponseWriter, r *http.Request) {
	k, err := h.whiteboardKey(r)
	if err != nil {
		h.fail(w, err)
		return
	}
	p, _ := PrincipalFrom(r.Context())
	sub, err := h.d.Whiteboards.Join(k, string(p.User), p.Username)
	if err != nil {
		h.whiteboardFail(w, err)
		return
	}
	defer sub.Leave()

	rc := http.NewResponseController(w)
	hd := w.Header()
	hd.Set("Content-Type", "text/event-stream")
	hd.Set("Cache-Control", "no-store")
	hd.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	beat := time.NewTicker(HeartbeatInterval)
	defer beat.Stop()
	write := func(chunk string) bool {
		_ = rc.SetWriteDeadline(time.Now().Add(HeartbeatInterval + 10*time.Second))
		if _, err := io.WriteString(w, chunk); err != nil {
			return false
		}
		return rc.Flush() == nil
	}
	if !write(": joined\n\n") {
		return
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case <-beat.C:
			if !write(": ping\n\n") {
				return
			}
		case e, ok := <-sub.C:
			if !ok {
				return
			}
			data, err := json.Marshal(e)
			if err != nil {
				h.log.Error("encoding a whiteboard event", "err", err)
				return
			}
			if !write("event: " + e.Kind + "\ndata: " + string(data) + "\n\n") {
				return
			}
		}
	}
}

func (h itemHandlers) whiteboardOps(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Client string          `json:"client"`
		Ops    []whiteboard.Op `json:"ops"`
	}
	if err := readJSONUpTo(w, r, &req, budget.Get().WhiteboardRequestBytes); err != nil {
		badRequest(w, err)
		return
	}
	k, err := h.whiteboardKey(r)
	if err != nil {
		h.fail(w, err)
		return
	}
	res, err := h.d.Whiteboards.Apply(k, req.Client, who(r), req.Ops)
	if err != nil {
		h.whiteboardFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h itemHandlers) whiteboardCheckpoint(w http.ResponseWriter, r *http.Request) {
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/octet-stream" {
		writeError(w, http.StatusUnsupportedMediaType, "the request body must be application/octet-stream")
		return
	}
	k, err := h.whiteboardKey(r)
	if err != nil {
		h.fail(w, err)
		return
	}
	limit := budget.Get().AttachmentBytes
	png, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("a file may have at most %d bytes", limit))
		return
	} else if err != nil {
		badRequest(w, err)
		return
	}
	q := r.URL.Query()
	sv, err := h.d.Whiteboards.Checkpoint(k, q.Get("client"), who(r), q.Get("token"), png)
	if err != nil {
		h.whiteboardFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sv)
}
