package server

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dbuchs7/orbithek/internal/budget"
	"github.com/dbuchs7/orbithek/internal/item"
	"github.com/dbuchs7/orbithek/internal/storage"
	"github.com/dbuchs7/orbithek/internal/whiteboard"
)

// attachmentRoutes are the routes of item attachments (ADR-0045).
func attachmentRoutes(h itemHandlers) []Route {
	return []Route{
		{Method: http.MethodGet, Pattern: "/api/items/{ref}/attachments", Summary: "List the item's files", Handler: http.HandlerFunc(h.listAttachments)},
		{Method: http.MethodPost, Pattern: "/api/items/{ref}/attachments", Summary: "Upload a file: the body is the content (application/octet-stream), name the file name; sourceItem and sourceAttachment name the file it is a copy or a new version of; place=strip removes where a photo was taken, from the file and its data; place=private keeps it to the uploader; video=1 takes a video of up to VideoBytes, given more time (ADR-0189)", Handler: http.HandlerFunc(h.uploadAttachment)},
		{Method: http.MethodGet, Pattern: "/api/items/{ref}/attachments/{id}", Summary: "Download a file; images show inline, everything else downloads", Handler: http.HandlerFunc(h.downloadAttachment)},
		{Method: http.MethodDelete, Pattern: "/api/items/{ref}/attachments/{id}", Summary: "Delete a file; its content is destroyed", Handler: http.HandlerFunc(h.deleteAttachment)},
		{Method: http.MethodPut, Pattern: "/api/items/{ref}/attachments/{id}/place", Summary: "Keep where a photo was taken to oneself, the one who added it (private), or show it to every reader again; others then read the photo data and get the file without it", Handler: http.HandlerFunc(h.setPlacePrivate)},
		{Method: http.MethodPut, Pattern: "/api/items/{ref}/attachments/{id}/labels", Summary: "Set a picture's labels (labels); they narrow the album and are no labels of the item, so no kanban or label board shows them", Handler: http.HandlerFunc(h.setPictureLabels)},
		{Method: http.MethodPut, Pattern: "/api/items/{ref}/attachments/{id}/name", Summary: "Rename a file of an item, a picture or any other (name, without its extension): every version of the file takes the name, keeping its extension and a drawing its marker; copies in other items keep theirs (ADR-0180, ADR-0182)", Handler: http.HandlerFunc(h.renamePicture)},
		{Method: http.MethodPost, Pattern: "/api/items/{ref}/attachments/{id}/move", Summary: "Move a file with all its versions to another item (to), keeping its identity: its versions, labels, notes, place and favourites go with it, and copies find it there; who may edit both items may. A file the item's text shows, a protected file and a drawing that comments are on stay (ADR-0188)", Handler: http.HandlerFunc(h.moveFile)},
		{Method: http.MethodPut, Pattern: "/api/items/{ref}/attachments/{id}/poster", Summary: "Keep a picture taken from a video as its poster (JPEG, PNG or WebP, application/octet-stream), shown in the album in its place; who may add files to the item may (ADR-0189)", Handler: http.HandlerFunc(h.setPoster)},
		{Method: http.MethodPut, Pattern: "/api/items/{ref}/attachments/{id}/notes", Summary: "Set a picture's speaker's notes (notes, plain text; empty clears them), kept on the file's first version so every version shows them (ADR-0176)", Handler: http.HandlerFunc(h.setPictureNotes)},
		{Method: http.MethodGet, Pattern: "/api/album", Summary: "The album: images, drawings and videos of every item one may read, newest first (order=taken: by when they were taken; order=slides: as the slides of a presentation, by the order of the folder or collection set by hand, saying so with ordered and whether one may change it with orderable, the rest by name), the versions of one file stacked under the newest (versions); space, folder (only its pictures), collection (the pictures of every room its rule gathers), q (name, label, key or title, or a day, month or year the picture was taken, e.g. 2026, 11.2026, 10.11.2026, 2026-11-10, November 2026, 10.11. for that day in every year), label, kind (image, drawing or video), offset and limit narrow it; person narrows to the pictures that show a person (ADR-0194)", Handler: http.HandlerFunc(h.album)},
		{Method: http.MethodGet, Pattern: "/api/album/folders", Summary: "The album's folders one may read, by name, each saying whether one may add pictures to it (editable), then its collections (collections, each with its rule): in space (every space when empty), right below parent (the top when empty)", Handler: http.HandlerFunc(h.albumFolders)},
		{Method: http.MethodPost, Pattern: "/api/album/folders", Summary: "Make a folder of the album: name, and space or parent (a folder); who may create there may", Handler: http.HandlerFunc(h.createAlbumFolder)},
		{Method: http.MethodPost, Pattern: "/api/album/collections", Summary: "Make a collection of the album, which gathers the labelled pictures of every room: name, space or parent (a folder), labels, match (any or all) and items (the labels of a picture's item count too); who may create there may", Handler: http.HandlerFunc(h.createAlbumCollection)},
		{Method: http.MethodPut, Pattern: "/api/album/collections/{ref}", Summary: "Change a collection's rule: labels, match (any or all) and items; who may edit the collection may", Handler: http.HandlerFunc(h.setAlbumCollection)},
		{Method: http.MethodPut, Pattern: "/api/album/order/{ref}", Summary: "Move pictures within the order of the slides of a folder or collection ({ref}), set by hand: pictures (the first versions of their stacks, first, in the order they are to have) go right before before, or to the end without it; the album lists the pictures so with order=slides, those the order does not name after the others by name; who may edit the folder or collection may (ADR-0195)", Handler: http.HandlerFunc(h.placePictures)},
		{Method: http.MethodDelete, Pattern: "/api/album/order/{ref}", Summary: "Forget the order of the slides set by hand of a folder or collection ({ref}): its slides are ordered by name again; who may edit it may (ADR-0195)", Handler: http.HandlerFunc(h.clearPictureOrder)},
		{Method: http.MethodGet, Pattern: "/api/album/labels", Summary: "The labels of the pictures one may read, most used first (space; q: holding it)", Handler: http.HandlerFunc(h.albumLabels)},
	}
}

// DrawingSuffix ends the name of a drawing's file (ADR-0053).
const DrawingSuffix = ".drawing.png"

// albumLimit is the most pictures one album request returns.
const albumLimit = 200

// album lists the images of the items the caller may read (ADR-0054). To
// use one in another item, the UI copies it there: a body shows only its
// own item's files, under that item's key and access.
func (h itemHandlers) album(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f, err := h.albumFilterOf(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	offset, limit := 0, 48
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "offset must be a number from 0")
			return
		}
		offset = n
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > albumLimit {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("limit must be from 1 to %d", albumLimit))
			return
		}
		limit = n
	}
	out := []albumPicture{}
	var page any
	h.d.DB.Read(func(s *item.Store) { page, err = h.albumPage(s.As(who(r)), &f, q, offset, limit, &out) })
	if err != nil {
		h.fail(w, err)
		return
	}
	// The vectors are files of storage, read once the store is free.
	h.markUnrecognised(out)
	writeJSON(w, http.StatusOK, page)
}

// albumPage is one page of the album's stacks for the filters f; out
// receives its pictures.
func (h itemHandlers) albumPage(x *item.Session, f *albumFilter, q url.Values, offset, limit int, out *[]albumPicture) (any, error) {
	// A collection gathers by its rule from every room (ADR-0175); the
	// other filters narrow what it gathers.
	if err := withCollection(x, f, q.Get("collection")); err != nil {
		return nil, err
	}
	// Ordered as slides: by the order set by hand (ADR-0195).
	ordered, orderable := withSlideRanks(x, f)
	// The versions of one file form a stack (ADR-0103), newest on top.
	all := albumStacks(x, *f)
	for _, st := range all[min(offset, len(all)):min(offset+limit, len(all))] {
		top := h.albumPictureOf(st[0])
		top.Notes = st[0].Notes
		taken := takenOf(st[0].Attachment)
		top.Taken = &taken
		top.Favourite = f.favourites[st[0].First]
		if r := st[0].Retention; r != nil {
			rule := ruleJSON(*r)
			top.Retention = &rule
		}
		for _, p := range st[1:] {
			top.Versions = append(top.Versions, h.albumPictureOf(p))
		}
		*out = append(*out, top)
	}
	next := 0
	if offset+limit < len(all) {
		next = offset + limit
	}
	page := map[string]any{"pictures": *out, "total": len(all), "next": next}
	if ordered {
		page["ordered"] = true
	}
	if orderable {
		page["orderable"] = true
	}
	return page, nil
}

// albumItemRef names a picture's item in the album's listings.
type albumItemRef struct {
	ID    item.ID `json:"id"`
	Key   string  `json:"key"`
	Title string  `json:"title"`
	Space string  `json:"space"`
	Type  string  `json:"type"`
}

// albumPicture is a picture as the album lists it (ADR-0054).
type albumPicture struct {
	Item       albumItemRef   `json:"item"`
	Attachment attachmentJSON `json:"attachment"`
	Drawing    bool           `json:"drawing"`
	// Video: the file is a video, shown by its poster when it has
	// one (Poster), else by its symbol (ADR-0189).
	Video  bool `json:"video,omitempty"`
	Poster bool `json:"poster,omitempty"`
	// Used: its item's text shows it, so the album does not delete
	// it. Editable: the caller may label or delete it (ADR-0090).
	Used     bool `json:"used"`
	Editable bool `json:"editable"`
	// First is the first version of the file, which names its
	// stack; Favourite whether the caller marked it; Taken when it
	// was taken, as the album sorts it (ADR-0177).
	First     item.ID    `json:"first"`
	Favourite bool       `json:"favourite"`
	Taken     *takenJSON `json:"taken,omitempty"`
	// Versions are the older versions of the file, newest first
	// (ADR-0103); absent for a file with one version.
	Versions []albumPicture `json:"versions,omitempty"`
	// Retention is the stack's own rule for its older versions
	// (ADR-0104); absent while the album's default applies.
	Retention *retentionJSON `json:"retention,omitempty"`
	// Notes are the stack's speaker's notes, for showing it as a
	// slide (ADR-0176); absent without notes.
	Notes string `json:"notes,omitempty"`
	// FacesUnrecognised: recognition is on, the caller may edit the
	// picture, and a face of it has nothing computed to recognise it,
	// so it is searched again (ADR-0199). The vectors never leave.
	FacesUnrecognised bool `json:"facesUnrecognised,omitempty"`
}

// albumPictureOf is a picture as the album lists it, without the parts
// that belong to its stack (versions, rule, notes, favourite, taken).
func (h itemHandlers) albumPictureOf(p item.Picture) albumPicture {
	return albumPicture{Item: albumItemRef{p.Item.ID, p.Item.Key, p.Item.Title, p.Item.Space, p.Item.Type},
		Attachment: toAttachmentJSON(p.Attachment), Drawing: albumKind(p.Attachment) == "drawing", Video: albumVideo(p.Attachment),
		Poster: albumVideo(p.Attachment) && h.d.DB.HasPoster(p.Item.ID, p.Attachment.ID), Used: p.Used, Editable: p.Editable, First: p.First}
}

type attachmentJSON struct {
	ID      item.ID   `json:"id"`
	Name    string    `json:"name"`
	Type    string    `json:"type"`
	Size    int64     `json:"size"`
	SHA256  string    `json:"sha256"`
	Author  string    `json:"author"`
	Created time.Time `json:"created"`
	// Inline is whether the file shows in the page (an image of a safe
	// format), rather than only downloads.
	Inline bool `json:"inline"`
	// Origin says where a copy came from (ADR-0055), as far as the reader
	// may know; absent for a file that is no copy.
	Origin *copyOriginJSON `json:"origin,omitempty"`
	// Labels are the picture's own (ADR-0090).
	Labels []string `json:"labels,omitempty"`
	// Photo is what the picture told about itself when it was stored
	// (ADR-0177).
	Photo *item.Photo `json:"photo,omitempty"`
	// Faces are the faces found on it and the persons named on them;
	// FacesSeen says it was searched for faces (ADR-0194).
	Faces     []faceJSON `json:"faces,omitempty"`
	FacesSeen bool       `json:"facesSeen,omitempty"`
}

type copyOriginJSON struct {
	// Readable is false when the source is gone or not the reader's to
	// see; nothing else is told then.
	Readable bool `json:"readable"`
	Item     *struct {
		ID    item.ID `json:"id"`
		Key   string  `json:"key"`
		Title string  `json:"title"`
	} `json:"item,omitempty"`
	// Newer is the newest version of the source, when it is newer than the
	// copy (a drawing edited since).
	Newer *attachmentJSON `json:"newer,omitempty"`
}

// withOrigin adds where a copy came from.
func withOrigin(x *item.Session, id item.ID, a attachmentJSON) attachmentJSON {
	o, ok, err := x.FileOrigin(id, a.ID)
	if err != nil || !ok {
		return a
	}
	a.Origin = &copyOriginJSON{Readable: o.Readable}
	if o.Readable {
		a.Origin.Item = &struct {
			ID    item.ID `json:"id"`
			Key   string  `json:"key"`
			Title string  `json:"title"`
		}{o.Item.ID, o.Item.Key, o.Item.Title}
		if o.Latest != nil {
			n := toAttachmentJSON(*o.Latest)
			a.Origin.Newer = &n
		}
	}
	return a
}

func toAttachmentJSON(a item.Attachment) attachmentJSON {
	return attachmentJSON{ID: a.ID, Name: a.Name, Type: a.Type, Size: a.Size, SHA256: a.SHA256, Author: a.Author,
		Created: time.Unix(a.Created, 0).UTC(), Inline: inlineTypes[a.Type], Labels: a.Labels, Photo: a.Photo,
		Faces: toFacesJSON(a.Faces), FacesSeen: a.FacesSeen}
}

func (h itemHandlers) setPictureLabels(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Labels []string `json:"labels"`
	}
	if decode(w, r, &req) {
		h.change(w, r, func(x *item.Session, _ *item.Store, it item.Item) error {
			return x.SetPictureLabels(it.ID, item.ID(r.PathValue("id")), req.Labels)
		})
	}
}

// renamePicture renames every version of a file, a picture or any other
// (ADR-0180, ADR-0182).
func (h itemHandlers) renamePicture(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if decode(w, r, &req) {
		h.change(w, r, func(x *item.Session, _ *item.Store, it item.Item) error {
			return x.RenameFile(it.ID, item.ID(r.PathValue("id")), req.Name)
		})
	}
}

// moveFile moves a file with all its versions to another item, keeping
// its identity (ADR-0188): the content of each version is sealed again
// for the new item first, then the move is recorded, which removes the
// former content. A drawing open as a live whiteboard stays where it is
// while people work on it (ADR-0172).
func (h itemHandlers) moveFile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		To string `json:"to"`
	}
	if !decode(w, r, &req) {
		return
	}
	file := item.ID(r.PathValue("id"))
	var from, to, first item.ID
	var live []item.Attachment
	var check error
	h.d.DB.Read(func(s *item.Store) {
		x := s.As(who(r))
		it, err := resolveItem(x, r.PathValue("ref"))
		if err != nil {
			check = err
			return
		}
		dst, err := resolveItem(x, req.To)
		if err != nil {
			check = err
			return
		}
		from, to = it.ID, dst.ID
		first, live, check = x.PlanMove(from, file, to)
	})
	if check != nil {
		h.fail(w, check)
		return
	}
	if h.d.Whiteboards != nil && !h.d.Whiteboards.Yield(whiteboard.Key{Item: from, First: first}) {
		writeError(w, http.StatusConflict, "this drawing is open as a live whiteboard; move it once the whiteboard is closed")
		return
	}
	var stored []item.ID
	discard := func() {
		for _, id := range stored {
			h.d.DB.DiscardAttachment(to, id)
		}
	}
	for _, a := range live {
		if err := h.d.DB.MoveAttachmentContent(from, to, a); err != nil {
			discard()
			h.fail(w, err)
			return
		}
		stored = append(stored, a.ID)
	}
	h.write(w, r, http.StatusOK, func(x *item.Session, _ *item.Store) (any, error) {
		if err := x.MoveFile(from, file, to, stored); err != nil {
			discard()
			return nil, err
		}
		a, err := x.Attachment(to, file)
		if err != nil {
			return nil, err
		}
		return map[string]attachmentJSON{"attachment": withOrigin(x, to, toAttachmentJSON(a))}, nil
	})
}

// setPictureNotes sets the speaker's notes of a picture (ADR-0176).
func (h itemHandlers) setPictureNotes(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Notes string `json:"notes"`
	}
	if decode(w, r, &req) {
		h.change(w, r, func(x *item.Session, _ *item.Store, it item.Item) error {
			return x.SetPictureNotes(it.ID, item.ID(r.PathValue("id")), req.Notes)
		})
	}
}

// placePictures moves pictures within the order of the slides of a
// folder or collection set by hand (ADR-0195).
func (h itemHandlers) placePictures(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Pictures []item.ID `json:"pictures"`
		Before   item.ID   `json:"before"`
	}
	if !decode(w, r, &req) {
		return
	}
	h.write(w, r, http.StatusOK, func(x *item.Session, _ *item.Store) (any, error) {
		it, err := resolveItem(x, r.PathValue("ref"))
		if err != nil {
			return nil, err
		}
		if err := x.PlacePictures(it.ID, req.Pictures, req.Before); err != nil {
			return nil, err
		}
		return map[string]bool{"ordered": true}, nil
	})
}

// clearPictureOrder forgets the order of the slides set by hand of a
// folder or collection (ADR-0195).
func (h itemHandlers) clearPictureOrder(w http.ResponseWriter, r *http.Request) {
	h.write(w, r, http.StatusOK, func(x *item.Session, _ *item.Store) (any, error) {
		it, err := resolveItem(x, r.PathValue("ref"))
		if err != nil {
			return nil, err
		}
		if err := x.ClearPictureOrder(it.ID); err != nil {
			return nil, err
		}
		return map[string]bool{"ordered": false}, nil
	})
}

// albumFolders lists the album's folders (ADR-0090) and, beside them,
// its collections (ADR-0175).
func (h itemHandlers) albumFolders(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	h.read(w, r, func(x *item.Session) (any, error) {
		space, parent := q.Get("space"), item.ID(q.Get("parent"))
		out := []map[string]any{}
		for _, f := range x.Folders(space, parent) {
			// editable says whether one may upload into it (ADR-0187).
			out = append(out, map[string]any{"id": f.ID, "key": f.Key, "name": f.Title, "space": f.Space, "parent": f.Parent, "editable": x.CanEdit(f.ID)})
		}
		collections := []map[string]any{}
		for _, c := range x.Collections(space, parent) {
			collections = append(collections, map[string]any{"id": c.Item.ID, "key": c.Item.Key, "name": c.Item.Title, "space": c.Item.Space,
				"parent": c.Item.Parent, "rule": toRuleJSON(c.Rule), "editable": c.Editable})
		}
		return map[string]any{"folders": out, "collections": collections}, nil
	})
}

// collectionRuleJSON is a collection's rule as the API takes and gives it.
type collectionRuleJSON struct {
	Labels []string `json:"labels"`
	// Match is any (a picture carries one of the labels) or all.
	Match string `json:"match"`
	// Items: the labels of a picture's item count as its own.
	Items bool `json:"items"`
}

func toRuleJSON(r item.CollectionRule) collectionRuleJSON {
	match := "any"
	if r.All {
		match = "all"
	}
	return collectionRuleJSON{Labels: append([]string{}, r.Labels...), Match: match, Items: r.Items}
}

// rule reads a rule given to the API; match is any when left out.
func (j collectionRuleJSON) rule() (item.CollectionRule, error) {
	if j.Match != "" && j.Match != "any" && j.Match != "all" {
		return item.CollectionRule{}, fmt.Errorf("%w: match is any or all", item.ErrValidation)
	}
	return item.CollectionRule{Labels: j.Labels, All: j.Match == "all", Items: j.Items}, nil
}

func (h itemHandlers) createAlbumCollection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Space  string  `json:"space"`
		Parent item.ID `json:"parent"`
		Name   string  `json:"name"`
		collectionRuleJSON
	}
	if !decode(w, r, &req) {
		return
	}
	h.write(w, r, http.StatusCreated, func(x *item.Session, _ *item.Store) (any, error) {
		rule, err := req.rule()
		if err != nil {
			return nil, err
		}
		c, err := x.CreateCollection(req.Space, req.Parent, req.Name, rule)
		if err != nil {
			return nil, err
		}
		return map[string]itemJSON{"item": toItemJSON(c)}, nil
	})
}

func (h itemHandlers) setAlbumCollection(w http.ResponseWriter, r *http.Request) {
	var req collectionRuleJSON
	if !decode(w, r, &req) {
		return
	}
	// The answer is the rule as kept: its labels normalised.
	h.write(w, r, http.StatusOK, func(x *item.Session, _ *item.Store) (any, error) {
		it, err := resolveItem(x, r.PathValue("ref"))
		if err != nil {
			return nil, err
		}
		rule, err := req.rule()
		if err != nil {
			return nil, err
		}
		if err := x.SetCollectionRule(it.ID, rule); err != nil {
			return nil, err
		}
		_, kept, err := x.Collection(it.ID)
		if err != nil {
			return nil, err
		}
		return map[string]collectionRuleJSON{"rule": toRuleJSON(kept)}, nil
	})
}

func (h itemHandlers) createAlbumFolder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Space  string  `json:"space"`
		Parent item.ID `json:"parent"`
		Name   string  `json:"name"`
	}
	if !decode(w, r, &req) {
		return
	}
	h.write(w, r, http.StatusCreated, func(x *item.Session, _ *item.Store) (any, error) {
		f, err := x.CreateFolder(req.Space, req.Parent, req.Name)
		if err != nil {
			return nil, err
		}
		return map[string]itemJSON{"item": toItemJSON(f)}, nil
	})
}

// albumLabels counts the labels of the pictures one may read: the
// suggestions while one labels a picture, and the album's filter.
func (h itemHandlers) albumLabels(w http.ResponseWriter, r *http.Request) {
	h.read(w, r, func(x *item.Session) (any, error) {
		count := map[string]int{}
		for _, p := range x.Pictures(r.URL.Query().Get("space"), func(a item.Attachment) bool { return len(a.Labels) > 0 }) {
			for _, l := range p.Attachment.Labels {
				count[l]++
			}
		}
		q, _ := item.NormalLabel(r.URL.Query().Get("q"))
		var all []item.LabelCount
		for l, n := range count {
			if strings.Contains(l, q) {
				all = append(all, item.LabelCount{Label: l, Count: n})
			}
		}
		slices.SortFunc(all, func(a, b item.LabelCount) int {
			if a.Count != b.Count {
				return b.Count - a.Count
			}
			return strings.Compare(a.Label, b.Label)
		})
		out := []map[string]any{}
		for _, l := range all[:min(len(all), budget.Get().PageItems)] {
			out = append(out, map[string]any{"label": l.Label, "count": l.Count})
		}
		return map[string]any{"labels": out, "total": map[string]any{"value": len(all), "exact": true}}, nil
	})
}

// inlineTypes are the image formats a browser shows without running
// anything, item.ImageTypes. SVG is not among them: it can hold scripts
// (ADR-0014).
var inlineTypes = func() map[string]bool {
	m := map[string]bool{}
	for _, t := range item.ImageTypes {
		m[t] = true
	}
	return m
}()

func (h itemHandlers) listAttachments(w http.ResponseWriter, r *http.Request) {
	h.read(w, r, func(x *item.Session) (any, error) {
		it, err := resolveItem(x, r.PathValue("ref"))
		if err != nil {
			return nil, err
		}
		list, err := x.Attachments(it.ID)
		out := []attachmentJSON{}
		for _, a := range list {
			out = append(out, withOrigin(x, it.ID, toAttachmentJSON(a)))
		}
		return map[string]any{"attachments": out}, err
	})
}

// uploadAttachment stores the request body as a file of the item. The body
// must be application/octet-stream: like application/json for the other
// routes (readJSON), it is not a type a cross-site form can send.
func (h itemHandlers) uploadAttachment(w http.ResponseWriter, r *http.Request) {
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/octet-stream" {
		writeError(w, http.StatusUnsupportedMediaType, "the request body must be application/octet-stream")
		return
	}
	name := r.URL.Query().Get("name")
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
	limit := budget.Get().AttachmentBytes
	// A video may be larger and take longer (ADR-0189); what is stored
	// under that budget must turn out to be one.
	video := r.URL.Query().Get("video") == "1"
	if video {
		limit = max(limit, budget.Get().VideoBytes)
		longTransfer(w, true)
	}
	var body io.Reader = http.MaxBytesReader(w, r.Body, limit+1)
	// An import may leave where a photo was taken behind (ADR-0177), or
	// keep it to the one who adds it (ADR-0178); a video's place is
	// removed once it is stored.
	place := placeChoice(r.URL.Query().Get("place"), false)
	if place == placeStrip && !video {
		var err error
		if body, err = withoutPlace(body, limit); err != nil {
			h.storeFail(w, err, limit)
			return
		}
	}
	a, err := h.d.DB.StoreAttachment(id, body, limit)
	var tooLarge *http.MaxBytesError
	switch {
	case errors.Is(err, storage.ErrAttachmentTooLarge) || errors.As(err, &tooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("a file may have at most %d bytes", limit))
		return
	case err != nil:
		h.fail(w, err)
		return
	}
	a.Name = name
	// A video is told by its own bytes (ADR-0189): QuickTime, and some
	// MP4 brands, read as an octet stream to the standard detection.
	if vt := h.videoType(id, a); vt != "" {
		a.Type = vt
	} else if video && a.Size > budget.Get().AttachmentBytes {
		h.d.DB.DiscardAttachment(id, a.ID)
		writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("a file other than a video may have at most %d bytes", budget.Get().AttachmentBytes))
		return
	}
	if place == placeStrip && albumVideo(a) {
		if a, err = h.videoWithoutPlace(id, a); err != nil {
			h.storeFail(w, err, limit)
			return
		}
		a.Name = name
	}
	// What a picture or a video tells about itself is recorded with it
	// (ADR-0177, ADR-0189).
	a.Photo = h.readPhoto(id, a)
	if place == placePrivate {
		a.Photo = keepPrivate(a.Photo)
	}
	// So is how a picture looks, by which the album finds duplicates
	// (ADR-0196).
	a.Fingerprint = h.makeFingerprint(r.Context(), id, a)
	if q := r.URL.Query(); q.Get("sourceItem") != "" || q.Get("sourceAttachment") != "" {
		a.Source = &item.Source{Item: item.ID(q.Get("sourceItem")), Attachment: item.ID(q.Get("sourceAttachment"))}
	}
	// A copy of a photo given to the caller other than as stored is the
	// source as given (ADR-0178, ADR-0198).
	h.markCopyAsGiven(r, id, &a)
	// A drawing in use as a live whiteboard gets its new versions from the
	// whiteboard only: a version from the drawing editor would be written
	// over by the session's next save (ADR-0172).
	if a.Source != nil && a.Source.Item == id && h.d.Whiteboards != nil {
		var first item.ID
		h.d.DB.Read(func(s *item.Store) {
			if d, err := s.As(who(r)).Drawing(id, a.Source.Attachment); err == nil {
				first = d.First
			}
		})
		if first != "" && !h.d.Whiteboards.Yield(whiteboard.Key{Item: id, First: first}) {
			h.d.DB.DiscardAttachment(id, a.ID)
			writeError(w, http.StatusConflict, "this drawing is open as a live whiteboard; work on it there, or save it once the whiteboard is closed")
			return
		}
	}
	h.write(w, r, http.StatusCreated, func(x *item.Session, _ *item.Store) (any, error) {
		got, err := x.AddAttachment(id, a)
		if err != nil {
			h.d.DB.DiscardAttachment(id, a.ID)
			return nil, err
		}
		return map[string]attachmentJSON{"attachment": withOrigin(x, id, toAttachmentJSON(got))}, nil
	})
}

// downloadAttachment sends a file. Only images of a safe format show
// inline; everything else is a download, and no file is ever run as a page
// of this site: the response forbids sniffing and sandboxes it.
func (h itemHandlers) downloadAttachment(w http.ResponseWriter, r *http.Request) {
	var it item.Item
	var a item.Attachment
	var err error
	var hidden bool
	h.d.DB.Read(func(s *item.Store) {
		x := s.As(who(r))
		if it, err = resolveItem(x, r.PathValue("ref")); err == nil {
			a, err = x.Attachment(it.ID, item.ID(r.PathValue("id")))
			hidden = err == nil && x.PlaceHidden(it.ID, a.ID)
		}
	})
	if err != nil {
		h.fail(w, err)
		return
	}
	// A video takes longer than the server's usual minute (ADR-0189).
	if albumVideo(a) {
		longTransfer(w, false)
	}
	// A place its uploader keeps to themselves leaves the file (ADR-0178).
	content, err := h.openFor(hidden, it.ID, a)
	if errors.Is(err, errPlaceStays) {
		writeError(w, http.StatusConflict, "this picture is given only without the place it was taken, and that place could not be removed")
		return
	}
	if err != nil {
		h.log.Error("attachment content missing", "item", it.ID, "attachment", a.ID, "err", err)
		writeError(w, http.StatusGone, "the content of this file is not available")
		return
	}
	defer content.Close()
	hd := w.Header()
	disposition := "attachment"
	if inlineTypes[a.Type] || slices.Contains(item.VideoTypes, a.Type) {
		disposition = "inline"
		hd.Set("Content-Type", a.Type)
	} else {
		hd.Set("Content-Type", "application/octet-stream")
	}
	if cd := mime.FormatMediaType(disposition, map[string]string{"filename": a.Name}); cd != "" {
		hd.Set("Content-Disposition", cd)
	} else {
		hd.Set("Content-Disposition", disposition)
	}
	hd.Set("X-Content-Type-Options", "nosniff")
	hd.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	hd.Set("Cache-Control", "private, max-age=3600")
	etag := a.SHA256
	if hidden {
		etag += "-noplace"
	}
	if timeWritten(a) {
		// A time set again gives another file (ADR-0198).
		etag += "-taken-" + a.Photo.Taken + a.Photo.Offset
	}
	hd.Set("ETag", strconv.Quote(etag))
	// Parts of a file are sent as asked for (Range), so a video plays and
	// seeks (ADR-0097); the ETag answers a repeated request.
	http.ServeContent(w, r, "", time.Time{}, readLogged{content, func(err error) {
		// The status is sent; a broken file shows as a cut-off download.
		h.log.Error("attachment content does not read", "item", it.ID, "attachment", a.ID, "err", err)
	}})
}

// readLogged reports a read that fails other than at the end.
type readLogged struct {
	io.ReadSeeker
	failed func(error)
}

func (r readLogged) Read(p []byte) (int, error) {
	n, err := r.ReadSeeker.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		r.failed(err)
	}
	return n, err
}

func (h itemHandlers) deleteAttachment(w http.ResponseWriter, r *http.Request) {
	h.change(w, r, func(x *item.Session, _ *item.Store, it item.Item) error {
		return x.DeleteAttachment(it.ID, item.ID(r.PathValue("id")))
	})
}
