package banksvc

import (
	"fmt"
	"sync"
	"time"
)

type Account struct {
	BIC         string `json:"bic"`
	Number      string `json:"number"`
	Name        string `json:"name"`
	Currency    string `json:"currency"`
	Balance     int64  `json:"balance"`
	CallbackURL string `json:"callbackURL,omitempty"`
}

type Reservation struct {
	Account string
	Amount  int64
	Created time.Time
}

type BankState struct {
	mu           sync.RWMutex
	accounts     map[string]*Account    // key: account number
	reservations map[string]*Reservation // key: account number
}

func NewBankState() *BankState {
	return &BankState{
		accounts:     make(map[string]*Account),
		reservations: make(map[string]*Reservation),
	}
}

func (b *BankState) AddAccount(a *Account) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.accounts[a.Number]; ok {
		return fmt.Errorf("account %s already exists", a.Number)
	}
	b.accounts[a.Number] = a
	return nil
}

func (b *BankState) GetAccount(number string) (*Account, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	a, ok := b.accounts[number]
	if !ok {
		return nil, fmt.Errorf("account %s not found", number)
	}
	return a, nil
}

func (b *BankState) ListAccounts() []*Account {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]*Account, 0, len(b.accounts))
	for _, a := range b.accounts {
		out = append(out, a)
	}
	return out
}

func (b *BankState) SetBalance(number string, amount int64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	a, ok := b.accounts[number]
	if !ok {
		return fmt.Errorf("account %s not found", number)
	}
	a.Balance = amount
	return nil
}

func (b *BankState) Reserve(account string, amount int64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	a, ok := b.accounts[account]
	if !ok {
		return fmt.Errorf("account %s not found", account)
	}
	if a.Balance < amount {
		return fmt.Errorf("insufficient balance in %s: have %d, need %d", account, a.Balance, amount)
	}
	a.Balance -= amount
	b.reservations[account] = &Reservation{
		Account: account,
		Amount:  amount,
		Created: time.Now(),
	}
	return nil
}

func (b *BankState) ReleaseReservation(account string, amount int64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	a, ok := b.accounts[account]
	if !ok {
		return fmt.Errorf("account %s not found", account)
	}
	delete(b.reservations, account)
	a.Balance += amount
	return nil
}

func (b *BankState) Credit(account string, amount int64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	a, ok := b.accounts[account]
	if !ok {
		return fmt.Errorf("account %s not found", account)
	}
	a.Balance += amount
	delete(b.reservations, account)
	return nil
}

func (b *BankState) ReverseCredit(account string, amount int64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	a, ok := b.accounts[account]
	if !ok {
		return fmt.Errorf("account %s not found", account)
	}
	if a.Balance < amount {
		return fmt.Errorf("insufficient balance in %s: have %d, need %d to reverse", account, a.Balance, amount)
	}
	a.Balance -= amount
	return nil
}

func (b *BankState) SeedDefaults() {
	for _, a := range []*Account{
		{BIC: "BANKUS33", Number: "ACC-A", Name: "First National Bank", Currency: "USD", Balance: 10_000_00, CallbackURL: "http://bank-service:8081/payments/callback"},
		{BIC: "BANKUS33", Number: "US123456789", Name: "First National Bank", Currency: "USD", Balance: 10_000_00, CallbackURL: "http://bank-service:8081/payments/callback"},
		{BIC: "BANKDEFF", Number: "ACC-B", Name: "Deutsche Exchange Bank", Currency: "EUR", Balance: 10_000_00, CallbackURL: "http://bank-service:8081/payments/callback"},
		{BIC: "BANKDEFF", Number: "DE89370400440532013000", Name: "Deutsche Exchange Bank", Currency: "EUR", Balance: 10_000_00, CallbackURL: "http://bank-service:8081/payments/callback"},
		{BIC: "BANKGB2L", Number: "ACC-C", Name: "London Clearing Bank", Currency: "GBP", Balance: 10_000_00, CallbackURL: "http://bank-service:8081/payments/callback"},
		{BIC: "BANKGB2L", Number: "GB29NWBK60161331926819", Name: "London Clearing Bank", Currency: "GBP", Balance: 10_000_00, CallbackURL: "http://bank-service:8081/payments/callback"},
	} {
		_ = b.AddAccount(a)
	}
}
