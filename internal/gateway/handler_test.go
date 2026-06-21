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
	cp := *p
	r.payments[p.ID] = &cp
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
	cp := *p
	return &cp, nil
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

func (r *memRepo) UpdateRoute(_ context.Context, id string, fee int64, estimatedMs int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.payments[id]; ok {
		p.RouteFee = fee
		p.RouteEstimatedMs = estimatedMs
		p.Status = domain.StatusRouted
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

func pollPaymentStatus(t *testing.T, handler http.Handler, id string, expected domain.PaymentStatus) {
	t.Helper()
	for i := 0; i < 50; i++ {
		req := httptest.NewRequest(http.MethodGet, "/payments/"+id, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code == http.StatusOK {
			var resp gateway.PaymentResponse
			if json.NewDecoder(w.Body).Decode(&resp) == nil && resp.Status == expected {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("payment %s did not reach status %s", id, expected)
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

	require.Equal(t, http.StatusAccepted, w.Code)

	var ack struct {
		ID        string `json:"id"`
		Status    string `json:"status"`
		ISOStatus string `json:"iso_status"`
	}
	err := json.NewDecoder(w.Body).Decode(&ack)
	require.NoError(t, err)
	require.Equal(t, "RECEIVED", ack.Status)
	require.Equal(t, "RCVD", ack.ISOStatus)
	require.NotEmpty(t, ack.ID)

	pollPaymentStatus(t, handler, ack.ID, domain.StatusCommitted)
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

	require.Equal(t, http.StatusAccepted, w.Code)

	var ack struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&ack))
	require.NotEmpty(t, ack.ID)
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

	require.Equal(t, http.StatusAccepted, w.Code)

	var ack struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&ack))
	require.NotEmpty(t, ack.ID)
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
	require.Equal(t, http.StatusAccepted, w.Code)

	var ack struct {
		ID string `json:"id"`
	}
	json.NewDecoder(w.Body).Decode(&ack)

	pollPaymentStatus(t, handler, ack.ID, domain.StatusCommitted)

	getReq := httptest.NewRequest(http.MethodGet, "/payments/"+ack.ID, nil)
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

	require.Equal(t, http.StatusAccepted, w.Code)

	var ack struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&ack))
	require.NotEmpty(t, ack.ID)

	pollPaymentStatus(t, handler, ack.ID, domain.StatusCommitted)

	getReq := httptest.NewRequest(http.MethodGet, "/payments/"+ack.ID, nil)
	getW := httptest.NewRecorder()
	handler.ServeHTTP(getW, getReq)
	require.Equal(t, http.StatusOK, getW.Code)

	var resp gateway.PaymentResponse
	require.NoError(t, json.NewDecoder(getW.Body).Decode(&resp))
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

	require.Equal(t, http.StatusAccepted, w.Code)

	var ack struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&ack))
	require.NotEmpty(t, ack.ID)

	pollPaymentStatus(t, handler, ack.ID, domain.StatusCommitted)

	getReq := httptest.NewRequest(http.MethodGet, "/payments/"+ack.ID, nil)
	getW := httptest.NewRecorder()
	handler.ServeHTTP(getW, getReq)
	require.Equal(t, http.StatusOK, getW.Code)

	var resp gateway.PaymentResponse
	require.NoError(t, json.NewDecoder(getW.Body).Decode(&resp))
	require.NotEmpty(t, resp.UETR)
	require.NotEmpty(t, resp.ISOStatus)
	require.Equal(t, "ACCP", resp.ISOStatus)
}

func TestSubmitPayment_Pacs008XML(t *testing.T) {
	handler := setupHandler()

	doc := iso20022.Document{
		XMLNS: iso20022.NamespacePacs008,
		FIToFICstmrCdtTrf: iso20022.FIToFICustomerCreditTransfer{
			GrpHdr: iso20022.GroupHeader{
				MsgID:         "MSG-XML-001",
				CreDtTm:       "2026-06-20T10:00:00Z",
				NbOfTxs:       "1",
				IntrBkSttlmDt: "2026-06-20",
				SttlmInf:      &iso20022.SettlementInformation{SttlmMtd: "CLRG"},
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
					DbtrAcct:       iso20022.AccountIdentification{ID: &iso20022.AccountID{Othr: &iso20022.GenericAccount{ID: "ACC-A"}}},
					DbtrAgt:        iso20022.FinancialInstitution{FinInstnID: iso20022.FinancialInstitutionID{Othr: &iso20022.GenericAccount{ID: "BANKUS33"}}},
					CdtrAgt:        iso20022.FinancialInstitution{FinInstnID: iso20022.FinancialInstitutionID{Othr: &iso20022.GenericAccount{ID: "BANKDEFF"}}},
					Cdtr:           iso20022.PartyIdentification{Nm: "XML Creditor"},
					CdtrAcct:       iso20022.AccountIdentification{ID: &iso20022.AccountID{Othr: &iso20022.GenericAccount{ID: "ACC-B"}}},
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

	require.Equal(t, http.StatusAccepted, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "application/xml")

	var respDoc iso20022.DocumentPacs002
	require.NoError(t, xml.NewDecoder(w.Body).Decode(&respDoc))
	require.NotNil(t, respDoc.FIToFIPmtStsRpt)
	require.Len(t, respDoc.FIToFIPmtStsRpt.TxInfAndSts, 1)
	txInf := respDoc.FIToFIPmtStsRpt.TxInfAndSts[0]
	require.Equal(t, iso20022.StatusReceived, txInf.TxSts)
	paymentID := txInf.OrgnlTxID
	require.NotEmpty(t, paymentID)

	pollPaymentStatus(t, handler, paymentID, domain.StatusCommitted)

	getReq := httptest.NewRequest(http.MethodGet, "/payments/"+paymentID, nil)
	getW := httptest.NewRecorder()
	handler.ServeHTTP(getW, getReq)
	require.Equal(t, http.StatusOK, getW.Code)

	var resp gateway.PaymentResponse
	require.NoError(t, json.NewDecoder(getW.Body).Decode(&resp))
	require.Equal(t, "e2e-xml-001", resp.EndToEndID)
	require.Equal(t, "660e8400-e29b-41d4-a716-446655440000", resp.UETR)
	require.Equal(t, "XML Debtor", resp.DebtorName)
	require.Equal(t, "XML Creditor", resp.CreditorName)
	require.Equal(t, int64(150_00), resp.Amount)
}

func TestGetPayment_Pacs002XMLResponse(t *testing.T) {
	handler := setupHandler()

	body := map[string]any{
		"end_to_end_id":   "e2e-pacs002-g001",
		"destination_bic": "BANKDEFF",
		"dest_account":    "ACC-B",
		"amount":          200_00,
		"currency":        "USD",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	require.Equal(t, http.StatusAccepted, w.Code)

	var ack struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&ack))

	pollPaymentStatus(t, handler, ack.ID, domain.StatusCommitted)

	getReq := httptest.NewRequest(http.MethodGet, "/payments/"+ack.ID, nil)
	getReq.Header.Set("Accept", "application/xml")
	getW := httptest.NewRecorder()
	handler.ServeHTTP(getW, getReq)

	require.Equal(t, http.StatusOK, getW.Code)
	require.Contains(t, getW.Header().Get("Content-Type"), "application/xml")

	body2 := getW.Body.String()
	require.True(t, strings.Contains(body2, "FIToFIPmtStsRpt"), "expected pacs.002 element in response")
	require.True(t, strings.Contains(body2, "e2e-pacs002-g001"), "expected EndToEndID in pacs.002")
}

func TestSubmitPayment_Pacs008XML_InvalidXML(t *testing.T) {
	handler := setupHandler()

	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewReader([]byte(`not xml`)))
	req.Header.Set("Content-Type", "application/xml")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "application/xml")
	body := w.Body.String()
	require.Contains(t, body, "FIToFIPmtStsRpt")
	require.Contains(t, body, iso20022.StatusRejected)
}

func TestSubmitPayment_JSON_Error_WhenNotXML(t *testing.T) {
	handler := setupHandler()

	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewReader([]byte(`not json either`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAccountLookup_XML_NotFound(t *testing.T) {
	handler := setupHandler()

	doc := iso20022.DocumentCamt003{
		XMLNS: iso20022.NamespaceCamt003,
		GetAcct: iso20022.GetAccount{
			MsgHdr: iso20022.MessageHeader{
				MsgID:   "CAML-ERR",
				CreDtTm: "2026-06-20T10:00:00Z",
			},
			AcctQryDef: iso20022.AccountQueryDefinition{
				AcctCrit: iso20022.AccountCriteria{
					NewCrit: []iso20022.NewCriteria{
						{
							SchCrit: iso20022.SearchCriteria{
								AcctId: iso20022.AccountIdentification{
									ID: &iso20022.AccountID{
										Othr: &iso20022.GenericAccount{ID: "NONEXISTENT"},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	xmlBytes, err := xml.Marshal(doc)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/payments/lookup", bytes.NewReader(xmlBytes))
	req.Header.Set("Content-Type", "application/xml")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "application/xml")
	body := w.Body.String()
	require.Contains(t, body, "AcctRpt")
}

func TestAccountLookup_XML_InvalidXML(t *testing.T) {
	handler := setupHandler()

	req := httptest.NewRequest(http.MethodPost, "/payments/lookup", bytes.NewReader([]byte(`not xml`)))
	req.Header.Set("Content-Type", "application/xml")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "application/xml")
	body := w.Body.String()
	require.Contains(t, body, "AcctRpt")
}

func TestAccountLookup_XML_MissingAccountIdentifier(t *testing.T) {
	handler := setupHandler()

	doc := iso20022.DocumentCamt003{
		XMLNS: iso20022.NamespaceCamt003,
		GetAcct: iso20022.GetAccount{
			MsgHdr: iso20022.MessageHeader{
				MsgID:   "CAML-ERR",
				CreDtTm: "2026-06-20T10:00:00Z",
			},
			AcctQryDef: iso20022.AccountQueryDefinition{
				AcctCrit: iso20022.AccountCriteria{
					NewCrit: []iso20022.NewCriteria{
						{
							SchCrit: iso20022.SearchCriteria{
								AcctId: iso20022.AccountIdentification{
									ID: nil,
								},
							},
						},
					},
				},
			},
		},
	}
	xmlBytes, err := xml.Marshal(doc)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/payments/lookup", bytes.NewReader(xmlBytes))
	req.Header.Set("Content-Type", "application/xml")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "application/xml")
	body := w.Body.String()
	require.Contains(t, body, "AcctRpt")
}
