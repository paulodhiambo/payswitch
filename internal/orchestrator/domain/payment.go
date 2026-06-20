package domain

import "time"

type PaymentStatus string

const (
	StatusReceived  PaymentStatus = "RECEIVED"
	StatusValidated PaymentStatus = "VALIDATED"
	StatusQuoted    PaymentStatus = "QUOTED"
	StatusScreened  PaymentStatus = "SCREENED"
	StatusReserved  PaymentStatus = "RESERVED"
	StatusCommitted PaymentStatus = "COMMITTED"
	StatusAborted   PaymentStatus = "ABORTED"
)

type Payment struct {
	ID             string
	EndToEndID     string
	SourceBIC      string
	DestinationBIC string
	SourceAccount  string
	DestAccount    string
	Amount         int64
	Currency       string
	Status         PaymentStatus
	QuoteID        *string
	ReservedAt     *time.Time
	ExpiresAt      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Reservation struct {
	PaymentID     string
	SourceAccount string
	Amount        int64
	Status        string
	ReservedAt    time.Time
	ExpiresAt     time.Time
}
