package gateway_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"switch/internal/gateway"
	"switch/internal/orchestrator/domain"
	"switch/internal/orchestrator/saga"
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

func setupHandler() *chi.Mux {
	repo := newMemRepo()
	bank := saga.NewMockBankClient()
	bank.SetBalance("SOURCE", 1_000_00)
	bank.SetBalance("DEST", 0)

	s := saga.New(
		&saga.ValidateStep{Repo: repo},
		&saga.ReserveStep{Repo: repo, Bank: bank},
		&saga.CommitStep{Repo: repo, Bank: bank},
	)

	h := gateway.NewHandler(repo, s)
	r := chi.NewRouter()
	h.Register(r)
	return r
}

func TestSubmitPayment_Success(t *testing.T) {
	r := setupHandler()

	body := map[string]any{
		"end_to_end_id":  "e2e-001",
		"source_bic":     "BANKUS33",
		"destination_bic": "BANKDEFF",
		"source_account": "SOURCE",
		"dest_account":   "DEST",
		"amount":         500_00,
		"currency":       "USD",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	var resp gateway.PaymentResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	require.Equal(t, domain.StatusCommitted, resp.Status)
}

func TestSubmitPayment_InvalidAmount(t *testing.T) {
	r := setupHandler()

	body := map[string]any{
		"end_to_end_id":  "e2e-002",
		"source_bic":     "BANKUS33",
		"destination_bic": "BANKDEFF",
		"source_account": "SOURCE",
		"dest_account":   "DEST",
		"amount":         -100,
		"currency":       "USD",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSubmitPayment_MissingFields(t *testing.T) {
	r := setupHandler()

	body := map[string]any{
		"end_to_end_id": "e2e-003",
		"amount":        100_00,
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetPayment_Found(t *testing.T) {
	r := setupHandler()

	body := map[string]any{
		"end_to_end_id":  "e2e-get-001",
		"source_bic":     "BANKUS33",
		"destination_bic": "BANKDEFF",
		"source_account": "SOURCE",
		"dest_account":   "DEST",
		"amount":         250_00,
		"currency":       "USD",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	getReq := httptest.NewRequest(http.MethodGet, "/payments/e2e-get-001", nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)

	require.Equal(t, http.StatusOK, getW.Code)

	var resp gateway.PaymentResponse
	err := json.NewDecoder(getW.Body).Decode(&resp)
	require.NoError(t, err)
	require.Equal(t, domain.StatusCommitted, resp.Status)
}

func TestGetPayment_NotFound(t *testing.T) {
	r := setupHandler()

	req := httptest.NewRequest(http.MethodGet, "/payments/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
}
