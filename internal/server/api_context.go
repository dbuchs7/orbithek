package server

import (
	"net/http"
	"time"

	"github.com/dbuchs7/orbithek/internal/budget"
	"github.com/dbuchs7/orbithek/internal/item"
)

// contextRoutes are the context of an item (ADR-0205): what refers to it,
// with the sentence it stands in, what shares its labels, and the daily
// notes that name it, each list within the budget ContextEntries.
func contextRoutes(h itemHandlers) []Route {
	return []Route{
		{Method: http.MethodGet, Pattern: "/api/items/{ref}/context", Summary: "The context of an item: the readable items that refer to it by relations, mentions, embeds or links, each with the sentence it stands in; those that share its labels; and the daily notes that name it; each list truncated beyond the budget ContextEntries (ADR-0205)", Handler: http.HandlerFunc(h.itemContext)},
	}
}

// contextItemJSON names an item of the context without its content.
type contextItemJSON struct {
	ID    item.ID `json:"id"`
	Key   string  `json:"key"`
	Title string  `json:"title"`
	Type  string  `json:"type"`
	Space string  `json:"space"`
	State string  `json:"state,omitempty"`
}

func toContextItem(it item.Item) contextItemJSON {
	return contextItemJSON{ID: it.ID, Key: it.Key, Title: it.Title, Type: it.Type, Space: it.Space, State: it.State}
}

type contextRefJSON struct {
	Item     contextItemJSON `json:"item"`
	Kind     string          `json:"kind"`
	Derived  bool            `json:"derived,omitempty"`
	Sentence string          `json:"sentence,omitempty"`
	Count    int             `json:"count"`
}

type contextSharedJSON struct {
	Item   contextItemJSON `json:"item"`
	Labels []string        `json:"labels"`
}

type contextDailyJSON struct {
	Item     contextItemJSON `json:"item"`
	Date     string          `json:"date"`
	Sentence string          `json:"sentence,omitempty"`
}

func (h itemHandlers) itemContext(w http.ResponseWriter, r *http.Request) {
	h.read(w, r, func(x *item.Session) (any, error) {
		it, err := resolveItem(x, r.PathValue("ref"))
		if err != nil {
			return nil, err
		}
		c, err := x.Context(it.ID, budget.Get().ContextEntries)
		if err != nil {
			return nil, err
		}
		refs, shared, daily := []contextRefJSON{}, []contextSharedJSON{}, []contextDailyJSON{}
		for _, ref := range c.References {
			refs = append(refs, contextRefJSON{Item: toContextItem(ref.Item), Kind: ref.Kind, Derived: ref.Derived, Sentence: ref.Sentence, Count: ref.Count})
		}
		for _, s := range c.Shared {
			shared = append(shared, contextSharedJSON{Item: toContextItem(s.Item), Labels: s.Labels})
		}
		for _, d := range c.Daily {
			daily = append(daily, contextDailyJSON{Item: toContextItem(d.Item), Date: d.Day.Format(time.DateOnly), Sentence: d.Sentence})
		}
		return map[string]any{"references": refs, "shared": shared, "daily": daily,
			"more": map[string]bool{"references": c.MoreReferences, "shared": c.MoreShared, "daily": c.MoreDaily}}, nil
	})
}
