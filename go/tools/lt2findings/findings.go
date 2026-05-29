package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var hexAddress = regexp.MustCompile(`^[0-9A-Fa-f]+$`)

func loadFindings(path string) ([]finding, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("open findings file: %w", err)
	}
	return parseFindingsINI(path, string(data))
}

func parseFindingsINI(path string, data string) ([]finding, error) {
	var findings []finding
	var current *finding

	flush := func() error {
		if current == nil {
			return nil
		}
		findings = append(findings, *current)
		current = nil
		return nil
	}

	lines := strings.Split(strings.ReplaceAll(data, "\r\n", "\n"), "\n")
	for i, raw := range lines {
		lineNo := i + 1
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			if !strings.HasSuffix(line, "]") {
				return nil, fmt.Errorf("%s:%d: malformed section header", path, lineNo)
			}
			if err := flush(); err != nil {
				return nil, err
			}
			id := strings.TrimSpace(line[1 : len(line)-1])
			if id == "" {
				return nil, fmt.Errorf("%s:%d: empty finding id", path, lineNo)
			}
			current = &finding{ID: id}
			continue
		}

		if current == nil {
			return nil, fmt.Errorf("%s:%d: key/value before first finding section", path, lineNo)
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("%s:%d: expected key = value", path, lineNo)
		}
		if err := setFindingField(current, strings.TrimSpace(key), strings.TrimSpace(value)); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, lineNo, err)
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	if len(findings) == 0 {
		return nil, errors.New("findings file has no sections")
	}
	return findings, nil
}

func setFindingField(f *finding, key string, value string) error {
	switch key {
	case "source_file":
		f.SourceFile = value
	case "program":
		f.Program = value
	case "address":
		f.Address = strings.ToUpper(value)
	case "kind":
		f.Kind = value
	case "label":
		f.Label = value
	case "title":
		f.Title = value
	case "c_symbol":
		f.CSymbol = value
	case "comment":
		f.Comment = value
	default:
		return fmt.Errorf("unknown finding field %q", key)
	}
	return nil
}

func validateFindings(findings []finding, cSearchDir string) error {
	seen := map[string]bool{}
	allowedKinds := map[string]bool{
		"function": true,
		"pre":      true,
		"string":   true,
	}
	cText, err := readCText(cSearchDir)
	if err != nil {
		return err
	}

	for _, f := range findings {
		if f.ID == "" {
			return errors.New("finding has empty id")
		}
		if seen[f.ID] {
			return fmt.Errorf("duplicate finding id %q", f.ID)
		}
		seen[f.ID] = true
		if f.SourceFile == "" || f.Program == "" || f.Address == "" ||
			f.Kind == "" || f.Label == "" || f.Title == "" || f.Comment == "" {
			return fmt.Errorf("%s: required field is empty", f.ID)
		}
		if !isAllowedSourceFile(f) {
			return fmt.Errorf("%s: source_file must point to an LT2 install file, local normalized target, or local installer license-manager target; got %q", f.ID, f.SourceFile)
		}
		if filepath.Base(f.SourceFile) != f.Program {
			return fmt.Errorf("%s: program %q does not match source file basename %q", f.ID, f.Program, filepath.Base(f.SourceFile))
		}
		if !hexAddress.MatchString(f.Address) {
			return fmt.Errorf("%s: address %q is not hex", f.ID, f.Address)
		}
		if !allowedKinds[f.Kind] {
			return fmt.Errorf("%s: unsupported kind %q", f.ID, f.Kind)
		}
		if f.CSymbol != "" && !strings.Contains(cText, f.CSymbol) {
			return fmt.Errorf("%s: c_symbol %q was not found under %s", f.ID, f.CSymbol, cSearchDir)
		}
	}
	return nil
}

func isAllowedSourceFile(f finding) bool {
	if strings.HasPrefix(f.SourceFile, "decompiled/local/lt2_install/") ||
		strings.HasPrefix(f.SourceFile, "decompiled/local/pdata_assets/") ||
		strings.HasPrefix(f.SourceFile, "decompiled/local/unpacked/") {
		return true
	}
	if f.Program == installerProgram && filepath.Base(f.SourceFile) == installerProgram {
		return strings.HasPrefix(f.SourceFile, "decompiled/local/installers/") &&
			strings.HasPrefix(f.ID, installerFindingPrefix)
	}
	return false
}
