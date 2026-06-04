package cantonledger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// PartyDetail is a party returned by the ledger parties API.
type PartyDetail struct {
	Party       string `json:"party"`
	IsLocal     bool   `json:"isLocal"`
	DisplayName string `json:"displayName,omitempty"`
}

type partiesResponse struct {
	PartyDetails []PartyDetail `json:"partyDetails"`
}

// Parties lists parties on the participant.
func (c *Client) Parties(ctx context.Context) ([]PartyDetail, error) {
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
		return []PartyDetail{}, nil
	}
	return out.PartyDetails, nil
}

// AllocateParty creates a new party with the given hint.
func (c *Client) AllocateParty(ctx context.Context, hint string) (PartyDetail, error) {
	url := c.baseURL + "/v2/parties"
	body := map[string]string{"partyIdHint": hint}
	data, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return PartyDetail{}, fmt.Errorf("create allocate request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	rawResp, err := c.doRaw(req)
	if err != nil {
		return PartyDetail{}, fmt.Errorf("allocate party: %w", err)
	}

	if details, ok := rawResp["partyDetails"].([]any); ok && len(details) > 0 {
		return parsePartyDetail(details[0])
	}
	if details, ok := rawResp["partyDetails"].(map[string]any); ok {
		return parsePartyDetail(details)
	}
	return PartyDetail{}, fmt.Errorf("allocate party returned unexpected response: %v", rawResp)
}

func parsePartyDetail(v any) (PartyDetail, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return PartyDetail{}, fmt.Errorf("unexpected party detail type")
	}
	p := PartyDetail{}
	if party, ok := m["party"].(string); ok {
		p.Party = party
	}
	if display, ok := m["displayName"].(string); ok {
		p.DisplayName = display
	}
	if isLocal, ok := m["isLocal"].(bool); ok {
		p.IsLocal = isLocal
	}
	return p, nil
}
