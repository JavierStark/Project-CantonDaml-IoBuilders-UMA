package ledger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL string
	userID  string
	party   string
	http    *http.Client
}

type Command struct {
	CreateCommand   *CreateCommand   `json:"CreateCommand,omitempty"`
	ExerciseCommand *ExerciseCommand `json:"ExerciseCommand,omitempty"`
}

type CreateCommand struct {
	TemplateID      string      `json:"templateId"`
	CreateArguments interface{} `json:"createArguments"`
}

type ExerciseCommand struct {
	TemplateID     string      `json:"templateId"`
	Choice         string      `json:"choice"`
	ContractID     string      `json:"contractId"`
	ChoiceArgument interface{} `json:"choiceArgument"`
}

type SubmitCommandsRequest struct {
	Commands  []Command `json:"commands"`
	UserID    string    `json:"userId"`
	CommandID string    `json:"commandId"`
	ActAs     []string  `json:"actAs"`
	ReadAs    []string  `json:"readAs"`
}

type SubmitAndWaitResponse struct {
	CompletionOffset int64 `json:"completionOffset"`
}

type LedgerEndResponse struct {
	Offset int64 `json:"offset"`
}

type ActiveContractsRequest struct {
	Filter         map[string]map[string]interface{} `json:"filter"`
	Verbose        bool                              `json:"verbose"`
	ActiveAtOffset int64                             `json:"activeAtOffset"`
	EventFormat    interface{}                       `json:"eventFormat"`
}

type ActiveContractsResponse struct {
	ContractEntry map[string]interface{} `json:"contractEntry"`
}

func New(baseURL, userID, party string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		userID:  userID,
		party:   party,
		http: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) SubmitCreate(ctx context.Context, commandID, templateID string, args interface{}) (SubmitAndWaitResponse, error) {
	req := SubmitCommandsRequest{
		Commands:  []Command{{CreateCommand: &CreateCommand{TemplateID: templateID, CreateArguments: args}}},
		UserID:    c.userID,
		CommandID: commandID,
		ActAs:     []string{c.party},
		ReadAs:    []string{c.party},
	}
	return c.submitAndWait(ctx, req)
}

func (c *Client) SubmitExercise(ctx context.Context, commandID, templateID, choice, contractID string, arg interface{}) (SubmitAndWaitResponse, error) {
	req := SubmitCommandsRequest{
		Commands: []Command{{ExerciseCommand: &ExerciseCommand{
			TemplateID:     templateID,
			Choice:         choice,
			ContractID:     contractID,
			ChoiceArgument: arg,
		}}},
		UserID:    c.userID,
		CommandID: commandID,
		ActAs:     []string{c.party},
		ReadAs:    []string{c.party},
	}
	return c.submitAndWait(ctx, req)
}

func (c *Client) LedgerEnd(ctx context.Context) (int64, error) {
	url := fmt.Sprintf("%s/v2/state/ledger-end", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("ledger-end status %d: %s", resp.StatusCode, string(body))
	}
	var out LedgerEndResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, err
	}
	return out.Offset, nil
}

func (c *Client) ActiveContracts(ctx context.Context, offset int64) ([]ActiveContractsResponse, error) {
	url := fmt.Sprintf("%s/v2/state/active-contracts", c.baseURL)
	body := ActiveContractsRequest{
		Filter: map[string]map[string]interface{}{
			"filtersByParty": {},
			"filtersForAnyParty": {
				"cumulative": []map[string]interface{}{
					{
						"identifierFilter": map[string]interface{}{
							"WildcardFilter": map[string]interface{}{
								"value": map[string]interface{}{
									"includeCreatedEventBlob": true,
								},
							},
						},
					},
				},
			},
		},
		Verbose:        false,
		ActiveAtOffset: offset,
		EventFormat:    nil,
	}
	var out []ActiveContractsResponse
	return postJSON(ctx, c.http, url, body, out)
}

func (c *Client) submitAndWait(ctx context.Context, req SubmitCommandsRequest) (SubmitAndWaitResponse, error) {
	url := fmt.Sprintf("%s/v2/commands/submit-and-wait", c.baseURL)
	var out SubmitAndWaitResponse
	return postJSON(ctx, c.http, url, req, out)
}

func postJSON[T any](ctx context.Context, httpClient *http.Client, url string, payload interface{}, zero T) (T, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return zero, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return zero, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return zero, fmt.Errorf("request failed %d: %s", resp.StatusCode, string(body))
	}
	if err := json.NewDecoder(resp.Body).Decode(&zero); err != nil {
		return zero, err
	}
	return zero, nil
}
