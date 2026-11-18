package engine

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dbuchs7/orbithek/internal/certs"
	"github.com/dbuchs7/orbithek/internal/certs/certstest"
	"github.com/dbuchs7/orbithek/internal/site"
)

// TestEnginesWithAnOrganisationsOwnCertificate: an engine whose
// certificate an organisation's own authority signed is read once the
// client is given that authority, and refused with the host's authorities
// alone; the authority changes nothing about which addresses an engine may
// have (ADR-draft-engine-ca-bundle).
func TestEnginesWithAnOrganisationsOwnCertificate(t *testing.T) {
	dir := t.TempDir()
	ca := certstest.NewCA(t, dir, "ca")
	certFile, keyFile := ca.Write(t, dir, "atlas", "127.0.0.1")
	pair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`[{"key":1,"processId":"order","name":"Order","version":1,"executable":true}]`))
	}))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{pair}}
	srv.StartTLS()
	defer srv.Close()

	roots, err := certs.Roots(ca.File)
	if err != nil {
		t.Fatal(err)
	}
	env := func(private bool) func(string) string {
		return func(k string) string {
			switch k {
			case "OT_ENGINE_ATLAS_TOKEN":
				return "secret"
			case AllowPrivateVar:
				if private {
					return "on"
				}
			}
			return ""
		}
	}
	e := site.Engine{ID: "atlas", Name: "Atlas", Kind: site.EngineAtlas, BaseURL: srv.URL}
	ctx := context.Background()

	trusting := NewReader(env(true), NewClientTrusting(env(true), roots))
	got, err := trusting.Processes(ctx, e)
	if err != nil || len(got) != 1 || got[0].ID != "order" {
		t.Errorf("with the authority: %v, %v; want the engine's process", got, err)
	}

	if _, err := NewReader(env(true), NewClient(env(true))).Processes(ctx, e); err == nil {
		t.Error("the host's authorities alone trusted the organisation's own")
	}
	if _, err := NewReader(env(true), NewClientTrusting(env(true), nil)).Processes(ctx, e); err == nil {
		t.Error("without an authority the engine's certificate was trusted")
	}

	other, err := certs.Roots(certstest.NewCA(t, dir, "other").File)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewReader(env(true), NewClientTrusting(env(true), other)).Processes(ctx, e); err == nil {
		t.Error("another authority vouched for the engine")
	}

	// The engine is in the server's own network: the authority does not
	// open that; OT_ENGINE_ALLOW_PRIVATE still decides.
	_, err = NewReader(env(false), NewClientTrusting(env(false), roots)).Processes(ctx, e)
	if !errors.Is(err, ErrUnavailable) || !strings.Contains(err.Error(), AllowPrivateVar) {
		t.Errorf("an engine in the own network without %s: %v; want the refusal of a private address", AllowPrivateVar, err)
	}
}
