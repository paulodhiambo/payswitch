package audit_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"switch/internal/audit"
	"switch/internal/orchestrator/domain"
)

func TestEventHandler_LogsPaymentEvent(t *testing.T) {
	h := audit.NewEventHandler()

	event := domain.PaymentEvent{
		PaymentID:  "pay-123",
		EndToEndID: "e2e-001",
		FromStatus: domain.StatusReceived,
		ToStatus:   domain.StatusValidated,
		SourceBIC:  "BANKUS33",
		DestBIC:    "BANKDEFF",
		Amount:     500_00,
		Currency:   "USD",
		Timestamp:  time.Now(),
	}
	payload, _ := json.Marshal(event)

	err := h.HandlePaymentEvent(context.Background(), "payment.validated", payload)
	require.NoError(t, err)
}

func TestEventHandler_InvalidPayload(t *testing.T) {
	h := audit.NewEventHandler()

	err := h.HandlePaymentEvent(context.Background(), "payment.received", []byte(`not json`))
	require.Error(t, err)
}
