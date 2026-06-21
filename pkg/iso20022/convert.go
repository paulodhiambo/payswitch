package iso20022

import (
	"fmt"
	"math"
	"time"
)

type PaymentData struct {
	ID          string
	EndToEndID  string
	UETR        string
	InstrID     string
	SourceBIC   string
	DestBIC     string
	SourceAccount string
	DestAccount   string
	DebtorName  string
	CreditorName string
	Amount      int64
	Currency    string
	ChargeBearer   string
	SettlementDate time.Time
	PurposeCode    string
	RemittanceInfo string
	Status    string
	CreatedAt time.Time
}

func amountToMinorUnits(v float64) int64 {
	return int64(math.Round(v * 100))
}

func minorUnitsToAmount(v int64) float64 {
	return float64(v) / 100
}

func accountBICFromAgent(agt FinancialInstitution) string {
	if agt.FinInstnID.BICFI != "" {
		return agt.FinInstnID.BICFI
	}
	if agt.FinInstnID.Othr != nil {
		return agt.FinInstnID.Othr.ID
	}
	return ""
}

func accountIDValue(a AccountIdentification) string {
	if a.IBAN != "" {
		return a.IBAN
	}
	if a.ID != nil && a.ID.Othr != nil {
		return a.ID.Othr.ID
	}
	return ""
}

func FromPacs008(doc *Document) (PaymentData, error) {
	txs := doc.FIToFICstmrCdtTrf.CdtTrfTxInf
	if len(txs) == 0 {
		return PaymentData{}, fmt.Errorf("pacs.008: no CdtTrfTxInf transactions")
	}
	tx := txs[0]
	grp := doc.FIToFICstmrCdtTrf.GrpHdr

	d := PaymentData{
		EndToEndID:   tx.PmtID.EndToEndID,
		UETR:         tx.PmtID.UETR,
		InstrID:      tx.PmtID.InstrID,
		SourceBIC:    accountBICFromAgent(tx.DbtrAgt),
		DestBIC:      accountBICFromAgent(tx.CdtrAgt),
		DebtorName:   tx.Dbtr.Nm,
		CreditorName: tx.Cdtr.Nm,
		Amount:       amountToMinorUnits(tx.IntrBkSttlmAmt.Value),
		Currency:     tx.IntrBkSttlmAmt.Ccy,
		ChargeBearer: tx.ChrgBr,
		SourceAccount: accountIDValue(tx.DbtrAcct),
		DestAccount:   accountIDValue(tx.CdtrAcct),
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
			if tx.PmtTpInf != nil && tx.PmtTpInf.CtgyPurp != nil {
				return tx.PmtTpInf.CtgyPurp.Cd
			}
			return ""
		}(),
	}

	if grp.IntrBkSttlmDt != "" {
		for _, layout := range []string{"2006-01-02", "2006-01-02T15:04:05Z07:00"} {
			if t, err := time.Parse(layout, grp.IntrBkSttlmDt); err == nil {
				d.SettlementDate = t
				break
			}
		}
	}

	return d, nil
}

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

	amt := minorUnitsToAmount(d.Amount)
	tx := CreditTransferTransaction{
		PmtID: PaymentIdentification{
			InstrID:    d.InstrID,
			EndToEndID: d.EndToEndID,
			TxID:       d.ID,
			UETR:       d.UETR,
		},
		IntrBkSttlmAmt: Amount{Value: amt, Ccy: d.Currency},
		IntrBkSttlmDt:  sttlmDate.Format("2006-01-02"),
		ChrgBr:         chargeBearer,
		Dbtr:           PartyIdentification{Nm: d.DebtorName},
		DbtrAcct:       AccountIdentification{ID: &AccountID{Othr: &GenericAccount{ID: d.SourceAccount}}},
		DbtrAgt:        FinancialInstitution{FinInstnID: FinancialInstitutionID{Othr: &GenericAccount{ID: d.SourceBIC}}},
		CdtrAgt:        FinancialInstitution{FinInstnID: FinancialInstitutionID{Othr: &GenericAccount{ID: d.DestBIC}}},
		Cdtr:           PartyIdentification{Nm: d.CreditorName},
		CdtrAcct:       AccountIdentification{ID: &AccountID{Othr: &GenericAccount{ID: d.DestAccount}}},
	}
	if d.PurposeCode != "" {
		tx.Purp = &Purpose{Cd: d.PurposeCode}
	}
	if d.RemittanceInfo != "" {
		tx.RmtInf = &RemittanceInformation{Ustrd: d.RemittanceInfo}
	}

	return &Document{
		XMLNS: NamespacePacs008,
		FIToFICstmrCdtTrf: FIToFICustomerCreditTransfer{
			GrpHdr: GroupHeader{
				MsgID:   msgID,
				CreDtTm: now.Format(time.RFC3339),
				NbOfTxs: "1",
				TtlIntrBkSttlmAmt: &Amount{Value: amt, Ccy: d.Currency},
				IntrBkSttlmDt:     sttlmDate.Format("2006-01-02"),
				SttlmInf:          &SettlementInformation{SttlmMtd: "CLRG"},
			},
			CdtTrfTxInf: []CreditTransferTransaction{tx},
		},
	}
}

func ToPacs002(msgID string, d PaymentData) *DocumentPacs002 {
	now := d.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
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
		XMLNS: NamespacePacs002,
		FIToFIPmtStsRpt: FIToFIPaymentStatusReport{
			GrpHdr: Pacs002GroupHeader{
				MsgID:   msgID,
				CreDtTm: now.Format(time.RFC3339),
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

func DomainStatusToISO(status string) string {
	switch status {
	case "RECEIVED":
		return StatusReceived
	case "VALIDATED", "LOOKED_UP", "ROUTED", "QUOTED", "SCREENED":
		return StatusPending
	case "RESERVED":
		return StatusAccepted
	case "COMMITTED":
		return StatusAccepted
	case "SETTLED":
		return StatusAcceptedSettlement
	case "ABORTED":
		return StatusRejected
	default:
		return StatusPending
	}
}
