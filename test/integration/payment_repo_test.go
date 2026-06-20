package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"switch/internal/orchestrator/db"
	"switch/internal/orchestrator/domain"
)

func TestPaymentRepo_CreateAndGet(t *testing.T) {
	repo := db.NewPaymentRepo(env.pgPool)
	ctx := context.Background()

	p := &domain.Payment{
		ID:             uuid.New().String(),
		EndToEndID:     "repo-test-" + uuid.New().String(),
		SourceBIC:      "BANKUS33",
		DestinationBIC: "BANKDEFF",
		SourceAccount:  "ACC-A",
		DestAccount:    "DE89370400440532013000",
		Amount:         500_00,
		Currency:       "USD",
		Status:         domain.StatusReceived,
	}

	err := repo.Create(ctx, p)
	require.NoError(t, err)
	require.NotZero(t, p.CreatedAt)

	got, err := repo.GetByID(ctx, p.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, p.ID, got.ID)
	assert.Equal(t, p.EndToEndID, got.EndToEndID)
	assert.Equal(t, domain.StatusReceived, got.Status)
	assert.Equal(t, int64(500_00), got.Amount)

	byE2E, err := repo.GetByEndToEndID(ctx, p.EndToEndID)
	require.NoError(t, err)
	require.NotNil(t, byE2E)
	assert.Equal(t, p.ID, byE2E.ID)
}

func TestPaymentRepo_CreateWithEvent(t *testing.T) {
	repo := db.NewPaymentRepo(env.pgPool)
	ctx := context.Background()

	p := &domain.Payment{
		ID:             uuid.New().String(),
		EndToEndID:     "create-event-" + uuid.New().String(),
		SourceBIC:      "BANKUS33",
		DestinationBIC: "BANKDEFF",
		SourceAccount:  "ACC-A",
		DestAccount:    "DE89370400440532013000",
		Amount:         100_00,
		Currency:       "EUR",
		Status:         domain.StatusReceived,
	}

	err := repo.CreateWithEvent(ctx, p)
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, p.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, domain.StatusReceived, got.Status)
}

func TestPaymentRepo_UpdateStatus(t *testing.T) {
	repo := db.NewPaymentRepo(env.pgPool)
	ctx := context.Background()

	p := &domain.Payment{
		ID:             uuid.New().String(),
		EndToEndID:     "update-status-" + uuid.New().String(),
		SourceBIC:      "BANKUS33",
		DestinationBIC: "BANKDEFF",
		SourceAccount:  "ACC-A",
		DestAccount:    "DE89370400440532013000",
		Amount:         250_00,
		Currency:       "GBP",
		Status:         domain.StatusReceived,
	}

	err := repo.Create(ctx, p)
	require.NoError(t, err)

	err = repo.UpdateStatus(ctx, p.ID, domain.StatusValidated)
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, p.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, domain.StatusValidated, got.Status)
}

func TestPaymentRepo_MarkReservedAndFindExpired(t *testing.T) {
	repo := db.NewPaymentRepo(env.pgPool)
	ctx := context.Background()

	p := &domain.Payment{
		ID:             uuid.New().String(),
		EndToEndID:     "reserve-test-" + uuid.New().String(),
		SourceBIC:      "BANKUS33",
		DestinationBIC: "BANKDEFF",
		SourceAccount:  "ACC-A",
		DestAccount:    "DE89370400440532013000",
		Amount:         750_00,
		Currency:       "USD",
		Status:         domain.StatusReceived,
	}

	err := repo.Create(ctx, p)
	require.NoError(t, err)

	ttl := 1 * time.Second
	err = repo.MarkReserved(ctx, p.ID, ttl)
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, p.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, domain.StatusReserved, got.Status)
	require.NotNil(t, got.ReservedAt)
	require.NotNil(t, got.ExpiresAt)

	beforeExpiry, err := repo.FindExpiredReservations(ctx, time.Now())
	require.NoError(t, err)
	require.Empty(t, beforeExpiry, "should not be expired before TTL")

	time.Sleep(2 * time.Second)

	afterExpiry, err := repo.FindExpiredReservations(ctx, time.Now())
	require.NoError(t, err)
	require.NotEmpty(t, afterExpiry, "should find expired reservations")

	found := false
	for _, r := range afterExpiry {
		if r.PaymentID == p.ID {
			found = true
			assert.Equal(t, "ACC-A", r.SourceAccount)
			assert.Equal(t, int64(750_00), r.Amount)
			break
		}
	}
	assert.True(t, found, "expected expired reservation to be found")
}
