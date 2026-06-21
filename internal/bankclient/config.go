package bankclient

import "context"

type BankAPIConfig struct {
	BaseURL        string
	APIKey         string
	Enabled        bool
	LookupURL      string
	PaymentURL     string
	StatusCheckURL string
}

type ConfigProvider interface {
	GetBankAPIConfig(ctx context.Context, bic string) (*BankAPIConfig, error)
}
