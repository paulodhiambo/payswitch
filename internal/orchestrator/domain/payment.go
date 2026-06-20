package domain

import "time"

type PaymentStatus string

const (
	StatusReceived  PaymentStatus = "RECEIVED"
	StatusValidated PaymentStatus = "VALIDATED"
	StatusRouted    PaymentStatus = "ROUTED"
	StatusQuoted    PaymentStatus = "QUOTED"
	StatusScreened  PaymentStatus = "SCREENED"
	StatusReserved  PaymentStatus = "RESERVED"
	StatusCommitted PaymentStatus = "COMMITTED"
	StatusSettled   PaymentStatus = "SETTLED"
	StatusAborted   PaymentStatus = "ABORTED"
)

type Payment struct {
	ID               string
	EndToEndID       string
	SourceBIC        string
	DestinationBIC   string
	SourceAccount    string
	DestAccount      string
	Amount           int64
	Currency         string
	Status           PaymentStatus
	// ISO 20022 fields
	UETR             string    // Unique End-to-End Transaction Reference (UUID4, pacs.008 UETR)
	InstrID          string    // Instruction ID (pacs.008 InstrId)
	ChargeBearer     string    // Charge bearer code: DEBT/CRED/SHAR/SLEV
	SettlementDate   time.Time // Interbank settlement date (IntrBkSttlmDt)
	DebtorName       string    // Debtor party name (Dbtr.Nm)
	CreditorName     string    // Creditor party name (Cdtr.Nm)
	PurposeCode      string    // Purpose code (Purp.Cd), optional
	RemittanceInfo   string    // Unstructured remittance info (RmtInf.Ustrd), optional
	// Internal enrichment fields
	QuoteID          *string
	RouteFee         int64
	RouteEstimatedMs int
	ReservedAt       *time.Time
	ExpiresAt        *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
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
