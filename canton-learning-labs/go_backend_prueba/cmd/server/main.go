package main

import (
	"fmt"
	"log"
	"net/http"

	"go_backend_prueba/internal/config"
	"go_backend_prueba/internal/httpapi"
	"go_backend_prueba/internal/ledger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	ledgerClient := ledger.New(cfg.LedgerAPIURL, cfg.UserID, cfg.Party, cfg.RequestTimeout)
	server := httpapi.NewServer(cfg, ledgerClient)

	addr := fmt.Sprintf("%s:%d", cfg.HTTPHost, cfg.HTTPPort)
	log.Printf("backend listening on %s", addr)
	log.Printf("ledger api %s", cfg.LedgerAPIURL)

	if err := http.ListenAndServe(addr, server.Routes()); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
