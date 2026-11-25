// Package shopcat writes a catalogue maintained in the wiki as the
// catalogue document a process engine imports
// (ADR-0251): a catalogue is an item
// of type shop-catalogue, its products are the items of type shop-product
// below it, and the structure and precedence between them are relations
// made by hand. The engine stays the system of record for what is
// published from it, ordered and held; the wiki is where the catalogue is
// written, discussed and versioned.
//
// The package reads items and writes a document; it neither reads the
// store nor calls an engine, so what a publish sends is a function of the
// items alone and can be shown before it is sent.
package shopcat

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/dbuchs7/orbithek/internal/item"
)

// The types a catalogue is written in. They are installation types,
// defined once by an administrator (Types), so every room may hold
// catalogues.
const (
	CatalogueType = "shop-catalogue"
	ProductType   = "shop-product"
)

// The states of a product, the engine's own: only an active product is
// offered; a draft is written but not offered, a withdrawn product is
// kept for the orders that name it.
const (
	StateDraft     = "draft"
	StateActive    = "active"
	StateWithdrawn = "withdrawn"
)

// Languages are the languages a name may be given in beside the title.
var Languages = []string{"de", "fr", "it", "en"}

// The relation types that become edges of the catalogue, the engine's own
// words: composition (the part comes with the whole), aggregation (the
// part is an option beside it), requires (provisioned only after) and
// excludes (never held together). For the first two the relation goes from
// the whole to the part, for requires from the dependent product to its
// precondition.
var EdgeKinds = []string{"composition", "aggregation", "requires", "excludes"}

// Approvals are the kinds of approval a product may ask for.
var Approvals = []string{"none", "superior", "role", "fixed"}

// Types are the definitions of the two types, as an administrator sets
// them up.
func Types() []item.TypeDef {
	names := func() []item.FieldDef {
		out := []item.FieldDef{}
		for _, l := range Languages {
			out = append(out, item.FieldDef{Name: "name-" + l, Kind: item.KindText})
		}
		return out
	}
	catalogue := item.TypeDef{Name: CatalogueType, Body: true, Fields: append([]item.FieldDef{
		{Name: "engine", Kind: item.KindText, Required: true},
		{Name: "catalogue-id", Kind: item.KindText, Unique: true},
		{Name: "languages", Kind: item.KindText, Required: true},
		{Name: "rank", Kind: item.KindNumber, Required: true},
		{Name: "audience", Kind: item.KindText},
	}, names()...)}
	product := item.TypeDef{Name: ProductType, Body: true, Fields: append([]item.FieldDef{
		{Name: "product-id", Kind: item.KindText, Unique: true},
		{Name: "provision-process", Kind: item.KindText, Recommended: true},
		{Name: "deprovision-process", Kind: item.KindText, Recommended: true},
		{Name: "approval", Kind: item.KindEnum, Values: Approvals},
		{Name: "approver", Kind: item.KindText},
		{Name: "order-form", Kind: item.KindText},
		{Name: "price", Kind: item.KindText},
		{Name: "category", Kind: item.KindText},
		{Name: "multiple", Kind: item.KindBool},
		{Name: "max-days", Kind: item.KindNumber},
		{Name: "orderable-from", Kind: item.KindDate},
		{Name: "orderable-until", Kind: item.KindDate},
		{Name: "eligible", Kind: item.KindText},
	}, names()...), Workflow: &item.Workflow{
		Initial: StateDraft,
		States:  []string{StateDraft, StateActive, StateWithdrawn},
		Transitions: []item.Transition{
			{Name: "activate", From: []string{StateDraft, StateWithdrawn}, To: StateActive},
			{Name: "withdraw", From: []string{StateActive}, To: StateWithdrawn},
			{Name: "back to draft", From: []string{StateActive, StateWithdrawn}, To: StateDraft},
		},
	}}
	return []item.TypeDef{catalogue, product}
}

// Document is the catalogue document the engine imports (its
// catalog.Document): the catalogue, its products, and whether to publish
// a release of it.
type Document struct {
	Catalogs []Catalog `json:"catalogs"`
	Products []Product `json:"products"`
	Publish  bool      `json:"publish,omitempty"`
}

// Catalog is the catalogue as the engine keeps it.
type Catalog struct {
	ID        string            `json:"id"`
	Texts     map[string]string `json:"texts"`
	Rank      int               `json:"rank"`
	Languages []string          `json:"languages"`
	Items     []string          `json:"items"`
	Groups    []string          `json:"groups,omitempty"`
	Edges     []Edge            `json:"edges,omitempty"`
	// MaintainedIn says where the catalogue is maintained: the engine
	// refuses changes made elsewhere while it says so.
	MaintainedIn *Maintainer `json:"maintainedIn,omitempty"`
}

// Maintainer is where a catalogue is maintained, as the document says it:
// the address of its page.
type Maintainer struct {
	URL string `json:"url,omitempty"`
}

// Edge links two products of the catalogue.
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

// Approval is who approves an order of a product.
type Approval struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref,omitempty"`
}

// Lifecycle is when a product may be ordered, in seconds since 1970.
type Lifecycle struct {
	From  int64 `json:"from,omitempty"`
	Until int64 `json:"until,omitempty"`
}

// Product is a product as the engine keeps it.
type Product struct {
	ID                 string            `json:"id"`
	HomeCatalog        string            `json:"homeCatalog"`
	State              string            `json:"state"`
	Texts              map[string]string `json:"texts"`
	Descriptions       map[string]string `json:"descriptions,omitempty"`
	Lifecycle          Lifecycle         `json:"lifecycle,omitempty"`
	Approval           Approval          `json:"approval"`
	ProvisionProcess   string            `json:"provisionProcess"`
	DeprovisionProcess string            `json:"deprovisionProcess"`
	ConfigForm         string            `json:"configForm,omitempty"`
	Price              string            `json:"price,omitempty"`
	Category           string            `json:"category,omitempty"`
	MultipleAllowed    bool              `json:"multipleAllowed,omitempty"`
	MaxDays            int               `json:"maxDays,omitempty"`
	Eligible           []string          `json:"eligible,omitempty"`
	Keywords           []string          `json:"keywords,omitempty"`
}

// Input is what a document is written from: the catalogue item, the live
// product items below it, the relations made by hand between them, the
// Markdown of each product's body by item ID, and the address of the
// catalogue's page.
type Input struct {
	Catalogue    item.Item
	Products     []item.Item
	Relations    []item.Relation
	Descriptions map[item.ID]string
	URL          string
}

// Problem is a reason a document cannot be written, naming the item it is
// about by its key.
type Problem struct {
	Item    string `json:"item"`
	Problem string `json:"problem"`
}

// documentID is what an ID at the engine may be: a word a URL carries.
var documentID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)

// language is a language tag as the engine takes it.
var language = regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)

// CatalogueID is the engine's ID of a catalogue item: its field
// catalogue-id, or one made of its item ID, which never changes (D2).
func CatalogueID(it item.Item) string {
	return idOf(it, "catalogue-id")
}

// ProductID is the engine's ID of a product item, as CatalogueID.
func ProductID(it item.Item) string {
	return idOf(it, "product-id")
}

func idOf(it item.Item, field string) string {
	if v := strings.TrimSpace(text(it, field)); v != "" {
		return v
	}
	return "ot-" + string(it.ID)
}

// Engine is the engine a catalogue item is published to.
func Engine(it item.Item) string { return strings.TrimSpace(text(it, "engine")) }

func text(it item.Item, field string) string {
	if v, ok := it.Properties[field]; ok && v != nil {
		return item.FormatValue(v)
	}
	return ""
}

// list reads a field of words separated by commas, without blanks and
// duplicates, in their order.
func list(it item.Item, field string) []string {
	var out []string
	for _, w := range strings.Split(text(it, field), ",") {
		if w = strings.TrimSpace(w); w != "" && !slices.Contains(out, w) {
			out = append(out, w)
		}
	}
	return out
}

func number(it item.Item, field string) (int, bool) {
	switch v := it.Properties[field].(type) {
	case float64:
		return int(v), v == float64(int(v))
	case int:
		return v, true
	}
	return 0, false
}

func unix(it item.Item, field string) int64 {
	if t, ok := it.Properties[field].(time.Time); ok {
		return t.Unix()
	}
	return 0
}

// texts are an item's names: the title in the first language, and each
// language's name field where it is set.
func texts(it item.Item, langs []string) map[string]string {
	out := map[string]string{}
	for _, l := range Languages {
		if v := strings.TrimSpace(text(it, "name-"+l)); v != "" {
			out[l] = v
		}
	}
	if len(langs) > 0 && strings.TrimSpace(it.Title) != "" {
		if _, ok := out[langs[0]]; !ok {
			out[langs[0]] = strings.TrimSpace(it.Title)
		}
	}
	return out
}

// Write writes the document of a catalogue, to be published when publish
// is set, or answers every problem that stops it at once. What the engine
// checks besides — bindings to processes it runs, approvers it knows, a
// rank another catalogue holds — the engine answers when it is asked, with
// a dry run first.
func Write(in Input, publish bool) (Document, []Problem) {
	var problems []Problem
	add := func(it item.Item, format string, a ...any) {
		problems = append(problems, Problem{Item: it.Key, Problem: fmt.Sprintf(format, a...)})
	}
	c := in.Catalogue
	if c.Type != CatalogueType {
		add(c, "is of type %s, not %s", c.Type, CatalogueType)
		return Document{}, problems
	}
	cat := Catalog{ID: CatalogueID(c), Languages: list(c, "languages"), Groups: list(c, "audience"), Items: []string{}}
	if !documentID.MatchString(cat.ID) {
		add(c, "catalogue-id %q: an ID has letters, digits, '.', '_' and '-', at most 100", cat.ID)
	}
	if len(cat.Languages) == 0 {
		add(c, "languages: name at least one language, such as de, en")
	}
	for _, l := range cat.Languages {
		if !language.MatchString(l) {
			add(c, "languages: %q is no language tag", l)
		}
	}
	rank, ok := number(c, "rank")
	if !ok || rank < 1 {
		add(c, "rank: a whole number from 1, unique among the engine's catalogues")
	}
	cat.Rank = rank
	cat.Texts = texts(c, cat.Languages)
	if len(cat.Texts) == 0 {
		add(c, "has no name")
	}
	// The engine takes the address of the catalogue's page only as https;
	// a server reached by plain http publishes without it, and the engine
	// still knows the catalogue is maintained here from the grant.
	if strings.HasPrefix(in.URL, "https://") {
		cat.MaintainedIn = &Maintainer{URL: in.URL}
	}

	ids := map[item.ID]string{}
	seen := map[string]string{}
	var products []Product
	for _, it := range in.Products {
		if it.Type != ProductType {
			continue
		}
		p := Product{ID: ProductID(it), HomeCatalog: cat.ID, State: it.State, Texts: texts(it, cat.Languages),
			ProvisionProcess: strings.TrimSpace(text(it, "provision-process")), DeprovisionProcess: strings.TrimSpace(text(it, "deprovision-process")),
			ConfigForm: strings.TrimSpace(text(it, "order-form")), Price: strings.TrimSpace(text(it, "price")), Category: strings.TrimSpace(text(it, "category")),
			Lifecycle: Lifecycle{From: unix(it, "orderable-from"), Until: unix(it, "orderable-until")},
			Eligible:  list(it, "eligible"), Keywords: slices.Clone(it.Labels)}
		if b, ok := it.Properties["multiple"].(bool); ok {
			p.MultipleAllowed = b
		}
		if n, ok := number(it, "max-days"); ok && n > 0 {
			p.MaxDays = n
		}
		p.Approval = Approval{Kind: strings.TrimSpace(text(it, "approval")), Ref: strings.TrimSpace(text(it, "approver"))}
		if p.Approval.Kind == "" {
			p.Approval.Kind = "none"
		}
		if d := strings.TrimSpace(in.Descriptions[it.ID]); d != "" && len(cat.Languages) > 0 {
			p.Descriptions = map[string]string{cat.Languages[0]: d}
		}
		switch {
		case !documentID.MatchString(p.ID):
			add(it, "product-id %q: an ID has letters, digits, '.', '_' and '-', at most 100", p.ID)
		case seen[p.ID] != "":
			add(it, "product-id %q is %s's as well", p.ID, seen[p.ID])
		}
		seen[p.ID] = it.Key
		if len(p.Texts) == 0 {
			add(it, "has no name")
		}
		if (p.Approval.Kind == "role" || p.Approval.Kind == "fixed") && p.Approval.Ref == "" {
			add(it, "approver: an approval by %s names who approves", p.Approval.Kind)
		}
		if p.Lifecycle.From != 0 && p.Lifecycle.Until != 0 && p.Lifecycle.Until < p.Lifecycle.From {
			add(it, "orderable-until is before orderable-from")
		}
		if p.State == StateActive {
			if p.ProvisionProcess == "" || p.DeprovisionProcess == "" {
				add(it, "an active product names the process that provisions it and the one that takes it back")
			}
			cat.Items = append(cat.Items, p.ID)
		}
		ids[it.ID] = p.ID
		products = append(products, p)
	}
	// Edges join offered products alone: an edge to a draft would name
	// a product the catalogue does not hold.
	for _, r := range in.Relations {
		from, to := ids[r.From], ids[r.To]
		if r.Derived || !slices.Contains(cat.Items, from) || !slices.Contains(cat.Items, to) || !slices.Contains(EdgeKinds, r.Type) {
			continue
		}
		cat.Edges = append(cat.Edges, Edge{From: from, To: to, Kind: r.Type})
	}
	slices.SortFunc(cat.Edges, func(a, b Edge) int {
		return strings.Compare(a.Kind+"\x00"+a.From+"\x00"+a.To, b.Kind+"\x00"+b.From+"\x00"+b.To)
	})
	if len(problems) > 0 {
		return Document{}, problems
	}
	if products == nil {
		products = []Product{}
	}
	return Document{Catalogs: []Catalog{cat}, Products: products, Publish: publish}, nil
}
