package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestPackAndUnpackFileArchive(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input")
	if err := os.MkdirAll(filepath.Join(input, "nested", "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(input, "root.txt"), []byte("root\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(input, "nested", "data.bin"), []byte{0, 1, 2, 3, 4}, 0o600); err != nil {
		t.Fatal(err)
	}

	archive := filepath.Join(dir, "bundle.rb")
	entries, written, err := packFileArchive(input, archive)
	if err != nil {
		t.Fatal(err)
	}
	if entries != 4 {
		t.Fatalf("entries = %d, want 4", entries)
	}
	if written <= int64(len(fileArchiveMagic)) {
		t.Fatalf("written = %d, want archive data", written)
	}

	output := filepath.Join(dir, "output")
	unpacked, err := unpackFileArchive(archive, output)
	if err != nil {
		t.Fatal(err)
	}
	if unpacked != entries {
		t.Fatalf("unpacked = %d, want %d", unpacked, entries)
	}
	assertFileBytes(t, filepath.Join(output, "root.txt"), []byte("root\n"))
	assertFileBytes(t, filepath.Join(output, "nested", "data.bin"), []byte{0, 1, 2, 3, 4})
	if info, err := os.Stat(filepath.Join(output, "nested", "empty")); err != nil || !info.IsDir() {
		t.Fatalf("empty directory was not restored: %v", err)
	}
}

func TestUnpackFileArchiveRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "bad.rb")
	var buf bytes.Buffer
	buf.Write(fileArchiveMagic[:])
	writeLE(t, &buf, uint32(1))
	writeLE(t, &buf, uint32(len("../bad.txt")))
	writeLE(t, &buf, uint32(fileArchiveTypeDir))
	writeLE(t, &buf, uint32(0o755))
	writeLE(t, &buf, uint64(0))
	writeLE(t, &buf, uint64(0))
	buf.WriteString("../bad.txt")
	if err := os.WriteFile(archive, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := unpackFileArchive(archive, filepath.Join(dir, "out")); err == nil {
		t.Fatal("unpackFileArchive succeeded, want traversal error")
	}
	if _, err := cleanArchivePath("a/../bad.txt"); err == nil {
		t.Fatal("cleanArchivePath accepted a path with traversal")
	}
}

func assertFileBytes(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s = %x, want %x", path, got, want)
	}
}

func writeLE(t *testing.T, buf *bytes.Buffer, value any) {
	t.Helper()
	if err := binary.Write(buf, binary.LittleEndian, value); err != nil {
		t.Fatal(err)
	}
}
