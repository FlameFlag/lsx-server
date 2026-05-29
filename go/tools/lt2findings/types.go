package main

type finding struct {
	ID         string
	SourceFile string
	Program    string
	Address    string
	Kind       string
	Label      string
	Title      string
	CSymbol    string
	Comment    string
}

type cSymbol struct {
	Kind      string
	Name      string
	Source    string
	Line      int
	FindingID string
}
