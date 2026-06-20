package gateway

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"

	"switch/internal/orchestrator/domain"
	"switch/internal/orchestrator/ports"
	"switch/internal/orchestrator/saga"
	"switch/pkg/middleware"
	"switch/pkg/telemetry"
)

type PaymentRequest struct {
	EndToEndID     string `json:"end_to_end_id"`
	DestinationBIC string `json:"destination_bic"`
	DestAccount    string `json:"dest_account"`
	Amount         int64  `json:"amount"`
	Currency       string `json:"currency"`
}

type PaymentResponse struct {
	ID      string              `json:"id"`
	Status  domain.PaymentStatus `json:"status"`
}

type ParticipantResolver interface {
	GetBankForParticipant(ctx context.Context, id string) (bic, account string, err error)
}

type Handler struct {
	repo       ports.PaymentRepository
	saga       *saga.Saga
	resolver   ParticipantResolver
}

func NewHandler(repo ports.PaymentRepository, s *saga.Saga, resolver ParticipantResolver) *Handler {
	return &Handler{repo: repo, saga: s, resolver: resolver}
}

func (h *Handler) Register(r chi.Router) {
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	r.Post("/payments", h.SubmitPayment)
	r.Get("/payments/{id}", h.GetPayment)
}

func (h *Handler) SubmitPayment(w http.ResponseWriter, r *http.Request) {
	participantID, ok := r.Context().Value(middleware.ParticipantCtxKey).(string)
	if !ok {
		http.Error(w, `{"error":"participant identity required"}`, http.StatusUnauthorized)
		return
	}

	sourceBIC, sourceAccount, err := h.resolver.GetBankForParticipant(r.Context(), participantID)
	if err != nil {
		slog.Warn("unknown participant", "participant_id", participantID)
		http.Error(w, `{"error":"unknown participant"}`, http.StatusUnauthorized)
		return
	}

	var req PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	payment := &domain.Payment{
		ID:             uuid.New().String(),
		EndToEndID:     req.EndToEndID,
		SourceBIC:      sourceBIC,
		DestinationBIC: req.DestinationBIC,
		SourceAccount:  sourceAccount,
		DestAccount:    req.DestAccount,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Status:         domain.StatusReceived,
	}

	if telemetry.Tracer != nil {
		ctx, span := telemetry.Tracer.Start(r.Context(), "submit_payment",
			trace.WithAttributes(telemetry.SpanAttrs(payment.ID, payment.EndToEndID, string(payment.Status))...),
		)
		defer span.End()
		r = r.WithContext(ctx)
	}

	if err := h.repo.Create(r.Context(), payment); err != nil {
		slog.Error("create payment", "id", payment.ID, "error", err)
		http.Error(w, `{"error":"failed to create payment"}`, http.StatusInternalServerError)
		return
	}

	if err := h.saga.Run(r.Context(), payment); err != nil {
		slog.Error("saga failed", "id", payment.ID, "error", err)
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	slog.Info("payment submitted",
		"id", payment.ID,
		"end_to_end_id", payment.EndToEndID,
		"status", payment.Status,
		"source", payment.SourceBIC,
		"dest", payment.DestinationBIC,
		"amount", payment.Amount,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(PaymentResponse{ID: payment.ID, Status: payment.Status})
}

func (h *Handler) GetPayment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p, err := h.repo.GetByEndToEndID(r.Context(), id)
	if err != nil {
		slog.Warn("payment lookup failed", "id", id, "error", err)
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
