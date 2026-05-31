package main

import (
	"net"
	"testing"
	"time"

	"github.com/pion/turn/v4"
)

func TestTURNServerAllocatesRelay(t *testing.T) {
	port := freeUDPPort(t)
	cfg := config{
		ListenAddr: "127.0.0.1",
		Port:       port,
		PublicIP:   "127.0.0.1",
		Realm:      "lead.remoteassist",
		Username:   "lead",
		Password:   "leadpass",
	}

	server, listenAddress, _, err := startTURNServer(cfg)
	if err != nil {
		t.Fatalf("start TURN server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Fatalf("close TURN server: %v", err)
		}
	}()

	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen client UDP: %v", err)
	}
	defer conn.Close()

	client, err := turn.NewClient(&turn.ClientConfig{
		TURNServerAddr: listenAddress,
		Username:       cfg.Username,
		Password:       cfg.Password,
		Realm:          cfg.Realm,
		RTO:            50 * time.Millisecond,
		Conn:           conn,
	})
	if err != nil {
		t.Fatalf("new TURN client: %v", err)
	}
	defer client.Close()

	if err := client.Listen(); err != nil {
		t.Fatalf("listen TURN client: %v", err)
	}

	relayConn, err := client.Allocate()
	if err != nil {
		t.Fatalf("allocate relay: %v", err)
	}
	defer relayConn.Close()

	relayAddr, ok := relayConn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("relay address has unexpected type %T", relayConn.LocalAddr())
	}
	if !relayAddr.IP.Equal(net.ParseIP(cfg.PublicIP)) {
		t.Fatalf("relay address IP = %s, want %s", relayAddr.IP, cfg.PublicIP)
	}
	if relayAddr.Port == 0 {
		t.Fatal("relay address should include an allocated port")
	}
}

func freeUDPPort(t *testing.T) int {
	t.Helper()

	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve UDP port: %v", err)
	}
	defer conn.Close()

	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("reserved address has unexpected type %T", conn.LocalAddr())
	}
	return addr.Port
}

func TestLoadConfigRejectsInvalidRelayRange(t *testing.T) {
	cfg := config{
		ListenAddr: "127.0.0.1",
		Port:       3478,
		PublicIP:   "127.0.0.1",
		Realm:      "lead.remoteassist",
		Username:   "lead",
		Password:   "leadpass",
		MinPort:    55000,
		MaxPort:    50000,
	}

	if _, _, _, err := startTURNServer(cfg); err == nil {
		t.Fatal("startTURNServer should reject an invalid relay port range")
	}
}
