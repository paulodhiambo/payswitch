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

	"switch/internal/compliance"
	"switch/internal/gateway"
	"switch/internal/orchestrator/domain"
	"switch/internal/orchestrator/saga"
	"switch/internal/participant"
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
