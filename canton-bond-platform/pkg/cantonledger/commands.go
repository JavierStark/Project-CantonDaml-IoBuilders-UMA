package cantonledger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Command is a ledger command for submit-and-wait.
type Command struct {
	CreateCommand   *CreateCommand   `json:"CreateCommand,omitempty"`
	ExerciseCommand *ExerciseCommand `json:"ExerciseCommand,omitempty"`
}

// CreateCommand creates a new contract.
type CreateCommand struct {
	TemplateID      string `json:"templateId"`
	CreateArguments any    `json:"createArguments"`
}

// ExerciseCommand exercises a choice on a contract.
type ExerciseCommand struct {
	TemplateID     string `json:"templateId"`
	Choice         string `json:"choice"`
	ContractID     string `json:"contractId"`
	ChoiceArgument any    `json:"choiceArgument"`
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

// SubmitCommand submits a single command and waits for completion.
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
