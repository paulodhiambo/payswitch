package notification

import (
	"context"
	"log"
)

type Service struct{}

func New() *Service {
	return &Service{}
}

type NotificationRequest struct {
	ParticipantID string
	Channel       string
	Title         string
	Body          string
	PaymentID     string
	Status        string
}

func (s *Service) Notify(ctx context.Context, req *NotificationRequest) error {
	log.Printf("[NOTIFY] participant=%s channel=%s title=%q payment=%s status=%s",
		req.ParticipantID, req.Channel, req.Title, req.PaymentID, req.Status)
	return nil
}
