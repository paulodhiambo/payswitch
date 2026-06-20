package reconciliation

import (
	"context"
	"fmt"
	"log"
	"sync"
)

type Record struct {
	PaymentID   string
	SourceBIC   string
	DestBIC     string
	Amount      int64
	Currency    string
	Status      string
	Matched     bool
	Discrepancy string
}

type Service struct {
	mu      sync.Mutex
	records map[string]Record
}

func New() *Service {
	return &Service{records: make(map[string]Record)}
}

// AddRecord satisfies ports.ReconciliationClient.
// AddRecord satisfies ports.ReconciliationClient.
func (s *Service) AddRecord(_ context.Context, paymentID, sourceBIC, destBIC string, amount int64, currency, status string) error {
	s.Insert(Record{
		PaymentID: paymentID,
		SourceBIC: sourceBIC,
		DestBIC:   destBIC,
		Amount:    amount,
		Currency:  currency,
		Status:    status,
	})
	return nil
}

// Insert stores a full Record directly; used by the gRPC server and tests.
func (s *Service) Insert(r Record) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[r.PaymentID] = r
}

func (s *Service) Match(paymentID, expectedStatus string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, ok := s.records[paymentID]
	if !ok {
		return fmt.Errorf("payment %s not found in reconciliation records", paymentID)
	}
	if r.Status == expectedStatus {
		r.Matched = true
		s.records[paymentID] = r
		return nil
	}
	msg := fmt.Sprintf("expected %s, got %s", expectedStatus, r.Status)
	r.Discrepancy = msg
	s.records[paymentID] = r
	return fmt.Errorf("%s", msg)
}

func (s *Service) Report() []Record {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]Record, 0, len(s.records))
	unmatched := 0
	for _, r := range s.records {
		result = append(result, r)
		if !r.Matched {
			unmatched++
		}
	}

	log.Printf("[RECON] total=%d unmatched=%d", len(result), unmatched)
	return result
}

func (s *Service) ImportExternal(source string) error {
	log.Printf("[RECON] importing external statement from %s", source)
	return nil
}
