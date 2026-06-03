package main

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var fileArchiveMagic = [8]byte{'L', 'T', '2', 'R', 'B', 'F', 'S', '1'}

const (
	fileArchiveTypeFile = 1
	fileArchiveTypeDir  = 2
)

type fileArchiveEntry struct {
	fsPath      string
	archivePath string
	mode        fs.FileMode
	isDir       bool
}

func packFileArchive(inputPath string, outputPath string) (int, int64, error) {
	if inputPath == "" {
		return 0, 0, errors.New("input path is empty")
	}
	if outputPath == "" {
		return 0, 0, errors.New("output path is empty")
	}

	entries, err := collectFileArchiveEntries(inputPath)
	if err != nil {
		return 0, 0, err
	}
	if uint64(len(entries)) > uint64(^uint32(0)) {
		return 0, 0, errors.New("too many archive entries")
	}

	outputDir := filepath.Dir(outputPath)
	temp, err := os.CreateTemp(outputDir, "."+filepath.Base(outputPath)+".tmp-*")
	if err != nil {
		return 0, 0, fmt.Errorf("create temp output: %w", err)
	}
	tempPath := temp.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tempPath)
		}
	}()

	if _, err := temp.Write(fileArchiveMagic[:]); err != nil {
		_ = temp.Close()
		return 0, 0, fmt.Errorf("write archive header: %w", err)
	}
	if err := binary.Write(temp, binary.LittleEndian, uint32(len(entries))); err != nil {
		_ = temp.Close()
		return 0, 0, fmt.Errorf("write archive entry count: %w", err)
	}

	for _, entry := range entries {
		if err := writeFileArchiveEntry(temp, entry); err != nil {
			_ = temp.Close()
			return 0, 0, err
		}
	}

	written, err := temp.Seek(0, io.SeekEnd)
	if err != nil {
		_ = temp.Close()
		return 0, 0, fmt.Errorf("measure archive output: %w", err)
	}
	if err := temp.Close(); err != nil {
		return 0, 0, fmt.Errorf("close temp output: %w", err)
	}
	if err := os.Rename(tempPath, outputPath); err != nil {
		return 0, 0, fmt.Errorf("replace output: %w", err)
	}
	removeTemp = false
	return len(entries), written, nil
}

func collectFileArchiveEntries(inputPath string) ([]fileArchiveEntry, error) {
	info, err := os.Stat(inputPath)
	if err != nil {
		return nil, fmt.Errorf("stat archive input: %w", err)
	}
	if !info.IsDir() {
		name := filepath.Base(inputPath)
		archivePath, err := cleanArchivePath(name)
		if err != nil {
			return nil, err
		}
		return []fileArchiveEntry{{
			fsPath:      inputPath,
			archivePath: archivePath,
			mode:        info.Mode().Perm(),
		}}, nil
	}

	var entries []fileArchiveEntry
	err = filepath.WalkDir(inputPath, func(current string, dirEntry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == inputPath {
			return nil
		}
		info, err := dirEntry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("archive input contains symlink %s", current)
		}
		if !info.Mode().IsRegular() && !info.IsDir() {
			return fmt.Errorf("archive input contains unsupported file type %s", current)
		}
		rel, err := filepath.Rel(inputPath, current)
		if err != nil {
			return err
		}
		archivePath, err := cleanArchivePath(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		entries = append(entries, fileArchiveEntry{
			fsPath:      current,
			archivePath: archivePath,
			mode:        info.Mode().Perm(),
			isDir:       info.IsDir(),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk archive input: %w", err)
	}
	return entries, nil
}

func writeFileArchiveEntry(output io.Writer, entry fileArchiveEntry) error {
	kind := uint32(fileArchiveTypeFile)
	var rawSize uint64
	var compressed []byte

	if entry.isDir {
		kind = fileArchiveTypeDir
	} else {
		data, err := os.ReadFile(entry.fsPath)
		if err != nil {
			return fmt.Errorf("read archive input %s: %w", entry.fsPath, err)
		}
		rawSize = uint64(len(data))
		compressed, err = zlibBytes(data)
		if err != nil {
			return fmt.Errorf("compress archive input %s: %w", entry.fsPath, err)
		}
	}

	header := struct {
		PathLen        uint32
		Type           uint32
		Mode           uint32
		RawSize        uint64
		CompressedSize uint64
	}{
		PathLen:        uint32(len(entry.archivePath)),
		Type:           kind,
		Mode:           uint32(entry.mode.Perm()),
		RawSize:        rawSize,
		CompressedSize: uint64(len(compressed)),
	}
	if err := binary.Write(output, binary.LittleEndian, header); err != nil {
		return fmt.Errorf("write archive entry header: %w", err)
	}
	if _, err := io.WriteString(output, entry.archivePath); err != nil {
		return fmt.Errorf("write archive entry path: %w", err)
	}
	if len(compressed) > 0 {
		if _, err := output.Write(compressed); err != nil {
			return fmt.Errorf("write archive entry data: %w", err)
		}
	}
	return nil
}

func zlibBytes(data []byte) ([]byte, error) {
	var out bytes.Buffer
	zw, err := zlib.NewWriterLevel(&out, zlib.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := zw.Write(data); err != nil {
		_ = zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func unpackFileArchive(inputPath string, outputDir string) (int, error) {
	if inputPath == "" {
		return 0, errors.New("input path is empty")
	}
	if outputDir == "" {
		return 0, errors.New("output directory is empty")
	}

	input, err := os.Open(inputPath)
	if err != nil {
		return 0, fmt.Errorf("open archive input: %w", err)
	}
	defer func() { _ = input.Close() }()

	var magic [8]byte
	if _, err := io.ReadFull(input, magic[:]); err != nil {
		return 0, fmt.Errorf("read archive magic: %w", err)
	}
	if magic != fileArchiveMagic {
		return 0, errors.New("input is not an lt2rb file archive")
	}

	var count uint32
	if err := binary.Read(input, binary.LittleEndian, &count); err != nil {
		return 0, fmt.Errorf("read archive entry count: %w", err)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return 0, fmt.Errorf("create archive output directory: %w", err)
	}

	for i := uint32(0); i < count; i++ {
		if err := readFileArchiveEntry(input, outputDir); err != nil {
			return int(i), fmt.Errorf("entry %d: %w", i, err)
		}
	}
	return int(count), nil
}

func readFileArchiveEntry(input io.Reader, outputDir string) error {
	header := struct {
		PathLen        uint32
		Type           uint32
		Mode           uint32
		RawSize        uint64
		CompressedSize uint64
	}{}
	if err := binary.Read(input, binary.LittleEndian, &header); err != nil {
		return fmt.Errorf("read archive entry header: %w", err)
	}
	if header.PathLen == 0 || header.PathLen > 1<<20 {
		return fmt.Errorf("bad archive path length %d", header.PathLen)
	}
	pathBytes := make([]byte, header.PathLen)
	if _, err := io.ReadFull(input, pathBytes); err != nil {
		return fmt.Errorf("read archive entry path: %w", err)
	}
	archivePath, err := cleanArchivePath(string(pathBytes))
	if err != nil {
		return err
	}
	outputPath := filepath.Join(outputDir, filepath.FromSlash(archivePath))

	switch header.Type {
	case fileArchiveTypeDir:
		if header.RawSize != 0 || header.CompressedSize != 0 {
			return errors.New("directory entry has file payload")
		}
		return os.MkdirAll(outputPath, modeOrDefault(header.Mode, 0o755))
	case fileArchiveTypeFile:
		compressed := make([]byte, header.CompressedSize)
		if _, err := io.ReadFull(input, compressed); err != nil {
			return fmt.Errorf("read archive entry data: %w", err)
		}
		data, err := inflateZlibBytes(compressed, header.RawSize)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return fmt.Errorf("create archive output parent: %w", err)
		}
		return os.WriteFile(outputPath, data, modeOrDefault(header.Mode, 0o644))
	default:
		return fmt.Errorf("unknown archive entry type %d", header.Type)
	}
}

func inflateZlibBytes(compressed []byte, rawSize uint64) ([]byte, error) {
	if rawSize > uint64(int64(^uint64(0)>>1)) {
		return nil, errors.New("archive entry is too large for this runtime")
	}
	zr, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("initialize zlib reader: %w", err)
	}
	defer func() { _ = zr.Close() }()

	data, err := io.ReadAll(io.LimitReader(zr, int64(rawSize)+1))
	if err != nil {
		return nil, fmt.Errorf("decompress archive entry: %w", err)
	}
	if uint64(len(data)) != rawSize {
		return nil, fmt.Errorf("decompressed size mismatch: got %d, want %d", len(data), rawSize)
	}
	return data, nil
}

func cleanArchivePath(name string) (string, error) {
	if name == "" {
		return "", errors.New("archive path is empty")
	}
	if strings.Contains(name, "\\") {
		return "", fmt.Errorf("archive path %q contains a backslash", name)
	}
	for part := range strings.SplitSeq(name, "/") {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("unsafe archive path %q", name)
		}
	}
	cleaned := path.Clean(name)
	if cleaned == "." || path.IsAbs(cleaned) || strings.HasPrefix(cleaned, "../") || cleaned == ".." {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	return cleaned, nil
}

func modeOrDefault(mode uint32, fallback fs.FileMode) fs.FileMode {
	perm := fs.FileMode(mode & 0o777)
	if perm == 0 {
		return fallback
	}
	return perm
}
