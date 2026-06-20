package saga_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"switch/internal/orchestrator/saga"
)

func TestMockBankClient_ReserveAndCredit(t *testing.T) {
	bank := saga.NewMockBankClient()
	bank.SetBalance("src-acc", 1_000_00)

	err := bank.Reserve(context.Background(), "BIC1", "src-acc", 500_00)
	require.NoError(t, err)

	err = bank.Credit(context.Background(), "BIC2", "dst-acc", 500_00)
	require.NoError(t, err)
}

func TestMockBankClient_InsufficientBalance(t *testing.T) {
	bank := saga.NewMockBankClient()
	bank.SetBalance("src-acc", 100_00)

	err := bank.Reserve(context.Background(), "BIC1", "src-acc", 500_00)
	require.Error(t, err)
	require.Contains(t, err.Error(), "insufficient balance")
}

func TestMockBankClient_ReserveRelease(t *testing.T) {
	bank := saga.NewMockBankClient()
	bank.SetBalance("src-acc", 1_000_00)

	require.NoError(t, bank.Reserve(context.Background(), "BIC1", "src-acc", 500_00))
	require.NoError(t, bank.ReleaseReservation(context.Background(), "BIC1", "src-acc", 500_00))
}

func TestMockBankClient_ReverseCredit(t *testing.T) {
	bank := saga.NewMockBankClient()
	bank.SetBalance("src-acc", 1_000_00)
	bank.SetBalance("dst-acc", 0)

	require.NoError(t, bank.Reserve(context.Background(), "BIC1", "src-acc", 500_00))
	require.NoError(t, bank.Credit(context.Background(), "BIC2", "dst-acc", 500_00))
	require.NoError(t, bank.ReverseCredit(context.Background(), "BIC2", "dst-acc", 500_00))
}

func TestMockBankClient_FailOn(t *testing.T) {
	bank := saga.NewMockBankClient()
	bank.SetBalance("src-acc", 1_000_00)
	bank.SetFailOn("reserve")

	err := bank.Reserve(context.Background(), "BIC1", "src-acc", 500_00)
	require.Error(t, err)
	require.Contains(t, err.Error(), "reserve failed")
}
