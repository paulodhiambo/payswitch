package banksvc

import (
	"encoding/xml"
	"net/http"
	"time"

	"switch/pkg/iso20022"
)

func (s *Server) handleCamt003(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var doc iso20022.DocumentCamt003
	if err := xml.NewDecoder(r.Body).Decode(&doc); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	crit := doc.GetAcct.AcctQryDef.AcctCrit.NewCrit
	if len(crit) == 0 || crit[0].SchCrit.AcctId.ID == nil || crit[0].SchCrit.AcctId.ID.Othr == nil {
		http.Error(w, "account identifier not found", http.StatusBadRequest)
		return
	}
	accountNumber := crit[0].SchCrit.AcctId.ID.Othr.ID

	acct, err := s.bank.GetAccount(accountNumber)
	if err != nil {
		writeCamt004Error(w, accountNumber, err.Error())
		return
	}

	resp := iso20022.DocumentCamt003Response{
		XMLNS: iso20022.NamespaceCamt003,
		AcctRpt: iso20022.AccountReport{
			MsgHdr: iso20022.MessageHeader{
				MsgID:   "BANK-ACCT-" + accountNumber,
				CreDtTm: time.Now().UTC().Format(time.RFC3339),
			},
			RptOrErr: iso20022.ReportOrError{
				AcctRpt: []iso20022.ReportedAccount{
					{
						AcctId: iso20022.AccountIdentification{ID: &iso20022.AccountID{Othr: &iso20022.GenericAccount{ID: accountNumber}}},
						Nm:     acct.Name,
						Ccy:    acct.Currency,
						LglNm:  acct.BIC,
					},
				},
			},
		},
	}

	out, _ := xml.MarshalIndent(resp, "", "  ")
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(xml.Header))
	w.Write(out)
}

func writeCamt004Error(w http.ResponseWriter, accountNumber, reason string) {
	resp := iso20022.DocumentCamt003Response{
		XMLNS: iso20022.NamespaceCamt003,
		AcctRpt: iso20022.AccountReport{
			MsgHdr: iso20022.MessageHeader{
				MsgID:   "ERR-" + accountNumber,
				CreDtTm: time.Now().UTC().Format(time.RFC3339),
			},
		},
	}
	out, _ := xml.MarshalIndent(resp, "", "  ")
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(xml.Header))
	w.Write(out)
}

func (s *Server) handlePacs008(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var doc iso20022.Document
	if err := xml.NewDecoder(r.Body).Decode(&doc); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	d, err := iso20022.FromPacs008(&doc)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tx := doc.FIToFICstmrCdtTrf.CdtTrfTxInf[0]

	amt := d.Amount

	if err := s.bank.Credit(d.DestAccount, amt); err != nil {
		writePacs002Error(w, d, "RJCT", "AM09", "Credit failed: "+err.Error())
		return
	}

	docPacs002 := iso20022.ToPacs002("STS-"+d.ID, iso20022.PaymentData{
		ID:             d.ID,
		EndToEndID:     d.EndToEndID,
		UETR:           d.UETR,
		InstrID:        d.InstrID,
		SourceBIC:      d.SourceBIC,
		DestBIC:        d.DestBIC,
		Amount:         d.Amount,
		Currency:       d.Currency,
		DebtorName:     d.DebtorName,
		CreditorName:   d.CreditorName,
		PurposeCode:    d.PurposeCode,
		RemittanceInfo: d.RemittanceInfo,
		Status:         "SETTLED",
	})
	docPacs002.FIToFIPmtStsRpt.TxInfAndSts[0].OrgnlTxID = tx.PmtID.TxID

	out, _ := xml.MarshalIndent(docPacs002, "", "  ")
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(xml.Header))
	w.Write(out)
}

func writePacs002Error(w http.ResponseWriter, d iso20022.PaymentData, status, reasonCode, reasonText string) {
	doc := iso20022.ToPacs002("STS-"+d.ID, iso20022.PaymentData{
		ID:             d.ID,
		EndToEndID:     d.EndToEndID,
		UETR:           d.UETR,
		InstrID:        d.InstrID,
		SourceBIC:      d.SourceBIC,
		DestBIC:        d.DestBIC,
		Amount:         d.Amount,
		Currency:       d.Currency,
		DebtorName:     d.DebtorName,
		CreditorName:   d.CreditorName,
		PurposeCode:    d.PurposeCode,
		RemittanceInfo: d.RemittanceInfo,
		Status:         "ABORTED",
	})
	doc.FIToFIPmtStsRpt.TxInfAndSts[0].TxSts = status
	doc.FIToFIPmtStsRpt.TxInfAndSts[0].StsRsnInf = &iso20022.StatusReason{
		Rsn:      iso20022.StatusReasonCode{Cd: reasonCode},
		AddtlInf: reasonText,
	}
	out, _ := xml.MarshalIndent(doc, "", "  ")
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(xml.Header))
	w.Write(out)
}
