package photosource

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// serve starts a TLS test server that answers for every host: the client
// it answers dials the server whatever the address names and checks the
// server's certificate, so the production addresses of Google and iCloud,
// and the checks of the hosts they hand out, are used unchanged.
func serve(t *testing.T, h http.Handler) *http.Client {
	t.Helper()
	srv := httptest.NewTLSServer(h)
	t.Cleanup(srv.Close)
	base := srv.Client().Transport.(*http.Transport).Clone()
	base.TLSClientConfig.ServerName = "example.com"
	addr := srv.Listener.Addr().String()
	base.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, network, addr)
	}
	return &http.Client{Transport: base}
}

// hostOf answers the host a request was sent to, without a port.
func hostOf(r *http.Request) string {
	if h, _, err := net.SplitHostPort(r.Host); err == nil {
		return h
	}
	return r.Host
}

func wantErr(t *testing.T, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("error %v, want %v", err, target)
	}
}
