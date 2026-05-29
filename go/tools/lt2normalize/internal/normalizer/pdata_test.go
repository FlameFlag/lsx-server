package normalizer

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractPDATASectionWritesAssetsAndSkipsFalseMagic(t *testing.T) {
	bmp := append([]byte("BM"), bytes.Repeat([]byte{0x33}, 32)...)
	utf16Text := []byte{
		'L', 0, 'i', 0, 'c', 0, 'e', 0, 'n', 0, 's', 0, 'e', 0, ' ', 0,
		'e', 0, 'r', 0, 'r', 0, 'o', 0, 'r', 0, '\r', 0, '\n', 0,
	}

	var container bytes.Buffer
	container.Write(pdataSignature)
	container.Write(bytes.Repeat([]byte{0xA5}, pdataFirstMetadataSize))
	firstStreamOffset := container.Len()
	container.Write(zlibBytes(t, bmp, zlib.BestCompression))

	container.Write([]byte{0x42, 0x78, 0xDA, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09})
	secondStreamOffset := container.Len()
	container.Write(zlibBytes(t, utf16Text, zlib.DefaultCompression))
	container.Write(bytes.Repeat([]byte{0}, 16))

	outputDir := t.TempDir()
	assets, err := extractPDATASection(container.Bytes(), outputDir, 0x47000)
	if err != nil {
		t.Fatalf("extractPDATASection returned error: %v", err)
	}
	if len(assets) != 2 {
		t.Fatalf("expected 2 assets, got %d", len(assets))
	}

	if assets[0].SectionOffset != firstStreamOffset {
		t.Fatalf("first stream offset = 0x%X, want 0x%X", assets[0].SectionOffset, firstStreamOffset)
	}
	if assets[0].Kind != "bmp_image" || !strings.Contains(filepath.Base(assets[0].Path), "0x047013_bmp_image.bmp") {
		t.Fatalf("unexpected first asset path/kind: %q %q", assets[0].Kind, assets[0].Path)
	}
	gotBMP, err := os.ReadFile(assets[0].Path)
	if err != nil {
		t.Fatalf("read first asset: %v", err)
	}
	if !bytes.Equal(gotBMP, bmp) {
		t.Fatal("first asset bytes did not match decompressed BMP")
	}

	if assets[1].SectionOffset != secondStreamOffset {
		t.Fatalf("second stream offset = 0x%X, want 0x%X", assets[1].SectionOffset, secondStreamOffset)
	}
	if assets[1].Kind != "utf16le_text" || !strings.HasSuffix(filepath.Base(assets[1].Path), "_utf16le_text.txt") {
		t.Fatalf("unexpected second asset path/kind: %q %q", assets[1].Kind, assets[1].Path)
	}
	gotText, err := os.ReadFile(assets[1].Path)
	if err != nil {
		t.Fatalf("read second asset: %v", err)
	}
	if !bytes.Equal(gotText, utf16Text) {
		t.Fatal("second asset bytes did not match decompressed UTF-16 text")
	}
}

func TestExtractPDATASectionRejectsBadSignature(t *testing.T) {
	section := append([]byte("NOTPDAT"), bytes.Repeat([]byte{0}, pdataFirstMetadataSize+2)...)
	_, err := extractPDATASection(section, t.TempDir(), 0)
	if err == nil {
		t.Fatal("expected a signature error")
	}
}

func TestExtractPDATASectionPreservesNonZeroTail(t *testing.T) {
	payload := []byte("payload")
	tail := []byte{0xC8, 0x6B, 0x44, 0x10, 0x00}

	var container bytes.Buffer
	container.Write(pdataSignature)
	container.Write(bytes.Repeat([]byte{0xA5}, pdataFirstMetadataSize))
	container.Write(zlibBytes(t, payload, zlib.DefaultCompression))
	container.Write(tail)

	outputDir := t.TempDir()
	assets, err := extractPDATASection(container.Bytes(), outputDir, 0x47000)
	if err != nil {
		t.Fatalf("extractPDATASection returned error: %v", err)
	}
	if len(assets) != 2 {
		t.Fatalf("expected zlib asset plus tail, got %d assets", len(assets))
	}
	if assets[1].Kind != "encrypted_tail" {
		t.Fatalf("tail kind = %q, want encrypted_tail", assets[1].Kind)
	}
	gotTail, err := os.ReadFile(assets[1].Path)
	if err != nil {
		t.Fatalf("read tail asset: %v", err)
	}
	if !bytes.Equal(gotTail, tail) {
		t.Fatal("tail bytes did not match")
	}
}

func TestRecoverPDATAEncryptedTail(t *testing.T) {
	tail := bytes.Repeat([]byte{0xA5}, pdataRecoveredTailSize+16)
	copy(tail[:4], []byte{0, 0, 0, 0})

	recovered, err := recoverPDATAEncryptedTail(tail)
	if err != nil {
		t.Fatalf("recoverPDATAEncryptedTail returned error: %v", err)
	}
	if len(recovered) != pdataRecoveredTailSize {
		t.Fatalf("recovered size = 0x%X, want 0x%X", len(recovered), pdataRecoveredTailSize)
	}
	if got, want := recovered[:8], []byte{0, 0, 0, 0, 'P', 'E', 0, 0}; !bytes.Equal(got, want) {
		t.Fatalf("recovered prefix = % X, want % X", got, want)
	}
	sum := sha256.Sum256(recovered)
	if got, want := hex.EncodeToString(sum[:]), "d6c286ec456324bb2e0482284899a19240b61450620b2b06e829bc3aaad159a0"; got != want {
		t.Fatalf("recovered sha256 = %s, want %s", got, want)
	}

	recoveredTail, err := recoverPDATARuntimeTail(tail)
	if err != nil {
		t.Fatalf("recoverPDATARuntimeTail returned error: %v", err)
	}
	if len(recoveredTail) != len(tail) {
		t.Fatalf("recovered tail size = 0x%X, want 0x%X", len(recoveredTail), len(tail))
	}
	if !bytes.Equal(recoveredTail[:pdataRecoveredTailSize], recovered) {
		t.Fatal("runtime tail prefix does not contain recovered header")
	}
	if !bytes.Equal(recoveredTail[pdataRecoveredTailSize:], tail[pdataRecoveredTailSize:]) {
		t.Fatal("runtime tail suffix should be left unchanged")
	}
}

func TestRecoverPDATAEncryptedTailRejectsShortTail(t *testing.T) {
	_, err := recoverPDATAEncryptedTail(make([]byte, pdataRecoveredTailSize-1))
	if err == nil {
		t.Fatal("expected short tail error")
	}
}

func zlibBytes(t *testing.T, data []byte, level int) []byte {
	t.Helper()

	var buf bytes.Buffer
	writer, err := zlib.NewWriterLevel(&buf, level)
	if err != nil {
		t.Fatalf("NewWriterLevel: %v", err)
	}
	if _, err := writer.Write(data); err != nil {
		t.Fatalf("write zlib fixture: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zlib fixture: %v", err)
	}
	return buf.Bytes()
}
