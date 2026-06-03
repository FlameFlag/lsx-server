package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRunPacksAndUnpacksAssets(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "assets")
	if err := os.MkdirAll(filepath.Join(input, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(input, "root.txt"), []byte("root\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(input, "nested", "data.bin"), []byte{0, 1, 2}, 0o644); err != nil {
		t.Fatal(err)
	}

	archive := filepath.Join(dir, "assets.rb")
	if err := run([]string{"-quiet", "pack", input, archive}, new(bytes.Buffer), new(bytes.Buffer)); err != nil {
		t.Fatal(err)
	}

	output := filepath.Join(dir, "out")
	if err := run([]string{"-quiet", "unpack", archive, output}, new(bytes.Buffer), new(bytes.Buffer)); err != nil {
		t.Fatal(err)
	}
	assertFileBytes(t, filepath.Join(output, "root.txt"), []byte("root\n"))
	assertFileBytes(t, filepath.Join(output, "nested", "data.bin"), []byte{0, 1, 2})
}

func TestRunUnpacksLemonadeRBBitmaps(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "Lemonade2.rb")
	rb := testRB(testBitmap{width: 1, height: 1, format: formatRGB565Mask, data: []byte{0x00, 0xF8, 0xFF}})
	if err := os.WriteFile(input, rb, 0o644); err != nil {
		t.Fatal(err)
	}

	output := filepath.Join(dir, "assets")
	if err := run([]string{"-quiet", "unpack", input, output}, new(bytes.Buffer), new(bytes.Buffer)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(output, "bitmaps", "bitmap_000_1x1_rgb565a8.png")); err != nil {
		t.Fatalf("bitmap was not unpacked: %v", err)
	}
}

func TestRunPacksSingleAsset(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "single.txt")
	if err := os.WriteFile(input, []byte("single\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	archive := filepath.Join(dir, "single.rb")
	if err := run([]string{"-quiet", "compress", input, archive}, new(bytes.Buffer), new(bytes.Buffer)); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "out")
	if err := run([]string{"-quiet", "decompress", archive, output}, new(bytes.Buffer), new(bytes.Buffer)); err != nil {
		t.Fatal(err)
	}
	assertFileBytes(t, filepath.Join(output, "single.txt"), []byte("single\n"))
}

func TestRunRejectsObsoleteInstallerFlags(t *testing.T) {
	err := run([]string{"-offset", "4", "installer.exe", "out.rb"}, new(bytes.Buffer), new(bytes.Buffer))
	if err == nil {
		t.Fatal("run accepted obsolete installer mode")
	}
}
