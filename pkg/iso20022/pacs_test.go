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
	assert.Equal(t, "US123456789", tx.DbtrAcct.IBAN)
	assert.Equal(t, "BANKUS33", tx.DbtrAgt.FinInstnID.BICFI)
	assert.Equal(t, "BANKDEFF", tx.CdtrAgt.FinInstnID.BICFI)
	assert.Equal(t, "Euro Import GmbH", tx.Cdtr.Nm)
	assert.Equal(t, "DE89370400440532013000", tx.CdtrAcct.IBAN)

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

func TestStatusConstants(t *testing.T) {
	assert.Equal(t, "ACCP", iso20022.StatusAccepted)
	assert.Equal(t, "ACSC", iso20022.StatusAcceptedSettlement)
	assert.Equal(t, "RJCT", iso20022.StatusRejected)
	assert.Equal(t, "PDNG", iso20022.StatusPending)
	assert.Equal(t, "RCVD", iso20022.StatusReceived)
}
