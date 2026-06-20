package main

import (
	"context"
	"log/slog"
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
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.With("service", "audit-service")
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
			logger.Info("audit-service consuming", "topic", t)
			err := eventbus.Consume(ctx, cfg.KafkaBrokers, t, "audit-service",
				func(ctx context.Context, msg kafka.Message) error {
					return handler.HandlePaymentEvent(ctx, t, msg.Value)
				})
			if err != nil && err != context.Canceled {
				logger.Error("consumer error", "topic", t, "error", err)
			}
		}()
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("audit-service shutting down")
	cancel()
}
