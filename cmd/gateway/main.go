package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"switch/internal/compliance"
	"switch/internal/gateway"
	"switch/internal/orchestrator/db"
	"switch/internal/orchestrator/saga"
	"switch/internal/orchestrator/sweep"
	"switch/internal/participant"
	"switch/pkg/config"
	"switch/pkg/eventbus"
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

	complianceClient := compliance.New()

	paymentSaga := saga.New(
		&saga.ValidateStep{Repo: repo},
		&saga.ScreenStep{Client: complianceClient, Repo: repo},
		&saga.ReserveStep{Repo: repo, Bank: bank, TTL: saga.DefaultReservationTTL},
		&saga.CommitStep{Repo: repo, Bank: bank},
	)

	h := gateway.NewHandler(repo, paymentSaga, participantReg)

	r := chi.NewRouter()
	h.Register(r)

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
		Handler:      handlerWithDevParticipant(r, participantReg),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("gateway listening on %s (dev mode: bank-a injected)", cfg.HTTPAddr)
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

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}
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
