package normalizer

import (
	"bytes"
	"compress/zlib"
	"debug/pe"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	pdataFirstMetadataSize = 11
	pdataDLLCharacteristic = 0x2000
	maxPDATAAssetSize      = 256 << 20
)

var pdataSignature = []byte("PDATA000")

type pdataAsset struct {
	Index            int
	FileOffset       uint64
	SectionOffset    int
	CompressedSize   int
	DecompressedSize int
	Kind             string
	Path             string
}

func extractPDATA(path, outputDir string) ([]pdataAsset, error) {
	sectionData, rawOffset, err := readPDATASection(path)
	if err != nil {
		return nil, err
	}
	return extractPDATASection(sectionData, outputDir, uint64(rawOffset))
}

func readPDATASection(path string) ([]byte, uint32, error) {
	file, err := pe.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = file.Close() }()

	section := peSection(file, ".pdata")
	if section != nil {
		data, err := section.Data()
		if err != nil {
			return nil, 0, fmt.Errorf("read .pdata section: %w", err)
		}
		return data, section.Offset, nil
	}
	return nil, 0, errors.New(".pdata section not found")
}

func extractPDATASection(sectionData []byte, outputDir string, sectionRawOffset uint64) ([]pdataAsset, error) {
	firstStreamOffset := len(pdataSignature) + pdataFirstMetadataSize
	if len(sectionData) < firstStreamOffset+2 {
		return nil, fmt.Errorf(".pdata section too small: %d bytes", len(sectionData))
	}
	if !bytes.HasPrefix(sectionData, pdataSignature) {
		return nil, fmt.Errorf(".pdata signature mismatch: got % X, expected % X",
			sectionData[:len(pdataSignature)], pdataSignature)
	}
	if !isZlibMagicAt(sectionData, firstStreamOffset) {
		return nil, fmt.Errorf("first .pdata stream at section offset 0x%X has magic % X, expected 78 DA or 78 9C",
			firstStreamOffset, sectionData[firstStreamOffset:firstStreamOffset+2])
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create .pdata output directory: %w", err)
	}

	var assets []pdataAsset
	searchOffset := firstStreamOffset
	lastStreamEnd := firstStreamOffset
	for searchOffset < len(sectionData)-1 {
		streamOffset := searchOffset
		if len(assets) > 0 {
			streamOffset = findNextZlibMagic(sectionData, searchOffset)
			if streamOffset < 0 {
				break
			}
		}

		decompressed, compressedSize, err := inflatePDATAStream(sectionData, streamOffset)
		if err != nil {
			if len(assets) == 0 {
				return nil, fmt.Errorf("inflate first .pdata stream at section offset 0x%X: %w", streamOffset, err)
			}
			searchOffset = streamOffset + 1
			continue
		}

		index := len(assets) + 1
		kind, extension := describePDATAAsset(decompressed)
		fileOffset := sectionRawOffset + uint64(streamOffset)
		outputPath := filepath.Join(outputDir, fmt.Sprintf("pdata_%03d_0x%06X_%s%s",
			index, fileOffset, kind, extension))
		if err := writePDATAAsset(outputPath, decompressed); err != nil {
			return nil, err
		}

		assets = append(assets, pdataAsset{
			Index:            index,
			FileOffset:       fileOffset,
			SectionOffset:    streamOffset,
			CompressedSize:   compressedSize,
			DecompressedSize: len(decompressed),
			Kind:             kind,
			Path:             outputPath,
		})
		lastStreamEnd = streamOffset + compressedSize
		searchOffset = lastStreamEnd
	}

	if len(assets) == 0 {
		return nil, errors.New(".pdata did not contain any valid zlib streams")
	}
	tail := sectionData[lastStreamEnd:]
	if hasNonZeroByte(tail) {
		index := len(assets) + 1
		fileOffset := sectionRawOffset + uint64(lastStreamEnd)
		outputPath := filepath.Join(outputDir, fmt.Sprintf("pdata_%03d_0x%06X_encrypted_tail.bin",
			index, fileOffset))
		if err := writePDATAAsset(outputPath, tail); err != nil {
			return nil, err
		}
		assets = append(assets, pdataAsset{
			Index:            index,
			FileOffset:       fileOffset,
			SectionOffset:    lastStreamEnd,
			CompressedSize:   len(tail),
			DecompressedSize: len(tail),
			Kind:             "encrypted_tail",
			Path:             outputPath,
		})

		if len(tail) >= pdataRecoveredTailSize {
			recovered, err := recoverPDATAEncryptedTail(tail)
			if err != nil {
				return nil, err
			}
			index = len(assets) + 1
			outputPath = filepath.Join(outputDir, fmt.Sprintf("pdata_%03d_0x%06X_recovered_tail_header.bin",
				index, fileOffset))
			if err := writePDATAAsset(outputPath, recovered); err != nil {
				return nil, err
			}
			assets = append(assets, pdataAsset{
				Index:            index,
				FileOffset:       fileOffset,
				SectionOffset:    lastStreamEnd,
				CompressedSize:   len(tail),
				DecompressedSize: len(recovered),
				Kind:             "recovered_tail_header",
				Path:             outputPath,
			})

			recoveredTail, err := recoverPDATARuntimeTail(tail)
			if err != nil {
				return nil, err
			}
			index = len(assets) + 1
			outputPath = filepath.Join(outputDir, fmt.Sprintf("pdata_%03d_0x%06X_recovered_tail.bin",
				index, fileOffset))
			if err := writePDATAAsset(outputPath, recoveredTail); err != nil {
				return nil, err
			}
			assets = append(assets, pdataAsset{
				Index:            index,
				FileOffset:       fileOffset,
				SectionOffset:    lastStreamEnd,
				CompressedSize:   len(tail),
				DecompressedSize: len(recoveredTail),
				Kind:             "recovered_tail",
				Path:             outputPath,
			})
		}
	}
	return assets, nil
}

func inflatePDATAStream(sectionData []byte, streamOffset int) ([]byte, int, error) {
	reader := bytes.NewReader(sectionData[streamOffset:])
	zr, err := zlib.NewReader(reader)
	if err != nil {
		return nil, 0, err
	}

	decompressed, readErr := io.ReadAll(io.LimitReader(zr, maxPDATAAssetSize+1))
	closeErr := zr.Close()
	if readErr != nil {
		return nil, 0, readErr
	}
	if closeErr != nil {
		return nil, 0, closeErr
	}
	if len(decompressed) > maxPDATAAssetSize {
		return nil, 0, fmt.Errorf("decompressed stream exceeds %d bytes", maxPDATAAssetSize)
	}

	compressedSize := len(sectionData[streamOffset:]) - reader.Len()
	if compressedSize <= 0 {
		return nil, 0, errors.New("zlib stream consumed no input")
	}
	return decompressed, compressedSize, nil
}

func findNextZlibMagic(data []byte, start int) int {
	if start < 0 {
		start = 0
	}
	for offset := start; offset < len(data)-1; offset++ {
		if isZlibMagicAt(data, offset) {
			return offset
		}
	}
	return -1
}

func isZlibMagicAt(data []byte, offset int) bool {
	if offset < 0 || offset+1 >= len(data) {
		return false
	}
	return data[offset] == 0x78 && (data[offset+1] == 0xDA || data[offset+1] == 0x9C)
}

func hasNonZeroByte(data []byte) bool {
	for _, b := range data {
		if b != 0 {
			return true
		}
	}
	return false
}

func describePDATAAsset(data []byte) (string, string) {
	if len(data) >= 2 && data[0] == 'B' && data[1] == 'M' {
		return "bmp_image", ".bmp"
	}
	if kind, extension, ok := describePEAsset(data); ok {
		return kind, extension
	}
	if looksUTF16LEText(data) {
		return "utf16le_text", ".txt"
	}
	if looksASCIIText(data) {
		return "text", ".txt"
	}
	return "blob", ".bin"
}

func describePEAsset(data []byte) (string, string, bool) {
	if len(data) < 2 || data[0] != 'M' || data[1] != 'Z' {
		return "", "", false
	}

	file, err := pe.NewFile(bytes.NewReader(data))
	if err != nil {
		return "", "", false
	}
	defer func() { _ = file.Close() }()

	bits := "pe"
	switch file.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		bits = "pe32"
	case *pe.OptionalHeader64:
		bits = "pe64"
	}
	if file.Characteristics&pdataDLLCharacteristic != 0 {
		return bits + "_dll", ".dll", true
	}
	return bits + "_image", ".exe", true
}

func looksUTF16LEText(data []byte) bool {
	sample := min(len(data), 8192)
	sample -= sample % 2
	if sample < 16 {
		return false
	}

	var zeroHighBytes, printable int
	pairs := sample / 2
	for offset := 0; offset < sample; offset += 2 {
		lo := data[offset]
		hi := data[offset+1]
		if hi == 0 {
			zeroHighBytes++
			if lo == '\t' || lo == '\n' || lo == '\r' || (lo >= 0x20 && lo < 0x7F) {
				printable++
			}
		}
	}
	return zeroHighBytes*100/pairs >= 70 && printable*100/pairs >= 55
}

func looksASCIIText(data []byte) bool {
	sample := min(len(data), 8192)
	if sample < 8 {
		return false
	}

	var printable int
	for _, b := range data[:sample] {
		if b == '\t' || b == '\n' || b == '\r' || (b >= 0x20 && b < 0x7F) {
			printable++
		}
	}
	return printable*100/sample >= 90
}

func writePDATAAsset(path string, data []byte) error {
	outputDir := filepath.Dir(path)
	temp, err := os.CreateTemp(outputDir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp .pdata asset: %w", err)
	}
	tempPath := temp.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tempPath)
		}
	}()

	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write .pdata asset: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close .pdata asset: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace .pdata asset: %w", err)
	}
	removeTemp = false
	return nil
}
