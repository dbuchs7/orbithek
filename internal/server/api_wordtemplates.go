package server

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/dbuchs7/orbithek/internal/audit"
	"github.com/dbuchs7/orbithek/internal/auth"
	"github.com/dbuchs7/orbithek/internal/budget"
	"github.com/dbuchs7/orbithek/internal/export"
	"github.com/dbuchs7/orbithek/internal/item"
	"github.com/dbuchs7/orbithek/internal/markdown"
	"github.com/dbuchs7/orbithek/internal/site"
	"github.com/dbuchs7/orbithek/internal/wordtemplate"
)

// wordTemplateRoutes are the Word templates of types (ADR-0167):
// administrators keep them, versioned, with the corporate design; every
// signed-in reader gets a page of such a type framed as its template's
// document, and an editor moves a page to a newer version of it.
func wordTemplateRoutes(d Deps) []Route {
	h := itemHandlers{d: d, log: d.Log}
	if h.log == nil {
		h.log = slog.New(slog.DiscardHandler)
	}
	get, post, put, del := http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete
	return []Route{
		{Method: get, Pattern: "/api/word-templates", Summary: "The Word templates of types, their versions, what the newest holds and what uses them (administrators)", Handler: http.HandlerFunc(h.listWordTemplates)},
		{Method: post, Pattern: "/api/word-templates/{id}", Summary: "Add a Word template, or a new version of it, from a .dotx or .docx file, checked and cleaned (administrators; name and file in the query, the body is the file, application/octet-stream)", Handler: http.HandlerFunc(h.addWordTemplate)},
		{Method: put, Pattern: "/api/word-templates/{id}", Summary: "Rename a Word template (administrators)", Handler: http.HandlerFunc(h.renameWordTemplate)},
		{Method: del, Pattern: "/api/word-templates/{id}", Summary: "Remove a Word template that no type and no item uses (administrators)", Handler: http.HandlerFunc(h.removeWordTemplate)},
		{Method: get, Pattern: "/api/word-templates/{id}/versions/{version}/file", Summary: "A version of a Word template as kept, cleaned (administrators)", Handler: http.HandlerFunc(h.wordTemplateFile)},
		{Method: get, Pattern: "/api/word-templates/{id}/versions/{version}/style.css", Summary: "The stylesheet that frames a page as a version of a Word template", Handler: http.HandlerFunc(h.wordTemplateCSS)},
		{Method: get, Pattern: "/api/word-templates/{id}/versions/{version}/media/{name}", Summary: "A picture of a version of a Word template: PNG, JPEG or GIF", Handler: http.HandlerFunc(h.wordTemplateMedia)},
		{Method: get, Pattern: "/api/items/{ref}/document", Summary: "The item framed as its Word template's document: sheets with the template's header, footer and content, its fields filled in, and where the text goes", Handler: http.HandlerFunc(h.itemDocument)},
		{Method: post, Pattern: "/api/items/{ref}/template", Summary: "Move the item to the newest version of its type's Word template, or to the version given", Handler: http.HandlerFunc(h.setItemTemplate)},
	}
}

// templates keeps parsed templates by their content, so a page's frame
// does not unpack its template on every read. Versions never change, so
// an entry never goes stale; the cache is emptied when it grows past what
// an installation keeps.
var templates = struct {
	sync.Mutex
	byTag map[string]*wordtemplate.Template
}{byTag: map[string]*wordtemplate.Template{}}

// unpackedTemplate bounds what a template's parts may unpack to.
func unpackedTemplate() int64 { return budget.Get().WordTemplateBytes * 8 }

// loadTemplate returns a version of a template, parsed; version 0 is the
// newest.
func (h itemHandlers) loadTemplate(id string, version int) (*wordtemplate.Template, site.WordTemplateVersion, error) {
	meta, data, ok := h.d.DB.Site.WordTemplateFile(id, version)
	if !ok {
		return nil, meta, fmt.Errorf("%w: Word template %q, version %d", item.ErrNotFound, id, version)
	}
	templates.Lock()
	defer templates.Unlock()
	if t, ok := templates.byTag[meta.Tag]; ok {
		return t, meta, nil
	}
	t, err := wordtemplate.Parse(data, unpackedTemplate())
	if err != nil {
		return nil, meta, err
	}
	if len(templates.byTag) >= 256 {
		templates.byTag = map[string]*wordtemplate.Template{}
	}
	templates.byTag[meta.Tag] = t
	return t, meta, nil
}

// templateRef is the template version an item is written with: its own,
// or the newest of its type's template for an item made before the type
// had one. Nil when the type has none.
func (h itemHandlers) templateRef(it item.Item, td item.TypeDef) *item.TemplateRef {
	if td.Template == "" {
		return nil
	}
	if it.Template != nil && it.Template.ID == td.Template {
		return it.Template
	}
	if v := h.d.DB.Site.NewestWordTemplate(td.Template); v > 0 {
		return &item.TemplateRef{ID: td.Template, Version: v}
	}
	return nil
}

// controls are the texts a template's content controls show of an item:
// its title, key, type, state, room, labels and properties, by name in
// lower case, as a reader reads them: a referenced item by its key and
// title, a file by its name (ADR-0202).
func controls(s *item.Store, it item.Item, td item.TypeDef, spaceName string, names map[auth.UserID]string) map[string]string {
	out := map[string]string{
		"title": it.Title, "key": it.Key, "type": it.Type, "state": it.State, "space": spaceName,
		"labels": strings.Join(it.Labels, ", "),
	}
	for _, f := range td.Fields {
		out[strings.ToLower(f.Name)] = propertyText(shownValue(s, it, f.Kind, it.Properties[f.Name], true), f.Kind, names)
	}
	return out
}

// startMarkdown writes a type's start text as Markdown to edit; the
// blocks Markdown has no form for are lines of their own.
func startMarkdown(blocks []item.Block) string {
	var parts []string
	var run []item.Block
	flush := func() {
		if len(run) > 0 {
			parts = append(parts, strings.TrimSuffix(export.BlocksMarkdown(run), "\n"))
			run = nil
		}
	}
	for _, b := range blocks {
		if marker, ok := startMarkers[b.Kind]; ok {
			flush()
			parts = append(parts, marker)
			continue
		}
		run = append(run, b)
	}
	flush()
	return strings.Join(parts, "\n\n")
}

// startMarkers are the lines that stand for blocks Markdown has no form
// for in a type's start text.
var startMarkers = map[item.BlockKind]string{item.TOC: "[toc]", item.Children: "[children]", item.Figures: "[figures]", item.Tables: "[tables]"}

var startMarkerLine = regexp.MustCompile(`(?mi)^[ \t]*\[(toc|children|figures|tables)\][ \t]*$`)

// startFromMarkdown converts a type's start text from Markdown: a level-1
// heading stays a heading, links keep their safe targets, pictures keep
// only their text, and the marker lines become their blocks.
func startFromMarkdown(src string) []item.Block {
	var out []item.Block
	convert := func(chunk string) {
		if strings.TrimSpace(chunk) == "" {
			return
		}
		// A front matter title keeps a leading heading among the blocks.
		d := markdown.Convert([]byte("---\ntitle: start\n---\n"+chunk), markdown.Source{Picture: func(string) string { return "" }})
		out = append(out, d.Blocks...)
	}
	at := 0
	for _, m := range startMarkerLine.FindAllStringSubmatchIndex(src, -1) {
		convert(src[at:m[0]])
		kind := map[string]item.BlockKind{"toc": item.TOC, "children": item.Children, "figures": item.Figures, "tables": item.Tables}[strings.ToLower(src[m[2]:m[3]])]
		out = append(out, item.Block{Kind: kind})
		at = m[1]
	}
	convert(src[at:])
	return out
}

type wordTemplateJSON struct {
	site.WordTemplate
	// Info is what the newest version holds; Types name the types that
	// use the template, Items count the pages made with it.
	Info  *wordtemplate.Info `json:"info,omitempty"`
	Error string             `json:"error,omitempty"`
	Types []string           `json:"types"`
	Items int                `json:"items"`
}

func (h itemHandlers) wordTemplateJSON(t site.WordTemplate) wordTemplateJSON {
	out := wordTemplateJSON{WordTemplate: t, Types: []string{}}
	if tp, _, err := h.loadTemplate(t.ID, 0); err == nil {
		info := tp.Info()
		out.Info = &info
	} else {
		out.Error = err.Error()
	}
	h.d.DB.Read(func(s *item.Store) {
		types, n := s.TemplateUses(t.ID)
		if types != nil {
			out.Types = types
		}
		out.Items = n
	})
	return out
}

// adminOnly answers a request of someone who is no administrator.
func (h itemHandlers) adminOnly(w http.ResponseWriter, r *http.Request) bool {
	var err error
	h.d.DB.Read(func(s *item.Store) { err = h.isAdmin(s, r) })
	if err != nil {
		h.fail(w, err)
		return false
	}
	return true
}

func (h itemHandlers) listWordTemplates(w http.ResponseWriter, r *http.Request) {
	if !h.adminOnly(w, r) {
		return
	}
	out := []wordTemplateJSON{}
	for _, t := range h.d.DB.Site.WordTemplates() {
		out = append(out, h.wordTemplateJSON(t))
	}
	writeJSON(w, http.StatusOK, map[string]any{"templates": out, "controlPrefix": wordtemplate.ControlPrefix, "maxBytes": budget.Get().WordTemplateBytes})
}

// templateFailed answers a refused change of the templates.
func (h itemHandlers) templateFailed(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, site.ErrInvalid), errors.Is(err, wordtemplate.ErrInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, site.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		h.fail(w, err)
	}
}

func (h itemHandlers) addWordTemplate(w http.ResponseWriter, r *http.Request) {
	if !h.adminOnly(w, r) {
		return
	}
	id := r.PathValue("id")
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/octet-stream" {
		writeError(w, http.StatusUnsupportedMediaType, "the request body must be application/octet-stream")
		return
	}
	limit := budget.Get().WordTemplateBytes
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &tooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("a Word template may have at most %d bytes", limit))
		return
	case err != nil:
		writeError(w, http.StatusBadRequest, "the file could not be read")
		return
	}
	tp, err := wordtemplate.Parse(data, unpackedTemplate())
	if err != nil {
		h.templateFailed(w, err)
		return
	}
	clean, err := tp.Package()
	if err != nil {
		h.fail(w, err)
		return
	}
	q := r.URL.Query()
	name := q.Get("name")
	if name == "" {
		if t, ok := h.d.DB.Site.WordTemplate(id); ok {
			name = t.Name
		}
	}
	version, err := h.d.DB.Site.AddWordTemplate(who(r), id, name, q.Get("file"), clean)
	if err != nil {
		h.templateFailed(w, err)
		return
	}
	h.d.DB.Audit.Record(audit.Entry{Event: audit.DesignChanged, Actor: who(r), Addr: clientAddr(r),
		Detail: map[string]string{"part": "word-template", "template": id, "version": strconv.Itoa(version)}})
	t, _ := h.d.DB.Site.WordTemplate(id)
	out := h.wordTemplateJSON(t)
	// What cleaning took out is told once, when it happens: the kept
	// version is the cleaned one.
	if out.Info != nil {
		out.Info.Removed = tp.Info().Removed
	}
	writeJSON(w, http.StatusCreated, map[string]any{"template": out, "version": version})
}

func (h itemHandlers) renameWordTemplate(w http.ResponseWriter, r *http.Request) {
	if !h.adminOnly(w, r) {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &req) {
		return
	}
	id := r.PathValue("id")
	if err := h.d.DB.Site.RenameWordTemplate(who(r), id, req.Name); err != nil {
		h.templateFailed(w, err)
		return
	}
	h.d.DB.Audit.Record(audit.Entry{Event: audit.DesignChanged, Actor: who(r), Addr: clientAddr(r), Detail: map[string]string{"part": "word-template", "template": id, "renamed": "true"}})
	t, _ := h.d.DB.Site.WordTemplate(id)
	writeJSON(w, http.StatusOK, map[string]any{"template": h.wordTemplateJSON(t)})
}

func (h itemHandlers) removeWordTemplate(w http.ResponseWriter, r *http.Request) {
	if !h.adminOnly(w, r) {
		return
	}
	id := r.PathValue("id")
	var types []string
	var n int
	h.d.DB.Read(func(s *item.Store) { types, n = s.TemplateUses(id) })
	// A template is removed only when nothing uses it, as an item is
	// deleted only when nothing refers to it (D8): pages keep exporting
	// with the version they were made with.
	if len(types) > 0 || n > 0 {
		writeError(w, http.StatusConflict, fmt.Sprintf("the Word template is used by %d types (%s) and %d items", len(types), strings.Join(types, ", "), n))
		return
	}
	if err := h.d.DB.Site.RemoveWordTemplate(who(r), id); err != nil {
		h.templateFailed(w, err)
		return
	}
	h.d.DB.Audit.Record(audit.Entry{Event: audit.DesignChanged, Actor: who(r), Addr: clientAddr(r), Detail: map[string]string{"part": "word-template", "template": id, "removed": "true"}})
	w.WriteHeader(http.StatusNoContent)
}

// versionOf reads the path's template ID and version.
func versionOf(w http.ResponseWriter, r *http.Request) (string, int, bool) {
	id := r.PathValue("id")
	v, err := strconv.Atoi(r.PathValue("version"))
	if !site.ValidWordTemplateID(id) || err != nil || v < 1 {
		writeError(w, http.StatusNotFound, "no such version of a Word template")
		return "", 0, false
	}
	return id, v, true
}

func (h itemHandlers) wordTemplateFile(w http.ResponseWriter, r *http.Request) {
	if !h.adminOnly(w, r) {
		return
	}
	id, v, ok := versionOf(w, r)
	if !ok {
		return
	}
	meta, data, found := h.d.DB.Site.WordTemplateFile(id, v)
	if !found {
		writeError(w, http.StatusNotFound, "no such version of a Word template")
		return
	}
	attach(w, "application/vnd.openxmlformats-officedocument.wordprocessingml.template", cmp.Or(meta.File, id+".dotx"))
	_, _ = w.Write(data)
}

// templateScope is the class a page framed as a version of a template
// carries, which its stylesheet's rules are under.
func templateScope(id string, version int) string {
	return fmt.Sprintf("wt-t-%s-%d", id, version)
}

func (h itemHandlers) wordTemplateCSS(w http.ResponseWriter, r *http.Request) {
	id, v, ok := versionOf(w, r)
	if !ok {
		return
	}
	tp, _, err := h.loadTemplate(id, v)
	if err != nil {
		h.templateFailed(w, err)
		return
	}
	serveCSS(w, r, tp.Stylesheet(".wt-doc."+templateScope(id, v)))
}

func (h itemHandlers) wordTemplateMedia(w http.ResponseWriter, r *http.Request) {
	id, v, ok := versionOf(w, r)
	if !ok {
		return
	}
	tp, meta, err := h.loadTemplate(id, v)
	if err != nil {
		h.templateFailed(w, err)
		return
	}
	data, typ, found := tp.Media(r.PathValue("name"))
	if !found {
		writeError(w, http.StatusNotFound, "no such picture in the Word template")
		return
	}
	hd := w.Header()
	hd.Set("Content-Type", typ)
	hd.Set("X-Content-Type-Options", "nosniff")
	hd.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	// A version never changes; its pictures may be kept.
	hd.Set("Cache-Control", "private, max-age=86400")
	hd.Set("ETag", strconv.Quote(meta.Tag))
	_, _ = w.Write(data)
}

// documentJSON is a page framed as its template's document.
type documentJSON struct {
	Template struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Version int    `json:"version"`
		Newest  int    `json:"newest"`
	} `json:"template"`
	Scope      string             `json:"scope"`
	Stylesheet string             `json:"stylesheet"`
	Frame      wordtemplate.Frame `json:"frame"`
}

func (h itemHandlers) itemDocument(w http.ResponseWriter, r *http.Request) {
	var it item.Item
	var td item.TypeDef
	var values map[string]string
	var err error
	h.d.DB.Read(func(s *item.Store) {
		x := s.As(who(r))
		if it, err = resolveItem(x, r.PathValue("ref")); err != nil {
			return
		}
		td = s.Type(it.Type)
		names := h.d.DB.Accounts.Usernames(userIDs([]item.Item{it}, s.Type))
		spaceName := it.Space
		for _, sp := range x.Spaces() {
			if sp.Key == it.Space {
				spaceName = sp.Name
			}
		}
		values = controls(s, it, td, spaceName, names)
	})
	if err != nil {
		h.fail(w, err)
		return
	}
	ref := h.templateRef(it, td)
	if ref == nil {
		writeError(w, http.StatusNotFound, "the item's type has no Word template")
		return
	}
	tp, _, err := h.loadTemplate(ref.ID, ref.Version)
	if err != nil {
		h.templateFailed(w, err)
		return
	}
	var out documentJSON
	out.Template.ID, out.Template.Version = ref.ID, ref.Version
	if t, ok := h.d.DB.Site.WordTemplate(ref.ID); ok {
		out.Template.Name, out.Template.Newest = t.Name, len(t.Versions)
	}
	base := fmt.Sprintf("/api/word-templates/%s/versions/%d/", ref.ID, ref.Version)
	out.Scope = templateScope(ref.ID, ref.Version)
	out.Stylesheet = base + "style.css"
	out.Frame = tp.Frame(values, func(name string) string { return base + "media/" + name })
	writeJSON(w, http.StatusOK, out)
}

func (h itemHandlers) setItemTemplate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Version int `json:"version"`
	}
	if r.ContentLength != 0 && !decode(w, r, &req) {
		return
	}
	h.write(w, r, http.StatusOK, func(x *item.Session, s *item.Store) (any, error) {
		it, err := resolveItem(x, r.PathValue("ref"))
		if err != nil {
			return nil, err
		}
		td := s.Type(it.Type)
		if td.Template == "" {
			return nil, fmt.Errorf("%w: the type %s has no Word template", item.ErrValidation, it.Type)
		}
		newest := h.d.DB.Site.NewestWordTemplate(td.Template)
		v := cmp.Or(req.Version, newest)
		if v < 1 || v > newest {
			return nil, fmt.Errorf("%w: the Word template %s has no version %d", item.ErrValidation, td.Template, v)
		}
		if err := x.SetTemplate(it.ID, item.TemplateRef{ID: td.Template, Version: v}); err != nil {
			return nil, err
		}
		it, err = x.Get(it.ID)
		if err != nil {
			return nil, err
		}
		return map[string]itemJSON{"item": withSecrets(x, toItemJSON(it))}, nil
	})
}
