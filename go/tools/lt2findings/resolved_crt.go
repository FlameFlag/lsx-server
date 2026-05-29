package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type analysisEvent struct {
	Schema     string `json:"schema"`
	Stage      string `json:"stage"`
	Program    string `json:"program"`
	Address    string `json:"address"`
	Action     string `json:"action"`
	SymbolName string `json:"symbol_name"`
	OldName    string `json:"old_name"`
	NewName    string `json:"new_name"`
	Category   string `json:"category"`
	Confidence int    `json:"confidence"`
	Decision   string `json:"decision"`
	Evidence   string `json:"evidence"`
}

func loadResolvedCRTSymbols(path string, findings []finding, cSymbols []cSymbol) ([]cSymbol, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read resolved CRT wrappers: %w", err)
	}

	conflicts := map[string]string{}
	for _, f := range findings {
		conflicts[f.Label] = "finding " + f.ID
	}
	for _, s := range cSymbols {
		conflicts[s.Name] = "C symbol " + s.Source
	}

	var symbols []cSymbol
	seen := map[string]bool{}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	for i, raw := range lines {
		lineNo := i + 1
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var event analysisEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, fmt.Errorf("%s:%d: decode JSONL analysis event: %w", path, lineNo, err)
		}
		if event.Schema != "lt2.analysis_event.v1" {
			return nil, fmt.Errorf("%s:%d: unsupported analysis event schema %q", path, lineNo, event.Schema)
		}
		if event.Stage != "crt_resolver" {
			continue
		}
		if event.Category != "crt_wrapper" || event.Decision != "accepted" {
			continue
		}
		name := strings.TrimSpace(event.SymbolName)
		if name == "" {
			name = strings.TrimSpace(event.NewName)
		}
		if name == "" {
			return nil, fmt.Errorf("%s:%d: empty resolved CRT name", path, lineNo)
		}
		if event.Action != "" && event.Action != "renamed" && event.Action != "confirmed_existing" {
			return nil, fmt.Errorf("%s:%d: unsupported CRT resolver action %q", path, lineNo, event.Action)
		}
		if owner, ok := conflicts[name]; ok {
			return nil, fmt.Errorf("%s:%d: resolved CRT name %q conflicts with %s", path, lineNo, name, owner)
		}
		key := strings.Join([]string{
			strings.TrimSpace(event.Program),
			strings.TrimSpace(event.Address),
			name,
		}, "\x00")
		if seen[key] {
			continue
		}
		seen[key] = true
		symbols = append(symbols, cSymbol{
			Kind:      "resolved_crt_wrapper",
			Name:      name,
			Source:    filepath.ToSlash(path),
			Line:      lineNo,
			FindingID: "",
		})
	}
	sort.SliceStable(symbols, func(i, j int) bool {
		return symbols[i].Name < symbols[j].Name
	})
	return symbols, nil
}
