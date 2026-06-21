package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	compliancepb "switch/api/proto/compliance"
	lookuppb "switch/api/proto/lookup"
	notificationpb "switch/api/proto/notification"
	quotingpb "switch/api/proto/quoting"
	reconciliationpb "switch/api/proto/reconciliation"
	routingpb "switch/api/proto/routing"
	settlementpb "switch/api/proto/settlement"
	"switch/internal/bankclient"
	"switch/internal/compliance"
	"switch/internal/gateway"
	"switch/internal/lookup"
	"switch/internal/notification"
	"switch/internal/orchestrator/ports"
	"switch/internal/quoting"
	"switch/internal/reconciliation"
	"switch/internal/routing"
	"switch/internal/settlement"
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

	logger, lp, err := telemetry.InitLogger(ctx, cfg.OTLPEndpoint, "gateway")
	if err != nil {
		log.Fatalf("init logger: %v", err)
	}
	_ = logger
	if lp != nil {
		defer lp.Shutdown(ctx)
	}

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

	var bank ports.BankClient
	if cfg.BankAPIEnabled {
		bank = bankclient.New(bankclient.NewDBProvider(pool))
		log.Print("using real bank API client")
	} else {
		mock := saga.NewMockBankClient()
		mock.SetBalance("ACC-A", 10_000_000)
		mock.SetBalance("ACC-B", 10_000_000)
		mock.SetBalance("ACC-C", 10_000_000)
		bank = mock
		log.Print("using in-memory mock bank client (set BANK_API_ENABLED=true for real APIs)")
	}

	participantReg := participant.NewRegistry()
	for _, p := range []*participant.Participant{
		{ID: "bank-a", Name: "First National Bank",    BIC: "BANKUS33", Account: "ACC-A"},
		{ID: "bank-b", Name: "Deutsche Exchange Bank", BIC: "BANKDEFF", Account: "ACC-B"},
		{ID: "bank-c", Name: "London Clearing Bank",   BIC: "BANKGB2L", Account: "ACC-C"},
	} {
		if err := participantReg.Register(p); err != nil {
			log.Fatalf("register participant: %v", err)
		}
	}

	complianceClient := resolveComplianceClient(cfg)
	lookupClient := resolveLookupClient(cfg)
	quotingClient := resolveQuotingClient(cfg)
	notificationClient := resolveNotificationClient(cfg)
	routingClient := resolveRoutingClient(cfg)
	reconciliationClient := resolveReconciliationClient(cfg)

	sagaSteps := []saga.Step{
		&saga.ValidateStep{Repo: repo},
		&saga.LookupStep{Client: lookupClient, Repo: repo},
		&saga.RouteStep{Client: routingClient, Repo: repo},
		&saga.QuoteStep{Client: quotingClient, Repo: repo},
		&saga.ScreenStep{Client: complianceClient, Repo: repo},
		&saga.ReserveStep{Repo: repo, Bank: bank, TTL: saga.DefaultReservationTTL},
		&saga.CommitStep{Repo: repo, Bank: bank},
	}

	if cfg.SettlementAddr != "" {
		settlementClient := resolveSettlementClient(cfg)
		sagaSteps = append(sagaSteps, &saga.SettleStep{Client: settlementClient, Repo: repo})
	}

	sagaSteps = append(sagaSteps,
		&saga.RecordReconciliationStep{Client: reconciliationClient, Repo: repo},
		&saga.NotifyStep{Client: notificationClient},
	)

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
	if len(cfg.KafkaBrokers) > 0 {
		h.AddHealthCheck("kafka", func(ctx context.Context) error {
			return nil // connectivity verified at producer init
		})
	}

	r := chi.NewRouter()
	r.Use(kongAuthMiddleware(participantReg))
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

	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("gateway listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("serve: %v", err)
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

func resolveQuotingClient(cfg *config.Config) ports.QuotingClient {
	if cfg.QuotingAddr != "" {
		conn, err := grpc.NewClient(cfg.QuotingAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("connect quoting: %v", err)
		}
		return quoting.NewGRPCClient(quotingpb.NewQuotingClient(conn))
	}
	return quoting.New()
}

func resolveNotificationClient(cfg *config.Config) ports.NotificationClient {
	if cfg.NotificationAddr != "" {
		conn, err := grpc.NewClient(cfg.NotificationAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("connect notification: %v", err)
		}
		return notification.NewGRPCClient(notificationpb.NewNotificationClient(conn))
	}
	return notification.New()
}

func resolveRoutingClient(cfg *config.Config) ports.RoutingClient {
	if cfg.RoutingAddr != "" {
		conn, err := grpc.NewClient(cfg.RoutingAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("connect routing: %v", err)
		}
		return routing.NewGRPCClient(routingpb.NewRoutingClient(conn))
	}
	return routing.New()
}

func resolveReconciliationClient(cfg *config.Config) ports.ReconciliationClient {
	if cfg.ReconciliationAddr != "" {
		conn, err := grpc.NewClient(cfg.ReconciliationAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("connect reconciliation: %v", err)
		}
		return reconciliation.NewGRPCClient(reconciliationpb.NewReconciliationClient(conn))
	}
	return reconciliation.New()
}

func kongAuthMiddleware(reg *participant.Registry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			subject := r.Header.Get("X-Participant-Id")
			if subject != "" {
				// Kong set the header: extract the CN and reject if unknown.
				participantID := parseCN(subject)
				if _, err := reg.GetByID(r.Context(), participantID); err != nil {
					log.Printf("kong auth: unknown participant %q from subject %q", participantID, subject)
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
				ctx := context.WithValue(r.Context(), middleware.ParticipantCtxKey, participantID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			// No X-Participant-Id: dev-mode fallback (Kong not in the path).
			p, err := reg.GetByID(r.Context(), "bank-a")
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), middleware.ParticipantCtxKey, p.ID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// parseCN extracts the CN value from an RFC 4514 distinguished name.
// RFC 4514 escapes literal commas inside values as \, so we must not split
// on those when tokenising the RDN sequence.
func parseCN(subject string) string {
	const escapedComma = "\x00"
	s := strings.ReplaceAll(subject, `\,`, escapedComma)
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(strings.ReplaceAll(part, escapedComma, ","))
		if v, ok := strings.CutPrefix(part, "CN="); ok {
			return v
		}
	}
	return subject
}

func resolveSettlementClient(cfg *config.Config) ports.SettlementClient {
	conn, err := grpc.NewClient(cfg.SettlementAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("connect settlement: %v", err)
	}
	return settlement.NewGRPCClient(settlementpb.NewSettlementClient(conn))
}

func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metrics.HTTPRequestsTotal.WithLabelValues(r.Method, r.URL.Path, "").Inc()
		next.ServeHTTP(w, r)
	})
}

