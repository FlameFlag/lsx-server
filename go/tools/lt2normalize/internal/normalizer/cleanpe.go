package normalizer

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
)

const (
	cleanTextRVA         = uint32(0x1000)
	cleanDefaultEntryRVA = uint32(0x886E3)
	cleanRDataRVA        = uint32(0x92000)
	cleanDataRVA         = uint32(0x9C000)
	cleanRsrcName        = ".rsrc"
	cleanIDATAName       = ".idata"
	cleanIATRVA          = uint32(0x92000)
	cleanIATSize         = uint32(0x2B8)
	cleanRsrcRVA         = uint32(0xA3000)
)

type cleanPEInfo struct {
	OutputPath string
	EntryRVA   uint32
	Sections   []payloadSectionWrite
	Warnings   []string
}

type cleanSectionBuild struct {
	Name        string
	RVA         uint32
	VirtualSize uint32
	RawSize     uint32
	RawData     []byte
	Flags       uint32
}

type cleanImportDLL struct {
	Name          string
	FirstThunkRVA uint32
	Entries       []cleanImportEntry
}

type cleanImportEntry struct {
	Name    string
	Ordinal uint16
}

type cleanImportBuild struct {
	Data []byte
	RVA  uint32
	Size uint32
}

func rebuildCleanPE(packedPath, payloadPath, outputPath string, entryRVA uint32) (cleanPEInfo, error) {
	packed, err := os.ReadFile(packedPath)
	if err != nil {
		return cleanPEInfo{}, err
	}
	payload, err := os.ReadFile(payloadPath)
	if err != nil {
		return cleanPEInfo{}, err
	}
	if len(payload) != payloadCombinedSize {
		return cleanPEInfo{}, fmt.Errorf("payload has size 0x%X, expected 0x%X", len(payload), payloadCombinedSize)
	}
	payload = bytes.Clone(payload)
	if err := applyCleanRuntimePatches(payload); err != nil {
		return cleanPEInfo{}, err
	}

	reader := bytes.NewReader(packed)
	file, err := pe.NewFile(reader)
	if err != nil {
		return cleanPEInfo{}, err
	}
	optional, ok := file.OptionalHeader.(*pe.OptionalHeader32)
	if !ok {
		return cleanPEInfo{}, errors.New("packed EXE is not PE32")
	}
	if optional.SizeOfHeaders == 0 || int(optional.SizeOfHeaders) > len(packed) {
		return cleanPEInfo{}, fmt.Errorf("invalid SizeOfHeaders 0x%X", optional.SizeOfHeaders)
	}
	rsrcSection := findPESection(file, cleanRsrcName)
	if rsrcSection == nil {
		return cleanPEInfo{}, fmt.Errorf("%s section not found", cleanRsrcName)
	}
	rsrcData, err := rsrcSection.Data()
	if err != nil {
		return cleanPEInfo{}, fmt.Errorf("read %s section: %w", cleanRsrcName, err)
	}
	rsrcData = bytes.Clone(rsrcData)
	if err := relocateResourceDataEntries(rsrcData, rsrcSection.VirtualAddress, cleanRsrcRVA, rsrcSection.VirtualSize); err != nil {
		return cleanPEInfo{}, fmt.Errorf("relocate %s directory: %w", cleanRsrcName, err)
	}

	idataRVA := alignUp32(cleanRsrcRVA+rsrcSection.VirtualSize, optional.SectionAlignment)
	idata := buildCleanImportSection(idataRVA)

	sections := []cleanSectionBuild{
		{Name: ".text", RVA: cleanTextRVA, VirtualSize: 0x902D6, RawSize: payloadTextSize, RawData: payload[:payloadTextSize], Flags: 0x60000020},
		{Name: ".rdata", RVA: cleanRDataRVA, VirtualSize: 0x9AEE, RawSize: payloadRDataSize, RawData: payload[payloadTextSize : payloadTextSize+payloadRDataSize], Flags: 0x40000040},
		{Name: ".data", RVA: cleanDataRVA, VirtualSize: 0x6578, RawSize: payloadDataSize, RawData: payload[payloadTextSize+payloadRDataSize:], Flags: 0xC0000040},
		{Name: cleanRsrcName, RVA: cleanRsrcRVA, VirtualSize: rsrcSection.VirtualSize, RawSize: rsrcSection.Size, RawData: rsrcData, Flags: rsrcSection.Characteristics},
		{Name: cleanIDATAName, RVA: idata.RVA, VirtualSize: idata.Size, RawSize: alignUp32(idata.Size, optional.FileAlignment), RawData: idata.Data, Flags: 0x40000040},
	}

	peOffset, sectionTable, optionalOffset, err := peLayoutOffsets(packed)
	if err != nil {
		return cleanPEInfo{}, err
	}
	if sectionTable+len(sections)*40 > int(optional.SizeOfHeaders) {
		return cleanPEInfo{}, errors.New("clean section table exceeds header size")
	}

	out := make([]byte, optional.SizeOfHeaders)
	copy(out, packed[:optional.SizeOfHeaders])
	binary.LittleEndian.PutUint16(out[peOffset+6:peOffset+8], uint16(len(sections)))
	patchCleanOptionalHeader(out, optionalOffset, entryRVA, sections, optional.FileAlignment, optional.SectionAlignment, idata)

	rawOffset := alignUp32(optional.SizeOfHeaders, optional.FileAlignment)
	sectionWrites := make([]payloadSectionWrite, 0, len(sections))
	for index, section := range sections {
		headerOffset := sectionTable + index*40
		writeSectionHeader(out[headerOffset:headerOffset+40], section, rawOffset)
		out = padTo(out, int(rawOffset))
		out = append(out, section.RawData...)
		if uint32(len(section.RawData)) < section.RawSize {
			out = append(out, make([]byte, int(section.RawSize)-len(section.RawData))...)
		}
		sectionWrites = append(sectionWrites, payloadSectionWrite{Name: section.Name, RawOffset: rawOffset, RawSize: section.RawSize})
		rawOffset = alignUp32(rawOffset+section.RawSize, optional.FileAlignment)
	}
	for index := len(sections); index < int(binary.LittleEndian.Uint16(packed[peOffset+6:peOffset+8])); index++ {
		headerOffset := sectionTable + index*40
		clear(out[headerOffset : headerOffset+40])
	}

	if err := writeFileAtomic(outputPath, out); err != nil {
		return cleanPEInfo{}, err
	}
	return cleanPEInfo{
		OutputPath: outputPath,
		EntryRVA:   entryRVA,
		Sections:   sectionWrites,
		Warnings: []string{
			"clean PE strips Armadillo carrier sections; OEP is set to recovered CRT/game startup at RVA 0x886E3",
			"standard import directory is rebuilt from recovered IAT order and resolved live/export evidence",
		},
	}, nil
}

func findPESection(file *pe.File, name string) *pe.Section {
	return peSection(file, name)
}

func applyCleanRuntimePatches(payload []byte) error {
	if len(payload) < payloadTextSize {
		return errors.New("payload text is truncated")
	}
	// The recovered game has an unsynchronized lazy UI-resource initializer that
	// publishes its initialized flag before filling the global objects. On modern
	// Windows, FMOD/DirectSound worker threads can observe the early flag and use
	// a zero child pointer. Delay publishing the flag until the initializer epilogue.
	if err := patchTextVA(payload, 0x0045749C, []byte{0x88, 0x1D, 0x18, 0x07, 0x4A, 0x00}, []byte{0x90, 0x90, 0x90, 0x90, 0x90, 0x90}); err != nil {
		return err
	}
	if err := patchPayloadVA(payload, 0x004A0718, []byte{0x01}, []byte{0x00}); err != nil {
		return err
	}
	if err := patchTextVA(payload, 0x00457C6B, []byte{0x5F, 0x5E, 0x5D, 0x5B, 0x83, 0xC4, 0x2C, 0xC3}, []byte{0xE9, 0, 0, 0, 0, 0x90, 0x90, 0x90}); err != nil {
		return err
	}
	jumpOffset := textOffsetFromVA(0x00457C6B)
	jumpTarget := uint32(0x0042E970)
	jumpSourceEnd := uint32(0x00457C6B + 5)
	binary.LittleEndian.PutUint32(payload[jumpOffset+1:jumpOffset+5], jumpTarget-jumpSourceEnd)
	cavePatch := []byte{0xC6, 0x05, 0x18, 0x07, 0x4A, 0x00, 0x01, 0x5F, 0x5E, 0x5D, 0x5B, 0x83, 0xC4, 0x2C, 0xC3}
	return patchTextVA(payload, 0x0042E970, bytes.Repeat([]byte{0x90}, len(cavePatch)), cavePatch)
}

func patchTextVA(payload []byte, va uint32, want, replacement []byte) error {
	return patchPayloadOffset(payload, textOffsetFromVA(va), va, want, replacement)
}

func patchPayloadVA(payload []byte, va uint32, want, replacement []byte) error {
	rva := uint64(va) - originalImageBase
	var offset uint64
	switch {
	case rva >= uint64(cleanTextRVA) && rva < uint64(cleanTextRVA+payloadTextSize):
		offset = rva - uint64(cleanTextRVA)
	case rva >= uint64(cleanRDataRVA) && rva < uint64(cleanRDataRVA+payloadRDataSize):
		offset = uint64(payloadTextSize) + rva - uint64(cleanRDataRVA)
	case rva >= uint64(cleanDataRVA) && rva < uint64(cleanDataRVA+payloadDataSize):
		offset = uint64(payloadTextSize+payloadRDataSize) + rva - uint64(cleanDataRVA)
	default:
		return fmt.Errorf("patch at VA 0x%X is not in payload sections", va)
	}
	return patchPayloadOffset(payload, int(offset), va, want, replacement)
}

func patchPayloadOffset(payload []byte, offset int, va uint32, want, replacement []byte) error {
	if len(want) != len(replacement) {
		return fmt.Errorf("patch at VA 0x%X has mismatched lengths", va)
	}
	if offset > len(payload) || len(payload)-offset < len(want) {
		return fmt.Errorf("patch at VA 0x%X is out of range", va)
	}
	if !bytes.Equal(payload[offset:offset+len(want)], want) {
		return fmt.Errorf("patch at VA 0x%X found % X, want % X", va, payload[offset:offset+len(want)], want)
	}
	copy(payload[offset:offset+len(replacement)], replacement)
	return nil
}

func textOffsetFromVA(va uint32) int {
	return int(uint64(va) - originalImageBase - uint64(cleanTextRVA))
}

func relocateResourceDataEntries(data []byte, oldRVA, newRVA, virtualSize uint32) error {
	visited := map[uint32]bool{}
	var walk func(uint32) error
	walk = func(dirOffset uint32) error {
		if visited[dirOffset] {
			return nil
		}
		visited[dirOffset] = true
		if dirOffset > uint32(len(data)) || uint32(len(data))-dirOffset < 16 {
			return fmt.Errorf("resource directory offset 0x%X is out of range", dirOffset)
		}
		entryCount := int(binary.LittleEndian.Uint16(data[dirOffset+12:dirOffset+14])) + int(binary.LittleEndian.Uint16(data[dirOffset+14:dirOffset+16]))
		entriesOffset := dirOffset + 16
		if entriesOffset > uint32(len(data)) || uint32(len(data))-entriesOffset < uint32(entryCount*8) {
			return fmt.Errorf("resource directory entries at 0x%X exceed section", entriesOffset)
		}
		for index := range entryCount {
			entryOffset := entriesOffset + uint32(index*8)
			value := binary.LittleEndian.Uint32(data[entryOffset+4 : entryOffset+8])
			childOffset := value & 0x7FFFFFFF
			if value&0x80000000 != 0 {
				if err := walk(childOffset); err != nil {
					return err
				}
				continue
			}
			if childOffset > uint32(len(data)) || uint32(len(data))-childOffset < 16 {
				return fmt.Errorf("resource data entry offset 0x%X is out of range", childOffset)
			}
			dataRVA := binary.LittleEndian.Uint32(data[childOffset : childOffset+4])
			if dataRVA >= oldRVA && dataRVA < oldRVA+virtualSize {
				binary.LittleEndian.PutUint32(data[childOffset:childOffset+4], newRVA+(dataRVA-oldRVA))
			}
		}
		return nil
	}
	return walk(0)
}

func peLayoutOffsets(data []byte) (int, int, int, error) {
	if len(data) < 0x40 || !bytes.Equal(data[:2], []byte("MZ")) {
		return 0, 0, 0, errors.New("not a PE/MZ file")
	}
	peOffset := int(binary.LittleEndian.Uint32(data[0x3C:0x40]))
	if peOffset < 0 || peOffset+24 > len(data) || !bytes.Equal(data[peOffset:peOffset+4], []byte("PE\x00\x00")) {
		return 0, 0, 0, errors.New("invalid PE header")
	}
	optionalSize := int(binary.LittleEndian.Uint16(data[peOffset+20 : peOffset+22]))
	optionalOffset := peOffset + 24
	sectionTable := optionalOffset + optionalSize
	return peOffset, sectionTable, optionalOffset, nil
}

func patchCleanOptionalHeader(out []byte, optionalOffset int, entryRVA uint32, sections []cleanSectionBuild, fileAlignment uint32, sectionAlignment uint32, idata cleanImportBuild) {
	binary.LittleEndian.PutUint32(out[optionalOffset+4:optionalOffset+8], payloadTextSize)
	binary.LittleEndian.PutUint32(out[optionalOffset+8:optionalOffset+12], payloadRDataSize+payloadDataSize+sections[3].RawSize+sections[4].RawSize)
	binary.LittleEndian.PutUint32(out[optionalOffset+12:optionalOffset+16], 0)
	binary.LittleEndian.PutUint32(out[optionalOffset+16:optionalOffset+20], entryRVA)
	binary.LittleEndian.PutUint32(out[optionalOffset+20:optionalOffset+24], cleanTextRVA)
	binary.LittleEndian.PutUint32(out[optionalOffset+24:optionalOffset+28], cleanRDataRVA)
	last := sections[len(sections)-1]
	rsrc := sections[3]
	binary.LittleEndian.PutUint32(out[optionalOffset+56:optionalOffset+60], alignUp32(last.RVA+last.VirtualSize, sectionAlignment))
	binary.LittleEndian.PutUint32(out[optionalOffset+64:optionalOffset+68], 0)
	dataDir := optionalOffset + 96
	for index := range 16 {
		binary.LittleEndian.PutUint32(out[dataDir+index*8:dataDir+index*8+4], 0)
		binary.LittleEndian.PutUint32(out[dataDir+index*8+4:dataDir+index*8+8], 0)
	}
	binary.LittleEndian.PutUint32(out[dataDir+2*8:dataDir+2*8+4], rsrc.RVA)
	binary.LittleEndian.PutUint32(out[dataDir+2*8+4:dataDir+2*8+8], rsrc.VirtualSize)
	binary.LittleEndian.PutUint32(out[dataDir+1*8:dataDir+1*8+4], idata.RVA)
	binary.LittleEndian.PutUint32(out[dataDir+1*8+4:dataDir+1*8+8], uint32((len(cleanImportManifest)+1)*20))
	binary.LittleEndian.PutUint32(out[dataDir+12*8:dataDir+12*8+4], cleanIATRVA)
	binary.LittleEndian.PutUint32(out[dataDir+12*8+4:dataDir+12*8+8], cleanIATSize)
	_ = fileAlignment
}

func buildCleanImportSection(rva uint32) cleanImportBuild {
	descriptorSize := (len(cleanImportManifest) + 1) * 20
	data := make([]byte, descriptorSize)
	lookupRVAs := make([]uint32, len(cleanImportManifest))
	nameRVAs := make([]uint32, len(cleanImportManifest))

	for dllIndex, dll := range cleanImportManifest {
		data = alignBytes(data, 4)
		lookupRVAs[dllIndex] = rva + uint32(len(data))
		data = append(data, make([]byte, (len(dll.Entries)+1)*4)...)
	}

	entryNameRVA := make([][]uint32, len(cleanImportManifest))
	for dllIndex, dll := range cleanImportManifest {
		entryNameRVA[dllIndex] = make([]uint32, len(dll.Entries))
		for entryIndex, entry := range dll.Entries {
			if entry.Ordinal != 0 {
				continue
			}
			data = alignBytes(data, 2)
			entryNameRVA[dllIndex][entryIndex] = rva + uint32(len(data))
			data = append(data, 0, 0)
			data = append(data, entry.Name...)
			data = append(data, 0)
		}
	}

	for dllIndex, dll := range cleanImportManifest {
		nameRVAs[dllIndex] = rva + uint32(len(data))
		data = append(data, dll.Name...)
		data = append(data, 0)
	}

	for dllIndex, dll := range cleanImportManifest {
		descriptorOffset := dllIndex * 20
		binary.LittleEndian.PutUint32(data[descriptorOffset:descriptorOffset+4], lookupRVAs[dllIndex])
		binary.LittleEndian.PutUint32(data[descriptorOffset+12:descriptorOffset+16], nameRVAs[dllIndex])
		binary.LittleEndian.PutUint32(data[descriptorOffset+16:descriptorOffset+20], dll.FirstThunkRVA)

		lookupOffset := int(lookupRVAs[dllIndex] - rva)
		for entryIndex, entry := range dll.Entries {
			value := entryNameRVA[dllIndex][entryIndex]
			if entry.Ordinal != 0 {
				value = 0x80000000 | uint32(entry.Ordinal)
			}
			binary.LittleEndian.PutUint32(data[lookupOffset+entryIndex*4:lookupOffset+entryIndex*4+4], value)
		}
	}

	return cleanImportBuild{Data: data, RVA: rva, Size: uint32(len(data))}
}

func alignBytes(data []byte, alignment int) []byte {
	if alignment <= 1 {
		return data
	}
	for len(data)%alignment != 0 {
		data = append(data, 0)
	}
	return data
}

func writeSectionHeader(header []byte, section cleanSectionBuild, rawOffset uint32) {
	clear(header)
	copy(header[:8], []byte(section.Name))
	binary.LittleEndian.PutUint32(header[8:12], section.VirtualSize)
	binary.LittleEndian.PutUint32(header[12:16], section.RVA)
	binary.LittleEndian.PutUint32(header[16:20], section.RawSize)
	binary.LittleEndian.PutUint32(header[20:24], rawOffset)
	binary.LittleEndian.PutUint32(header[36:40], section.Flags)
}

func padTo(data []byte, size int) []byte {
	if len(data) >= size {
		return data
	}
	return append(data, make([]byte, size-len(data))...)
}

func alignUp32(value uint32, alignment uint32) uint32 {
	if alignment == 0 {
		return value
	}
	return (value + alignment - 1) & ^(alignment - 1)
}
