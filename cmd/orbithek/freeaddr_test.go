package main

import (
	"net"
	"strconv"
	"testing"
)

// The addresses freeAddr hands to serve lie below every common ephemeral
// range (Linux 32768–60999, macOS and Windows 49152–65535), so that no
// other process, such as another package's tests asking for port 0, is
// given the port between freeAddr's check and serve's bind; and it hands
// out each port once per run.
func TestFreeAddrAvoidsTheEphemeralRangeAndRepeatsNoPort(t *testing.T) {
	seen := map[string]bool{}
	for range 50 {
		addr := freeAddr(t)
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			t.Fatal(err)
		}
		n, err := strconv.Atoi(port)
		if host != "127.0.0.1" || err != nil || n < freePortLow || n >= freePortHigh {
			t.Fatalf("freeAddr gave %s, want 127.0.0.1 with a port in [%d, %d)", addr, freePortLow, freePortHigh)
		}
		if seen[port] {
			t.Fatalf("freeAddr gave port %s twice", port)
		}
		seen[port] = true
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			t.Fatalf("the port freeAddr gave is not free: %v", err)
		}
		ln.Close()
	}
}
