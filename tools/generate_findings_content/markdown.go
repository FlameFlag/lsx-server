package main

import (
	"errors"
	"fmt"

	blackfriday "github.com/russross/blackfriday/v2"
)

const markdownExtensions = blackfriday.NoIntraEmphasis |
	blackfriday.Tables |
	blackfriday.FencedCode |
	blackfriday.HeadingIDs

type articleBuilder struct {
	doc     article
	current *section
}

func parseMarkdown(data []byte) (article, error) {
	front, body, err := parseFrontMatter(data)
	if err != nil {
		return article{}, err
	}

	doc, err := articleFromFrontMatter(front)
	if err != nil {
		return article{}, err
	}

	root := blackfriday.New(blackfriday.WithExtensions(markdownExtensions)).Parse(body)
	builder := articleBuilder{doc: doc}
	for node := root.FirstChild; node != nil; node = node.Next {
		if err := builder.add(node); err != nil {
			return article{}, err
		}
	}
	builder.flushSection()

	if len(builder.doc.Sections) == 0 {
		return article{}, errors.New("no sections found")
	}
	return builder.doc, nil
}

func articleFromFrontMatter(front frontMatter) (article, error) {
	doc := article{
		Kicker:  front.Kicker,
		Title:   front.Title,
		Updated: front.Updated,
	}
	if doc.Kicker == "" || doc.Title == "" || doc.Updated == "" {
		return article{}, errors.New("front matter must include kicker, title, and updated")
	}
	return doc, nil
}

func (b *articleBuilder) add(node *blackfriday.Node) error {
	if isSectionHeading(node) {
		return b.startSection(node)
	}
	if b.current == nil {
		applyIntroNode(&b.doc, node)
		return nil
	}
	applySectionNode(b.current, node)
	return nil
}

func (b *articleBuilder) startSection(node *blackfriday.Node) error {
	b.flushSection()

	title := nodeText(node)
	if node.HeadingID == "" {
		return fmt.Errorf("section %q is missing {#id}", title)
	}
	b.current = &section{
		ID:       node.HeadingID,
		Order:    len(b.doc.Sections) + 1,
		Title:    title,
		NavTitle: title,
	}
	return nil
}

func (b *articleBuilder) flushSection() {
	if b.current == nil {
		return
	}
	b.current.WordCount = sectionWordCount(*b.current)
	b.doc.Sections = append(b.doc.Sections, *b.current)
	b.current = nil
}

func isSectionHeading(node *blackfriday.Node) bool {
	return node.Type == blackfriday.Heading && node.Level == 2
}
