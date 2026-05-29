package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseByteCount(t *testing.T) {
	tests := map[string]int64{
		"42":      42,
		"0xFEA4C": 0xFEA4C,
		"0":       0,
	}

	for input, want := range tests {
		got, err := parseByteCount(input)
		if err != nil {
			t.Fatalf("parseByteCount(%q): %v", input, err)
		}
		if got != want {
			t.Fatalf("parseByteCount(%q) = %d, want %d", input, got, want)
		}
	}
}

func TestDecompressBzip2Section(t *testing.T) {
	fixture, err := hex.DecodeString("425a68393141592653598979dccd000002518000104000124490002000310c08200f296268c3e2ee48a70a12112f3b99a0")
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	input := filepath.Join(dir, "installer.exe")
	output := filepath.Join(dir, "Lemonade2.rb")
	payload := append([]byte("JUNK"), fixture...)
	payload = append(payload, []byte("TRAILER")...)

	if err := os.WriteFile(input, payload, 0o644); err != nil {
		t.Fatal(err)
	}

	offsets, err := findBzip2Offsets(input)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(offsets, []int64{4}) {
		t.Fatalf("findBzip2Offsets() = %v, want [4]", offsets)
	}

	written, err := decompressBzip2Section(input, output, 4, int64(len(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	if written != int64(len("hello rb\n")) {
		t.Fatalf("written = %d, want %d", written, len("hello rb\n"))
	}

	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello rb\n" {
		t.Fatalf("output = %q, want %q", got, "hello rb\n")
	}
}

func TestDecompressRejectsBadMagic(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "installer.exe")
	output := filepath.Join(dir, "Lemonade2.rb")

	if err := os.WriteFile(input, []byte("not bzip2"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := decompressBzip2Section(input, output, 0, 4); err == nil {
		t.Fatal("decompressBzip2Section() succeeded, want an error")
	}
}

func TestCompressRBFileAndRoundtripMD5(t *testing.T) {
	if _, err := exec.LookPath("bzip2"); err != nil {
		t.Skipf("bzip2 executable not available: %v", err)
	}

	dir := t.TempDir()
	rb := filepath.Join(dir, "Lemonade2.rb")
	compressed := filepath.Join(dir, "Lemonade2.rb.bz2")
	installer := filepath.Join(dir, "installer.exe")
	if err := os.WriteFile(rb, []byte("hello rb\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	written, sum, err := compressRBFile(rb, compressed)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(compressed)
	if err != nil {
		t.Fatal(err)
	}
	if written != int64(len(data)) {
		t.Fatalf("written = %d, want %d", written, len(data))
	}
	if want := md5.Sum(data); sum != want {
		t.Fatalf("md5 = %x, want %x", sum, want)
	}

	payload := append([]byte("JUNK"), data...)
	payload = append(payload, []byte("TRAILER")...)
	if err := os.WriteFile(installer, payload, 0o644); err != nil {
		t.Fatal(err)
	}

	match, originalMD5, recompressedMD5, _, err := roundtripMD5(installer, rb, 4, int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if !match || originalMD5 != recompressedMD5 {
		t.Fatalf("roundtrip mismatch: %v %x %x", match, originalMD5, recompressedMD5)
	}

	recompressed, err := os.ReadFile(rb + ".bz2")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(recompressed, data) {
		t.Fatal("recompressed bytes differ from original bzip2 stream")
	}
}
