package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	reconciliationpb "switch/api/proto/reconciliation"
	"switch/internal/reconciliation"
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

	logger := telemetry.InitLogger("reconciliation-service")

	if cfg.OTLPEndpoint != "" {
		tp, err := telemetry.InitTracer(ctx, cfg.OTLPEndpoint, "reconciliation-service")
		if err != nil {
			logger.Error("failed to init tracer", "error", err)
		} else {
			defer tp.Shutdown(ctx)
		}
	}

	metrics.Listen(cfg.MetricsAddr)

	svc := reconciliation.New()
	grpcSrv := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	reconciliationpb.RegisterReconciliationServer(grpcSrv, reconciliation.NewGRPCServer(svc))
	reflection.Register(grpcSrv)

	addr := cfg.GRPCAddr
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Error("listen", "error", err)
		os.Exit(1)
	}

	go func() {
		logger.Info("reconciliation-service gRPC listening", "addr", addr)
		if err := grpcSrv.Serve(lis); err != nil {
			logger.Error("serve", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("reconciliation-service shutting down")
	grpcSrv.GracefulStop()
}
