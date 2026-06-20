package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"switch/internal/lookup"
	"switch/pkg/cache"
	"switch/pkg/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	var svc *lookup.Service
	if cfg.RedisAddr != "" {
		svc = lookup.New(cache.New(cfg.RedisAddr))
		log.Printf("lookup-service with Redis cache at %s", cfg.RedisAddr)
	} else {
		svc = lookup.New(nil)
		log.Print("lookup-service started (no cache)")
	}
	_ = svc

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Print("lookup-service shutting down...")
}
