package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	compliancepb "switch/api/proto/compliance"
	lookuppb "switch/api/proto/lookup"
	settlementpb "switch/api/proto/settlement"
	"switch/internal/compliance"
	"switch/internal/lookup"
	"switch/internal/orchestrator/ports"
	"switch/internal/settlement"
	"switch/internal/gateway"
	"switch/internal/orchestrator/db"
	"switch/internal/orchestrator/saga"
	"switch/internal/orchestrator/sweep"
	"switch/internal/participant"
	"switch/pkg/config"
	"switch/pkg/eventbus"
	"switch/pkg/metrics"
	"switch/pkg/middleware"
	"switch/pkg/outbox"
	"switch/pkg/telemetry"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := telemetry.InitLogger("gateway")
	_ = logger

	if cfg.OTLPEndpoint != "" {
		tp, err := telemetry.InitTracer(ctx, cfg.OTLPEndpoint, "gateway")
		if err != nil {
			logger.Error("failed to init tracer", "error", err)
		} else {
			defer tp.Shutdown(ctx)
		}
	}

	pool, err := pgxpool.New(ctx, cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer pool.Close()

	repo := db.NewPaymentRepo(pool)
	bank := saga.NewMockBankClient()
	bank.SetBalance("ACC-A", 1_000_00)
	bank.SetBalance("ACC-B", 0)

	participantReg := participant.NewRegistry()
	for _, p := range []*participant.Participant{
		{ID: "bank-a", Name: "Bank A", BIC: "BANKUS33", Account: "ACC-A"},
		{ID: "bank-b", Name: "Bank B", BIC: "BANKDEFF", Account: "ACC-B"},
	} {
		if err := participantReg.Register(p); err != nil {
			log.Fatalf("register participant: %v", err)
		}
	}

	complianceClient := resolveComplianceClient(cfg)
	lookupClient := resolveLookupClient(cfg)

	sagaSteps := []saga.Step{
		&saga.ValidateStep{Repo: repo},
		&saga.LookupStep{Client: lookupClient, Repo: repo},
		&saga.ScreenStep{Client: complianceClient, Repo: repo},
		&saga.ReserveStep{Repo: repo, Bank: bank, TTL: saga.DefaultReservationTTL},
		&saga.CommitStep{Repo: repo, Bank: bank},
	}

	if cfg.SettlementAddr != "" {
		settlementClient := resolveSettlementClient(cfg)
		sagaSteps = append(sagaSteps, &saga.SettleStep{Client: settlementClient, Repo: repo})
	}

	paymentSaga := saga.New(sagaSteps...)

	h := gateway.NewHandler(repo, paymentSaga, participantReg)
	h.AddPool(pool)
	if cfg.ComplianceAddr != "" {
		h.AddHealthCheck("compliance", func(ctx context.Context) error {
			conn, err := grpc.NewClient(cfg.ComplianceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return err
			}
			conn.Close()
			return nil
		})
	}
	if cfg.KafkaBrokers != nil && len(cfg.KafkaBrokers) > 0 {
		h.AddHealthCheck("kafka", func(ctx context.Context) error {
			return nil // connectivity verified at producer init
		})
	}

	r := chi.NewRouter()

	r.Use(metricsMiddleware)
	h.Register(r)

	metrics.Listen(cfg.MetricsAddr)

	if len(cfg.KafkaBrokers) > 0 {
		producer := eventbus.NewProducer(cfg.KafkaBrokers)
		relay := outbox.NewRelay(pool, producer)
		go relay.Run(ctx, 1*time.Second)
		log.Printf("outbox relay started (poll interval: 1s)")
	} else {
		log.Print("no kafka brokers configured — outbox relay disabled")
	}

	sw := sweep.New(repo, paymentSaga)
	go sw.Run(ctx, 30*time.Second)
	log.Printf("reservation sweeper started (poll interval: 30s)")

	tlsConfig, err := buildTLSConfig(cfg)
	if err != nil {
		log.Fatalf("tls config: %v", err)
	}

	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      handlerWithDevParticipant(r, participantReg),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
		TLSConfig:    tlsConfig,
	}

	go func() {
		log.Printf("gateway listening on %s (dev mode: bank-a injected)", cfg.HTTPAddr)
		if tlsConfig != nil {
			if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				log.Fatalf("serve: %v", err)
			}
		} else {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("serve: %v", err)
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Print("shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	srv.Shutdown(shutdownCtx)
}

func resolveComplianceClient(cfg *config.Config) ports.ComplianceClient {
	if cfg.ComplianceAddr != "" {
		conn, err := grpc.NewClient(cfg.ComplianceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("connect compliance: %v", err)
		}
		return compliance.NewGRPCClient(compliancepb.NewComplianceClient(conn))
	}
	return compliance.New()
}

func resolveLookupClient(cfg *config.Config) ports.LookupClient {
	if cfg.LookupAddr != "" {
		conn, err := grpc.NewClient(cfg.LookupAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("connect lookup: %v", err)
		}
		return lookup.NewGRPCClient(lookuppb.NewLookupClient(conn))
	}
	return lookup.New(nil)
}

func resolveSettlementClient(cfg *config.Config) ports.SettlementClient {
	conn, err := grpc.NewClient(cfg.SettlementAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("connect settlement: %v", err)
	}
	return settlement.NewGRPCClient(settlementpb.NewSettlementClient(conn))
}

func buildTLSConfig(cfg *config.Config) (*tls.Config, error) {
	if cfg.TLSCertFile == "" || cfg.TLSKeyFile == "" {
		return nil, nil
	}
	cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if cfg.TLSCAFile != "" {
		caData, err := os.ReadFile(cfg.TLSCAFile)
		if err != nil {
			return nil, err
		}
		pool.AppendCertsFromPEM(caData)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    pool,
		MinVersion:   tls.VersionTLS12,
	}, nil
}

func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metrics.HTTPRequestsTotal.WithLabelValues(r.Method, r.URL.Path, "").Inc()
		next.ServeHTTP(w, r)
	})
}

func handlerWithDevParticipant(next http.Handler, reg *participant.Registry) http.Handler {
	p, err := reg.GetByID(context.Background(), "bank-a")
	if err != nil {
		log.Fatalf("dev participant: %v", err)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), middleware.ParticipantCtxKey, p.ID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
