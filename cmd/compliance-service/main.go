package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"switch/internal/compliance"
	"switch/pkg/config"
)

func main() {
	_, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	compliance.New()

	log.Print("compliance-service ready (invoked synchronously via orchestrator)")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Print("compliance-service shutting down...")
	cancel()
}
