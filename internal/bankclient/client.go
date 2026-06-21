package bankclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"switch/internal/orchestrator/ports"
)

type Client struct {
	provider ConfigProvider
	http     *http.Client
}

func New(provider ConfigProvider) *Client {
	return &Client{
		provider: provider,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) Reserve(ctx context.Context, bic, account string, amount int64) error {
	return c.callBank(ctx, bic, "reserve", map[string]any{
		"account": account,
		"amount":  amount,
	})
}

func (c *Client) ReleaseReservation(ctx context.Context, bic, account string, amount int64) error {
	return c.callBank(ctx, bic, "release", map[string]any{
		"account": account,
		"amount":  amount,
	})
}

func (c *Client) Credit(ctx context.Context, bic, account string, amount int64) error {
	return c.callBank(ctx, bic, "credit", map[string]any{
		"account": account,
		"amount":  amount,
	})
}

func (c *Client) ReverseCredit(ctx context.Context, bic, account string, amount int64) error {
	return c.callBank(ctx, bic, "reverse-credit", map[string]any{
		"account": account,
		"amount":  amount,
	})
}

type bankResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

func (c *Client) callBank(ctx context.Context, bic, operation string, body any) error {
	cfg, err := c.provider.GetBankAPIConfig(ctx, bic)
	if err != nil {
		return fmt.Errorf("bank %s: get config: %w", bic, err)
	}
	if !cfg.Enabled {
		return fmt.Errorf("bank %s: API not enabled", bic)
	}
	if cfg.BaseURL == "" {
		return fmt.Errorf("bank %s: base URL not configured", bic)
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("bank %s: marshal request: %w", bic, err)
	}

	targetURL := cfg.BaseURL + "/" + operation
	if cfg.PaymentURL != "" {
		targetURL = cfg.PaymentURL + "/" + operation
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("bank %s: create request: %w", bic, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("X-API-Key", cfg.APIKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("bank %s: %s request failed: %w", bic, operation, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("bank %s: %s returned %d: %s", bic, operation, resp.StatusCode, string(respBody))
	}

	var br bankResponse
	if err := json.Unmarshal(respBody, &br); err != nil {
		return fmt.Errorf("bank %s: %s decode response: %w", bic, operation, err)
	}
	if br.Status == "error" {
		return fmt.Errorf("bank %s: %s error: %s", bic, operation, br.Message)
	}

	return nil
}

// compile-time check that Client implements ports.BankClient
var _ ports.BankClient = (*Client)(nil)
