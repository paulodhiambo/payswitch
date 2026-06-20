package audit

import (
	"context"
	"encoding/json"
	"log"

	"switch/internal/orchestrator/domain"
)

type EventHandler struct {
	logger *log.Logger
}

func NewEventHandler() *EventHandler {
	return &EventHandler{logger: log.Default()}
}

func (h *EventHandler) HandlePaymentEvent(ctx context.Context, topic string, payload []byte) error {
	var event domain.PaymentEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return err
	}

	h.logger.Printf("[AUDIT] topic=%s payment_id=%s e2e=%s transition=%s->%s amount=%d%s",
		topic, event.PaymentID, event.EndToEndID, event.FromStatus, event.ToStatus,
		event.Amount, event.Currency)

	return nil
}
