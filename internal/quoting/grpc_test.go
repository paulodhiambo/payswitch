package quoting_test

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	quotingpb "switch/api/proto/quoting"
	"switch/internal/quoting"
)

func startTestGRPC(t *testing.T) *grpc.ClientConn {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := grpc.NewServer()
	svc := quoting.New()
	quotingpb.RegisterQuotingServer(srv, quoting.NewGRPCServer(svc))
	go srv.Serve(lis)
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient(
		fmt.Sprintf("passthrough:///%s", lis.Addr().String()),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestGRPC_GetQuote_RoundTrip(t *testing.T) {
	conn := startTestGRPC(t)
	client := quotingpb.NewQuotingClient(conn)

	resp, err := client.GetQuote(context.Background(), &quotingpb.QuoteRequest{
		SourceBic:      "BANKUS33",
		DestinationBic: "BANKDEFF",
		Amount:         100_00,
		Currency:       "USD",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Quote.Id)
	assert.Equal(t, "BANKUS33", resp.Quote.SourceBic)
	assert.Equal(t, "BANKDEFF", resp.Quote.DestinationBic)
	assert.Equal(t, int64(100_00), resp.Quote.Amount)
	assert.Equal(t, "USD", resp.Quote.Currency)
	assert.Greater(t, resp.Quote.Fee, int64(0))
	assert.Greater(t, resp.Quote.EstimatedLatencyMs, int64(0))
	assert.NotEmpty(t, resp.Quote.ExpiresAt)
}

func TestGRPC_AcceptQuote_RoundTrip(t *testing.T) {
	conn := startTestGRPC(t)
	client := quotingpb.NewQuotingClient(conn)

	getResp, err := client.GetQuote(context.Background(), &quotingpb.QuoteRequest{
		SourceBic:      "BANKUS33",
		DestinationBic: "BANKDEFF",
		Amount:         50_00,
		Currency:       "EUR",
	})
	require.NoError(t, err)
	require.NotEmpty(t, getResp.Quote.Id)

	acceptResp, err := client.AcceptQuote(context.Background(), &quotingpb.AcceptQuoteRequest{
		QuoteId: getResp.Quote.Id,
	})
	require.NoError(t, err)
	assert.True(t, acceptResp.Accepted)
}
