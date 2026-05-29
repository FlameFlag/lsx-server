package ui

import (
	"fmt"
	"os"
)

type Response struct {
	RegistrationName string
	ActivationKey    string
	KeyFormat        string
	Note             string
}

type GenerateFunc func(name string) (Response, error)

func Run(mode string, generate GenerateFunc) int {
	switch mode {
	case "auto":
		return runDefault(generate)
	case "gtk":
		return runGTK(generate)
	case "swiftui":
		return runSwift(generate)
	case "windows", "windigo", "win32":
		return runWindows(generate)
	default:
		fmt.Fprintf(os.Stderr, "unknown UI mode %q\n", mode)
		return 2
	}
}
