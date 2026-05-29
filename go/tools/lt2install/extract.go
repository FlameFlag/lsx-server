package main

import (
	"compress/bzip2"
	"compress/zlib"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func extractEntry(installer *os.File, installerSize int64, entry installEntry, outputPath string, force bool) error {
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("%s: create output directory: %w", entry.name, err)
	}

	temp, err := os.CreateTemp(outputDir, "."+filepath.Base(outputPath)+".tmp-*")
	if err != nil {
		return fmt.Errorf("%s: create temp output: %w", entry.name, err)
	}
	tempPath := temp.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tempPath)
		}
	}()

	hasher := md5.New()
	written, copyErr := writeEntryParts(io.MultiWriter(temp, hasher), installer, installerSize, entry)
	closeErr := temp.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return fmt.Errorf("%s: close temp output: %w", entry.name, closeErr)
	}

	if written != entry.size {
		return fmt.Errorf("%s: wrote %d bytes, expected %d", entry.name, written, entry.size)
	}
	sum := hex.EncodeToString(hasher.Sum(nil))
	if sum != entry.md5 {
		return fmt.Errorf("%s: MD5 %s, expected %s", entry.name, sum, entry.md5)
	}

	if err := os.Chmod(tempPath, entry.mode); err != nil {
		return fmt.Errorf("%s: chmod temp output: %w", entry.name, err)
	}
	if force {
		if err := os.Remove(outputPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%s: remove existing output: %w", entry.name, err)
		}
	}
	if err := os.Rename(tempPath, outputPath); err != nil {
		return fmt.Errorf("%s: replace output: %w", entry.name, err)
	}
	removeTemp = false

	return nil
}

func writeEntryParts(dst io.Writer, installer *os.File, installerSize int64, entry installEntry) (int64, error) {
	var written int64
	for _, part := range entry.parts {
		if part.offset < 0 || part.length <= 0 {
			return written, fmt.Errorf("%s: invalid %s section 0x%X+%d", entry.name, part.algo, part.offset, part.length)
		}
		if part.offset+part.length > installerSize {
			return written, fmt.Errorf("%s: section 0x%X+%d exceeds installer size %d", entry.name, part.offset, part.length, installerSize)
		}

		section := io.NewSectionReader(installer, part.offset, part.length)
		if err := checkMagic(section, entry.name, part); err != nil {
			return written, err
		}

		reader, closeReader, err := partReader(section, part.algo)
		if err != nil {
			return written, fmt.Errorf("%s: open %s stream at 0x%X: %w", entry.name, part.algo, part.offset, err)
		}
		n, copyErr := io.Copy(dst, reader)
		closeReader()
		written += n
		if copyErr != nil {
			return written, fmt.Errorf("%s: extract %s stream at 0x%X: %w", entry.name, part.algo, part.offset, copyErr)
		}
	}
	return written, nil
}

func partReader(reader io.Reader, algo compression) (io.Reader, func(), error) {
	switch algo {
	case compressionRaw:
		return reader, func() {}, nil
	case compressionBzip2:
		return bzip2.NewReader(reader), func() {}, nil
	case compressionZlib:
		rc, err := zlib.NewReader(reader)
		if err != nil {
			return nil, nil, err
		}
		return rc, func() { _ = rc.Close() }, nil
	default:
		return nil, nil, fmt.Errorf("unknown compression %q", algo)
	}
}

func checkMagic(section *io.SectionReader, entryName string, part archivePart) error {
	var want []byte
	switch part.algo {
	case compressionRaw:
		return nil
	case compressionBzip2:
		want = []byte("BZh")
	case compressionZlib:
		want = []byte{0x78, 0xDA}
	default:
		return fmt.Errorf("%s: unknown compression %q", entryName, part.algo)
	}

	got := make([]byte, len(want))
	if _, err := section.ReadAt(got, 0); err != nil {
		return fmt.Errorf("%s: read stream magic: %w", entryName, err)
	}
	for i := range want {
		if got[i] != want[i] {
			return fmt.Errorf("%s: section at 0x%X has magic % X, expected % X", entryName, part.offset, got, want)
		}
	}
	if _, err := section.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("%s: reset section reader: %w", entryName, err)
	}
	return nil
}
