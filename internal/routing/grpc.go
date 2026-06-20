package routing

import (
	"context"

	routingpb "switch/api/proto/routing"
)

type GRPCServer struct {
	routingpb.UnimplementedRoutingServer
	svc *Service
}

func NewGRPCServer(svc *Service) *GRPCServer {
	return &GRPCServer{svc: svc}
}

func (s *GRPCServer) FindRoute(ctx context.Context, req *routingpb.FindRouteRequest) (*routingpb.FindRouteResponse, error) {
	route, err := s.svc.FindRoute(ctx, req.GetSourceBic(), req.GetDestinationBic(), req.GetCurrency())
	if err != nil {
		return nil, err
	}
	return &routingpb.FindRouteResponse{
		Route: &routingpb.Route{
			SourceBic:      route.SourceBIC,
			DestinationBic: route.DestinationBIC,
			Currency:       route.Currency,
			Fee:            route.Fee,
			EstimatedMs:    int32(route.EstimatedMs),
		},
	}, nil
}

func (s *GRPCServer) AddRoute(ctx context.Context, req *routingpb.AddRouteRequest) (*routingpb.AddRouteResponse, error) {
	r := req.GetRoute()
	s.svc.AddRoute(Route{
		SourceBIC:      r.GetSourceBic(),
		DestinationBIC: r.GetDestinationBic(),
		Currency:       r.GetCurrency(),
		Fee:            r.GetFee(),
		EstimatedMs:    int(r.GetEstimatedMs()),
	})
	return &routingpb.AddRouteResponse{}, nil
}
