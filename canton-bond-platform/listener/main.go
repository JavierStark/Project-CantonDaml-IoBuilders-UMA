package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"canton-bond-platform/pkg/cantonledger"
)

type Config struct {
	ParticipantURL string
	UserID         string
	PollInterval   time.Duration
	RequestTimeout time.Duration
	EmitInitial    bool
	Templates      []string
}

func LoadConfig() (Config, error) {
	pollInterval, err := time.ParseDuration(getEnv("LISTENER_POLL_INTERVAL", "2s"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid LISTENER_POLL_INTERVAL: %w", err)
	}
	requestTimeout, err := time.ParseDuration(getEnv("LISTENER_REQUEST_TIMEOUT", "30s"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid LISTENER_REQUEST_TIMEOUT: %w", err)
	}
	emitInitial := strings.EqualFold(getEnv("LISTENER_EMIT_INITIAL", "false"), "true")

	templates := splitList(getEnv("LISTENER_TEMPLATES", strings.Join(cantonledger.BondTemplates(), ",")))

	return Config{
		ParticipantURL: getEnv("LISTENER_PARTICIPANT_URL", "http://participant1:5013"),
		UserID:         getEnv("LISTENER_USER_ID", "ledger-api-user"),
		PollInterval:   pollInterval,
		RequestTimeout: requestTimeout,
		EmitInitial:    emitInitial,
		Templates:      templates,
	}, nil
}

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	client := cantonledger.New(cfg.ParticipantURL, cfg.UserID, cfg.RequestTimeout)
	log.Printf("bond listener started (participant=%s, interval=%s)", cfg.ParticipantURL, cfg.PollInterval)

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	var prev map[string]cantonledger.CreatedEvent
	initialized := false

	for {
		events, err := fetchEvents(client, cfg.Templates)
		if err != nil {
			log.Printf("poll error: %v", err)
		} else {
			current := make(map[string]cantonledger.CreatedEvent, len(events))
			for _, evt := range events {
				current[evt.ContractID] = evt
			}

			if !initialized {
				if cfg.EmitInitial {
					for _, evt := range current {
						logEvent("created", evt)
					}
				}
				initialized = true
				prev = current
			} else {
				for id, evt := range current {
					if _, ok := prev[id]; !ok {
						logEvent("created", evt)
					}
				}
				for id, evt := range prev {
					if _, ok := current[id]; !ok {
						logEvent("archived", evt)
					}
				}
				prev = current
			}
		}

		<-ticker.C
	}
}

func fetchEvents(client *cantonledger.Client, templates []string) ([]cantonledger.CreatedEvent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), client.HTTPClient().Timeout)
	defer cancel()

	offset, err := client.LedgerEnd(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := client.ActiveContracts(ctx, offset)
	if err != nil {
		return nil, err
	}
	return cantonledger.ExtractCreatedEvents(resp, templates...), nil
}

func logEvent(action string, evt cantonledger.CreatedEvent) {
	payload := map[string]any{
		"time":       time.Now().UTC().Format(time.RFC3339Nano),
		"action":     action,
		"contractId": evt.ContractID,
		"templateId": evt.TemplateID,
	}

	if fields := extractKeyFields(evt); len(fields) > 0 {
		payload["fields"] = fields
	}
	if len(evt.CreateArguments) > 0 {
		payload["raw"] = evt.CreateArguments
	}

	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("event marshal error: %v", err)
		return
	}
	log.Printf("%s", data)
}

func extractKeyFields(evt cantonledger.CreatedEvent) map[string]any {
	fields := map[string]any{}

	setIf := func(key string, val any) {
		if val == nil {
			return
		}
		switch v := val.(type) {
		case string:
			if strings.TrimSpace(v) == "" {
				return
			}
		}
		fields[key] = val
	}

	setIf("admin", evt.GetStringField("admin"))
	setIf("owner", evt.GetStringField("owner"))
	setIf("amount", evt.GetStringField("amount"))
	setIf("couponRate", evt.GetStringField("couponRate"))
	setIf("maturityDate", evt.GetStringField("maturityDate"))
	setIf("description", evt.GetStringField("description"))

	if transferRaw, ok := evt.GetField("transfer"); ok {
		if transfer, ok := transferRaw.(map[string]any); ok {
			if sender, ok := transfer["sender"].(string); ok {
				setIf("sender", sender)
			}
			if receiver, ok := transfer["receiver"].(string); ok {
				setIf("receiver", receiver)
			}
			if amount, ok := transfer["amount"]; ok {
				setIf("transferAmount", amount)
			}
		}
	}

	if instRaw, ok := evt.GetField("instrumentId"); ok {
		if inst, ok := instRaw.(map[string]any); ok {
			admin := fmt.Sprintf("%v", inst["admin"])
			id := fmt.Sprintf("%v", inst["id"])
			if admin != "" || id != "" {
				setIf("instrumentId", fmt.Sprintf("%s:%s", admin, id))
			}
		}
	}

	return fields
}

func splitList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
