package main

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
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

func TestAuthLoginBindsAndroidDeviceAndListsDevices(t *testing.T) {
	t.Setenv("AUTH_DB_PATH", filepath.Join(t.TempDir(), "auth.db"))
	auth := newAuthService()
	defer auth.close()
	hub := newSignalingHub(nil, "turn", "", auth)
	server := httptest.NewServer(auth.routes(hub))
	defer server.Close()

	androidLogin := postJSON(t, server.URL+"/api/login", map[string]any{
		"username":   "admin",
		"password":   "admin",
		"source":     "android",
		"deviceId":   "device-1",
		"deviceName": "Pixel Test",
	}, "")
	if androidLogin["token"] == "" {
		t.Fatalf("android login should return a token: %#v", androidLogin)
	}
	device, ok := androidLogin["device"].(map[string]any)
	if !ok || device["id"] != "device-1" || device["name"] != "Pixel Test" {
		t.Fatalf("android login should bind device: %#v", androidLogin["device"])
	}
	postJSON(t, server.URL+"/api/login", map[string]any{
		"username":   "admin",
		"password":   "admin",
		"source":     "android",
		"deviceId":   "device-2",
		"deviceName": "Tablet Test",
	}, "")

	viewerLogin := postJSON(t, server.URL+"/api/login", map[string]any{
		"username": "admin",
		"password": "admin",
		"source":   "viewer",
	}, "")
	devices, ok := viewerLogin["devices"].([]any)
	if !ok || len(devices) != 2 {
		t.Fatalf("viewer login devices = %#v", viewerLogin["devices"])
	}

	token, _ := viewerLogin["token"].(string)
	devicesResp := getJSON(t, server.URL+"/api/devices", token)
	list, ok := devicesResp["devices"].([]any)
	if !ok || len(list) != 2 {
		t.Fatalf("devices response = %#v", devicesResp)
	}

	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/logout", nil)
	if err != nil {
		t.Fatalf("new logout request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("logout request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("logout status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	devicesStatus := getStatus(t, server.URL+"/api/devices", token)
	if devicesStatus != http.StatusUnauthorized {
		t.Fatalf("devices after logout status = %d, want %d", devicesStatus, http.StatusUnauthorized)
	}
}

func TestAuthStorePersistsTokenAndDevices(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "auth.db")
	t.Setenv("AUTH_DB_PATH", storePath)

	auth := newAuthService()
	defer auth.close()
	hub := newSignalingHub(nil, "turn", "", auth)
	server := httptest.NewServer(auth.routes(hub))

	login := postJSON(t, server.URL+"/api/login", map[string]any{
		"username":   "admin",
		"password":   "admin",
		"source":     "android",
		"deviceId":   "device-1",
		"deviceName": "Pixel Test",
	}, "")
	token, _ := login["token"].(string)
	server.Close()

	reloaded := newAuthService()
	defer reloaded.close()
	reloadedHub := newSignalingHub(nil, "turn", "", reloaded)
	reloadedServer := httptest.NewServer(reloaded.routes(reloadedHub))
	defer reloadedServer.Close()

	devices := getJSON(t, reloadedServer.URL+"/api/devices", token)
	list, ok := devices["devices"].([]any)
	if !ok || len(list) != 1 {
		t.Fatalf("reloaded devices = %#v", devices)
	}
	record := list[0].(map[string]any)
	if record["id"] != "device-1" || record["online"] != false {
		t.Fatalf("reloaded device = %#v", record)
	}
}

func TestSignalingHubRelaysViewerAndAndroidMessages(t *testing.T) {
	t.Setenv("AUTH_DB_PATH", filepath.Join(t.TempDir(), "auth.db"))
	auth := newAuthService()
	defer auth.close()
	auth.mu.Lock()
	auth.tokens["test-token"] = "admin"
	auth.devices["device-1"] = deviceRecord{ID: "device-1", AccountID: "admin", Name: "Test Device"}
	auth.mu.Unlock()
	hub := newSignalingHub([]iceServer{{URLs: "stun:test.example:19302"}}, "turn", "", auth)
	server := httptest.NewServer(hub)
	defer server.Close()

	wsURL := "ws" + server.URL[len("http"):] + "?token=test-token&deviceId=device-1"
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
		"type":     "hello",
		"role":     "android",
		"deviceId": "device-1",
		"width":    float64(1080),
		"height":   float64(2400),
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

func TestSignalingHubRejectsUnauthorizedWebSocket(t *testing.T) {
	t.Setenv("AUTH_DB_PATH", filepath.Join(t.TempDir(), "auth.db"))
	auth := newAuthService()
	defer auth.close()
	hub := newSignalingHub(nil, "turn", "", auth)
	server := httptest.NewServer(hub)
	defer server.Close()

	wsURL := "ws" + server.URL[len("http"):]
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		t.Fatal("anonymous websocket dial should fail")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous websocket status = %#v, want %d", resp, http.StatusUnauthorized)
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

func postJSON(t *testing.T, rawURL string, body map[string]any, token string) map[string]any {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, rawURL, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post json: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	return data
}

func getJSON(t *testing.T, rawURL string, token string) map[string]any {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get json: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	return data
}

func getStatus(t *testing.T, rawURL string, token string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}
