package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/dbuchs7/orbithek/internal/budget"
	"github.com/dbuchs7/orbithek/internal/item"
	"github.com/dbuchs7/orbithek/internal/linkpreview"
)

// LinkPreviews reads what web pages say about themselves for editors and
// web link items (ADR-0206). A nil *LinkPreviews means the server reads
// no pages: OT_LINK_PREVIEWS is off, as every outgoing call of the server
// is until an operator switches it on.
type LinkPreviews struct {
	reader *linkpreview.Reader
}

// NewLinkPreviews returns link previews that read with reader.
func NewLinkPreviews(reader *linkpreview.Reader) *LinkPreviews {
	return &LinkPreviews{reader: reader}
}

// previewTimeout bounds one request's readings of a page and its
// picture together; each reading has its own budget, LinkPreviewSeconds.
func previewTimeout() time.Duration {
	return time.Duration(2*budget.Get().LinkPreviewSeconds+5) * time.Second
}

// The fields a page fills, by name and kind, in any type that has them
// (ADR-0206). The url field names the page; the others are what the page
// says about itself.
const (
	urlField         = "url"
	pictureName      = "preview"
	errPreviewsOff   = "the server reads no web pages; an operator sets OT_LINK_PREVIEWS=on"
	errNoAddress     = "the item has no web address in a field url"
	errBeingEdited   = "the body is being edited in a live session; join it instead"
	problemNoPicture = "the page's picture was not kept: "
)

func webLinkRoutes(h itemHandlers) []Route {
	return []Route{
		{Method: http.MethodGet, Pattern: "/api/linkpreview", Summary: "Whether the server reads what web pages say about themselves (OT_LINK_PREVIEWS, ADR-0206)", Handler: http.HandlerFunc(h.previewsOn)},
		{Method: http.MethodPost, Pattern: "/api/linkpreview", Summary: "What the web page at url says about itself: title, description, site, picture, canonical address, author, day of publication; " +
			"only public addresses, nothing is kept (off unless OT_LINK_PREVIEWS=on)", Handler: http.HandlerFunc(h.preview)},
		{Method: http.MethodPost, Pattern: "/api/weblinks", Summary: "Save a web page as a web link item in space, below parent: url, title (else the page's), quote (the text one keeps, as quotes of its body) and text " +
			"(the page's main text into its body); with OT_LINK_PREVIEWS=on its fields and picture come from the page; the ready-made type weblink is defined when there is none (ADR-0206)", Handler: http.HandlerFunc(h.saveWebLink)},
		{Method: http.MethodPost, Pattern: "/api/items/{ref}/weblink", Summary: "Read the page of the item's url field again: its fields site, description, author and published, where the type has them, and its picture, " +
			"a new version when it changed; text takes the page's main text below the body (off unless OT_LINK_PREVIEWS=on)", Handler: http.HandlerFunc(h.readWebLinkAgain)},
		{Method: http.MethodPost, Pattern: "/api/items/{ref}/body/blocks", Summary: "Add blocks at the end of the body, as one reads it, in one step; refused while people edit it in a live session (ADR-0207)", Handler: http.HandlerFunc(h.appendBlocks)},
	}
}

func (h itemHandlers) previewsOn(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"enabled": h.d.LinkPreviews != nil})
}

// previewFailed answers why a page was not read: a wrong address is the
// caller's, a page that fails the page's, and too many readings at once
// the server's for now.
func previewFailed(w http.ResponseWriter, err error) {
	var status linkpreview.StatusError
	switch {
	case errors.Is(err, linkpreview.ErrAddress), errors.Is(err, linkpreview.ErrInternal):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, linkpreview.ErrBusy):
		w.Header().Set("Retry-After", "5")
		writeError(w, http.StatusServiceUnavailable, err.Error())
	case errors.As(err, &status), errors.Is(err, linkpreview.ErrNotPage), errors.Is(err, linkpreview.ErrUnreachable):
		writeError(w, http.StatusBadGateway, err.Error())
	default:
		writeError(w, http.StatusBadGateway, linkpreview.ErrUnreachable.Error())
	}
}

func (h itemHandlers) preview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if !decode(w, r, &req) {
		return
	}
	if h.d.LinkPreviews == nil {
		writeError(w, http.StatusConflict, errPreviewsOff)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), previewTimeout())
	defer cancel()
	p, err := h.d.LinkPreviews.reader.Read(ctx, req.URL, false)
	if err != nil {
		previewFailed(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// pageProps are the values of a page for the fields of td that take
// them: text fields site, description and author, and a date field
// published. A value the page does not give is left out.
func pageProps(td item.TypeDef, p linkpreview.Page) item.Props {
	values := map[string]string{"site": p.Site, "description": p.Description, "author": p.Author}
	out := item.Props{}
	for _, f := range td.Fields {
		switch v := values[f.Name]; {
		case f.Kind == item.KindText && v != "":
			out[f.Name] = v
		case f.Kind == item.KindDate && f.Name == "published" && p.Published != "":
			if d, err := time.Parse("2006-01-02", p.Published); err == nil {
				out[f.Name] = d
			}
		}
	}
	return out
}

// quoteBlocks makes the text one keeps into quotes, a paragraph each.
func quoteBlocks(text string) []item.Block {
	var out []item.Block
	for _, part := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, item.Block{Kind: item.Quote, Text: part})
		}
	}
	return out
}

// addressTitle names a page by its address when it names itself not.
func addressTitle(address string) string {
	u, err := linkpreview.WebAddress(address)
	if err != nil {
		return address
	}
	t := strings.TrimPrefix(u.Hostname(), "www.") + strings.TrimSuffix(u.EscapedPath(), "/")
	return t
}

// pictureFile names a page's picture by its format.
func pictureFile(data []byte) string {
	ext := map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/gif": ".gif", "image/webp": ".webp"}[http.DetectContentType(data)]
	return pictureName + ext
}

// currentPicture is the item's newest live picture named as a page's.
func currentPicture(it item.Item) *item.Attachment {
	for i := len(it.Attachments) - 1; i >= 0; i-- {
		a := it.Attachments[i]
		if !a.Deleted && item.IsPicture(a) && strings.HasPrefix(a.Name, pictureName+".") {
			return &it.Attachments[i]
		}
	}
	return nil
}

// keepPicture stores a page's picture as a file of the item, a new
// version of prev when given, and records it as the caller. Nothing new
// is stored at critical storage (ADR-0163): StoreAttachment refuses.
func (h itemHandlers) keepPicture(r *http.Request, id item.ID, data []byte, prev *item.Attachment) error {
	a, err := h.d.DB.StoreAttachment(id, bytes.NewReader(data), budget.Get().AttachmentBytes)
	if err != nil {
		return err
	}
	a.Name = pictureFile(data)
	if prev != nil {
		a.Source = &item.Source{Item: id, Attachment: prev.ID}
	}
	err = h.d.DB.Write(func(s *item.Store) error {
		_, err := s.As(who(r)).AddAttachment(id, a)
		return err
	})
	if err != nil {
		h.d.DB.DiscardAttachment(id, a.ID)
	}
	return err
}

func (h itemHandlers) saveWebLink(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL    string  `json:"url"`
		Space  string  `json:"space"`
		Parent item.ID `json:"parent"`
		Title  string  `json:"title"`
		Quote  string  `json:"quote"`
		Text   bool    `json:"text"`
	}
	if !decode(w, r, &req) {
		return
	}
	address := strings.TrimSpace(req.URL)
	if _, err := linkpreview.WebAddress(address); err != nil {
		writeError(w, http.StatusBadRequest, "url: "+err.Error())
		return
	}
	// Who may not create there learns so before any page is read.
	var check error
	h.d.DB.Read(func(s *item.Store) { check = s.As(who(r)).MayCreateIn(req.Space, req.Parent) })
	if check != nil {
		h.fail(w, check)
		return
	}
	var page linkpreview.Page
	var picture []byte
	var problems []string
	if lp := h.d.LinkPreviews; lp != nil {
		ctx, cancel := context.WithTimeout(r.Context(), previewTimeout())
		defer cancel()
		var err error
		if page, err = lp.reader.Read(ctx, address, req.Text); err != nil {
			problems = append(problems, err.Error())
		} else if page.Image != "" {
			if picture, err = lp.reader.Picture(ctx, page.Image); err != nil {
				problems = append(problems, problemNoPicture+err.Error())
			}
		}
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = page.Title
	}
	if title == "" {
		title = addressTitle(address)
	}
	blocks := quoteBlocks(req.Quote)
	blocks = append(blocks, page.Text...)
	var created item.Item
	err := h.d.DB.Write(func(s *item.Store) error {
		props := item.Props{urlField: address}
		td, _ := item.TemplateType(item.WebLinkType)
		if cur := s.Type(item.WebLinkType); cur.Name != "" {
			td = cur
		}
		for k, v := range pageProps(td, page) {
			props[k] = v
		}
		n := item.NewItem{Space: req.Space, Parent: req.Parent, Type: item.WebLinkType, Title: title, Properties: props}
		if len(blocks) > 0 {
			n.Body = &item.Doc{Blocks: blocks}
		}
		var err error
		created, err = s.As(who(r)).CreateFromTemplate(n)
		return err
	})
	if err != nil {
		h.fail(w, err)
		return
	}
	if picture != nil {
		if err := h.keepPicture(r, created.ID, picture, nil); err != nil {
			problems = append(problems, problemNoPicture+err.Error())
		}
	}
	res := map[string]any{"item": h.asRead(r, created.ID)}
	if len(problems) > 0 {
		res["problem"] = strings.Join(problems, "; ")
	}
	writeJSON(w, http.StatusCreated, res)
}

// asRead is an item as the caller reads it now.
func (h itemHandlers) asRead(r *http.Request, id item.ID) itemJSON {
	var out itemJSON
	h.d.DB.Read(func(s *item.Store) {
		x := s.As(who(r))
		if it, err := x.Get(id); err == nil {
			out = withSecrets(x, toItemJSON(it))
		}
	})
	return out
}

// webLinkOf is the item a request names, as the caller reads it, with
// its type and the address of its url field; it must be editable.
func (h itemHandlers) webLinkOf(r *http.Request) (item.Item, item.TypeDef, string, error) {
	var it item.Item
	var td item.TypeDef
	var err error
	h.d.DB.Read(func(s *item.Store) {
		x := s.As(who(r))
		if it, err = resolveItem(x, r.PathValue("ref")); err != nil {
			return
		}
		if !x.CanEdit(it.ID) {
			err = item.ErrForbidden
			return
		}
		td = s.Type(it.Type)
	})
	if err != nil {
		return it, td, "", err
	}
	address, _ := it.Properties[urlField].(string)
	if !slices.ContainsFunc(td.Fields, func(f item.FieldDef) bool { return f.Name == urlField && f.Kind == item.KindURL }) || address == "" {
		return it, td, "", badInput(errNoAddress)
	}
	return it, td, address, nil
}

func (h itemHandlers) readWebLinkAgain(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text bool `json:"text"`
	}
	if !decode(w, r, &req) {
		return
	}
	lp := h.d.LinkPreviews
	if lp == nil {
		writeError(w, http.StatusConflict, errPreviewsOff)
		return
	}
	it, td, address, err := h.webLinkOf(r)
	if err != nil {
		h.fail(w, err)
		return
	}
	// The text is taken only while no one edits it live (ADR-0010).
	if req.Text && h.d.Collab != nil && h.d.Collab.Busy(it.ID) {
		writeError(w, http.StatusConflict, errBeingEdited)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), previewTimeout())
	defer cancel()
	page, err := lp.reader.Read(ctx, address, req.Text)
	if err != nil {
		previewFailed(w, err)
		return
	}
	var problems []string
	pictured := false
	if page.Image != "" {
		data, err := lp.reader.Picture(ctx, page.Image)
		prev := currentPicture(it)
		sum := sha256.Sum256(data)
		switch {
		case err != nil:
			problems = append(problems, problemNoPicture+err.Error())
		case prev != nil && prev.SHA256 == hex.EncodeToString(sum[:]):
			// The same picture is kept once.
		default:
			if err := h.keepPicture(r, it.ID, data, prev); err != nil {
				problems = append(problems, problemNoPicture+err.Error())
			} else {
				pictured = true
			}
		}
	}
	changed := []string{}
	err = h.d.DB.Write(func(s *item.Store) error {
		x := s.As(who(r))
		cur, err := x.Get(it.ID)
		if err != nil {
			return err
		}
		props := pageProps(td, page)
		for _, f := range td.Fields {
			v, ok := props[f.Name]
			if !ok || item.FormatValue(v) == item.FormatValue(cur.Properties[f.Name]) {
				continue
			}
			if err := x.SetProperty(it.ID, f.Name, v); err != nil {
				return err
			}
			changed = append(changed, f.Name)
		}
		if req.Text && len(page.Text) > 0 {
			blocks := slices.Clone(cur.Body.Blocks)
			if len(blocks) > 0 {
				blocks = append(blocks, item.Block{Kind: item.Divider})
			}
			return x.CommitBody(it.ID, item.Doc{Blocks: append(blocks, page.Text...)})
		}
		return nil
	})
	if err != nil {
		h.fail(w, err)
		return
	}
	if h.d.Collab != nil {
		h.d.Collab.Revalidate()
	}
	res := map[string]any{"item": h.asRead(r, it.ID), "changed": changed, "picture": pictured}
	if len(problems) > 0 {
		res["problem"] = strings.Join(problems, "; ")
	}
	writeJSON(w, http.StatusOK, res)
}

// appendBlocks adds blocks at the end of a body in one step (ADR-0207):
// the body as the caller reads it, extended, is committed under the
// store's lock, so nothing written in between is lost. Who does not read
// every protected passage cannot change the text (ADR-0088).
func (h itemHandlers) appendBlocks(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Blocks []blockJSON `json:"blocks"`
	}
	if !decode(w, r, &req) {
		return
	}
	if len(req.Blocks) == 0 {
		writeError(w, http.StatusBadRequest, "blocks: at least one block is added")
		return
	}
	add, err := toDoc(req.Blocks, nil)
	if err != nil {
		h.fail(w, err)
		return
	}
	if id, err := h.resolveID(r); err == nil && h.d.Collab != nil && h.d.Collab.Busy(id) {
		writeError(w, http.StatusConflict, errBeingEdited)
		return
	}
	h.change(w, r, func(x *item.Session, _ *item.Store, it item.Item) error {
		return x.CommitBody(it.ID, item.Doc{Blocks: append(slices.Clone(it.Body.Blocks), add.Blocks...)})
	})
}
