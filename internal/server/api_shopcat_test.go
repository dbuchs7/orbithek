package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/dbuchs7/orbithek/internal/engineauth/engineauthtest"
)

// TestACatalogueMaintainedInTheWikiIsPublishedAtItsEngine covers the
// catalogue maintenance of ADR-0251:
// administrators set up the two types; a catalogue item and the product
// items below it are shown as the document they would be published as,
// or with their problems; who may edit the catalogue publishes it with
// their own grant of the shop, as a dry run first, and the engine's
// refusal comes back in its words; who may not edit it may not publish
// it; a protected passage of a product's text stays in the wiki.
func TestACatalogueMaintainedInTheWikiIsPublishedAtItsEngine(t *testing.T) {
	w, fake := connectWorld(t)
	fake.Shop = true
	fake.ShopAnswers = map[string]engineauthtest.ShopAnswer{
		"POST /api/v1/catalogs/import": {Body: `{"created":["cat-it"],"updated":[],"releases":[{"id":"rel-1"}]}`},
	}
	w.bert.expect("POST", "/api/shop/types", "", http.StatusForbidden)
	got := w.admin.expect("POST", "/api/shop/types", "", http.StatusOK)
	if toJSON(t, got) != `{"defined":["shop-catalogue","shop-product"],"existing":[]}` {
		t.Fatalf("types = %v", got)
	}
	if got := w.admin.expect("POST", "/api/shop/types", "", http.StatusOK); toJSON(t, got) != `{"defined":[],"existing":["shop-catalogue","shop-product"]}` {
		t.Fatalf("types again = %v", got)
	}

	cat := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"shop-catalogue","title":"IT-Arbeitsplatz",
		"properties":{"engine":"atlas","catalogue-id":"cat-it","languages":"de, en","rank":10}}`, http.StatusCreated)
	catID := field(cat, "item", "id").(string)
	laptop := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"shop-product","title":"Laptop","parent":"`+catID+`",
		"properties":{"product-id":"laptop","provision-process":"give","deprovision-process":"take","approval":"superior"}}`, http.StatusCreated)
	lapID := field(laptop, "item", "id").(string)
	dock := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"shop-product","title":"Dock","parent":"`+lapID+`",
		"properties":{"product-id":"dock","provision-process":"d1","deprovision-process":"d2"}}`, http.StatusCreated)
	dockID := field(dock, "item", "id").(string)
	w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"shop-product","title":"Elsewhere",
		"properties":{"product-id":"elsewhere"}}`, http.StatusCreated)
	for _, id := range []string{lapID, dockID} {
		w.bert.expect("POST", "/api/items/"+id+"/transitions", `{"name":"activate"}`, http.StatusOK)
	}
	w.bert.expect("POST", "/api/relations", `{"type":"aggregation","from":"`+lapID+`","to":"`+dockID+`"}`, http.StatusCreated)
	w.bert.expect("PUT", "/api/items/"+lapID+"/body", `{"blocks":[{"kind":"paragraph","text":"Für alle."}]}`, http.StatusOK)

	got = w.bert.expect("GET", "/api/items/"+catID+"/shop/document", "", http.StatusOK)
	doc := toJSON(t, got["document"])
	for _, want := range []string{`"id":"cat-it"`, `"items":["laptop","dock"]`, `{"from":"laptop","kind":"aggregation","to":"dock"}`,
		`"descriptions":{"de":"Für alle."}`} {
		if !strings.Contains(doc, want) {
			t.Errorf("document lacks %s:\n%s", want, doc)
		}
	}
	// The test server is reached by plain http, so the address is left out.
	if strings.Contains(doc, "elsewhere") || strings.Contains(doc, "maintainedIn") {
		t.Errorf("a product outside the catalogue is in it:\n%s", doc)
	}
	w.vera.expect("GET", "/api/items/"+catID+"/shop/document", "", http.StatusForbidden)
	w.bert.expect("GET", "/api/items/"+lapID+"/shop/document", "", http.StatusBadRequest)

	// Publishing needs a grant of the shop.
	if rec := w.bert.do("POST", "/api/items/"+catID+"/shop/publish", `{"dryRun":true}`); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), `"connect":true`) {
		t.Fatalf("publishing unconnected: %d %s", rec.Code, rec.Body)
	}
	back, state := beginAt(t, w.bert, fake, "/api/engines/atlas/connect?shop=1")
	callback(t, w.admin.h, back, state)

	got = w.bert.expect("POST", "/api/items/"+catID+"/shop/publish", `{"dryRun":true}`, http.StatusOK)
	if got["dryRun"] != true || got["catalogue"] != "cat-it" || got["result"].(map[string]any)["created"] == nil {
		t.Fatalf("dry run = %v", got)
	}
	call := fake.ShopCalls[len(fake.ShopCalls)-1]
	if call.Path != "/api/v1/catalogs/import?overwrite=*&dryRun=true" || call.Method != "POST" {
		t.Fatalf("the engine was asked %s %s", call.Method, call.Path)
	}
	var sent map[string]any
	if err := json.Unmarshal([]byte(call.Body), &sent); err != nil || sent["publish"] != true || toJSON(t, sent["catalogs"]) != toJSON(t, got2(t, w, catID)) {
		t.Fatalf("sent %s (%v)", call.Body, err)
	}
	w.bert.expect("POST", "/api/items/"+catID+"/shop/publish", "", http.StatusOK)
	if call := fake.ShopCalls[len(fake.ShopCalls)-1]; call.Path != "/api/v1/catalogs/import?overwrite=*" {
		t.Fatalf("a publish was asked as %s", call.Path)
	}

	// The engine's refusal, in its words.
	fake.ShopAnswers["POST /api/v1/catalogs/import"] = engineauthtest.ShopAnswer{Status: http.StatusForbidden, Body: `{"error":"you do not maintain catalogues"}`}
	if got := w.bert.expect("POST", "/api/items/"+catID+"/shop/publish", "{}", http.StatusForbidden); got["error"] != "you do not maintain catalogues" {
		t.Errorf("a refusal = %v", got)
	}
	w.vera.expect("POST", "/api/items/"+catID+"/shop/publish", "{}", http.StatusForbidden)
	w.bert.expect("POST", "/api/items/"+catID+"/shop/publish", `{"later":true}`, http.StatusBadRequest)

	// Problems are answered before the engine is asked.
	w.bert.expect("PUT", "/api/items/"+dockID+"/properties/provision-process", `{"value":null}`, http.StatusOK)
	n := len(fake.ShopCalls)
	got = w.bert.expect("POST", "/api/items/"+catID+"/shop/publish", "{}", http.StatusUnprocessableEntity)
	if !strings.Contains(toJSON(t, got["problems"]), "names the process that provisions it") || len(fake.ShopCalls) != n {
		t.Errorf("problems = %v after %d calls", got, len(fake.ShopCalls)-n)
	}
	if got := w.bert.expect("GET", "/api/items/"+catID+"/shop/document", "", http.StatusOK); got["document"] != nil || len(got["problems"].([]any)) != 1 {
		t.Errorf("a document with problems = %v", got)
	}
	// An engine that is not set up.
	w.bert.expect("PUT", "/api/items/"+dockID+"/properties/provision-process", `{"value":"d1"}`, http.StatusOK)
	w.bert.expect("PUT", "/api/items/"+catID+"/properties/engine", `{"value":"nowhere"}`, http.StatusOK)
	w.bert.expect("POST", "/api/items/"+catID+"/shop/publish", "{}", http.StatusUnprocessableEntity)
}

// got2 is the catalogue of an item's document as the wiki shows it.
func got2(t *testing.T, w world, catID string) any {
	t.Helper()
	return w.bert.expect("GET", "/api/items/"+catID+"/shop/document", "", http.StatusOK)["document"].(map[string]any)["catalogs"]
}
