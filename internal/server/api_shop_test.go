package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dbuchs7/orbithek/internal/engineauth/engineauthtest"
)

// TestAPersonShopsAtAnEngineWithTheirOwnGrant covers the shop of
// ADR-0251: one learns whether one
// may shop; without a connection, or with one for tasks alone, one is
// asked to connect for the shop; connected so, the catalogues, a
// catalogue's newest release, forms, orders and approvals are the
// engine's answers to one's own grant, and its refusals come back in its
// status and words; bodies are JSON objects; pictures are handed on only
// as pictures that carry no script.
func TestAPersonShopsAtAnEngineWithTheirOwnGrant(t *testing.T) {
	w, fake := connectWorld(t)
	fake.Shop = true
	fake.ShopAnswers = map[string]engineauthtest.ShopAnswer{
		"GET /api/v1/shop/catalogs":                   {Body: `[{"id":"cat-it","texts":{"de":"IT"}}]`},
		"GET /api/v1/catalogs/cat-it/releases":        {Body: `[{"id":"rel-2","items":[{"id":"laptop"}]},{"id":"rel-1"}]`},
		"GET /api/v1/catalogs/cat-new/releases":       {Body: `[]`},
		"GET /api/v1/catalogs/cat-odd/releases":       {Body: `{"id":"rel"}`},
		"POST /api/v1/orders":                         {Status: http.StatusCreated, Body: `{"id":"ord-1","lines":[]}`},
		"GET /api/v1/orders":                          {Body: `[{"id":"ord-1"}]`},
		"GET /api/v1/orders/ord-1":                    {Body: `{"id":"ord-1"}`},
		"POST /api/v1/orders/ord-1/cancel":            {Body: `{"id":"ord-1","cancelled":true}`},
		"POST /api/v1/orders/ord-1/lines/x/return":    {Status: http.StatusConflict, Body: `{"error":"the position is not held"}`},
		"GET /api/v1/approvals":                       {Body: `{"items":[],"total":0}`},
		"POST /api/v1/approvals/decide":               {Status: http.StatusBadRequest, Body: `{"error":"a refusal needs a reason"}`},
		"GET /api/v1/shop/tasks":                      {Body: ``},
		"GET /api/v1/catalog-products/laptop/picture": {Body: "\x89PNG\r\n", ContentType: "image/png"},
		"GET /api/v1/catalog-products/evil/picture":   {Body: "<svg onload=alert(1)>", ContentType: "image/svg+xml"},
		"GET /api/v1/catalogs/cat-it/logo":            {Body: "GIF89a", ContentType: "image/gif"},
		"GET /api/v1/shop/odd":                        {Body: "not json"},
	}
	base := "/api/engines/atlas/shop"

	got := w.bert.expect("GET", base, "", http.StatusOK)
	if got["offered"] != true || got["connectable"] != true || got["connected"] != false || got["shop"] != false {
		t.Fatalf("status unconnected = %v", got)
	}
	w.bert.expect("GET", "/api/engines/nowhere/shop", "", http.StatusNotFound)
	if rec := w.bert.do("GET", base+"/catalogs", ""); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), `"connect":true`) {
		t.Fatalf("catalogues unconnected: %d %s", rec.Code, rec.Body)
	}

	// Connected for tasks alone: asked to connect again.
	back, state := begin(t, w.bert, fake)
	callback(t, w.admin.h, back, state)
	if got := w.bert.expect("GET", base, "", http.StatusOK); got["connected"] != true || got["shop"] != false {
		t.Fatalf("status for tasks = %v", got)
	}
	if rec := w.bert.do("GET", base+"/catalogs", ""); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "connect again for the shop") {
		t.Fatalf("catalogues for tasks: %d %s", rec.Code, rec.Body)
	}

	back, state = beginAt(t, w.bert, fake, "/api/engines/atlas/connect?shop=1")
	if fake.LastAuthorize.Get("resource") != fake.URL+"/shop" {
		t.Fatalf("asked for %q", fake.LastAuthorize.Get("resource"))
	}
	callback(t, w.admin.h, back, state)
	if got := w.bert.expect("GET", base, "", http.StatusOK); got["shop"] != true {
		t.Fatalf("status for the shop = %v", got)
	}

	if rec := w.bert.do("GET", base+"/catalogs", ""); rec.Code != http.StatusOK || rec.Body.String() != `[{"id":"cat-it","texts":{"de":"IT"}}]` {
		t.Fatalf("catalogues: %d %s", rec.Code, rec.Body)
	}
	if got := w.bert.expect("GET", base+"/catalogs/cat-it/release", "", http.StatusOK); got["release"].(map[string]any)["id"] != "rel-2" {
		t.Errorf("newest release = %v", got)
	}
	if call := fake.ShopCalls[len(fake.ShopCalls)-1]; call.Path != "/api/v1/catalogs/cat-it/releases?limit=1" {
		t.Errorf("the release was asked as %q", call.Path)
	}
	if got := w.bert.expect("GET", base+"/catalogs/cat-new/release", "", http.StatusOK); got["release"] != nil {
		t.Errorf("no release yet = %v", got)
	}
	w.bert.expect("GET", base+"/catalogs/cat-odd/release", "", http.StatusBadGateway)

	// Orders: the body goes as sent; it must be one JSON object, sent as JSON.
	order := `{"releaseId":"rel-2","items":["laptop"],"config":{"laptop":{"size":"14"}}}`
	if rec := w.bert.do("POST", base+"/orders", order); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"ord-1"`) {
		t.Fatalf("placing: %d %s", rec.Code, rec.Body)
	}
	if call := fake.ShopCalls[len(fake.ShopCalls)-1]; call.Body != order || call.Person != fake.Person.ID || call.Method != "POST" {
		t.Errorf("the engine got %+v", call)
	}
	w.bert.expect("POST", base+"/orders", `["laptop"]`, http.StatusBadRequest)
	w.bert.expect("POST", base+"/orders", `{"releaseId":`, http.StatusBadRequest)
	req := httptest.NewRequest("POST", base+"/orders", strings.NewReader(order))
	req.Header.Set("Content-Type", "text/plain")
	req.AddCookie(w.bert.cookie)
	rec := httptest.NewRecorder()
	w.bert.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("a form post: %d %s", rec.Code, rec.Body)
	}
	if rec := w.bert.do("GET", base+"/orders", ""); rec.Body.String() != `[{"id":"ord-1"}]` {
		t.Errorf("orders: %s", rec.Body)
	}
	w.bert.expect("GET", base+"/orders/ord-1", "", http.StatusOK)
	// A path value is one segment at the engine.
	w.bert.expect("GET", base+"/orders/ord%2F1", "", http.StatusNotFound)
	if call := fake.ShopCalls[len(fake.ShopCalls)-1]; call.Path != "/api/v1/orders/ord%2F1" {
		t.Errorf("an escaped order id was asked as %q", call.Path)
	}
	if got := w.bert.expect("POST", base+"/orders/ord-1/cancel", "", http.StatusOK); got["cancelled"] != true {
		t.Errorf("cancel = %v", got)
	}
	got = w.bert.expect("POST", base+"/orders/ord-1/lines/x/return", "", http.StatusConflict)
	if got["error"] != "the position is not held" || got["engine"] == nil {
		t.Errorf("a refused return = %v", got)
	}

	// Approvals.
	w.bert.expect("GET", base+"/approvals", "", http.StatusOK)
	w.bert.expect("GET", base+"/approvals?before=5", "", http.StatusOK)
	if call := fake.ShopCalls[len(fake.ShopCalls)-1]; call.Path != "/api/v1/approvals?before=5" {
		t.Errorf("a page of approvals was asked as %q", call.Path)
	}
	w.bert.expect("GET", base+"/approvals?before=x", "", http.StatusBadRequest)
	if got := w.bert.expect("POST", base+"/approvals/decide", `{"approved":false,"taskKeys":[1]}`, http.StatusBadRequest); got["error"] != "a refusal needs a reason" {
		t.Errorf("a refused decision = %v", got)
	}
	if rec := w.bert.do("GET", base+"/tasks", ""); rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "null" {
		t.Errorf("an empty answer: %d %q", rec.Code, rec.Body)
	}
	w.bert.expect("GET", base+"/forms/none", "", http.StatusNotFound)

	// Pictures.
	rec = w.bert.do("GET", base+"/products/laptop/picture", "")
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" || rec.Header().Get("X-Content-Type-Options") != "nosniff" || rec.Body.String() != "\x89PNG\r\n" {
		t.Errorf("a picture: %d %v %q", rec.Code, rec.Header(), rec.Body)
	}
	w.bert.expect("GET", base+"/products/evil/picture", "", http.StatusBadGateway)
	if rec := w.bert.do("GET", base+"/catalogs/cat-it/logo", ""); rec.Header().Get("Content-Type") != "image/gif" {
		t.Errorf("a logo: %d %v", rec.Code, rec.Header())
	}

	// A grant the engine no longer accepts asks to connect again.
	fake.Withdraw()
	if rec := w.bert.do("GET", base+"/catalogs", ""); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), `"connect":true`) {
		t.Errorf("after the grant was withdrawn: %d %s", rec.Code, rec.Body)
	}
}
