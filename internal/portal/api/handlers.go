package api

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"switch/internal/portal/auth"
	"switch/internal/portal/store"
)

type Server struct {
	store         *store.Store
	csrf          *auth.CSRFStore
	log           *slog.Logger
	allowedOrigin string // PORTAL_ORIGIN env; empty = same-origin only
}

func NewServer(s *store.Store, csrf *auth.CSRFStore, log *slog.Logger, allowedOrigin string) *Server {
	return &Server{store: s, csrf: csrf, log: log, allowedOrigin: allowedOrigin}
}

func (s *Server) Routes(r chi.Router) {
	// Health endpoint is exempt from auth and CSRF so Docker/K8s probes work.
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Serve static files for the portal React SPA
	staticDir := os.Getenv("PORTAL_STATIC_DIR")
	if staticDir == "" {
		staticDir = "./portal/dist"
	}

	fs := http.FileServer(http.Dir(staticDir))
	r.HandleFunc("/portal/*", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		relPath := strings.TrimPrefix(path, "/portal")
		if relPath == "" || relPath == "/" {
			http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
			return
		}

		// If the file exists and is not a directory, serve it.
		// Otherwise, fallback to index.html (client-side routing fallback)
		fpath := filepath.Join(staticDir, relPath)
		if info, err := os.Stat(fpath); err != nil || info.IsDir() {
			http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
			return
		}

		http.StripPrefix("/portal", fs).ServeHTTP(w, r)
	})

	// All API routes require a valid Authentik session and CSRF token.
	// Mounted under /portal/api/v1 so Kong can proxy /portal/* without
	// stripping the path prefix. The /api/v1/* paths are kept as aliases
	// for direct-access dev mode (port 8090).
	r.Group(func(r chi.Router) {
		r.Use(s.corsMiddleware)
		r.Use(AuthMiddleware)
		r.Use(CSRFMiddleware(s.csrf))

		s.mountAPIRoutes(r, "/portal/api/v1")
		s.mountAPIRoutes(r, "/api/v1")
	})
}

// mountAPIRoutes registers all API endpoints under the given prefix.
func (s *Server) mountAPIRoutes(r chi.Router, prefix string) {
	r.Get(prefix+"/me", s.getMe)
	r.Get(prefix+"/csrf-token", s.getCSRFToken)

	r.Route(prefix+"/banks", func(r chi.Router) {
		r.With(RequireRoles(auth.RoleSwitchAdmin, auth.RoleSwitchOps)).Get("/", s.listBanks)
		r.With(RequireRoles(auth.RoleSwitchAdmin)).Post("/", s.createBank)
		r.Route("/{bankId}", func(r chi.Router) {
			r.Use(s.bankScopeMiddleware)
			r.Get("/", s.getBank)
			r.With(RequireRoles(auth.RoleSwitchAdmin)).Patch("/status", s.updateBankStatus)
			r.Get("/certificates", s.listCertificates)
			r.With(RequireRoles(auth.RoleSwitchAdmin, auth.RoleBankAdmin)).Post("/certificates", s.createCertificate)
			r.With(RequireRoles(auth.RoleSwitchAdmin, auth.RoleBankAdmin)).Delete("/certificates/{certId}", s.revokeCertificate)
		})
	})

	r.Route(prefix+"/transactions", func(r chi.Router) {
		r.Get("/", s.listTransactions)
		// /export must be registered before /{paymentId} so chi matches it as
		// a static segment rather than treating "export" as a paymentId value.
		r.With(RequireRoles(
			auth.RoleSwitchAdmin, auth.RoleSwitchOps,
			auth.RoleBankAdmin, auth.RoleBankOperator,
		)).Get("/export", s.createExport)
		r.Get("/{paymentId}", s.getTransaction)
		r.Get("/{paymentId}/timeline", s.getTransactionTimeline)
	})

	r.Get(prefix+"/export-jobs/{jobId}", s.getExportStatus)

	r.Route(prefix+"/dashboard", func(r chi.Router) {
		r.Get("/summary", s.getDashboardSummary)
		r.Get("/abort-reasons", s.getAbortReasons)
	})

	r.Route(prefix+"/settlement", func(r chi.Router) {
		r.Get("/windows", s.listSettlementWindows)
		r.Get("/windows/{windowId}", s.getSettlementWindow)
	})

	r.With(RequireRoles(auth.RoleSwitchAdmin)).Get(prefix+"/audit-log", s.listAuditLogs)
}

// ---------------------------------------------------------------------------
// Identity
// ---------------------------------------------------------------------------

func (s *Server) getMe(w http.ResponseWriter, r *http.Request) {
	claims := auth.FromRequest(r)
	resp := map[string]any{
		"userId":        claims.Sub,
		"username":      claims.Username,
		"role":          claims.Role,
		"participantId": claims.ParticipantID,
		"bankId":        nil,
		"bankName":      nil,
	}
	if claims.ParticipantID != "" {
		if bank, err := s.store.GetBankByBIC(r.Context(), claims.ParticipantID); err == nil && bank != nil {
			resp["bankId"] = bank.ID
			resp["bankName"] = bank.Name
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) getCSRFToken(w http.ResponseWriter, r *http.Request) {
	claims := auth.FromRequest(r)
	token := s.csrf.Generate(claims.Sub)
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

// ---------------------------------------------------------------------------
// Banks
// ---------------------------------------------------------------------------

func (s *Server) listBanks(w http.ResponseWriter, r *http.Request) {
	f := store.BankListFilter{
		Status:   r.URL.Query().Get("status"),
		Page:     intQuery(r, "page", 1),
		PageSize: intQuery(r, "pageSize", 20),
	}
	result, err := s.store.ListBanks(r.Context(), f)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type createBankRequest struct {
	BIC               string `json:"bic"`
	Name              string `json:"name"`
	Country           string `json:"country"`
	SettlementAccount string `json:"settlementAccount"`
	Notes             string `json:"notes"`
}

func (s *Server) createBank(w http.ResponseWriter, r *http.Request) {
	var req createBankRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.BIC == "" || req.Name == "" || req.Country == "" || req.SettlementAccount == "" {
		writeError(w, http.StatusBadRequest, "bic, name, country, and settlementAccount are required")
		return
	}

	bank, err := s.store.CreateBank(r.Context(), store.CreateBankParams{
		BIC:               req.BIC,
		Name:              req.Name,
		Country:           req.Country,
		SettlementAccount: req.SettlementAccount,
		Notes:             req.Notes,
	})
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	claims := auth.FromRequest(r)
	_ = s.store.InsertAuditLog(r.Context(), claims.Sub, claims.Role, "bank.create", "bank", bank.ID, req)

	writeJSON(w, http.StatusCreated, bank)
}

func (s *Server) getBank(w http.ResponseWriter, r *http.Request) {
	bankID := chi.URLParam(r, "bankId")
	bank, err := s.store.GetBank(r.Context(), bankID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if bank == nil {
		writeError(w, http.StatusNotFound, "bank not found")
		return
	}
	writeJSON(w, http.StatusOK, bank)
}

type updateStatusRequest struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func (s *Server) updateBankStatus(w http.ResponseWriter, r *http.Request) {
	bankID := chi.URLParam(r, "bankId")
	var req updateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Status == "" {
		writeError(w, http.StatusBadRequest, "status is required")
		return
	}

	bank, err := s.store.UpdateBankStatus(r.Context(), bankID, req.Status, req.Reason)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if bank == nil {
		writeError(w, http.StatusNotFound, "bank not found")
		return
	}

	claims := auth.FromRequest(r)
	_ = s.store.InsertAuditLog(r.Context(), claims.Sub, claims.Role, "bank.status_change", "bank", bankID, req)

	writeJSON(w, http.StatusOK, bank)
}

// ---------------------------------------------------------------------------
// Certificates
// ---------------------------------------------------------------------------

func (s *Server) listCertificates(w http.ResponseWriter, r *http.Request) {
	bankID := chi.URLParam(r, "bankId")
	certs, err := s.store.ListCertificates(r.Context(), bankID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, certs)
}

type createCertRequest struct {
	PEM string `json:"pem"`
}

func (s *Server) createCertificate(w http.ResponseWriter, r *http.Request) {
	bankID := chi.URLParam(r, "bankId")
	var req createCertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.PEM == "" {
		writeError(w, http.StatusBadRequest, "pem is required")
		return
	}

	block, _ := pem.Decode([]byte(req.PEM))
	if block == nil || block.Type != "CERTIFICATE" {
		writeError(w, http.StatusBadRequest, "invalid PEM: expected a CERTIFICATE block")
		return
	}
	x509cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		writeError(w, http.StatusBadRequest, "cannot parse certificate: "+err.Error())
		return
	}

	sum := sha256.Sum256(x509cert.Raw)
	fingerprint := hex.EncodeToString(sum[:])

	cert, err := s.store.CreateCertificate(r.Context(), store.CreateCertificateParams{
		BankID:      bankID,
		Subject:     x509cert.Subject.String(),
		Fingerprint: fingerprint,
		NotBefore:   x509cert.NotBefore,
		NotAfter:    x509cert.NotAfter,
	})
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	claims := auth.FromRequest(r)
	_ = s.store.InsertAuditLog(r.Context(), claims.Sub, claims.Role, "certificate.create", "certificate", cert.ID, map[string]string{
		"subject":     cert.Subject,
		"fingerprint": cert.Fingerprint,
	})

	writeJSON(w, http.StatusCreated, cert)
}

func (s *Server) revokeCertificate(w http.ResponseWriter, r *http.Request) {
	bankID := chi.URLParam(r, "bankId")
	certID := chi.URLParam(r, "certId")

	if err := s.store.RevokeCertificate(r.Context(), bankID, certID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "certificate not found or already revoked")
			return
		}
		s.serverError(w, r, err)
		return
	}

	claims := auth.FromRequest(r)
	_ = s.store.InsertAuditLog(r.Context(), claims.Sub, claims.Role, "certificate.revoke", "certificate", certID, nil)

	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Transactions
// ---------------------------------------------------------------------------

func (s *Server) listTransactions(w http.ResponseWriter, r *http.Request) {
	claims := auth.FromRequest(r)
	f := store.TransactionFilter{
		Status:   r.URL.Query().Get("status"),
		Page:     intQuery(r, "page", 1),
		PageSize: intQuery(r, "pageSize", 20),
	}

	if claims.IsBankRole() {
		f.BIC = claims.ParticipantID
	} else if bic := r.URL.Query().Get("bic"); bic != "" {
		f.BIC = bic
	}

	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.From = &t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.To = &t
		}
	}
	if v := r.URL.Query().Get("minAmount"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			f.MinAmount = &n
		}
	}
	if v := r.URL.Query().Get("maxAmount"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			f.MaxAmount = &n
		}
	}

	result, err := s.store.ListTransactions(r.Context(), f)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) getTransaction(w http.ResponseWriter, r *http.Request) {
	paymentID := chi.URLParam(r, "paymentId")
	tx, err := s.store.GetTransaction(r.Context(), paymentID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if tx == nil {
		writeError(w, http.StatusNotFound, "transaction not found")
		return
	}
	claims := auth.FromRequest(r)
	if !txScopeCheck(claims, tx.SourceBank.BIC, tx.DestinationBank.BIC) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	writeJSON(w, http.StatusOK, tx)
}

func (s *Server) getTransactionTimeline(w http.ResponseWriter, r *http.Request) {
	paymentID := chi.URLParam(r, "paymentId")

	claims := auth.FromRequest(r)
	if claims.IsBankRole() {
		sourceBIC, destBIC, err := s.store.GetTransactionBICs(r.Context(), paymentID)
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		if sourceBIC == "" {
			writeError(w, http.StatusNotFound, "transaction not found")
			return
		}
		if !txScopeCheck(claims, sourceBIC, destBIC) {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
	}

	events, err := s.store.GetTransactionTimeline(r.Context(), paymentID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, events)
}

// ---------------------------------------------------------------------------
// Dashboard
// ---------------------------------------------------------------------------

func (s *Server) getDashboardSummary(w http.ResponseWriter, r *http.Request) {
	claims := auth.FromRequest(r)
	bic := ""
	if claims.IsBankRole() {
		bic = claims.ParticipantID
	} else {
		bic = r.URL.Query().Get("bic")
	}
	rangeStr := r.URL.Query().Get("range")
	if rangeStr == "" {
		rangeStr = "24h"
	}

	summary, err := s.store.GetDashboardSummary(r.Context(), bic, rangeStr)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) getAbortReasons(w http.ResponseWriter, r *http.Request) {
	claims := auth.FromRequest(r)
	bic := ""
	if claims.IsBankRole() {
		bic = claims.ParticipantID
	} else {
		bic = r.URL.Query().Get("bic")
	}
	rangeStr := r.URL.Query().Get("range")
	if rangeStr == "" {
		rangeStr = "24h"
	}

	breakdown, err := s.store.GetAbortReasons(r.Context(), bic, rangeStr)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, breakdown)
}

// ---------------------------------------------------------------------------
// Settlement
// ---------------------------------------------------------------------------

func (s *Server) listSettlementWindows(w http.ResponseWriter, r *http.Request) {
	claims := auth.FromRequest(r)
	bic := ""
	if claims.IsBankRole() {
		bic = claims.ParticipantID
	} else {
		bic = r.URL.Query().Get("bic")
	}

	var from, to *time.Time
	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			from = &t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			to = &t
		}
	}

	result, err := s.store.ListSettlementWindows(r.Context(), bic, from, to,
		intQuery(r, "page", 1), intQuery(r, "pageSize", 20))
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) getSettlementWindow(w http.ResponseWriter, r *http.Request) {
	windowID := chi.URLParam(r, "windowId")
	claims := auth.FromRequest(r)
	bic := ""
	if claims.IsBankRole() {
		bic = claims.ParticipantID
	}

	window, err := s.store.GetSettlementWindow(r.Context(), windowID, bic)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if window == nil {
		writeError(w, http.StatusNotFound, "settlement window not found")
		return
	}
	writeJSON(w, http.StatusOK, window)
}

// ---------------------------------------------------------------------------
// Audit
// ---------------------------------------------------------------------------

func (s *Server) listAuditLogs(w http.ResponseWriter, r *http.Request) {
	f := store.AuditFilter{
		ActorID:  r.URL.Query().Get("actorId"),
		Action:   r.URL.Query().Get("action"),
		Page:     intQuery(r, "page", 1),
		PageSize: intQuery(r, "pageSize", 20),
	}
	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.From = &t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.To = &t
		}
	}

	result, err := s.store.ListAuditLog(r.Context(), f)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ---------------------------------------------------------------------------
// Exports
// ---------------------------------------------------------------------------

func (s *Server) createExport(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "csv"
	}
	if format != "csv" && format != "xlsx" {
		writeError(w, http.StatusBadRequest, `format must be "csv" or "xlsx"`)
		return
	}

	// Carry the same filters as listTransactions so the export respects scope.
	claims := auth.FromRequest(r)
	filters := map[string]string{
		"format": format,
		"status": r.URL.Query().Get("status"),
		"bic":    r.URL.Query().Get("bic"),
		"from":   r.URL.Query().Get("from"),
		"to":     r.URL.Query().Get("to"),
	}
	if claims.IsBankRole() {
		filters["bic"] = claims.ParticipantID
	}

	filtersJSON, _ := json.Marshal(filters)
	job, err := s.store.CreateExportJob(r.Context(), claims.Sub, format, filtersJSON)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) getExportStatus(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")
	job, err := s.store.GetExportJob(r.Context(), jobID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if job == nil {
		writeError(w, http.StatusNotFound, "export job not found")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func (s *Server) serverError(w http.ResponseWriter, r *http.Request, err error) {
	s.log.Error("internal error", "method", r.Method, "path", r.URL.Path, "error", err)
	writeError(w, http.StatusInternalServerError, "internal server error")
}

func intQuery(r *http.Request, key string, defaultVal int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return defaultVal
	}
	return n
}
