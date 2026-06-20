package main

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"log"
	"os"
	"os/signal"
	"syscall"

	"switch/internal/certificate"
	"switch/internal/participant"
	"switch/pkg/config"
)

func main() {
	_, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := participant.NewRegistry()
	svc := certificate.New(registry)
	_ = svc

	loadDemoParticipants(registry)

	log.Print("certificate-service ready")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Print("certificate-service shutting down...")
}

func loadDemoParticipants(r *participant.Registry) {
	for _, p := range []*participant.Participant{
		{ID: "bank-a", Name: "Bank A", BIC: "BANKUS33", Account: "ACC-A"},
		{ID: "bank-b", Name: "Bank B", BIC: "BANKDEFF", Account: "ACC-B"},
	} {
		if err := r.Register(p); err != nil {
			log.Printf("register demo %s: %v", p.ID, err)
		}
	}
	log.Print("loaded 2 demo participants")
}

func certFromPEM(pemBytes []byte) *x509.Certificate {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil
	}
	cert, _ := x509.ParseCertificate(block.Bytes)
	return cert
}
