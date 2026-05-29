package main

import (
	"bytes"
	"compress/zlib"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractEntry(t *testing.T) {
	t.Run("zlib", func(t *testing.T) {
		data := []byte("hello zlib\n")
		compressed := zlibFixture(t, data)
		testExtractEntry(t, installEntry{
			name: "hello.txt",
			size: int64(len(data)),
			md5:  md5String(data),
			mode: 0o644,
			parts: []archivePart{
				{algo: compressionZlib, offset: 4, length: int64(len(compressed))},
			},
		}, compressed, data)
	})

	t.Run("zlib plus raw overlay", func(t *testing.T) {
		data := []byte("hello zlib\n")
		overlay := []byte("raw overlay")
		compressed := zlibFixture(t, data)
		testExtractEntry(t, installEntry{
			name: "with-overlay.exe",
			size: int64(len(data) + len(overlay)),
			md5:  md5String(append(append([]byte{}, data...), overlay...)),
			mode: 0o755,
			parts: []archivePart{
				{algo: compressionZlib, offset: 4, length: int64(len(compressed))},
				{algo: compressionRaw, offset: int64(4 + len(compressed)), length: int64(len(overlay))},
			},
		}, append(compressed, overlay...), append(append([]byte{}, data...), overlay...))
	})

	t.Run("bzip2", func(t *testing.T) {
		data := []byte("hello rb\n")
		compressed, err := hex.DecodeString("425a68393141592653598979dccd000002518000104000124490002000310c08200f296268c3e2ee48a70a12112f3b99a0")
		if err != nil {
			t.Fatal(err)
		}
		testExtractEntry(t, installEntry{
			name: "hello.rb",
			size: int64(len(data)),
			md5:  md5String(data),
			mode: 0o644,
			parts: []archivePart{
				{algo: compressionBzip2, offset: 4, length: int64(len(compressed))},
			},
		}, compressed, data)
	})
}

func testExtractEntry(t *testing.T, entry installEntry, compressed []byte, want []byte) {
	t.Helper()

	dir := t.TempDir()
	installerPath := filepath.Join(dir, "installer.exe")
	installerBytes := append([]byte("JUNK"), compressed...)
	installerBytes = append(installerBytes, []byte("TRAILER")...)
	if err := os.WriteFile(installerPath, installerBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	installer, err := os.Open(installerPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = installer.Close() }()

	outputPath := filepath.Join(dir, entry.name)
	if err := extractEntry(installer, int64(len(installerBytes)), entry, outputPath, false); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func zlibFixture(t *testing.T, data []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	writer, err := zlib.NewWriterLevel(&buf, zlib.BestCompression)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func md5String(data []byte) string {
	sum := md5.Sum(data)
	return hex.EncodeToString(sum[:])
}

func TestBuildUninstallerFilePayload(t *testing.T) {
	got := buildUninstallerFilePayload()
	wantHex := "0800000001002e0800666d6f642e646c6c0001002e0d004c656d6f6e616465322e6578650001002e0c004c656d6f6e616465322e72620005002e5c4c73781400436865636b436f6e6e656374696f6e2e68746d6c0005002e5c4c737810004e6f436f6e6e656374696f6e2e6769660005002e5c4c737809005468756d62732e64620001002e0b006f7074696f6e732e6461740001002e130054656e656f6e494552656c656173652e646c6c00"
	want, err := hex.DecodeString(wantHex)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("file payload hex = %x, want %x", got, want)
	}
}

func TestBuildUninstallerIconPayload(t *testing.T) {
	got := buildUninstallerIconPayload(`C:\Users\Admin`)
	if len(got) != 366 {
		t.Fatalf("icon payload length = %d, want 366", len(got))
	}
	if binary.LittleEndian.Uint32(got) != 3 {
		t.Fatalf("icon count = %d, want 3", binary.LittleEndian.Uint32(got))
	}
	for _, want := range [][]byte{
		[]byte(`C:\Users\Admin\AppData\Roaming\Microsoft\Windows\Start Menu\Programs\Lemonade Tycoon 2 - New York City/`),
		[]byte(`Uninstall Lemonade Tycoon 2 - New York City.lnk`),
		[]byte(`C:\Users\Admin\AppData\Roaming\Microsoft\Windows\Start Menu\Programs\Lemonade Tycoon 2 - New York City%`),
		[]byte(`C:\Users\Admin\Desktop%`),
	} {
		if !bytes.Contains(got, want) {
			t.Fatalf("icon payload does not contain %q", want)
		}
	}
}

func TestFinalizeUninstaller(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Uninstal.exe")

	textBlocks := uninstallerTestBlock(uninstallTextBlockID, []byte("texts"))
	textBlocks = append(textBlocks, uninstallerTestBlock(0x7f7f, nil)...)
	registryBlocks := uninstallerTestBlock(0x123a, []byte{0x02})
	registryBlocks = append(registryBlocks, uninstallerTestBlock(0x1237, []byte("registry"))...)

	overlay := uninstallerTestChunk(uninstallerRegistryID, registryBlocks)
	overlay = append(overlay, uninstallerTestChunk(0x0001143a, []byte("ignored"))...)
	overlay = append(overlay, uninstallerTestChunk(uninstallerTextsID, textBlocks)...)
	input := append(make([]byte, uninstallerOverlayStart), overlay...)
	if err := os.WriteFile(path, input, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := finalizeUninstaller(path); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	expectedOverlay := uninstallerTestBlock(uninstallTextBlockID, []byte("texts"))
	var fileBlock bytes.Buffer
	writeUninstallerBlock(&fileBlock, uninstallFilesBlockID, buildUninstallerFilePayload())
	expectedOverlay = append(expectedOverlay, fileBlock.Bytes()...)
	var iconBlock bytes.Buffer
	writeUninstallerBlock(&iconBlock, uninstallIconsBlockID, buildUninstallerIconPayload(`C:\Users\Admin`))
	expectedOverlay = append(expectedOverlay, iconBlock.Bytes()...)
	expectedOverlay = append(expectedOverlay, registryBlocks...)
	expectedOverlay = append(expectedOverlay, finalizedUninstallerTerminator...)
	want := append(make([]byte, uninstallerOverlayStart), expectedOverlay...)

	if !bytes.Equal(got, want) {
		t.Fatalf("finalized uninstaller differs: got %x, want %x", got[uninstallerOverlayStart:], expectedOverlay)
	}
}

func uninstallerTestBlock(id uint32, payload []byte) []byte {
	var block bytes.Buffer
	writeUninstallerBlock(&block, id, payload)
	return block.Bytes()
}

func uninstallerTestChunk(id uint32, decoded []byte) []byte {
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	if _, err := writer.Write(decoded); err != nil {
		panic(err)
	}
	if err := writer.Close(); err != nil {
		panic(err)
	}

	record := make([]byte, 5+compressed.Len())
	binary.LittleEndian.PutUint32(record[0:4], uint32(len(decoded)))
	record[4] = 1
	copy(record[5:], compressed.Bytes())

	var chunk bytes.Buffer
	writeUint32(&chunk, id)
	writeUint32(&chunk, uint32(len(record)))
	chunk.Write(record)
	return chunk.Bytes()
}
