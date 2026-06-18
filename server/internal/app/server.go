package app

import (
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"lead-turn-server/internal/auth"
	"lead-turn-server/internal/config"
	"lead-turn-server/internal/signaling"
)

func StartHTTPServer(cfg config.Config, publicIP string) (*http.Server, error) {
	if err := config.ValidateHTTP(cfg.HTTP); err != nil {
		return nil, err
	}

	iceServers := config.BuildICEServers(cfg, publicIP)
	authService := auth.NewService(cfg.Auth)
	hub := signaling.NewHub(iceServers, config.RelayMode(cfg), cfg.TURN.SFUURL, authService)
	router := gin.New()
	router.Use(gin.Recovery())
	authService.RegisterRoutes(router.Group("/api"))
	router.GET("/ws", gin.WrapH(hub))
	publicDir := StaticPublicDir()
	router.GET("/", func(c *gin.Context) {
		indexPath := filepath.Join(publicDir, "index.html")
		setStaticCacheHeaders(c, indexPath)
		c.File(indexPath)
	})
	router.StaticFS("/assets", http.Dir(filepath.Join(publicDir, "assets")))
	router.NoRoute(func(c *gin.Context) {
		if c.Request.Method != http.MethodGet {
			c.Status(http.StatusNotFound)
			return
		}
		if strings.HasPrefix(c.Request.URL.Path, "/api/") || c.Request.URL.Path == "/api" {
			c.Status(http.StatusNotFound)
			return
		}
		if servePublicFile(c, publicDir) {
			return
		}
		indexPath := filepath.Join(publicDir, "index.html")
		setStaticCacheHeaders(c, indexPath)
		c.File(indexPath)
	})

	addr := net.JoinHostPort(cfg.HTTP.Addr, strconv.Itoa(cfg.HTTP.Port))
	server := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}
	server.RegisterOnShutdown(func() {
		if err := authService.Close(); err != nil {
			log.Printf("close auth database: %v", err)
		}
	})
	go func() {
		log.Printf("HTTP signaling/static server listening on http://%s", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP server stopped unexpectedly: %v", err)
		}
	}()
	return server, nil
}

func servePublicFile(c *gin.Context, publicDir string) bool {
	cleanPath := path.Clean(c.Request.URL.Path)
	if cleanPath == "/" || cleanPath == "." {
		return false
	}

	relPath := filepath.FromSlash(strings.TrimPrefix(cleanPath, "/"))
	filePath := filepath.Join(publicDir, relPath)
	info, err := os.Stat(filePath)
	if err != nil || info.IsDir() {
		return false
	}

	setStaticCacheHeaders(c, filePath)
	if filepath.Ext(filePath) == ".webmanifest" {
		c.Header("Content-Type", "application/manifest+json; charset=utf-8")
	}
	c.File(filePath)
	return true
}

func setStaticCacheHeaders(c *gin.Context, filePath string) {
	switch filepath.Base(filePath) {
	case "index.html", "manifest.webmanifest", "registerSW.js", "sw.js", "lead-sw.js", "logo.png", "favicon.ico":
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
	}
}

func StaticPublicDir() string {
	if _, err := os.Stat(filepath.Join("public", "index.html")); err == nil {
		return "public"
	}
	if _, err := os.Stat(filepath.Join("server", "public", "index.html")); err == nil {
		return filepath.Join("server", "public")
	}
	_, file, _, ok := runtime.Caller(0)
	if ok {
		dir := filepath.Dir(file)
		if _, err := os.Stat(filepath.Join(dir, "..", "..", "public", "index.html")); err == nil {
			return filepath.Join(dir, "..", "..", "public")
		}
		return filepath.Join(dir, "public")
	}
	return "public"
}
