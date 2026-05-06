package main

import (
	"bytes"
	"html/template"
	"os"
	"path/filepath"
)

func generate(cfg config) error {
	content, err := os.ReadFile(cfg.SourcePath)
	if err != nil {
		return err
	}

	doc, err := parseMarkdown(content)
	if err != nil {
		return err
	}

	return writeHTML(cfg, doc)
}

func writeHTML(cfg config, doc article) error {
	var out bytes.Buffer

	tmpl, err := template.New(filepath.Base(cfg.TemplatePath)).
		Funcs(templateFuncs()).
		ParseFiles(cfg.TemplatePath)
	if err != nil {
		return err
	}
	if err := tmpl.Execute(&out, doc); err != nil {
		return err
	}
	return os.WriteFile(cfg.OutputPath, out.Bytes(), 0o644)
}

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"statsTable": func(rows [][]string) table {
			return table{
				Headers: []string{"Item", "Finding"},
				Rows:    rows,
			}
		},
	}
}
