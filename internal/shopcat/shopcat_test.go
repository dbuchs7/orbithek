package shopcat

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dbuchs7/orbithek/internal/item"
)

func catalogue() item.Item {
	return item.Item{ID: "c1", Key: "IT-1", Type: CatalogueType, Title: "IT-Arbeitsplatz",
		Properties: item.Props{"engine": "atlas", "languages": "de, en, de", "rank": float64(10), "audience": "staff, , it", "name-en": "IT workplace"}}
}

func product(id, key, title, state string, props item.Props) item.Item {
	return item.Item{ID: item.ID(id), Key: key, Type: ProductType, Title: title, State: state, Properties: props}
}

// TestACatalogueIsWrittenAsTheEnginesDocument: a catalogue item and its
// product items become the engine's document, IDs made of item IDs where
// no field names one; names in every language given; an active product
// is offered, a draft written but not offered; hand-made relations of the
// engine's kinds between offered products become edges, others not; the
// body becomes the first language's description and labels keywords.
func TestACatalogueIsWrittenAsTheEnginesDocument(t *testing.T) {
	from := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	laptop := product("p1", "IT-2", "Laptop", StateActive, item.Props{"provision-process": "laptop-geben", "deprovision-process": "laptop-nehmen",
		"approval": "superior", "order-form": "laptop-form", "price": "CHF 1200", "category": "Hardware", "multiple": true, "max-days": float64(30),
		"orderable-from": from, "eligible": "staff", "name-fr": "Portable"})
	laptop.Labels = []string{"hardware", "mobil"}
	dock := product("p2", "IT-3", "Dockingstation", StateActive, item.Props{"product-id": "dock", "provision-process": "d1", "deprovision-process": "d2"})
	draft := product("p3", "IT-4", "Tablet", StateDraft, nil)
	page := item.Item{ID: "x", Key: "IT-5", Type: "page", Title: "Notes"}
	in := Input{Catalogue: catalogue(), Products: []item.Item{laptop, dock, draft, page},
		Relations: []item.Relation{
			{Type: "aggregation", From: "p1", To: "p2"},
			{Type: "requires", From: "p2", To: "p1"},
			{Type: "composition", From: "p1", To: "p3"}, // to a draft
			{Type: "blocks", From: "p1", To: "p2"},      // not the engine's
			{Type: "aggregation", From: "p1", To: "p2", Derived: true},
		},
		Descriptions: map[item.ID]string{"p1": "Ein **Laptop**.\n"}, URL: "https://wiki.example.org/#/item/IT-1"}
	doc, problems := Write(in, true)
	if problems != nil {
		t.Fatal(problems)
	}
	got, _ := json.Marshal(doc)
	want := `{"catalogs":[{"id":"ot-c1","texts":{"de":"IT-Arbeitsplatz","en":"IT workplace"},"rank":10,"languages":["de","en"],` +
		`"items":["ot-p1","dock"],"groups":["staff","it"],"edges":[{"from":"ot-p1","to":"dock","kind":"aggregation"},{"from":"dock","to":"ot-p1","kind":"requires"}],` +
		`"maintainedIn":{"url":"https://wiki.example.org/#/item/IT-1"}}],` +
		`"products":[{"id":"ot-p1","homeCatalog":"ot-c1","state":"active","texts":{"de":"Laptop","fr":"Portable"},"descriptions":{"de":"Ein **Laptop**."},` +
		`"lifecycle":{"from":1798761600},"approval":{"kind":"superior"},"provisionProcess":"laptop-geben","deprovisionProcess":"laptop-nehmen",` +
		`"configForm":"laptop-form","price":"CHF 1200","category":"Hardware","multipleAllowed":true,"maxDays":30,"eligible":["staff"],"keywords":["hardware","mobil"]},` +
		`{"id":"dock","homeCatalog":"ot-c1","state":"active","texts":{"de":"Dockingstation"},"lifecycle":{},"approval":{"kind":"none"},"provisionProcess":"d1","deprovisionProcess":"d2"},` +
		`{"id":"ot-p3","homeCatalog":"ot-c1","state":"draft","texts":{"de":"Tablet"},"lifecycle":{},"approval":{"kind":"none"},"provisionProcess":"","deprovisionProcess":""}],"publish":true}`
	if string(got) != want {
		t.Fatalf("document\n got %s\nwant %s", got, want)
	}
	if doc, _ := Write(Input{Catalogue: catalogue()}, false); doc.Publish || len(doc.Products) != 0 || doc.Products == nil || doc.Catalogs[0].MaintainedIn != nil {
		t.Errorf("an empty catalogue = %+v", doc)
	}
	if CatalogueID(catalogue()) != "ot-c1" || Engine(catalogue()) != "atlas" || ProductID(dock) != "dock" {
		t.Error("IDs or engine")
	}
}

// TestADocumentIsNotWrittenWithProblems: every problem the wiki can see
// is answered at once, naming its item by key.
func TestADocumentIsNotWrittenWithProblems(t *testing.T) {
	c := catalogue()
	c.Properties = item.Props{"catalogue-id": "no spaces", "languages": "Deutsch", "rank": 1.5}
	c.Title = ""
	twin := item.Props{"product-id": "same"}
	in := Input{Catalogue: c, Products: []item.Item{
		product("p1", "IT-2", "", StateActive, item.Props{"approval": "role"}),
		product("p2", "IT-3", "A", StateDraft, twin),
		product("p3", "IT-4", "B", StateDraft, item.Props{"product-id": "same", "orderable-from": time.Unix(200, 0), "orderable-until": time.Unix(100, 0)}),
		product("p4", "IT-5", "C", StateDraft, item.Props{"product-id": "-bad"}),
	}}
	doc, problems := Write(in, true)
	if doc.Catalogs != nil {
		t.Errorf("a document with problems = %+v", doc)
	}
	var all []string
	for _, p := range problems {
		all = append(all, p.Item+": "+p.Problem)
	}
	got := strings.Join(all, "\n")
	for _, want := range []string{
		`IT-1: catalogue-id "no spaces"`, `IT-1: languages: "Deutsch" is no language tag`, "IT-1: rank:", "IT-1: has no name",
		"IT-2: has no name", "IT-2: approver: an approval by role", "IT-2: an active product names the process",
		`IT-4: product-id "same" is IT-3's as well`, "IT-4: orderable-until is before orderable-from", `IT-5: product-id "-bad"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("problems lack %q:\n%s", want, got)
		}
	}
	if _, problems := Write(Input{Catalogue: item.Item{Key: "X-1", Type: "page"}}, false); len(problems) != 1 || !strings.Contains(problems[0].Problem, "not shop-catalogue") {
		t.Errorf("another type = %v", problems)
	}
	if _, problems := Write(Input{Catalogue: item.Item{Key: "X-1", Type: CatalogueType, Title: "T", Properties: item.Props{"rank": 2}}}, false); len(problems) == 0 || !strings.Contains(problems[0].Problem, "languages") {
		t.Errorf("without languages = %v", problems)
	}
}

// TestTheTypesAreValid: the two types define as the store takes them.
func TestTheTypesAreValid(t *testing.T) {
	s := item.NewStore()
	for _, td := range Types() {
		if err := s.DefineType("admin", td); err != nil {
			t.Fatalf("%s: %v", td.Name, err)
		}
	}
	if w := s.Type(ProductType).Workflow; w == nil || w.Initial != StateDraft || w.Finished(StateActive) {
		t.Errorf("product workflow = %+v", w)
	}
}
