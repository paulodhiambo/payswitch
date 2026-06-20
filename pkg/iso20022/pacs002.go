package iso20022

import "encoding/xml"

// DocumentPacs002 is the root element of a pacs.002 Status Report message.
type DocumentPacs002 struct {
	XMLName            xml.Name           `xml:"Document"`
	FIToFIPmtStsRpt FIToFIPaymentStatusReport `xml:"FIToFIPmtStsRpt"`
}

type FIToFIPaymentStatusReport struct {
	GrpHdr      GroupHeader          `xml:"GrpHdr"`
	TxInfAndSts []TransactionStatus `xml:"TxInfAndSts"`
}

type TransactionStatus struct {
	OrgnlInstrID string `xml:"OrgnlInstrId"`
	OrgnlEndToEndID string `xml:"OrgnlEndToEndId"`
	OrgnlTxID    string `xml:"OrgnlTxId"`
	TxSts        string `xml:"TxSts"`
	StsRsnInf    *StatusReason `xml:"StsRsnInf,omitempty"`
}

type StatusReason struct {
	Rsn  StatusReasonCode `xml:"Rsn"`
	AddtlInf string       `xml:"AddtlInf"`
}

type StatusReasonCode struct {
	Cd string `xml:"Cd"`
}

// Constants for common status codes.
const (
	StatusAccepted           = "ACCP"
	StatusAcceptedSettlement = "ACSC"
	StatusRejected           = "RJCT"
	StatusPending            = "PDNG"
	StatusReceived           = "RCVD"
)

// Constants for reason codes.
const (
	ReasonNoInstruction = "NARR"
	ReasonCutOffTime    = "CUTA"
	ReasonInvalidAmount = "AM09"
)
