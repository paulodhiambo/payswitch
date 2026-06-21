package main

import (
	"log/slog"
	"os"

	"switch/internal/banksvc"
)

func main() {
	addr := os.Getenv("BANK_ADDR")
	if addr == "" {
		addr = ":8081"
	}

	slog.Info("starting mock bank service", "addr", addr)

	svc := banksvc.New(addr)
	if err := svc.Start(); err != nil {
		slog.Error("bank service failed", "error", err)
		os.Exit(1)
	}
}
