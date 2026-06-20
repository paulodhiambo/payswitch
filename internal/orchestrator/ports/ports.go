package ports

import (
	"context"
	"time"

	"switch/internal/orchestrator/domain"
)

type PaymentRepository interface {
	Create(ctx context.Context, p *domain.Payment) error
	UpdateStatus(ctx context.Context, id string, status domain.PaymentStatus) error
	GetByID(ctx context.Context, id string) (*domain.Payment, error)
	GetByEndToEndID(ctx context.Context, e2eID string) (*domain.Payment, error)
	FindExpiredReservations(ctx context.Context, before time.Time) ([]domain.Reservation, error)
	MarkReserved(ctx context.Context, id string, ttl time.Duration) error
}

type BankClient interface {
	Reserve(ctx context.Context, bic, account string, amount int64) error
	ReleaseReservation(ctx context.Context, bic, account string, amount int64) error
	Credit(ctx context.Context, bic, account string, amount int64) error
	ReverseCredit(ctx context.Context, bic, account string, amount int64) error
}

type ComplianceClient interface {
	Screen(ctx context.Context, p *domain.Payment) (domain.ComplianceResult, error)
}

type LookupClient interface {
	ResolveBIC(ctx context.Context, bic string) (name, country string, err error)
}

type QuotingClient interface {
	GetQuoteClient(ctx context.Context, sourceBIC, destBIC string, amount int64, currency string) (quoteID string, fee int64, total int64, err error)
}

type NotificationClient interface {
	NotifyClient(ctx context.Context, participantID, channel, title, body, paymentID, status string) error
}

type SettlementClient interface {
	Submit(ctx context.Context, p *domain.Payment) error
}

type ReconciliationClient interface {
	AddRecordClient(ctx context.Context, paymentID, sourceBIC, destBIC string, amount int64, currency, status string) error
}

type RoutingClient interface {
	FindRouteClient(ctx context.Context, sourceBIC, destBIC, currency string) (fee int64, estimatedMs int, err error)
}

type OutboxWriter interface {
	Write(ctx context.Context, topic, key string, event any) error
}
