package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"switch/internal/orchestrator/domain"
)

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

func (r *PaymentRepo) UpdateStatus(ctx context.Context, id string, status domain.PaymentStatus) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE payment SET status = $1, updated_at = now() WHERE id = $2`,
		status, id)
	return err
}

func (r *PaymentRepo) GetByEndToEndID(ctx context.Context, e2eID string) (*domain.Payment, error) {
	p := &domain.Payment{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, end_to_end_id, source_bic, destination_bic, source_account,
		        dest_account, amount, currency, status, reserved_at, expires_at,
		        created_at, updated_at
		 FROM payment WHERE end_to_end_id = $1`, e2eID).
		Scan(&p.ID, &p.EndToEndID, &p.SourceBIC, &p.DestinationBIC,
			&p.SourceAccount, &p.DestAccount, &p.Amount, &p.Currency, &p.Status,
			&p.ReservedAt, &p.ExpiresAt, &p.CreatedAt, &p.UpdatedAt)
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
