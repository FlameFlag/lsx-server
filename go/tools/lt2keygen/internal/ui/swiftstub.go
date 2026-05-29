//go:build !darwin || !swiftui

package ui

import (
	"fmt"
	"os"
)

func runSwift(GenerateFunc) int {
	fmt.Fprintln(os.Stderr, "SwiftUI support is not built in. Rebuild on macOS 26+ with: go run -tags swiftui ./tools/lt2keygen -ui swiftui")
	return 2
}
