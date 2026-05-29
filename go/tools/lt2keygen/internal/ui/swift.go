//go:build darwin && swiftui

package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func runSwift(generate GenerateFunc) int {
	packagePath, err := swiftPackagePath()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	helper, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve helper executable: %v\n", err)
		return 1
	}

	cmd := exec.Command("swift", "run", "--package-path", packagePath, "Lemonade2Keygen", "--helper", helper)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "launch SwiftUI app: %v\n", err)
		return 1
	}
	return 0
}

func swiftPackagePath() (string, error) {
	if override := os.Getenv("LT2KEYGEN_SWIFT_PACKAGE"); override != "" {
		return override, nil
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("resolve Swift package: runtime caller unavailable")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "macos", "Lemonade2Keygen")
	if _, err := os.Stat(filepath.Join(path, "Package.swift")); err != nil {
		return "", fmt.Errorf("resolve Swift package at %s: %w", path, err)
	}
	return path, nil
}
