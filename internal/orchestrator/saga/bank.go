package saga

import (
	"context"
	"fmt"
	"sync"
)

type MockBankClient struct {
	mu          sync.Mutex
	reservations map[string]int64
	balances    map[string]int64
	failOn      map[string]bool
}

func NewMockBankClient() *MockBankClient {
	return &MockBankClient{
		reservations: make(map[string]int64),
		balances:     make(map[string]int64),
		failOn:       make(map[string]bool),
	}
}

func (m *MockBankClient) SetBalance(account string, amount int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.balances[account] = amount
}

func (m *MockBankClient) SetFailOn(method string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failOn[method] = true
}

func (m *MockBankClient) Reserve(_ context.Context, bic, account string, amount int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failOn["reserve"] {
		return fmt.Errorf("mock: reserve failed for %s/%s", bic, account)
	}
	bal := m.balances[account]
	if bal < amount {
		return fmt.Errorf("mock: insufficient balance in %s/%s: have %d, need %d", bic, account, bal, amount)
	}
	m.reservations[account] += amount
	return nil
}

func (m *MockBankClient) ReleaseReservation(_ context.Context, bic, account string, amount int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reservations[account] -= amount
	return nil
}

func (m *MockBankClient) Credit(_ context.Context, bic, account string, amount int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failOn["credit"] {
		return fmt.Errorf("mock: credit failed for %s/%s", bic, account)
	}
	m.balances[account] += amount
	m.reservations[account] -= amount
	return nil
}

func (m *MockBankClient) ReverseCredit(_ context.Context, _ string, account string, amount int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.balances[account] -= amount
	return nil
}
