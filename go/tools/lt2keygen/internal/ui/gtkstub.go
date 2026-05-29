//go:build !gtk

package ui

import (
	"fmt"
	"os"
)

func runGTK(GenerateFunc) int {
	fmt.Fprintln(os.Stderr, "GTK UI support is not built in. Rebuild with: go run -tags gtk ./tools/lt2keygen -ui gtk")
	return 2
}
