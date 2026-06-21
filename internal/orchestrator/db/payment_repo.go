package db

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"switch/internal/orchestrator/db/sqlc"
	"switch/internal/orchestrator/domain"
	"switch/pkg/outbox"
)

func textOrNull(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func dateOrNull(t time.Time) pgtype.Date {
	if t.IsZero() {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: t, Valid: true}
}

type PaymentRepo struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

func NewPaymentRepo(pool *pgxpool.Pool) *PaymentRepo {
	return &PaymentRepo{
		pool: pool,
		q:    sqlc.New(pool),
	}
}

func (r *PaymentRepo) Create(ctx context.Context, p *domain.Payment) error {
	sqlcPayment, err := r.q.CreatePayment(ctx, sqlc.CreatePaymentParams{
		ID:               p.ID,
		EndToEndID:       p.EndToEndID,
		SourceBic:        p.SourceBIC,
		DestinationBic:   p.DestinationBIC,
		SourceAccount:    p.SourceAccount,
		DestAccount:      p.DestAccount,
		Amount:           p.Amount,
		Currency:         p.Currency,
		Status:           string(p.Status),
		Uetr:             textOrNull(p.UETR),
		InstrID:          textOrNull(p.InstrID),
		ChargeBearer:     textOrNull(p.ChargeBearer),
		SttlmDt:          dateOrNull(p.SettlementDate),
		DebtorName:       textOrNull(p.DebtorName),
		CreditorName:     textOrNull(p.CreditorName),
		PurposeCode:      textOrNull(p.PurposeCode),
		RemittanceInfo:   textOrNull(p.RemittanceInfo),
		RouteFee:         pgtype.Int8{Int64: p.RouteFee, Valid: true},
		RouteEstimatedMs: pgtype.Int4{Int32: int32(p.RouteEstimatedMs), Valid: true},
	})
	if err != nil {
		return err
	}
	mapToDomain(sqlcPayment, p)
	return nil
}

func (r *PaymentRepo) CreateWithEvent(ctx context.Context, p *domain.Payment) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	q := r.q.WithTx(tx)
	sqlcPayment, err := q.CreatePayment(ctx, sqlc.CreatePaymentParams{
		ID:             p.ID,
		EndToEndID:     p.EndToEndID,
		SourceBic:      p.SourceBIC,
		DestinationBic: p.DestinationBIC,
		SourceAccount:  p.SourceAccount,
		DestAccount:    p.DestAccount,
		Amount:         p.Amount,
		Currency:       p.Currency,
		Status:         string(p.Status),
		Uetr:           textOrNull(p.UETR),
		InstrID:        textOrNull(p.InstrID),
		ChargeBearer:   textOrNull(p.ChargeBearer),
		SttlmDt:        dateOrNull(p.SettlementDate),
		DebtorName:     textOrNull(p.DebtorName),
		CreditorName:   textOrNull(p.CreditorName),
		PurposeCode:    textOrNull(p.PurposeCode),
		RemittanceInfo:   textOrNull(p.RemittanceInfo),
		RouteFee:         pgtype.Int8{Int64: p.RouteFee, Valid: true},
		RouteEstimatedMs: pgtype.Int4{Int32: int32(p.RouteEstimatedMs), Valid: true},
	})
	if err != nil {
		return err
	}
	mapToDomain(sqlcPayment, p)

	if err := outbox.Write(ctx, tx, "payment.received", p.EndToEndID, domain.PaymentEvent{
		PaymentID:  p.ID,
		EndToEndID: p.EndToEndID,
		ToStatus:   domain.StatusReceived,
		SourceBIC:  p.SourceBIC,
		DestBIC:    p.DestinationBIC,
		Amount:     p.Amount,
		Currency:   p.Currency,
		Timestamp:  sqlcPayment.CreatedAt.Time,
	}); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *PaymentRepo) UpdateStatus(ctx context.Context, id string, status domain.PaymentStatus) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	q := r.q.WithTx(tx)
	fromStatus, err := q.UpdatePaymentStatusReturning(ctx, sqlc.UpdatePaymentStatusReturningParams{
		Status: string(status),
		ID:     id,
	})
	if err != nil {
		return err
	}

	if err := writeEventTx(ctx, q, tx, id, domain.PaymentStatus(fromStatus), status); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *PaymentRepo) MarkReserved(ctx context.Context, id string, ttl time.Duration) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	q := r.q.WithTx(tx)
	now := time.Now()
	expiresAt := now.Add(ttl)

	if err := q.MarkPaymentReserved(ctx, sqlc.MarkPaymentReservedParams{
		Status:     string(domain.StatusReserved),
		ReservedAt: pgtype.Timestamptz{Time: now, Valid: true},
		ExpiresAt:  pgtype.Timestamptz{Time: expiresAt, Valid: true},
		ID:         id,
	}); err != nil {
		return err
	}

	if err := writeEventTx(ctx, q, tx, id, domain.StatusValidated, domain.StatusReserved); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func writeEventTx(ctx context.Context, q *sqlc.Queries, tx pgx.Tx, paymentID string, from, to domain.PaymentStatus) error {
	row, err := q.GetPaymentEventFields(ctx, paymentID)
	if err != nil {
		return err
	}

	return outbox.Write(ctx, tx, "payment."+strings.ToLower(string(to)), row.EndToEndID, domain.PaymentEvent{
		PaymentID:  row.ID,
		EndToEndID: row.EndToEndID,
		FromStatus: from,
		ToStatus:   to,
		SourceBIC:  row.SourceBic,
		DestBIC:    row.DestinationBic,
		Amount:     row.Amount,
		Currency:   row.Currency,
	})
}

func (r *PaymentRepo) GetByID(ctx context.Context, id string) (*domain.Payment, error) {
	sqlcPayment, err := r.q.GetPaymentByID(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return mapFromSQLC(sqlcPayment), nil
}

func (r *PaymentRepo) GetByEndToEndID(ctx context.Context, e2eID string) (*domain.Payment, error) {
	sqlcPayment, err := r.q.GetPaymentByEndToEndID(ctx, e2eID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return mapFromSQLC(sqlcPayment), nil
}

func (r *PaymentRepo) UpdateRoute(ctx context.Context, id string, fee int64, estimatedMs int) error {
	return r.q.UpdatePaymentRoute(ctx, sqlc.UpdatePaymentRouteParams{
		RouteFee:         pgtype.Int8{Int64: fee, Valid: true},
		RouteEstimatedMs: pgtype.Int4{Int32: int32(estimatedMs), Valid: true},
		Status:           string(domain.StatusRouted),
		ID:               id,
	})
}

func (r *PaymentRepo) FindExpiredReservations(ctx context.Context, before time.Time) ([]domain.Reservation, error) {
	rows, err := r.q.FindExpiredReservations(ctx, pgtype.Timestamptz{Time: before, Valid: true})
	if err != nil {
		return nil, err
	}

	result := make([]domain.Reservation, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.Reservation{
			PaymentID:     row.ID,
			SourceAccount: row.SourceAccount,
			Amount:        row.Amount,
			Status:        "RESERVED",
			ReservedAt:    row.ReservedAt.Time,
			ExpiresAt:     row.ExpiresAt.Time,
		})
	}
	return result, nil
}

func mapFromSQLC(s *sqlc.Payment) *domain.Payment {
	p := &domain.Payment{
		ID:             s.ID,
		EndToEndID:     s.EndToEndID,
		SourceBIC:      s.SourceBic,
		DestinationBIC: s.DestinationBic,
		SourceAccount:  s.SourceAccount,
		DestAccount:    s.DestAccount,
		Amount:         s.Amount,
		Currency:       s.Currency,
		Status:         domain.PaymentStatus(s.Status),
		CreatedAt:      s.CreatedAt.Time,
		UpdatedAt:      s.UpdatedAt.Time,
	}
	if s.QuoteID.Valid {
		p.QuoteID = &s.QuoteID.String
	}
	if s.RouteFee.Valid {
		p.RouteFee = s.RouteFee.Int64
	}
	if s.RouteEstimatedMs.Valid {
		p.RouteEstimatedMs = int(s.RouteEstimatedMs.Int32)
	}
	if s.ReservedAt.Valid {
		p.ReservedAt = &s.ReservedAt.Time
	}
	if s.ExpiresAt.Valid {
		p.ExpiresAt = &s.ExpiresAt.Time
	}
	if s.Uetr.Valid {
		p.UETR = s.Uetr.String
	}
	if s.InstrID.Valid {
		p.InstrID = s.InstrID.String
	}
	if s.ChargeBearer.Valid {
		p.ChargeBearer = s.ChargeBearer.String
	}
	if s.SttlmDt.Valid {
		p.SettlementDate = s.SttlmDt.Time
	}
	if s.DebtorName.Valid {
		p.DebtorName = s.DebtorName.String
	}
	if s.CreditorName.Valid {
		p.CreditorName = s.CreditorName.String
	}
	if s.PurposeCode.Valid {
		p.PurposeCode = s.PurposeCode.String
	}
	if s.RemittanceInfo.Valid {
		p.RemittanceInfo = s.RemittanceInfo.String
	}
	return p
}

func mapToDomain(s *sqlc.Payment, p *domain.Payment) {
	p.CreatedAt = s.CreatedAt.Time
	p.UpdatedAt = s.UpdatedAt.Time
	if s.QuoteID.Valid {
		p.QuoteID = &s.QuoteID.String
	}
	if s.RouteFee.Valid {
		p.RouteFee = s.RouteFee.Int64
	}
	if s.RouteEstimatedMs.Valid {
		p.RouteEstimatedMs = int(s.RouteEstimatedMs.Int32)
	}
	if s.ReservedAt.Valid {
		p.ReservedAt = &s.ReservedAt.Time
	}
	if s.ExpiresAt.Valid {
		p.ExpiresAt = &s.ExpiresAt.Time
	}
}
