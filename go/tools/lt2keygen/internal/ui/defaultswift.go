//go:build darwin && swiftui

package ui

func runDefault(generate GenerateFunc) int {
	return runSwift(generate)
}
