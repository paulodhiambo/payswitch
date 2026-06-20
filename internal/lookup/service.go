package lookup

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type BankInfo struct {
	BIC         string   `json:"bic"`
	Name        string   `json:"name"`
	Country     string   `json:"country"`
	Supported   []string `json:"supported_currencies"`
	RoutingInfo string   `json:"routing_info"`
}

type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
}

type Service struct {
	mu    sync.RWMutex
	banks map[string]*BankInfo
	cache Cache
}

func New(cache Cache) *Service {
	s := &Service{
		banks: make(map[string]*BankInfo),
		cache: cache,
	}
	s.seedDefaults()
	return s
}

func (s *Service) seedDefaults() {
	for _, b := range []*BankInfo{
		{BIC: "BANKUS33", Name: "Bank A US", Country: "US", Supported: []string{"USD"}, RoutingInfo: "CHIPS:1234"},
		{BIC: "BANKDEFF", Name: "Bank B DE", Country: "DE", Supported: []string{"EUR"}, RoutingInfo: "SEPA:DE123"},
		{BIC: "BANKGB2L", Name: "Bank C UK", Country: "GB", Supported: []string{"GBP"}, RoutingInfo: "FPS:4567"},
	} {
		s.banks[b.BIC] = b
	}
}

func (s *Service) Lookup(ctx context.Context, bic string) (*BankInfo, error) {
	cacheKey := "bank:" + bic

	if s.cache != nil {
		val, err := s.cache.Get(ctx, cacheKey)
		if err == nil && val != "" {
			var info BankInfo
			if json.Unmarshal([]byte(val), &info) == nil {
				return &info, nil
			}
		}
	}

	s.mu.RLock()
	info, ok := s.banks[bic]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("bank %s not found", bic)
	}

	if s.cache != nil {
		data, _ := json.Marshal(info)
		s.cache.Set(ctx, cacheKey, string(data), 5*time.Minute)
	}

	return info, nil
}

func (s *Service) Register(b *BankInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.banks[b.BIC] = b
}
