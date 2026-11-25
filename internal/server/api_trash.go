package server

import (
	"net/http"
	"time"

	"github.com/dbuchs7/orbithek/internal/item"
)

// itemTrashRoutes list the items in a space's trash (ADR-0021).
func itemTrashRoutes(h itemHandlers) []Route {
	return []Route{
		{Method: http.MethodGet, Pattern: "/api/spaces/{space}/trash", Summary: "The items in a space's trash that the caller reads, one per trash root, the most recently trashed first, paged: who trashed it and when, how many readable items went with it, whether the caller may restore or purge it, and the trashed parent to restore first (waiting)", Handler: http.HandlerFunc(h.spaceTrash)},
	}
}

type trashEntryJSON struct {
	ID         item.ID   `json:"id"`
	Key        string    `json:"key"`
	Title      string    `json:"title"`
	Type       string    `json:"type"`
	By         string    `json:"by"`
	At         time.Time `json:"at"`
	Items      int       `json:"items"`
	MayRestore bool      `json:"mayRestore"`
	MayPurge   bool      `json:"mayPurge"`
	Waiting    string    `json:"waiting,omitempty"`
}

func (h itemHandlers) spaceTrash(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, ok := parseLimit(w, q)
	if !ok {
		return
	}
	h.read(w, r, func(x *item.Session) (any, error) {
		entries, err := x.TrashOf(r.PathValue("space"))
		if err != nil {
			return nil, err
		}
		page, res, err := paged(entries, func(e item.TrashEntry) item.ID { return e.Item.ID }, item.ID(q.Get("after")), limit)
		if err != nil {
			return nil, err
		}
		out := make([]trashEntryJSON, len(page))
		for i, e := range page {
			out[i] = trashEntryJSON{ID: e.Item.ID, Key: e.Item.Key, Title: e.Item.Title, Type: e.Item.Type, By: e.By, At: e.At,
				Items: e.Items, MayRestore: e.MayRestore, MayPurge: e.MayPurge, Waiting: e.Waiting}
		}
		res["items"] = out
		return res, nil
	})
}
