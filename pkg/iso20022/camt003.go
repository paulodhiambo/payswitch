package iso20022

import "encoding/xml"

const NamespaceCamt003 = "urn:iso:std:iso:20022:tech:xsd:camt.003.001.07"

type DocumentCamt003 struct {
	XMLName xml.Name  `xml:"Document"`
	XMLNS   string    `xml:"xmlns,attr,omitempty"`
	GetAcct GetAccount `xml:"GetAcct"`
}

type GetAccount struct {
	MsgHdr     MessageHeader `xml:"MsgHdr"`
	AcctQryDef AccountQueryDefinition `xml:"AcctQryDef"`
}

type MessageHeader struct {
	MsgID   string `xml:"MsgId"`
	CreDtTm string `xml:"CreDtTm"`
}

type AccountQueryDefinition struct {
	AcctCrit AccountCriteria `xml:"AcctCrit"`
}

type AccountCriteria struct {
	NewCrit []NewCriteria `xml:"NewCrit"`
}

type NewCriteria struct {
	SchCrit SearchCriteria `xml:"SchCrit"`
}

type SearchCriteria struct {
	AcctId AccountIdentification `xml:"AcctId"`
}

type DocumentCamt003Response struct {
	XMLName xml.Name       `xml:"Document"`
	XMLNS   string         `xml:"xmlns,attr,omitempty"`
	AcctRpt AccountReport  `xml:"AcctRpt"`
}

type AccountReport struct {
	MsgHdr  MessageHeader  `xml:"MsgHdr"`
	RptOrErr ReportOrError `xml:"RptOrErr"`
}

type ReportOrError struct {
	AcctRpt []ReportedAccount `xml:"AcctRpt,omitempty"`
	OprlErr *OperationalError `xml:"OprlErr,omitempty"`
}

type OperationalError struct {
	Prtry ProprietaryError `xml:"Prtry"`
	Desc  string           `xml:"Desc,omitempty"`
}

type ProprietaryError struct {
	Cd   string `xml:"Cd"`
	Issr string `xml:"Issr,omitempty"`
}

type ReportedAccount struct {
	AcctId AccountIdentification `xml:"AcctId"`
	Nm     string                `xml:"Nm,omitempty"`
	Tp     string                `xml:"Tp,omitempty"`
	Ccy    string                `xml:"Ccy,omitempty"`
	LglNm  string                `xml:"LglNm,omitempty"`
}
