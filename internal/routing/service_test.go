package routing_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"switch/internal/routing"
)

func TestFindRoute_Found(t *testing.T) {
	svc := routing.New()
	route, err := svc.FindRoute(context.Background(), "BANKUS33", "BANKDEFF", "USD")
	require.NoError(t, err)
	require.Equal(t, "BANKUS33", route.SourceBIC)
	require.Equal(t, "BANKDEFF", route.DestinationBIC)
	require.Equal(t, int64(150), route.Fee)
}

func TestFindRoute_NotFound(t *testing.T) {
	svc := routing.New()
	_, err := svc.FindRoute(context.Background(), "BANKUS33", "NONEXIST", "USD")
	require.Error(t, err)
}

func TestFindRoute_WrongCurrency(t *testing.T) {
	svc := routing.New()
	_, err := svc.FindRoute(context.Background(), "BANKUS33", "BANKDEFF", "GBP")
	require.Error(t, err)
}

func TestAddRoute(t *testing.T) {
	svc := routing.New()
	svc.AddRoute(routing.Route{
		SourceBIC: "BANKUS33", DestinationBIC: "BANKDEFF",
		Currency: "GBP", Fee: 300, EstimatedMs: 3000,
	})

	route, err := svc.FindRoute(context.Background(), "BANKUS33", "BANKDEFF", "GBP")
	require.NoError(t, err)
	require.Equal(t, int64(300), route.Fee)
}
