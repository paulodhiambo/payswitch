package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	settlementpb "switch/api/proto/settlement"
	"switch/internal/settlement"
	"switch/pkg/config"
	"switch/pkg/ledger"
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

	logger := telemetry.InitLogger("settlement-service")

	if cfg.OTLPEndpoint != "" {
		tp, err := telemetry.InitTracer(ctx, cfg.OTLPEndpoint, "settlement-service")
		if err != nil {
			logger.Error("failed to init tracer", "error", err)
		} else {
			defer tp.Shutdown(ctx)
		}
	}

	metrics.Listen(cfg.MetricsAddr)

	engine := settlement.NewEngine()
	window := settlement.NewWindow(5*time.Minute, 100)
	engine.AddWindow(window)

	if len(cfg.ScyllaHosts) > 0 {
		store, err := ledger.NewStore(cfg.ScyllaHosts, cfg.ScyllaKeyspace)
		if err != nil {
			logger.Error("connect scylla", "error", err)
		} else {
			window.Ledger = &settlementLedgerAdapter{store: store}
			logger.Info("ledger connected", "hosts", cfg.ScyllaHosts)
		}
	} else {
		logger.Info("no scylla hosts configured — ledger disabled")
	}

	grpcSrv := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	settlementpb.RegisterSettlementServer(grpcSrv, settlement.NewGRPCServer(engine))
	reflection.Register(grpcSrv)

	addr := cfg.GRPCAddr
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Error("listen", "error", err)
		os.Exit(1)
	}

	go func() {
		logger.Info("settlement-service gRPC listening", "addr", addr)
		if err := grpcSrv.Serve(lis); err != nil {
			logger.Error("serve", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("settlement-service shutting down")
	grpcSrv.GracefulStop()
	window.Settle(ctx)
}

type settlementLedgerAdapter struct {
	store *ledger.Store
}

func (a *settlementLedgerAdapter) WriteSettlement(ctx context.Context, bic, paymentID, eventType, payload string, amount int64) error {
	return a.store.Append(ledger.Entry{
		ParticipantID: bic,
		DateBucket:    time.Now().UTC().Format("2006-01-02"),
		PaymentID:     paymentID,
		EventType:     eventType,
		Payload:       payload,
		CreatedAt:     time.Now().UTC(),
	})
}
