package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"switch/internal/reconciliation"
	"switch/pkg/config"
)

func main() {
	_, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc := reconciliation.New()
	_ = svc

	log.Print("reconciliation-service started")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Print("reconciliation-service shutting down...")
}
