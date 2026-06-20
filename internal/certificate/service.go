package certificate

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"fmt"

	"switch/internal/participant"
)

type Service struct {
	registry *participant.Registry
}

func New(registry *participant.Registry) *Service {
	return &Service{registry: registry}
}

func (s *Service) RegisterParticipant(ctx context.Context, id, name, bic, account string, cert *x509.Certificate) error {
	hash := sha256.Sum256(cert.Raw)
	certHash := fmt.Sprintf("%x", hash[:])
	return s.registry.Register(&participant.Participant{
		ID:       id,
		Name:     name,
		BIC:      bic,
		Account:  account,
		CertHash: certHash,
	})
}

func (s *Service) LookupParticipant(ctx context.Context, id string) (*participant.Participant, error) {
	return s.registry.GetByID(ctx, id)
}
