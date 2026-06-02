package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type Server struct {
	cfg     Config
	clients map[string]*Client
}

func NewServer(cfg Config) *Server {
	clients := make(map[string]*Client)
	for _, p := range cfg.Participants {
		clients[p.Name] = New(p.URL, cfg.UserID, cfg.RequestTimeout)
	}
	return &Server{cfg: cfg, clients: clients}
}

func (s *Server) Router() *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Content-Type", "Authorization"},
	}))
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	v1 := e.Group("/api/v1")
	v1.GET("/health", s.handleHealth)
	v1.GET("/parties", s.handleParties)
	v1.POST("/parties", s.handleAllocateParty)
	v1.GET("/holdings", s.handleListHoldings)
	v1.POST("/mint", s.handleMint)
	v1.POST("/transfer", s.handleTransfer)
	v1.POST("/transfer/accept", s.handleAcceptTransfer)
	v1.POST("/transfer/reject", s.handleRejectTransfer)
	v1.POST("/transfer/withdraw", s.handleWithdrawTransfer)
	v1.POST("/burn", s.handleBurn)
	v1.POST("/self-transfer", s.handleSelfTransfer)
	v1.GET("/transfer-instructions", s.handleListTransferInstructions)
	v1.GET("/factory", s.handleFactory)
	v1.POST("/factory", s.handleFactory)

	return e
}

func (s *Server) clientForParty(party string) (*Client, error) {
	party = strings.TrimSpace(party)
	p := s.cfg.PartyToParticipant(party)
	if p == nil {
		partyShort := strings.SplitN(party, "::", 2)[0]
		if partyShort != "" {
			p = s.cfg.PartyToParticipant(partyShort)
		}
	}
	if p == nil {
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

func (s *Server) lookupPartyIdentifier(ctx context.Context, client *Client, partyName string) (string, error) {
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

func (s *Server) handleHealth(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleParties(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), s.cfg.RequestTimeout)
	defer cancel()

	var allParties []map[string]string
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
			allParties = append(allParties, map[string]string{
				"identifier":  party.Party,
				"displayName": party.DisplayName,
				"participant": p.Name,
			})
		}
	}
	if allParties == nil {
		allParties = []map[string]string{}
	}
	return c.JSON(http.StatusOK, allParties)
}

type allocatePartyRequest struct {
	Participant string `json:"participant"`
	Hint        string `json:"hint"`
}

func (s *Server) handleAllocateParty(c echo.Context) error {
	var req allocatePartyRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if req.Participant == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "participant is required"})
	}
	if req.Hint == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "hint is required"})
	}
	client, ok := s.clients[req.Participant]
	if !ok {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("unknown participant: %s", req.Participant)})
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), s.cfg.RequestTimeout)
	defer cancel()

	party, err := client.AllocateParty(ctx, req.Hint)
	if err != nil {
		log.Printf("error allocating party: %v", err)
		return c.JSON(http.StatusBadGateway, map[string]string{"error": fmt.Sprintf("failed to allocate party: %v", err)})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"identifier":  party.Party,
		"displayName": party.DisplayName,
		"participant": req.Participant,
	})
}

func (s *Server) handleListHoldings(c echo.Context) error {
	party := c.QueryParam("party")
	if party == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "party query parameter is required"})
	}

	client, err := s.clientForParty(party)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), s.cfg.RequestTimeout)
	defer cancel()

	partyID, err := s.lookupPartyIdentifier(ctx, client, party)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}

	offset, err := client.LedgerEnd(ctx)
	if err != nil {
		log.Printf("ledger-end error: %v", err)
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "failed to query ledger end"})
	}

	resp, err := client.ActiveContracts(ctx, offset,
		TemplateSimpleHolding,
		TemplateLockedSimpleHolding,
	)
	if err != nil {
		log.Printf("active-contracts error: %v", err)
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "failed to query active contracts"})
	}

	events := ExtractCreatedEvents(resp,
		TemplateSimpleHolding,
		TemplateLockedSimpleHolding,
	)

	var holdings []map[string]any
	for _, evt := range events {
		admin := evt.GetStringField("admin")
		owner := evt.GetStringField("owner")
		if owner != partyID && admin != partyID {
			continue
		}

		h := map[string]any{
			"contractId":   evt.ContractID,
			"templateId":   evt.TemplateID,
			"admin":        admin,
			"owner":        owner,
			"amount":       evt.GetDecimalField("amount"),
			"couponRate":   evt.GetDecimalField("couponRate"),
			"maturityDate": evt.GetStringField("maturityDate"),
			"description":  evt.GetStringField("description"),
			"locked":       evt.IsLocked(),
		}

		instRaw, _ := evt.GetField("instrumentId")
		if instMap, ok := instRaw.(map[string]any); ok {
			a, _ := instMap["admin"]
			id, _ := instMap["id"]
			h["instrumentId"] = fmt.Sprintf("%v:%v", a, id)
		}

		holdings = append(holdings, h)
	}

	if holdings == nil {
		holdings = []map[string]any{}
	}
	return c.JSON(http.StatusOK, holdings)
}

type mintRequest struct {
	Admin        string  `json:"admin"`
	Owner        string  `json:"owner"`
	Amount       float64 `json:"amount"`
	CouponRate   float64 `json:"couponRate"`
	MaturityDate string  `json:"maturityDate"`
	Description  string  `json:"description"`
}

func (s *Server) handleMint(c echo.Context) error {
	var req mintRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if req.Admin == "" || req.Owner == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "admin and owner are required"})
	}
	if req.Amount <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "amount must be positive"})
	}
	if req.MaturityDate == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "maturityDate is required"})
	}

	adminClient, err := s.clientForParty(req.Admin)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "admin: " + err.Error()})
	}
	ownerClient, err := s.clientForParty(req.Owner)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "owner: " + err.Error()})
	}
	if adminClient != ownerClient {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "admin and owner must be on the same participant"})
	}
	client := adminClient

	ctx, cancel := context.WithTimeout(c.Request().Context(), s.cfg.RequestTimeout)
	defer cancel()

	adminID, err := s.lookupPartyIdentifier(ctx, client, req.Admin)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "admin party not found: " + err.Error()})
	}
	ownerID, err := s.lookupPartyIdentifier(ctx, client, req.Owner)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "owner party not found: " + err.Error()})
	}

	offset, err := client.LedgerEnd(ctx)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "failed to query ledger end"})
	}

	resp, err := client.ActiveContracts(ctx, offset, TemplateSimpleTokenRules)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "failed to query factory"})
	}

	events := ExtractCreatedEvents(resp, TemplateSimpleTokenRules)
	if len(events) == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "no SimpleTokenRules factory found. Create one via GET or POST /api/v1/factory"})
	}

	factoryCID := events[0].ContractID

	choiceArg := map[string]any{
		"owner":        ownerID,
		"instrumentId": InstrumentID(adminID, "BOND"),
		"amount":       DamlDecimal(req.Amount),
		"couponRate":   DamlDecimal(req.CouponRate),
		"maturityDate": req.MaturityDate,
		"description":  req.Description,
	}

	cmdID := newCommandID("mint")
	submitReq := Command{
		ExerciseCommand: &ExerciseCommand{
			TemplateID:     TemplateSimpleTokenRules,
			Choice:         ChoiceMint,
			ContractID:     factoryCID,
			ChoiceArgument: choiceArg,
		},
	}

	offset, err = client.SubmitCommand(ctx, cmdID, submitReq, []string{adminID, ownerID})
	if err != nil {
		log.Printf("mint error: %v", err)
		return c.JSON(http.StatusBadGateway, map[string]string{"error": fmt.Sprintf("mint failed: %v", err)})
	}

	return c.JSON(http.StatusOK, map[string]any{
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

func (s *Server) handleTransfer(c echo.Context) error {
	var req transferRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if req.Sender == "" || req.Receiver == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "sender and receiver are required"})
	}
	if req.Amount <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "amount must be positive"})
	}

	senderClient, err := s.clientForParty(req.Sender)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "sender: " + err.Error()})
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), s.cfg.RequestTimeout)
	defer cancel()

	senderID, err := s.lookupPartyIdentifier(ctx, senderClient, req.Sender)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "sender party not found"})
	}

	receiverClient, err := s.clientForParty(req.Receiver)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "receiver: " + err.Error()})
	}
	receiverID, err := s.lookupPartyIdentifier(ctx, receiverClient, req.Receiver)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "receiver party not found"})
	}

	factoryClient, ok := s.clients["participant1"]
	if !ok {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "no participant1 client for factory lookup"})
	}

	factoryOffset, err := factoryClient.LedgerEnd(ctx)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "failed to query factory ledger end"})
	}
	factoryResp, err := factoryClient.ActiveContracts(ctx, factoryOffset, TemplateSimpleTokenRules)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "failed to query factory"})
	}
	factoryEvents := ExtractCreatedEvents(factoryResp, TemplateSimpleTokenRules)
	if len(factoryEvents) == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "no SimpleTokenRules factory found"})
	}
	factoryCID := factoryEvents[0].ContractID

	client := senderClient
	holdingsOffset, err := client.LedgerEnd(ctx)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "failed to query ledger end"})
	}
	holdingsResp, err := client.ActiveContracts(ctx, holdingsOffset, TemplateSimpleHolding)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "failed to query holdings"})
	}
	holdingsEvents := ExtractCreatedEvents(holdingsResp,
		TemplateSimpleHolding,
		TemplateLockedSimpleHolding,
	)

	var inputCIDs []string
	remaining := req.Amount

	if len(req.HoldingCids) > 0 {
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
		return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("insufficient holdings: need %.2f more", remaining)})
	}

	factoryAdmin := factoryEvents[0].GetStringField("admin")

	transferArg := map[string]any{
		"sender":           senderID,
		"receiver":         receiverID,
		"amount":           DamlDecimal(req.Amount),
		"instrumentId":     InstrumentID(factoryAdmin, "BOND"),
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
	submitReq := Command{
		ExerciseCommand: &ExerciseCommand{
			TemplateID:     TemplateTransferFactory,
			Choice:         ChoiceTransferFactoryTransfer,
			ContractID:     factoryCID,
			ChoiceArgument: choiceArg,
		},
	}

	offset, err := client.SubmitCommand(ctx, cmdID, submitReq, []string{senderID})
	if err != nil {
		log.Printf("transfer error: %v", err)
		return c.JSON(http.StatusBadGateway, map[string]string{"error": fmt.Sprintf("transfer failed: %v", err)})
	}

	return c.JSON(http.StatusOK, map[string]any{
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

func (s *Server) handleAcceptTransfer(c echo.Context) error {
	return s.handleTransferAction(c, ChoiceTransferInstructionAccept, "accept")
}

func (s *Server) handleRejectTransfer(c echo.Context) error {
	return s.handleTransferAction(c, ChoiceTransferInstructionReject, "reject")
}

func (s *Server) handleWithdrawTransfer(c echo.Context) error {
	return s.handleTransferAction(c, ChoiceTransferInstructionWithdraw, "withdraw")
}

func (s *Server) handleTransferAction(c echo.Context, choice, action string) error {
	var req transferActionRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if req.Party == "" || req.ContractID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "party and contractId are required"})
	}

	client, err := s.clientForParty(req.Party)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), s.cfg.RequestTimeout)
	defer cancel()

	partyID, err := s.lookupPartyIdentifier(ctx, client, req.Party)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "party not found"})
	}

	cmdID := newCommandID(action)
	submitReq := Command{
		ExerciseCommand: &ExerciseCommand{
			TemplateID:  TemplateTransferInstruction,
			Choice:      choice,
			ContractID:  req.ContractID,
			ChoiceArgument: map[string]any{
				"extraArgs": map[string]any{"context": map[string]any{"values": map[string]any{}}, "meta": map[string]any{"values": map[string]any{}}},
			},
		},
	}

	offset, err := client.SubmitCommand(ctx, cmdID, submitReq, []string{partyID})
	if err != nil {
		log.Printf("%s error: %v", action, err)
		return c.JSON(http.StatusBadGateway, map[string]string{"error": fmt.Sprintf("%s failed: %v", action, err)})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"status": action + "ed",
		"offset": offset,
	})
}

type burnRequest struct {
	Party      string `json:"party"`
	ContractID string `json:"contractId"`
	AsAdmin    bool   `json:"asAdmin,omitempty"`
}

func (s *Server) handleBurn(c echo.Context) error {
	var req burnRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if req.Party == "" || req.ContractID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "party and contractId are required"})
	}

	client, err := s.clientForParty(req.Party)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), s.cfg.RequestTimeout)
	defer cancel()

	offset, err := client.LedgerEnd(ctx)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "failed to query ledger end"})
	}
	contracts, err := client.ActiveContracts(ctx, offset, TemplateSimpleHolding)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "failed to query holdings"})
	}
	events := ExtractCreatedEvents(contracts, TemplateSimpleHolding)

	var adminID, ownerID string
	found := false
	for _, evt := range events {
		if evt.ContractID == req.ContractID {
			adminID = evt.GetStringField("admin")
			ownerID = evt.GetStringField("owner")
			found = true
			break
		}
	}
	if !found {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "holding contract not found"})
	}

	choice := ChoiceBurn
	if req.AsAdmin {
		choice = ChoiceBurnByAdmin
	}

	cmdID := newCommandID("burn")
	submitReq := Command{
		ExerciseCommand: &ExerciseCommand{
			TemplateID:     TemplateSimpleHolding,
			Choice:         choice,
			ContractID:     req.ContractID,
			ChoiceArgument: map[string]any{},
		},
	}

	offset, err = client.SubmitCommand(ctx, cmdID, submitReq, []string{adminID, ownerID})
	if err != nil {
		log.Printf("burn error: %v", err)
		return c.JSON(http.StatusBadGateway, map[string]string{"error": fmt.Sprintf("burn failed: %v", err)})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"status": "burned",
		"offset": offset,
	})
}

func (s *Server) handleSelfTransfer(c echo.Context) error {
	var req transferRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if req.Sender == "" || req.Receiver == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "sender and receiver are required"})
	}
	if req.Sender != req.Receiver {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "self-transfer requires sender == receiver"})
	}

	return s.handleTransfer(c)
}

func (s *Server) handleListTransferInstructions(c echo.Context) error {
	party := c.QueryParam("party")
	if party == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "party query parameter is required"})
	}

	client, err := s.clientForParty(party)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), s.cfg.RequestTimeout)
	defer cancel()

	partyID, err := s.lookupPartyIdentifier(ctx, client, party)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "party not found"})
	}

	offset, err := client.LedgerEnd(ctx)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "failed to query ledger end"})
	}

	resp, err := client.ActiveContracts(ctx, offset, TemplateSimpleTransferInstruction)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "failed to query transfer instructions"})
	}

	events := ExtractCreatedEvents(resp, TemplateSimpleTransferInstruction)

	var transfers []map[string]any
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

		var amount float64
		if amtRaw, ok := transferMap["amount"]; ok {
			switch v := amtRaw.(type) {
			case string:
				fmt.Sscanf(v, "%f", &amount)
			case float64:
				amount = v
			}
		}

		t := map[string]any{
			"contractId": evt.ContractID,
			"sender":     sender,
			"receiver":   receiver,
			"amount":     amount,
		}

		transfers = append(transfers, t)
	}

	if transfers == nil {
		transfers = []map[string]any{}
	}
	return c.JSON(http.StatusOK, transfers)
}

func (s *Server) handleFactory(c echo.Context) error {
	client, ok := s.clients["participant1"]
	if !ok {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "no participant1 client"})
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), s.cfg.RequestTimeout)
	defer cancel()

	offset, err := client.LedgerEnd(ctx)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "failed to query ledger end"})
	}

	resp, err := client.ActiveContracts(ctx, offset, TemplateSimpleTokenRules)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "failed to query factory"})
	}

	events := ExtractCreatedEvents(resp, TemplateSimpleTokenRules)
	if len(events) == 0 {
		var adminID string
		var lastErr error
		for i := 0; i < 10; i++ {
			adminID, lastErr = s.lookupPartyIdentifier(ctx, client, "admin")
			if lastErr == nil {
				break
			}
			select {
			case <-ctx.Done():
				return c.JSON(http.StatusGatewayTimeout, map[string]string{"error": "timeout waiting for bootstrap to complete"})
			case <-time.After(3 * time.Second):
			}
		}
		if lastErr != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "admin party not found on participant1 after retries. Bootstrap may not be complete."})
		}

		var observerIDs []string
		for _, p := range s.cfg.Participants {
			pc := s.clients[p.Name]
			if pc == nil {
				continue
			}
			allParties, err := pc.Parties(ctx)
			if err != nil {
				continue
			}
			for _, party := range allParties {
				if party.Party != adminID {
					observerIDs = append(observerIDs, party.Party)
				}
			}
		}
		if observerIDs == nil {
			observerIDs = []string{}
		}

		createArgs := map[string]any{
			"admin":                adminID,
			"supportedInstruments": []string{"BOND"},
			"observers":            observerIDs,
		}

		cmdID := newCommandID("create-factory")
		submitReq := Command{
			CreateCommand: &CreateCommand{
				TemplateID:      TemplateSimpleTokenRules,
				CreateArguments: createArgs,
			},
		}

		offset, err := client.SubmitCommand(ctx, cmdID, submitReq, []string{adminID})
		if err != nil {
			log.Printf("create factory error: %v", err)
			return c.JSON(http.StatusBadGateway, map[string]string{"error": fmt.Sprintf("create factory failed: %v", err)})
		}

		return c.JSON(http.StatusOK, map[string]any{
			"status":      "created",
			"offset":      offset,
			"admin":       adminID,
			"instruments": []string{"BOND"},
		})
	}

	factory := events[0]
	return c.JSON(http.StatusOK, map[string]any{
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
