package saga

import (
	"context"
	"fmt"
	"time"

	"switch/internal/orchestrator/domain"
	"switch/internal/orchestrator/ports"
)

const DefaultReservationTTL = 5 * time.Minute

type ValidateStep struct {
	Repo ports.PaymentRepository
}

func (s *ValidateStep) Name() string { return "validate" }

func (s *ValidateStep) Execute(ctx context.Context, p *domain.Payment) error {
	if p.Amount <= 0 {
		return fmt.Errorf("invalid amount: %d", p.Amount)
	}
	if p.SourceBIC == "" || p.DestinationBIC == "" {
		return fmt.Errorf("source and destination BIC are required")
	}
	if p.SourceAccount == "" || p.DestAccount == "" {
		return fmt.Errorf("source and destination accounts are required")
	}
	if p.Currency == "" {
		return fmt.Errorf("currency is required")
	}
	if err := s.Repo.UpdateStatus(ctx, p.ID, domain.StatusValidated); err != nil {
		return fmt.Errorf("update status to VALIDATED: %w", err)
	}
	return nil
}

func (s *ValidateStep) Compensate(ctx context.Context, p *domain.Payment) error {
	return s.Repo.UpdateStatus(ctx, p.ID, domain.StatusAborted)
}

type ScreenStep struct {
	Client ports.ComplianceClient
	Repo   ports.PaymentRepository
}

func (s *ScreenStep) Name() string { return "screen" }

func (s *ScreenStep) Execute(ctx context.Context, p *domain.Payment) error {
	result, err := s.Client.Screen(ctx, p)
	if err != nil {
		return fmt.Errorf("compliance screen: %w", err)
	}
	if !result.Cleared {
		return fmt.Errorf("compliance check failed: %s", result.Reason)
	}
	if err := s.Repo.UpdateStatus(ctx, p.ID, domain.StatusScreened); err != nil {
		return fmt.Errorf("update status to SCREENED: %w", err)
	}
	return nil
}

func (s *ScreenStep) Compensate(ctx context.Context, p *domain.Payment) error {
	return s.Repo.UpdateStatus(ctx, p.ID, domain.StatusAborted)
}

type ReserveStep struct {
	Repo ports.PaymentRepository
	Bank ports.BankClient
	TTL  time.Duration
}

func (s *ReserveStep) Name() string { return "reserve_source" }

func (s *ReserveStep) Execute(ctx context.Context, p *domain.Payment) error {
	if err := s.Bank.Reserve(ctx, p.SourceBIC, p.SourceAccount, p.Amount); err != nil {
		return fmt.Errorf("reserve at source bank: %w", err)
	}
	ttl := s.TTL
	if ttl <= 0 {
		ttl = DefaultReservationTTL
	}
	if err := s.Repo.MarkReserved(ctx, p.ID, ttl); err != nil {
		return fmt.Errorf("mark reserved: %w", err)
	}
	return nil
}

func (s *ReserveStep) Compensate(ctx context.Context, p *domain.Payment) error {
	_ = s.Bank.ReleaseReservation(ctx, p.SourceBIC, p.SourceAccount, p.Amount)
	return s.Repo.UpdateStatus(ctx, p.ID, domain.StatusAborted)
}

type CommitStep struct {
	Repo ports.PaymentRepository
	Bank ports.BankClient
}

func (s *CommitStep) Name() string { return "credit_destination" }

func (s *CommitStep) Execute(ctx context.Context, p *domain.Payment) error {
	if err := s.Bank.Credit(ctx, p.DestinationBIC, p.DestAccount, p.Amount); err != nil {
		return fmt.Errorf("credit destination bank: %w", err)
	}
	if err := s.Repo.UpdateStatus(ctx, p.ID, domain.StatusCommitted); err != nil {
		return fmt.Errorf("update status to COMMITTED: %w", err)
	}
	return nil
}

func (s *CommitStep) Compensate(ctx context.Context, p *domain.Payment) error {
	_ = s.Bank.ReverseCredit(ctx, p.DestinationBIC, p.DestAccount, p.Amount)
	return s.Repo.UpdateStatus(ctx, p.ID, domain.StatusAborted)
}
