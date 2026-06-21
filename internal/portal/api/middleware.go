package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"switch/internal/portal/auth"
)

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		// Only reflect the origin if it matches the configured PORTAL_ORIGIN.
		// An empty allowedOrigin disables cross-origin access entirely (the
		// normal production case when the SPA is served through Authentik).
		if origin != "" && s.allowedOrigin != "" && origin == s.allowedOrigin {
			w.Header().Set("Access-Control-Allow-Origin", s.allowedOrigin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := auth.FromRequest(r)
		if claims.Sub == "" {
			writeError(w, http.StatusUnauthorized, "missing authentication")
			return
		}
		ctx := auth.WithClaims(r.Context(), claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequireRoles(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := auth.FromRequest(r)
			for _, role := range roles {
				if claims.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}
			writeError(w, http.StatusForbidden, "insufficient permissions")
		})
	}
}

func CSRFMiddleware(store *auth.CSRFStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}
			claims := auth.FromRequest(r)
			token := r.Header.Get("X-CSRF-Token")
			if token == "" {
				token = r.Header.Get("X-XSRF-TOKEN")
			}
			if !store.Valid(claims.Sub, token) {
				writeError(w, http.StatusForbidden, "invalid or missing CSRF token")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// bankScopeMiddleware enforces that BANK_* roles can only reach endpoints for
// their own bank. It resolves the {bankId} path parameter to a bank record and
// compares the bank's BIC with claims.ParticipantID (set as an Authentik user
// attribute). Switch staff pass through unconditionally.
func (s *Server) bankScopeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := auth.FromRequest(r)
		if !claims.IsBankRole() {
			next.ServeHTTP(w, r)
			return
		}
		bankID := chi.URLParam(r, "bankId")
		if bankID == "" {
			next.ServeHTTP(w, r)
			return
		}
		bank, err := s.store.GetBank(r.Context(), bankID)
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		if bank == nil {
			writeError(w, http.StatusNotFound, "bank not found")
			return
		}
		if bank.BIC != claims.ParticipantID {
			writeError(w, http.StatusForbidden, "access denied to this bank")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// txScopeCheck returns true if the caller's claims permit access to a
// transaction involving the given source and destination BICs.
func txScopeCheck(claims auth.Claims, sourceBIC, destBIC string) bool {
	if !claims.IsBankRole() {
		return true
	}
	return claims.ParticipantID == sourceBIC || claims.ParticipantID == destBIC
}
