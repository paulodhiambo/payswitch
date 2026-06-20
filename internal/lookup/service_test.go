package lookup_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"switch/internal/lookup"
)

type mockCache struct {
	mu   sync.Mutex
	data map[string]string
}

func newMockCache() *mockCache {
	return &mockCache{data: make(map[string]string)}
}

func (c *mockCache) Get(_ context.Context, key string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.data[key], nil
}

func (c *mockCache) Set(_ context.Context, key, val string, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = val
	return nil
}

func TestLookup_Found(t *testing.T) {
	svc := lookup.New(newMockCache())
	info, err := svc.Lookup(context.Background(), "BANKUS33")
	require.NoError(t, err)
	require.Equal(t, "Bank A US", info.Name)
	require.Equal(t, "US", info.Country)
}

func TestLookup_NotFound(t *testing.T) {
	svc := lookup.New(newMockCache())
	_, err := svc.Lookup(context.Background(), "NONEXIST")
	require.Error(t, err)
}

func TestLookup_CachesResult(t *testing.T) {
	mc := newMockCache()
	svc := lookup.New(mc)

	info, err := svc.Lookup(context.Background(), "BANKDEFF")
	require.NoError(t, err)

	cacheKey := "bank:BANKDEFF"
	cached, _ := mc.Get(context.Background(), cacheKey)
	var cachedInfo lookup.BankInfo
	err = json.Unmarshal([]byte(cached), &cachedInfo)
	require.NoError(t, err)
	require.Equal(t, info.BIC, cachedInfo.BIC)
}

func TestLookup_Register(t *testing.T) {
	svc := lookup.New(newMockCache())
	svc.Register(&lookup.BankInfo{
		BIC: "TESTBIC1", Name: "Test Bank", Country: "XX",
		Supported: []string{"USD"}, RoutingInfo: "TEST:001",
	})

	info, err := svc.Lookup(context.Background(), "TESTBIC1")
	require.NoError(t, err)
	require.Equal(t, "Test Bank", info.Name)
}
