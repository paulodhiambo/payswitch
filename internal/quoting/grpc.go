package quoting

import (
	"context"
	"time"

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
	q, err := s.svc.getQuote(ctx, req.GetSourceBic(), req.GetDestinationBic(), req.GetAmount(), req.GetCurrency())
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

type GRPCClient struct {
	client quotingpb.QuotingClient
}

func NewGRPCClient(client quotingpb.QuotingClient) *GRPCClient {
	return &GRPCClient{client: client}
}

func (c *GRPCClient) GetQuote(ctx context.Context, sourceBIC, destBIC string, amount int64, currency string) (string, int64, int64, error) {
	resp, err := c.client.GetQuote(ctx, &quotingpb.QuoteRequest{
		SourceBic:      sourceBIC,
		DestinationBic: destBIC,
		Amount:         amount,
		Currency:       currency,
	})
	if err != nil {
		return "", 0, 0, err
	}
	q := resp.GetQuote()
	expiresAt, _ := time.Parse("2006-01-02T15:04:05Z07:00", q.GetExpiresAt())
	_ = expiresAt
	return q.GetId(), q.GetFee(), q.GetTotal(), nil
}

func (s *GRPCServer) AcceptQuote(ctx context.Context, req *quotingpb.AcceptQuoteRequest) (*quotingpb.AcceptQuoteResponse, error) {
	err := s.svc.AcceptQuote(ctx, req.GetQuoteId())
	if err != nil {
		return &quotingpb.AcceptQuoteResponse{Accepted: false}, nil
	}
	return &quotingpb.AcceptQuoteResponse{Accepted: true}, nil
}
