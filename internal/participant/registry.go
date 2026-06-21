package participant

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"sync"
)

type Participant struct {
	ID       string
	Name     string
	BIC      string
	Account  string
	CertHash string
}

type Registry struct {
	mu           sync.RWMutex
	byID         map[string]*Participant
	byCertHash   map[string]*Participant
}

func NewRegistry() *Registry {
	return &Registry{
		byID:       make(map[string]*Participant),
		byCertHash: make(map[string]*Participant),
	}
}

func (r *Registry) Register(p *Participant) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.byID[p.ID]; ok {
		return fmt.Errorf("participant %s already registered", p.ID)
	}
	r.byID[p.ID] = p
	if p.CertHash != "" {
		r.byCertHash[p.CertHash] = p
	}
	return nil
}

func (r *Registry) GetByID(_ context.Context, id string) (*Participant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.byID[id]
	if !ok {
		return nil, fmt.Errorf("participant %s not found", id)
	}
	return p, nil
}

func (r *Registry) Resolve(_ context.Context, cert *x509.Certificate) (string, error) {
	hash := sha256.Sum256(cert.Raw)
	thumbprint := hex.EncodeToString(hash[:])

	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.byCertHash[thumbprint]
	if !ok {
		return "", fmt.Errorf("certificate not registered")
	}
	return p.ID, nil
}

func (r *Registry) GetBankForParticipant(ctx context.Context, id string) (bic, account string, err error) {
	p, err := r.GetByID(ctx, id)
	if err != nil {
		return "", "", err
	}
	return p.BIC, p.Account, nil
}

func (r *Registry) LookupAccount(ctx context.Context, account string) (bic string, bankName string, err error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, p := range r.byID {
		if p.Account == account {
			return p.BIC, p.Name, nil
		}
	}
	return "", "", fmt.Errorf("account %s not found", account)
}

func (r *Registry) LookupBIC(ctx context.Context, bic string) (account string, bankName string, err error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, p := range r.byID {
		if p.BIC == bic {
			return p.Account, p.Name, nil
		}
	}
	return "", "", fmt.Errorf("BIC %s not found", bic)
}
