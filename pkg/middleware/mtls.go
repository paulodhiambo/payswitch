package middleware

import (
	"context"
	"crypto/x509"
	"net/http"
)

const ParticipantCtxKey contextKey = "participant_id"

type CertRegistry interface {
	Resolve(ctx context.Context, cert *x509.Certificate) (participantID string, err error)
}

func ExtractParticipant(registry CertRegistry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
				http.Error(w, "client certificate required", http.StatusUnauthorized)
				return
			}
			participantID, err := registry.Resolve(r.Context(), r.TLS.PeerCertificates[0])
			if err != nil {
				http.Error(w, "unknown or revoked certificate", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), ParticipantCtxKey, participantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
