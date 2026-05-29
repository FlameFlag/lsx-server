//go:build gtk && (!darwin || !swiftui) && (!windows || !windigo)

package ui

func runDefault(generate GenerateFunc) int {
	return runGTK(generate)
}
