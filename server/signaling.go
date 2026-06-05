package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type iceServer struct {
	URLs       any    `json:"urls"`
	Username   string `json:"username,omitempty"`
	Credential string `json:"credential,omitempty"`
}

type signalingHub struct {
	upgrader websocket.Upgrader
	nextID   atomic.Uint64

	mu                    sync.Mutex
	iceServers            []iceServer
	mode                  string
	sfuURL                string
	androidClients        map[*signalClient]struct{}
	waitingAndroidClients map[*signalClient]struct{}
	viewers               map[string]*signalClient
	activeAndroid         *signalClient
	streamInfo            map[string]any
}

type signalClient struct {
	id   string
	role string
	conn *websocket.Conn
	mu   sync.Mutex
}

func newSignalingHub(iceServers []iceServer, mode string, sfuURL string) *signalingHub {
	return &signalingHub{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		iceServers:            iceServers,
		mode:                  mode,
		sfuURL:                sfuURL,
		androidClients:        make(map[*signalClient]struct{}),
		waitingAndroidClients: make(map[*signalClient]struct{}),
		viewers:               make(map[string]*signalClient),
	}
}

func (h *signalingHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade websocket: %v", err)
		return
	}

	client := &signalClient{
		id:   fmt.Sprintf("viewer-%d", h.nextID.Add(1)),
		role: "unknown",
		conn: conn,
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

func (h *signalingHub) handleControl(client *signalClient, payload []byte) {
	var data map[string]any
	if err := json.Unmarshal(payload, &data); err != nil {
		return
	}

	msgType, _ := data["type"].(string)
	role, _ := data["role"].(string)

	h.mu.Lock()
	defer h.mu.Unlock()

	if msgType == "hello" && role == "android" {
		client.role = "android"
		h.androidClients[client] = struct{}{}
		delete(h.waitingAndroidClients, client)
		h.activeAndroid = client
		h.streamInfo = cloneMap(data)
		h.streamInfo["type"] = "stream-info"
		client.send(h.withServerConfig(map[string]any{"type": "config"}))
		h.broadcastToViewers(h.withServerConfig(cloneMap(h.streamInfo)))
		for _, viewer := range h.viewers {
			h.sendToAndroidLocked(map[string]any{"type": "viewer-joined", "viewerId": viewer.id})
		}
		return
	}

	if msgType == "hello" && role == "android-waiting" {
		client.role = "android-waiting"
		h.waitingAndroidClients[client] = struct{}{}
		client.send(h.withServerConfig(map[string]any{"type": "config"}))
		if h.activeAndroid == nil && len(h.viewers) > 0 {
			client.send(map[string]any{"type": "remote-request"})
		}
		return
	}

	if msgType == "hello" && role == "viewer" {
		client.role = "viewer"
		h.viewers[client.id] = client
		if h.activeAndroid != nil {
			client.send(h.withServerConfig(cloneMap(h.streamInfo)))
			h.sendToAndroidLocked(map[string]any{"type": "viewer-joined", "viewerId": client.id})
		} else {
			client.send(h.withServerConfig(map[string]any{"type": "waiting"}))
			h.notifyWaitingAndroidLocked()
		}
		return
	}

	switch client.role {
	case "android":
		if msgType == "stream-info" {
			h.streamInfo = mergeMap(h.streamInfo, data)
			h.broadcastToViewers(h.withServerConfig(cloneMap(data)))
			return
		}
		viewerID, _ := data["viewerId"].(string)
		if viewerID == "" {
			return
		}
		if viewer := h.viewers[viewerID]; viewer != nil {
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
		h.sendToAndroidLocked(next)
	}
}

func (h *signalingHub) removeClient(client *signalClient) {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch client.role {
	case "android":
		delete(h.androidClients, client)
		if h.activeAndroid == client {
			h.activeAndroid = nil
			for candidate := range h.androidClients {
				h.activeAndroid = candidate
				break
			}
			h.streamInfo = nil
			h.broadcastToViewers(map[string]any{"type": "android-disconnected"})
		}
	case "android-waiting":
		delete(h.waitingAndroidClients, client)
	case "viewer":
		delete(h.viewers, client.id)
		h.sendToAndroidLocked(map[string]any{"type": "viewer-left", "viewerId": client.id})
	}
}

func (h *signalingHub) sendToAndroidLocked(data map[string]any) {
	if h.activeAndroid != nil {
		h.activeAndroid.send(data)
	}
}

func (h *signalingHub) notifyWaitingAndroidLocked() {
	for client := range h.waitingAndroidClients {
		client.send(map[string]any{"type": "remote-request"})
	}
}

func (h *signalingHub) broadcastToViewers(data map[string]any) {
	for _, viewer := range h.viewers {
		viewer.send(data)
	}
}

func (h *signalingHub) withServerConfig(data map[string]any) map[string]any {
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

func parseICEServersJSON(value string) ([]iceServer, bool) {
	if value == "" {
		return nil, false
	}
	var servers []iceServer
	if err := json.Unmarshal([]byte(value), &servers); err != nil {
		log.Printf("Invalid ICE_SERVERS_JSON: %v", err)
		return nil, false
	}
	return servers, true
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
