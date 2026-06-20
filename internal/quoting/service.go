package quoting

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Quote struct {
	ID                string
	SourceBIC         string
	DestinationBIC    string
	Amount            int64
	Currency          string
	Fee               int64
	Total             int64
	EstimatedLatencyMs int64
	ExpiresAt         time.Time
	Accepted          bool
}

type Service struct {
	mu      sync.Mutex
	quotes  map[string]*Quote
	baseFee int64
}

func New() *Service {
	return &Service{
		quotes:  make(map[string]*Quote),
		baseFee: 25,
	}
}

func (s *Service) GetQuote(ctx context.Context, sourceBIC, destBIC string, amount int64, currency string) (*Quote, error) {
	if sourceBIC == "" || destBIC == "" {
		return nil, fmt.Errorf("source and destination BIC are required")
	}
	if amount <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}

	fee := s.baseFee + int64(amount/10000)
	total := amount + fee

	q := &Quote{
		ID:                uuid.New().String(),
		SourceBIC:         sourceBIC,
		DestinationBIC:    destBIC,
		Amount:            amount,
		Currency:          currency,
		Fee:               fee,
		Total:             total,
		EstimatedLatencyMs: int64(500 + rand.Int63n(3000)),
		ExpiresAt:         time.Now().Add(30 * time.Second),
	}

	s.mu.Lock()
	s.quotes[q.ID] = q
	s.mu.Unlock()

	return q, nil
}

func (s *Service) GetQuoteClient(ctx context.Context, sourceBIC, destBIC string, amount int64, currency string) (string, int64, int64, error) {
	q, err := s.GetQuote(ctx, sourceBIC, destBIC, amount, currency)
	if err != nil {
		return "", 0, 0, err
	}
	return q.ID, q.Fee, q.Total, nil
}

func (s *Service) AcceptQuote(ctx context.Context, quoteID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	q, ok := s.quotes[quoteID]
	if !ok {
		return fmt.Errorf("quote %s not found", quoteID)
	}
	if time.Now().After(q.ExpiresAt) {
		return fmt.Errorf("quote %s has expired", quoteID)
	}
	if q.Accepted {
		return fmt.Errorf("quote %s already accepted", quoteID)
	}
	q.Accepted = true
	return nil
}
