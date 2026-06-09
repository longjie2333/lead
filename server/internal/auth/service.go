package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
	"lead-turn-server/internal/config"
)

const (
	userRoleAdmin = "admin"
	userRoleUser  = "user"
)

type Service struct {
	mu      sync.Mutex
	users   map[string]userAccount
	tokens  map[string]string
	devices map[string]deviceRecord
	dbPath  string
	db      *sql.DB
	cfg     config.AuthConfig
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
	Role         string
	CreatedAt    int64
	UpdatedAt    int64
}

type apiUser struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	IsAdmin   bool   `json:"isAdmin"`
	CreatedAt int64  `json:"createdAt,omitempty"`
	UpdatedAt int64  `json:"updatedAt,omitempty"`
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
	User    apiUser        `json:"user"`
	Device  *deviceRecord  `json:"device,omitempty"`
	Devices []deviceRecord `json:"devices,omitempty"`
}

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
	IsAdmin  *bool  `json:"isAdmin"`
}

type updateUserRequest struct {
	Username *string `json:"username"`
	Password *string `json:"password"`
	Role     *string `json:"role"`
	IsAdmin  *bool   `json:"isAdmin"`
}

func NewService(cfg config.AuthConfig) *Service {
	service := &Service{
		users:   make(map[string]userAccount),
		tokens:  make(map[string]string),
		devices: make(map[string]deviceRecord),
		dbPath:  cfg.DBPath,
		cfg:     cfg,
	}
	if err := service.initStore(); err != nil {
		logAuthError("init auth sqlite", err)
	}
	if err := service.loadStore(); err != nil {
		logAuthError("load auth sqlite", err)
	}
	if err := service.ensureBootstrapAdmin(); err != nil {
		logAuthError("ensure bootstrap admin", err)
	}
	return service
}

func (a *Service) Handler() http.Handler {
	router := gin.New()
	router.Use(gin.Recovery())
	a.RegisterRoutes(router.Group("/api"))
	return router
}

func (a *Service) RegisterRoutes(group *gin.RouterGroup) {
	group.POST("/login", a.handleLogin)
	group.POST("/logout", a.handleLogout)
	group.GET("/devices", a.handleDevices)
	group.GET("/users", a.requireAdminGin(a.handleListUsers))
	group.POST("/users", a.requireAdminGin(a.handleCreateUser))
	group.GET("/users/:id", a.requireAuthGin(a.handleGetUser))
	group.PUT("/users/:id", a.requireAuthGin(a.handleUpdateUser))
	group.PATCH("/users/:id", a.requireAuthGin(a.handleUpdateUser))
	group.DELETE("/users/:id", a.requireAdminGin(a.handleDeleteUser))
}

func (a *Service) Close() error {
	if a.db == nil {
		return nil
	}
	return a.db.Close()
}

func (a *Service) handleLogin(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "invalid json")
		return
	}
	account, ok := a.authenticate(req.Username, req.Password)
	if !ok {
		c.String(http.StatusUnauthorized, "invalid username or password")
		return
	}

	token, err := randomToken()
	if err != nil {
		c.String(http.StatusInternalServerError, "token generation failed")
		return
	}

	resp := loginResponse{
		Token: token,
		User:  userToAPI(account),
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
		c.String(http.StatusInternalServerError, "save auth store failed")
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (a *Service) handleLogout(c *gin.Context) {
	token := BearerToken(c.Request)
	if token != "" {
		a.mu.Lock()
		delete(a.tokens, token)
		saveErr := a.saveStoreLocked()
		a.mu.Unlock()
		if saveErr != nil {
			c.String(http.StatusInternalServerError, "save auth store failed")
			return
		}
	}
	c.Status(http.StatusNoContent)
}

func (a *Service) handleDevices(c *gin.Context) {
	accountID, ok := a.accountForRequest(c.Request)
	if !ok {
		c.String(http.StatusUnauthorized, "unauthorized")
		return
	}
	a.mu.Lock()
	devices := a.devicesForAccountLocked(accountID)
	a.mu.Unlock()
	c.JSON(http.StatusOK, gin.H{"devices": devices})
}

func (a *Service) handleListUsers(c *gin.Context) {
	a.mu.Lock()
	users := make([]apiUser, 0, len(a.users))
	for _, user := range a.users {
		users = append(users, userToAPI(user))
	}
	a.mu.Unlock()
	sort.Slice(users, func(i, j int) bool {
		if users[i].CreatedAt == users[j].CreatedAt {
			return users[i].Username < users[j].Username
		}
		return users[i].CreatedAt < users[j].CreatedAt
	})
	c.JSON(http.StatusOK, gin.H{"users": users})
}

func (a *Service) handleCreateUser(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "invalid json")
		return
	}
	username := strings.TrimSpace(req.Username)
	if username == "" || strings.TrimSpace(req.Password) == "" {
		c.String(http.StatusBadRequest, "username and password are required")
		return
	}
	role, err := roleFromInput(req.Role, req.IsAdmin, userRoleUser)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	passwordHash, err := hashPassword(req.Password)
	if err != nil {
		c.String(http.StatusInternalServerError, "password hash failed")
		return
	}
	now := time.Now().UnixMilli()
	user := userAccount{
		ID:           newUserID(),
		Username:     username,
		PasswordHash: passwordHash,
		Role:         role,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	a.mu.Lock()
	if _, exists := a.users[username]; exists {
		a.mu.Unlock()
		c.String(http.StatusConflict, "username already exists")
		return
	}
	a.users[username] = user
	saveErr := a.saveStoreLocked()
	a.mu.Unlock()
	if saveErr != nil {
		c.String(http.StatusInternalServerError, "save auth store failed")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"user": userToAPI(user)})
}

func (a *Service) handleGetUser(c *gin.Context) {
	requester := currentUser(c)
	id := strings.TrimSpace(c.Param("id"))

	a.mu.Lock()
	user, ok := a.userByIDLocked(id)
	a.mu.Unlock()
	if !ok {
		c.String(http.StatusNotFound, "user not found")
		return
	}
	if !requester.isAdmin() && requester.ID != user.ID {
		c.String(http.StatusForbidden, "admin required")
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": userToAPI(user)})
}

func (a *Service) handleUpdateUser(c *gin.Context) {
	requester := currentUser(c)
	id := strings.TrimSpace(c.Param("id"))
	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "invalid json")
		return
	}

	a.mu.Lock()
	user, ok := a.userByIDLocked(id)
	if !ok {
		a.mu.Unlock()
		c.String(http.StatusNotFound, "user not found")
		return
	}

	roleChangeRequested := req.Role != nil || req.IsAdmin != nil
	usernameChangeRequested := req.Username != nil && strings.TrimSpace(*req.Username) != user.Username
	if !requester.isAdmin() {
		if requester.ID != user.ID {
			a.mu.Unlock()
			c.String(http.StatusForbidden, "admin required")
			return
		}
		if roleChangeRequested || usernameChangeRequested {
			a.mu.Unlock()
			c.String(http.StatusForbidden, "admin required")
			return
		}
	}

	if req.Username != nil {
		nextUsername := strings.TrimSpace(*req.Username)
		if nextUsername == "" {
			a.mu.Unlock()
			c.String(http.StatusBadRequest, "username is required")
			return
		}
		if nextUsername != user.Username {
			if _, exists := a.users[nextUsername]; exists {
				a.mu.Unlock()
				c.String(http.StatusConflict, "username already exists")
				return
			}
			delete(a.users, user.Username)
			user.Username = nextUsername
		}
	}
	if req.Password != nil {
		if strings.TrimSpace(*req.Password) == "" {
			a.mu.Unlock()
			c.String(http.StatusBadRequest, "password is required")
			return
		}
		passwordHash, err := hashPassword(*req.Password)
		if err != nil {
			a.mu.Unlock()
			c.String(http.StatusInternalServerError, "password hash failed")
			return
		}
		user.PasswordHash = passwordHash
	}
	if roleChangeRequested {
		role, err := roleFromInput(stringValue(req.Role), req.IsAdmin, user.Role)
		if err != nil {
			a.mu.Unlock()
			c.String(http.StatusBadRequest, err.Error())
			return
		}
		if user.Role == userRoleAdmin && role != userRoleAdmin && a.adminCountLocked() == 1 {
			a.mu.Unlock()
			c.String(http.StatusBadRequest, "cannot remove the last admin")
			return
		}
		user.Role = role
	}
	user.UpdatedAt = time.Now().UnixMilli()
	a.users[user.Username] = user
	saveErr := a.saveStoreLocked()
	a.mu.Unlock()
	if saveErr != nil {
		c.String(http.StatusInternalServerError, "save auth store failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": userToAPI(user)})
}

func (a *Service) handleDeleteUser(c *gin.Context) {
	requester := currentUser(c)
	id := strings.TrimSpace(c.Param("id"))

	a.mu.Lock()
	user, ok := a.userByIDLocked(id)
	if !ok {
		a.mu.Unlock()
		c.String(http.StatusNotFound, "user not found")
		return
	}
	if requester.ID == user.ID {
		a.mu.Unlock()
		c.String(http.StatusBadRequest, "cannot delete current user")
		return
	}
	if user.Role == userRoleAdmin && a.adminCountLocked() == 1 {
		a.mu.Unlock()
		c.String(http.StatusBadRequest, "cannot delete the last admin")
		return
	}
	delete(a.users, user.Username)
	for token, accountID := range a.tokens {
		if accountID == user.ID {
			delete(a.tokens, token)
		}
	}
	for id, device := range a.devices {
		if device.AccountID == user.ID {
			delete(a.devices, id)
		}
	}
	saveErr := a.saveStoreLocked()
	a.mu.Unlock()
	if saveErr != nil {
		c.String(http.StatusInternalServerError, "save auth store failed")
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *Service) requireAuthGin(next func(*gin.Context)) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := BearerToken(c.Request)
		a.mu.Lock()
		accountID, ok := a.tokens[token]
		var account userAccount
		if ok {
			account, ok = a.userByIDLocked(accountID)
		}
		a.mu.Unlock()
		if !ok {
			c.String(http.StatusUnauthorized, "unauthorized")
			c.Abort()
			return
		}
		c.Set("account", account)
		next(c)
	}
}

func (a *Service) requireAdminGin(next func(*gin.Context)) gin.HandlerFunc {
	return a.requireAuthGin(func(c *gin.Context) {
		account := currentUser(c)
		if !account.isAdmin() {
			c.String(http.StatusForbidden, "admin required")
			c.Abort()
			return
		}
		next(c)
	})
}

func (a *Service) authenticate(username, password string) (userAccount, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	account, ok := a.users[strings.TrimSpace(username)]
	if !ok || !checkPassword(account.PasswordHash, password) {
		return userAccount{}, false
	}
	return account, true
}

func (a *Service) AccountForToken(token string) (string, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	accountID, ok := a.tokens[token]
	if !ok {
		return "", false
	}
	if _, ok := a.userByIDLocked(accountID); !ok {
		return "", false
	}
	return accountID, true
}

func (a *Service) accountForRequest(r *http.Request) (string, bool) {
	return a.AccountForToken(BearerToken(r))
}

func (a *Service) SetDeviceOnline(accountID, deviceID string, online bool) {
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

func (a *Service) DeviceBelongsTo(accountID, deviceID string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	record, ok := a.devices[deviceID]
	return ok && record.AccountID == accountID
}

func (a *Service) devicesForAccountLocked(accountID string) []deviceRecord {
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

func (a *Service) loadStore() error {
	if a.db == nil {
		return nil
	}
	userRows, err := a.db.Query("SELECT id, username, password_hash, role, created_at, updated_at FROM users")
	if err != nil {
		return err
	}
	defer userRows.Close()

	users := make(map[string]userAccount)
	for userRows.Next() {
		var user userAccount
		if err := userRows.Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.CreatedAt, &user.UpdatedAt); err != nil {
			return err
		}
		user.Role = normalizeRole(user.Role)
		users[user.Username] = user
	}
	if err := userRows.Err(); err != nil {
		return err
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
	a.users = users
	for token, accountID := range tokens {
		if _, ok := a.userByIDLocked(accountID); ok {
			a.tokens[token] = accountID
		}
	}
	for _, device := range devices {
		id := strings.TrimSpace(device.ID)
		if _, ok := a.userByIDLocked(device.AccountID); !ok {
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

func (a *Service) initStore() error {
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
CREATE TABLE IF NOT EXISTS users (
	id TEXT PRIMARY KEY,
	username TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	role TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);
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
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_devices_account ON devices(account_id, updated_at DESC);
`); err != nil {
		_ = db.Close()
		return err
	}
	a.db = db
	return nil
}

func (a *Service) ensureBootstrapAdmin() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.users) > 0 {
		return nil
	}
	now := time.Now().UnixMilli()
	passwordHash, err := hashPassword(a.cfg.AdminPassword)
	if err != nil {
		return err
	}
	admin := userAccount{
		ID:           "admin",
		Username:     strings.TrimSpace(a.cfg.AdminUsername),
		PasswordHash: passwordHash,
		Role:         userRoleAdmin,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	a.users[admin.Username] = admin
	return a.saveStoreLocked()
}

func (a *Service) saveStoreLocked() error {
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
	if _, err := tx.Exec("DELETE FROM users"); err != nil {
		return err
	}

	userStmt, err := tx.Prepare("INSERT INTO users(id, username, password_hash, role, created_at, updated_at) VALUES(?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer userStmt.Close()

	usernames := make([]string, 0, len(a.users))
	for username := range a.users {
		usernames = append(usernames, username)
	}
	sort.Strings(usernames)
	for _, username := range usernames {
		user := a.users[username]
		if _, err := userStmt.Exec(user.ID, user.Username, user.PasswordHash, user.Role, user.CreatedAt, user.UpdatedAt); err != nil {
			return err
		}
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

func (a *Service) userByIDLocked(id string) (userAccount, bool) {
	for _, user := range a.users {
		if user.ID == id {
			return user, true
		}
	}
	return userAccount{}, false
}

func (a *Service) adminCountLocked() int {
	count := 0
	for _, user := range a.users {
		if user.isAdmin() {
			count++
		}
	}
	return count
}

func (u userAccount) isAdmin() bool {
	return u.Role == userRoleAdmin
}

func userToAPI(user userAccount) apiUser {
	role := normalizeRole(user.Role)
	return apiUser{
		ID:        user.ID,
		Username:  user.Username,
		Role:      role,
		IsAdmin:   role == userRoleAdmin,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

func currentUser(c *gin.Context) userAccount {
	value, _ := c.Get("account")
	account, _ := value.(userAccount)
	return account
}

func roleFromInput(role string, isAdmin *bool, fallback string) (string, error) {
	if isAdmin != nil {
		if *isAdmin {
			return userRoleAdmin, nil
		}
		return userRoleUser, nil
	}
	if strings.TrimSpace(role) == "" {
		return normalizeRole(fallback), nil
	}
	role = normalizeRole(role)
	if role != userRoleAdmin && role != userRoleUser {
		return "", errors.New("role must be admin or user")
	}
	return role, nil
}

func normalizeRole(role string) string {
	if strings.EqualFold(strings.TrimSpace(role), userRoleAdmin) {
		return userRoleAdmin
	}
	return userRoleUser
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func checkPassword(hash, password string) bool {
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil {
		return true
	}
	return hash == password
}

func newUserID() string {
	token, err := randomToken()
	if err != nil {
		return time.Now().Format("20060102150405.000000000")
	}
	return "user-" + token[:16]
}

func BearerToken(r *http.Request) string {
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

func logAuthError(prefix string, err error) {
	if err != nil {
		// Keep auth startup non-fatal so tests and local runs can still use the in-memory fallback.
		log.Printf("%s: %v", prefix, err)
	}
}
