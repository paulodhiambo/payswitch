package saga

import (
	"context"
	"fmt"
	"log/slog"
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

type LookupStep struct {
	Client ports.LookupClient
	Repo   ports.PaymentRepository
}

func (s *LookupStep) Name() string { return "lookup_bic" }

func (s *LookupStep) Execute(ctx context.Context, p *domain.Payment) error {
	name, country, err := s.Client.ResolveBIC(ctx, p.DestinationBIC)
	if err != nil {
		return fmt.Errorf("lookup destination BIC: %w", err)
	}
	slog.Debug("destination BIC resolved", "bic", p.DestinationBIC, "name", name, "country", country)
	if err := s.Repo.UpdateStatus(ctx, p.ID, domain.StatusValidated); err != nil {
		return fmt.Errorf("update status after BIC lookup: %w", err)
	}
	return nil
}

func (s *LookupStep) Compensate(_ context.Context, _ *domain.Payment) error {
	return nil
}

type RouteStep struct {
	Client ports.RoutingClient
	Repo   ports.PaymentRepository
}

func (s *RouteStep) Name() string { return "route" }

func (s *RouteStep) Execute(ctx context.Context, p *domain.Payment) error {
	fee, estimatedMs, err := s.Client.FindRouteClient(ctx, p.SourceBIC, p.DestinationBIC, p.Currency)
	if err != nil {
		return fmt.Errorf("find route: %w", err)
	}
	slog.Debug("route found", "source", p.SourceBIC, "dest", p.DestinationBIC, "fee", fee, "estimated_ms", estimatedMs)
	return s.Repo.UpdateStatus(ctx, p.ID, domain.StatusValidated)
}

func (s *RouteStep) Compensate(_ context.Context, _ *domain.Payment) error {
	return nil
}

type QuoteStep struct {
	Client ports.QuotingClient
	Repo   ports.PaymentRepository
}

func (s *QuoteStep) Name() string { return "quote" }

func (s *QuoteStep) Execute(ctx context.Context, p *domain.Payment) error {
	quoteID, fee, total, err := s.Client.GetQuoteClient(ctx, p.SourceBIC, p.DestinationBIC, p.Amount, p.Currency)
	if err != nil {
		return fmt.Errorf("get quote: %w", err)
	}
	slog.Debug("quote obtained", "quote_id", quoteID, "fee", fee, "total", total)
	if err := s.Repo.UpdateStatus(ctx, p.ID, domain.StatusQuoted); err != nil {
		return fmt.Errorf("update status to QUOTED: %w", err)
	}
	return nil
}

func (s *QuoteStep) Compensate(ctx context.Context, p *domain.Payment) error {
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

type SettleStep struct {
	Client ports.SettlementClient
	Repo   ports.PaymentRepository
}

func (s *SettleStep) Name() string { return "settle" }

func (s *SettleStep) Execute(ctx context.Context, p *domain.Payment) error {
	if err := s.Client.Submit(ctx, p); err != nil {
		return fmt.Errorf("submit for settlement: %w", err)
	}
	return s.Repo.UpdateStatus(ctx, p.ID, domain.StatusCommitted)
}

func (s *SettleStep) Compensate(ctx context.Context, p *domain.Payment) error {
	return s.Repo.UpdateStatus(ctx, p.ID, domain.StatusAborted)
}

type RecordReconciliationStep struct {
	Client ports.ReconciliationClient
	Repo   ports.PaymentRepository
}

func (s *RecordReconciliationStep) Name() string { return "record_reconciliation" }

func (s *RecordReconciliationStep) Execute(ctx context.Context, p *domain.Payment) error {
	if err := s.Client.AddRecordClient(ctx, p.ID, p.SourceBIC, p.DestinationBIC, p.Amount, p.Currency, string(p.Status)); err != nil {
		slog.Warn("reconciliation record failed", "payment_id", p.ID, "error", err)
	}
	return nil
}

func (s *RecordReconciliationStep) Compensate(_ context.Context, _ *domain.Payment) error {
	return nil
}

type NotifyStep struct {
	Client ports.NotificationClient
}

func (s *NotifyStep) Name() string { return "notify" }

func (s *NotifyStep) Execute(ctx context.Context, p *domain.Payment) error {
	if err := s.Client.NotifyClient(ctx, p.SourceBIC, "webhook", "Payment Processed",
		"Your payment has been processed.", p.ID, string(p.Status)); err != nil {
		slog.Warn("notify source failed", "payment_id", p.ID, "error", err)
	}
	if err := s.Client.NotifyClient(ctx, p.DestinationBIC, "webhook", "Payment Received",
		"A payment has been credited to your account.", p.ID, string(p.Status)); err != nil {
		slog.Warn("notify destination failed", "payment_id", p.ID, "error", err)
	}
	return nil
}

func (s *NotifyStep) Compensate(_ context.Context, _ *domain.Payment) error {
	return nil
}
