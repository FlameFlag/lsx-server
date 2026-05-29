//go:build !windows || !windigo

package ui

import (
	"fmt"
	"os"
)

func runWindows(GenerateFunc) int {
	fmt.Fprintln(os.Stderr, "Windows native UI support is not built in. Rebuild on Windows with: go run -tags windigo ./tools/lt2keygen -ui windows")
	return 2
}
