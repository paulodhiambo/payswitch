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
)

func TestSaga_FullFlow(t *testing.T) {
	repo := db.NewPaymentRepo(env.pgPool)
	ctx := context.Background()

	bank := saga.NewMockBankClient()
	bank.SetBalance("ACC-A", 1_000_00)

	complianceClient := compliance.New()

	s := saga.New(
		&saga.ValidateStep{Repo: repo},
		&saga.ScreenStep{Client: complianceClient, Repo: repo},
		&saga.ReserveStep{Repo: repo, Bank: bank, TTL: 5 * time.Minute},
		&saga.CommitStep{Repo: repo, Bank: bank},
	)

	p := &domain.Payment{
		ID:             uuid.New().String(),
		EndToEndID:     "saga-full-" + uuid.New().String(),
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

	err = s.Run(ctx, p)
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, p.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, domain.StatusCommitted, got.Status)
}

func TestSaga_CompensationOnFailure(t *testing.T) {
	repo := db.NewPaymentRepo(env.pgPool)
	ctx := context.Background()

	bank := saga.NewMockBankClient()
	bank.SetBalance("ACC-A", 100_00)

	complianceClient := compliance.New()

	s := saga.New(
		&saga.ValidateStep{Repo: repo},
		&saga.ScreenStep{Client: complianceClient, Repo: repo},
		&saga.ReserveStep{Repo: repo, Bank: bank, TTL: 5 * time.Minute},
		&saga.CommitStep{Repo: repo, Bank: bank},
	)

	p := &domain.Payment{
		ID:             uuid.New().String(),
		EndToEndID:     "saga-comp-" + uuid.New().String(),
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

	err = s.Run(ctx, p)
	require.Error(t, err)

	got, err := repo.GetByID(ctx, p.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, domain.StatusAborted, got.Status,
		"saga compensation should set status to ABORTED")
}

func TestSaga_ValidateRejectsInvalidAmount(t *testing.T) {
	repo := db.NewPaymentRepo(env.pgPool)
	ctx := context.Background()

	step := &saga.ValidateStep{Repo: repo}

	p := &domain.Payment{
		ID:             uuid.New().String(),
		EndToEndID:     "saga-invalid-" + uuid.New().String(),
		SourceBIC:      "BANKUS33",
		DestinationBIC: "BANKDEFF",
		SourceAccount:  "ACC-A",
		DestAccount:    "DE89370400440532013000",
		Amount:         0,
		Currency:       "USD",
		Status:         domain.StatusReceived,
	}

	err := step.Execute(ctx, p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid amount")
}

func TestSaga_ReserveFailsOnInsufficientBalance(t *testing.T) {
	repo := db.NewPaymentRepo(env.pgPool)
	ctx := context.Background()

	bank := saga.NewMockBankClient()
	bank.SetBalance("ACC-A", 50_00)

	step := &saga.ReserveStep{
		Repo: repo,
		Bank: bank,
		TTL:  5 * time.Minute,
	}

	p := &domain.Payment{
		ID:             uuid.New().String(),
		EndToEndID:     "saga-insuf-" + uuid.New().String(),
		SourceBIC:      "BANKUS33",
		DestinationBIC: "BANKDEFF",
		SourceAccount:  "ACC-A",
		DestAccount:    "DE89370400440532013000",
		Amount:         500_00,
		Currency:       "USD",
		Status:         domain.StatusValidated,
	}

	err := step.Execute(ctx, p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient")
}
