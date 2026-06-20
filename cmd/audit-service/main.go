package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/segmentio/kafka-go"

	"switch/internal/audit"
	"switch/pkg/config"
	"switch/pkg/eventbus"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := audit.NewEventHandler()

	topics := []string{
		"payment.received",
		"payment.validated",
		"payment.reserved",
		"payment.committed",
		"payment.aborted",
	}

	for _, topic := range topics {
		t := topic
		go func() {
			log.Printf("audit-service consuming %s", t)
			err := eventbus.Consume(ctx, cfg.KafkaBrokers, t, "audit-service",
				func(ctx context.Context, msg kafka.Message) error {
					return handler.HandlePaymentEvent(ctx, t, msg.Value)
				})
			if err != nil && err != context.Canceled {
				log.Printf("consumer %s error: %v", t, err)
			}
		}()
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Print("audit-service shutting down...")
	cancel()
}
