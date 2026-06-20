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

type PaymentEvent struct {
	PaymentID  string        `json:"payment_id"`
	EndToEndID string        `json:"end_to_end_id"`
	FromStatus PaymentStatus `json:"from_status"`
	ToStatus   PaymentStatus `json:"to_status"`
	SourceBIC  string        `json:"source_bic"`
	DestBIC    string        `json:"dest_bic"`
	Amount     int64         `json:"amount"`
	Currency   string        `json:"currency"`
	Timestamp  time.Time     `json:"timestamp"`
}

type ComplianceResult struct {
	Cleared bool
	Reason  string
}
