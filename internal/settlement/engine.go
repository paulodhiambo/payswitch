package settlement

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// LedgerWriter records settlement entries for audit trail.
type LedgerWriter interface {
	WriteSettlement(ctx context.Context, bic, paymentID, eventType, payload string, amount int64) error
}

type SettlementEntry struct {
	PaymentID     string
	SourceBIC     string
	DestBIC       string
	Amount        int64
	Currency      string
	CommittedAt   time.Time
}

type NetPosition struct {
	BIC     string
	Amount  int64
	Currency string
}

type Window struct {
	mu       sync.Mutex
	entries  []SettlementEntry
	interval time.Duration
	maxSize  int
	closed   bool
	Ledger   LedgerWriter
}

func NewWindow(interval time.Duration, maxSize int) *Window {
	return &Window{
		entries:  make([]SettlementEntry, 0),
		interval: interval,
		maxSize:  maxSize,
	}
}

func (w *Window) Add(ctx context.Context, entry SettlementEntry) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("settlement window is closed")
	}

	w.entries = append(w.entries, entry)

	if len(w.entries) >= w.maxSize {
		go w.Settle(ctx)
	}

	return nil
}

func (w *Window) Settle(ctx context.Context) ([]NetPosition, error) {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil, fmt.Errorf("window already settled")
	}
	w.closed = true
	entries := w.entries
	w.entries = nil
	w.mu.Unlock()

	positions := calculateNetPositions(entries)

	for _, pos := range positions {
		log.Printf("[SETTLE] %s net %s %d", pos.BIC, pos.Currency, pos.Amount)
	}

	if w.Ledger != nil {
		for _, e := range entries {
			_ = w.Ledger.WriteSettlement(ctx, e.SourceBIC, e.PaymentID, "settlement.debit",
				fmt.Sprintf(`{"amount":%d,"currency":"%s"}`, e.Amount, e.Currency), -e.Amount)
			_ = w.Ledger.WriteSettlement(ctx, e.DestBIC, e.PaymentID, "settlement.credit",
				fmt.Sprintf(`{"amount":%d,"currency":"%s"}`, e.Amount, e.Currency), e.Amount)
		}
		for _, pos := range positions {
			_ = w.Ledger.WriteSettlement(ctx, pos.BIC, "net", "settlement.net",
				fmt.Sprintf(`{"amount":%d,"currency":"%s"}`, pos.Amount, pos.Currency), pos.Amount)
		}
	}

	return positions, nil
}

func calculateNetPositions(entries []SettlementEntry) []NetPosition {
	netMap := make(map[string]int64)

	for _, e := range entries {
		netMap[e.SourceBIC] -= e.Amount
		netMap[e.DestBIC] += e.Amount
	}

	var positions []NetPosition
	for bic, amount := range netMap {
		if amount != 0 {
			currency := ""
			for _, e := range entries {
				if e.SourceBIC == bic || e.DestBIC == bic {
					currency = e.Currency
					break
				}
			}
			positions = append(positions, NetPosition{
				BIC: bic, Amount: amount, Currency: currency,
			})
		}
	}

	return positions
}

type Engine struct {
	windows []*Window
}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) AddWindow(w *Window) {
	e.windows = append(e.windows, w)
}
