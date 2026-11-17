package server

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/dbuchs7/orbithek/internal/auth"
	"github.com/dbuchs7/orbithek/internal/certs"
	"github.com/dbuchs7/orbithek/internal/certs/certstest"
	"github.com/dbuchs7/orbithek/internal/storage"
	"github.com/dbuchs7/orbithek/internal/web"
)

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

// testDeps opens a data directory with cheap password hashing.
func testDeps(t *testing.T) Deps {
	t.Helper()
	db, _, err := storage.OpenWith(t.TempDir(), storage.Options{Auth: []auth.Option{auth.WithIterations(1000)}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return Deps{UI: web.FS(), DB: db}
}

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	d := testDeps(t)
	return Handler(Routes(d), d.DB.Accounts)
}

func TestRouteTableIsComplete(t *testing.T) {
	seen := map[string]bool{}
	for _, rt := range Routes(testDeps(t)) {
		key := rt.Method + " " + rt.Pattern
		if rt.Method == "" || rt.Pattern == "" || rt.Summary == "" || rt.Handler == nil {
			t.Errorf("%s: method, pattern, summary and handler are required", key)
		}
		if seen[key] {
			t.Errorf("%s: registered twice", key)
		}
		seen[key] = true
	}
}

func TestPublicRoutes(t *testing.T) {
	h := testHandler(t)
	cases := []struct {
		path, status, contentType, body string
	}{
		{"/healthz", "200", "text/plain", "ok"},
		{"/", "200", "text/html", `<main id="app">`},
		{"/assets/app.js", "200", "javascript", "/api/me"},
		{"/assets/style.css", "200", "text/css", "color-scheme"},
		{"/vendor/prosemirror.js", "200", "javascript", "EditorView"},
		{"/favicon.ico", "200", "image/", ""},
		{"/assets/brand/favicon.svg", "200", "image/svg+xml", "prefers-color-scheme"},
		{"/assets/brand/logo-black.svg", "200", "image/svg+xml", "<title>Orbithek</title>"},
		{"/assets/brand/apple-touch-icon.png", "200", "image/png", ""},
		{"/vendor/prosemirror.LICENSE.txt", "200", "text/plain", "MIT"},
		{"/draw", "200", "text/html", "/assets/draw.js"},
		{"/assets/draw.js", "200", "javascript", "exportEmbedScene"},
		{"/vendor/excalidraw.js", "200", "javascript", ""},
		{"/vendor/excalidraw.LICENSE.txt", "200", "text/plain", "Copyright (c) 2020 Excalidraw"},
	}
	for _, c := range cases {
		rec := get(t, h, c.path)
		if got := rec.Result().Status; !strings.HasPrefix(got, c.status) {
			t.Errorf("GET %s: status %s, want %s", c.path, got, c.status)
		}
		if got := rec.Header().Get("Content-Type"); !strings.Contains(got, c.contentType) {
			t.Errorf("GET %s: Content-Type %q, want %q", c.path, got, c.contentType)
		}
		if !strings.Contains(rec.Body.String(), c.body) {
			t.Errorf("GET %s: body does not contain %q", c.path, c.body)
		}
	}
}

func TestUnknownPathsAndListingsAreNotFound(t *testing.T) {
	h := testHandler(t)
	for _, path := range []string{"/nope", "/assets/", "/assets/missing.js", "/index.html/x"} {
		if rec := get(t, h, path); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s: status %d, want 404", path, rec.Code)
		}
	}
}

func TestWrongMethodIsRejected(t *testing.T) {
	h := testHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/healthz", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /healthz: status %d, want 405", rec.Code)
	}
}

func TestUndeclaredAccessIsGated(t *testing.T) {
	called := false
	secret := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })
	// No Access field: the zero value must be Authenticated.
	h := Handler([]Route{{Method: http.MethodGet, Pattern: "/secret", Summary: "s", Handler: secret}}, nil)
	if rec := get(t, h, "/secret"); rec.Code != http.StatusUnauthorized {
		t.Errorf("status %d, want 401", rec.Code)
	}
	if called {
		t.Error("the handler of an authenticated route ran without a principal")
	}
}

func TestEveryResponseCarriesSecurityHeaders(t *testing.T) {
	h := testHandler(t)
	for _, path := range []string{"/", "/healthz", "/nope"} {
		hd := get(t, h, path).Header()
		if !strings.Contains(hd.Get("Content-Security-Policy"), "default-src 'self'") {
			t.Errorf("GET %s: missing content security policy", path)
		}
		if hd.Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("GET %s: missing nosniff", path)
		}
	}
}

// TestOnlyTheDrawingPageRelaxesThePolicy: the drawing page may take inline
// styles and be framed by the UI (ADR-0053); everything else keeps the
// strict policy and may not be framed.
func TestOnlyTheDrawingPageRelaxesThePolicy(t *testing.T) {
	h := testHandler(t)
	draw := get(t, h, "/draw").Header().Get("Content-Security-Policy")
	if draw != DrawPolicy || !strings.Contains(draw, "frame-ancestors 'self'") || strings.Contains(draw, "script-src") {
		t.Errorf("drawing page policy %q", draw)
	}
	for _, path := range []string{"/", "/assets/draw.js", "/vendor/excalidraw.js"} {
		if p := get(t, h, path).Header().Get("Content-Security-Policy"); strings.Contains(p, "unsafe-inline") || !strings.Contains(p, "frame-ancestors 'none'") {
			t.Errorf("GET %s: policy %q", path, p)
		}
	}
}

func TestAccessString(t *testing.T) {
	if Public.String() != "public" || Authenticated.String() != "authenticated" {
		t.Error("unexpected access class names")
	}
}

func TestServeAnswersAndShutsDownWhenContextEnds(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	h := testHandler(t)
	go func() {
		done <- Serve(ctx, ln, h, nil, slog.New(slog.DiscardHandler))
	}()

	res, err := http.Get("http://" + ln.Addr().String() + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if string(body) != "ok\n" {
		t.Errorf("body %q, want ok", body)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Serve returned %v, want nil after shutdown", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after the context ended")
	}
}

func TestServeReportsListenerFailure(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ln.Close()
	err = Serve(context.Background(), ln, http.NotFoundHandler(), nil, slog.New(slog.DiscardHandler))
	if err == nil {
		t.Fatal("Serve on a closed listener returned nil")
	}
}

// TestServeTerminatesTLS: given a TLS configuration, Serve answers HTTPS on
// the listener, over HTTP/2 where the client speaks it, requests count as
// secure, so cookies are Secure (ADR-0037), and a stream is flushed as it
// is written (ADR-draft-tls-in-the-binary).
func TestServeTerminatesTLS(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := certstest.Write(t, dir, "wiki", "wiki.example.org")
	store, err := certs.Open([]certs.Pair{{Cert: certFile, Key: keyFile}}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	more := make(chan struct{})
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s secure=%v\n", r.Proto, isSecure(r))
		if r.URL.Path == "/stream" {
			rc := http.NewResponseController(w)
			if err := rc.SetWriteDeadline(time.Now().Add(time.Minute)); err != nil {
				t.Errorf("no write deadline on a stream: %v", err)
			}
			if err := rc.Flush(); err != nil {
				t.Errorf("cannot flush a stream: %v", err)
			}
			<-more
			fmt.Fprintln(w, "more")
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	var logged bytes.Buffer
	go func() { done <- Serve(ctx, ln, h, store.Config(), slog.New(slog.NewTextHandler(&logged, nil))) }()

	pool := x509.NewCertPool()
	leaves, err := certs.Leaves([]string{certFile})
	if err != nil {
		t.Fatal(err)
	}
	pool.AddCert(leaves[0])
	client := func(http2 bool) *http.Client {
		return &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{
			TLSClientConfig:   &tls.Config{RootCAs: pool, ServerName: "wiki.example.org"},
			ForceAttemptHTTP2: http2,
		}}
	}
	url := "https://" + ln.Addr().String()
	for _, c := range []struct {
		http2 bool
		want  string
	}{{true, "HTTP/2.0 secure=true\n"}, {false, "HTTP/1.1 secure=true\n"}} {
		res, err := client(c.http2).Get(url + "/")
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if string(body) != c.want {
			t.Errorf("answer %q, want %q", body, c.want)
		}
	}

	res, err := client(true).Get(url + "/stream")
	if err != nil {
		t.Fatal(err)
	}
	lines := bufio.NewReader(res.Body)
	if first, err := lines.ReadString('\n'); err != nil || first != "HTTP/2.0 secure=true\n" {
		t.Errorf("first line of the stream %q, %v; want it before the stream ends", first, err)
	}
	close(more)
	if second, err := lines.ReadString('\n'); err != nil || second != "more\n" {
		t.Errorf("second line of the stream %q, %v", second, err)
	}
	res.Body.Close()

	// A client that does not speak TLS gets no content.
	if res, err := http.Get("http://" + ln.Addr().String() + "/"); err == nil {
		res.Body.Close()
		if res.StatusCode == http.StatusOK {
			t.Error("a request in the clear was answered")
		}
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Serve returned %v, want nil after shutdown", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after the context ended")
	}
	if out := logged.String(); !strings.Contains(out, "tls=true") {
		t.Errorf("the start does not say that TLS is served:\n%s", out)
	}
}

// TestPageVersionsItsLogosAndIcons: the page names each logo and icon by
// an address that carries its content's hash, so a browser that kept an
// earlier one fetches a changed one, and the address serves it.
func TestPageVersionsItsLogosAndIcons(t *testing.T) {
	h := testHandler(t)
	page := get(t, h, "/").Body.String()
	refs := regexp.MustCompile(`"(/(?:assets/brand/[a-z0-9-]+\.(?:svg|png)|favicon\.ico))\?v=([0-9a-f]{12})"`).FindAllStringSubmatch(page, -1)
	if len(refs) < 5 {
		t.Fatalf("versioned references: %v", refs)
	}
	for _, ref := range refs {
		if rec := get(t, h, ref[1]+"?v="+ref[2]); rec.Code != 200 {
			t.Errorf("%s answers %d", ref[1], rec.Code)
		}
	}
	if strings.Contains(page, `"/assets/brand/logo-black.svg"`) || strings.Contains(page, `"/assets/brand/favicon.svg"`) {
		t.Error("an unversioned logo or icon is left")
	}
}
