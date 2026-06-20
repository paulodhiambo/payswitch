package sweep

import (
	"context"
	"log"
	"time"

	"switch/internal/orchestrator/domain"
	"switch/internal/orchestrator/ports"
)

type SagaRunner interface {
	CompensatePayment(ctx context.Context, p *domain.Payment) error
}

type Sweeper struct {
	repo ports.PaymentRepository
	saga SagaRunner
}

func New(repo ports.PaymentRepository, saga SagaRunner) *Sweeper {
	return &Sweeper{repo: repo, saga: saga}
}

func (s *Sweeper) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.SweepOnce(ctx)
		}
	}
}

func (s *Sweeper) SweepOnce(ctx context.Context) {
	expired, err := s.repo.FindExpiredReservations(ctx, time.Now())
	if err != nil {
		log.Printf("sweeper: find expired reservations: %v", err)
		return
	}
	for _, r := range expired {
		p, err := s.repo.GetByID(ctx, r.PaymentID)
		if err != nil || p == nil {
			log.Printf("sweeper: load payment %s: %v", r.PaymentID, err)
			continue
		}
		if err := s.saga.CompensatePayment(ctx, p); err != nil {
			log.Printf("sweeper: compensate payment %s: %v", r.PaymentID, err)
		} else {
			log.Printf("sweeper: compensated expired reservation %s", r.PaymentID)
		}
	}
}
