package normalizer

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"errors"
	"fmt"
)

const generatedPreferredBase = uint32(0x10000000)

type generatedImageInfo struct {
	OutputPath    string
	LoadBase      uint32
	PreferredBase uint32
	Size          int
	Relocations   int
	Mutations     generatedMutationInfo
}

func deriveGeneratedImage(packedPath, outputPath string, loadBase uint32, replayTEA bool) (generatedImageInfo, error) {
	sectionData, _, err := readPDATASection(packedPath)
	if err != nil {
		return generatedImageInfo{}, err
	}
	dllData, err := extractGeneratedDLLFromPDATA(sectionData)
	if err != nil {
		return generatedImageInfo{}, err
	}
	image, preferredBase, relocationCount, err := mapPEImage(dllData, loadBase)
	if err != nil {
		return generatedImageInfo{}, err
	}
	mutations := normalizeGeneratedInitialImage(image, loadBase)
	if replayTEA {
		mutations.ReplayedTEARows, err = replayGeneratedTEARows(image, loadBase)
		if err != nil {
			return generatedImageInfo{}, err
		}
	}
	if err := writeFileAtomic(outputPath, image); err != nil {
		return generatedImageInfo{}, err
	}
	return generatedImageInfo{
		OutputPath:    outputPath,
		LoadBase:      loadBase,
		PreferredBase: preferredBase,
		Size:          len(image),
		Relocations:   relocationCount,
		Mutations:     mutations,
	}, nil
}

func extractGeneratedDLLFromPDATA(sectionData []byte) ([]byte, error) {
	firstStreamOffset := len(pdataSignature) + pdataFirstMetadataSize
	if len(sectionData) < firstStreamOffset+2 || !bytes.HasPrefix(sectionData, pdataSignature) {
		return nil, errors.New(".pdata signature mismatch")
	}
	for searchOffset := firstStreamOffset; searchOffset < len(sectionData)-1; {
		streamOffset := searchOffset
		if streamOffset != firstStreamOffset {
			streamOffset = findNextZlibMagic(sectionData, searchOffset)
			if streamOffset < 0 {
				break
			}
		}
		decompressed, compressedSize, err := inflatePDATAStream(sectionData, streamOffset)
		if err != nil {
			searchOffset = streamOffset + 1
			continue
		}
		if isGeneratedPE(decompressed) {
			return decompressed, nil
		}
		searchOffset = streamOffset + compressedSize
	}
	return nil, errors.New("generated PE asset not found in .pdata")
}

func isGeneratedPE(data []byte) bool {
	if len(data) < 0x40 || !bytes.Equal(data[:2], []byte("MZ")) {
		return false
	}
	peOffset := int(binary.LittleEndian.Uint32(data[0x3C:0x40]))
	if peOffset < 0 || peOffset+0x18+0x60 > len(data) || !bytes.Equal(data[peOffset:peOffset+4], []byte("PE\x00\x00")) {
		return false
	}
	optional := peOffset + 0x18
	magic := binary.LittleEndian.Uint16(data[optional : optional+2])
	if magic != 0x10B {
		return false
	}
	return binary.LittleEndian.Uint32(data[optional+0x1C:optional+0x20]) == generatedPreferredBase
}

func mapPEImage(data []byte, loadBase uint32) ([]byte, uint32, int, error) {
	reader := bytes.NewReader(data)
	file, err := pe.NewFile(reader)
	if err != nil {
		return nil, 0, 0, err
	}
	optional, ok := file.OptionalHeader.(*pe.OptionalHeader32)
	if !ok {
		return nil, 0, 0, errors.New("generated PE is not PE32")
	}
	if optional.SizeOfImage == 0 {
		return nil, 0, 0, errors.New("generated PE has zero SizeOfImage")
	}
	image := make([]byte, optional.SizeOfImage)
	headerSize := int(optional.SizeOfHeaders)
	if headerSize > len(data) {
		return nil, 0, 0, errors.New("generated PE headers exceed file size")
	}
	copy(image, data[:headerSize])
	for _, section := range file.Sections {
		start := int(section.VirtualAddress)
		size := int(section.Size)
		if size == 0 {
			continue
		}
		if start < 0 || start+size > len(image) {
			return nil, 0, 0, fmt.Errorf("section %s raw map exceeds image size", section.Name)
		}
		sectionData, err := section.Data()
		if err != nil {
			return nil, 0, 0, fmt.Errorf("read section %s: %w", section.Name, err)
		}
		copy(image[start:start+size], sectionData)
	}
	relocationCount, err := applyBaseRelocations(image, optional.ImageBase, loadBase, optional.DataDirectory[5].VirtualAddress, optional.DataDirectory[5].Size)
	if err != nil {
		return nil, 0, 0, err
	}
	return image, optional.ImageBase, relocationCount, nil
}

func applyBaseRelocations(image []byte, preferredBase uint32, loadBase uint32, relocRVA uint32, relocSize uint32) (int, error) {
	if relocRVA == 0 || relocSize == 0 || preferredBase == loadBase {
		return 0, nil
	}
	pos := int(relocRVA)
	end := int(relocRVA + relocSize)
	if pos < 0 || end < pos || end > len(image) {
		return 0, fmt.Errorf("relocation table 0x%X..0x%X exceeds image size 0x%X", relocRVA, relocRVA+relocSize, len(image))
	}
	delta := loadBase - preferredBase
	count := 0
	for pos+8 <= end {
		page := binary.LittleEndian.Uint32(image[pos : pos+4])
		blockSize := int(binary.LittleEndian.Uint32(image[pos+4 : pos+8]))
		pos += 8
		if blockSize < 8 || pos+blockSize-8 > end {
			return 0, fmt.Errorf("invalid relocation block size 0x%X", blockSize)
		}
		entryCount := (blockSize - 8) / 2
		for range entryCount {
			entry := binary.LittleEndian.Uint16(image[pos : pos+2])
			pos += 2
			typ := entry >> 12
			offset := uint32(entry & 0x0FFF)
			if typ != 3 {
				continue
			}
			rva := int(page + offset)
			if rva < 0 || rva+4 > len(image) {
				return 0, fmt.Errorf("relocation rva 0x%X exceeds image size 0x%X", rva, len(image))
			}
			value := binary.LittleEndian.Uint32(image[rva : rva+4])
			binary.LittleEndian.PutUint32(image[rva:rva+4], value+delta)
			count++
		}
		if blockSize%2 != 0 {
			pos++
		}
	}
	return count, nil
}
