package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go_backend_prueba/internal/config"
	"go_backend_prueba/internal/ledger"
)

type Server struct {
	cfg    config.Config
	ledger *ledger.Client
}

func NewServer(cfg config.Config, ledgerClient *ledger.Client) *Server {
	return &Server{
		cfg:    cfg,
		ledger: ledgerClient,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ledger-end", s.handleLedgerEnd)
	mux.HandleFunc("/contracts", s.handleContracts)
	mux.HandleFunc("/bonds", s.handleCreateBond)
	mux.HandleFunc("/settlements", s.handleSettlement)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleLedgerEnd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.RequestTimeout)
	defer cancel()

	offset, err := s.ledger.LedgerEnd(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"offset": offset})
}

func (s *Server) handleContracts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.RequestTimeout)
	defer cancel()

	offset, err := s.ledger.LedgerEnd(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	ctx, cancel = context.WithTimeout(r.Context(), s.cfg.RequestTimeout)
	defer cancel()

	contracts, err := s.ledger.ActiveContracts(ctx, offset)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, contracts)
}

type createBondRequest struct {
	CommandID  string                 `json:"commandId"`
	TemplateID string                 `json:"templateId"`
	Args       map[string]interface{} `json:"args"`
}

func (s *Server) handleCreateBond(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req createBondRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.CommandID == "" {
		req.CommandID = fmt.Sprintf("bond-%d", time.Now().UnixNano())
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.RequestTimeout)
	defer cancel()

	templateID := req.TemplateID
	if templateID == "" {
		templateID = s.cfg.TemplateBond
	}
	resp, err := s.ledger.SubmitCreate(ctx, req.CommandID, templateID, req.Args)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

type settlementRequest struct {
	CommandID  string                 `json:"commandId"`
	TemplateID string                 `json:"templateId"`
	ContractID string                 `json:"contractId"`
	Choice     string                 `json:"choice"`
	ChoiceArg  map[string]interface{} `json:"choiceArg"`
}

func (s *Server) handleSettlement(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req settlementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.CommandID == "" {
		req.CommandID = fmt.Sprintf("settle-%d", time.Now().UnixNano())
	}
	if req.TemplateID == "" {
		req.TemplateID = s.cfg.TemplateBond
	}
	if req.Choice == "" {
		req.Choice = s.cfg.ChoiceSettle
	}
	if req.ContractID == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("contractId required"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.RequestTimeout)
	defer cancel()

	resp, err := s.ledger.SubmitExercise(ctx, req.CommandID, req.TemplateID, req.Choice, req.ContractID, req.ChoiceArg)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
