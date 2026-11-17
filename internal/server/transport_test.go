package server

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

// TestRedirectToHTTPS: the listener for plain HTTP sends a reader to the
// same host and path over HTTPS and answers nothing else
// (ADR-draft-hsts-and-redirect).
func TestRedirectToHTTPS(t *testing.T) {
	for _, c := range []struct {
		name, port, method, host, target string
		status                           int
		location                         string
	}{
		{"a page", "", "GET", "wiki.example.org", "/a/b?x=1&y=2", http.StatusFound, "https://wiki.example.org/a/b?x=1&y=2"},
		{"the host's own port is dropped", "", "GET", "wiki.example.org:8081", "/", http.StatusFound, "https://wiki.example.org/"},
		{"another HTTPS port", "8443", "GET", "wiki.example.org:80", "/a", http.StatusFound, "https://wiki.example.org:8443/a"},
		{"the standard port is not written", "443", "GET", "wiki.example.org", "/a", http.StatusFound, "https://wiki.example.org/a"},
		{"an address", "", "GET", "192.0.2.7:80", "/", http.StatusFound, "https://192.0.2.7/"},
		{"an IPv6 address", "8443", "GET", "[2001:db8::7]:80", "/", http.StatusFound, "https://[2001:db8::7]:8443/"},
		{"an escaped path", "", "GET", "wiki.example.org", "/a%2Fb/c%20d", http.StatusFound, "https://wiki.example.org/a%2Fb/c%20d"},
		{"a path that looks like a host", "", "GET", "wiki.example.org", "//evil.example/x", http.StatusFound, "https://wiki.example.org//evil.example/x"},
		{"HEAD", "", "HEAD", "wiki.example.org", "/a", http.StatusFound, "https://wiki.example.org/a"},
		{"a write is not repeated", "", "POST", "wiki.example.org", "/api/login", http.StatusBadRequest, ""},
		{"no host", "", "GET", "", "/a", http.StatusBadRequest, ""},
		{"a host that carries a user", "", "GET", "wiki.example.org@evil.example", "/a", http.StatusBadRequest, ""},
		{"a host that carries a path", "", "GET", "evil.example/wiki.example.org", "/a", http.StatusBadRequest, ""},
	} {
		req := httptest.NewRequest(c.method, c.target, nil)
		req.Host = c.host
		rec := httptest.NewRecorder()
		RedirectToHTTPS(c.port).ServeHTTP(rec, req)
		if rec.Code != c.status || rec.Header().Get("Location") != c.location {
			t.Errorf("%s: status %d, Location %q; want %d, %q", c.name, rec.Code, rec.Header().Get("Location"), c.status, c.location)
		}
		if rec.Header().Get("Strict-Transport-Security") != "" {
			t.Errorf("%s: a transport policy was sent over plain HTTP", c.name)
		}
	}
}

func TestHSTSValue(t *testing.T) {
	for _, c := range []struct {
		in, want string
		ok       bool
	}{
		{"", "", true},
		{" 31536000 ", "max-age=31536000", true},
		{"0", "max-age=0", true},
		{"-1", "", false},
		{"on", "", false},
		{"1.5", "", false},
		{"31536000; includeSubDomains", "", false},
	} {
		got, err := HSTSValue(c.in)
		if got != c.want || (err == nil) != c.ok {
			t.Errorf("HSTSValue(%q) = %q, %v; want %q, ok %v", c.in, got, err, c.want, c.ok)
		}
	}
}

// TestHSTSOnlyOverHTTPS: the policy is sent with every answer to a request
// that arrived over HTTPS, the server's own or a trusted proxy's
// (ADR-0037), and never over plain HTTP, where RFC 6797 has it ignored.
func TestHSTSOnlyOverHTTPS(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	proxies := []netip.Prefix{netip.MustParsePrefix("10.0.0.1/32")}
	h := BehindProxies(proxies, WithHSTS("max-age=300", ok))

	send := func(h http.Handler, f func(*http.Request)) string {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "203.0.113.9:4711"
		f(req)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Header().Get("Strict-Transport-Security")
	}
	if got := send(h, func(r *http.Request) { r.TLS = &tls.ConnectionState{} }); got != "max-age=300" {
		t.Errorf("over the server's own TLS: %q", got)
	}
	if got := send(h, func(r *http.Request) {
		r.RemoteAddr = "10.0.0.1:4711"
		r.Header.Set("X-Forwarded-Proto", "https")
	}); got != "max-age=300" {
		t.Errorf("through a trusted proxy that says https: %q", got)
	}
	if got := send(h, func(*http.Request) {}); got != "" {
		t.Errorf("over plain HTTP: %q", got)
	}
	if got := send(h, func(r *http.Request) { r.Header.Set("X-Forwarded-Proto", "https") }); got != "" {
		t.Errorf("a client that only claims https: %q", got)
	}
	if got := send(WithHSTS("", ok), func(r *http.Request) { r.TLS = &tls.ConnectionState{} }); got != "" {
		t.Errorf("without the setting: %q", got)
	}
}
