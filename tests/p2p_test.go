//go:build p2p

package tests

import (
	"io"
	"log"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	client "github.com/Xenthera/chute-client"
)

func TestP2PHarness(t *testing.T) {
	log.SetOutput(io.Discard)

	serverAddr := os.Getenv("CHUTE_SERVER")
	if serverAddr == "" {
		serverAddr = "localhost:8080"
	}
	serverAddr = strings.Replace(serverAddr, "localhost", "127.0.0.1", 1)

	aConn := mustListenUDP(t)
	bConn := mustListenUDP(t)
	cConn := mustListenUDP(t)
	defer aConn.Close()
	defer bConn.Close()
	defer cConn.Close()

	a := client.NewChuteSession(aConn, "A")
	b := client.NewChuteSession(bConn, "B")
	c := client.NewChuteSession(cConn, "C")
	a.Start()
	b.Start()
	c.Start()

	registerClient(t, serverAddr, "A", aConn)
	registerClient(t, serverAddr, "B", bConn)
	registerClient(t, serverAddr, "C", cConn)
	defer client.UnregisterWithServer(serverAddr, "A")
	defer client.UnregisterWithServer(serverAddr, "B")
	defer client.UnregisterWithServer(serverAddr, "C")

	t.Log("test 1: connect A -> B")
	bEndpoint, err := client.LookupPeer(serverAddr, "B")
	if err != nil {
		t.Fatalf("lookup B failed: %v", err)
	}
	if err := a.Connect(bEndpoint, "B"); err != nil {
		t.Fatalf("connect A->B failed: %v", err)
	}
	if err := a.Send([]byte("hello B")); err != nil {
		t.Fatalf("send A->B failed: %v", err)
	}
	expectReceive(t, b, "hello B", 2*time.Second)

	t.Log("test 2: busy check C -> B")
	bEndpoint, err = client.LookupPeer(serverAddr, "B")
	if err != nil {
		t.Fatalf("lookup B failed: %v", err)
	}
	if err := c.Connect(bEndpoint, "B"); err == nil {
		t.Fatalf("expected busy on connect C->B")
	}

	if !b.IsConnectedTo("A") {
		t.Fatalf("expected B to remain connected to A")
	}
}

func mustListenUDP(t *testing.T) *net.UDPConn {
	t.Helper()
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	return conn
}

func registerClient(t *testing.T, serverAddr, id string, conn *net.UDPConn) {
	t.Helper()
	port := conn.LocalAddr().(*net.UDPAddr).Port
	if err := client.RegisterWithServer(serverAddr, id, port); err != nil {
		t.Fatalf("register %s failed: %v", id, err)
	}
}

func expectReceive(t *testing.T, session *client.ChuteSession, expected string, timeout time.Duration) {
	t.Helper()
	select {
	case msg := <-session.ReceiveChan:
		if string(msg) != expected {
			t.Fatalf("unexpected message: %q", string(msg))
		}
	case <-time.After(timeout):
		t.Fatalf("timeout waiting for message: %s", expected)
	}
}
