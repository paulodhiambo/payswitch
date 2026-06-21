package iso20022_test

import (
	"encoding/xml"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"switch/pkg/iso20022"
)

func loadGolden(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	require.NoError(t, err)
	return data
}

func TestPacs008_UnmarshalGolden(t *testing.T) {
	data := loadGolden(t, "pacs008_simple.xml")

	var doc iso20022.Document
	err := xml.Unmarshal(data, &doc)
	require.NoError(t, err)

	grp := doc.FIToFICstmrCdtTrf.GrpHdr
	assert.Equal(t, "PAYMENT-20250620-001", grp.MsgID)
	assert.Equal(t, "1", grp.NbOfTxs)
	assert.Equal(t, 500.00, grp.TtlIntrBkSttlmAmt.Value)
	assert.Equal(t, "USD", grp.TtlIntrBkSttlmAmt.Ccy)

	txs := doc.FIToFICstmrCdtTrf.CdtTrfTxInf
	require.Len(t, txs, 1)
	tx := txs[0]

	assert.Equal(t, "E2E-20250620-001", tx.PmtID.EndToEndID)
	assert.Equal(t, "5c4f5e3a-9b8a-4f2e-8d7c-1a2b3c4d5e6f", tx.PmtID.UETR)
	assert.Equal(t, 500.00, tx.IntrBkSttlmAmt.Value)
	assert.Equal(t, "USD", tx.IntrBkSttlmAmt.Ccy)
	assert.Equal(t, "CRED", tx.ChrgBr)

	assert.Equal(t, "ACME Corp", tx.Dbtr.Nm)

	require.NotNil(t, tx.DbtrAcct.ID)
	require.NotNil(t, tx.DbtrAcct.ID.Othr)
	assert.Equal(t, "US123456789", tx.DbtrAcct.ID.Othr.ID)

	require.NotNil(t, tx.DbtrAgt.FinInstnID.Othr)
	assert.Equal(t, "BANKUS33", tx.DbtrAgt.FinInstnID.Othr.ID)
	require.NotNil(t, tx.CdtrAgt.FinInstnID.Othr)
	assert.Equal(t, "BANKDEFF", tx.CdtrAgt.FinInstnID.Othr.ID)

	assert.Equal(t, "Euro Import GmbH", tx.Cdtr.Nm)
	require.NotNil(t, tx.CdtrAcct.ID)
	require.NotNil(t, tx.CdtrAcct.ID.Othr)
	assert.Equal(t, "DE89370400440532013000", tx.CdtrAcct.ID.Othr.ID)

	require.NotNil(t, tx.Purp)
	assert.Equal(t, "SUPP", tx.Purp.Cd)

	require.NotNil(t, tx.RmtInf)
	assert.Equal(t, "Invoice INV-2025-001", tx.RmtInf.Ustrd)
}

func TestPacs008_RoundTrip(t *testing.T) {
	data := loadGolden(t, "pacs008_simple.xml")

	var doc iso20022.Document
	err := xml.Unmarshal(data, &doc)
	require.NoError(t, err)

	output, err := xml.MarshalIndent(doc, "", "  ")
	require.NoError(t, err)

	var doc2 iso20022.Document
	err = xml.Unmarshal(output, &doc2)
	require.NoError(t, err)

	assert.Equal(t, doc.FIToFICstmrCdtTrf.GrpHdr.MsgID, doc2.FIToFICstmrCdtTrf.GrpHdr.MsgID)
	assert.Equal(t, doc.FIToFICstmrCdtTrf.CdtTrfTxInf[0].PmtID.EndToEndID,
		doc2.FIToFICstmrCdtTrf.CdtTrfTxInf[0].PmtID.EndToEndID)
	assert.Equal(t, doc.FIToFICstmrCdtTrf.CdtTrfTxInf[0].IntrBkSttlmAmt.Value,
		doc2.FIToFICstmrCdtTrf.CdtTrfTxInf[0].IntrBkSttlmAmt.Value)
	assert.Equal(t, doc.FIToFICstmrCdtTrf.CdtTrfTxInf[0].Dbtr.Nm,
		doc2.FIToFICstmrCdtTrf.CdtTrfTxInf[0].Dbtr.Nm)
}

func TestPacs002_UnmarshalGolden(t *testing.T) {
	data := loadGolden(t, "pacs002_settled.xml")

	var doc iso20022.DocumentPacs002
	err := xml.Unmarshal(data, &doc)
	require.NoError(t, err)

	grp := doc.FIToFIPmtStsRpt.GrpHdr
	assert.Equal(t, "STATUS-20250620-001", grp.MsgID)

	st := doc.FIToFIPmtStsRpt.TxInfAndSts
	require.Len(t, st, 1)
	assert.Equal(t, "INSTR-001", st[0].OrgnlInstrID)
	assert.Equal(t, "E2E-20250620-001", st[0].OrgnlEndToEndID)
	assert.Equal(t, "TX-001", st[0].OrgnlTxID)
	assert.Equal(t, iso20022.StatusAcceptedSettlement, st[0].TxSts)

	require.NotNil(t, st[0].StsRsnInf)
	assert.Equal(t, "G000", st[0].StsRsnInf.Rsn.Cd)
	assert.Equal(t, "Settlement completed successfully", st[0].StsRsnInf.AddtlInf)
}

func TestPacs002_RoundTrip(t *testing.T) {
	data := loadGolden(t, "pacs002_settled.xml")

	var doc iso20022.DocumentPacs002
	err := xml.Unmarshal(data, &doc)
	require.NoError(t, err)

	output, err := xml.MarshalIndent(doc, "", "  ")
	require.NoError(t, err)

	var doc2 iso20022.DocumentPacs002
	err = xml.Unmarshal(output, &doc2)
	require.NoError(t, err)

	assert.Equal(t, doc.FIToFIPmtStsRpt.TxInfAndSts[0].TxSts,
		doc2.FIToFIPmtStsRpt.TxInfAndSts[0].TxSts)
}

func TestCamt003_UnmarshalGolden(t *testing.T) {
	data := loadGolden(t, "camt003_lookup.xml")

	var doc iso20022.DocumentCamt003
	err := xml.Unmarshal(data, &doc)
	require.NoError(t, err)

	msg := doc.GetAcct.MsgHdr
	assert.Equal(t, "COPEDUA145075", msg.MsgID)

	crit := doc.GetAcct.AcctQryDef.AcctCrit.NewCrit
	require.Len(t, crit, 1)
	sch := crit[0].SchCrit
	require.NotNil(t, sch.AcctId.ID)
	require.NotNil(t, sch.AcctId.ID.Othr)
	assert.Equal(t, "4001111547141", sch.AcctId.ID.Othr.ID)
}

func TestCamt003_RoundTrip(t *testing.T) {
	data := loadGolden(t, "camt003_lookup.xml")

	var doc iso20022.DocumentCamt003
	err := xml.Unmarshal(data, &doc)
	require.NoError(t, err)

	output, err := xml.MarshalIndent(doc, "", "  ")
	require.NoError(t, err)

	var doc2 iso20022.DocumentCamt003
	err = xml.Unmarshal(output, &doc2)
	require.NoError(t, err)

	assert.Equal(t, doc.GetAcct.MsgHdr.MsgID, doc2.GetAcct.MsgHdr.MsgID)
	assert.Equal(t, doc.GetAcct.AcctQryDef.AcctCrit.NewCrit[0].SchCrit.AcctId.ID.Othr.ID,
		doc2.GetAcct.AcctQryDef.AcctCrit.NewCrit[0].SchCrit.AcctId.ID.Othr.ID)
}

func TestStatusConstants(t *testing.T) {
	assert.Equal(t, "ACCP", iso20022.StatusAccepted)
	assert.Equal(t, "ACSC", iso20022.StatusAcceptedSettlement)
	assert.Equal(t, "RJCT", iso20022.StatusRejected)
	assert.Equal(t, "PDNG", iso20022.StatusPending)
	assert.Equal(t, "RCVD", iso20022.StatusReceived)
}

func TestConvertPacs008(t *testing.T) {
	data := loadGolden(t, "pacs008_simple.xml")

	var doc iso20022.Document
	err := xml.Unmarshal(data, &doc)
	require.NoError(t, err)

	d, err := iso20022.FromPacs008(&doc)
	require.NoError(t, err)

	assert.Equal(t, "E2E-20250620-001", d.EndToEndID)
	assert.Equal(t, "5c4f5e3a-9b8a-4f2e-8d7c-1a2b3c4d5e6f", d.UETR)
	assert.Equal(t, "INSTR-001", d.InstrID)
	assert.Equal(t, "BANKUS33", d.SourceBIC)
	assert.Equal(t, "BANKDEFF", d.DestBIC)
	assert.Equal(t, "US123456789", d.SourceAccount)
	assert.Equal(t, "DE89370400440532013000", d.DestAccount)
	assert.Equal(t, "ACME Corp", d.DebtorName)
	assert.Equal(t, "Euro Import GmbH", d.CreditorName)
	assert.Equal(t, int64(50000), d.Amount)
	assert.Equal(t, "USD", d.Currency)
	assert.Equal(t, "CRED", d.ChargeBearer)
	assert.Equal(t, "SUPP", d.PurposeCode)
	assert.Equal(t, "Invoice INV-2025-001", d.RemittanceInfo)
}

func TestDomainStatusToISO(t *testing.T) {
	assert.Equal(t, "RCVD", iso20022.DomainStatusToISO("RECEIVED"))
	assert.Equal(t, "PDNG", iso20022.DomainStatusToISO("LOOKED_UP"))
	assert.Equal(t, "ACSC", iso20022.DomainStatusToISO("SETTLED"))
	assert.Equal(t, "RJCT", iso20022.DomainStatusToISO("ABORTED"))
}

func TestToPacs002(t *testing.T) {
	d := iso20022.PaymentData{
		ID:         "pmt-001",
		EndToEndID: "e2e-001",
		UETR:       "abc-123",
		InstrID:    "instr-001",
		Amount:     50000,
		Currency:   "USD",
		Status:     "SETTLED",
	}
	doc := iso20022.ToPacs002("sts-001", d)
	sts := doc.FIToFIPmtStsRpt.TxInfAndSts[0]
	assert.Equal(t, "instr-001", sts.OrgnlInstrID)
	assert.Equal(t, "e2e-001", sts.OrgnlEndToEndID)
	assert.Equal(t, "pmt-001", sts.OrgnlTxID)
	assert.Equal(t, "ACSC", sts.TxSts)
}

func TestToPacs008(t *testing.T) {
	d := iso20022.PaymentData{
		ID:           "pmt-001",
		EndToEndID:   "e2e-001",
		UETR:         "abc-123",
		InstrID:      "instr-001",
		SourceBIC:    "BANKUS33",
		DestBIC:      "BANKDEFF",
		SourceAccount: "US123456789",
		DestAccount:  "DE89370400440532013000",
		DebtorName:   "ACME Corp",
		CreditorName: "Euro Import GmbH",
		Amount:       50000,
		Currency:     "USD",
		ChargeBearer: "CRED",
		PurposeCode:  "SUPP",
	}
	doc := iso20022.ToPacs008("MSG-001", d)
	tx := doc.FIToFICstmrCdtTrf.CdtTrfTxInf[0]
	assert.Equal(t, "instr-001", tx.PmtID.InstrID)
	assert.Equal(t, "e2e-001", tx.PmtID.EndToEndID)
	assert.Equal(t, "pmt-001", tx.PmtID.TxID)
	assert.Equal(t, "abc-123", tx.PmtID.UETR)
	assert.Equal(t, 500.00, tx.IntrBkSttlmAmt.Value)
	assert.Equal(t, "USD", tx.IntrBkSttlmAmt.Ccy)
	assert.Equal(t, "CRED", tx.ChrgBr)
	assert.Equal(t, "ACME Corp", tx.Dbtr.Nm)
	assert.Equal(t, "Euro Import GmbH", tx.Cdtr.Nm)

	require.NotNil(t, tx.DbtrAcct.ID)
	require.NotNil(t, tx.DbtrAcct.ID.Othr)
	assert.Equal(t, "US123456789", tx.DbtrAcct.ID.Othr.ID)

	require.NotNil(t, tx.CdtrAcct.ID)
	require.NotNil(t, tx.CdtrAcct.ID.Othr)
	assert.Equal(t, "DE89370400440532013000", tx.CdtrAcct.ID.Othr.ID)

	require.NotNil(t, tx.DbtrAgt.FinInstnID.Othr)
	assert.Equal(t, "BANKUS33", tx.DbtrAgt.FinInstnID.Othr.ID)
	require.NotNil(t, tx.CdtrAgt.FinInstnID.Othr)
	assert.Equal(t, "BANKDEFF", tx.CdtrAgt.FinInstnID.Othr.ID)

	require.NotNil(t, tx.Purp)
	assert.Equal(t, "SUPP", tx.Purp.Cd)
}
