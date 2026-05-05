package main

import (
	"log"
	"net/http"

	"tender/server/internal/app"
	"tender/server/internal/config"
)

func main() {
	cfg := config.Load()

	srv, err := app.NewServer(cfg)
	if err != nil {
		log.Fatalf("failed to initialize server: %v", err)
	}

	log.Printf("server listening on %s", cfg.Addr)
	if err := http.ListenAndServe(cfg.Addr, srv.Handler()); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
