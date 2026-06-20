package iso20022

import (
	"fmt"
	"math"
	"time"
)

// PaymentData is a neutral DTO carrying the ISO 20022 fields that flow
// between the gateway and the pacs.008/pacs.002 serialisers. It avoids
// importing the internal domain package from this library package.
type PaymentData struct {
	// Core identifiers
	ID          string    // internal switch payment ID (used as TxId)
	EndToEndID  string    // pacs.008 EndToEndId
	UETR        string    // pacs.008 UETR (UUID4)
	InstrID     string    // pacs.008 InstrId
	// Parties
	SourceBIC   string
	DestBIC     string
	SourceAccount string
	DestAccount   string
	DebtorName  string
	CreditorName string
	// Amount (internal minor units, e.g. cents)
	Amount      int64
	Currency    string
	// ISO 20022 payment attributes
	ChargeBearer   string // DEBT / CRED / SHAR / SLEV
	SettlementDate time.Time
	PurposeCode    string
	RemittanceInfo string
	// Status (domain status string, converted to pacs.002 code by DomainStatusToISO)
	Status    string
	CreatedAt time.Time
}

// amountToMinorUnits converts an ISO 20022 decimal amount to minor units
// (e.g. 500.00 USD → 50000). Assumes 2 decimal places for all currencies;
// use a currency precision table for JPY, KWD, etc. in production.
func amountToMinorUnits(v float64) int64 {
	return int64(math.Round(v * 100))
}

// minorUnitsToAmount converts minor units back to an ISO 20022 decimal amount.
func minorUnitsToAmount(v int64) float64 {
	return float64(v) / 100
}

// FromPacs008 extracts a PaymentData from the first transaction in a pacs.008
// document. Returns an error if the document has no transactions.
func FromPacs008(doc *Document) (PaymentData, error) {
	txs := doc.FIToFICstmrCdtTrf.CdtTrfTxInf
	if len(txs) == 0 {
		return PaymentData{}, fmt.Errorf("pacs.008: no CdtTrfTxInf transactions")
	}
	tx := txs[0]
	grp := doc.FIToFICstmrCdtTrf.GrpHdr

	d := PaymentData{
		EndToEndID:     tx.PmtID.EndToEndID,
		UETR:           tx.PmtID.UETR,
		InstrID:        tx.PmtID.InstrID,
		SourceBIC:      tx.DbtrAgt.FinInstnID.BICFI,
		DestBIC:        tx.CdtrAgt.FinInstnID.BICFI,
		DebtorName:     tx.Dbtr.Nm,
		CreditorName:   tx.Cdtr.Nm,
		Amount:         amountToMinorUnits(tx.IntrBkSttlmAmt.Value),
		Currency:       tx.IntrBkSttlmAmt.Ccy,
		ChargeBearer:   tx.ChrgBr,
		RemittanceInfo: func() string {
			if tx.RmtInf != nil {
				return tx.RmtInf.Ustrd
			}
			return ""
		}(),
		PurposeCode: func() string {
			if tx.Purp != nil {
				return tx.Purp.Cd
			}
			return ""
		}(),
	}

	// Account: prefer IBAN, fall back to Othr.Id
	if tx.DbtrAcct.IBAN != "" {
		d.SourceAccount = tx.DbtrAcct.IBAN
	} else if tx.DbtrAcct.Othr != nil {
		d.SourceAccount = tx.DbtrAcct.Othr.ID
	}
	if tx.CdtrAcct.IBAN != "" {
		d.DestAccount = tx.CdtrAcct.IBAN
	} else if tx.CdtrAcct.Othr != nil {
		d.DestAccount = tx.CdtrAcct.Othr.ID
	}

	// Settlement date from group header
	if grp.IntrBkSttlmDt != "" {
		if t, err := time.Parse("2006-01-02", grp.IntrBkSttlmDt); err == nil {
			d.SettlementDate = t
		}
	}

	return d, nil
}

// ToPacs008 builds a pacs.008 Document from PaymentData.
// msgID is the group-level message identifier (e.g. "MSG-"+payment.ID).
func ToPacs008(msgID string, d PaymentData) *Document {
	now := d.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	sttlmDate := d.SettlementDate
	if sttlmDate.IsZero() {
		sttlmDate = now
	}
	chargeBearer := d.ChargeBearer
	if chargeBearer == "" {
		chargeBearer = "SLEV"
	}

	dbtrAcct := AccountIdentification{IBAN: d.SourceAccount}
	if d.SourceAccount != "" && !isIBAN(d.SourceAccount) {
		dbtrAcct = AccountIdentification{Othr: &GenericAccount{ID: d.SourceAccount}}
	}
	cdtrAcct := AccountIdentification{IBAN: d.DestAccount}
	if d.DestAccount != "" && !isIBAN(d.DestAccount) {
		cdtrAcct = AccountIdentification{Othr: &GenericAccount{ID: d.DestAccount}}
	}

	amt := minorUnitsToAmount(d.Amount)
	tx := CreditTransferTransaction{
		PmtID: PaymentIdentification{
			InstrID:    d.InstrID,
			EndToEndID: d.EndToEndID,
			TxID:       d.ID,
			UETR:       d.UETR,
		},
		IntrBkSttlmAmt: Amount{Value: amt, Ccy: d.Currency},
		ChrgBr:         chargeBearer,
		Dbtr:           PartyIdentification{Nm: d.DebtorName},
		DbtrAcct:       dbtrAcct,
		DbtrAgt:        FinancialInstitution{FinInstnID: FinancialInstitutionID{BICFI: d.SourceBIC}},
		CdtrAgt:        FinancialInstitution{FinInstnID: FinancialInstitutionID{BICFI: d.DestBIC}},
		Cdtr:           PartyIdentification{Nm: d.CreditorName},
		CdtrAcct:       cdtrAcct,
	}
	if d.PurposeCode != "" {
		tx.Purp = &Purpose{Cd: d.PurposeCode}
	}
	if d.RemittanceInfo != "" {
		tx.RmtInf = &RemittanceInformation{Ustrd: d.RemittanceInfo}
	}

	return &Document{
		FIToFICstmrCdtTrf: FIToFICustomerCreditTransfer{
			GrpHdr: GroupHeader{
				MsgID:   msgID,
				CreDtTm: now.Format(time.RFC3339),
				NbOfTxs: "1",
				TtlIntrBkSttlmAmt: Amount{Value: amt, Ccy: d.Currency},
				IntrBkSttlmDt:     sttlmDate.Format("2006-01-02"),
				SttlmInf:          SettlementInformation{SttlmMtd: "CLRG"},
			},
			CdtTrfTxInf: []CreditTransferTransaction{tx},
		},
	}
}

// ToPacs002 builds a pacs.002 status report Document from PaymentData.
// msgID is the group-level message identifier (e.g. "STS-"+payment.ID).
func ToPacs002(msgID string, d PaymentData) *DocumentPacs002 {
	now := d.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	sttlmDate := d.SettlementDate
	if sttlmDate.IsZero() {
		sttlmDate = now
	}

	isoStatus := DomainStatusToISO(d.Status)
	var stsRsn *StatusReason
	if d.Status == "ABORTED" {
		stsRsn = &StatusReason{
			Rsn:      StatusReasonCode{Cd: ReasonNoInstruction},
			AddtlInf: "Payment was aborted",
		}
	}

	return &DocumentPacs002{
		FIToFIPmtStsRpt: FIToFIPaymentStatusReport{
			GrpHdr: GroupHeader{
				MsgID:             msgID,
				CreDtTm:           now.Format(time.RFC3339),
				NbOfTxs:           "1",
				TtlIntrBkSttlmAmt: Amount{Value: minorUnitsToAmount(d.Amount), Ccy: d.Currency},
				IntrBkSttlmDt:     sttlmDate.Format("2006-01-02"),
				SttlmInf:          SettlementInformation{SttlmMtd: "CLRG"},
			},
			TxInfAndSts: []TransactionStatus{
				{
					OrgnlInstrID:    d.InstrID,
					OrgnlEndToEndID: d.EndToEndID,
					OrgnlTxID:       d.ID,
					TxSts:           isoStatus,
					StsRsnInf:       stsRsn,
				},
			},
		},
	}
}

// DomainStatusToISO maps internal domain payment statuses to pacs.002 TxSts codes.
func DomainStatusToISO(status string) string {
	switch status {
	case "RECEIVED":
		return StatusReceived // RCVD
	case "VALIDATED", "ROUTED", "QUOTED", "SCREENED":
		return StatusPending // PDNG
	case "RESERVED":
		return StatusAccepted // ACCP — accepted, pending settlement
	case "COMMITTED":
		return StatusAccepted // ACCP — funds credited
	case "SETTLED":
		return StatusAcceptedSettlement // ACSC
	case "ABORTED":
		return StatusRejected // RJCT
	default:
		return StatusPending
	}
}

// isIBAN returns true if the account string looks like an IBAN (starts with
// two uppercase letters followed by digits). This is a heuristic sufficient
// for routing the value to the correct XML element.
func isIBAN(s string) bool {
	if len(s) < 5 {
		return false
	}
	return s[0] >= 'A' && s[0] <= 'Z' && s[1] >= 'A' && s[1] <= 'Z'
}
