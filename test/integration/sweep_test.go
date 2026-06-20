package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"switch/internal/compliance"
	"switch/internal/orchestrator/db"
	"switch/internal/orchestrator/domain"
	"switch/internal/orchestrator/saga"
	"switch/internal/orchestrator/sweep"
)

func TestSweeper_ExpiresExpiredReservation(t *testing.T) {
	repo := db.NewPaymentRepo(env.pgPool)
	ctx := context.Background()

	bank := saga.NewMockBankClient()
	bank.SetBalance("ACC-A", 1_000_00)

	complianceClient := compliance.New()
	paymentSaga := saga.New(
		&saga.ValidateStep{Repo: repo},
		&saga.ScreenStep{Client: complianceClient, Repo: repo},
		&saga.ReserveStep{Repo: repo, Bank: bank, TTL: 1 * time.Second},
		&saga.CommitStep{Repo: repo, Bank: bank},
	)

	p := &domain.Payment{
		ID:             uuid.New().String(),
		EndToEndID:     "sweep-test-" + uuid.New().String(),
		SourceBIC:      "BANKUS33",
		DestinationBIC: "BANKDEFF",
		SourceAccount:  "ACC-A",
		DestAccount:    "DE89370400440532013000",
		Amount:         500_00,
		Currency:       "USD",
		Status:         domain.StatusReceived,
	}

	err := repo.CreateWithEvent(ctx, p)
	require.NoError(t, err)

	err = paymentSaga.Run(ctx, p)
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, p.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, domain.StatusCommitted, got.Status)
}

func TestSweeper_CompensatesExpiredReservation(t *testing.T) {
	repo := db.NewPaymentRepo(env.pgPool)
	ctx := context.Background()

	bank := saga.NewMockBankClient()
	bank.SetBalance("ACC-A", 1_000_00)

	paymentSaga := saga.New(
		&saga.ReserveStep{Repo: repo, Bank: bank, TTL: 1 * time.Second},
	)

	p := &domain.Payment{
		ID:             uuid.New().String(),
		EndToEndID:     "sweep-expire-" + uuid.New().String(),
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

	err = repo.UpdateStatus(ctx, p.ID, domain.StatusValidated)
	require.NoError(t, err)

	err = repo.MarkReserved(ctx, p.ID, 1*time.Second)
	require.NoError(t, err)

	time.Sleep(2 * time.Second)

	sw := sweep.New(repo, paymentSaga)
	sw.SweepOnce(ctx)

	got, err := repo.GetByID(ctx, p.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, domain.StatusAborted, got.Status,
		"sweeper should abort expired reservations")
}
