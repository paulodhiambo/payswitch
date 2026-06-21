package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
	LookupAccount(ctx context.Context, account string) (bic string, bankName string, err error)
	LookupBIC(ctx context.Context, bic string) (account string, bankName string, err error)
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
	r.Post("/payments/lookup", h.AccountLookup)
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
		slog.Warn("missing participant identity in request context")
		ct := r.Header.Get("Content-Type")
		if strings.Contains(ct, "application/xml") {
			h.writePacs002BadRequestError(w, "participant identity required", http.StatusUnauthorized)
		} else {
			http.Error(w, `{"error":"participant identity required"}`, http.StatusUnauthorized)
		}
		return
	}

	sourceBIC, sourceAccount, err := h.resolver.GetBankForParticipant(r.Context(), participantID)
	if err != nil {
		slog.Warn("unknown participant",
			"participant_id", participantID,
		)
		ct := r.Header.Get("Content-Type")
		if strings.Contains(ct, "application/xml") {
			h.writePacs002BadRequestError(w, "unknown participant", http.StatusUnauthorized)
		} else {
			http.Error(w, `{"error":"unknown participant"}`, http.StatusUnauthorized)
		}
		return
	}
	slog.Debug("participant resolved",
		"participant_id", participantID,
		"source_bic", sourceBIC,
		"source_account", sourceAccount,
	)

	var payment *domain.Payment
	ct := r.Header.Get("Content-Type")
	bodySize := r.Header.Get("Content-Length")
	slog.Debug("incoming payment request",
		"participant_id", participantID,
		"content_type", ct,
		"content_length", bodySize,
		"method", r.Method,
		"path", r.URL.Path,
	)

	if strings.Contains(ct, "application/xml") {
		payment, err = h.parsePacs008(w, r, sourceBIC, sourceAccount)
	} else {
		payment, err = h.parseJSON(w, r, sourceBIC, sourceAccount)
	}
	if err != nil {
		slog.Warn("payment parse failed",
			"content_type", ct,
			"error", err,
		)
		if strings.Contains(ct, "application/xml") {
			h.writePacs002BadRequestError(w, err.Error(), http.StatusBadRequest)
		} else {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		}
		return
	}

	slog.Debug("payment parsed successfully",
		"id", payment.ID,
		"end_to_end_id", payment.EndToEndID,
		"uetr", payment.UETR,
		"instruction_id", payment.InstrID,
		"source_bic", payment.SourceBIC,
		"dest_bic", payment.DestinationBIC,
		"amount", payment.Amount,
		"currency", payment.Currency,
		"debtor", payment.DebtorName,
		"creditor", payment.CreditorName,
		"purpose", payment.PurposeCode,
		"settlement_date", payment.SettlementDate.Format("2006-01-02"),
		"charge_bearer", payment.ChargeBearer,
	)

	if telemetry.Tracer != nil {
		ctx, span := telemetry.Tracer.Start(r.Context(), "submit_payment",
			trace.WithAttributes(telemetry.SpanAttrs(payment.ID, payment.EndToEndID, string(payment.Status))...),
		)
		defer span.End()
		r = r.WithContext(ctx)
	}

	if err := h.repo.CreateWithEvent(r.Context(), payment); err != nil {
		slog.Error("failed to persist payment",
			"id", payment.ID,
			"error", err,
		)
		status := http.StatusInternalServerError
		message := err.Error()
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			status = http.StatusConflict
			message = "a payment with this end-to-end ID already exists"
		}
		if strings.Contains(ct, "application/xml") {
			h.writePacs002BadRequestError(w, message, status)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(map[string]string{"error": message})
		}
		return
	}

	slog.Info("payment accepted for async processing",
		"id", payment.ID,
		"uetr", payment.UETR,
		"end_to_end_id", payment.EndToEndID,
		"source", payment.SourceBIC,
		"dest", payment.DestinationBIC,
		"amount", payment.Amount,
		"currency", payment.Currency,
		"debtor", payment.DebtorName,
		"creditor", payment.CreditorName,
	)

	// Return 202 Accepted immediately — the saga runs in a background
	// goroutine and delivers the result via a pacs.002 callback.
	// If the original request was XML, acknowledge with a pacs.002 / RCVD;
	// otherwise return the usual JSON ack.
	if strings.Contains(ct, "application/xml") {
		h.writePacs002(w, payment, http.StatusAccepted)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{
			"id":         payment.ID,
			"status":     "RECEIVED",
			"iso_status": "RCVD",
		})
	}

	go h.processPaymentAsync(payment)
}

func (h *Handler) processPaymentAsync(payment *domain.Payment) {
	// Use a detached context so the saga is not cancelled when the HTTP
	// client disconnects. A 60-second timeout prevents goroutine leaks.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	slog.Debug("starting async saga processing",
		"id", payment.ID,
		"end_to_end_id", payment.EndToEndID,
		"source_bic", payment.SourceBIC,
		"dest_bic", payment.DestinationBIC,
		"amount", payment.Amount,
		"currency", payment.Currency,
		"initial_status", payment.Status,
	)

	if err := h.saga.Run(ctx, payment); err != nil {
		slog.Error("async saga failed",
			"id", payment.ID,
			"end_to_end_id", payment.EndToEndID,
			"error", err,
			"final_status", payment.Status,
		)
		if h.pgpool != nil {
			_, dbErr := h.pgpool.Exec(ctx,
				`UPDATE transaction_view SET abort_reason = $1 WHERE payment_id = $2`,
				err.Error(), payment.ID)
			if dbErr != nil {
				slog.Error("failed to update transaction_view abort_reason", "error", dbErr)
			}
		}
	} else {
		slog.Info("async saga completed successfully",
			"id", payment.ID,
			"end_to_end_id", payment.EndToEndID,
			"final_status", payment.Status,
		)
	}

	if h.pgpool == nil {
		slog.Warn("no database pool configured, skipping callback lookup",
			"id", payment.ID, "bic", payment.SourceBIC)
		return
	}

	callbackURL, err := h.getCallbackURL(ctx, payment.SourceBIC)
	if err != nil {
		slog.Error("failed to resolve callback URL from database",
			"id", payment.ID,
			"bic", payment.SourceBIC,
			"error", err,
		)
		return
	}
	if callbackURL == "" {
		slog.Debug("no callback URL configured for bank, skipping callback",
			"id", payment.ID,
			"bic", payment.SourceBIC,
		)
		return
	}

	slog.Debug("resolved callback URL",
		"id", payment.ID,
		"bic", payment.SourceBIC,
		"callback_url", callbackURL,
	)

	h.sendCallback(ctx, callbackURL, payment)
}

func (h *Handler) getCallbackURL(ctx context.Context, bic string) (string, error) {
	var url string
	err := h.pgpool.QueryRow(ctx,
		`SELECT COALESCE(callback_url, '') FROM bank WHERE bic = $1`, bic,
	).Scan(&url)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return url, err
}

func (h *Handler) sendCallback(ctx context.Context, callbackURL string, p *domain.Payment) {
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
	body, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		slog.Error("failed to marshal pacs.002 for callback",
			"id", p.ID,
			"error", err,
		)
		return
	}

	xmlBody := []byte(xml.Header)
	xmlBody = append(xmlBody, body...)

	slog.Debug("sending callback",
		"id", p.ID,
		"url", callbackURL,
		"status", p.Status,
		"iso_status", iso20022.DomainStatusToISO(string(p.Status)),
		"body_size", len(xmlBody),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, callbackURL, bytes.NewReader(xmlBody))
	if err != nil {
		slog.Error("failed to create callback HTTP request",
			"id", p.ID,
			"url", callbackURL,
			"error", err,
		)
		return
	}
	req.Header.Set("Content-Type", "application/xml")

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	elapsed := time.Since(start)

	if err != nil {
		slog.Error("callback HTTP request failed",
			"id", p.ID,
			"url", callbackURL,
			"error", err,
			"elapsed_ms", elapsed.Milliseconds(),
		)
		return
	}
	resp.Body.Close()

	slog.Info("callback delivered",
		"id", p.ID,
		"url", callbackURL,
		"status", resp.StatusCode,
		"elapsed_ms", elapsed.Milliseconds(),
	)

	if resp.StatusCode >= 300 {
		slog.Warn("callback returned non-success status code",
			"id", p.ID,
			"url", callbackURL,
			"status", resp.StatusCode,
			"elapsed_ms", elapsed.Milliseconds(),
		)
		return
	}
}

func (h *Handler) GetPayment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p, err := h.repo.GetByID(r.Context(), id)
	if err != nil || p == nil {
		slog.Warn("payment lookup failed",
			"id", id,
			"error", err,
		)
		http.Error(w, `{"error":"payment not found"}`, http.StatusNotFound)
		return
	}

	slog.Debug("payment retrieved",
		"id", p.ID,
		"status", p.Status,
		"iso_status", iso20022.DomainStatusToISO(string(p.Status)),
		"end_to_end_id", p.EndToEndID,
		"amount", p.Amount,
		"currency", p.Currency,
	)

	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/xml") {
		h.writePacs002(w, p, http.StatusOK)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toResponse(p))
}

// AccountLookupResponse is the JSON response for account lookups.
type AccountLookupResponse struct {
	AccountNumber string `json:"account_number"`
	BankName      string `json:"bank_name"`
	BIC           string `json:"bic"`
	Currency      string `json:"currency"`
}

// AccountLookup handles camt.003 account lookup requests.
// Accepts JSON ({"account_number": "..."}) or camt.003 XML.
func (h *Handler) AccountLookup(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB

	ct := r.Header.Get("Content-Type")
	isXML := strings.Contains(ct, "application/xml")
	var account string

	slog.Debug("account lookup request",
		"content_type", ct,
		"method", r.Method,
		"path", r.URL.Path,
		"is_xml", isXML,
	)

	if isXML {
		var doc iso20022.DocumentCamt003
		if err := xml.NewDecoder(r.Body).Decode(&doc); err != nil {
			slog.Warn("failed to parse camt.003 XML",
				"error", err,
			)
			h.writeCamt004Error(w, "", "invalid camt.003 XML", http.StatusBadRequest)
			return
		}
		crit := doc.GetAcct.AcctQryDef.AcctCrit.NewCrit
		if len(crit) == 0 || crit[0].SchCrit.AcctId.ID == nil || crit[0].SchCrit.AcctId.ID.Othr == nil {
			slog.Warn("camt.003 missing account identifier")
			h.writeCamt004Error(w, "", "account identifier not found in camt.003", http.StatusBadRequest)
			return
		}
		account = crit[0].SchCrit.AcctId.ID.Othr.ID
		slog.Debug("parsed camt.003 request",
			"account", account,
			"msg_id", doc.GetAcct.MsgHdr.MsgID,
		)
	} else {
		var req struct {
			AccountNumber string `json:"account_number"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.Warn("failed to parse JSON lookup",
				"error", err,
			)
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}
		account = req.AccountNumber
		slog.Debug("parsed JSON lookup request",
			"account", account,
		)
	}

	if account == "" {
		slog.Warn("empty account number in lookup request",
			"content_type", ct,
		)
		if isXML {
			h.writeCamt004Error(w, "", "account_number is required", http.StatusBadRequest)
		} else {
			http.Error(w, `{"error":"account_number is required"}`, http.StatusBadRequest)
		}
		return
	}

	bic, bankName, err := h.resolver.LookupAccount(r.Context(), account)
	if err != nil {
		slog.Warn("account lookup failed",
			"account", account,
			"error", err,
		)
		if isXML {
			h.writeCamt004Error(w, account, "account not found", http.StatusNotFound)
		} else {
			http.Error(w, `{"error":"account not found"}`, http.StatusNotFound)
		}
		return
	}

	slog.Debug("account resolved",
		"account", account,
		"bic", bic,
		"bank_name", bankName,
	)

	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/xml") {
		h.writeCamt004(w, account, bankName, bic)
		return
	}

	writeJSON(w, http.StatusOK, AccountLookupResponse{
		AccountNumber: account,
		BankName:      bankName,
		BIC:           bic,
		Currency:      "USD",
	})
}

// writeCamt004 writes a camt.004 account report XML response.
func (h *Handler) writeCamt004(w http.ResponseWriter, accountNumber, bankName, bic string) {
	doc := iso20022.DocumentCamt003Response{
		XMLNS: iso20022.NamespaceCamt003,
		AcctRpt: iso20022.AccountReport{
			MsgHdr: iso20022.MessageHeader{
				MsgID:   "ACCT-" + accountNumber,
				CreDtTm: time.Now().UTC().Format(time.RFC3339),
			},
			RptOrErr: iso20022.ReportOrError{
				AcctRpt: []iso20022.ReportedAccount{
					{
						AcctId: iso20022.AccountIdentification{ID: &iso20022.AccountID{Othr: &iso20022.GenericAccount{ID: accountNumber}}},
						Nm:     bankName,
						Ccy:    "USD",
						LglNm:  bic,
					},
				},
			},
		},
	}
	out, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		http.Error(w, `{"error":"failed to serialise camt.004"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(xml.Header))
	w.Write(out)
}

// writeCamt004Error sends a camt.004 with an OperationalError for
// account-lookup failures on XML camt.003 requests.
func (h *Handler) writeCamt004Error(w http.ResponseWriter, accountNumber, reason string, code int) {
	doc := iso20022.DocumentCamt003Response{
		XMLNS: iso20022.NamespaceCamt003,
		AcctRpt: iso20022.AccountReport{
			MsgHdr: iso20022.MessageHeader{
				MsgID:   "ERR-" + accountNumber,
				CreDtTm: time.Now().UTC().Format(time.RFC3339),
			},
			RptOrErr: iso20022.ReportOrError{
				OprlErr: &iso20022.OperationalError{
					Prtry: iso20022.ProprietaryError{
						Cd:   iso20022.ReasonNotFound,
						Issr: "SWITCH",
					},
					Desc: reason,
				},
			},
		},
	}
	out, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		http.Error(w, `{"error":"failed to serialise camt.004 error"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(code)
	w.Write([]byte(xml.Header))
	w.Write(out)
}

// parseJSON decodes a JSON PaymentRequest and builds a domain.Payment.
func (h *Handler) parseJSON(w http.ResponseWriter, r *http.Request, sourceBIC, sourceAccount string) (*domain.Payment, error) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
	var req PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, err
	}
	return buildPayment(req, sourceBIC, sourceAccount), nil
}

// parsePacs008 decodes a pacs.008 XML body and builds a domain.Payment.
// The SourceBIC/SourceAccount from the participant registry override whatever
// the message carries — the switch trusts its own participant table.
func (h *Handler) parsePacs008(w http.ResponseWriter, r *http.Request, sourceBIC, sourceAccount string) (*domain.Payment, error) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
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

// writePacs002BadRequestError sends a minimal pacs.002 with RJCT status
// for errors on pacs.008 XML requests.
func (h *Handler) writePacs002BadRequestError(w http.ResponseWriter, errMsg string, code int) {
	doc := &iso20022.DocumentPacs002{
		XMLNS: iso20022.NamespacePacs002,
		FIToFIPmtStsRpt: iso20022.FIToFIPaymentStatusReport{
			GrpHdr: iso20022.Pacs002GroupHeader{
				MsgID:   "ERR-" + uuid.New().String(),
				CreDtTm: time.Now().UTC().Format(time.RFC3339),
			},
			TxInfAndSts: []iso20022.TransactionStatus{
				{
					TxSts: iso20022.StatusRejected,
					StsRsnInf: &iso20022.StatusReason{
						Rsn:      iso20022.StatusReasonCode{Cd: iso20022.ReasonNoInstruction},
						AddtlInf: errMsg,
					},
				},
			},
		},
	}
	out, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		http.Error(w, `{"error":"failed to serialise pacs.002 error"}`, code)
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
