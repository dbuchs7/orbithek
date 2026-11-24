package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"

	"github.com/dbuchs7/orbithek/internal/budget"
	"github.com/dbuchs7/orbithek/internal/engine"
	"github.com/dbuchs7/orbithek/internal/engineauth"
	"github.com/dbuchs7/orbithek/internal/engineconn"
	"github.com/dbuchs7/orbithek/internal/site"
)

// shopRoutes are a person's shop at a process engine, used with the
// person's own grant of it (ADR-0251):
// the catalogues they may order from, their orders and the approvals they
// hold. The engine decides everything — what a person sees, may order and
// may decide — as on its own shop page; this server names the routes it
// calls and hands the engine's answers and refusals on. Only people with
// an account here, connected to the engine, reach it.
func shopRoutes(d Deps) []Route {
	h := shopHandlers{d: d}
	pass := func(method, engine string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) { h.pass(w, r, method, engine) }
	}
	return []Route{
		{Method: http.MethodGet, Pattern: "/api/engines/{id}/shop", Summary: "Whether one may shop at a process engine: whether it offers a grant of one's shop, whether one is connected, and whether one's connection is such a grant (ADR-0251)", Handler: http.HandlerFunc(h.status)},
		{Method: http.MethodGet, Pattern: "/api/engines/{id}/shop/catalogs", Summary: "The engine's catalogues one may order from, highest rank first, as the engine answers them to one's grant", Handler: pass(http.MethodGet, "/api/v1/shop/catalogs")},
		{Method: http.MethodGet, Pattern: "/api/engines/{id}/shop/catalogs/{catalog}/release", Summary: "The newest release of a catalogue one may order from — what can be ordered from it — as {\"release\": …}, null before the first", Handler: http.HandlerFunc(h.release)},
		{Method: http.MethodGet, Pattern: "/api/engines/{id}/shop/catalogs/{catalog}/logo", Summary: "A catalogue's logo, a PNG, JPEG, WebP or GIF picture", Handler: http.HandlerFunc(h.picture)},
		{Method: http.MethodGet, Pattern: "/api/engines/{id}/shop/products/{product}/picture", Summary: "A product's picture, a PNG, JPEG, WebP or GIF picture", Handler: http.HandlerFunc(h.picture)},
		{Method: http.MethodGet, Pattern: "/api/engines/{id}/shop/forms/{form}", Summary: "A product's order form with its form-js schema, as one's grant reaches it", Handler: pass(http.MethodGet, "/api/v1/forms/{form}")},
		{Method: http.MethodPost, Pattern: "/api/engines/{id}/shop/releases/{release}/items/{item}/form-data/{field}/query", Summary: "Search the directory behind a field of a product's order form while it is filled in", Handler: pass(http.MethodPost, "/api/v1/shop/releases/{release}/items/{item}/form-data/{field}/query")},
		{Method: http.MethodPost, Pattern: "/api/engines/{id}/shop/orders", Summary: "Place an order as oneself against one release: {\"releaseId\", \"items\", \"variants\", \"config\"}; the engine answers the placed order or why it refuses it", Handler: pass(http.MethodPost, "/api/v1/orders")},
		{Method: http.MethodGet, Pattern: "/api/engines/{id}/shop/orders", Summary: "One's orders at the engine, newest first, each with the status of its lines", Handler: pass(http.MethodGet, "/api/v1/orders")},
		{Method: http.MethodGet, Pattern: "/api/engines/{id}/shop/orders/{order}", Summary: "One of one's orders, with the status of every line", Handler: pass(http.MethodGet, "/api/v1/orders/{order}")},
		{Method: http.MethodPost, Pattern: "/api/engines/{id}/shop/orders/{order}/cancel", Summary: "Withdraw an order: everything in it that has not happened yet ({\"reason\"} optional)", Handler: pass(http.MethodPost, "/api/v1/orders/{order}/cancel")},
		{Method: http.MethodPost, Pattern: "/api/engines/{id}/shop/orders/{order}/lines/{item}/cancel", Summary: "Withdraw one position of an order ({\"reason\"} optional)", Handler: pass(http.MethodPost, "/api/v1/orders/{order}/lines/{item}/cancel")},
		{Method: http.MethodPost, Pattern: "/api/engines/{id}/shop/orders/{order}/lines/{item}/return", Summary: "Give back one provisioned position, which starts its deprovisioning", Handler: pass(http.MethodPost, "/api/v1/orders/{order}/lines/{item}/return")},
		{Method: http.MethodGet, Pattern: "/api/engines/{id}/shop/approvals", Summary: "The open approvals one holds as an approver, newest first; before= continues a page", Handler: http.HandlerFunc(h.approvals)},
		{Method: http.MethodPost, Pattern: "/api/engines/{id}/shop/approvals/decide", Summary: "Decide approvals one holds, all of one order: {\"approved\", \"reason\", \"taskKeys\"}; a refusal needs a reason", Handler: pass(http.MethodPost, "/api/v1/approvals/decide")},
		{Method: http.MethodGet, Pattern: "/api/engines/{id}/shop/tasks", Summary: "The open tasks of one's orders and of orders one holds a task in", Handler: pass(http.MethodGet, "/api/v1/shop/tasks")},
	}
}

type shopHandlers struct {
	d Deps
}

// pictureTypes are the pictures handed on from the engine; an SVG could
// carry script and is not.
var pictureTypes = map[string]bool{"image/png": true, "image/jpeg": true, "image/webp": true, "image/gif": true}

// shopStatusJSON says whether one may shop at an engine.
type shopStatusJSON struct {
	Engine map[string]string `json:"engine"`
	// Offered says the engine grants people their shop; Connectable that
	// this server may connect one to it.
	Offered     bool `json:"offered"`
	Connectable bool `json:"connectable"`
	// Connected says one is connected; Shop that the connection is a grant
	// of one's shop rather than of one's tasks alone, so connecting again
	// is needed before one can shop.
	Connected bool `json:"connected"`
	Shop      bool `json:"shop"`
	// Silent says one is connected again without a page of one's own.
	Silent bool `json:"silent"`
}

func (h shopHandlers) engineOf(w http.ResponseWriter, r *http.Request) (site.Engine, bool) {
	e, ok := h.d.DB.Site.Engine(r.PathValue("id"))
	if !ok || e.Kind != site.EngineAtlas {
		writeError(w, http.StatusNotFound, "no engine "+r.PathValue("id")+" is set up")
		return site.Engine{}, false
	}
	return e, true
}

func (h shopHandlers) status(w http.ResponseWriter, r *http.Request) {
	e, ok := h.engineOf(w, r)
	if !ok {
		return
	}
	p, _ := PrincipalFrom(r.Context())
	c, connected := h.d.Connect.Connections().Get(string(p.User), e.ID)
	writeJSON(w, http.StatusOK, shopStatusJSON{
		Engine:  map[string]string{"id": e.ID, "name": e.Name},
		Offered: h.d.Connect.OffersShop(r.Context(), e), Connectable: h.d.Connect.Configured(e.ID),
		Connected: connected, Shop: connected && c.Shop,
		Silent: p.Via == "session" && h.d.Connect.Silent(e.ID, string(p.User)),
	})
}

// token is the access token of the caller's grant of their shop, or
// answers why there is none: not connected, or connected for tasks alone.
func (h shopHandlers) token(w http.ResponseWriter, r *http.Request, e site.Engine) (string, bool) {
	p, _ := PrincipalFrom(r.Context())
	user := string(p.User)
	c, connected := h.d.Connect.Connections().Get(user, e.ID)
	switch {
	case !connected:
		writeJSON(w, http.StatusConflict, map[string]any{"error": "connect your account to " + e.Name + " first", "connect": true})
		return "", false
	case !c.Shop:
		writeJSON(w, http.StatusConflict, map[string]any{"error": "your connection to " + e.Name + " reaches your tasks only; connect again for the shop", "connect": true})
		return "", false
	}
	tok, err := h.d.Connect.Token(r.Context(), e, user)
	switch {
	case errors.Is(err, engineauth.ErrEnded), errors.Is(err, engineconn.ErrNotConnected):
		writeJSON(w, http.StatusConflict, map[string]any{"error": e.Name + " no longer accepts your connection; connect again", "connect": true})
		return "", false
	case err != nil:
		writeError(w, http.StatusBadGateway, "engine "+e.ID+": "+err.Error())
		return "", false
	}
	return tok, true
}

// call makes the person's request of the engine's shop, and answers the
// engine's refusal itself; ok is false when it did.
func (h shopHandlers) call(w http.ResponseWriter, r *http.Request, e site.Engine, method, path string, body []byte, accept string) (engine.ShopAnswer, bool) {
	tok, ok := h.token(w, r, e)
	if !ok {
		return engine.ShopAnswer{}, false
	}
	a, err := h.d.Engines.Shop(r.Context(), e, tok, method, path, body, accept)
	if err == nil {
		return a, true
	}
	p, _ := PrincipalFrom(r.Context())
	var se *engine.ShopError
	switch {
	case errors.As(err, &se) && se.Status >= 400 && se.Status < 500:
		// The engine's refusal in its own words and status: what the person
		// may not order, decide or see, and the problems of a basket.
		out := map[string]any{"error": se.Message}
		if len(se.Body) > 0 {
			out["engine"] = se.Body
		}
		writeJSON(w, se.Status, out)
	case errors.Is(err, engine.ErrRefused):
		h.d.Connect.Refused(string(p.User), e.ID)
		writeJSON(w, http.StatusConflict, map[string]any{"error": e.Name + " refused your connection; try again, or connect again", "connect": true})
	default:
		writeError(w, http.StatusBadGateway, "engine "+e.ID+": "+err.Error())
	}
	return engine.ShopAnswer{}, false
}

// enginePath fills the engine's route with the request's path values,
// each escaped as one segment.
func enginePath(r *http.Request, route string) string {
	var out bytes.Buffer
	for i := 0; i < len(route); i++ {
		if route[i] != '{' {
			out.WriteByte(route[i])
			continue
		}
		end := bytes.IndexByte([]byte(route[i:]), '}')
		out.WriteString(url.PathEscape(r.PathValue(route[i+1 : i+end])))
		i += end
	}
	return out.String()
}

// shopBody reads the request's body for the engine: a JSON object sent as
// application/json, which is also the defence against a cross-site form,
// or nothing.
func shopBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, budget.Get().RequestBody))
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "the request body is too large")
		return nil, false
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return []byte(`{}`), true
	}
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, errUnsupportedMedia.Error())
		return nil, false
	}
	if t := bytes.TrimSpace(raw); t[0] != '{' || !json.Valid(t) {
		writeError(w, http.StatusBadRequest, "the request body must be one JSON object")
		return nil, false
	}
	return raw, true
}

// pass hands a request on to the engine's route and its JSON answer back.
func (h shopHandlers) pass(w http.ResponseWriter, r *http.Request, method, route string) {
	e, ok := h.engineOf(w, r)
	if !ok {
		return
	}
	var body []byte
	if method == http.MethodPost {
		if body, ok = shopBody(w, r); !ok {
			return
		}
	} else if q := r.URL.RawQuery; q != "" && route == "/api/v1/shop/catalogs" {
		route += "?" + q
	}
	a, ok := h.call(w, r, e, method, enginePath(r, route), body, "")
	if !ok {
		return
	}
	writeEngineJSON(w, a.Body)
}

// writeEngineJSON answers the engine's JSON as it gave it, with 200 for
// any of its successes, or null for an empty answer; a body that is no
// JSON is not handed on.
func writeEngineJSON(w http.ResponseWriter, body []byte) {
	t := bytes.TrimSpace(body)
	if len(t) == 0 {
		t = []byte("null")
	}
	if !json.Valid(t) {
		writeError(w, http.StatusBadGateway, "the engine answered no JSON")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(t)
}

func (h shopHandlers) release(w http.ResponseWriter, r *http.Request) {
	e, ok := h.engineOf(w, r)
	if !ok {
		return
	}
	a, ok := h.call(w, r, e, http.MethodGet, "/api/v1/catalogs/"+url.PathEscape(r.PathValue("catalog"))+"/releases?limit=1", nil, "")
	if !ok {
		return
	}
	var releases []json.RawMessage
	if err := json.Unmarshal(a.Body, &releases); err != nil {
		writeError(w, http.StatusBadGateway, "the engine answered no list of releases")
		return
	}
	out := json.RawMessage("null")
	if len(releases) > 0 {
		out = releases[0]
	}
	writeJSON(w, http.StatusOK, map[string]json.RawMessage{"release": out})
}

func (h shopHandlers) approvals(w http.ResponseWriter, r *http.Request) {
	e, ok := h.engineOf(w, r)
	if !ok {
		return
	}
	path := "/api/v1/approvals"
	if b := r.URL.Query().Get("before"); b != "" {
		n, err := strconv.ParseUint(b, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "before must be a whole number")
			return
		}
		path += "?before=" + strconv.FormatUint(n, 10)
	}
	a, ok := h.call(w, r, e, http.MethodGet, path, nil, "")
	if !ok {
		return
	}
	writeEngineJSON(w, a.Body)
}

// picture hands on a catalogue's logo or a product's picture, as a
// picture of a type that carries no script.
func (h shopHandlers) picture(w http.ResponseWriter, r *http.Request) {
	e, ok := h.engineOf(w, r)
	if !ok {
		return
	}
	path := "/api/v1/catalog-products/" + url.PathEscape(r.PathValue("product")) + "/picture"
	if c := r.PathValue("catalog"); c != "" {
		path = "/api/v1/catalogs/" + url.PathEscape(c) + "/logo"
	}
	a, ok := h.call(w, r, e, http.MethodGet, path, nil, "image/png, image/jpeg, image/webp, image/gif")
	if !ok {
		return
	}
	mt, _, _ := mime.ParseMediaType(a.ContentType)
	if !pictureTypes[mt] {
		writeError(w, http.StatusBadGateway, "the engine answered no picture")
		return
	}
	w.Header().Set("Content-Type", mt)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'")
	w.Header().Set("Cache-Control", "private, max-age=300")
	_, _ = w.Write(a.Body)
}
