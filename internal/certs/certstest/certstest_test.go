package certstest

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteMakesAPairForItsHosts(t *testing.T) {
	certFile, keyFile := Write(t, t.TempDir(), "wiki", "wiki.example.org", "127.0.0.1")
	pair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		t.Fatal(err)
	}
	leaf := pair.Leaf
	if err := leaf.VerifyHostname("wiki.example.org"); err != nil {
		t.Error(err)
	}
	if len(leaf.IPAddresses) != 1 || !leaf.IPAddresses[0].Equal(net.ParseIP("127.0.0.1")) {
		t.Errorf("addresses %v, want 127.0.0.1", leaf.IPAddresses)
	}
}

func TestAnAuthoritySignsCertificates(t *testing.T) {
	dir := t.TempDir()
	ca := NewCA(t, dir, "ca")
	certFile, keyFile := ca.Write(t, dir, "wiki", "wiki.example.org")
	pair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := os.ReadFile(ca.File)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(bundle) {
		t.Fatal("the authority's file holds no certificate")
	}
	if _, err := pair.Leaf.Verify(x509.VerifyOptions{Roots: roots, DNSName: "wiki.example.org"}); err != nil {
		t.Errorf("a certificate of the authority does not verify against it: %v", err)
	}
	selfSigned, _ := Write(t, dir, "other", "wiki.example.org")
	other, err := tls.LoadX509KeyPair(selfSigned, filepath.Join(dir, "other.key"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Leaf.Verify(x509.VerifyOptions{Roots: roots, DNSName: "wiki.example.org"}); err == nil {
		t.Error("a self-signed certificate verifies against the authority")
	}
}
