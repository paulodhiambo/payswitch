package sweep_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"switch/internal/orchestrator/domain"
	"switch/internal/orchestrator/sweep"
)

type mockRepo struct {
	mu            sync.Mutex
	expired       []domain.Reservation
	payments      map[string]*domain.Payment
	findErr       error
	getByIDCalls int
}

func newMockRepo() *mockRepo {
	return &mockRepo{payments: make(map[string]*domain.Payment)}
}

func (r *mockRepo) FindExpiredReservations(_ context.Context, _ time.Time) ([]domain.Reservation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.expired, r.findErr
}

func (r *mockRepo) GetByID(_ context.Context, id string) (*domain.Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.getByIDCalls++
	return r.payments[id], nil
}

func (r *mockRepo) Create(_ context.Context, _ *domain.Payment) error           { return nil }
func (r *mockRepo) CreateWithEvent(_ context.Context, _ *domain.Payment) error  { return nil }
func (r *mockRepo) UpdateStatus(_ context.Context, _ string, _ domain.PaymentStatus) error { return nil }
func (r *mockRepo) GetByEndToEndID(_ context.Context, _ string) (*domain.Payment, error) { return nil, nil }
func (r *mockRepo) MarkReserved(_ context.Context, _ string, _ time.Duration) error { return nil }
func (r *mockRepo) UpdateRoute(_ context.Context, _ string, _ int64, _ int) error  { return nil }

type mockSaga struct {
	compensated []string
	err         error
}

func (s *mockSaga) CompensatePayment(_ context.Context, p *domain.Payment) error {
	s.compensated = append(s.compensated, p.ID)
	return s.err
}

func TestSweeper_SweepsExpiredReservations(t *testing.T) {
	repo := newMockRepo()
	repo.expired = []domain.Reservation{
		{PaymentID: "pay-1", Amount: 100_00},
		{PaymentID: "pay-2", Amount: 200_00},
	}
	repo.payments["pay-1"] = &domain.Payment{ID: "pay-1"}
	repo.payments["pay-2"] = &domain.Payment{ID: "pay-2"}

	saga := &mockSaga{}
	sw := sweep.New(repo, saga)

	sw.SweepOnce(context.Background())

	require.Equal(t, 2, len(saga.compensated))
	require.Equal(t, "pay-1", saga.compensated[0])
	require.Equal(t, "pay-2", saga.compensated[1])
}

func TestSweeper_SkipsMissingPayment(t *testing.T) {
	repo := newMockRepo()
	repo.expired = []domain.Reservation{
		{PaymentID: "pay-missing"},
	}

	saga := &mockSaga{}
	sw := sweep.New(repo, saga)

	sw.SweepOnce(context.Background())

	require.Equal(t, 0, len(saga.compensated))
}

func TestSweeper_HandlesFindError(t *testing.T) {
	repo := newMockRepo()
	repo.findErr = errors.New("db error")

	saga := &mockSaga{}
	sw := sweep.New(repo, saga)

	sw.SweepOnce(context.Background())
	require.Equal(t, 0, len(saga.compensated))
}
