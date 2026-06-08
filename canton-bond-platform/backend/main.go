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

	s, err := NewServer(cfg)
	if err != nil {
		log.Fatalf("server init error: %v", err)
	}
	e := s.Router()

	addr := fmt.Sprintf("%s:%d", cfg.HTTPHost, cfg.HTTPPort)
	log.Printf("bond API listening on %s", addr)

	log.Printf("ledger transport: %s", cfg.LedgerTransport)
	for _, p := range cfg.Participants {
		if cfg.LedgerTransport == "grpc" {
			log.Printf("  participant %s -> %s (grpc), fallback http %s [%s]", p.Name, p.GRPCURL, p.URL, p.Parties)
		} else {
			log.Printf("  participant %s -> %s (http) [%s]", p.Name, p.URL, p.Parties)
		}
	}

	e.Logger.Fatal(e.Start(addr))
}
