package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"switch/internal/orchestrator/domain"
	"switch/pkg/outbox"
)

const defaultReservationTTL = 5 * time.Minute

type PaymentRepo struct {
	pool *pgxpool.Pool
}

func NewPaymentRepo(pool *pgxpool.Pool) *PaymentRepo {
	return &PaymentRepo{pool: pool}
}

func (r *PaymentRepo) Create(ctx context.Context, p *domain.Payment) error {
	return r.pool.QueryRow(ctx,
		`INSERT INTO payment (id, end_to_end_id, source_bic, destination_bic,
		                      source_account, dest_account, amount, currency, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING created_at, updated_at`,
		p.ID, p.EndToEndID, p.SourceBIC, p.DestinationBIC,
		p.SourceAccount, p.DestAccount, p.Amount, p.Currency, p.Status,
	).Scan(&p.CreatedAt, &p.UpdatedAt)
}

func (r *PaymentRepo) CreateWithEvent(ctx context.Context, p *domain.Payment) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := tx.QueryRow(ctx,
		`INSERT INTO payment (id, end_to_end_id, source_bic, destination_bic,
		                      source_account, dest_account, amount, currency, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING created_at, updated_at`,
		p.ID, p.EndToEndID, p.SourceBIC, p.DestinationBIC,
		p.SourceAccount, p.DestAccount, p.Amount, p.Currency, p.Status,
	).Scan(&p.CreatedAt, &p.UpdatedAt); err != nil {
		return err
	}

	if err := outbox.Write(ctx, tx, "payment.received", p.EndToEndID, domain.PaymentEvent{
		PaymentID:  p.ID,
		EndToEndID: p.EndToEndID,
		ToStatus:   domain.StatusReceived,
		SourceBIC:  p.SourceBIC,
		DestBIC:    p.DestinationBIC,
		Amount:     p.Amount,
		Currency:   p.Currency,
		Timestamp:  p.CreatedAt,
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

	var fromStatus domain.PaymentStatus
	if err := tx.QueryRow(ctx,
		`UPDATE payment SET status = $1, updated_at = now() WHERE id = $2
		 RETURNING status`, status, id).Scan(&fromStatus); err != nil {
		return err
	}

	if err := r.writeEvent(ctx, tx, id, fromStatus, status); err != nil {
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

	now := time.Now()
	expiresAt := now.Add(ttl)

	if _, err := tx.Exec(ctx,
		`UPDATE payment SET status = $1, reserved_at = $2, expires_at = $3, updated_at = now()
		 WHERE id = $4`,
		domain.StatusReserved, now, expiresAt, id); err != nil {
		return err
	}

	if err := r.writeEvent(ctx, tx, id, domain.StatusValidated, domain.StatusReserved); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *PaymentRepo) writeEvent(ctx context.Context, tx pgx.Tx, paymentID string, from, to domain.PaymentStatus) error {
	var p domain.Payment
	if err := tx.QueryRow(ctx,
		`SELECT id, end_to_end_id, source_bic, destination_bic, amount, currency
		 FROM payment WHERE id = $1`, paymentID).
		Scan(&p.ID, &p.EndToEndID, &p.SourceBIC, &p.DestinationBIC,
			&p.Amount, &p.Currency); err != nil {
		return err
	}

	return outbox.Write(ctx, tx, "payment."+string(to), p.EndToEndID, domain.PaymentEvent{
		PaymentID:  p.ID,
		EndToEndID: p.EndToEndID,
		FromStatus: from,
		ToStatus:   to,
		SourceBIC:  p.SourceBIC,
		DestBIC:    p.DestinationBIC,
		Amount:     p.Amount,
		Currency:   p.Currency,
	})
}

func (r *PaymentRepo) GetByID(ctx context.Context, id string) (*domain.Payment, error) {
	p := &domain.Payment{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, end_to_end_id, source_bic, destination_bic, source_account,
		        dest_account, amount, currency, status, quote_id, reserved_at, expires_at,
		        created_at, updated_at
		 FROM payment WHERE id = $1`, id).
		Scan(&p.ID, &p.EndToEndID, &p.SourceBIC, &p.DestinationBIC,
			&p.SourceAccount, &p.DestAccount, &p.Amount, &p.Currency, &p.Status,
			&p.QuoteID, &p.ReservedAt, &p.ExpiresAt, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return p, nil
}

func (r *PaymentRepo) GetByEndToEndID(ctx context.Context, e2eID string) (*domain.Payment, error) {
	p := &domain.Payment{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, end_to_end_id, source_bic, destination_bic, source_account,
		        dest_account, amount, currency, status, quote_id, reserved_at, expires_at,
		        created_at, updated_at
		 FROM payment WHERE end_to_end_id = $1`, e2eID).
		Scan(&p.ID, &p.EndToEndID, &p.SourceBIC, &p.DestinationBIC,
			&p.SourceAccount, &p.DestAccount, &p.Amount, &p.Currency, &p.Status,
			&p.QuoteID, &p.ReservedAt, &p.ExpiresAt, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return p, nil
}

func (r *PaymentRepo) FindExpiredReservations(ctx context.Context, before time.Time) ([]domain.Reservation, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, source_account, amount, reserved_at, expires_at
		 FROM payment
		 WHERE status = 'RESERVED' AND expires_at < $1
		 FOR UPDATE SKIP LOCKED`, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.Reservation
	for rows.Next() {
		var res domain.Reservation
		if err := rows.Scan(&res.PaymentID, &res.SourceAccount, &res.Amount,
			&res.ReservedAt, &res.ExpiresAt); err != nil {
			return nil, err
		}
		res.Status = "RESERVED"
		result = append(result, res)
	}
	return result, rows.Err()
}
