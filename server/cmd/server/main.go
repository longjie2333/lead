package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"lead-turn-server/internal/app"
	"lead-turn-server/internal/config"
	"lead-turn-server/internal/turnserver"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	server, listenAddress, publicIP, err := turnserver.Start(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			log.Printf("close TURN server: %v", err)
		}
	}()

	log.Printf("TURN UDP listening on %s", listenAddress)
	log.Printf("TURN relay address %s", publicIP.String())

	httpServer, err := app.StartHTTPServer(cfg, publicIP.String())
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown HTTP server: %v", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	log.Print("server stopped")
}
