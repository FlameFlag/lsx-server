//go:build windows && windigo

package ui

func runDefault(generate GenerateFunc) int {
	return runWindows(generate)
}
