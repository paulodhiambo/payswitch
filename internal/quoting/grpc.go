package quoting

import (
	"context"

	quotingpb "switch/api/proto/quoting"
)

type GRPCServer struct {
	quotingpb.UnimplementedQuotingServer
	svc *Service
}

func NewGRPCServer(svc *Service) *GRPCServer {
	return &GRPCServer{svc: svc}
}

func (s *GRPCServer) GetQuote(ctx context.Context, req *quotingpb.QuoteRequest) (*quotingpb.QuoteResponse, error) {
	q, err := s.svc.GetQuote(ctx, req.GetSourceBic(), req.GetDestinationBic(), req.GetAmount(), req.GetCurrency())
	if err != nil {
		return nil, err
	}
	return &quotingpb.QuoteResponse{
		Quote: &quotingpb.Quote{
			Id:                q.ID,
			SourceBic:         q.SourceBIC,
			DestinationBic:    q.DestinationBIC,
			Amount:            q.Amount,
			Currency:          q.Currency,
			Fee:               q.Fee,
			Total:             q.Total,
			EstimatedLatencyMs: q.EstimatedLatencyMs,
			ExpiresAt:         q.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		},
	}, nil
}

func (s *GRPCServer) AcceptQuote(ctx context.Context, req *quotingpb.AcceptQuoteRequest) (*quotingpb.AcceptQuoteResponse, error) {
	err := s.svc.AcceptQuote(ctx, req.GetQuoteId())
	if err != nil {
		return &quotingpb.AcceptQuoteResponse{Accepted: false}, nil
	}
	return &quotingpb.AcceptQuoteResponse{Accepted: true}, nil
}
