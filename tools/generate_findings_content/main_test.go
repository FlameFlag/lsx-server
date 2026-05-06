package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseMarkdownArticle(t *testing.T) {
	source := "" +
		"---\n" +
		"kicker: Test kicker\n" +
		"title: Test title\n" +
		"updated: 2026-05-05\n" +
		"---\n\n" +
		"Intro with `inline` code.\n\n" +
		"| Item | Finding |\n" +
		"| --- | --- |\n" +
		"| A | B |\n\n" +
		"## First section {#first-section}\n\n" +
		"Summary: Short summary.\n\n" +
		"Takeaway: Useful takeaway.\n\n" +
		"Body with `technical-name`.\n\n" +
		"| Name | Value |\n" +
		"| --- | --- |\n" +
		"| Short row |\n\n" +
		"```go title=\"Example snippet\"\n" +
		"fmt.Println(\"hi\")\n" +
		"```\n"
	doc, err := parseMarkdown([]byte(source))
	if err != nil {
		t.Fatal(err)
	}

	if doc.Kicker != "Test kicker" || doc.Title != "Test title" || doc.Updated != "2026-05-05" {
		t.Fatalf("front matter parsed incorrectly: %#v", doc)
	}
	if got := doc.Intro[0]; got != "Intro with `inline` code." {
		t.Fatalf("intro = %q", got)
	}
	if len(doc.Stats) != 1 || doc.Stats[0][0] != "A" || doc.Stats[0][1] != "B" {
		t.Fatalf("stats = %#v", doc.Stats)
	}

	if len(doc.Sections) != 1 {
		t.Fatalf("sections = %d, want 1", len(doc.Sections))
	}
	sec := doc.Sections[0]
	if sec.ID != "first-section" || sec.Order != 1 || sec.Title != "First section" {
		t.Fatalf("section identity = %#v", sec)
	}
	if sec.Summary != "Short summary." || sec.Takeaway != "Useful takeaway." {
		t.Fatalf("section callouts = %#v", sec)
	}
	if got := sec.Body[0]; got != "Body with `technical-name`." {
		t.Fatalf("body = %q", got)
	}
	if got := sec.Table.Rows[0]; len(got) != 2 || got[0] != "Short row" || got[1] != "" {
		t.Fatalf("padded table row = %#v", got)
	}
	if len(sec.Snippets) != 1 {
		t.Fatalf("snippets = %d, want 1", len(sec.Snippets))
	}
	snip := sec.Snippets[0]
	if snip.ID != "example-snippet" || snip.Title != "Example snippet" || snip.Language != "go" || snip.LineCount != 1 {
		t.Fatalf("snippet = %#v", snip)
	}
}

func TestGenerateFindingsHTML(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "content.md")
	outputPath := filepath.Join(dir, "content.html.tmpl")
	source := "" +
		"---\n" +
		"kicker: Test kicker\n" +
		"title: Test title\n" +
		"updated: 2026-05-05\n" +
		"---\n\n" +
		"Intro.\n\n" +
		"| Item | Finding |\n" +
		"| --- | --- |\n" +
		"| A | B |\n\n" +
		"## First section {#first-section}\n\n" +
		"Summary: Short summary.\n\n" +
		"```go title=\"Example snippet\"\n" +
		"fmt.Println(\"hi\")\n" +
		"```\n"
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := generate(config{
		SourcePath:   sourcePath,
		TemplatePath: "article.html.tmpl",
		OutputPath:   outputPath,
	}); err != nil {
		t.Fatal(err)
	}

	generated, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	body := string(generated)
	for _, want := range []string{
		`<header class="findings-hero">`,
		`<section class="finding-section" id="first-section">`,
		`data-code-language="go"`,
		`fmt.Println(&#34;hi&#34;)`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("generated HTML missing %q", want)
		}
	}
}
