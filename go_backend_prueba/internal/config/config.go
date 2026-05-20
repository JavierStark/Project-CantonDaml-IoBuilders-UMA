package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	LedgerAPIHost string
	LedgerAPIPort int
	LedgerAPIURL  string
	HTTPHost      string
	HTTPPort      int

	UserID string
	Party  string

	TemplateBond string
	ChoiceSettle string

	RequestTimeout time.Duration
}

func Load() (Config, error) {
	host := getenv("CANTON_LEDGER_API_HOST", "127.0.0.1")
	portStr := getenv("CANTON_LEDGER_API_PORT", "5013")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return Config{}, fmt.Errorf("invalid CANTON_LEDGER_API_PORT: %w", err)
	}

	httpHost := getenv("HTTP_HOST", "127.0.0.1")
	httpPortStr := getenv("HTTP_PORT", "8080")
	httpPort, err := strconv.Atoi(httpPortStr)
	if err != nil {
		return Config{}, fmt.Errorf("invalid HTTP_PORT: %w", err)
	}

	timeoutStr := getenv("REQUEST_TIMEOUT", "10s")
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return Config{}, fmt.Errorf("invalid REQUEST_TIMEOUT: %w", err)
	}

	return Config{
		LedgerAPIHost:  host,
		LedgerAPIPort:  port,
		LedgerAPIURL:   fmt.Sprintf("http://%s:%d", host, port),
		HTTPHost:       httpHost,
		HTTPPort:       httpPort,
		UserID:         getenv("CANTON_USER_ID", "ledger-api-user"),
		Party:          getenv("CANTON_PARTY", "Issuer"),
		TemplateBond:   getenv("TEMPLATE_BOND", "#Loan.Main:DebtInstrument"),
		ChoiceSettle:   getenv("CHOICE_SETTLE", "AtomicSettlement"),
		RequestTimeout: timeout,
	}, nil
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
