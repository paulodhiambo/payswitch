package notification

import (
	"context"

	notificationpb "switch/api/proto/notification"
)

type GRPCServer struct {
	notificationpb.UnimplementedNotificationServer
	svc *Service
}

func NewGRPCServer(svc *Service) *GRPCServer {
	return &GRPCServer{svc: svc}
}

type GRPCClient struct {
	client notificationpb.NotificationClient
}

func NewGRPCClient(client notificationpb.NotificationClient) *GRPCClient {
	return &GRPCClient{client: client}
}

func (c *GRPCClient) Notify(ctx context.Context, participantID, channel, title, body, paymentID, status string) error {
	_, err := c.client.Notify(ctx, &notificationpb.NotificationRequest{
		ParticipantId: participantID,
		Channel:       channel,
		Title:         title,
		Body:          body,
		PaymentId:     paymentID,
		Status:        status,
	})
	return err
}

func (s *GRPCServer) Notify(ctx context.Context, req *notificationpb.NotificationRequest) (*notificationpb.NotificationResponse, error) {
	err := s.svc.notify(ctx, &NotificationRequest{
		ParticipantID: req.GetParticipantId(),
		Channel:       req.GetChannel(),
		Title:         req.GetTitle(),
		Body:          req.GetBody(),
		PaymentID:     req.GetPaymentId(),
		Status:        req.GetStatus(),
	})
	if err != nil {
		return &notificationpb.NotificationResponse{Sent: false}, nil
	}
	return &notificationpb.NotificationResponse{Sent: true}, nil
}
