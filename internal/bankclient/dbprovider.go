package bankclient

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DBProvider struct {
	pool *pgxpool.Pool
}

func NewDBProvider(pool *pgxpool.Pool) *DBProvider {
	return &DBProvider{pool: pool}
}

func (p *DBProvider) GetBankAPIConfig(ctx context.Context, bic string) (*BankAPIConfig, error) {
	var cfg BankAPIConfig
	err := p.pool.QueryRow(ctx,
		`SELECT COALESCE(api_base_url, ''), COALESCE(api_key, ''), api_enabled,
		        COALESCE(lookup_api_url, ''), COALESCE(payment_api_url, ''), COALESCE(status_check_api_url, '')
		 FROM bank WHERE bic = $1`, bic,
	).Scan(&cfg.BaseURL, &cfg.APIKey, &cfg.Enabled, &cfg.LookupURL, &cfg.PaymentURL, &cfg.StatusCheckURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return &BankAPIConfig{Enabled: false}, nil
	}
	return &cfg, err
}
