package certs

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dbuchs7/orbithek/internal/certs/certstest"
)

func TestPairsMatchesCertificatesWithKeysInOrder(t *testing.T) {
	got, err := Pairs([]string{"a.crt", "b.crt"}, []string{"a.key", "b.key"})
	if err != nil {
		t.Fatal(err)
	}
	want := []Pair{{"a.crt", "a.key"}, {"b.crt", "b.key"}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("pairs %v, want %v", got, want)
	}
	if got, err := Pairs(nil, nil); err != nil || got != nil {
		t.Errorf("no files: %v, %v; want no pairs and no error", got, err)
	}
	for _, c := range []struct{ certs, keys []string }{
		{[]string{"a.crt"}, nil},
		{nil, []string{"a.key"}},
		{[]string{"a.crt", "b.crt"}, []string{"a.key"}},
	} {
		if _, err := Pairs(c.certs, c.keys); err == nil {
			t.Errorf("%v with %v: no error for certificates without their keys", c.certs, c.keys)
		}
	}
}

// serve answers TLS handshakes with the store's configuration and returns
// the address to dial.
func serve(t *testing.T, s *Store) string {
	t.Helper()
	ln, err := tls.Listen("tcp", "127.0.0.1:0", s.Config())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_ = conn.(*tls.Conn).Handshake()
			}()
		}
	}()
	return ln.Addr().String()
}

// presented dials addr asking for name and returns the certificate the
// server presented, unverified: the tests compare it with the files.
func presented(t *testing.T, addr, name string, more ...func(*tls.Config)) *x509.Certificate {
	t.Helper()
	cfg := &tls.Config{ServerName: name, InsecureSkipVerify: true}
	for _, f := range more {
		f(cfg)
	}
	conn, err := tls.Dial("tcp", addr, cfg)
	if err != nil {
		t.Fatalf("dial for %q: %v", name, err)
	}
	defer conn.Close()
	return conn.ConnectionState().PeerCertificates[0]
}

func leaf(t *testing.T, certFile string) *x509.Certificate {
	t.Helper()
	leaves, err := Leaves([]string{certFile})
	if err != nil {
		t.Fatal(err)
	}
	return leaves[0]
}

func open(t *testing.T, log *slog.Logger, pairs ...Pair) *Store {
	t.Helper()
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	s, err := Open(pairs, log)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// TestTheNameAskedForChoosesTheCertificate: with several pairs, the
// certificate presented is the one made for the name the client asks for
// (SNI); a name none is made for, and a client that names none, get the
// first.
func TestTheNameAskedForChoosesTheCertificate(t *testing.T) {
	dir := t.TempDir()
	wikiCert, wikiKey := certstest.Write(t, dir, "wiki", "wiki.example.org")
	docsCert, docsKey := certstest.Write(t, dir, "docs", "docs.example.com", "*.docs.example.com")
	addr := serve(t, open(t, nil, Pair{wikiCert, wikiKey}, Pair{docsCert, docsKey}))

	for _, c := range []struct{ name, want string }{
		{"wiki.example.org", wikiCert},
		{"docs.example.com", docsCert},
		{"team.docs.example.com", docsCert},
		{"unknown.example.net", wikiCert},
		{"", wikiCert},
	} {
		if got := presented(t, addr, c.name); !got.Equal(leaf(t, c.want)) {
			t.Errorf("asked for %q: got the certificate of %v, want the one in %s", c.name, got.DNSNames, filepath.Base(c.want))
		}
	}
}

// TestARenewedCertificateIsServedWithoutARestart: the pair is read again
// when its files change.
func TestARenewedCertificateIsServedWithoutARestart(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := certstest.Write(t, dir, "wiki", "wiki.example.org")
	var logged bytes.Buffer
	addr := serve(t, open(t, slog.New(slog.NewTextHandler(&logged, nil)), Pair{certFile, keyFile}))
	before := presented(t, addr, "wiki.example.org")

	certstest.Write(t, dir, "wiki", "wiki.example.org")
	later := time.Now().Add(time.Minute)
	for _, f := range []string{certFile, keyFile} {
		if err := os.Chtimes(f, later, later); err != nil {
			t.Fatal(err)
		}
	}
	after := presented(t, addr, "wiki.example.org")
	if after.Equal(before) {
		t.Fatal("the certificate from before the renewal is still served")
	}
	if !after.Equal(leaf(t, certFile)) {
		t.Error("the certificate served is not the one in the file")
	}
	if !strings.Contains(logged.String(), "read the changed certificate") {
		t.Errorf("the renewal was not logged:\n%s", logged.String())
	}
}

// TestAChangeThatCannotBeReadKeepsTheCertificate: a renewal caught half
// written, or a file gone, leaves the loaded certificate in use and warns
// once, not on every handshake; the next change is read again.
func TestAChangeThatCannotBeReadKeepsTheCertificate(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := certstest.Write(t, dir, "wiki", "wiki.example.org")
	var logged bytes.Buffer
	addr := serve(t, open(t, slog.New(slog.NewTextHandler(&logged, nil)), Pair{certFile, keyFile}))
	before := presented(t, addr, "wiki.example.org")

	// The certificate is the new one, the key still the old one.
	otherCert, _ := certstest.Write(t, t.TempDir(), "wiki", "wiki.example.org")
	renewed, err := os.ReadFile(otherCert)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(certFile, renewed, 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(certFile, later, later); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if got := presented(t, addr, "wiki.example.org"); !got.Equal(before) {
			t.Fatal("a pair that does not belong together replaced the loaded certificate")
		}
	}
	if n := strings.Count(logged.String(), "cannot read the changed certificate"); n != 1 {
		t.Errorf("warned %d times, want once:\n%s", n, logged.String())
	}

	if err := os.Remove(keyFile); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if got := presented(t, addr, "wiki.example.org"); !got.Equal(before) {
			t.Fatal("a missing key file replaced the loaded certificate")
		}
	}
	if n := strings.Count(logged.String(), "cannot read the changed certificate"); n != 2 {
		t.Errorf("warned %d times after the key went missing, want twice in all:\n%s", n, logged.String())
	}

	// The renewal completes.
	certstest.Write(t, dir, "wiki", "wiki.example.org")
	latest := later.Add(time.Minute)
	for _, f := range []string{certFile, keyFile} {
		if err := os.Chtimes(f, latest, latest); err != nil {
			t.Fatal(err)
		}
	}
	if got := presented(t, addr, "wiki.example.org"); !got.Equal(leaf(t, certFile)) {
		t.Error("the completed renewal is not served")
	}
}

func TestOpenRefusesWhatItCannotServe(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := certstest.Write(t, dir, "wiki", "wiki.example.org")
	_, otherKey := certstest.Write(t, dir, "other", "other.example.org")
	garbage := filepath.Join(dir, "garbage.pem")
	if err := os.WriteFile(garbage, []byte("not PEM"), 0o600); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.DiscardHandler)
	for name, pairs := range map[string][]Pair{
		"no pair":               nil,
		"a missing file":        {{filepath.Join(dir, "none.crt"), keyFile}},
		"a missing key":         {{certFile, filepath.Join(dir, "none.key")}},
		"a key of another":      {{certFile, otherKey}},
		"a file that is no PEM": {{garbage, keyFile}},
	} {
		if _, err := Open(pairs, log); err == nil {
			t.Errorf("%s: Open returned no error", name)
		}
	}
}

func TestOpenLogsWhatEachCertificateIsFor(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := certstest.Write(t, dir, "wiki", "wiki.example.org", "127.0.0.1")
	var logged bytes.Buffer
	open(t, slog.New(slog.NewTextHandler(&logged, nil)), Pair{certFile, keyFile})
	if out := logged.String(); !strings.Contains(out, "wiki.example.org,127.0.0.1") || !strings.Contains(out, "not-after=") {
		t.Errorf("the start does not say what the certificate is for and until when:\n%s", out)
	}
}

// TestOnlyCurrentProtocolsAndCiphers: TLS 1.2 with a cipher that
// authenticates what it encrypts is the least the server accepts.
func TestOnlyCurrentProtocolsAndCiphers(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := certstest.Write(t, dir, "wiki", "wiki.example.org")
	addr := serve(t, open(t, nil, Pair{certFile, keyFile}))

	dial := func(f func(*tls.Config)) (uint16, error) {
		cfg := &tls.Config{ServerName: "wiki.example.org", InsecureSkipVerify: true}
		f(cfg)
		conn, err := tls.Dial("tcp", addr, cfg)
		if err != nil {
			return 0, err
		}
		defer conn.Close()
		return conn.ConnectionState().Version, nil
	}
	if v, err := dial(func(*tls.Config) {}); err != nil || v != tls.VersionTLS13 {
		t.Errorf("a current client: version %x, %v; want TLS 1.3", v, err)
	}
	if v, err := dial(func(c *tls.Config) { c.MaxVersion = tls.VersionTLS12 }); err != nil || v != tls.VersionTLS12 {
		t.Errorf("a TLS 1.2 client: version %x, %v; want TLS 1.2", v, err)
	}
	if _, err := dial(func(c *tls.Config) { c.MinVersion, c.MaxVersion = tls.VersionTLS10, tls.VersionTLS11 }); err == nil {
		t.Error("a TLS 1.1 client was served")
	}
	if _, err := dial(func(c *tls.Config) {
		c.MaxVersion = tls.VersionTLS12
		c.CipherSuites = []uint16{tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA, tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA}
	}); err == nil {
		t.Error("a client offering only CBC ciphers was served")
	}
}

// TestPinnedTrustsExactlyTheConfiguredCertificates: the probe's client
// accepts a server that presents one of the certificate files, whatever
// name it is reached by, and no other.
func TestPinnedTrustsExactlyTheConfiguredCertificates(t *testing.T) {
	dir := t.TempDir()
	wikiCert, wikiKey := certstest.Write(t, dir, "wiki", "wiki.example.org")
	docsCert, docsKey := certstest.Write(t, dir, "docs", "docs.example.com")
	strangerCert, strangerKey := certstest.Write(t, dir, "stranger", "wiki.example.org")
	ours := serve(t, open(t, nil, Pair{wikiCert, wikiKey}, Pair{docsCert, docsKey}))
	theirs := serve(t, open(t, nil, Pair{strangerCert, strangerKey}))

	cfg, err := Pinned([]string{wikiCert, docsCert})
	if err != nil {
		t.Fatal(err)
	}
	dial := func(addr string) error {
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second}, "tcp", addr, cfg)
		if err != nil {
			return err
		}
		return conn.Close()
	}
	if err := dial(ours); err != nil {
		t.Errorf("the server with the configured certificates: %v", err)
	}
	if err := dial(theirs); err == nil {
		t.Error("a server with another certificate for the same name was trusted")
	}

	if _, err := Pinned([]string{filepath.Join(dir, "none.crt")}); err == nil {
		t.Error("a missing certificate file: no error")
	}
	if _, err := Pinned([]string{wikiKey}); err == nil {
		t.Error("a file without a certificate: no error")
	}
	if _, err := Pinned(nil); err == nil {
		t.Error("no certificate file: no error")
	}
}
