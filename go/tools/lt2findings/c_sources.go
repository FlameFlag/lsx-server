package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func readCText(root string) (string, error) {
	var b strings.Builder
	err := walkCSourceFiles(root, func(path string) error {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		b.Write(data)
		b.WriteByte('\n')
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("scan C sources: %w", err)
	}
	return b.String(), nil
}

func walkCSourceFiles(root string, visit func(path string) error) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldSkipCSourceDir(filepath.Base(path)) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isCSourceFile(path) {
			return nil
		}
		return visit(path)
	})
}

func shouldSkipCSourceDir(base string) bool {
	switch base {
	case "generated", "lt2_install", "local", "ghidra_projects":
		return true
	default:
		return false
	}
}

func isCSourceFile(path string) bool {
	switch filepath.Ext(path) {
	case ".c", ".h":
		return true
	default:
		return false
	}
}
