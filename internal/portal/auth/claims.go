package auth

import (
	"context"
	"net/http"
)

type ctxKey struct{}

const (
	RoleSwitchAdmin  = "SWITCH_ADMIN"
	RoleSwitchOps    = "SWITCH_OPS"
	RoleBankAdmin    = "BANK_ADMIN"
	RoleBankOperator = "BANK_OPERATOR"
	RoleBankViewer   = "BANK_VIEWER"
)

type Claims struct {
	Sub           string
	Username      string
	Role          string
	ParticipantID string
}

func FromRequest(r *http.Request) Claims {
	if c, ok := r.Context().Value(ctxKey{}).(Claims); ok {
		return c
	}
	return Claims{
		Sub:           r.Header.Get("X-authentik-uid"),
		Username:      r.Header.Get("X-authentik-username"),
		Role:          r.Header.Get("X-User-Role"),
		ParticipantID: r.Header.Get("X-Participant-Id"),
	}
}

func WithClaims(ctx context.Context, c Claims) context.Context {
	return context.WithValue(ctx, ctxKey{}, c)
}

func (c Claims) IsSwitchStaff() bool {
	return c.Role == RoleSwitchAdmin || c.Role == RoleSwitchOps
}

func (c Claims) IsBankRole() bool {
	return c.Role == RoleBankAdmin || c.Role == RoleBankOperator || c.Role == RoleBankViewer
}

func (c Claims) CanWriteBanks() bool {
	return c.Role == RoleSwitchAdmin
}

func ScopeFilter(c Claims) (allParticipants bool, participantID string) {
	if c.IsSwitchStaff() {
		return true, ""
	}
	return false, c.ParticipantID
}
