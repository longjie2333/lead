package app

import (
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
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
	router.GET("/viewer", func(c *gin.Context) {
		c.File(filepath.Join(publicDir, "index.html"))
	})
	router.StaticFS("/viewer", http.Dir(publicDir))
	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Hello")
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
