package main

import (
	"encoding/json"
	"net"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"
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

func TestSignalingHubRelaysViewerAndAndroidMessages(t *testing.T) {
	hub := newSignalingHub([]iceServer{{URLs: "stun:test.example:19302"}}, "turn", "")
	server := httptest.NewServer(hub)
	defer server.Close()

	wsURL := "ws" + server.URL[len("http"):]
	android := dialTestWebSocket(t, wsURL)
	defer android.Close()
	viewer := dialTestWebSocket(t, wsURL)
	defer viewer.Close()

	writeJSON(t, viewer, map[string]any{"type": "hello", "role": "viewer"})
	waiting := readJSON(t, viewer)
	if waiting["type"] != "waiting" {
		t.Fatalf("viewer initial message type = %v, want waiting", waiting["type"])
	}
	if waiting["relayMode"] != "turn" {
		t.Fatalf("relayMode = %v, want turn", waiting["relayMode"])
	}

	writeJSON(t, android, map[string]any{
		"type":   "hello",
		"role":   "android",
		"width":  float64(1080),
		"height": float64(2400),
	})
	config := readJSON(t, android)
	if config["type"] != "config" {
		t.Fatalf("android message type = %v, want config", config["type"])
	}
	streamInfo := readJSON(t, viewer)
	if streamInfo["type"] != "stream-info" || streamInfo["width"] != float64(1080) {
		t.Fatalf("viewer stream info = %#v", streamInfo)
	}
	joined := readJSON(t, android)
	if joined["type"] != "viewer-joined" {
		t.Fatalf("android joined message = %#v", joined)
	}

	writeJSON(t, viewer, map[string]any{"type": "control", "action": "tap", "x": 0.5, "y": 0.25})
	control := readJSON(t, android)
	if control["type"] != "control" || control["viewerId"] == "" {
		t.Fatalf("android control message = %#v", control)
	}
}

func dialTestWebSocket(t *testing.T, rawURL string) *websocket.Conn {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse websocket URL: %v", err)
	}
	conn, _, err := websocket.DefaultDialer.Dial(parsed.String(), nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	return conn
}

func writeJSON(t *testing.T, conn *websocket.Conn, data map[string]any) {
	t.Helper()
	conn.SetWriteDeadline(time.Now().Add(time.Second))
	if err := conn.WriteJSON(data); err != nil {
		t.Fatalf("write websocket JSON: %v", err)
	}
}

func readJSON(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(time.Second))
	_, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read websocket: %v", err)
	}
	var data map[string]any
	if err := json.Unmarshal(payload, &data); err != nil {
		t.Fatalf("decode websocket JSON: %v", err)
	}
	return data
}
