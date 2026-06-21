package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	portalapi "switch/internal/portal/api"
	"switch/internal/portal/auth"
	"switch/internal/portal/store"
	"switch/pkg/config"
	"switch/pkg/metrics"
	"switch/pkg/telemetry"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger, lp, err := telemetry.InitLogger(ctx, cfg.OTLPEndpoint, "portal-api")
	if err != nil {
		log.Fatalf("init logger: %v", err)
	}
	if lp != nil {
		defer lp.Shutdown(ctx)
	}

	if cfg.OTLPEndpoint != "" {
		tp, err := telemetry.InitTracer(ctx, cfg.OTLPEndpoint, "portal-api")
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

	var csrfSecret []byte
	if raw := cfg.CSRFSecret; raw != "" {
		decoded, err := hex.DecodeString(raw)
		if err != nil {
			log.Fatalf("invalid CSRF_SECRET (must be hex-encoded): %v", err)
		}
		csrfSecret = decoded
	} else {
		csrfSecret = make([]byte, 32)
		if _, err := rand.Read(csrfSecret); err != nil {
			log.Fatalf("generate csrf secret: %v", err)
		}
		logger.Warn("CSRF_SECRET not set — using random key; CSRF tokens are invalidated on restart")
	}
	csrfStore := auth.NewCSRFStore(csrfSecret)

	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				csrfStore.Cleanup()
			case <-ctx.Done():
				return
			}
		}
	}()

	st := store.New(pool)
	srv := portalapi.NewServer(st, csrfStore, logger, cfg.PortalOrigin)

	r := chi.NewRouter()
	srv.Routes(r)

	metrics.Listen(cfg.MetricsAddr)

	httpSrv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("portal-api listening on %s", cfg.HTTPAddr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("serve: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Print("shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	httpSrv.Shutdown(shutdownCtx)
}
