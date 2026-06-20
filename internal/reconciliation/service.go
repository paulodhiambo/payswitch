package reconciliation

import (
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
	mu       sync.Mutex
	records  []Record
}

func New() *Service {
	return &Service{records: make([]Record, 0)}
}

func (s *Service) AddRecord(r Record) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, r)
}

func (s *Service) Match(paymentID, expectedStatus string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, r := range s.records {
		if r.PaymentID == paymentID {
			if r.Status == expectedStatus {
				s.records[i].Matched = true
				return nil
			}
			msg := fmt.Sprintf(
				"expected %s, got %s", expectedStatus, r.Status)
			s.records[i].Discrepancy = msg
			return fmt.Errorf("%s", msg)
		}
	}

	return fmt.Errorf("payment %s not found in reconciliation records", paymentID)
}

func (s *Service) Report() []Record {
	s.mu.Lock()
	defer s.mu.Unlock()

	unmatched := 0
	total := len(s.records)
	for _, r := range s.records {
		if !r.Matched {
			unmatched++
		}
	}

	log.Printf("[RECON] total=%d unmatched=%d", total, unmatched)

	result := make([]Record, len(s.records))
	copy(result, s.records)
	return result
}

func (s *Service) ImportExternal(source string) error {
	log.Printf("[RECON] importing external statement from %s", source)
	return nil
}
