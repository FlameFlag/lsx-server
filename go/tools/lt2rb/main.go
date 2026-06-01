package main

import (
	"bytes"
	"compress/bzip2"
	"crypto/md5"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	defaultRBOffset = int64(0xFEA4C)
	defaultRBLength = int64(8072295)
	defaultOutput   = "Lemonade2.rb"
	sourceURL       = "https://www.myabandonware.com/game/lemonade-tycoon-2-c4g"
)

type config struct {
	input          string
	output         string
	outputSet      bool
	offset         int64
	length         int64
	compressRB     bool
	extractImages  string
	rbInput        bool
	noTransparency bool
	roundtripMD5   bool
	scan           bool
	quiet          bool
}

type byteCountFlag struct {
	value *int64
}

type outputFlag struct {
	cfg *config
}

func (f outputFlag) Set(s string) error {
	f.cfg.output = s
	f.cfg.outputSet = true
	return nil
}

func (f outputFlag) String() string {
	if f.cfg == nil {
		return ""
	}
	return f.cfg.output
}

func (f byteCountFlag) Set(s string) error {
	value, err := parseByteCount(s)
	if err != nil {
		return err
	}
	*f.value = value
	return nil
}

func (f byteCountFlag) String() string {
	if f.value == nil {
		return ""
	}
	return fmt.Sprintf("0x%X", *f.value)
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "lt2rb: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer, stderr io.Writer) error {
	cfg := config{
		output: defaultOutput,
		offset: defaultRBOffset,
		length: defaultRBLength,
	}

	fs := flag.NewFlagSet("lt2rb", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Var(byteCountFlag{value: &cfg.offset}, "offset", "compressed stream byte offset; accepts decimal or 0x-prefixed hex")
	fs.Var(byteCountFlag{value: &cfg.length}, "length", "compressed byte count; use 0 to read to end of input")
	fs.BoolVar(&cfg.compressRB, "compress-rb", false, "treat input as a decompressed .rb file and write a bzip2 stream")
	fs.StringVar(&cfg.extractImages, "extract-images", "", "directory where bitmap records should be written as PNGs")
	fs.BoolVar(&cfg.rbInput, "rb-input", false, "treat input as an already decompressed Lemonade2.rb file")
	fs.BoolVar(&cfg.noTransparency, "no-transparency", false, "preserve chroma-key pixels instead of making them transparent")
	fs.Var(outputFlag{cfg: &cfg}, "output", "output .rb or compressed stream path")
	fs.BoolVar(&cfg.roundtripMD5, "roundtrip-md5", false, "decompress, recompress, and require the compressed MD5 to match the original section")
	fs.BoolVar(&cfg.scan, "scan", false, "list bzip2 stream candidates and exit")
	fs.BoolVar(&cfg.quiet, "quiet", false, "suppress success output")
	fs.Usage = func() {
		_, _ = fmt.Fprintf(stderr, "Usage: %s [flags] installer.exe [output.rb]\n\n", fs.Name())
		_, _ = fmt.Fprintln(stderr, "Decompresses the Lemonade Tycoon 2 resource-bundle payload from a setup EXE you provide.")
		_, _ = fmt.Fprintf(stderr, "Defaults target Lemonade2.rb at offset 0x%X with length %d.\n\n", defaultRBOffset, defaultRBLength)
		_, _ = fmt.Fprintf(stderr, "You can get the installer from:\n  %s\n", sourceURL)
		_, _ = fmt.Fprintln(stderr, "Download/extract the setup EXE first, then pass that EXE to this tool.")
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
	if len(positional) == 0 {
		fs.Usage()
		return errors.New("missing installer input path")
	}
	if len(positional) > 2 {
		return fmt.Errorf("expected at most input and output paths, got %d arguments", len(positional))
	}

	cfg.input = positional[0]
	if len(positional) == 2 {
		if cfg.rbInput {
			return errors.New("-rb-input accepts only one positional input path")
		}
		cfg.output = positional[1]
		cfg.outputSet = true
	}
	if cfg.compressRB && cfg.output == defaultOutput && !cfg.outputSet {
		cfg.output = cfg.input + ".bz2"
	}

	if cfg.scan {
		if cfg.rbInput {
			return errors.New("-scan expects an installer input, not -rb-input")
		}
		if cfg.compressRB || cfg.roundtripMD5 {
			return errors.New("-scan cannot be combined with compression modes")
		}
		return printBzip2Offsets(stdout, cfg.input)
	}

	if cfg.compressRB {
		if cfg.rbInput || cfg.roundtripMD5 {
			return errors.New("-compress-rb cannot be combined with -rb-input or -roundtrip-md5")
		}
		written, sum, err := compressRBFile(cfg.input, cfg.output)
		if err != nil {
			return err
		}
		if !cfg.quiet {
			_, _ = fmt.Fprintf(stdout, "wrote %s (%d bytes, md5 %x)\n", cfg.output, written, sum)
		}
		return nil
	}

	if cfg.rbInput {
		if cfg.roundtripMD5 {
			return errors.New("-roundtrip-md5 expects an installer input, not -rb-input")
		}
		if cfg.extractImages == "" {
			return errors.New("-rb-input requires -extract-images")
		}
		rb, err := os.ReadFile(cfg.input)
		if err != nil {
			return fmt.Errorf("read rb input: %w", err)
		}
		count, err := extractBitmapPNGs(rb, cfg.extractImages, !cfg.noTransparency)
		if err != nil {
			return err
		}
		if !cfg.quiet {
			_, _ = fmt.Fprintf(stdout, "wrote %d bitmap PNG(s) to %s\n", count, cfg.extractImages)
		}
		return nil
	}

	written, err := decompressBzip2Section(cfg.input, cfg.output, cfg.offset, cfg.length)
	if err != nil {
		return err
	}
	if !cfg.quiet {
		_, _ = fmt.Fprintf(stdout, "wrote %s (%d bytes)\n", cfg.output, written)
	}

	if cfg.roundtripMD5 {
		match, originalMD5, recompressedMD5, compressedPath, err := roundtripMD5(cfg.input, cfg.output, cfg.offset, cfg.length)
		if err != nil {
			return err
		}
		if !cfg.quiet {
			_, _ = fmt.Fprintf(stdout, "recompressed %s\n", compressedPath)
			_, _ = fmt.Fprintf(stdout, "original compressed md5:     %x\n", originalMD5)
			_, _ = fmt.Fprintf(stdout, "recompressed stream md5:    %x\n", recompressedMD5)
		}
		if !match {
			return errors.New("round-trip compressed MD5 mismatch")
		}
		if !cfg.quiet {
			_, _ = fmt.Fprintln(stdout, "round-trip compressed MD5 matches")
		}
	}

	if cfg.extractImages != "" {
		rb, err := os.ReadFile(cfg.output)
		if err != nil {
			return fmt.Errorf("read decompressed rb: %w", err)
		}
		count, err := extractBitmapPNGs(rb, cfg.extractImages, !cfg.noTransparency)
		if err != nil {
			return err
		}
		if !cfg.quiet {
			_, _ = fmt.Fprintf(stdout, "wrote %d bitmap PNG(s) to %s\n", count, cfg.extractImages)
		}
	}

	return nil
}

func compressRBFile(inputPath string, outputPath string) (int64, [md5.Size]byte, error) {
	if inputPath == "" {
		return 0, [md5.Size]byte{}, errors.New("input path is empty")
	}
	if outputPath == "" {
		return 0, [md5.Size]byte{}, errors.New("output path is empty")
	}
	if _, err := exec.LookPath("bzip2"); err != nil {
		return 0, [md5.Size]byte{}, fmt.Errorf("bzip2 executable is required for compression: %w", err)
	}

	outputDir := filepath.Dir(outputPath)
	temp, err := os.CreateTemp(outputDir, "."+filepath.Base(outputPath)+".tmp-*")
	if err != nil {
		return 0, [md5.Size]byte{}, fmt.Errorf("create temp output: %w", err)
	}
	tempPath := temp.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tempPath)
		}
	}()

	hash := md5.New()
	cmd := exec.Command("bzip2", "-c", "-9", "--", inputPath)
	cmd.Stdout = io.MultiWriter(temp, hash)
	stderr := new(bytes.Buffer)
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		_ = temp.Close()
		return 0, [md5.Size]byte{}, fmt.Errorf("compress rb with bzip2: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	written, err := temp.Seek(0, io.SeekEnd)
	if err != nil {
		_ = temp.Close()
		return 0, [md5.Size]byte{}, fmt.Errorf("measure compressed output: %w", err)
	}
	if err := temp.Close(); err != nil {
		return 0, [md5.Size]byte{}, fmt.Errorf("close temp output: %w", err)
	}
	if err := os.Rename(tempPath, outputPath); err != nil {
		return 0, [md5.Size]byte{}, fmt.Errorf("replace output: %w", err)
	}
	removeTemp = false

	var sum [md5.Size]byte
	copy(sum[:], hash.Sum(nil))
	return written, sum, nil
}

func roundtripMD5(installerPath string, rbPath string, offset int64, length int64) (bool, [md5.Size]byte, [md5.Size]byte, string, error) {
	original, err := readCompressedSection(installerPath, offset, length)
	if err != nil {
		return false, [md5.Size]byte{}, [md5.Size]byte{}, "", err
	}
	originalMD5 := md5.Sum(original)

	compressedPath := rbPath + ".bz2"
	_, recompressedMD5, err := compressRBFile(rbPath, compressedPath)
	if err != nil {
		return false, originalMD5, [md5.Size]byte{}, compressedPath, err
	}
	return originalMD5 == recompressedMD5, originalMD5, recompressedMD5, compressedPath, nil
}

func readCompressedSection(inputPath string, offset int64, length int64) ([]byte, error) {
	input, err := os.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("open input: %w", err)
	}
	defer func() { _ = input.Close() }()

	stat, err := input.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat input: %w", err)
	}
	if offset >= stat.Size() {
		return nil, fmt.Errorf("offset 0x%X is past end of input (%d bytes)", offset, stat.Size())
	}
	sectionLength := length
	if sectionLength == 0 {
		sectionLength = stat.Size() - offset
	}
	if sectionLength > stat.Size()-offset {
		return nil, fmt.Errorf("section 0x%X+%d exceeds input size %d", offset, sectionLength, stat.Size())
	}
	data := make([]byte, sectionLength)
	if _, err := input.ReadAt(data, offset); err != nil {
		return nil, fmt.Errorf("read compressed section: %w", err)
	}
	return data, nil
}

func parseByteCount(s string) (int64, error) {
	value, err := strconv.ParseInt(strings.TrimSpace(s), 0, 64)
	if err != nil {
		return 0, err
	}
	if value < 0 {
		return 0, errors.New("value must not be negative")
	}
	return value, nil
}

func decompressBzip2Section(inputPath string, outputPath string, offset int64, length int64) (int64, error) {
	if inputPath == "" {
		return 0, errors.New("input path is empty")
	}
	if outputPath == "" {
		return 0, errors.New("output path is empty")
	}

	input, err := os.Open(inputPath)
	if err != nil {
		return 0, fmt.Errorf("open input: %w", err)
	}
	defer func() { _ = input.Close() }()

	stat, err := input.Stat()
	if err != nil {
		return 0, fmt.Errorf("stat input: %w", err)
	}
	if offset >= stat.Size() {
		return 0, fmt.Errorf("offset 0x%X is past end of input (%d bytes)", offset, stat.Size())
	}

	sectionLength := length
	if sectionLength == 0 {
		sectionLength = stat.Size() - offset
	}
	if sectionLength > stat.Size()-offset {
		return 0, fmt.Errorf("section 0x%X+%d exceeds input size %d", offset, sectionLength, stat.Size())
	}

	section := io.NewSectionReader(input, offset, sectionLength)
	magic := make([]byte, 3)
	if _, err := section.ReadAt(magic, 0); err != nil {
		return 0, fmt.Errorf("read bzip2 magic: %w", err)
	}
	if !bytes.Equal(magic, []byte("BZh")) {
		return 0, fmt.Errorf("section at 0x%X does not start with bzip2 magic BZh", offset)
	}

	outputDir := filepath.Dir(outputPath)
	temp, err := os.CreateTemp(outputDir, "."+filepath.Base(outputPath)+".tmp-*")
	if err != nil {
		return 0, fmt.Errorf("create temp output: %w", err)
	}
	tempPath := temp.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tempPath)
		}
	}()

	written, copyErr := io.Copy(temp, bzip2.NewReader(section))
	closeErr := temp.Close()
	if copyErr != nil {
		return written, fmt.Errorf("decompress bzip2 stream: %w", copyErr)
	}
	if closeErr != nil {
		return written, fmt.Errorf("close temp output: %w", closeErr)
	}
	if err := os.Rename(tempPath, outputPath); err != nil {
		return written, fmt.Errorf("replace output: %w", err)
	}
	removeTemp = false

	return written, nil
}

func printBzip2Offsets(w io.Writer, inputPath string) error {
	offsets, err := findBzip2Offsets(inputPath)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(w, "found %d bzip2 stream candidate(s)\n", len(offsets))
	for _, offset := range offsets {
		_, _ = fmt.Fprintf(w, "0x%X (%d)\n", offset, offset)
	}
	return nil
}

func findBzip2Offsets(inputPath string) ([]int64, error) {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}

	offsets := make([]int64, 0)
	for i := 0; i+4 <= len(data); i++ {
		if bytes.Equal(data[i:i+3], []byte("BZh")) && data[i+3] >= '1' && data[i+3] <= '9' {
			offsets = append(offsets, int64(i))
		}
	}
	return offsets, nil
}
