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

	routingpb "switch/api/proto/routing"
	"switch/internal/routing"
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := telemetry.InitLogger("routing-service")

	if cfg.OTLPEndpoint != "" {
		tp, err := telemetry.InitTracer(ctx, cfg.OTLPEndpoint, "routing-service")
		if err != nil {
			logger.Error("failed to init tracer", "error", err)
		} else {
			defer tp.Shutdown(ctx)
		}
	}

	metrics.Listen(cfg.MetricsAddr)

	svc := routing.New()
	grpcSrv := grpc.NewServer()
	routingpb.RegisterRoutingServer(grpcSrv, routing.NewGRPCServer(svc))
	reflection.Register(grpcSrv)

	addr := cfg.GRPCAddr
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Error("listen", "error", err)
		os.Exit(1)
	}

	go func() {
		logger.Info("routing-service gRPC listening", "addr", addr)
		if err := grpcSrv.Serve(lis); err != nil {
			logger.Error("serve", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("routing-service shutting down")
	grpcSrv.GracefulStop()
}
