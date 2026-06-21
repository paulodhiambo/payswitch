package iso20022

import "encoding/xml"

const NamespacePacs002 = "urn:iso:std:iso:20022:tech:xsd:pacs.002.001.10"

type DocumentPacs002 struct {
	XMLName            xml.Name           `xml:"Document"`
	XMLNS              string             `xml:"xmlns,attr,omitempty"`
	FIToFIPmtStsRpt FIToFIPaymentStatusReport `xml:"FIToFIPmtStsRpt"`
}

type FIToFIPaymentStatusReport struct {
	GrpHdr      Pacs002GroupHeader `xml:"GrpHdr"`
	TxInfAndSts []TransactionStatus `xml:"TxInfAndSts"`
}

type Pacs002GroupHeader struct {
	MsgID   string `xml:"MsgId"`
	CreDtTm string `xml:"CreDtTm"`
}

type TransactionStatus struct {
	OrgnlInstrID    string       `xml:"OrgnlInstrId"`
	OrgnlEndToEndID string       `xml:"OrgnlEndToEndId"`
	OrgnlTxID       string       `xml:"OrgnlTxId"`
	TxSts           string       `xml:"TxSts"`
	StsRsnInf       *StatusReason `xml:"StsRsnInf,omitempty"`
}

type StatusReason struct {
	Rsn      StatusReasonCode `xml:"Rsn"`
	AddtlInf string           `xml:"AddtlInf"`
}

type StatusReasonCode struct {
	Cd string `xml:"Cd"`
}

const (
	StatusAccepted           = "ACCP"
	StatusAcceptedSettlement = "ACSC"
	StatusRejected           = "RJCT"
	StatusPending            = "PDNG"
	StatusReceived           = "RCVD"
)

const (
	ReasonNoInstruction = "NARR"
	ReasonCutOffTime    = "CUTA"
	ReasonInvalidAmount = "AM09"
	ReasonNotFound      = "N404"
)
