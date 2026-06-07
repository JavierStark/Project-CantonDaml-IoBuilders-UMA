package cantonledger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client talks to the Canton JSON Ledger API v2.
type Client struct {
	baseURL string
	userID  string
	http    *http.Client
}

// New creates a ledger API client.
func New(baseURL, userID string, timeout time.Duration) *Client {

	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     0,
		DisableCompression:  true,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	return &Client{
		baseURL: baseURL,
		userID:  userID,
		http:    client,
	}
}

// HTTPClient returns the underlying HTTP client (e.g. for caller timeouts).
func (c *Client) HTTPClient() *http.Client {
	return c.http
}

type ledgerEndResponse struct {
	Offset int64 `json:"offset"`
}

// ActiveContractsResponse is the JSON shape returned by active-contracts.
type ActiveContractsResponse []map[string]any

// LedgerEnd returns the current ledger offset.
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

// ActiveContracts queries contracts at the given offset using a wildcard filter.
// Template filtering is applied client-side via ExtractCreatedEvents.
func (c *Client) ActiveContracts(ctx context.Context, offset int64, _ ...string) (ActiveContractsResponse, error) {
	url := c.baseURL + "/v2/state/active-contracts"

	body := map[string]any{
		"filter":         WildcardFilter(),
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

	var out ActiveContractsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode active-contracts: %w", err)
	}
	return out, nil
}

// WildcardFilter is the standard active-contracts filter used across services.
func WildcardFilter() map[string]any {
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

func (c *Client) doRaw(req *http.Request) (map[string]any, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return out, nil
}

// PartyFilter crea el filtro exacto que exige Canton v3 para escuchar los contratos de un usuario específico.
func PartyFilter(party string) map[string]any {
	return map[string]any{
		"filtersByParty": map[string]any{
			party: map[string]any{
				"cumulative": []map[string]any{
					{
						"identifierFilter": map[string]any{
							"WildcardFilter": map[string]any{
								"value": map[string]any{
									"includeCreatedEventBlob": false,
								},
							},
						},
					},
				},
			},
		},
	}
}
