package routing

import (
	"context"
	"fmt"
	"sync"
)

type Route struct {
	SourceBIC      string `json:"source_bic"`
	DestinationBIC string `json:"destination_bic"`
	Currency       string `json:"currency"`
	Fee            int64  `json:"fee"`
	EstimatedMs    int    `json:"estimated_ms"`
}

type BankLookup interface {
	Lookup(ctx context.Context, bic string) (interface {
		GetSupported() []string
	}, error)
}

type Service struct {
	mu     sync.RWMutex
	routes []Route
}

func New() *Service {
	s := &Service{}
	s.seedDefaults()
	return s
}

func (s *Service) seedDefaults() {
	s.routes = []Route{
		{SourceBIC: "BANKUS33", DestinationBIC: "BANKDEFF", Currency: "USD", Fee: 150, EstimatedMs: 2000},
		{SourceBIC: "BANKUS33", DestinationBIC: "BANKGB2L", Currency: "USD", Fee: 200, EstimatedMs: 1500},
		{SourceBIC: "BANKDEFF", DestinationBIC: "BANKUS33", Currency: "EUR", Fee: 120, EstimatedMs: 2500},
		{SourceBIC: "BANKGB2L", DestinationBIC: "BANKUS33", Currency: "GBP", Fee: 180, EstimatedMs: 1800},
	}
}

func (s *Service) FindRoute(_ context.Context, sourceBIC, destBIC, currency string) (*Route, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, r := range s.routes {
		if r.SourceBIC == sourceBIC && r.DestinationBIC == destBIC && r.Currency == currency {
			return &r, nil
		}
	}

	return nil, fmt.Errorf("no route from %s to %s for %s", sourceBIC, destBIC, currency)
}

func (s *Service) AddRoute(r Route) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.routes = append(s.routes, r)
}
