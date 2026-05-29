package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

const (
	defaultFindingsPath    = "decompiled/findings/findings.ini"
	defaultCIndexPath      = "decompiled/src/generated/findings_index.h"
	defaultCSearchDir      = "decompiled/src"
	defaultResolvedCRTPath = "decompiled/analysis/resolved_crt_wrappers.jsonl"
	installerProgram       = "Lemonade Tycoon 2 - New York City.exe"
	installerFindingPrefix = "license."
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "lt2findings: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	var findingsPath string
	var cIndexPath string
	var cSearchDir string
	var resolvedCRTPath string
	var write bool
	var check bool

	fs := flag.NewFlagSet("lt2findings", flag.ContinueOnError)
	fs.StringVar(&findingsPath, "findings", defaultFindingsPath, "INI findings file")
	fs.StringVar(&cIndexPath, "c-index", defaultCIndexPath, "generated C/X-macro index")
	fs.StringVar(&cSearchDir, "c-dir", defaultCSearchDir, "directory to scan for C symbols")
	fs.StringVar(&resolvedCRTPath, "resolved-crt", defaultResolvedCRTPath, "optional JSONL analysis event log from ResolveCrtWrappers.java")
	fs.BoolVar(&write, "write", false, "write the generated C/X-macro index")
	fs.BoolVar(&check, "check", false, "validate findings and verify the generated index is current")
	fs.Usage = func() {
		_, _ = fmt.Fprintf(fs.Output(), "Usage: %s [flags]\n\n", fs.Name())
		_, _ = fmt.Fprintln(fs.Output(), "Keeps LT2 extracted-file findings in sync between Ghidra and readable C comments.")
		_, _ = fmt.Fprintln(fs.Output())
		_, _ = fmt.Fprintln(fs.Output(), "Examples:")
		_, _ = fmt.Fprintln(fs.Output(), "  go run ./tools/lt2findings -write")
		_, _ = fmt.Fprintln(fs.Output(), "  go run ./tools/lt2findings -check")
		_, _ = fmt.Fprintln(fs.Output())
		_, _ = fmt.Fprintln(fs.Output(), "Flags:")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := chdirRepoRootForDefaultPaths(findingsPath, cIndexPath, cSearchDir, resolvedCRTPath); err != nil {
		return err
	}

	findings, err := loadFindings(findingsPath)
	if err != nil {
		return err
	}
	if err := validateFindings(findings, cSearchDir); err != nil {
		return err
	}
	cSymbols, err := scanCSymbols(cSearchDir, findings)
	if err != nil {
		return err
	}
	resolvedCRT, err := loadResolvedCRTSymbols(resolvedCRTPath, findings, cSymbols)
	if err != nil {
		return err
	}

	generated := renderCIndex(findingsPath, findings, cSymbols, resolvedCRT)
	if write {
		if err := os.WriteFile(cIndexPath, generated, 0o644); err != nil {
			return fmt.Errorf("write C index: %w", err)
		}
		fmt.Printf("wrote %s (%d findings, %d C symbols, %d resolved CRT wrappers)\n",
			cIndexPath, len(findings), len(cSymbols), len(resolvedCRT))
	}
	if check {
		current, err := os.ReadFile(cIndexPath)
		if err != nil {
			return fmt.Errorf("read C index: %w", err)
		}
		if !bytes.Equal(current, generated) {
			return fmt.Errorf("%s is out of sync; run go run ./tools/lt2findings -write", cIndexPath)
		}
		fmt.Printf("validated %d findings, %d C symbols, %d resolved CRT wrappers, and %s\n",
			len(findings), len(cSymbols), len(resolvedCRT), cIndexPath)
	}
	if !write && !check {
		fmt.Printf("validated %d findings, %d C symbols, and %d resolved CRT wrappers from %s\n",
			len(findings), len(cSymbols), len(resolvedCRT), findingsPath)
	}
	return nil
}

func chdirRepoRootForDefaultPaths(findingsPath string, cIndexPath string, cSearchDir string, resolvedCRTPath string) error {
	if findingsPath != defaultFindingsPath || cIndexPath != defaultCIndexPath || cSearchDir != defaultCSearchDir || resolvedCRTPath != defaultResolvedCRTPath {
		return nil
	}
	if _, err := os.Stat(defaultFindingsPath); err == nil {
		return nil
	}
	if _, err := os.Stat(filepath.Join("..", defaultFindingsPath)); err != nil {
		return nil
	}
	return os.Chdir("..")
}
