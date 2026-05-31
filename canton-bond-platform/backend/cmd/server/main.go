package main

import (
	"fmt"
	"log"
	"net/http"

	"canton-bond-platform/backend/internal/api"
	"canton-bond-platform/backend/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	server := api.NewServer(cfg)

	addr := fmt.Sprintf("%s:%d", cfg.HTTPHost, cfg.HTTPPort)
	log.Printf("bond API listening on %s", addr)

	for _, p := range cfg.Participants {
		log.Printf("  participant %s -> %s [%s]", p.Name, p.URL, p.Parties)
	}

	if err := http.ListenAndServe(addr, server.Routes()); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
