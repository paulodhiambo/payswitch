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

	lookuppb "switch/api/proto/lookup"
	"switch/internal/lookup"
	"switch/pkg/cache"
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

	logger := telemetry.InitLogger("lookup-service")

	if cfg.OTLPEndpoint != "" {
		tp, err := telemetry.InitTracer(ctx, cfg.OTLPEndpoint, "lookup-service")
		if err != nil {
			logger.Error("failed to init tracer", "error", err)
		} else {
			defer tp.Shutdown(ctx)
		}
	}

	var svc *lookup.Service
	if cfg.RedisAddr != "" {
		svc = lookup.New(cache.New(cfg.RedisAddr))
		logger.Info("lookup-service with Redis cache", "addr", cfg.RedisAddr)
	} else {
		svc = lookup.New(nil)
		logger.Info("lookup-service started (no cache)")
	}

	metrics.Listen(cfg.MetricsAddr)

	grpcSrv := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	lookuppb.RegisterLookupServer(grpcSrv, lookup.NewGRPCServer(svc))
	reflection.Register(grpcSrv)

	addr := cfg.GRPCAddr
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Error("listen", "error", err)
		os.Exit(1)
	}

	go func() {
		logger.Info("lookup-service gRPC listening", "addr", addr)
		if err := grpcSrv.Serve(lis); err != nil {
			logger.Error("serve", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("lookup-service shutting down")
	grpcSrv.GracefulStop()
}
