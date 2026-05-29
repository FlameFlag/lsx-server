//go:build (!darwin || !swiftui) && (!windows || !windigo) && (!gtk || (darwin && swiftui) || (windows && windigo))

package ui

import (
	"fmt"
	"os"
)

func runDefault(GenerateFunc) int {
	fmt.Fprintln(os.Stderr, "No default GUI backend is built in. Rebuild with -tags swiftui on macOS, -tags windigo on Windows, or -tags gtk for GTK4.")
	return 2
}
