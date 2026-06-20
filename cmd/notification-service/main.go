package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	notificationpb "switch/api/proto/notification"
	"switch/internal/notification"
	"switch/pkg/config"
	"switch/pkg/metrics"
	"switch/pkg/telemetry"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := telemetry.InitLogger("notification-service")

	metrics.Listen(cfg.MetricsAddr)

	svc := notification.New()

	grpcSrv := grpc.NewServer()
	notificationpb.RegisterNotificationServer(grpcSrv, notification.NewGRPCServer(svc))
	reflection.Register(grpcSrv)

	addr := cfg.GRPCAddr
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Error("listen", "error", err)
		os.Exit(1)
	}

	go func() {
		logger.Info("notification-service gRPC listening", "addr", addr)
		if err := grpcSrv.Serve(lis); err != nil {
			logger.Error("serve", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("notification-service shutting down")
	grpcSrv.GracefulStop()
}
