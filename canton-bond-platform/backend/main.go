package main

import (
	"fmt"
	"log"
)

func main() {
	cfg, err := Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	s := NewServer(cfg)
	e := s.Router()

	addr := fmt.Sprintf("%s:%d", cfg.HTTPHost, cfg.HTTPPort)
	log.Printf("bond API listening on %s", addr)

	for _, p := range cfg.Participants {
		log.Printf("  participant %s -> %s [%s]", p.Name, p.URL, p.Parties)
	}

	e.Logger.Fatal(e.Start(addr))
}
