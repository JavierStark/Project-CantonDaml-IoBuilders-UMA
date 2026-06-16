package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Participant struct {
	Name    string
	URL     string
	GRPCURL string
	Parties []string
}

type Config struct {
	HTTPHost string
	HTTPPort int

	Participants []Participant

	UserID          string
	RequestTimeout  time.Duration
	LedgerTransport string
}

func Load() (Config, error) {
	httpHost := getEnv("HTTP_HOST", "0.0.0.0")
	httpPortStr := getEnv("HTTP_PORT", "8080")
	httpPort, err := strconv.Atoi(httpPortStr)
	if err != nil {
		return Config{}, fmt.Errorf("invalid HTTP_PORT: %w", err)
	}

	timeoutStr := getEnv("REQUEST_TIMEOUT", "30s")
	requestTimeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return Config{}, fmt.Errorf("invalid REQUEST_TIMEOUT: %w", err)
	}

	userID := getEnv("CANTON_USER_ID", "ledger-api-user")
	ledgerTransport := strings.ToLower(getEnv("LEDGER_TRANSPORT", "grpc"))
	if ledgerTransport != "grpc" && ledgerTransport != "http" {
		return Config{}, fmt.Errorf("invalid LEDGER_TRANSPORT %q: expected grpc or http", ledgerTransport)
	}

	participants := []Participant{
		{
			Name:    "participant1",
			URL:     getEnv("PARTICIPANT1_URL", "http://participant1:5013"),
			GRPCURL: getEnv("PARTICIPANT1_GRPC_URL", "participant1:5011"),
			Parties: splitParties(getEnv("PARTICIPANT1_PARTIES", "admin,alice,executor")),
		},
		{
			Name:    "participant2",
			URL:     getEnv("PARTICIPANT2_URL", "http://participant2:5023"),
			GRPCURL: getEnv("PARTICIPANT2_GRPC_URL", "participant2:5021"),
			Parties: splitParties(getEnv("PARTICIPANT2_PARTIES", "bob")),
		},
		{
			Name:    "participant3",
			URL:     getEnv("PARTICIPANT3_URL", "http://participant3:5033"),
			GRPCURL: getEnv("PARTICIPANT3_GRPC_URL", "participant3:5031"),
			Parties: splitParties(getEnv("PARTICIPANT3_PARTIES", "charlie")),
		},
	}

	return Config{
		HTTPHost:        httpHost,
		HTTPPort:        httpPort,
		Participants:    participants,
		UserID:          userID,
		RequestTimeout:  requestTimeout,
		LedgerTransport: ledgerTransport,
	}, nil
}

func (c *Config) PartyToParticipant(party string) *Participant {
	for _, p := range c.Participants {
		for _, pp := range p.Parties {
			if pp == party {
				return &p
			}
		}
	}
	return nil
}

func splitParties(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
