package compliance

import (
	"context"

	compliancepb "switch/api/proto/compliance"
	paymentpb "switch/api/proto/payment"
	"switch/internal/orchestrator/domain"
)

type GRPCServer struct {
	compliancepb.UnimplementedComplianceServer
	svc *Service
}

func NewGRPCServer(svc *Service) *GRPCServer {
	return &GRPCServer{svc: svc}
}

func (s *GRPCServer) Screen(ctx context.Context, req *compliancepb.ScreenRequest) (*compliancepb.ScreenResponse, error) {
	p := &domain.Payment{
		ID:             req.Payment.GetId(),
		EndToEndID:     req.Payment.GetEndToEndId(),
		SourceBIC:      req.Payment.GetSourceBic(),
		DestinationBIC: req.Payment.GetDestinationBic(),
		Amount:         req.Payment.GetAmount(),
		Currency:       req.Payment.GetCurrency(),
	}
	result, err := s.svc.Screen(ctx, p)
	if err != nil {
		return nil, err
	}
	return &compliancepb.ScreenResponse{
		Cleared: result.Cleared,
		Reason:  result.Reason,
	}, nil
}

type GRPCClient struct {
	client compliancepb.ComplianceClient
}

func NewGRPCClient(client compliancepb.ComplianceClient) *GRPCClient {
	return &GRPCClient{client: client}
}

func (c *GRPCClient) Screen(ctx context.Context, p *domain.Payment) (domain.ComplianceResult, error) {
	req := &compliancepb.ScreenRequest{
		Payment: &paymentpb.Payment{
			Id:             p.ID,
			EndToEndId:     p.EndToEndID,
			SourceBic:      p.SourceBIC,
			DestinationBic: p.DestinationBIC,
			SourceAccount:  p.SourceAccount,
			DestAccount:    p.DestAccount,
			Amount:         p.Amount,
			Currency:       p.Currency,
		},
	}
	resp, err := c.client.Screen(ctx, req)
	if err != nil {
		return domain.ComplianceResult{}, err
	}
	return domain.ComplianceResult{Cleared: resp.Cleared, Reason: resp.Reason}, nil
}
