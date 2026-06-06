package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type authService struct {
	mu       sync.Mutex
	users    map[string]userAccount
	tokens   map[string]string
	devices  map[string]deviceRecord
	dbPath   string
	db       *sql.DB
	sessions *signalingHub
}

type storedDeviceRecord struct {
	ID         string `json:"id"`
	AccountID  string `json:"accountId"`
	Name       string `json:"name"`
	LastSource string `json:"lastSource"`
	UpdatedAt  int64  `json:"updatedAt"`
}

type userAccount struct {
	ID           string
	Username     string
	PasswordHash string
}

type deviceRecord struct {
	ID         string `json:"id"`
	AccountID  string `json:"-"`
	Name       string `json:"name"`
	LastSource string `json:"lastSource"`
	Online     bool   `json:"online"`
	UpdatedAt  int64  `json:"updatedAt"`
}

type loginRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	Source     string `json:"source"`
	DeviceID   string `json:"deviceId"`
	DeviceName string `json:"deviceName"`
}

type loginResponse struct {
	Token   string         `json:"token"`
	User    loginUser      `json:"user"`
	Device  *deviceRecord  `json:"device,omitempty"`
	Devices []deviceRecord `json:"devices,omitempty"`
}

type loginUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

func newAuthService() *authService {
	password := envString("ADMIN_PASSWORD", "admin")
	service := &authService{
		users: map[string]userAccount{
			"admin": {
				ID:           "admin",
				Username:     "admin",
				PasswordHash: password,
			},
		},
		tokens:  make(map[string]string),
		devices: make(map[string]deviceRecord),
		dbPath:  envString("AUTH_DB_PATH", filepath.Join("data", "auth.db")),
	}
	if err := service.initStore(); err != nil {
		log.Printf("init auth sqlite: %v", err)
	}
	if err := service.loadStore(); err != nil {
		log.Printf("load auth sqlite: %v", err)
	}
	return service
}

func (a *authService) routes(hub *signalingHub) http.Handler {
	a.sessions = hub
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", a.handleLogin)
	mux.HandleFunc("/api/logout", a.handleLogout)
	mux.HandleFunc("/api/devices", a.handleDevices)
	return mux
}

func (a *authService) close() error {
	if a.db == nil {
		return nil
	}
	return a.db.Close()
}

func (a *authService) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	account, ok := a.authenticate(req.Username, req.Password)
	if !ok {
		http.Error(w, "invalid username or password", http.StatusUnauthorized)
		return
	}

	token, err := randomToken()
	if err != nil {
		http.Error(w, "token generation failed", http.StatusInternalServerError)
		return
	}

	resp := loginResponse{
		Token: token,
		User:  loginUser{ID: account.ID, Username: account.Username},
	}
	now := time.Now().UnixMilli()

	a.mu.Lock()
	a.tokens[token] = account.ID
	if req.Source == "android" {
		deviceID := strings.TrimSpace(req.DeviceID)
		if deviceID == "" {
			deviceID = "android-" + token[:12]
		}
		name := strings.TrimSpace(req.DeviceName)
		if name == "" {
			name = deviceID
		}
		record := deviceRecord{
			ID:         deviceID,
			AccountID:  account.ID,
			Name:       name,
			LastSource: req.Source,
			UpdatedAt:  now,
		}
		if existing, ok := a.devices[deviceID]; ok {
			record.Online = existing.Online
		}
		a.devices[deviceID] = record
		resp.Device = &record
	}
	resp.Devices = a.devicesForAccountLocked(account.ID)
	saveErr := a.saveStoreLocked()
	a.mu.Unlock()
	if saveErr != nil {
		http.Error(w, "save auth store failed", http.StatusInternalServerError)
		return
	}

	writeAPIJSON(w, http.StatusOK, resp)
}

func (a *authService) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	token := bearerToken(r)
	if token != "" {
		a.mu.Lock()
		delete(a.tokens, token)
		saveErr := a.saveStoreLocked()
		a.mu.Unlock()
		if saveErr != nil {
			http.Error(w, "save auth store failed", http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *authService) handleDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	accountID, ok := a.accountForRequest(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	a.mu.Lock()
	devices := a.devicesForAccountLocked(accountID)
	a.mu.Unlock()
	writeAPIJSON(w, http.StatusOK, map[string]any{"devices": devices})
}

func (a *authService) authenticate(username, password string) (userAccount, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	account, ok := a.users[strings.TrimSpace(username)]
	if !ok || account.PasswordHash != password {
		return userAccount{}, false
	}
	return account, true
}

func (a *authService) accountForToken(token string) (string, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	accountID, ok := a.tokens[token]
	return accountID, ok
}

func (a *authService) accountForRequest(r *http.Request) (string, bool) {
	return a.accountForToken(bearerToken(r))
}

func (a *authService) setDeviceOnline(accountID, deviceID string, online bool) {
	if deviceID == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	record, ok := a.devices[deviceID]
	if !ok || record.AccountID != accountID {
		return
	}
	record.Online = online
	record.UpdatedAt = time.Now().UnixMilli()
	a.devices[deviceID] = record
}

func (a *authService) deviceBelongsTo(accountID, deviceID string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	record, ok := a.devices[deviceID]
	return ok && record.AccountID == accountID
}

func (a *authService) devicesForAccountLocked(accountID string) []deviceRecord {
	devices := make([]deviceRecord, 0)
	for _, device := range a.devices {
		if device.AccountID == accountID {
			devices = append(devices, device)
		}
	}
	sort.Slice(devices, func(i, j int) bool {
		return devices[i].UpdatedAt > devices[j].UpdatedAt
	})
	return devices
}

func (a *authService) loadStore() error {
	if a.db == nil {
		return nil
	}
	tokenRows, err := a.db.Query("SELECT token, account_id FROM tokens")
	if err != nil {
		return err
	}
	defer tokenRows.Close()

	tokens := make(map[string]string)
	for tokenRows.Next() {
		var token string
		var accountID string
		if err := tokenRows.Scan(&token, &accountID); err != nil {
			return err
		}
		tokens[strings.TrimSpace(token)] = strings.TrimSpace(accountID)
	}
	if err := tokenRows.Err(); err != nil {
		return err
	}

	deviceRows, err := a.db.Query("SELECT id, account_id, name, last_source, updated_at FROM devices")
	if err != nil {
		return err
	}
	defer deviceRows.Close()

	devices := make([]storedDeviceRecord, 0)
	for deviceRows.Next() {
		var device storedDeviceRecord
		if err := deviceRows.Scan(&device.ID, &device.AccountID, &device.Name, &device.LastSource, &device.UpdatedAt); err != nil {
			return err
		}
		devices = append(devices, device)
	}
	if err := deviceRows.Err(); err != nil {
		return err
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	for token, accountID := range tokens {
		if _, ok := a.usersByIDLocked(accountID); ok {
			a.tokens[token] = accountID
		}
	}
	for _, device := range devices {
		id := strings.TrimSpace(device.ID)
		if _, ok := a.usersByIDLocked(device.AccountID); !ok {
			continue
		}
		a.devices[id] = deviceRecord{
			ID:         id,
			AccountID:  device.AccountID,
			Name:       device.Name,
			LastSource: device.LastSource,
			Online:     false,
			UpdatedAt:  device.UpdatedAt,
		}
	}
	return nil
}

func (a *authService) initStore() error {
	if strings.TrimSpace(a.dbPath) == "" {
		return nil
	}
	dir := filepath.Dir(a.dbPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	db, err := sql.Open("sqlite3", a.dbPath)
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`
PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;
CREATE TABLE IF NOT EXISTS tokens (
	token TEXT PRIMARY KEY,
	account_id TEXT NOT NULL,
	created_at INTEGER NOT NULL DEFAULT (unixepoch() * 1000)
);
CREATE TABLE IF NOT EXISTS devices (
	id TEXT PRIMARY KEY,
	account_id TEXT NOT NULL,
	name TEXT NOT NULL,
	last_source TEXT NOT NULL,
	updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_devices_account ON devices(account_id, updated_at DESC);
`); err != nil {
		_ = db.Close()
		return err
	}
	a.db = db
	return nil
}

func (a *authService) saveStoreLocked() error {
	if a.db == nil {
		return nil
	}
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM tokens"); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM devices"); err != nil {
		return err
	}

	tokenStmt, err := tx.Prepare("INSERT INTO tokens(token, account_id) VALUES(?, ?)")
	if err != nil {
		return err
	}
	defer tokenStmt.Close()

	tokenKeys := make([]string, 0, len(a.tokens))
	for token := range a.tokens {
		tokenKeys = append(tokenKeys, token)
	}
	sort.Strings(tokenKeys)
	for _, token := range tokenKeys {
		if _, err := tokenStmt.Exec(token, a.tokens[token]); err != nil {
			return err
		}
	}

	deviceStmt, err := tx.Prepare("INSERT INTO devices(id, account_id, name, last_source, updated_at) VALUES(?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer deviceStmt.Close()

	deviceIDs := make([]string, 0, len(a.devices))
	for id := range a.devices {
		deviceIDs = append(deviceIDs, id)
	}
	sort.Strings(deviceIDs)
	for _, id := range deviceIDs {
		device := a.devices[id]
		if _, err := deviceStmt.Exec(id, device.AccountID, device.Name, device.LastSource, device.UpdatedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (a *authService) usersByIDLocked(id string) (userAccount, bool) {
	for _, user := range a.users {
		if user.ID == id {
			return user, true
		}
	}
	return userAccount{}, false
}

func cloneStringMap(input map[string]string) map[string]string {
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func bearerToken(r *http.Request) string {
	if token := strings.TrimSpace(r.URL.Query().Get("token")); token != "" {
		return token
	}
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return ""
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := hex.EncodeToString(buf)
	if token == "" {
		return "", errors.New("empty token")
	}
	return token, nil
}

func writeAPIJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
