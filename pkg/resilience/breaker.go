package resilience

import (
	"time"

	"github.com/sony/gobreaker"
)

func NewBankBreaker(bankBIC string) *gobreaker.CircuitBreaker {
	return gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "bank-" + bankBIC,
		MaxRequests: 5,
		Interval:    30 * time.Second,
		Timeout:     15 * time.Second,
		ReadyToTrip: func(c gobreaker.Counts) bool { return c.ConsecutiveFailures > 5 },
	})
}
