package notification_test

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	notificationpb "switch/api/proto/notification"
	"switch/internal/notification"
)

func startTestGRPC(t *testing.T) *grpc.ClientConn {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := grpc.NewServer()
	svc := notification.New()
	notificationpb.RegisterNotificationServer(srv, notification.NewGRPCServer(svc))
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

func TestGRPC_Notify_RoundTrip(t *testing.T) {
	conn := startTestGRPC(t)
	client := notificationpb.NewNotificationClient(conn)

	resp, err := client.Notify(context.Background(), &notificationpb.NotificationRequest{
		ParticipantId: "bank-a",
		Channel:       "webhook",
		Title:         "Payment Received",
		Body:          "Your payment of USD 100.00 has been received.",
		PaymentId:     "pay_123",
		Status:        "RECEIVED",
	})
	require.NoError(t, err)
	assert.True(t, resp.Sent)
}
