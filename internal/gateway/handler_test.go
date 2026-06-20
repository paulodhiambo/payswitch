package gateway_test

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"switch/internal/compliance"
	"switch/internal/gateway"
	"switch/internal/orchestrator/domain"
	"switch/internal/orchestrator/saga"
	"switch/internal/participant"
	"switch/pkg/iso20022"
	"switch/pkg/middleware"
)

type memRepo struct {
	mu       sync.Mutex
	payments map[string]*domain.Payment
}

func newMemRepo() *memRepo {
	return &memRepo{payments: make(map[string]*domain.Payment)}
}

func (r *memRepo) Create(_ context.Context, p *domain.Payment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.payments[p.ID] = p
	return nil
}

func (r *memRepo) UpdateStatus(_ context.Context, id string, status domain.PaymentStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.payments[id]; ok {
		p.Status = status
	}
	return nil
}

func (r *memRepo) GetByID(_ context.Context, id string) (*domain.Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.payments[id]
	if !ok {
		return nil, nil
	}
	return p, nil
}

func (r *memRepo) GetByEndToEndID(_ context.Context, e2eID string) (*domain.Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.payments {
		if p.EndToEndID == e2eID {
			return p, nil
		}
	}
	return nil, nil
}

func (r *memRepo) FindExpiredReservations(_ context.Context, before time.Time) ([]domain.Reservation, error) {
	return nil, nil
}

func (r *memRepo) MarkReserved(_ context.Context, id string, ttl time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.payments[id]; ok {
		p.Status = domain.StatusReserved
	}
	return nil
}

func (r *memRepo) CreateWithEvent(_ context.Context, p *domain.Payment) error {
	return r.Create(context.Background(), p)
}

func setupHandler() http.Handler {
	repo := newMemRepo()
	bank := saga.NewMockBankClient()
	bank.SetBalance("ACC-A", 1_000_00)
	bank.SetBalance("ACC-B", 0)
	complianceClient := compliance.New()

	reg := participant.NewRegistry()
	reg.Register(&participant.Participant{ID: "test-participant", BIC: "BANKUS33", Account: "ACC-A"})

	s := saga.New(
		&saga.ValidateStep{Repo: repo},
		&saga.ScreenStep{Client: complianceClient, Repo: repo},
		&saga.ReserveStep{Repo: repo, Bank: bank, TTL: saga.DefaultReservationTTL},
		&saga.CommitStep{Repo: repo, Bank: bank},
	)

	h := gateway.NewHandler(repo, s, reg)
	r := chi.NewRouter()
	h.Register(r)

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := context.WithValue(req.Context(), middleware.ParticipantCtxKey, "test-participant")
		r.ServeHTTP(w, req.WithContext(ctx))
	})
}

func TestSubmitPayment_Success(t *testing.T) {
	handler := setupHandler()

	body := map[string]any{
		"end_to_end_id":   "e2e-001",
		"destination_bic": "BANKDEFF",
		"dest_account":    "ACC-B",
		"amount":          500_00,
		"currency":        "USD",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	var resp gateway.PaymentResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	require.Equal(t, domain.StatusCommitted, resp.Status)
}

func TestSubmitPayment_InvalidAmount(t *testing.T) {
	handler := setupHandler()

	body := map[string]any{
		"end_to_end_id":   "e2e-002",
		"destination_bic": "BANKDEFF",
		"dest_account":    "ACC-B",
		"amount":          -100,
		"currency":        "USD",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSubmitPayment_MissingFields(t *testing.T) {
	handler := setupHandler()

	body := map[string]any{
		"end_to_end_id": "e2e-003",
		"amount":        100_00,
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetPayment_Found(t *testing.T) {
	handler := setupHandler()

	body := map[string]any{
		"end_to_end_id":   "e2e-get-001",
		"destination_bic": "BANKDEFF",
		"dest_account":    "ACC-B",
		"amount":          250_00,
		"currency":        "USD",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	getReq := httptest.NewRequest(http.MethodGet, "/payments/e2e-get-001", nil)
	getW := httptest.NewRecorder()
	handler.ServeHTTP(getW, getReq)

	require.Equal(t, http.StatusOK, getW.Code)

	var resp gateway.PaymentResponse
	err := json.NewDecoder(getW.Body).Decode(&resp)
	require.NoError(t, err)
	require.Equal(t, domain.StatusCommitted, resp.Status)
}

func TestGetPayment_NotFound(t *testing.T) {
	handler := setupHandler()

	req := httptest.NewRequest(http.MethodGet, "/payments/nonexistent", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestSubmitPayment_ISO20022Fields(t *testing.T) {
	handler := setupHandler()

	body := map[string]any{
		"end_to_end_id":    "e2e-iso-001",
		"destination_bic":  "BANKDEFF",
		"dest_account":     "ACC-B",
		"amount":           300_00,
		"currency":         "USD",
		"uetr":             "550e8400-e29b-41d4-a716-446655440000",
		"instruction_id":   "INSTR-001",
		"charge_bearer":    "SLEV",
		"settlement_date":  "2026-06-20",
		"debtor_name":      "Alice Smith",
		"creditor_name":    "Bob Jones",
		"purpose_code":     "SALA",
		"remittance_info":  "Invoice #1234",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	var resp gateway.PaymentResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.Equal(t, "550e8400-e29b-41d4-a716-446655440000", resp.UETR)
	require.Equal(t, "INSTR-001", resp.InstructionID)
	require.Equal(t, "Alice Smith", resp.DebtorName)
	require.Equal(t, "Bob Jones", resp.CreditorName)
	require.Equal(t, "SALA", resp.PurposeCode)
	require.Equal(t, "Invoice #1234", resp.RemittanceInfo)
	require.NotEmpty(t, resp.ISOStatus)
}

func TestSubmitPayment_ISOStatusField(t *testing.T) {
	handler := setupHandler()

	body := map[string]any{
		"end_to_end_id":   "e2e-iso-002",
		"destination_bic": "BANKDEFF",
		"dest_account":    "ACC-B",
		"amount":          100_00,
		"currency":        "USD",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	var resp gateway.PaymentResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.NotEmpty(t, resp.UETR) // auto-generated
	require.NotEmpty(t, resp.ISOStatus)
	require.Equal(t, "ACCP", resp.ISOStatus) // COMMITTED → ACCP
}

func TestSubmitPayment_Pacs008XML(t *testing.T) {
	handler := setupHandler()

	doc := iso20022.Document{
		FIToFICstmrCdtTrf: iso20022.FIToFICustomerCreditTransfer{
			GrpHdr: iso20022.GroupHeader{
				MsgID:         "MSG-XML-001",
				CreDtTm:       "2026-06-20T10:00:00Z",
				NbOfTxs:       "1",
				IntrBkSttlmDt: "2026-06-20",
				SttlmInf:      iso20022.SettlementInformation{SttlmMtd: "CLRG"},
			},
			CdtTrfTxInf: []iso20022.CreditTransferTransaction{
				{
					PmtID: iso20022.PaymentIdentification{
						InstrID:    "INSTR-XML-001",
						EndToEndID: "e2e-xml-001",
						TxID:       "TX-001",
						UETR:       "660e8400-e29b-41d4-a716-446655440000",
					},
					IntrBkSttlmAmt: iso20022.Amount{Value: 150.00, Ccy: "USD"},
					ChrgBr:         "SHAR",
					Dbtr:           iso20022.PartyIdentification{Nm: "XML Debtor"},
					DbtrAcct:       iso20022.AccountIdentification{Othr: &iso20022.GenericAccount{ID: "ACC-A"}},
					DbtrAgt:        iso20022.FinancialInstitution{FinInstnID: iso20022.FinancialInstitutionID{BICFI: "BANKUS33"}},
					CdtrAgt:        iso20022.FinancialInstitution{FinInstnID: iso20022.FinancialInstitutionID{BICFI: "BANKDEFF"}},
					Cdtr:           iso20022.PartyIdentification{Nm: "XML Creditor"},
					CdtrAcct:       iso20022.AccountIdentification{Othr: &iso20022.GenericAccount{ID: "ACC-B"}},
				},
			},
		},
	}
	xmlBytes, err := xml.Marshal(doc)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewReader(xmlBytes))
	req.Header.Set("Content-Type", "application/xml")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	var resp gateway.PaymentResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.Equal(t, "e2e-xml-001", resp.EndToEndID)
	require.Equal(t, "660e8400-e29b-41d4-a716-446655440000", resp.UETR)
	require.Equal(t, "XML Debtor", resp.DebtorName)
	require.Equal(t, "XML Creditor", resp.CreditorName)
	require.Equal(t, int64(150_00), resp.Amount)
}

func TestSubmitPayment_Pacs002XMLResponse(t *testing.T) {
	handler := setupHandler()

	body := map[string]any{
		"end_to_end_id":   "e2e-pacs002-001",
		"destination_bic": "BANKDEFF",
		"dest_account":    "ACC-B",
		"amount":          200_00,
		"currency":        "USD",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/xml")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "application/xml")

	body2 := w.Body.String()
	require.True(t, strings.Contains(body2, "FIToFIPmtStsRpt"), "expected pacs.002 element in response")
	require.True(t, strings.Contains(body2, "e2e-pacs002-001"), "expected EndToEndID in pacs.002")
}
