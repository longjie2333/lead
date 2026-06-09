package signaling

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"lead-turn-server/internal/auth"
	"lead-turn-server/internal/config"
)

type Hub struct {
	upgrader websocket.Upgrader
	nextID   atomic.Uint64

	mu         sync.Mutex
	auth       *auth.Service
	iceServers []config.ICEServer
	mode       string
	sfuURL     string
	devices    map[string]*deviceSession
}

type deviceSession struct {
	accountID             string
	deviceID              string
	activeAndroid         *signalClient
	waitingAndroidClients map[*signalClient]struct{}
	viewers               map[string]*signalClient
	streamInfo            map[string]any
}

type signalClient struct {
	id        string
	role      string
	accountID string
	deviceID  string
	conn      *websocket.Conn
	mu        sync.Mutex
}

func NewHub(iceServers []config.ICEServer, mode string, sfuURL string, auth *auth.Service) *Hub {
	return &Hub{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		auth:       auth,
		iceServers: iceServers,
		mode:       mode,
		sfuURL:     sfuURL,
		devices:    make(map[string]*deviceSession),
	}
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	token := auth.BearerToken(r)
	accountID, ok := h.auth.AccountForToken(token)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	deviceID := r.URL.Query().Get("deviceId")
	if deviceID != "" && !h.auth.DeviceBelongsTo(accountID, deviceID) {
		http.Error(w, "device not found", http.StatusForbidden)
		return
	}
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade websocket: %v", err)
		return
	}

	client := &signalClient{
		id:        fmt.Sprintf("viewer-%d", h.nextID.Add(1)),
		role:      "unknown",
		accountID: accountID,
		deviceID:  deviceID,
		conn:      conn,
	}
	defer h.removeClient(client)
	defer conn.Close()

	for {
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if messageType != websocket.TextMessage {
			continue
		}
		h.handleControl(client, payload)
	}
}

func (h *Hub) handleControl(client *signalClient, payload []byte) {
	var data map[string]any
	if err := json.Unmarshal(payload, &data); err != nil {
		return
	}

	msgType, _ := data["type"].(string)
	role, _ := data["role"].(string)
	if incomingDeviceID, _ := data["deviceId"].(string); client.deviceID == "" && incomingDeviceID != "" {
		client.deviceID = incomingDeviceID
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if msgType == "hello" {
		switch role {
		case "android":
			h.registerAndroidLocked(client, data)
		case "android-waiting":
			h.registerWaitingAndroidLocked(client)
		case "viewer":
			h.registerViewerLocked(client)
		}
		return
	}

	session := h.devices[client.deviceID]
	if session == nil {
		client.send(h.withServerConfig(map[string]any{"type": "waiting"}))
		return
	}

	switch client.role {
	case "android":
		if msgType == "stream-info" {
			session.streamInfo = mergeMap(session.streamInfo, data)
			h.broadcastToViewers(session, h.withServerConfig(cloneMap(data)))
			return
		}
		viewerID, _ := data["viewerId"].(string)
		if viewerID == "" {
			return
		}
		if viewer := session.viewers[viewerID]; viewer != nil {
			viewer.send(data)
		}
	case "viewer":
		if msgType == "ping" {
			client.send(map[string]any{
				"type":         "pong",
				"clientTimeMs": data["clientTimeMs"],
				"serverTimeMs": nowMillis(),
			})
			return
		}
		next := cloneMap(data)
		next["viewerId"] = client.id
		next["deviceId"] = client.deviceID
		h.sendToAndroidLocked(session, next)
	}
}

func (h *Hub) registerAndroidLocked(client *signalClient, data map[string]any) {
	if client.deviceID == "" || !h.auth.DeviceBelongsTo(client.accountID, client.deviceID) {
		client.send(map[string]any{"type": "error", "message": "deviceId is required"})
		return
	}
	session := h.sessionLocked(client.accountID, client.deviceID)
	client.role = "android"
	session.activeAndroid = client
	session.streamInfo = cloneMap(data)
	session.streamInfo["type"] = "stream-info"
	h.auth.SetDeviceOnline(client.accountID, client.deviceID, true)
	client.send(h.withServerConfig(map[string]any{"type": "config"}))
	h.broadcastToViewers(session, h.withServerConfig(cloneMap(session.streamInfo)))
	for _, viewer := range session.viewers {
		h.sendToAndroidLocked(session, map[string]any{"type": "viewer-joined", "viewerId": viewer.id, "deviceId": client.deviceID})
	}
}

func (h *Hub) registerWaitingAndroidLocked(client *signalClient) {
	if client.deviceID == "" || !h.auth.DeviceBelongsTo(client.accountID, client.deviceID) {
		client.send(map[string]any{"type": "error", "message": "deviceId is required"})
		return
	}
	session := h.sessionLocked(client.accountID, client.deviceID)
	client.role = "android-waiting"
	session.waitingAndroidClients[client] = struct{}{}
	h.auth.SetDeviceOnline(client.accountID, client.deviceID, false)
	client.send(h.withServerConfig(map[string]any{"type": "config"}))
	if session.activeAndroid == nil && len(session.viewers) > 0 {
		client.send(map[string]any{"type": "remote-request", "deviceId": client.deviceID})
	}
}

func (h *Hub) registerViewerLocked(client *signalClient) {
	if client.deviceID == "" || !h.auth.DeviceBelongsTo(client.accountID, client.deviceID) {
		client.send(map[string]any{"type": "error", "message": "viewer deviceId is required"})
		return
	}
	session := h.sessionLocked(client.accountID, client.deviceID)
	client.role = "viewer"
	session.viewers[client.id] = client
	if session.activeAndroid != nil {
		client.send(h.withServerConfig(cloneMap(session.streamInfo)))
		h.sendToAndroidLocked(session, map[string]any{"type": "viewer-joined", "viewerId": client.id, "deviceId": client.deviceID})
		return
	}
	client.send(h.withServerConfig(map[string]any{"type": "waiting"}))
	h.notifyWaitingAndroidLocked(session)
}

func (h *Hub) removeClient(client *signalClient) {
	h.mu.Lock()
	defer h.mu.Unlock()

	session := h.devices[client.deviceID]
	if session == nil {
		return
	}
	switch client.role {
	case "android":
		if session.activeAndroid == client {
			session.activeAndroid = nil
			session.streamInfo = nil
			h.auth.SetDeviceOnline(client.accountID, client.deviceID, false)
			h.broadcastToViewers(session, map[string]any{"type": "android-disconnected", "deviceId": client.deviceID})
		}
	case "android-waiting":
		delete(session.waitingAndroidClients, client)
	case "viewer":
		delete(session.viewers, client.id)
		h.sendToAndroidLocked(session, map[string]any{"type": "viewer-left", "viewerId": client.id, "deviceId": client.deviceID})
	}
	if session.activeAndroid == nil && len(session.waitingAndroidClients) == 0 && len(session.viewers) == 0 {
		delete(h.devices, client.deviceID)
	}
}

func (h *Hub) sessionLocked(accountID, deviceID string) *deviceSession {
	session := h.devices[deviceID]
	if session == nil {
		session = &deviceSession{
			accountID:             accountID,
			deviceID:              deviceID,
			waitingAndroidClients: make(map[*signalClient]struct{}),
			viewers:               make(map[string]*signalClient),
		}
		h.devices[deviceID] = session
	}
	return session
}

func (h *Hub) sendToAndroidLocked(session *deviceSession, data map[string]any) {
	if session.activeAndroid != nil {
		session.activeAndroid.send(data)
	}
}

func (h *Hub) notifyWaitingAndroidLocked(session *deviceSession) {
	for client := range session.waitingAndroidClients {
		client.send(map[string]any{"type": "remote-request", "deviceId": session.deviceID})
	}
}

func (h *Hub) broadcastToViewers(session *deviceSession, data map[string]any) {
	for _, viewer := range session.viewers {
		viewer.send(data)
	}
}

func (h *Hub) withServerConfig(data map[string]any) map[string]any {
	if data == nil {
		data = make(map[string]any)
	}
	data["iceServers"] = h.iceServers
	data["relayMode"] = h.mode
	data["sfuUrl"] = h.sfuURL
	return data
}

func (c *signalClient) send(data map[string]any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.conn.WriteJSON(data); err != nil {
		log.Printf("write websocket: %v", err)
	}
}

func cloneMap(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func mergeMap(base map[string]any, next map[string]any) map[string]any {
	if base == nil {
		return cloneMap(next)
	}
	merged := cloneMap(base)
	for key, value := range next {
		merged[key] = value
	}
	return merged
}

func nowMillis() int64 {
	return time.Now().UnixMilli()
}
