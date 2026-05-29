package main

import (
	"fmt"
	"os"

	"lt2_reverse/lsx_server_go/tools/lt2normalize/internal/normalizer"
)

func main() {
	if err := normalizer.Run(os.Args[1:], os.Stdout); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "lt2normalize: %v\n", err)
		os.Exit(1)
	}
}
