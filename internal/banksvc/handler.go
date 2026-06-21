package banksvc

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
)

type apiRequest struct {
	Account string `json:"account"`
	Amount  int64  `json:"amount"`
}

type apiResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Balance int64  `json:"balance,omitempty"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func (s *Server) handleReserve(w http.ResponseWriter, r *http.Request) {
	var req apiRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{Status: "error", Message: err.Error()})
		return
	}
	if err := s.bank.Reserve(req.Account, req.Amount); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{Status: "error", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "ok"})
}

func (s *Server) handleRelease(w http.ResponseWriter, r *http.Request) {
	var req apiRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{Status: "error", Message: err.Error()})
		return
	}
	if err := s.bank.ReleaseReservation(req.Account, req.Amount); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{Status: "error", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "ok"})
}

func (s *Server) handleCredit(w http.ResponseWriter, r *http.Request) {
	var req apiRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{Status: "error", Message: err.Error()})
		return
	}
	if err := s.bank.Credit(req.Account, req.Amount); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{Status: "error", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "ok"})
}

func (s *Server) handleReverseCredit(w http.ResponseWriter, r *http.Request) {
	var req apiRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{Status: "error", Message: err.Error()})
		return
	}
	if err := s.bank.ReverseCredit(req.Account, req.Amount); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{Status: "error", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "ok"})
}

func (s *Server) handleGetAccount(w http.ResponseWriter, r *http.Request) {
	number := r.PathValue("number")
	if number == "" {
		writeJSON(w, http.StatusBadRequest, apiResponse{Status: "error", Message: "account number required"})
		return
	}
	acct, err := s.bank.GetAccount(number)
	if err != nil {
		writeJSON(w, http.StatusNotFound, apiResponse{Status: "error", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, acct)
}

func (s *Server) handleListAccounts(w http.ResponseWriter, _ *http.Request) {
	accounts := s.bank.ListAccounts()
	writeJSON(w, http.StatusOK, accounts)
}

func (s *Server) handleCreateAccount(w http.ResponseWriter, r *http.Request) {
	var acct Account
	if err := json.NewDecoder(r.Body).Decode(&acct); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{Status: "error", Message: err.Error()})
		return
	}
	if err := s.bank.AddAccount(&acct); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{Status: "error", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, acct)
}

func (s *Server) handlePacs002Callback(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("read callback body", "error", err)
		writeJSON(w, http.StatusBadRequest, apiResponse{Status: "error", Message: err.Error()})
		return
	}
	slog.Info("received pacs.002 callback",
		"content_type", r.Header.Get("Content-Type"),
		"body_size", len(body),
	)
	writeJSON(w, http.StatusOK, apiResponse{Status: "ok", Message: "callback received"})
}

func (s *Server) handleSetBalance(w http.ResponseWriter, r *http.Request) {
	number := r.PathValue("number")
	if number == "" {
		writeJSON(w, http.StatusBadRequest, apiResponse{Status: "error", Message: "account number required"})
		return
	}
	var req struct {
		Balance int64 `json:"balance"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{Status: "error", Message: err.Error()})
		return
	}
	if err := s.bank.SetBalance(number, req.Balance); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{Status: "error", Message: err.Error()})
		return
	}
	acct, _ := s.bank.GetAccount(number)
	writeJSON(w, http.StatusOK, acct)
}
