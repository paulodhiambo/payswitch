package settlement_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"switch/internal/settlement"
)

func TestWindow_AddAndSettle(t *testing.T) {
	w := settlement.NewWindow(1*time.Hour, 10)

	err := w.Add(context.Background(), settlement.SettlementEntry{
		PaymentID: "pay-1", SourceBIC: "A", DestBIC: "B",
		Amount: 100_00, Currency: "USD",
	})
	require.NoError(t, err)

	err = w.Add(context.Background(), settlement.SettlementEntry{
		PaymentID: "pay-2", SourceBIC: "B", DestBIC: "A",
		Amount: 50_00, Currency: "USD",
	})
	require.NoError(t, err)

	positions, err := w.Settle(context.Background())
	require.NoError(t, err)

	require.Len(t, positions, 2)
	for _, p := range positions {
		if p.BIC == "A" {
			require.Equal(t, int64(-50_00), p.Amount) // paid 100, received 50 = net -50
		}
		if p.BIC == "B" {
			require.Equal(t, int64(50_00), p.Amount) // received 100, paid 50 = net +50
		}
	}
}

func TestWindow_DoubleSettleError(t *testing.T) {
	w := settlement.NewWindow(1*time.Hour, 10)
	w.Settle(context.Background())
	_, err := w.Settle(context.Background())
	require.Error(t, err)
}

func TestWindow_EmptySettle(t *testing.T) {
	w := settlement.NewWindow(1*time.Hour, 10)
	positions, err := w.Settle(context.Background())
	require.NoError(t, err)
	require.Empty(t, positions)
}

func TestWindow_ZeroNetPositionsOmitted(t *testing.T) {
	w := settlement.NewWindow(1*time.Hour, 10)

	w.Add(context.Background(), settlement.SettlementEntry{
		PaymentID: "pay-1", SourceBIC: "A", DestBIC: "B",
		Amount: 100_00, Currency: "USD",
	})
	w.Add(context.Background(), settlement.SettlementEntry{
		PaymentID: "pay-2", SourceBIC: "B", DestBIC: "A",
		Amount: 100_00, Currency: "USD",
	})

	positions, _ := w.Settle(context.Background())
	require.Empty(t, positions)
}
