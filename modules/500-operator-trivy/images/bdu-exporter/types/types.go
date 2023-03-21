package types

type Vulnerabilities struct {
	Vul []Vulnerability `xml:"vul"`
}

type Vulnerability struct {
	Identifier  string       `xml:"identifier"`
	Identifiers []Identifier `xml:"identifiers>identifier"`
}

type Identifier struct {
	Type       string `xml:"type,attr"`
	Link       string `xml:"link,attr"`
	Identifier string `xml:",chardata"`
}
