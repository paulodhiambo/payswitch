package iso20022

import "encoding/xml"

const (
	Namespace    = "urn:iso:std:iso:20022:tech:xsd:pacs.008.001.12"
	NamespacePacs002 = "urn:iso:std:iso:20022:tech:xsd:pacs.002.001.14"
)

// Document is the root element of a pacs.008 Credit Transfer message.
type Document struct {
	XMLName           xml.Name          `xml:"Document"`
	FIToFICstmrCdtTrf FIToFICustomerCreditTransfer `xml:"FIToFICstmrCdtTrf"`
}

// FIToFICustomerCreditTransfer is the root of pacs.008.
type FIToFICustomerCreditTransfer struct {
	GrpHdr  GroupHeader  `xml:"GrpHdr"`
	CdtTrfTxInf []CreditTransferTransaction `xml:"CdtTrfTxInf"`
}

// GroupHeader contains message-level metadata.
type GroupHeader struct {
	MsgID        string `xml:"MsgId"`
	CreDtTm      string `xml:"CreDtTm"`
	NbOfTxs      string `xml:"NbOfTxs"`
	TtlIntrBkSttlmAmt Amount `xml:"TtlIntrBkSttlmAmt"`
	IntrBkSttlmDt     string `xml:"IntrBkSttlmDt"`
	SttlmInf     SettlementInformation `xml:"SttlmInf"`
}

type Amount struct {
	Value    float64 `xml:",chardata"`
	Ccy      string  `xml:"Ccy,attr"`
}

type SettlementInformation struct {
	SttlmMtd string `xml:"SttlmMtd"`
	SttlmAcct *AccountIdentification `xml:"SttlmAcct,omitempty"`
}

type CreditTransferTransaction struct {
	PmtID      PaymentIdentification `xml:"PmtId"`
	IntrBkSttlmAmt Amount             `xml:"IntrBkSttlmAmt"`
	SttlmTmInd     *SettlementTimeIndicator `xml:"SttlmTmInd,omitempty"`
	ChrgBr        string             `xml:"ChrgBr"`
	Dbtr          PartyIdentification `xml:"Dbtr"`
	DbtrAcct      AccountIdentification `xml:"DbtrAcct"`
	DbtrAgt       FinancialInstitution `xml:"DbtrAgt"`
	CdtrAgt       FinancialInstitution `xml:"CdtrAgt"`
	Cdtr          PartyIdentification `xml:"Cdtr"`
	CdtrAcct      AccountIdentification `xml:"CdtrAcct"`
	Purp          *Purpose           `xml:"Purp,omitempty"`
	RmtInf        *RemittanceInformation `xml:"RmtInf,omitempty"`
}

type PaymentIdentification struct {
	InstrID    string `xml:"InstrId"`
	EndToEndID string `xml:"EndToEndId"`
	TxID       string `xml:"TxId"`
	UETR       string `xml:"UETR"`
}

type SettlementTimeIndicator struct {
	DbtrDtTm string `xml:"DbtrDtTm"`
	CdtrDtTm string `xml:"CdtrDtTm"`
}

type PartyIdentification struct {
	Nm  string        `xml:"Nm"`
	PstlAdr *PostalAddress `xml:"PstlAdr,omitempty"`
	ID  *PartyID       `xml:"Id,omitempty"`
}

type PostalAddress struct {
	Ctry   string `xml:"Ctry"`
	AdrLine string `xml:"AdrLine"`
}

type PartyID struct {
	OrgID *OrganisationID `xml:"OrgId,omitempty"`
	PrvtID *PrivateID     `xml:"PrvtId,omitempty"`
}

type OrganisationID struct {
	BICOrBEI string `xml:"BICOrBEI"`
}

type PrivateID struct {
	Othr string `xml:"Othr"`
}

type AccountIdentification struct {
	IBAN string `xml:"IBAN"`
	Othr *GenericAccount `xml:"Othr,omitempty"`
}

type GenericAccount struct {
	ID string `xml:"Id"`
}

type FinancialInstitution struct {
	FinInstnID FinancialInstitutionID `xml:"FinInstnId"`
}

type FinancialInstitutionID struct {
	BICFI string `xml:"BICFI"`
	ClrSysMmbID *ClearingSystemMember `xml:"ClrSysMmbId,omitempty"`
	Nm    string `xml:"Nm,omitempty"`
}

type ClearingSystemMember struct {
	ClrSysID string `xml:"ClrSysId"`
	MmbID    string `xml:"MmbId"`
}

type Purpose struct {
	Cd string `xml:"Cd"`
}

type RemittanceInformation struct {
	Ustrd string `xml:"Ustrd"`
}
