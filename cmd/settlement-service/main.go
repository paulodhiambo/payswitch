package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"switch/internal/settlement"
	"switch/pkg/config"
)

func main() {
	_, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	engine := settlement.NewEngine()
	window := settlement.NewWindow(5*time.Minute, 100)
	engine.AddWindow(window)

	log.Print("settlement-service started (5min net settlement window)")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Print("settlement-service shutting down...")
	window.Settle(ctx)
}
