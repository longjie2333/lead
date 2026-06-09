package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/turn/v4"
	"lead-turn-server/internal/auth"
	"lead-turn-server/internal/config"
	"lead-turn-server/internal/signaling"
	"lead-turn-server/internal/turnserver"
)

func TestTURNServerAllocatesRelay(t *testing.T) {
	port := freeUDPPort(t)
	cfg := config.Config{
		TURN: config.TURNConfig{
			ListenAddr: "127.0.0.1",
			Port:       port,
			PublicIP:   "127.0.0.1",
			Realm:      "lead.remoteassist",
			Username:   "lead",
			Password:   "leadpass",
		},
	}

	server, listenAddress, _, err := turnserver.Start(cfg)
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
		Username:       cfg.TURN.Username,
		Password:       cfg.TURN.Password,
		Realm:          cfg.TURN.Realm,
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
	if !relayAddr.IP.Equal(net.ParseIP(cfg.TURN.PublicIP)) {
		t.Fatalf("relay address IP = %s, want %s", relayAddr.IP, cfg.TURN.PublicIP)
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

func freeTCPPort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve TCP port: %v", err)
	}
	defer listener.Close()

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("reserved TCP address has unexpected type %T", listener.Addr())
	}
	return addr.Port
}

func TestLoadConfigRejectsInvalidRelayRange(t *testing.T) {
	cfg := config.Config{
		TURN: config.TURNConfig{
			ListenAddr: "127.0.0.1",
			Port:       3478,
			PublicIP:   "127.0.0.1",
			Realm:      "lead.remoteassist",
			Username:   "lead",
			Password:   "leadpass",
			MinPort:    55000,
			MaxPort:    50000,
		},
	}

	if _, _, _, err := turnserver.Start(cfg); err == nil {
		t.Fatal("turnserver.Start should reject an invalid relay port range")
	}
}

func TestHTTPServerServesGinRoutes(t *testing.T) {
	t.Setenv("AUTH_DB_PATH", filepath.Join(t.TempDir(), "auth.db"))
	port := freeTCPPort(t)
	cfg := config.Config{
		HTTP: config.HTTPConfig{
			Addr: "127.0.0.1",
			Port: port,
		},
		Auth: config.AuthConfig{
			DBPath:        os.Getenv("AUTH_DB_PATH"),
			AdminUsername: "admin",
			AdminPassword: "admin",
		},
		TURN: config.TURNConfig{
			Port:     3478,
			Username: "lead",
			Password: "leadpass",
		},
	}
	server, err := StartHTTPServer(cfg, "127.0.0.1")
	if err != nil {
		t.Fatalf("start HTTP server: %v", err)
	}
	defer server.Shutdown(context.Background())

	baseURL := "http://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	waitForHTTP(t, baseURL+"/")
	if status := getStatus(t, baseURL+"/viewer", ""); status != http.StatusOK {
		t.Fatalf("viewer status = %d, want %d", status, http.StatusOK)
	}
	if status := getStatus(t, baseURL+"/viewer/viewer.js", ""); status != http.StatusOK {
		t.Fatalf("viewer asset status = %d, want %d", status, http.StatusOK)
	}
	login := postJSON(t, baseURL+"/api/login", map[string]any{
		"username": "admin",
		"password": "admin",
		"source":   "viewer",
	}, "")
	if login["token"] == "" {
		t.Fatalf("login response should contain token: %#v", login)
	}
}

func TestAuthLoginBindsAndroidDeviceAndListsDevices(t *testing.T) {
	t.Setenv("AUTH_DB_PATH", filepath.Join(t.TempDir(), "auth.db"))
	auth := newTestAuthService()
	defer auth.Close()
	server := httptest.NewServer(auth.Handler())
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

	auth := newTestAuthService()
	defer auth.Close()
	server := httptest.NewServer(auth.Handler())

	login := postJSON(t, server.URL+"/api/login", map[string]any{
		"username":   "admin",
		"password":   "admin",
		"source":     "android",
		"deviceId":   "device-1",
		"deviceName": "Pixel Test",
	}, "")
	token, _ := login["token"].(string)
	server.Close()

	reloaded := newTestAuthService()
	defer reloaded.Close()
	reloadedServer := httptest.NewServer(reloaded.Handler())
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

func TestUserCRUDAndAdminPermissions(t *testing.T) {
	t.Setenv("AUTH_DB_PATH", filepath.Join(t.TempDir(), "auth.db"))
	auth := newTestAuthService()
	defer auth.Close()
	server := httptest.NewServer(auth.Handler())
	defer server.Close()

	adminLogin := postJSON(t, server.URL+"/api/login", map[string]any{
		"username": "admin",
		"password": "admin",
		"source":   "viewer",
	}, "")
	adminToken, _ := adminLogin["token"].(string)
	adminUser := adminLogin["user"].(map[string]any)
	if adminUser["role"] != "admin" || adminUser["isAdmin"] != true {
		t.Fatalf("bootstrap admin user = %#v", adminUser)
	}

	createResp := postJSONStatus(t, server.URL+"/api/users", map[string]any{
		"username": "operator",
		"password": "secret",
	}, adminToken, http.StatusCreated)
	created := createResp["user"].(map[string]any)
	operatorID, _ := created["id"].(string)
	if created["role"] != "user" || created["isAdmin"] != false {
		t.Fatalf("created user should default to normal user: %#v", created)
	}

	operatorLogin := postJSON(t, server.URL+"/api/login", map[string]any{
		"username": "operator",
		"password": "secret",
		"source":   "viewer",
	}, "")
	operatorToken, _ := operatorLogin["token"].(string)
	selfResp := getJSON(t, server.URL+"/api/users/"+operatorID, operatorToken)
	self := selfResp["user"].(map[string]any)
	if self["id"] != operatorID {
		t.Fatalf("normal user should read self: %#v", selfResp)
	}

	if status := postJSONStatusOnly(t, server.URL+"/api/users", map[string]any{
		"username": "blocked",
		"password": "secret",
	}, operatorToken); status != http.StatusForbidden {
		t.Fatalf("normal user create status = %d, want %d", status, http.StatusForbidden)
	}

	if status := putJSONStatusOnly(t, server.URL+"/api/users/"+operatorID, map[string]any{
		"role": "admin",
	}, operatorToken); status != http.StatusForbidden {
		t.Fatalf("normal user role update status = %d, want %d", status, http.StatusForbidden)
	}

	updatedResp := putJSONStatus(t, server.URL+"/api/users/"+operatorID, map[string]any{
		"role": "admin",
	}, adminToken, http.StatusOK)
	updated := updatedResp["user"].(map[string]any)
	if updated["role"] != "admin" || updated["isAdmin"] != true {
		t.Fatalf("admin should promote user: %#v", updated)
	}

	usersResp := getJSON(t, server.URL+"/api/users", adminToken)
	users := usersResp["users"].([]any)
	if len(users) != 2 {
		t.Fatalf("users response = %#v", usersResp)
	}

	deleteResp, err := http.NewRequest(http.MethodDelete, server.URL+"/api/users/"+operatorID, nil)
	if err != nil {
		t.Fatalf("new delete request: %v", err)
	}
	deleteResp.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err := http.DefaultClient.Do(deleteResp)
	if err != nil {
		t.Fatalf("delete user request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete user status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	if status := getStatus(t, server.URL+"/api/users/"+operatorID, adminToken); status != http.StatusNotFound {
		t.Fatalf("deleted user lookup status = %d, want %d", status, http.StatusNotFound)
	}
}

func TestSignalingHubRelaysViewerAndAndroidMessages(t *testing.T) {
	t.Setenv("AUTH_DB_PATH", filepath.Join(t.TempDir(), "auth.db"))
	auth := newTestAuthService()
	defer auth.Close()
	authServer := httptest.NewServer(auth.Handler())
	defer authServer.Close()
	login := postJSON(t, authServer.URL+"/api/login", map[string]any{
		"username":   "admin",
		"password":   "admin",
		"source":     "android",
		"deviceId":   "device-1",
		"deviceName": "Test Device",
	}, "")
	token, _ := login["token"].(string)
	hub := signaling.NewHub([]config.ICEServer{{URLs: "stun:test.example:19302"}}, "turn", "", auth)
	server := httptest.NewServer(hub)
	defer server.Close()

	wsURL := "ws" + server.URL[len("http"):] + "?token=" + url.QueryEscape(token) + "&deviceId=device-1"
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
	configMessage := readJSON(t, android)
	if configMessage["type"] != "config" {
		t.Fatalf("android message type = %v, want config", configMessage["type"])
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
	auth := newTestAuthService()
	defer auth.Close()
	hub := signaling.NewHub(nil, "turn", "", auth)
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

func waitForHTTP(t *testing.T, rawURL string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(rawURL)
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("HTTP server did not become ready at %s", rawURL)
}

func newTestAuthService() *auth.Service {
	return auth.NewService(config.AuthConfig{
		DBPath:        os.Getenv("AUTH_DB_PATH"),
		AdminUsername: "admin",
		AdminPassword: "admin",
	})
}

func postJSONStatus(t *testing.T, rawURL string, body map[string]any, token string, wantStatus int) map[string]any {
	t.Helper()
	return jsonRequest(t, http.MethodPost, rawURL, body, token, wantStatus)
}

func putJSONStatus(t *testing.T, rawURL string, body map[string]any, token string, wantStatus int) map[string]any {
	t.Helper()
	return jsonRequest(t, http.MethodPut, rawURL, body, token, wantStatus)
}

func postJSONStatusOnly(t *testing.T, rawURL string, body map[string]any, token string) int {
	t.Helper()
	return jsonStatus(t, http.MethodPost, rawURL, body, token)
}

func putJSONStatusOnly(t *testing.T, rawURL string, body map[string]any, token string) int {
	t.Helper()
	return jsonStatus(t, http.MethodPut, rawURL, body, token)
}

func jsonRequest(t *testing.T, method string, rawURL string, body map[string]any, token string, wantStatus int) map[string]any {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	req, err := http.NewRequest(method, rawURL, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s json: %v", method, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		t.Fatalf("status = %d, want %d", resp.StatusCode, wantStatus)
	}
	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	return data
}

func jsonStatus(t *testing.T, method string, rawURL string, body map[string]any, token string) int {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	req, err := http.NewRequest(method, rawURL, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s json status: %v", method, err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}
