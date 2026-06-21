package iso20022

import "encoding/xml"

const NamespacePacs008 = "urn:iso:std:iso:20022:tech:xsd:pacs.008.001.08"

type Document struct {
	XMLName           xml.Name          `xml:"Document"`
	XMLNS             string            `xml:"xmlns,attr,omitempty"`
	FIToFICstmrCdtTrf FIToFICustomerCreditTransfer `xml:"FIToFICstmrCdtTrf"`
}

type FIToFICustomerCreditTransfer struct {
	GrpHdr      GroupHeader               `xml:"GrpHdr"`
	CdtTrfTxInf []CreditTransferTransaction `xml:"CdtTrfTxInf"`
}

type GroupHeader struct {
	MsgID        string `xml:"MsgId"`
	CreDtTm      string `xml:"CreDtTm"`
	NbOfTxs      string `xml:"NbOfTxs"`
	TtlIntrBkSttlmAmt *Amount `xml:"TtlIntrBkSttlmAmt,omitempty"`
	IntrBkSttlmDt     string `xml:"IntrBkSttlmDt,omitempty"`
	SttlmInf     *SettlementInformation `xml:"SttlmInf,omitempty"`
	InstgAgt     *FinancialInstitution  `xml:"InstgAgt,omitempty"`
	InstdAgt     *FinancialInstitution  `xml:"InstdAgt,omitempty"`
}

type Amount struct {
	Value float64 `xml:",chardata"`
	Ccy   string  `xml:"Ccy,attr"`
}

type SettlementInformation struct {
	SttlmMtd string                 `xml:"SttlmMtd"`
	SttlmAcct *AccountIdentification `xml:"SttlmAcct,omitempty"`
}

type CreditTransferTransaction struct {
	PmtID          PaymentIdentification  `xml:"PmtId"`
	PmtTpInf       *PaymentTypeInfo       `xml:"PmtTpInf,omitempty"`
	IntrBkSttlmAmt Amount                 `xml:"IntrBkSttlmAmt"`
	IntrBkSttlmDt  string                 `xml:"IntrBkSttlmDt,omitempty"`
	ChrgBr         string                 `xml:"ChrgBr"`
	InitgPty       *PartyIdentification   `xml:"InitgPty,omitempty"`
	Dbtr           PartyIdentification    `xml:"Dbtr"`
	DbtrAcct       AccountIdentification  `xml:"DbtrAcct"`
	DbtrAgt        FinancialInstitution   `xml:"DbtrAgt"`
	CdtrAgt        FinancialInstitution   `xml:"CdtrAgt"`
	Cdtr           PartyIdentification    `xml:"Cdtr"`
	CdtrAcct       AccountIdentification  `xml:"CdtrAcct"`
	Purp           *Purpose               `xml:"Purp,omitempty"`
	RmtInf         *RemittanceInformation `xml:"RmtInf,omitempty"`
}

type PaymentIdentification struct {
	InstrID    string `xml:"InstrId"`
	EndToEndID string `xml:"EndToEndId"`
	TxID       string `xml:"TxId"`
	UETR       string `xml:"UETR,omitempty"`
}

type PaymentTypeInfo struct {
	CtgyPurp *CategoryPurpose `xml:"CtgyPurp,omitempty"`
}

type CategoryPurpose struct {
	Cd string `xml:"Cd"`
}

type PartyIdentification struct {
	Nm      string         `xml:"Nm"`
	PstlAdr *PostalAddress `xml:"PstlAdr,omitempty"`
	ID      *PartyID       `xml:"Id,omitempty"`
}

type PostalAddress struct {
	Ctry    string `xml:"Ctry"`
	AdrLine string `xml:"AdrLine"`
}

type PartyID struct {
	OrgID  *OrganisationID `xml:"OrgId,omitempty"`
	PrvtID *PrivateID      `xml:"PrvtId,omitempty"`
}

type OrganisationID struct {
	BICOrBEI string          `xml:"BICOrBEI,omitempty"`
	Othr     *GenericID      `xml:"Othr,omitempty"`
}

type GenericID struct {
	ID      string        `xml:"Id"`
	SchmeNm *SchemeName   `xml:"SchmeNm,omitempty"`
}

type SchemeName struct {
	Cd string `xml:"Cd"`
}

type PrivateID struct {
	Othr string `xml:"Othr"`
}

type AccountIdentification struct {
	IBAN string     `xml:"IBAN,omitempty"`
	ID   *AccountID `xml:"Id,omitempty"`
}

type AccountID struct {
	Othr *GenericAccount `xml:"Othr,omitempty"`
}

type GenericAccount struct {
	ID string `xml:"Id"`
}

type FinancialInstitution struct {
	FinInstnID FinancialInstitutionID `xml:"FinInstnId"`
}

type FinancialInstitutionID struct {
	BICFI       string                `xml:"BICFI,omitempty"`
	ClrSysMmbID *ClearingSystemMember `xml:"ClrSysMmbId,omitempty"`
	Othr        *GenericAccount       `xml:"Othr,omitempty"`
	Nm          string                `xml:"Nm,omitempty"`
}

type ClearingSystemMember struct {
	ClrSysID string `xml:"ClrSysId"`
	MmbID    string `xml:"MmbId"`
}

type Purpose struct {
	Cd string `xml:"Cd"`
}

type RemittanceInformation struct {
	Ustrd string              `xml:"Ustrd,omitempty"`
	Strd  *StructuredRemittance `xml:"Strd,omitempty"`
}

type StructuredRemittance struct {
	RfrdDocInf *ReferredDocumentInfo `xml:"RfrdDocInf,omitempty"`
}

type ReferredDocumentInfo struct {
	Tp      *DocumentType `xml:"Tp,omitempty"`
	Nb      string        `xml:"Nb,omitempty"`
	RltdDt  string        `xml:"RltdDt,omitempty"`
}

type DocumentType struct {
	CdOrPrtry *CodeOrProprietary `xml:"CdOrPrtry,omitempty"`
}

type CodeOrProprietary struct {
	Cd string `xml:"Cd"`
}
