//go:build linux

package namespace

import (
	"net"
	"slices"
	"testing"
)

// The socket tables of the real kernel: a TCP listener and a bound UDP socket
// are both read as held, and nothing was bound to find out.
func TestBoundPortsInRange_readsTheKernelsSocketTables(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	tcpPort, udpPort := ln.Addr().(*net.TCPAddr).Port, pc.LocalAddr().(*net.UDPAddr).Port

	for _, p := range []int{tcpPort, udpPort} {
		held, err := BoundPortsInRange(p, p)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(held, []int{p}) {
			t.Errorf("port %d: held %v, want [%d]", p, held, p)
		}
	}
}
