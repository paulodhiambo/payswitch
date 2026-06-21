package banksvc

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Server struct {
	bank *BankState
	addr string
}

func New(addr string) *Server {
	bank := NewBankState()
	bank.SeedDefaults()
	return &Server{
		bank: bank,
		addr: addr,
	}
}

func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	r.Route("/admin", func(r chi.Router) {
		r.Get("/accounts", s.handleListAccounts)
		r.Post("/accounts", s.handleCreateAccount)
		r.Get("/accounts/{number}", s.handleGetAccount)
		r.Put("/accounts/{number}/balance", s.handleSetBalance)
	})

	r.Post("/reserve", s.handleReserve)
	r.Post("/release", s.handleRelease)
	r.Post("/credit", s.handleCredit)
	r.Post("/reverse-credit", s.handleReverseCredit)

	r.Post("/lookup", s.handleCamt003)

	r.Post("/payments", s.handlePacs008)
	r.Post("/payments/callback", s.handlePacs002Callback)

	return r
}

func (s *Server) Start() error {
	slog.Info("bank-service starting", "addr", s.addr)
	return http.ListenAndServe(s.addr, s.Handler())
}
