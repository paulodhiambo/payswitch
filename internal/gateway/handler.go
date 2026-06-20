package gateway

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"switch/internal/orchestrator/domain"
	"switch/internal/orchestrator/ports"
	"switch/internal/orchestrator/saga"
)

type PaymentRequest struct {
	EndToEndID     string `json:"end_to_end_id"`
	SourceBIC      string `json:"source_bic"`
	DestinationBIC string `json:"destination_bic"`
	SourceAccount  string `json:"source_account"`
	DestAccount    string `json:"dest_account"`
	Amount         int64  `json:"amount"`
	Currency       string `json:"currency"`
}

type PaymentResponse struct {
	ID     string              `json:"id"`
	Status domain.PaymentStatus `json:"status"`
}

type Handler struct {
	repo ports.PaymentRepository
	saga *saga.Saga
}

func NewHandler(repo ports.PaymentRepository, s *saga.Saga) *Handler {
	return &Handler{repo: repo, saga: s}
}

func (h *Handler) Register(r chi.Router) {
	r.Post("/payments", h.SubmitPayment)
	r.Get("/payments/{id}", h.GetPayment)
}

func (h *Handler) SubmitPayment(w http.ResponseWriter, r *http.Request) {
	var req PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	payment := &domain.Payment{
		ID:             uuid.New().String(),
		EndToEndID:     req.EndToEndID,
		SourceBIC:      req.SourceBIC,
		DestinationBIC: req.DestinationBIC,
		SourceAccount:  req.SourceAccount,
		DestAccount:    req.DestAccount,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Status:         domain.StatusReceived,
	}

	if err := h.repo.Create(r.Context(), payment); err != nil {
		log.Printf("create payment: %v", err)
		http.Error(w, `{"error":"failed to create payment"}`, http.StatusInternalServerError)
		return
	}

	if err := h.saga.Run(r.Context(), payment); err != nil {
		log.Printf("saga failed for %s: %v", payment.ID, err)
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(PaymentResponse{ID: payment.ID, Status: payment.Status})
}

func (h *Handler) GetPayment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p, err := h.repo.GetByEndToEndID(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"payment not found"}`, http.StatusNotFound)
		return
	}
	if p == nil {
		http.Error(w, `{"error":"payment not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(PaymentResponse{ID: p.ID, Status: p.Status})
}
