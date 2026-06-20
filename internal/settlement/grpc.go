package settlement

import (
	"context"

	settlementpb "switch/api/proto/settlement"
	paymentpb "switch/api/proto/payment"
	"switch/internal/orchestrator/domain"
)

type GRPCServer struct {
	settlementpb.UnimplementedSettlementServer
	engine *Engine
}

func NewGRPCServer(engine *Engine) *GRPCServer {
	return &GRPCServer{engine: engine}
}

func (s *GRPCServer) SubmitForSettlement(ctx context.Context, req *paymentpb.Payment) (*settlementpb.SettlementAck, error) {
	s.engine.windows[0].Add(ctx, SettlementEntry{
		PaymentID:   req.GetId(),
		SourceBIC:   req.GetSourceBic(),
		DestBIC:     req.GetDestinationBic(),
		Amount:      req.GetAmount(),
		Currency:    req.GetCurrency(),
	})
	return &settlementpb.SettlementAck{Accepted: true}, nil
}

func (s *GRPCServer) SettleWindow(ctx context.Context, req *settlementpb.SettleWindowRequest) (*settlementpb.SettleWindowResponse, error) {
	positions, err := s.engine.windows[0].Settle(ctx)
	if err != nil {
		return nil, err
	}
	var pbPositions []*settlementpb.NetPosition
	for _, pos := range positions {
		pbPositions = append(pbPositions, &settlementpb.NetPosition{
			Bic:      pos.BIC,
			Amount:   pos.Amount,
			Currency: pos.Currency,
		})
	}
	return &settlementpb.SettleWindowResponse{Positions: pbPositions}, nil
}

func (s *GRPCServer) GetNetPositions(ctx context.Context, req *settlementpb.NetPositionRequest) (*settlementpb.NetPositionResponse, error) {
	return nil, nil
}

type GRPCClient struct {
	client settlementpb.SettlementClient
}

func NewGRPCClient(client settlementpb.SettlementClient) *GRPCClient {
	return &GRPCClient{client: client}
}

func (c *GRPCClient) Submit(ctx context.Context, p *domain.Payment) error {
	_, err := c.client.SubmitForSettlement(ctx, &paymentpb.Payment{
		Id:             p.ID,
		EndToEndId:     p.EndToEndID,
		SourceBic:      p.SourceBIC,
		DestinationBic: p.DestinationBIC,
		SourceAccount:  p.SourceAccount,
		DestAccount:    p.DestAccount,
		Amount:         p.Amount,
		Currency:       p.Currency,
	})
	return err
}
