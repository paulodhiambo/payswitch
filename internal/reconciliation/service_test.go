package reconciliation_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"switch/internal/reconciliation"
)

func TestService_MatchSuccess(t *testing.T) {
	svc := reconciliation.New()
	svc.AddRecord(reconciliation.Record{
		PaymentID: "pay-1", Status: "COMMITTED",
	})

	err := svc.Match("pay-1", "COMMITTED")
	require.NoError(t, err)
}

func TestService_MatchFailure(t *testing.T) {
	svc := reconciliation.New()
	svc.AddRecord(reconciliation.Record{
		PaymentID: "pay-1", Status: "ABORTED",
	})

	err := svc.Match("pay-1", "COMMITTED")
	require.Error(t, err)
}

func TestService_MatchNotFound(t *testing.T) {
	svc := reconciliation.New()
	err := svc.Match("nonexistent", "COMMITTED")
	require.Error(t, err)
}

func TestService_Report(t *testing.T) {
	svc := reconciliation.New()
	svc.AddRecord(reconciliation.Record{PaymentID: "pay-1", Status: "COMMITTED", Matched: true})
	svc.AddRecord(reconciliation.Record{PaymentID: "pay-2", Status: "ABORTED", Matched: false})

	report := svc.Report()
	require.Len(t, report, 2)
}
