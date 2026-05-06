package main

import (
	"strings"

	blackfriday "github.com/russross/blackfriday/v2"
)

func markdownTable(node *blackfriday.Node) table {
	var tbl table
	for child := node.FirstChild; child != nil; child = child.Next {
		switch child.Type {
		case blackfriday.TableHead:
			if row := child.FirstChild; row != nil {
				tbl.Headers = tableRow(row)
			}
		case blackfriday.TableBody:
			for row := child.FirstChild; row != nil; row = row.Next {
				tbl.Rows = append(tbl.Rows, paddedRow(tableRow(row), len(tbl.Headers)))
			}
		}
	}
	return tbl
}

func tableRow(row *blackfriday.Node) []string {
	var cells []string
	for cell := row.FirstChild; cell != nil; cell = cell.Next {
		if cell.Type == blackfriday.TableCell {
			cells = append(cells, nodeText(cell))
		}
	}
	return cells
}

func listItems(node *blackfriday.Node) []string {
	var items []string
	for item := node.FirstChild; item != nil; item = item.Next {
		if item.Type == blackfriday.Item {
			if text := nodeText(item); text != "" {
				items = append(items, text)
			}
		}
	}
	return items
}

func codeSnippet(node *blackfriday.Node) snippet {
	info := string(node.Info)
	code := strings.TrimRight(string(node.Literal), "\n")
	title := fenceTitle(info)
	return snippet{
		ID:        blackfriday.SanitizedAnchorName(title),
		Title:     title,
		Language:  fenceLanguage(info),
		Code:      code,
		LineCount: lineCount(code),
	}
}

func nodeText(node *blackfriday.Node) string {
	var parts []string
	node.Walk(func(n *blackfriday.Node, entering bool) blackfriday.WalkStatus {
		if !entering {
			return blackfriday.GoToNext
		}
		switch n.Type {
		case blackfriday.Text:
			parts = append(parts, string(n.Literal))
		case blackfriday.Code:
			parts = append(parts, "`"+string(n.Literal)+"`")
		case blackfriday.Softbreak, blackfriday.Hardbreak:
			parts = append(parts, " ")
		case blackfriday.CodeBlock:
			return blackfriday.SkipChildren
		}
		return blackfriday.GoToNext
	})
	return strings.Join(strings.Fields(strings.Join(parts, "")), " ")
}

func fenceLanguage(info string) string {
	fields := strings.Fields(info)
	if len(fields) == 0 || strings.Contains(fields[0], "=") {
		return ""
	}
	return fields[0]
}

func fenceTitle(info string) string {
	_, rest, ok := strings.Cut(info, `title="`)
	if !ok {
		return ""
	}
	title, _, _ := strings.Cut(rest, `"`)
	return title
}
