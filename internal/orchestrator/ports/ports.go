package ports

import (
	"context"
	"time"

	"switch/internal/orchestrator/domain"
)

type PaymentRepository interface {
	Create(ctx context.Context, p *domain.Payment) error
	UpdateStatus(ctx context.Context, id string, status domain.PaymentStatus) error
	GetByEndToEndID(ctx context.Context, e2eID string) (*domain.Payment, error)
	FindExpiredReservations(ctx context.Context, before time.Time) ([]domain.Reservation, error)
}

type BankClient interface {
	Reserve(ctx context.Context, bic, account string, amount int64) error
	ReleaseReservation(ctx context.Context, bic, account string, amount int64) error
	Credit(ctx context.Context, bic, account string, amount int64) error
	ReverseCredit(ctx context.Context, bic, account string, amount int64) error
}

type OutboxWriter interface {
	Write(ctx context.Context, topic, key string, event any) error
}
