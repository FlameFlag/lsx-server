package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

type config struct {
	force  bool
	list   bool
	dryRun bool
	quiet  bool
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "lt2install: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer, stderr io.Writer) error {
	cfg := config{}

	fs := flag.NewFlagSet("lt2install", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.BoolVar(&cfg.force, "force", false, "overwrite existing output files")
	fs.BoolVar(&cfg.list, "list", false, "list the payload manifest and exit")
	fs.BoolVar(&cfg.dryRun, "dry-run", false, "print planned writes without extracting")
	fs.BoolVar(&cfg.quiet, "quiet", false, "suppress non-error output")
	fs.Usage = func() {
		_, _ = fmt.Fprintf(stderr, "Usage: %s [flags] installer.exe output-dir\n\n", fs.Name())
		_, _ = fmt.Fprintln(stderr, "Recreates the Lemonade Tycoon 2 install directory from the Clickteam setup EXE.")
		_, _ = fmt.Fprintln(stderr)
		_, _ = fmt.Fprintln(stderr, "Example:")
		_, _ = fmt.Fprintf(stderr, "  go run ./tools/lt2install -- %q decompiled/local/lt2_install\n", "decompiled/local/installers/Lemonade Tycoon 2 - New York City.exe")
		_, _ = fmt.Fprintln(stderr)
		_, _ = fmt.Fprintln(stderr, "Flags:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	if cfg.list {
		printManifest(stdout)
		return nil
	}

	positional := fs.Args()
	if len(positional) != 2 {
		fs.Usage()
		return fmt.Errorf("expected installer path and output directory, got %d argument(s)", len(positional))
	}

	return install(cfg, positional[0], positional[1], stdout, stderr)
}
