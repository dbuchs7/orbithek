package certstest

import (
	"crypto/tls"
	"net"
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
