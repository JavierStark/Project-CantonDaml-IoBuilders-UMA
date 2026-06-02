package main

import (
	"fmt"
	"log"

	"canton-bond-platform/backend/internal/api"
	"canton-bond-platform/backend/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	s := api.NewServer(cfg)
	e := s.Router()

	addr := fmt.Sprintf("%s:%d", cfg.HTTPHost, cfg.HTTPPort)
	log.Printf("bond API listening on %s", addr)

	for _, p := range cfg.Participants {
		log.Printf("  participant %s -> %s [%s]", p.Name, p.URL, p.Parties)
	}

	e.Logger.Fatal(e.Start(addr))
}
