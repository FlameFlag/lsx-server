package main

import (
	"strings"

	blackfriday "github.com/russross/blackfriday/v2"
)

type introRule struct {
	NodeType blackfriday.NodeType
	Apply    func(*article, *blackfriday.Node)
}

type sectionRule struct {
	NodeType blackfriday.NodeType
	Apply    func(*section, *blackfriday.Node)
}

type paragraphRule struct {
	Prefix string
	Apply  func(*section, string)
}

var introRules = []introRule{
	{
		NodeType: blackfriday.Paragraph,
		Apply: func(doc *article, node *blackfriday.Node) {
			if text := nodeText(node); text != "" {
				doc.Intro = append(doc.Intro, text)
			}
		},
	},
	{
		NodeType: blackfriday.Table,
		Apply: func(doc *article, node *blackfriday.Node) {
			doc.Stats = markdownTable(node).Rows
		},
	},
}

var sectionRules = []sectionRule{
	{NodeType: blackfriday.Paragraph, Apply: applySectionParagraph},
	{NodeType: blackfriday.List, Apply: applySectionList},
	{NodeType: blackfriday.Table, Apply: applySectionTable},
	{NodeType: blackfriday.CodeBlock, Apply: applySectionCodeBlock},
}

var paragraphRules = []paragraphRule{
	{
		Prefix: "Summary:",
		Apply: func(sec *section, text string) {
			sec.Summary = strings.TrimSpace(text)
		},
	},
	{
		Prefix: "Takeaway:",
		Apply: func(sec *section, text string) {
			sec.Takeaway = strings.TrimSpace(text)
		},
	},
}

func applyIntroNode(doc *article, node *blackfriday.Node) {
	for _, rule := range introRules {
		if node.Type == rule.NodeType {
			rule.Apply(doc, node)
			return
		}
	}
}

func applySectionNode(sec *section, node *blackfriday.Node) {
	for _, rule := range sectionRules {
		if node.Type == rule.NodeType {
			rule.Apply(sec, node)
			return
		}
	}
}

func applySectionParagraph(sec *section, node *blackfriday.Node) {
	text := nodeText(node)
	for _, rule := range paragraphRules {
		if rest, ok := strings.CutPrefix(text, rule.Prefix); ok {
			rule.Apply(sec, rest)
			return
		}
	}
	if text != "" {
		sec.Body = append(sec.Body, text)
	}
}

func applySectionList(sec *section, node *blackfriday.Node) {
	sec.Findings = append(sec.Findings, listItems(node)...)
}

func applySectionTable(sec *section, node *blackfriday.Node) {
	tbl := markdownTable(node)
	sec.Table = &tbl
}

func applySectionCodeBlock(sec *section, node *blackfriday.Node) {
	sec.Snippets = append(sec.Snippets, codeSnippet(node))
}
