package main

type article struct {
	Kicker   string
	Title    string
	Updated  string
	Intro    []string
	Stats    [][]string
	Sections []section
}

type section struct {
	ID        string
	Order     int
	Title     string
	NavTitle  string
	Summary   string
	Takeaway  string
	Body      []string
	Findings  []string
	Table     *table
	Code      string
	Language  string
	Snippets  []snippet
	WordCount int
}

type table struct {
	Headers []string
	Rows    [][]string
}

type snippet struct {
	ID        string
	Title     string
	Language  string
	Code      string
	LineCount int
}

type frontMatter struct {
	Kicker  string `yaml:"kicker"`
	Title   string `yaml:"title"`
	Updated string `yaml:"updated"`
}
