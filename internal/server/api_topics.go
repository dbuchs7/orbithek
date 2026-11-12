package server

import (
	"net/http"
	"strings"

	"github.com/dbuchs7/orbithek/internal/item"
)

// topicRoutes are the routes of a label's page and its order set by hand
// (ADR-0202).
func topicRoutes(h itemHandlers) []Route {
	return []Route{
		{Method: http.MethodGet, Pattern: "/api/labels/{label}/page", Summary: "The page of a label: the first readable item of the type topic whose field label names it, in every room, by the order they were made (page, null without one); order lists the items carrying the label that the caller reads, in the order set by hand on the page, as far as it names them; orderable says whether the caller may change it (ADR-0202)", Handler: http.HandlerFunc(h.labelPage)},
		{Method: http.MethodPost, Pattern: "/api/labels/{label}/page", Summary: "Describe a label: make its page in a room (space; title, the label without one), an item of the type topic, which is defined from its template on first use; who may create in the room may (ADR-0202)", Handler: http.HandlerFunc(h.describeLabel)},
		{Method: http.MethodPut, Pattern: "/api/items/{ref}/order", Summary: "Move items within the order set by hand of a label's page ({ref}, of the type topic): items (IDs, in the order they are to have) go right before before, or to the end without it; each must carry the page's label and be readable; items the caller does not read keep their places; who may edit the page may (ADR-0202)", Handler: http.HandlerFunc(h.placeItems)},
		{Method: http.MethodDelete, Pattern: "/api/items/{ref}/order", Summary: "Forget the order set by hand of a label's page: its items are listed in the order they were made again; who may edit the page may (ADR-0202)", Handler: http.HandlerFunc(h.clearItemOrder)},
	}
}

// labelPage answers the page of a label the caller reads, with the places
// of the items set by hand that the caller reads (D10).
func (h itemHandlers) labelPage(w http.ResponseWriter, r *http.Request) {
	label, ok := item.NormalLabel(r.PathValue("label"))
	if !ok {
		writeError(w, http.StatusBadRequest, "not a label: lower-case letters and digits joined by -, _ or ., at most 64")
		return
	}
	h.read(w, r, func(x *item.Session) (any, error) {
		res := map[string]any{"label": label, "page": nil, "order": []item.ID{}, "orderable": false}
		t, found := x.TopicOf(label)
		if !found {
			return res, nil
		}
		order, err := x.ItemOrder(t.ID)
		if err != nil {
			return nil, err
		}
		res["page"] = toItemJSON(t)
		res["order"] = nonNilSlice(order)
		res["orderable"] = x.MayEdit(t.ID)
		return res, nil
	})
}

// describeLabel makes the page of a label in a room (ADR-0202).
func (h itemHandlers) describeLabel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Space string `json:"space"`
		Title string `json:"title"`
	}
	if !decode(w, r, &req) {
		return
	}
	h.write(w, r, http.StatusCreated, func(x *item.Session, _ *item.Store) (any, error) {
		it, err := x.DescribeLabel(req.Space, r.PathValue("label"), strings.TrimSpace(req.Title))
		if err != nil {
			return nil, err
		}
		return map[string]itemJSON{"item": toItemJSON(it)}, nil
	})
}

// placeItems moves items within the order set by hand of a label's page.
func (h itemHandlers) placeItems(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Items  []item.ID `json:"items"`
		Before item.ID   `json:"before"`
	}
	if !decode(w, r, &req) {
		return
	}
	h.write(w, r, http.StatusOK, func(x *item.Session, _ *item.Store) (any, error) {
		it, err := resolveItem(x, r.PathValue("ref"))
		if err != nil {
			return nil, err
		}
		if err := x.PlaceItems(it.ID, req.Items, req.Before); err != nil {
			return nil, err
		}
		return map[string]bool{"ordered": true}, nil
	})
}

// clearItemOrder forgets the order set by hand of a label's page.
func (h itemHandlers) clearItemOrder(w http.ResponseWriter, r *http.Request) {
	h.write(w, r, http.StatusOK, func(x *item.Session, _ *item.Store) (any, error) {
		it, err := resolveItem(x, r.PathValue("ref"))
		if err != nil {
			return nil, err
		}
		if err := x.ClearItemOrder(it.ID); err != nil {
			return nil, err
		}
		return map[string]bool{"ordered": false}, nil
	})
}
