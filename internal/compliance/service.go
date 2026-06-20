package compliance

import (
	"context"
	"fmt"

	"switch/internal/orchestrator/domain"
)

type Service struct{}

func New() *Service {
	return &Service{}
}

func (s *Service) Screen(_ context.Context, p *domain.Payment) (domain.ComplianceResult, error) {
	if p.SourceBIC == "" || p.DestinationBIC == "" {
		return domain.ComplianceResult{}, fmt.Errorf("missing BIC for screening")
	}
	return domain.ComplianceResult{Cleared: true, Reason: "auto-cleared"}, nil
}
