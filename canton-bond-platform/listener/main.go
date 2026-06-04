package main

import (
  "bytes"
  "context"
  "encoding/json"
  "fmt"
  "io"
  "log"
  "net/http"
  "os"
  "strings"
  "time"
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

  templates := splitList(getEnv("LISTENER_TEMPLATES", strings.Join(defaultTemplates(), ",")))

  return Config{
    ParticipantURL: getEnv("LISTENER_PARTICIPANT_URL", "http://participant1:5013"),
    UserID:         getEnv("LISTENER_USER_ID", "ledger-api-user"),
    PollInterval:   pollInterval,
    RequestTimeout: requestTimeout,
    EmitInitial:    emitInitial,
    Templates:      templates,
  }, nil
}

func defaultTemplates() []string {
  return []string{
    TemplateSimpleTokenRules,
    TemplateSimpleHolding,
    TemplateLockedSimpleHolding,
    TemplateSimpleTransferInstruction,
    TemplateSimpleAllocation,
  }
}

func main() {
  cfg, err := LoadConfig()
  if err != nil {
    log.Fatalf("config error: %v", err)
  }

  client := NewClient(cfg.ParticipantURL, cfg.UserID, cfg.RequestTimeout)
  log.Printf("bond listener started (participant=%s, interval=%s)", cfg.ParticipantURL, cfg.PollInterval)

  ticker := time.NewTicker(cfg.PollInterval)
  defer ticker.Stop()

  var prev map[string]CreatedEvent
  initialized := false

  for {
    events, err := fetchEvents(client, cfg.Templates)
    if err != nil {
      log.Printf("poll error: %v", err)
    } else {
      current := make(map[string]CreatedEvent, len(events))
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

func fetchEvents(client *Client, templates []string) ([]CreatedEvent, error) {
  ctx, cancel := context.WithTimeout(context.Background(), client.http.Timeout)
  defer cancel()

  offset, err := client.LedgerEnd(ctx)
  if err != nil {
    return nil, err
  }
  resp, err := client.ActiveContracts(ctx, offset)
  if err != nil {
    return nil, err
  }
  return ExtractCreatedEvents(resp, templates...), nil
}

func logEvent(action string, evt CreatedEvent) {
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

func extractKeyFields(evt CreatedEvent) map[string]any {
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

type Client struct {
  baseURL string
  userID  string
  http    *http.Client
}

type ledgerEndResponse struct {
  Offset int64 `json:"offset"`
}

type activeContractsResponse []map[string]any

func NewClient(baseURL, userID string, timeout time.Duration) *Client {
  return &Client{
    baseURL: baseURL,
    userID:  userID,
    http:    &http.Client{Timeout: timeout},
  }
}

func (c *Client) LedgerEnd(ctx context.Context) (int64, error) {
  url := c.baseURL + "/v2/state/ledger-end"
  req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
  if err != nil {
    return 0, fmt.Errorf("create ledger-end request: %w", err)
  }
  var out ledgerEndResponse
  if err := c.doJSON(req, &out); err != nil {
    return 0, fmt.Errorf("ledger-end: %w", err)
  }
  return out.Offset, nil
}

func (c *Client) ActiveContracts(ctx context.Context, offset int64) (activeContractsResponse, error) {
  url := c.baseURL + "/v2/state/active-contracts"

  body := map[string]any{
    "filter": map[string]any{
      "filtersByParty": map[string]any{},
      "filtersForAnyParty": map[string]any{
        "cumulative": []any{
          map[string]any{
            "identifierFilter": map[string]any{
              "WildcardFilter": map[string]any{
                "value": map[string]any{
                  "includeCreatedEventBlob": true,
                },
              },
            },
          },
        },
      },
    },
    "activeAtOffset": offset,
    "verbose":        false,
  }

  data, err := json.Marshal(body)
  if err != nil {
    return nil, fmt.Errorf("marshal active-contracts request: %w", err)
  }

  req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
  if err != nil {
    return nil, fmt.Errorf("create active-contracts request: %w", err)
  }
  req.Header.Set("Content-Type", "application/json")

  resp, err := c.http.Do(req)
  if err != nil {
    return nil, fmt.Errorf("active-contracts request: %w", err)
  }
  defer resp.Body.Close()

  if resp.StatusCode >= 300 {
    respBody, _ := io.ReadAll(resp.Body)
    return nil, fmt.Errorf("active-contracts status %d: %s", resp.StatusCode, string(respBody))
  }

  var out activeContractsResponse
  if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
    return nil, fmt.Errorf("decode active-contracts: %w", err)
  }
  return out, nil
}

func (c *Client) doJSON(req *http.Request, out any) error {
  resp, err := c.http.Do(req)
  if err != nil {
    return fmt.Errorf("http request: %w", err)
  }
  defer resp.Body.Close()

  if resp.StatusCode >= 300 {
    respBody, _ := io.ReadAll(resp.Body)
    return fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
  }

  if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
    return fmt.Errorf("decode response: %w", err)
  }
  return nil
}

type CreatedEvent struct {
  ContractID      string
  TemplateID      string
  CreateArguments map[string]any
}

func ExtractCreatedEvents(resp activeContractsResponse, filterTemplates ...string) []CreatedEvent {
  var events []CreatedEvent
  for _, entry := range resp {
    ce, ok := extractCreatedEvent(entry)
    if ok {
      if len(filterTemplates) > 0 {
        for _, ft := range filterTemplates {
          if templateIDMatches(ce.TemplateID, ft) {
            events = append(events, ce)
            break
          }
        }
      } else {
        events = append(events, ce)
      }
    }
  }
  return events
}

func extractCreatedEvent(entry map[string]any) (CreatedEvent, bool) {
  if ce, ok := entry["createdEvent"].(map[string]any); ok {
    return parseCreatedEvent(ce), true
  }
  if ce, ok := entry["contractEntry"].(map[string]any); ok {
    if evt, ok := ce["createdEvent"].(map[string]any); ok {
      return parseCreatedEvent(evt), true
    }
    if js, ok := ce["JsActiveContract"].(map[string]any); ok {
      if evt, ok := js["createdEvent"].(map[string]any); ok {
        return parseCreatedEvent(evt), true
      }
    }
  }
  return CreatedEvent{}, false
}

func parseCreatedEvent(raw map[string]any) CreatedEvent {
  evt := CreatedEvent{
    CreateArguments: raw,
  }
  if cid, ok := raw["contractId"].(string); ok {
    evt.ContractID = cid
  }
  if tid, ok := raw["templateId"].(string); ok {
    evt.TemplateID = tid
  }
  if args, ok := raw["createArgument"].(map[string]any); ok {
    evt.CreateArguments = args
  }
  return evt
}

func (e CreatedEvent) GetField(name string) (any, bool) {
  v, ok := e.CreateArguments[name]
  return v, ok
}

func (e CreatedEvent) GetStringField(name string) string {
  v, ok := e.GetField(name)
  if !ok {
    return ""
  }
  switch s := v.(type) {
  case string:
    return s
  default:
    b, _ := json.Marshal(s)
    return string(bytes.Trim(b, "\""))
  }
}

func templateIDMatches(actual, expected string) bool {
  if actual == expected {
    return true
  }
  if expected == "" || actual == "" {
    return false
  }
  return templateIDTail(actual) == templateIDTail(expected)
}

func templateIDTail(id string) string {
  if idx := strings.Index(id, ":"); idx >= 0 && idx+1 < len(id) {
    return id[idx+1:]
  }
  return id
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

const (
  TemplateSimpleTokenRules          = "#simple-token:SimpleToken.Rules:SimpleTokenRules"
  TemplateSimpleHolding             = "#simple-token:SimpleToken.Holding:SimpleHolding"
  TemplateLockedSimpleHolding       = "#simple-token:SimpleToken.Holding:LockedSimpleHolding"
  TemplateSimpleTransferInstruction = "#simple-token:SimpleToken.TransferInstruction:SimpleTransferInstruction"
  TemplateSimpleAllocation          = "#simple-token:SimpleToken.Allocation:SimpleAllocation"
)
