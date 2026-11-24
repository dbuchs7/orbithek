package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"

	"github.com/dbuchs7/orbithek/internal/export"
	"github.com/dbuchs7/orbithek/internal/item"
	"github.com/dbuchs7/orbithek/internal/shopcat"
	"github.com/dbuchs7/orbithek/internal/site"
)

// shopCatalogueRoutes maintain a catalogue of a process engine's shop in
// the wiki and publish it there (ADR-draft-a-shop-and-catalogues-from-an-engine):
// the catalogue and its products are items, written, discussed and
// versioned here; publishing sends them as the engine's catalogue document
// with the publisher's own grant, so the engine decides whether they may
// maintain it, and marks the catalogue as maintained here.
func shopCatalogueRoutes(d Deps) []Route {
	log := d.Log
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	h := shopCatalogueHandlers{shopHandlers: shopHandlers{d: d}, items: itemHandlers{d: d, log: log}, auth: authHandlers{d: d}}
	return []Route{
		{Method: http.MethodPost, Pattern: "/api/shop/types", Summary: "Administrators: define the installation types a shop's catalogue is written in, shop-catalogue and shop-product, where they are not yet defined; a type of that name already defined stays as it is", Handler: http.HandlerFunc(h.defineTypes)},
		{Method: http.MethodGet, Pattern: "/api/items/{ref}/shop/document", Summary: "The catalogue document a shop-catalogue item would be published as — the catalogue and the shop-product items below it that one reads — or every problem that stops it", Handler: http.HandlerFunc(h.document)},
		{Method: http.MethodPost, Pattern: "/api/items/{ref}/shop/publish", Summary: "Publish a shop-catalogue item at its engine with one's own grant of the shop: the engine imports the catalogue and its products, replacing what it held of them, and publishes a release; {\"dryRun\": true} asks the engine what it would do and changes nothing. Who may edit the item may publish it; the engine decides whether one maintains catalogues there", Handler: http.HandlerFunc(h.publish)},
	}
}

type shopCatalogueHandlers struct {
	shopHandlers
	items itemHandlers
	auth  authHandlers
}

func (h shopCatalogueHandlers) defineTypes(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFrom(r.Context())
	if !h.auth.isAdmin(p) {
		writeError(w, http.StatusForbidden, "only administrators define the installation's types")
		return
	}
	defined, existing := []string{}, []string{}
	err := h.d.DB.Write(func(s *item.Store) error {
		for _, td := range shopcat.Types() {
			if slices.ContainsFunc(s.Types(), func(t item.TypeDef) bool { return t.Name == td.Name }) {
				existing = append(existing, td.Name)
				continue
			}
			if err := s.DefineType(string(p.User), td); err != nil {
				return err
			}
			defined = append(defined, td.Name)
		}
		return nil
	})
	if err != nil {
		h.items.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]string{"defined": defined, "existing": existing})
}

// below reports whether an item is below another in the page tree.
func below(x *item.Session, it item.Item, ancestor item.ID) bool {
	for range 100 {
		if it.Parent == "" {
			return false
		}
		if it.Parent == ancestor {
			return true
		}
		parent, err := x.Get(it.Parent)
		if err != nil {
			return false
		}
		it = parent
	}
	return false
}

// publicBlocks are the blocks of a body everyone may read: a protected
// passage stays in the wiki (ADR-0088), since what a shop shows is read by
// everyone the catalogue is offered to.
func publicBlocks(blocks []item.Block) []item.Block {
	out := make([]item.Block, 0, len(blocks))
	for _, b := range blocks {
		if b.Secret == "" {
			out = append(out, b)
		}
	}
	return out
}

// gather reads what a catalogue item is published from, as the caller
// reads it (D10): the catalogue, which the caller must be able to edit,
// the shop-product items below it, their relations made by hand and the
// readable text of their bodies.
func (h shopCatalogueHandlers) gather(r *http.Request) (shopcat.Input, error) {
	var in shopcat.Input
	var err error
	h.d.DB.Read(func(s *item.Store) {
		x := s.As(who(r))
		var c item.Item
		if c, err = resolveItem(x, r.PathValue("ref")); err != nil {
			return
		}
		if c.Type != shopcat.CatalogueType {
			err = badInput("%s is no %s", c.Key, shopcat.CatalogueType)
			return
		}
		if !x.MayEdit(c.ID) {
			err = item.ErrForbidden
			return
		}
		in = shopcat.Input{Catalogue: c, Descriptions: map[item.ID]string{}, URL: origin(r) + "/#/item/" + string(c.ID)}
		for _, p := range x.Find(item.Query{Type: shopcat.ProductType, Space: c.Space}) {
			if !below(x, p, c.ID) {
				continue
			}
			in.Products = append(in.Products, p)
			in.Descriptions[p.ID] = export.BlocksMarkdown(publicBlocks(p.Body.Blocks))
			rels, rerr := x.Relations(p.ID, item.Outgoing)
			if rerr == nil {
				in.Relations = append(in.Relations, rels...)
			}
		}
		slices.SortFunc(in.Products, func(a, b item.Item) int { return compareKeys(a.Key, b.Key) })
	})
	return in, err
}

// compareKeys orders keys as their numbers do, IT-2 before IT-10.
func compareKeys(a, b string) int {
	if len(a) != len(b) {
		return len(a) - len(b)
	}
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func (h shopCatalogueHandlers) document(w http.ResponseWriter, r *http.Request) {
	in, err := h.gather(r)
	if err != nil {
		h.items.fail(w, err)
		return
	}
	doc, problems := shopcat.Write(in, true)
	out := map[string]any{"engine": shopcat.Engine(in.Catalogue), "problems": problems}
	if problems == nil {
		out["document"] = doc
		out["problems"] = []shopcat.Problem{}
	}
	writeJSON(w, http.StatusOK, out)
}

func (h shopCatalogueHandlers) publish(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DryRun bool `json:"dryRun"`
	}
	if r.ContentLength != 0 {
		if err := readJSON(w, r, &req); err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, errUnsupportedMedia) {
				status = http.StatusUnsupportedMediaType
			}
			writeError(w, status, err.Error())
			return
		}
	}
	in, err := h.gather(r)
	if err != nil {
		h.items.fail(w, err)
		return
	}
	doc, problems := shopcat.Write(in, true)
	if problems != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "the catalogue cannot be published as it is", "problems": problems})
		return
	}
	e, ok := h.d.DB.Site.Engine(shopcat.Engine(in.Catalogue))
	if !ok || e.Kind != site.EngineAtlas {
		writeError(w, http.StatusUnprocessableEntity, "engine: no process engine "+shopcat.Engine(in.Catalogue)+" is set up")
		return
	}
	body, err := json.Marshal(doc)
	if err != nil {
		h.items.fail(w, err)
		return
	}
	// The catalogue is maintained here: what the engine holds of it is
	// replaced, never decided product by product.
	path := "/api/v1/catalogs/import?overwrite=*"
	if req.DryRun {
		path += "&dryRun=true"
	}
	a, ok := h.call(w, r, e, http.MethodPost, path, body, "")
	if !ok {
		return
	}
	result := json.RawMessage(a.Body)
	if !json.Valid(result) {
		writeError(w, http.StatusBadGateway, "the engine answered no JSON")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"engine": e.ID, "dryRun": req.DryRun, "catalogue": doc.Catalogs[0].ID, "result": result})
}
