package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// ---------------------------------------------------------------------------
// Bank
// ---------------------------------------------------------------------------

type Bank struct {
	ID                string     `json:"id"`
	BIC               string     `json:"bic"`
	Name              string     `json:"name"`
	Country           string     `json:"country"`
	Status            string     `json:"status"`
	SettlementAccount string     `json:"settlementAccount"`
	APIBaseURL        string     `json:"apiBaseURL,omitempty"`
	APIEnabled        bool       `json:"apiEnabled"`
	CallbackURL       string     `json:"callbackURL,omitempty"`
	LookupAPIURL      string     `json:"lookupAPIURL,omitempty"`
	PaymentAPIURL     string     `json:"paymentAPIURL,omitempty"`
	StatusCheckAPIURL string     `json:"statusCheckAPIURL,omitempty"`
	OnboardedAt       *time.Time `json:"onboardedAt"`
	CreatedAt         time.Time  `json:"createdAt"`
}

type CreateBankParams struct {
	BIC               string
	Name              string
	Country           string
	SettlementAccount string
	Notes             string
	APIBaseURL        string
	APIEnabled        bool
	CallbackURL       string
	LookupAPIURL      string
	PaymentAPIURL     string
	StatusCheckAPIURL string
}

type UpdateBankAPIParams struct {
	APIBaseURL        string
	APIEnabled        bool
	LookupAPIURL      string
	PaymentAPIURL     string
	StatusCheckAPIURL string
}

func (s *Store) CreateBank(ctx context.Context, p CreateBankParams) (*Bank, error) {
	id := uuid.New().String()
	var b Bank
	err := s.pool.QueryRow(ctx,
		`INSERT INTO bank (id, bic, name, country, settlement_account, notes, api_base_url, api_enabled, callback_url, lookup_api_url, payment_api_url, status_check_api_url)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		 RETURNING id, bic, name, country, status, settlement_account,
		           COALESCE(api_base_url, ''), api_enabled, COALESCE(callback_url, ''),
		           COALESCE(lookup_api_url, ''), COALESCE(payment_api_url, ''), COALESCE(status_check_api_url, ''),
		           onboarded_at, created_at`,
		id, p.BIC, p.Name, p.Country, p.SettlementAccount, nilIfEmpty(p.Notes),
		nilIfEmpty(p.APIBaseURL), p.APIEnabled, nilIfEmpty(p.CallbackURL),
		nilIfEmpty(p.LookupAPIURL), nilIfEmpty(p.PaymentAPIURL), nilIfEmpty(p.StatusCheckAPIURL),
	).Scan(&b.ID, &b.BIC, &b.Name, &b.Country, &b.Status, &b.SettlementAccount,
		&b.APIBaseURL, &b.APIEnabled, &b.CallbackURL,
		&b.LookupAPIURL, &b.PaymentAPIURL, &b.StatusCheckAPIURL,
		&b.OnboardedAt, &b.CreatedAt)
	return &b, err
}

func (s *Store) GetBank(ctx context.Context, id string) (*Bank, error) {
	var b Bank
	err := s.pool.QueryRow(ctx,
		`SELECT id, bic, name, country, status, settlement_account,
		        COALESCE(api_base_url, ''), api_enabled, COALESCE(callback_url, ''),
		        COALESCE(lookup_api_url, ''), COALESCE(payment_api_url, ''), COALESCE(status_check_api_url, ''),
		        onboarded_at, created_at
		 FROM bank WHERE id = $1`, id,
	).Scan(&b.ID, &b.BIC, &b.Name, &b.Country, &b.Status, &b.SettlementAccount,
		&b.APIBaseURL, &b.APIEnabled, &b.CallbackURL,
		&b.LookupAPIURL, &b.PaymentAPIURL, &b.StatusCheckAPIURL,
		&b.OnboardedAt, &b.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &b, err
}

func (s *Store) GetBankByBIC(ctx context.Context, bic string) (*Bank, error) {
	var b Bank
	err := s.pool.QueryRow(ctx,
		`SELECT id, bic, name, country, status, settlement_account,
		        COALESCE(api_base_url, ''), api_enabled, COALESCE(callback_url, ''),
		        COALESCE(lookup_api_url, ''), COALESCE(payment_api_url, ''), COALESCE(status_check_api_url, ''),
		        onboarded_at, created_at
		 FROM bank WHERE bic = $1`, bic,
	).Scan(&b.ID, &b.BIC, &b.Name, &b.Country, &b.Status, &b.SettlementAccount,
		&b.APIBaseURL, &b.APIEnabled, &b.CallbackURL,
		&b.LookupAPIURL, &b.PaymentAPIURL, &b.StatusCheckAPIURL,
		&b.OnboardedAt, &b.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &b, err
}

func (s *Store) UpdateBankCallback(ctx context.Context, id string, callbackURL string) (*Bank, error) {
	var b Bank
	err := s.pool.QueryRow(ctx,
		`UPDATE bank
		 SET callback_url = $1
		 WHERE id = $2
		 RETURNING id, bic, name, country, status, settlement_account,
		           COALESCE(api_base_url, ''), api_enabled, COALESCE(callback_url, ''),
		           COALESCE(lookup_api_url, ''), COALESCE(payment_api_url, ''), COALESCE(status_check_api_url, ''),
		           onboarded_at, created_at`,
		nilIfEmpty(callbackURL), id,
	).Scan(&b.ID, &b.BIC, &b.Name, &b.Country, &b.Status, &b.SettlementAccount,
		&b.APIBaseURL, &b.APIEnabled, &b.CallbackURL,
		&b.LookupAPIURL, &b.PaymentAPIURL, &b.StatusCheckAPIURL,
		&b.OnboardedAt, &b.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &b, err
}

func (s *Store) UpdateBankAPI(ctx context.Context, id string, p UpdateBankAPIParams) (*Bank, error) {
	var b Bank
	err := s.pool.QueryRow(ctx,
		`UPDATE bank
		 SET api_base_url = $1, api_enabled = $2, lookup_api_url = $3, payment_api_url = $4, status_check_api_url = $5
		 WHERE id = $6
		 RETURNING id, bic, name, country, status, settlement_account,
		           COALESCE(api_base_url, ''), api_enabled, COALESCE(callback_url, ''),
		           COALESCE(lookup_api_url, ''), COALESCE(payment_api_url, ''), COALESCE(status_check_api_url, ''),
		           onboarded_at, created_at`,
		nilIfEmpty(p.APIBaseURL), p.APIEnabled, nilIfEmpty(p.LookupAPIURL), nilIfEmpty(p.PaymentAPIURL), nilIfEmpty(p.StatusCheckAPIURL), id,
	).Scan(&b.ID, &b.BIC, &b.Name, &b.Country, &b.Status, &b.SettlementAccount,
		&b.APIBaseURL, &b.APIEnabled, &b.CallbackURL,
		&b.LookupAPIURL, &b.PaymentAPIURL, &b.StatusCheckAPIURL,
		&b.OnboardedAt, &b.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &b, err
}

type BankListFilter struct {
	Status   string
	Page     int
	PageSize int
}

type PaginatedResult[T any] struct {
	Data     []T `json:"data"`
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}

func (s *Store) ListBanks(ctx context.Context, f BankListFilter) (*PaginatedResult[Bank], error) {
	where, args := []string{}, []any{}
	if f.Status != "" {
		args = append(args, f.Status)
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	var total int
	err := s.pool.QueryRow(ctx, "SELECT count(*) FROM bank "+whereClause, args...).Scan(&total)
	if err != nil {
		return nil, err
	}

	offset := (f.Page - 1) * f.PageSize
	args = append(args, f.PageSize, offset)
	query := fmt.Sprintf(
		`SELECT id, bic, name, country, status, settlement_account,
		        COALESCE(api_base_url, ''), api_enabled, COALESCE(callback_url, ''),
		        COALESCE(lookup_api_url, ''), COALESCE(payment_api_url, ''), COALESCE(status_check_api_url, ''),
		        onboarded_at, created_at
		 FROM bank %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		whereClause, len(args)-1, len(args))

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var banks []Bank
	for rows.Next() {
		var b Bank
		if err := rows.Scan(&b.ID, &b.BIC, &b.Name, &b.Country, &b.Status, &b.SettlementAccount,
			&b.APIBaseURL, &b.APIEnabled, &b.CallbackURL,
			&b.LookupAPIURL, &b.PaymentAPIURL, &b.StatusCheckAPIURL,
			&b.OnboardedAt, &b.CreatedAt); err != nil {
			return nil, err
		}
		banks = append(banks, b)
	}
	if banks == nil {
		banks = []Bank{}
	}
	return &PaginatedResult[Bank]{Data: banks, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}

var validTransitions = map[string][]string{
	"APPLICATION":       {"SANDBOX"},
	"SANDBOX":           {"CERTIFICATION"},
	"CERTIFICATION":     {"PRODUCTION_ACTIVE"},
	"PRODUCTION_ACTIVE": {"SUSPENDED"},
	"SUSPENDED":         {"PRODUCTION_ACTIVE", "DECOMMISSIONED"},
}

func (s *Store) UpdateBankStatus(ctx context.Context, id, newStatus, reason string) (*Bank, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Lock the row so concurrent status changes can't race through the
	// transition-validity check between our read and write.
	var currentStatus string
	var onboardedAt *time.Time
	err = tx.QueryRow(ctx,
		`SELECT status, onboarded_at FROM bank WHERE id = $1 FOR UPDATE`, id,
	).Scan(&currentStatus, &onboardedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	allowed := validTransitions[currentStatus]
	valid := false
	for _, a := range allowed {
		if a == newStatus {
			valid = true
			break
		}
	}
	if !valid {
		return nil, fmt.Errorf("invalid transition from %s to %s", currentStatus, newStatus)
	}

	var setOnboarded *time.Time
	if newStatus == "PRODUCTION_ACTIVE" && onboardedAt == nil {
		now := time.Now()
		setOnboarded = &now
	}

	_, err = tx.Exec(ctx,
		`UPDATE bank SET status = $1, onboarded_at = COALESCE($2, onboarded_at) WHERE id = $3`,
		newStatus, setOnboarded, id)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetBank(ctx, id)
}

// ---------------------------------------------------------------------------
// Certificate
// ---------------------------------------------------------------------------

type Certificate struct {
	ID          string     `json:"id"`
	BankID      string     `json:"bankId"`
	Subject     string     `json:"subject"`
	Fingerprint string     `json:"fingerprint"`
	NotBefore   time.Time  `json:"notBefore"`
	NotAfter    time.Time  `json:"notAfter"`
	Status      string     `json:"status"`
	IssuedAt    time.Time  `json:"issuedAt"`
	RevokedAt   *time.Time `json:"revokedAt"`
}

func (s *Store) ListCertificates(ctx context.Context, bankID string) ([]Certificate, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, bank_id, subject, fingerprint, not_before, not_after, status, issued_at, revoked_at
		 FROM bank_certificate WHERE bank_id = $1 ORDER BY issued_at DESC`, bankID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var certs []Certificate
	for rows.Next() {
		var c Certificate
		if err := rows.Scan(&c.ID, &c.BankID, &c.Subject, &c.Fingerprint, &c.NotBefore, &c.NotAfter, &c.Status, &c.IssuedAt, &c.RevokedAt); err != nil {
			return nil, err
		}
		certs = append(certs, c)
	}
	if certs == nil {
		certs = []Certificate{}
	}
	return certs, nil
}

type CreateCertificateParams struct {
	BankID      string
	Subject     string
	Fingerprint string
	NotBefore   time.Time
	NotAfter    time.Time
}

func (s *Store) CreateCertificate(ctx context.Context, p CreateCertificateParams) (*Certificate, error) {
	id := uuid.New().String()
	var c Certificate
	err := s.pool.QueryRow(ctx,
		`INSERT INTO bank_certificate (id, bank_id, subject, fingerprint, not_before, not_after)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, bank_id, subject, fingerprint, not_before, not_after, status, issued_at, revoked_at`,
		id, p.BankID, p.Subject, p.Fingerprint, p.NotBefore, p.NotAfter,
	).Scan(&c.ID, &c.BankID, &c.Subject, &c.Fingerprint, &c.NotBefore, &c.NotAfter, &c.Status, &c.IssuedAt, &c.RevokedAt)
	return &c, err
}

func (s *Store) RevokeCertificate(ctx context.Context, bankID, certID string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE bank_certificate SET status = 'REVOKED', revoked_at = now()
		 WHERE id = $1 AND bank_id = $2 AND status = 'ACTIVE'`, certID, bankID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ---------------------------------------------------------------------------
// Transaction
// ---------------------------------------------------------------------------

type TransactionSummary struct {
	PaymentID       string    `json:"paymentId"`
	EndToEndID      string    `json:"endToEndId"`
	SourceBank      BankRef   `json:"sourceBank"`
	DestinationBank BankRef   `json:"destinationBank"`
	Amount          int64     `json:"amount"`
	Currency        string    `json:"currency"`
	Status          string    `json:"status"`
	AbortReason     *string   `json:"abortReason"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type BankRef struct {
	BIC  string `json:"bic"`
	Name string `json:"name"`
}

type TransactionDetail struct {
	TransactionSummary
	UETR           *string            `json:"uetr"`
	InstructionID  *string            `json:"instructionId"`
	ChargeBearer   *string            `json:"chargeBearer"`
	SettlementDate *string            `json:"settlementDate"`
	DebtorName     *string            `json:"debtorName"`
	CreditorName   *string            `json:"creditorName"`
	PurposeCode    *string            `json:"purposeCode"`
	RemittanceInfo *string            `json:"remittanceInfo"`
	Timeline       []TransactionEvent `json:"timeline"`
}

type TransactionEvent struct {
	Event      string          `json:"event"`
	OccurredAt time.Time       `json:"occurredAt"`
	Detail     json.RawMessage `json:"detail"`
}

type TransactionFilter struct {
	Status    string
	BIC       string
	From      *time.Time
	To        *time.Time
	MinAmount *int64
	MaxAmount *int64
	Page      int
	PageSize  int
}

func (s *Store) ListTransactions(ctx context.Context, f TransactionFilter) (*PaginatedResult[TransactionSummary], error) {
	where, args := []string{}, []any{}
	addFilter := func(clause string, val any) {
		args = append(args, val)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}

	if f.Status != "" {
		addFilter("status = $%d", f.Status)
	}
	if f.BIC != "" {
		addFilter("(source_bic = $%d OR destination_bic = $%[1]d)", f.BIC)
	}
	if f.From != nil {
		addFilter("created_at >= $%d", *f.From)
	}
	if f.To != nil {
		addFilter("created_at <= $%d", *f.To)
	}
	if f.MinAmount != nil {
		addFilter("amount >= $%d", *f.MinAmount)
	}
	if f.MaxAmount != nil {
		addFilter("amount <= $%d", *f.MaxAmount)
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	var total int
	err := s.pool.QueryRow(ctx, "SELECT count(*) FROM transaction_view "+whereClause, args...).Scan(&total)
	if err != nil {
		return nil, err
	}

	offset := (f.Page - 1) * f.PageSize
	args = append(args, f.PageSize, offset)
	query := fmt.Sprintf(
		`SELECT payment_id, end_to_end_id, source_bic, source_bank_name,
		        destination_bic, destination_bank_name, amount, currency,
		        status, abort_reason, created_at, updated_at
		 FROM transaction_view %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		whereClause, len(args)-1, len(args))

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txns []TransactionSummary
	for rows.Next() {
		var t TransactionSummary
		var srcBIC, srcName, dstBIC, dstName string
		if err := rows.Scan(&t.PaymentID, &t.EndToEndID, &srcBIC, &srcName,
			&dstBIC, &dstName, &t.Amount, &t.Currency,
			&t.Status, &t.AbortReason, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		t.SourceBank = BankRef{BIC: srcBIC, Name: srcName}
		t.DestinationBank = BankRef{BIC: dstBIC, Name: dstName}
		txns = append(txns, t)
	}
	if txns == nil {
		txns = []TransactionSummary{}
	}
	return &PaginatedResult[TransactionSummary]{Data: txns, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}

// GetTransactionBICs returns the source and destination BICs for a payment.
// Returns ("", "", nil) when the payment does not exist.
func (s *Store) GetTransactionBICs(ctx context.Context, paymentID string) (sourceBIC, destBIC string, err error) {
	err = s.pool.QueryRow(ctx,
		`SELECT source_bic, destination_bic FROM transaction_view WHERE payment_id = $1`, paymentID,
	).Scan(&sourceBIC, &destBIC)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", nil
	}
	return
}

func (s *Store) GetTransaction(ctx context.Context, paymentID string) (*TransactionDetail, error) {
	var t TransactionDetail
	var srcBIC, srcName, dstBIC, dstName string
	var settlementDate *time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT payment_id, end_to_end_id, uetr, instruction_id,
		        source_bic, source_bank_name, destination_bic, destination_bank_name,
		        amount, currency, status, abort_reason, charge_bearer,
		        settlement_date, debtor_name, creditor_name, purpose_code,
		        remittance_info, created_at, updated_at
		 FROM transaction_view WHERE payment_id = $1`, paymentID,
	).Scan(&t.PaymentID, &t.EndToEndID, &t.UETR, &t.InstructionID,
		&srcBIC, &srcName, &dstBIC, &dstName,
		&t.Amount, &t.Currency, &t.Status, &t.AbortReason, &t.ChargeBearer,
		&settlementDate, &t.DebtorName, &t.CreditorName, &t.PurposeCode,
		&t.RemittanceInfo, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t.SourceBank = BankRef{BIC: srcBIC, Name: srcName}
	t.DestinationBank = BankRef{BIC: dstBIC, Name: dstName}
	if settlementDate != nil {
		s := settlementDate.Format("2006-01-02")
		t.SettlementDate = &s
	}

	t.Timeline, err = s.GetTransactionTimeline(ctx, paymentID)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) GetTransactionTimeline(ctx context.Context, paymentID string) ([]TransactionEvent, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT event_type, occurred_at, detail
		 FROM transaction_event_view WHERE payment_id = $1 ORDER BY occurred_at, id`, paymentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []TransactionEvent
	for rows.Next() {
		var e TransactionEvent
		if err := rows.Scan(&e.Event, &e.OccurredAt, &e.Detail); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	if events == nil {
		events = []TransactionEvent{}
	}
	return events, nil
}

// ---------------------------------------------------------------------------
// Dashboard
// ---------------------------------------------------------------------------

type DashboardBucket struct {
	PeriodStart           time.Time `json:"periodStart"`
	TotalTransactions     int       `json:"totalTransactions"`
	SuccessCount          int       `json:"successCount"`
	AbortCount            int       `json:"abortCount"`
	SuccessRate           float64   `json:"successRate"`
	P99LatencyMs          *int      `json:"p99LatencyMs"`
	TotalAmountMinorUnits int64     `json:"totalAmountMinorUnits"`
}

type DashboardSummary struct {
	Range   string            `json:"range"`
	Buckets []DashboardBucket `json:"buckets"`
}

func parseDuration(rangeStr string) (time.Duration, string) {
	switch rangeStr {
	case "1h":
		return 1 * time.Hour, "hour"
	case "6h":
		return 6 * time.Hour, "hour"
	case "24h":
		return 24 * time.Hour, "hour"
	case "7d":
		return 7 * 24 * time.Hour, "day"
	case "30d":
		return 30 * 24 * time.Hour, "day"
	default:
		return 24 * time.Hour, "hour"
	}
}

func (s *Store) GetDashboardSummary(ctx context.Context, bic, rangeStr string) (*DashboardSummary, error) {
	dur, truncUnit := parseDuration(rangeStr)
	since := time.Now().Add(-dur)

	bicFilter := ""
	args := []any{since}
	if bic != "" {
		bicFilter = "AND (source_bic = $2 OR destination_bic = $2)"
		args = append(args, bic)
	}

	query := fmt.Sprintf(`
		SELECT date_trunc('%s', created_at) AS period,
		       count(*) AS total,
		       count(*) FILTER (WHERE status IN ('COMMITTED','SETTLED')) AS success,
		       count(*) FILTER (WHERE status = 'ABORTED') AS aborted,
		       coalesce(sum(amount), 0) AS total_amount
		FROM transaction_view
		WHERE created_at >= $1 %s
		GROUP BY period ORDER BY period`, truncUnit, bicFilter)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var buckets []DashboardBucket
	for rows.Next() {
		var b DashboardBucket
		var total int
		if err := rows.Scan(&b.PeriodStart, &total, &b.SuccessCount, &b.AbortCount, &b.TotalAmountMinorUnits); err != nil {
			return nil, err
		}
		b.TotalTransactions = total
		if total > 0 {
			b.SuccessRate = float64(b.SuccessCount) / float64(total)
		}
		buckets = append(buckets, b)
	}
	if buckets == nil {
		buckets = []DashboardBucket{}
	}
	return &DashboardSummary{Range: rangeStr, Buckets: buckets}, nil
}

type AbortReason struct {
	Category   string  `json:"category"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

type AbortReasonBreakdown struct {
	Range   string        `json:"range"`
	Total   int           `json:"total"`
	Reasons []AbortReason `json:"reasons"`
}

func (s *Store) GetAbortReasons(ctx context.Context, bic, rangeStr string) (*AbortReasonBreakdown, error) {
	dur, _ := parseDuration(rangeStr)
	since := time.Now().Add(-dur)

	bicFilter := ""
	args := []any{since}
	if bic != "" {
		bicFilter = "AND (source_bic = $2 OR destination_bic = $2)"
		args = append(args, bic)
	}

	query := fmt.Sprintf(`
		SELECT coalesce(abort_reason, 'unknown') AS category, count(*) AS cnt
		FROM transaction_view
		WHERE status = 'ABORTED' AND created_at >= $1 %s
		GROUP BY category ORDER BY cnt DESC`, bicFilter)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reasons []AbortReason
	total := 0
	for rows.Next() {
		var r AbortReason
		if err := rows.Scan(&r.Category, &r.Count); err != nil {
			return nil, err
		}
		total += r.Count
		reasons = append(reasons, r)
	}
	for i := range reasons {
		if total > 0 {
			reasons[i].Percentage = float64(reasons[i].Count) / float64(total)
		}
	}
	if reasons == nil {
		reasons = []AbortReason{}
	}
	return &AbortReasonBreakdown{Range: rangeStr, Total: total, Reasons: reasons}, nil
}

// ---------------------------------------------------------------------------
// Settlement
// ---------------------------------------------------------------------------

type SettlementWindowSummary struct {
	ID                    string     `json:"id"`
	OpenedAt              time.Time  `json:"openedAt"`
	ClosedAt              *time.Time `json:"closedAt"`
	Status                string     `json:"status"`
	SettlementDate        string     `json:"settlementDate"`
	Currency              string     `json:"currency"`
	NetPositionMinorUnits *int64     `json:"netPositionMinorUnits"`
}

type SettlementPosition struct {
	BIC                string `json:"bic"`
	BankName           string `json:"bankName"`
	SentMinorUnits     int64  `json:"sentMinorUnits"`
	ReceivedMinorUnits int64  `json:"receivedMinorUnits"`
	NetMinorUnits      int64  `json:"netMinorUnits"`
	TransactionCount   int    `json:"transactionCount"`
}

type SettlementWindow struct {
	SettlementWindowSummary
	Positions []SettlementPosition `json:"positions"`
}

func (s *Store) ListSettlementWindows(ctx context.Context, bic string, from, to *time.Time, page, pageSize int) (*PaginatedResult[SettlementWindowSummary], error) {
	// Settlement windows are derived from transaction_view by grouping by date
	where, args := []string{}, []any{}
	addFilter := func(clause string, val any) {
		args = append(args, val)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}

	if bic != "" {
		addFilter("(source_bic = $%d OR destination_bic = $%[1]d)", bic)
	}
	if from != nil {
		addFilter("created_at::date >= $%d", *from)
	}
	if to != nil {
		addFilter("created_at::date <= $%d", *to)
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT count(DISTINCT created_at::date) FROM transaction_view %s", whereClause)
	var total int
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, err
	}

	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)
	query := fmt.Sprintf(`
		SELECT created_at::date AS settlement_date,
		       min(created_at) AS opened_at,
		       max(updated_at) AS closed_at,
		       count(*) AS tx_count,
		       'KES' AS currency,
		       coalesce(sum(amount), 0) AS net
		FROM transaction_view %s
		GROUP BY created_at::date
		ORDER BY created_at::date DESC
		LIMIT $%d OFFSET $%d`, whereClause, len(args)-1, len(args))

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var windows []SettlementWindowSummary
	for rows.Next() {
		var w SettlementWindowSummary
		var settDate time.Time
		var txCount int
		var net int64
		if err := rows.Scan(&settDate, &w.OpenedAt, &w.ClosedAt, &txCount, &w.Currency, &net); err != nil {
			return nil, err
		}
		w.ID = uuid.NewSHA1(uuid.NameSpaceURL, []byte(settDate.Format("2006-01-02"))).String()
		w.SettlementDate = settDate.Format("2006-01-02")
		w.Status = "CLOSED"
		if settDate.Format("2006-01-02") == time.Now().Format("2006-01-02") {
			w.Status = "OPEN"
		}
		w.NetPositionMinorUnits = &net
		windows = append(windows, w)
	}
	if windows == nil {
		windows = []SettlementWindowSummary{}
	}
	return &PaginatedResult[SettlementWindowSummary]{Data: windows, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *Store) GetSettlementWindow(ctx context.Context, id string, bic string) (*SettlementWindow, error) {
	// We derive windows from dates; try to find the date from the deterministic UUID
	// For simplicity, query all dates and match
	rows, err := s.pool.Query(ctx,
		`SELECT DISTINCT created_at::date FROM transaction_view ORDER BY created_at::date DESC LIMIT 365`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matchDate *time.Time
	for rows.Next() {
		var d time.Time
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		candidate := uuid.NewSHA1(uuid.NameSpaceURL, []byte(d.Format("2006-01-02"))).String()
		if candidate == id {
			matchDate = &d
			break
		}
	}
	if matchDate == nil {
		return nil, nil
	}

	bicFilter := ""
	args := []any{*matchDate}
	if bic != "" {
		bicFilter = "AND (source_bic = $2 OR destination_bic = $2)"
		args = append(args, bic)
	}

	// Get positions per BIC
	posQuery := fmt.Sprintf(`
		SELECT bic, bank_name,
		       coalesce(sum(sent), 0) AS sent,
		       coalesce(sum(received), 0) AS received,
		       coalesce(sum(sent) - sum(received), 0) AS net,
		       count(*) AS tx_count
		FROM (
			SELECT source_bic AS bic, source_bank_name AS bank_name, amount AS sent, 0 AS received
			FROM transaction_view WHERE created_at::date = $1 %[1]s
			UNION ALL
			SELECT destination_bic AS bic, destination_bank_name AS bank_name, 0 AS sent, amount AS received
			FROM transaction_view WHERE created_at::date = $1 %[1]s
		) sub
		GROUP BY bic, bank_name ORDER BY bic`, bicFilter)

	posRows, err := s.pool.Query(ctx, posQuery, args...)
	if err != nil {
		return nil, err
	}
	defer posRows.Close()

	var positions []SettlementPosition
	for posRows.Next() {
		var p SettlementPosition
		if err := posRows.Scan(&p.BIC, &p.BankName, &p.SentMinorUnits, &p.ReceivedMinorUnits, &p.NetMinorUnits, &p.TransactionCount); err != nil {
			return nil, err
		}
		positions = append(positions, p)
	}
	if positions == nil {
		positions = []SettlementPosition{}
	}

	w := &SettlementWindow{
		SettlementWindowSummary: SettlementWindowSummary{
			ID:             id,
			OpenedAt:       *matchDate,
			Status:         "CLOSED",
			SettlementDate: matchDate.Format("2006-01-02"),
			Currency:       "KES",
		},
		Positions: positions,
	}
	if matchDate.Format("2006-01-02") == time.Now().Format("2006-01-02") {
		w.Status = "OPEN"
	}
	return w, nil
}

// ---------------------------------------------------------------------------
// Audit Log
// ---------------------------------------------------------------------------

type AuditActor struct {
	ID    string `json:"id"`
	Email string `json:"email,omitempty"`
	Role  string `json:"role"`
}

type AuditLogEntry struct {
	ID         string          `json:"id"`
	Actor      AuditActor      `json:"actor"`
	Action     string          `json:"action"`
	TargetType string          `json:"targetType"`
	TargetID   string          `json:"targetId"`
	Diff       json.RawMessage `json:"diff"`
	OccurredAt time.Time       `json:"occurredAt"`
}

type AuditFilter struct {
	ActorID  string
	Action   string
	From     *time.Time
	To       *time.Time
	Page     int
	PageSize int
}

func (s *Store) InsertAuditLog(ctx context.Context, actorID, actorRole, action, targetType, targetID string, diff any) error {
	var diffJSON []byte
	if diff != nil {
		var err error
		diffJSON, err = json.Marshal(diff)
		if err != nil {
			return err
		}
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO audit_log (actor_id, actor_role, action, target_type, target_id, diff)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		actorID, actorRole, action, targetType, targetID, diffJSON)
	return err
}

func (s *Store) ListAuditLog(ctx context.Context, f AuditFilter) (*PaginatedResult[AuditLogEntry], error) {
	where, args := []string{}, []any{}
	addFilter := func(clause string, val any) {
		args = append(args, val)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}

	if f.ActorID != "" {
		addFilter("actor_id = $%d", f.ActorID)
	}
	if f.Action != "" {
		addFilter("action = $%d", f.Action)
	}
	if f.From != nil {
		addFilter("occurred_at >= $%d", *f.From)
	}
	if f.To != nil {
		addFilter("occurred_at <= $%d", *f.To)
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM audit_log "+whereClause, args...).Scan(&total); err != nil {
		return nil, err
	}

	offset := (f.Page - 1) * f.PageSize
	args = append(args, f.PageSize, offset)
	query := fmt.Sprintf(
		`SELECT id, actor_id, actor_role, action, target_type, target_id, diff, occurred_at
		 FROM audit_log %s ORDER BY occurred_at DESC LIMIT $%d OFFSET $%d`,
		whereClause, len(args)-1, len(args))

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []AuditLogEntry
	for rows.Next() {
		var e AuditLogEntry
		if err := rows.Scan(&e.ID, &e.Actor.ID, &e.Actor.Role, &e.Action, &e.TargetType, &e.TargetID, &e.Diff, &e.OccurredAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if entries == nil {
		entries = []AuditLogEntry{}
	}
	return &PaginatedResult[AuditLogEntry]{Data: entries, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}

// ---------------------------------------------------------------------------
// Export Job
// ---------------------------------------------------------------------------

type ExportJob struct {
	ID          string     `json:"jobId"`
	Status      string     `json:"status"`
	Format      string     `json:"format"`
	DownloadURL *string    `json:"downloadUrl"`
	RowCount    *int       `json:"rowCount"`
	Error       *string    `json:"error"`
	CreatedAt   time.Time  `json:"createdAt"`
	CompletedAt *time.Time `json:"completedAt"`
}

func (s *Store) CreateExportJob(ctx context.Context, ownerID, format string, filters json.RawMessage) (*ExportJob, error) {
	id := uuid.New().String()
	var j ExportJob
	err := s.pool.QueryRow(ctx,
		`INSERT INTO export_job (id, owner_id, format, filters)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, status, format, download_url, row_count, error, created_at, completed_at`,
		id, ownerID, format, filters,
	).Scan(&j.ID, &j.Status, &j.Format, &j.DownloadURL, &j.RowCount, &j.Error, &j.CreatedAt, &j.CompletedAt)
	return &j, err
}

func (s *Store) GetExportJob(ctx context.Context, jobID string) (*ExportJob, error) {
	var j ExportJob
	err := s.pool.QueryRow(ctx,
		`SELECT id, status, format, download_url, row_count, error, created_at, completed_at
		 FROM export_job WHERE id = $1`, jobID,
	).Scan(&j.ID, &j.Status, &j.Format, &j.DownloadURL, &j.RowCount, &j.Error, &j.CreatedAt, &j.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &j, err
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
