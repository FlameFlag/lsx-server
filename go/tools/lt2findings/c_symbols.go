package main

import (
	"fmt"
	"sort"
)

func scanCSymbols(root string, findings []finding) ([]cSymbol, error) {
	linked := map[string]string{}
	for _, f := range findings {
		if f.CSymbol != "" {
			linked[f.CSymbol] = f.ID
		}
	}

	var symbols []cSymbol
	seen := map[string]bool{}
	err := walkCSourceFiles(root, func(path string) error {
		fileSymbols, err := scanCSymbolsInFile(path, linked)
		if err != nil {
			return err
		}
		for _, symbol := range fileSymbols {
			key := symbol.Kind + "\x00" + symbol.Name + "\x00" + symbol.Source
			if seen[key] {
				continue
			}
			seen[key] = true
			symbols = append(symbols, symbol)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan C symbols: %w", err)
	}
	sort.SliceStable(symbols, func(i, j int) bool {
		if symbols[i].Source == symbols[j].Source {
			if symbols[i].Line == symbols[j].Line {
				return symbols[i].Name < symbols[j].Name
			}
			return symbols[i].Line < symbols[j].Line
		}
		return symbols[i].Source < symbols[j].Source
	})
	return symbols, nil
}
