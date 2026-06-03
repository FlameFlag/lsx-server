package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "lt2rb: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("lt2rb", flag.ContinueOnError)
	fs.SetOutput(stderr)
	quiet := fs.Bool("quiet", false, "suppress success output")
	fs.Usage = func() {
		_, _ = fmt.Fprintf(stderr, "Usage:\n")
		_, _ = fmt.Fprintf(stderr, "  %s unpack input.rb output-dir\n", fs.Name())
		_, _ = fmt.Fprintf(stderr, "  %s pack input-file-or-dir output.rb\n\n", fs.Name())
		_, _ = fmt.Fprintln(stderr, "Unpacks an .rb into usable assets, or packs a file/folder into an .rb.")
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

	positional := fs.Args()
	if len(positional) != 3 {
		fs.Usage()
		return fmt.Errorf("expected command, input, and output; got %d argument(s)", len(positional))
	}

	command, input, output := positional[0], positional[1], positional[2]
	switch command {
	case "unpack", "decompress", "extract":
		count, err := unpackAssets(input, output)
		if err != nil {
			return err
		}
		if !*quiet {
			_, _ = fmt.Fprintf(stdout, "unpacked %d asset(s) to %s\n", count, output)
		}
	case "pack", "compress":
		count, written, err := packFileArchive(input, output)
		if err != nil {
			return err
		}
		if !*quiet {
			_, _ = fmt.Fprintf(stdout, "packed %d asset(s) into %s (%d bytes)\n", count, output, written)
		}
	default:
		fs.Usage()
		return fmt.Errorf("unknown command %q", command)
	}
	return nil
}

func unpackAssets(inputPath string, outputDir string) (int, error) {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return 0, fmt.Errorf("read rb input: %w", err)
	}

	if len(data) >= len(fileArchiveMagic) && bytes.Equal(data[:len(fileArchiveMagic)], fileArchiveMagic[:]) {
		return unpackFileArchive(inputPath, outputDir)
	}

	bitmapDir := filepath.Join(outputDir, "bitmaps")
	count, err := extractBitmapPNGs(data, bitmapDir, true)
	if err != nil {
		return 0, fmt.Errorf("unpack Lemonade2 rb assets: %w", err)
	}
	return count, nil
}
