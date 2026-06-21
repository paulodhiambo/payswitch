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
	// All six directional pairs between the three participants, each supporting
	// USD, EUR, and GBP.  Fees are in the minor unit of the payment currency.
	s.routes = []Route{
		// BANKUS33 (US) → BANKDEFF (DE)
		{SourceBIC: "BANKUS33", DestinationBIC: "BANKDEFF", Currency: "USD", Fee: 150, EstimatedMs: 2000},
		{SourceBIC: "BANKUS33", DestinationBIC: "BANKDEFF", Currency: "EUR", Fee: 130, EstimatedMs: 2100},
		{SourceBIC: "BANKUS33", DestinationBIC: "BANKDEFF", Currency: "GBP", Fee: 170, EstimatedMs: 2200},
		// BANKUS33 (US) → BANKGB2L (UK)
		{SourceBIC: "BANKUS33", DestinationBIC: "BANKGB2L", Currency: "USD", Fee: 200, EstimatedMs: 1500},
		{SourceBIC: "BANKUS33", DestinationBIC: "BANKGB2L", Currency: "EUR", Fee: 180, EstimatedMs: 1600},
		{SourceBIC: "BANKUS33", DestinationBIC: "BANKGB2L", Currency: "GBP", Fee: 160, EstimatedMs: 1400},
		// BANKDEFF (DE) → BANKUS33 (US)
		{SourceBIC: "BANKDEFF", DestinationBIC: "BANKUS33", Currency: "USD", Fee: 140, EstimatedMs: 2200},
		{SourceBIC: "BANKDEFF", DestinationBIC: "BANKUS33", Currency: "EUR", Fee: 120, EstimatedMs: 2500},
		{SourceBIC: "BANKDEFF", DestinationBIC: "BANKUS33", Currency: "GBP", Fee: 160, EstimatedMs: 2300},
		// BANKDEFF (DE) → BANKGB2L (UK)
		{SourceBIC: "BANKDEFF", DestinationBIC: "BANKGB2L", Currency: "USD", Fee: 190, EstimatedMs: 1800},
		{SourceBIC: "BANKDEFF", DestinationBIC: "BANKGB2L", Currency: "EUR", Fee: 110, EstimatedMs: 1600},
		{SourceBIC: "BANKDEFF", DestinationBIC: "BANKGB2L", Currency: "GBP", Fee: 140, EstimatedMs: 1700},
		// BANKGB2L (UK) → BANKUS33 (US)
		{SourceBIC: "BANKGB2L", DestinationBIC: "BANKUS33", Currency: "USD", Fee: 210, EstimatedMs: 1500},
		{SourceBIC: "BANKGB2L", DestinationBIC: "BANKUS33", Currency: "EUR", Fee: 190, EstimatedMs: 1600},
		{SourceBIC: "BANKGB2L", DestinationBIC: "BANKUS33", Currency: "GBP", Fee: 180, EstimatedMs: 1800},
		// BANKGB2L (UK) → BANKDEFF (DE)
		{SourceBIC: "BANKGB2L", DestinationBIC: "BANKDEFF", Currency: "USD", Fee: 175, EstimatedMs: 1700},
		{SourceBIC: "BANKGB2L", DestinationBIC: "BANKDEFF", Currency: "EUR", Fee: 125, EstimatedMs: 1500},
		{SourceBIC: "BANKGB2L", DestinationBIC: "BANKDEFF", Currency: "GBP", Fee: 155, EstimatedMs: 1600},
	}
}

// FindRoute satisfies ports.RoutingClient, returning only the fee and latency estimate.
func (s *Service) FindRoute(ctx context.Context, sourceBIC, destBIC, currency string) (int64, int, error) {
	route, err := s.LookupRoute(ctx, sourceBIC, destBIC, currency)
	if err != nil {
		return 0, 0, err
	}
	return route.Fee, route.EstimatedMs, nil
}

// LookupRoute returns the full Route struct; used by the gRPC server and tests.
func (s *Service) LookupRoute(_ context.Context, sourceBIC, destBIC, currency string) (*Route, error) {
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
	for i, existing := range s.routes {
		if existing.SourceBIC == r.SourceBIC && existing.DestinationBIC == r.DestinationBIC && existing.Currency == r.Currency {
			s.routes[i] = r
			return
		}
	}
	s.routes = append(s.routes, r)
}
