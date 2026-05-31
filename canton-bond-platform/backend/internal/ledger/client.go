package ledger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Client communicates with a Canton participant JSON Ledger API v2.
type Client struct {
	baseURL string
	userID  string
	http    *http.Client
}

type partiesResponse struct {
	PartyDetails []partyDetail `json:"partyDetails"`
}

type partyDetail struct {
	Party       string `json:"party"`
	IsLocal     bool   `json:"isLocal"`
	DisplayName string `json:"displayName,omitempty"`
}

type ledgerEndResponse struct {
	Offset int64 `json:"offset"`
}

type submitRequest struct {
	Commands  []Command `json:"commands"`
	UserID    string    `json:"userId"`
	CommandID string    `json:"commandId"`
	ActAs     []string  `json:"actAs"`
	ReadAs    []string  `json:"readAs"`
}

type submitResponse struct {
	CompletionOffset int64 `json:"completionOffset"`
}

// Command is a Canton ledger API command.
type Command struct {
	CreateCommand   *CreateCommand   `json:"CreateCommand,omitempty"`
	ExerciseCommand *ExerciseCommand `json:"ExerciseCommand,omitempty"`
}

// CreateCommand creates a new contract.
type CreateCommand struct {
	TemplateID      string `json:"templateId"`
	CreateArguments any    `json:"createArguments"`
}

// ExerciseCommand exercises a choice on an existing contract.
// NOTE: JSON API V2 ExerciseCommand does not support choiceInterfaceId.
// For interface choices, use the interface templateId as TemplateID.
type ExerciseCommand struct {
	TemplateID     string `json:"templateId"`
	Choice         string `json:"choice"`
	ContractID     string `json:"contractId"`
	ChoiceArgument any    `json:"choiceArgument"`
}

type activeContractsResponse []map[string]any

// New creates a new Canton JSON API client.
func New(baseURL, userID string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		userID:  userID,
		http:    &http.Client{Timeout: timeout},
	}
}

// LedgerEnd returns the current ledger end offset.
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

// ActiveContracts returns active contracts, optionally filtered by template ID.
func (c *Client) ActiveContracts(ctx context.Context, offset int64, templateIDs ...string) (activeContractsResponse, error) {
	url := c.baseURL + "/v2/state/active-contracts"

	filter := c.buildFilter(templateIDs)

	body := map[string]any{
		"filter":         filter,
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

func (c *Client) buildFilter(templateIDs []string) map[string]any {
	// JSON API V2 WildcardFilter to get all contracts; template filtering
	// is done in Go via ExtractCreatedEvents.
	return map[string]any{
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
	}
}

// SubmitCommand submits a single command (create or exercise) and waits for completion.
func (c *Client) SubmitCommand(ctx context.Context, commandID string, cmd Command, actAs []string) (int64, error) {
	req := submitRequest{
		Commands:  []Command{cmd},
		UserID:    c.userID,
		CommandID: commandID,
		ActAs:     actAs,
		ReadAs:    actAs,
	}
	return c.submitAndWait(ctx, req)
}

func (c *Client) submitAndWait(ctx context.Context, req submitRequest) (int64, error) {
	url := c.baseURL + "/v2/commands/submit-and-wait"
	data, err := json.Marshal(req)
	if err != nil {
		return 0, fmt.Errorf("marshal submit request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return 0, fmt.Errorf("create submit request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return 0, fmt.Errorf("submit request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("submit status %d: %s", resp.StatusCode, string(respBody))
	}

	var out submitResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, fmt.Errorf("decode submit response: %w", err)
	}
	return out.CompletionOffset, nil
}

// Parties returns the list of parties on this participant.
func (c *Client) Parties(ctx context.Context) ([]partyDetail, error) {
	url := c.baseURL + "/v2/parties"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create parties request: %w", err)
	}
	var out partiesResponse
	if err := c.doJSON(req, &out); err != nil {
		return nil, fmt.Errorf("parties: %w", err)
	}
	if out.PartyDetails == nil {
		return []partyDetail{}, nil
	}
	return out.PartyDetails, nil
}

// AllocateParty creates a new party on this participant.
func (c *Client) AllocateParty(ctx context.Context, hint string) (partyDetail, error) {
	url := c.baseURL + "/v2/parties"
	body := map[string]string{"partyIdHint": hint}
	data, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return partyDetail{}, fmt.Errorf("create allocate request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	var out partiesResponse
	if err := c.doJSON(req, &out); err != nil {
		return partyDetail{}, fmt.Errorf("allocate party: %w", err)
	}
	if len(out.PartyDetails) == 0 {
		return partyDetail{}, fmt.Errorf("allocate party returned empty result")
	}
	return out.PartyDetails[0], nil
}

// ExtractCreatedEvents extracts all created events from the active contracts response,
// optionally filtered by matching template IDs.
func ExtractCreatedEvents(resp activeContractsResponse, filterTemplates ...string) []CreatedEvent {
	var events []CreatedEvent
	for _, entry := range resp {
		ce, ok := extractCreatedEvent(entry)
		if ok {
			if len(filterTemplates) > 0 {
				for _, ft := range filterTemplates {
					if ce.TemplateID == ft || (ft != "" && strings.Contains(ce.TemplateID, ft)) {
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
	// Try top-level "createdEvent"
	if ce, ok := entry["createdEvent"].(map[string]any); ok {
		return parseCreatedEvent(ce), true
	}
	// Try nested "contractEntry.createdEvent"
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

// CreatedEvent represents a created event from the ledger.
type CreatedEvent struct {
	ContractID      string
	TemplateID      string
	CreateArguments map[string]any
}

// GetField safely extracts a field from the create arguments.
func (e CreatedEvent) GetField(name string) (any, bool) {
	v, ok := e.CreateArguments[name]
	return v, ok
}

// GetStringField extracts a string field.
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

// GetDecimalField extracts a decimal field (Daml decimals are JSON strings).
func (e CreatedEvent) GetDecimalField(name string) float64 {
	s := e.GetStringField(name)
	if s == "" {
		return 0
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// GetNestedField extracts a field from a nested object.
func (e CreatedEvent) GetNestedField(name, subField string) string {
	v, ok := e.GetField(name)
	if !ok {
		return ""
	}
	m, ok := v.(map[string]any)
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	sv, ok := m[subField]
	if !ok {
		return ""
	}
	return fmt.Sprintf("%v", sv)
}

// IsLocked returns true if this is a LockedSimpleHolding.
func (e CreatedEvent) IsLocked() bool {
	return strings.Contains(e.TemplateID, "LockedSimpleHolding")
}

// DAML template IDs used in the bond contract.
const (
	TemplateSimpleTokenRules           = "#simple-token:SimpleToken.Rules:SimpleTokenRules"
	TemplateSimpleHolding              = "#simple-token:SimpleToken.Holding:SimpleHolding"
	TemplateLockedSimpleHolding        = "#simple-token:SimpleToken.Holding:LockedSimpleHolding"
	TemplateSimpleTransferInstruction  = "#simple-token:SimpleToken.TransferInstruction:SimpleTransferInstruction"
	TemplateSimpleAllocation           = "#simple-token:SimpleToken.Allocation:SimpleAllocation"
	TemplateTransferFactory            = "55ba4deb0ad4662c4168b39859738a0e91388d252286480c7331b3f71a517281:Splice.Api.Token.TransferInstructionV1:TransferFactory"
	TemplateTransferInstruction        = "55ba4deb0ad4662c4168b39859738a0e91388d252286480c7331b3f71a517281:Splice.Api.Token.TransferInstructionV1:TransferInstruction"

	ChoiceMint                  = "Mint"
	ChoiceTransferOwnership     = "TransferOwnership"
	ChoiceBurn                  = "Burn"
	ChoiceBurnByAdmin           = "BurnByAdmin"
	ChoiceTransferFactoryTransfer = "TransferFactory_Transfer"
	ChoiceTransferInstructionAccept = "TransferInstruction_Accept"
	ChoiceTransferInstructionReject = "TransferInstruction_Reject"
	ChoiceTransferInstructionWithdraw = "TransferInstruction_Withdraw"
	ChoiceLockedSimpleHoldingUnlock = "LockedSimpleHolding_Unlock"
)

// DamlDecimal encodes a float64 as a Daml-LF decimal string (e.g., "1000.0000000000").
func DamlDecimal(v float64) string {
	return big.NewFloat(v).Text('f', 10)
}

// DamlDate formats a date string as expected by Daml (YYYY-MM-DD).
func DamlDate(date string) string {
	return date
}

// InstrumentID returns the Daml InstrumentId JSON structure.
func InstrumentID(adminParty, code string) map[string]any {
	return map[string]any{
		"admin": adminParty,
		"id":    code,
	}
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
