package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

type CSRFStore struct {
	secret []byte
	mu     sync.RWMutex
	tokens map[string]time.Time
}

func NewCSRFStore(secret []byte) *CSRFStore {
	return &CSRFStore{
		secret: secret,
		tokens: make(map[string]time.Time),
	}
}

func (s *CSRFStore) Generate(uid string) string {
	nonce := make([]byte, 16)
	_, _ = rand.Read(nonce)
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(uid))
	mac.Write(nonce)
	token := hex.EncodeToString(nonce) + "." + hex.EncodeToString(mac.Sum(nil))

	s.mu.Lock()
	s.tokens[uid+":"+token] = time.Now().Add(24 * time.Hour)
	s.mu.Unlock()
	return token
}

func (s *CSRFStore) Valid(uid, token string) bool {
	s.mu.RLock()
	exp, ok := s.tokens[uid+":"+token]
	s.mu.RUnlock()
	return ok && time.Now().Before(exp)
}

func (s *CSRFStore) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, exp := range s.tokens {
		if now.After(exp) {
			delete(s.tokens, k)
		}
	}
}
