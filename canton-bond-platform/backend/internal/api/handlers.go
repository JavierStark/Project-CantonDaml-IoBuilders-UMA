package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"canton-bond-platform/backend/internal/config"
	"canton-bond-platform/backend/internal/ledger"
)

// Server wraps an HTTP server with the API handlers.
type Server struct {
	cfg    config.Config
	clients map[string]*ledger.Client // participant name -> client
}

// NewServer creates a new API server.
func NewServer(cfg config.Config) *Server {
	clients := make(map[string]*ledger.Client)
	for _, p := range cfg.Participants {
		clients[p.Name] = ledger.New(p.URL, cfg.UserID, cfg.RequestTimeout)
	}
	return &Server{cfg: cfg, clients: clients}
}

// Routes returns the HTTP handler with all routes registered.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	mux.HandleFunc("GET /api/v1/parties", s.handleParties)
	mux.HandleFunc("POST /api/v1/parties", s.handleAllocateParty)
	mux.HandleFunc("GET /api/v1/holdings", s.handleListHoldings)
	mux.HandleFunc("POST /api/v1/mint", s.handleMint)
	mux.HandleFunc("POST /api/v1/transfer", s.handleTransfer)
	mux.HandleFunc("POST /api/v1/transfer/accept", s.handleAcceptTransfer)
	mux.HandleFunc("POST /api/v1/transfer/reject", s.handleRejectTransfer)
	mux.HandleFunc("POST /api/v1/transfer/withdraw", s.handleWithdrawTransfer)
	mux.HandleFunc("POST /api/v1/burn", s.handleBurn)
	mux.HandleFunc("POST /api/v1/self-transfer", s.handleSelfTransfer)
	mux.HandleFunc("GET /api/v1/transfer-instructions", s.handleListTransferInstructions)
	mux.HandleFunc("GET /api/v1/factory", s.handleFactory)
	mux.HandleFunc("POST /api/v1/factory", s.handleFactory)

	return corsMiddleware(loggingMiddleware(mux))
}

func (s *Server) clientForParty(party string) (*ledger.Client, error) {
	party = strings.TrimSpace(party)
	p := s.cfg.PartyToParticipant(party)
	if p == nil {
		partyShort := strings.SplitN(party, "::", 2)[0]
		if partyShort != "" {
			p = s.cfg.PartyToParticipant(partyShort)
		}
	}
	if p == nil {
		// Fallback: resolve dynamically from participant party lists.
		ctx, cancel := context.WithTimeout(context.Background(), s.cfg.RequestTimeout)
		defer cancel()

		for _, participant := range s.cfg.Participants {
			client, ok := s.clients[participant.Name]
			if !ok {
				continue
			}
			parties, err := client.Parties(ctx)
			if err != nil {
				continue
			}
			for _, listed := range parties {
				listedShort := strings.SplitN(listed.Party, "::", 2)[0]
				if strings.EqualFold(listed.Party, party) ||
					strings.EqualFold(listedShort, party) ||
					(listed.DisplayName != "" && strings.EqualFold(listed.DisplayName, party)) {
					return client, nil
				}
			}
		}

		return nil, fmt.Errorf("unknown party %q", party)
	}
	client, ok := s.clients[p.Name]
	if !ok {
		return nil, fmt.Errorf("no client for participant %q hosting party %q", p.Name, party)
	}
	return client, nil
}

func (s *Server) lookupPartyIdentifier(ctx context.Context, client *ledger.Client, partyName string) (string, error) {
	partyName = strings.TrimSpace(partyName)
	partyShort := strings.SplitN(partyName, "::", 2)[0]
	parties, err := client.Parties(ctx)
	if err != nil {
		return "", fmt.Errorf("list parties: %w", err)
	}
	for _, p := range parties {
		listedShort := strings.SplitN(p.Party, "::", 2)[0]
		if strings.EqualFold(p.Party, partyName) ||
			strings.EqualFold(p.DisplayName, partyName) ||
			strings.EqualFold(listedShort, partyName) ||
			strings.EqualFold(listedShort, partyShort) {
			return p.Party, nil
		}
	}
	return "", fmt.Errorf("party %q not found", partyName)
}

// --- Handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleParties(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.RequestTimeout)
	defer cancel()

	type partyEntry struct {
		Identifier  string `json:"identifier"`
		DisplayName string `json:"displayName"`
		Participant string `json:"participant"`
	}

	var allParties []partyEntry
	seen := make(map[string]bool)
	for _, p := range s.cfg.Participants {
		client := s.clients[p.Name]
		parties, err := client.Parties(ctx)
		if err != nil {
			log.Printf("error listing parties on %s: %v", p.Name, err)
			continue
		}
		for _, party := range parties {
			if !party.IsLocal {
				continue
			}
			if seen[party.Party] {
				continue
			}
			seen[party.Party] = true
			allParties = append(allParties, partyEntry{
				Identifier:  party.Party,
				DisplayName: party.DisplayName,
				Participant: p.Name,
			})
		}
	}

	if allParties == nil {
		allParties = []partyEntry{}
	}
	writeJSON(w, http.StatusOK, allParties)
}

type allocatePartyRequest struct {
	Participant string `json:"participant"`
	Hint        string `json:"hint"`
}

func (s *Server) handleAllocateParty(w http.ResponseWriter, r *http.Request) {
	var req allocatePartyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Participant == "" {
		writeError(w, http.StatusBadRequest, "participant is required")
		return
	}
	if req.Hint == "" {
		writeError(w, http.StatusBadRequest, "hint is required")
		return
	}

	client, ok := s.clients[req.Participant]
	if !ok {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unknown participant: %s", req.Participant))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.RequestTimeout)
	defer cancel()

	party, err := client.AllocateParty(ctx, req.Hint)
	if err != nil {
		log.Printf("error allocating party: %v", err)
		writeError(w, http.StatusBadGateway, fmt.Sprintf("failed to allocate party: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"identifier":  party.Party,
		"displayName": party.DisplayName,
		"participant": req.Participant,
	})
}

func (s *Server) handleListHoldings(w http.ResponseWriter, r *http.Request) {
	party := r.URL.Query().Get("party")
	if party == "" {
		writeError(w, http.StatusBadRequest, "party query parameter is required")
		return
	}

	client, err := s.clientForParty(party)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.RequestTimeout)
	defer cancel()

	partyID, err := s.lookupPartyIdentifier(ctx, client, party)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	offset, err := client.LedgerEnd(ctx)
	if err != nil {
		log.Printf("ledger-end error: %v", err)
		writeError(w, http.StatusBadGateway, "failed to query ledger end")
		return
	}

	resp, err := client.ActiveContracts(ctx, offset,
		ledger.TemplateSimpleHolding,
		ledger.TemplateLockedSimpleHolding,
	)
	if err != nil {
		log.Printf("active-contracts error: %v", err)
		writeError(w, http.StatusBadGateway, "failed to query active contracts")
		return
	}

	events := ledger.ExtractCreatedEvents(resp,
		ledger.TemplateSimpleHolding,
		ledger.TemplateLockedSimpleHolding,
	)

	type holdingView struct {
		ContractID   string  `json:"contractId"`
		TemplateID   string  `json:"templateId"`
		Admin        string  `json:"admin"`
		Owner        string  `json:"owner"`
		InstrumentID string  `json:"instrumentId"`
		Amount       float64 `json:"amount"`
		CouponRate   float64 `json:"couponRate"`
		MaturityDate string  `json:"maturityDate"`
		Description  string  `json:"description"`
		Locked       bool    `json:"locked"`
	}

	var holdings []holdingView
	for _, evt := range events {
		admin := evt.GetStringField("admin")
		owner := evt.GetStringField("owner")
		if owner != partyID && admin != partyID {
			continue
		}

		h := holdingView{
			ContractID:   evt.ContractID,
			TemplateID:   evt.TemplateID,
			Admin:        admin,
			Owner:        owner,
			Amount:       evt.GetDecimalField("amount"),
			CouponRate:   evt.GetDecimalField("couponRate"),
			MaturityDate: evt.GetStringField("maturityDate"),
			Description:  evt.GetStringField("description"),
			Locked:       evt.IsLocked(),
		}

		// Extract instrument ID from nested object
		instRaw, _ := evt.GetField("instrumentId")
		if instMap, ok := instRaw.(map[string]any); ok {
			admin, _ := instMap["admin"]
			id, _ := instMap["id"]
			h.InstrumentID = fmt.Sprintf("%v:%v", admin, id)
		}

		holdings = append(holdings, h)
	}

	if holdings == nil {
		holdings = []holdingView{}
	}
	writeJSON(w, http.StatusOK, holdings)
}

type mintRequest struct {
	Admin        string  `json:"admin"`
	Owner        string  `json:"owner"`
	Amount       float64 `json:"amount"`
	CouponRate   float64 `json:"couponRate"`
	MaturityDate string  `json:"maturityDate"`
	Description  string  `json:"description"`
}

func (s *Server) handleMint(w http.ResponseWriter, r *http.Request) {
	var req mintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Admin == "" || req.Owner == "" {
		writeError(w, http.StatusBadRequest, "admin and owner are required")
		return
	}
	if req.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "amount must be positive")
		return
	}
	if req.MaturityDate == "" {
		writeError(w, http.StatusBadRequest, "maturityDate is required")
		return
	}

	// Both admin and owner must be on the same participant for the Mint choice
	adminClient, err := s.clientForParty(req.Admin)
	if err != nil {
		writeError(w, http.StatusBadRequest, "admin: "+err.Error())
		return
	}
	ownerClient, err := s.clientForParty(req.Owner)
	if err != nil {
		writeError(w, http.StatusBadRequest, "owner: "+err.Error())
		return
	}

	// Both must be on the same participant (Mint requires both as controllers)
	if adminClient != ownerClient {
		writeError(w, http.StatusBadRequest, "admin and owner must be on the same participant")
		return
	}
	client := adminClient

	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.RequestTimeout)
	defer cancel()

	adminID, err := s.lookupPartyIdentifier(ctx, client, req.Admin)
	if err != nil {
		writeError(w, http.StatusNotFound, "admin party not found: "+err.Error())
		return
	}
	ownerID, err := s.lookupPartyIdentifier(ctx, client, req.Owner)
	if err != nil {
		writeError(w, http.StatusNotFound, "owner party not found: "+err.Error())
		return
	}

	// Find the SimpleTokenRules factory contract
	offset, err := client.LedgerEnd(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to query ledger end")
		return
	}

	resp, err := client.ActiveContracts(ctx, offset, ledger.TemplateSimpleTokenRules)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to query factory")
		return
	}

	events := ledger.ExtractCreatedEvents(resp, ledger.TemplateSimpleTokenRules)
	if len(events) == 0 {
		writeError(w, http.StatusNotFound, "no SimpleTokenRules factory found. Create one via GET or POST /api/v1/factory")
		return
	}

	factoryCID := events[0].ContractID

	// Build the Mint choice argument
	choiceArg := map[string]any{
		"owner":        ownerID,
		"instrumentId": ledger.InstrumentID(adminID, "BOND"),
		"amount":       ledger.DamlDecimal(req.Amount),
		"couponRate":   ledger.DamlDecimal(req.CouponRate),
		"maturityDate": req.MaturityDate,
		"description":  req.Description,
	}

	cmdID := newCommandID("mint")
	submitReq := ledger.Command{
		ExerciseCommand: &ledger.ExerciseCommand{
			TemplateID:     ledger.TemplateSimpleTokenRules,
			Choice:         ledger.ChoiceMint,
			ContractID:     factoryCID,
			ChoiceArgument: choiceArg,
		},
	}

	offset, err = client.SubmitCommand(ctx, cmdID, submitReq, []string{adminID, ownerID})
	if err != nil {
		log.Printf("mint error: %v", err)
		writeError(w, http.StatusBadGateway, fmt.Sprintf("mint failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "created",
		"offset":   offset,
		"admin":    adminID,
		"owner":    ownerID,
		"amount":   req.Amount,
		"coupon":   req.CouponRate,
		"maturity": req.MaturityDate,
	})
}

type transferRequest struct {
	Sender      string   `json:"sender"`
	Receiver    string   `json:"receiver"`
	Amount      float64  `json:"amount"`
	HoldingCids []string `json:"holdingCids"`
}

func (s *Server) handleTransfer(w http.ResponseWriter, r *http.Request) {
	var req transferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Sender == "" || req.Receiver == "" {
		writeError(w, http.StatusBadRequest, "sender and receiver are required")
		return
	}
	if req.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "amount must be positive")
		return
	}

	senderClient, err := s.clientForParty(req.Sender)
	if err != nil {
		writeError(w, http.StatusBadRequest, "sender: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.RequestTimeout)
	defer cancel()

	senderID, err := s.lookupPartyIdentifier(ctx, senderClient, req.Sender)
	if err != nil {
		writeError(w, http.StatusNotFound, "sender party not found")
		return
	}

	receiverClient, err := s.clientForParty(req.Receiver)
	if err != nil {
		writeError(w, http.StatusBadRequest, "receiver: "+err.Error())
		return
	}
	receiverID, err := s.lookupPartyIdentifier(ctx, receiverClient, req.Receiver)
	if err != nil {
		writeError(w, http.StatusNotFound, "receiver party not found")
		return
	}

	// Use the sender's client to find the factory and holdings
	client := senderClient

	offset, err := client.LedgerEnd(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to query ledger end")
		return
	}

	// Find factory
	factoryResp, err := client.ActiveContracts(ctx, offset, ledger.TemplateSimpleTokenRules)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to query factory")
		return
	}
	factoryEvents := ledger.ExtractCreatedEvents(factoryResp, ledger.TemplateSimpleTokenRules)
	if len(factoryEvents) == 0 {
		writeError(w, http.StatusNotFound, "no SimpleTokenRules factory found")
		return
	}
	factoryCID := factoryEvents[0].ContractID

	// Find sender's holdings
	holdingsResp, err := client.ActiveContracts(ctx, offset, ledger.TemplateSimpleHolding)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to query holdings")
		return
	}
	holdingsEvents := ledger.ExtractCreatedEvents(holdingsResp,
		ledger.TemplateSimpleHolding,
		ledger.TemplateLockedSimpleHolding,
	)

	var inputCIDs []string
	remaining := req.Amount

	if len(req.HoldingCids) > 0 {
		// Use provided holdings - build a lookup map
		provided := make(map[string]bool, len(req.HoldingCids))
		for _, cid := range req.HoldingCids {
			provided[cid] = true
		}
		for _, evt := range holdingsEvents {
			if remaining <= 0 {
				break
			}
			if !provided[evt.ContractID] {
				continue
			}
			owner := evt.GetStringField("owner")
			if owner != senderID {
				continue
			}
			if evt.IsLocked() {
				continue
			}
			amt := evt.GetDecimalField("amount")
			if amt <= 0 {
				continue
			}
			inputCIDs = append(inputCIDs, evt.ContractID)
			remaining -= amt
		}
	} else {
		// Auto-select unlocked holdings
		for _, evt := range holdingsEvents {
			if remaining <= 0 {
				break
			}
			owner := evt.GetStringField("owner")
			if owner != senderID {
				continue
			}
			if evt.IsLocked() {
				continue
			}
			amt := evt.GetDecimalField("amount")
			if amt <= 0 {
				continue
			}
			inputCIDs = append(inputCIDs, evt.ContractID)
			remaining -= amt
		}
	}

	if remaining > 0 {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("insufficient holdings: need %.2f more", remaining))
		return
	}

	log.Printf("transfer inputs: %d holdings, first: %.20s, remaining: %.2f, amount: %.2f", len(inputCIDs), inputCIDs[0], remaining, req.Amount)

	// Get factory admin from the factory contract
	factoryAdmin := factoryEvents[0].GetStringField("admin")

	// Build TransferFactory_Transfer choice argument
	transferArg := map[string]any{
		"sender":           senderID,
		"receiver":         receiverID,
		"amount":           ledger.DamlDecimal(req.Amount),
		"instrumentId":     ledger.InstrumentID(factoryAdmin, "BOND"),
		"requestedAt":      time.Now().UTC().Format("2006-01-02T15:04:05.000000Z"),
		"executeBefore":    time.Now().UTC().Add(24 * time.Hour).Format("2006-01-02T15:04:05.000000Z"),
		"inputHoldingCids": inputCIDs,
		"meta":             map[string]any{"values": map[string]any{}},
	}
	choiceArg := map[string]any{
		"expectedAdmin": factoryAdmin,
		"transfer":      transferArg,
		"extraArgs":     map[string]any{"context": map[string]any{"values": map[string]any{}}, "meta": map[string]any{"values": map[string]any{}}},
	}

	cmdID := newCommandID("transfer")
	submitReq := ledger.Command{
		ExerciseCommand: &ledger.ExerciseCommand{
			TemplateID:     ledger.TemplateTransferFactory,
			Choice:         ledger.ChoiceTransferFactoryTransfer,
			ContractID:     factoryCID,
			ChoiceArgument: choiceArg,
		},
	}

	// Include admin in actAs so the contract is visible through the interface
	offset, err = client.SubmitCommand(ctx, cmdID, submitReq, []string{senderID, factoryAdmin})
	if err != nil {
		log.Printf("transfer error: %v", err)
		writeError(w, http.StatusBadGateway, fmt.Sprintf("transfer failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "pending",
		"offset":   offset,
		"sender":   senderID,
		"receiver": receiverID,
		"amount":   req.Amount,
	})
}

type transferActionRequest struct {
	Party      string `json:"party"`
	ContractID string `json:"contractId"`
}

func (s *Server) handleAcceptTransfer(w http.ResponseWriter, r *http.Request) {
	s.handleTransferAction(w, r, ledger.ChoiceTransferInstructionAccept, "accept")
}

func (s *Server) handleRejectTransfer(w http.ResponseWriter, r *http.Request) {
	s.handleTransferAction(w, r, ledger.ChoiceTransferInstructionReject, "reject")
}

func (s *Server) handleWithdrawTransfer(w http.ResponseWriter, r *http.Request) {
	s.handleTransferAction(w, r, ledger.ChoiceTransferInstructionWithdraw, "withdraw")
}

func (s *Server) handleTransferAction(w http.ResponseWriter, r *http.Request, choice, action string) {
	var req transferActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Party == "" || req.ContractID == "" {
		writeError(w, http.StatusBadRequest, "party and contractId are required")
		return
	}

	client, err := s.clientForParty(req.Party)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.RequestTimeout)
	defer cancel()

	partyID, err := s.lookupPartyIdentifier(ctx, client, req.Party)
	if err != nil {
		writeError(w, http.StatusNotFound, "party not found")
		return
	}

	cmdID := newCommandID(action)
	submitReq := ledger.Command{
		ExerciseCommand: &ledger.ExerciseCommand{
			TemplateID:  ledger.TemplateTransferInstruction,
			Choice:      choice,
			ContractID:  req.ContractID,
			ChoiceArgument: map[string]any{
				"extraArgs": map[string]any{"context": map[string]any{"values": map[string]any{}}, "meta": map[string]any{"values": map[string]any{}}},
			},
		},
	}

	// Interface choices need the interface templateId and the contract must
	// be visible to the submitting party through the interface.
	offset, err := client.SubmitCommand(ctx, cmdID, submitReq, []string{partyID})
	if err != nil {
		log.Printf("%s error: %v", action, err)
		writeError(w, http.StatusBadGateway, fmt.Sprintf("%s failed: %v", action, err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": action + "ed",
		"offset": offset,
	})
}

type burnRequest struct {
	Party      string `json:"party"`
	ContractID string `json:"contractId"`
	AsAdmin    bool   `json:"asAdmin,omitempty"`
}

func (s *Server) handleBurn(w http.ResponseWriter, r *http.Request) {
	var req burnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Party == "" || req.ContractID == "" {
		writeError(w, http.StatusBadRequest, "party and contractId are required")
		return
	}

	client, err := s.clientForParty(req.Party)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.RequestTimeout)
	defer cancel()

	partyID, err := s.lookupPartyIdentifier(ctx, client, req.Party)
	if err != nil {
		writeError(w, http.StatusNotFound, "party not found")
		return
	}

	choice := ledger.ChoiceBurn
	if req.AsAdmin {
		choice = ledger.ChoiceBurnByAdmin
	}

	cmdID := newCommandID("burn")
	submitReq := ledger.Command{
		ExerciseCommand: &ledger.ExerciseCommand{
			TemplateID:     ledger.TemplateSimpleHolding,
			Choice:         choice,
			ContractID:     req.ContractID,
			ChoiceArgument: map[string]any{},
		},
	}

	offset, err := client.SubmitCommand(ctx, cmdID, submitReq, []string{partyID})
	if err != nil {
		log.Printf("burn error: %v", err)
		writeError(w, http.StatusBadGateway, fmt.Sprintf("burn failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "burned",
		"offset": offset,
	})
}

func (s *Server) handleSelfTransfer(w http.ResponseWriter, r *http.Request) {
	var req transferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Sender == "" || req.Receiver == "" {
		writeError(w, http.StatusBadRequest, "sender and receiver are required")
		return
	}
	if req.Sender != req.Receiver {
		writeError(w, http.StatusBadRequest, "self-transfer requires sender == receiver")
		return
	}

	// Reuse the transfer handler but with sender == receiver
	s.handleTransfer(w, r)
}

func (s *Server) handleListTransferInstructions(w http.ResponseWriter, r *http.Request) {
	party := r.URL.Query().Get("party")
	if party == "" {
		writeError(w, http.StatusBadRequest, "party query parameter is required")
		return
	}

	client, err := s.clientForParty(party)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.RequestTimeout)
	defer cancel()

	partyID, err := s.lookupPartyIdentifier(ctx, client, party)
	if err != nil {
		writeError(w, http.StatusNotFound, "party not found")
		return
	}

	offset, err := client.LedgerEnd(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to query ledger end")
		return
	}

	resp, err := client.ActiveContracts(ctx, offset, ledger.TemplateSimpleTransferInstruction)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to query transfer instructions")
		return
	}

	events := ledger.ExtractCreatedEvents(resp, ledger.TemplateSimpleTransferInstruction)

	type transferView struct {
		ContractID string  `json:"contractId"`
		Sender     string  `json:"sender"`
		Receiver   string  `json:"receiver"`
		Amount     float64 `json:"amount"`
	}

	var transfers []transferView
	for _, evt := range events {
		transferRaw, hasTransfer := evt.GetField("transfer")
		if !hasTransfer {
			log.Printf("transfer-instructions: contract %s missing transfer field", evt.ContractID)
			continue
		}
		transferMap, ok := transferRaw.(map[string]any)
		if !ok {
			log.Printf("transfer-instructions: contract %s transfer field has unexpected type %T", evt.ContractID, transferRaw)
			continue
		}

		sender, senderOK := transferMap["sender"].(string)
		receiver, receiverOK := transferMap["receiver"].(string)
		if !senderOK || sender == "" || !receiverOK || receiver == "" {
			log.Printf("transfer-instructions: contract %s has invalid sender/receiver (sender=%v receiver=%v)", evt.ContractID, transferMap["sender"], transferMap["receiver"])
			continue
		}

		if sender != partyID && receiver != partyID {
			continue
		}

		t := transferView{
			ContractID: evt.ContractID,
			Sender:     sender,
			Receiver:   receiver,
		}

		if amtRaw, ok := transferMap["amount"]; ok {
			switch v := amtRaw.(type) {
			case string:
				fmt.Sscanf(v, "%f", &t.Amount)
			case float64:
				t.Amount = v
			}
		}

		transfers = append(transfers, t)
	}

	if transfers == nil {
		transfers = []transferView{}
	}
	writeJSON(w, http.StatusOK, transfers)
}

func (s *Server) handleFactory(w http.ResponseWriter, r *http.Request) {
	// Try to find the factory on participant1
	client, ok := s.clients["participant1"]
	if !ok {
		writeError(w, http.StatusInternalServerError, "no participant1 client")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.RequestTimeout)
	defer cancel()

	offset, err := client.LedgerEnd(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to query ledger end")
		return
	}

	resp, err := client.ActiveContracts(ctx, offset, ledger.TemplateSimpleTokenRules)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to query factory")
		return
	}

	events := ledger.ExtractCreatedEvents(resp, ledger.TemplateSimpleTokenRules)
	if len(events) == 0 {
		// No factory found, create one
		adminID, err := s.lookupPartyIdentifier(ctx, client, "admin")
		if err != nil {
			writeError(w, http.StatusNotFound, "admin party not found on participant1. Bootstrap first.")
			return
		}

		createArgs := map[string]any{
			"admin":               adminID,
			"supportedInstruments": []string{"BOND"},
		}

		cmdID := newCommandID("create-factory")
		submitReq := ledger.Command{
			CreateCommand: &ledger.CreateCommand{
				TemplateID:      ledger.TemplateSimpleTokenRules,
				CreateArguments: createArgs,
			},
		}

		offset, err := client.SubmitCommand(ctx, cmdID, submitReq, []string{adminID})
		if err != nil {
			log.Printf("create factory error: %v", err)
			writeError(w, http.StatusBadGateway, fmt.Sprintf("create factory failed: %v", err))
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"status":   "created",
			"offset":   offset,
			"admin":    adminID,
			"instruments": []string{"BOND"},
		})
		return
	}

	factory := events[0]
	writeJSON(w, http.StatusOK, map[string]any{
		"contractId":  factory.ContractID,
		"templateId":  factory.TemplateID,
		"admin":       factory.GetStringField("admin"),
		"instruments": factory.GetStringField("supportedInstruments"),
	})
}

func newCommandID(prefix string) string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(b))
}
