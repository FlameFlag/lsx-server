package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	root, err := repoRoot()
	if err != nil {
		fatal(err)
	}

	cfg := defaultConfig(root)
	flag.StringVar(&cfg.SourcePath, "source", cfg.SourcePath, "markdown source path")
	flag.StringVar(&cfg.TemplatePath, "template", cfg.TemplatePath, "HTML fragment template path")
	flag.StringVar(&cfg.OutputPath, "output", cfg.OutputPath, "generated HTML fragment output path")
	flag.Parse()

	if err := generate(cfg); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
