package gateway

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/trace"

	"switch/internal/orchestrator/domain"
	"switch/internal/orchestrator/ports"
	"switch/internal/orchestrator/saga"
	"switch/pkg/iso20022"
	"switch/pkg/middleware"
	"switch/pkg/telemetry"
)

// PaymentRequest is the JSON body accepted by POST /payments.
// All ISO 20022 fields are optional; the gateway fills in defaults where
// the spec requires a value (UETR, ChargeBearer, SettlementDate).
type PaymentRequest struct {
	// Core fields (required)
	EndToEndID     string `json:"end_to_end_id"`
	DestinationBIC string `json:"destination_bic"`
	DestAccount    string `json:"dest_account"`
	Amount         int64  `json:"amount"`
	Currency       string `json:"currency"`
	// ISO 20022 enrichment (optional)
	UETR           string `json:"uetr,omitempty"`           // generated if absent
	InstructionID  string `json:"instruction_id,omitempty"` // generated if absent
	ChargeBearer   string `json:"charge_bearer,omitempty"`  // default: SLEV
	SettlementDate string `json:"settlement_date,omitempty"` // YYYY-MM-DD, default: today
	DebtorName     string `json:"debtor_name,omitempty"`
	CreditorName   string `json:"creditor_name,omitempty"`
	PurposeCode    string `json:"purpose_code,omitempty"`
	RemittanceInfo string `json:"remittance_info,omitempty"`
}

// PaymentResponse is the JSON body returned by POST /payments and GET /payments/{id}.
type PaymentResponse struct {
	ID             string               `json:"id"`
	UETR           string               `json:"uetr"`
	InstructionID  string               `json:"instruction_id"`
	EndToEndID     string               `json:"end_to_end_id"`
	Status         domain.PaymentStatus `json:"status"`
	ISOStatus      string               `json:"iso_status"` // pacs.002 TxSts code
	Amount         int64                `json:"amount"`
	Currency       string               `json:"currency"`
	DebtorName     string               `json:"debtor_name,omitempty"`
	CreditorName   string               `json:"creditor_name,omitempty"`
	PurposeCode    string               `json:"purpose_code,omitempty"`
	RemittanceInfo string               `json:"remittance_info,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
}

type ParticipantResolver interface {
	GetBankForParticipant(ctx context.Context, id string) (bic, account string, err error)
}

type CheckFn func(ctx context.Context) error

type Handler struct {
	repo     ports.PaymentRepository
	saga     *saga.Saga
	resolver ParticipantResolver
	pgpool   *pgxpool.Pool
	mu       sync.RWMutex
	checks   []struct {
		name string
		fn   CheckFn
	}
}

func NewHandler(repo ports.PaymentRepository, s *saga.Saga, resolver ParticipantResolver) *Handler {
	return &Handler{repo: repo, saga: s, resolver: resolver}
}

func (h *Handler) AddPool(pool *pgxpool.Pool) {
	h.pgpool = pool
}

func (h *Handler) AddHealthCheck(name string, fn CheckFn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks = append(h.checks, struct {
		name string
		fn   CheckFn
	}{name, fn})
}

func (h *Handler) Register(r chi.Router) {
	r.Get("/healthz", h.healthz)
	r.Post("/payments", h.SubmitPayment)
	r.Get("/payments/{id}", h.GetPayment)
}

func (h *Handler) healthz(w http.ResponseWriter, req *http.Request) {
	h.mu.RLock()
	checks := make([]struct {
		name string
		fn   CheckFn
	}, len(h.checks))
	copy(checks, h.checks)
	h.mu.RUnlock()

	if h.pgpool != nil {
		if err := h.pgpool.Ping(req.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy", "error": "database unreachable"})
			return
		}
	}
	for _, c := range checks {
		if err := c.fn(req.Context()); err != nil {
			slog.Warn("health check failed", "check", c.name, "error", err)
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy", "error": c.name + ": " + err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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

	var payment *domain.Payment
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "application/xml") {
		payment, err = h.parsePacs008(r, sourceBIC, sourceAccount)
	} else {
		payment, err = h.parseJSON(r, sourceBIC, sourceAccount)
	}
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	if telemetry.Tracer != nil {
		ctx, span := telemetry.Tracer.Start(r.Context(), "submit_payment",
			trace.WithAttributes(telemetry.SpanAttrs(payment.ID, payment.EndToEndID, string(payment.Status))...),
		)
		defer span.End()
		r = r.WithContext(ctx)
	}

	if err := h.repo.CreateWithEvent(r.Context(), payment); err != nil {
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
		"uetr", payment.UETR,
		"end_to_end_id", payment.EndToEndID,
		"status", payment.Status,
		"iso_status", iso20022.DomainStatusToISO(string(payment.Status)),
		"source", payment.SourceBIC,
		"dest", payment.DestinationBIC,
		"amount", payment.Amount,
	)

	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/xml") {
		h.writePacs002(w, payment, http.StatusCreated)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toResponse(payment))
}

func (h *Handler) GetPayment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p, err := h.repo.GetByEndToEndID(r.Context(), id)
	if err != nil || p == nil {
		slog.Warn("payment lookup failed", "id", id)
		http.Error(w, `{"error":"payment not found"}`, http.StatusNotFound)
		return
	}

	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/xml") {
		h.writePacs002(w, p, http.StatusOK)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toResponse(p))
}

// parseJSON decodes a JSON PaymentRequest and builds a domain.Payment.
func (h *Handler) parseJSON(r *http.Request, sourceBIC, sourceAccount string) (*domain.Payment, error) {
	var req PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, err
	}
	return buildPayment(req, sourceBIC, sourceAccount), nil
}

// parsePacs008 decodes a pacs.008 XML body and builds a domain.Payment.
// The SourceBIC/SourceAccount from the participant registry override whatever
// the message carries — the switch trusts its own participant table.
func (h *Handler) parsePacs008(r *http.Request, sourceBIC, sourceAccount string) (*domain.Payment, error) {
	var doc iso20022.Document
	if err := xml.NewDecoder(r.Body).Decode(&doc); err != nil {
		return nil, err
	}
	d, err := iso20022.FromPacs008(&doc)
	if err != nil {
		return nil, err
	}
	req := PaymentRequest{
		EndToEndID:     d.EndToEndID,
		DestinationBIC: d.DestBIC,
		DestAccount:    d.DestAccount,
		Amount:         d.Amount,
		Currency:       d.Currency,
		UETR:           d.UETR,
		InstructionID:  d.InstrID,
		ChargeBearer:   d.ChargeBearer,
		DebtorName:     d.DebtorName,
		CreditorName:   d.CreditorName,
		PurposeCode:    d.PurposeCode,
		RemittanceInfo: d.RemittanceInfo,
	}
	if !d.SettlementDate.IsZero() {
		req.SettlementDate = d.SettlementDate.Format("2006-01-02")
	}
	return buildPayment(req, sourceBIC, sourceAccount), nil
}

// writePacs002 serialises a domain.Payment as a pacs.002 XML status report.
func (h *Handler) writePacs002(w http.ResponseWriter, p *domain.Payment, code int) {
	doc := iso20022.ToPacs002("STS-"+p.ID, iso20022.PaymentData{
		ID:             p.ID,
		EndToEndID:     p.EndToEndID,
		UETR:           p.UETR,
		InstrID:        p.InstrID,
		SourceBIC:      p.SourceBIC,
		DestBIC:        p.DestinationBIC,
		Amount:         p.Amount,
		Currency:       p.Currency,
		ChargeBearer:   p.ChargeBearer,
		SettlementDate: p.SettlementDate,
		DebtorName:     p.DebtorName,
		CreditorName:   p.CreditorName,
		PurposeCode:    p.PurposeCode,
		RemittanceInfo: p.RemittanceInfo,
		Status:         string(p.Status),
		CreatedAt:      p.CreatedAt,
	})
	out, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		http.Error(w, `{"error":"failed to serialise pacs.002"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(code)
	w.Write([]byte(xml.Header))
	w.Write(out)
}

func buildPayment(req PaymentRequest, sourceBIC, sourceAccount string) *domain.Payment {
	uetr := req.UETR
	if uetr == "" {
		uetr = uuid.New().String()
	}
	id := uuid.New().String()
	instrID := req.InstructionID
	if instrID == "" {
		instrID = id
	}
	chargeBearer := req.ChargeBearer
	if chargeBearer == "" {
		chargeBearer = "SLEV"
	}
	var sttlmDate time.Time
	if req.SettlementDate != "" {
		if t, err := time.Parse("2006-01-02", req.SettlementDate); err == nil {
			sttlmDate = t
		}
	}
	if sttlmDate.IsZero() {
		sttlmDate = time.Now().UTC().Truncate(24 * time.Hour)
	}

	return &domain.Payment{
		ID:             id,
		EndToEndID:     req.EndToEndID,
		SourceBIC:      sourceBIC,
		DestinationBIC: req.DestinationBIC,
		SourceAccount:  sourceAccount,
		DestAccount:    req.DestAccount,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Status:         domain.StatusReceived,
		UETR:           uetr,
		InstrID:        instrID,
		ChargeBearer:   chargeBearer,
		SettlementDate: sttlmDate,
		DebtorName:     req.DebtorName,
		CreditorName:   req.CreditorName,
		PurposeCode:    req.PurposeCode,
		RemittanceInfo: req.RemittanceInfo,
	}
}

func toResponse(p *domain.Payment) PaymentResponse {
	return PaymentResponse{
		ID:             p.ID,
		UETR:           p.UETR,
		InstructionID:  p.InstrID,
		EndToEndID:     p.EndToEndID,
		Status:         p.Status,
		ISOStatus:      iso20022.DomainStatusToISO(string(p.Status)),
		Amount:         p.Amount,
		Currency:       p.Currency,
		DebtorName:     p.DebtorName,
		CreditorName:   p.CreditorName,
		PurposeCode:    p.PurposeCode,
		RemittanceInfo: p.RemittanceInfo,
		CreatedAt:      p.CreatedAt,
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
