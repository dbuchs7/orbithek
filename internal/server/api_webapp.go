package server

import (
	"cmp"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/dbuchs7/orbithek/internal/budget"
)

// The installable web app (ADR-0209): a manifest, so a browser installs
// the UI as an app of its own, and a share target, so sharing a page or a
// text to the installed app opens the capture view. No service worker is
// served: the browsers that offer installing and sharing need none, and
// items are never kept for offline use.

// ManifestPath is where the web app's manifest is served.
const ManifestPath = "/manifest.webmanifest"

// CapturePath receives what is shared to the installed app.
const CapturePath = "/capture"

// captureFields are what a share or a bookmarklet passes on; the capture
// view reads nothing else.
var captureFields = []string{"title", "text", "url"}

func webAppRoutes(d Deps) []Route {
	return []Route{
		// A browser fetches the manifest without the session cookie; it
		// holds the installation's name and the product's icons only.
		{Method: http.MethodGet, Pattern: ManifestPath, Summary: "The web app's manifest: name, icons, start address and share target, so the UI is installed as an app (ADR-0209)",
			Access: Public, Handler: manifest(d)},
		// What is shared is passed on to the capture view in the address's
		// fragment, which the browser keeps to itself; the capture view
		// then asks to sign in where needed.
		{Method: http.MethodGet, Pattern: CapturePath, Summary: "Open the capture view with title, text and url, as a share to the installed app or a link does (ADR-0209)",
			Access: Public, Handler: http.HandlerFunc(capture)},
		{Method: http.MethodPost, Pattern: CapturePath, Summary: "The share target of the installed app: title, text and url as a form, passed on to the capture view (ADR-0209)",
			Access: Public, Handler: http.HandlerFunc(capture)},
	}
}

type manifestIcon struct {
	Src     string `json:"src"`
	Sizes   string `json:"sizes"`
	Type    string `json:"type"`
	Purpose string `json:"purpose,omitempty"`
}

func manifest(d Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		design := d.DB.Site.Design()
		name := cmp.Or(strings.TrimSpace(design.Name), "Orbithek")
		m := map[string]any{
			"id":               "/",
			"name":             name,
			"short_name":       name,
			"start_url":        "/",
			"scope":            "/",
			"display":          "standalone",
			"theme_color":      cmp.Or(design.HeaderBackground, "#0E2A38"),
			"background_color": cmp.Or(design.Background, "#ffffff"),
			"icons": []manifestIcon{
				{Src: "/assets/brand/icon-192.png", Sizes: "192x192", Type: "image/png"},
				{Src: "/assets/brand/icon-512.png", Sizes: "512x512", Type: "image/png"},
				{Src: "/assets/brand/icon-maskable-512.png", Sizes: "512x512", Type: "image/png", Purpose: "maskable"},
			},
			"share_target": map[string]any{
				"action": CapturePath, "method": "POST", "enctype": "application/x-www-form-urlencoded",
				"params": map[string]string{"title": "title", "text": "text", "url": "url"},
			},
			// The installed app's own menu, on a long press of its icon.
			"shortcuts": []map[string]any{{"name": "Capture", "url": "/#/capture",
				"icons": []manifestIcon{{Src: "/assets/brand/icon-192.png", Sizes: "192x192", Type: "image/png"}}}},
		}
		w.Header().Set("Content-Type", "application/manifest+json")
		w.Header().Set("Cache-Control", "no-cache")
		_ = json.NewEncoder(w).Encode(m)
	})
}

// capture passes a share on to the capture view, in the fragment of the
// address, which neither this server's logs nor a proxy's see again.
func capture(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, budget.Get().RequestBody)
	}
	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "the share is too large")
		return
	}
	q := url.Values{}
	for _, f := range captureFields {
		if v := strings.TrimSpace(r.Form.Get(f)); v != "" {
			q.Set(f, v)
		}
	}
	target := "/#/capture"
	if len(q) > 0 {
		target += "?" + q.Encode()
	}
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, target, http.StatusSeeOther)
}
