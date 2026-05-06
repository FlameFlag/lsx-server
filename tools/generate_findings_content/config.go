package main

import "path/filepath"

type config struct {
	SourcePath   string
	TemplatePath string
	OutputPath   string
}

func defaultConfig(root string) config {
	findingsDir := filepath.Join(root, "assets", "project", "findings")
	return config{
		SourcePath:   filepath.Join(findingsDir, "content.md"),
		TemplatePath: filepath.Join(root, "tools", "generate_findings_content", "article.html.tmpl"),
		OutputPath:   filepath.Join(findingsDir, "content.html.tmpl"),
	}
}
