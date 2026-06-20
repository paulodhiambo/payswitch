package lookup

import (
	"context"

	lookuppb "switch/api/proto/lookup"
)

type GRPCServer struct {
	lookuppb.UnimplementedLookupServer
	svc *Service
}

func NewGRPCServer(svc *Service) *GRPCServer {
	return &GRPCServer{svc: svc}
}

func (s *GRPCServer) ResolveBIC(ctx context.Context, req *lookuppb.BICRequest) (*lookuppb.BICResponse, error) {
	info, err := s.svc.Lookup(ctx, req.Bic)
	if err != nil {
		return nil, err
	}
	return &lookuppb.BICResponse{
		Bic:                 info.BIC,
		Name:                info.Name,
		Country:             info.Country,
		SupportedCurrencies: info.Supported,
		RoutingInfo:         info.RoutingInfo,
	}, nil
}

func (s *GRPCServer) SearchBanks(ctx context.Context, req *lookuppb.SearchRequest) (*lookuppb.SearchResponse, error) {
	return nil, nil
}

type GRPCClient struct {
	client lookuppb.LookupClient
}

func NewGRPCClient(client lookuppb.LookupClient) *GRPCClient {
	return &GRPCClient{client: client}
}

func (c *GRPCClient) ResolveBIC(ctx context.Context, bic string) (string, string, error) {
	resp, err := c.client.ResolveBIC(ctx, &lookuppb.BICRequest{Bic: bic})
	if err != nil {
		return "", "", err
	}
	return resp.Name, resp.Country, nil
}

func (c *GRPCClient) ResolveBICFull(ctx context.Context, bic string) (*BankInfo, error) {
	resp, err := c.client.ResolveBIC(ctx, &lookuppb.BICRequest{Bic: bic})
	if err != nil {
		return nil, err
	}
	return &BankInfo{
		BIC:         resp.Bic,
		Name:        resp.Name,
		Country:     resp.Country,
		Supported:   resp.SupportedCurrencies,
		RoutingInfo: resp.RoutingInfo,
	}, nil
}
