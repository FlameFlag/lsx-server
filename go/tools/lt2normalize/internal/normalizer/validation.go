package normalizer

import (
	"bytes"
	"compress/zlib"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
)

const (
	validationDefaultSeed        = uint32(0xCCF0580A)
	validationExpectedEntryTag   = byte(0x04)
	validationExpectedEntryIndex = 1
	validationMetadataDataOffset = 0xFE
)

type validationEntryInfo struct {
	OutputPath  string
	Seed        uint32
	Tag         byte
	EntryIndex  int
	EntryOffset int
	EntrySize   int
	RepeatCount int
	Shift       int
	Mutations   int
	MD5         string
	TrailerA    uint32
	TrailerB    uint32
	Complement  bool
}

func deriveMapperValidationEntry(packedPath, outputPath string, seed uint32) (validationEntryInfo, error) {
	metadata, data1, err := readMapperMetadataAndData1(packedPath)
	if err != nil {
		return validationEntryInfo{}, err
	}
	entries, err := mapperValidationEntries(metadata)
	if err != nil {
		return validationEntryInfo{}, err
	}
	if len(data1) < mapperData1ContextOffset+0x40 {
		return validationEntryInfo{}, fmt.Errorf(".data1 too small for validation fields: 0x%X", len(data1))
	}
	data1Field38 := binary.LittleEndian.Uint32(data1[mapperData1ContextOffset+0x38:])
	data1Field3c := binary.LittleEndian.Uint32(data1[mapperData1ContextOffset+0x3C:])
	key, tag, repeatCount, shift, mutationCount, digest := deriveValidationKey(seed, data1Field38, data1Field3c, false)

	var selected *mapperValidationEntry
	for index := range entries {
		if entries[index].Tag == tag {
			selected = &entries[index]
			break
		}
	}
	if selected == nil {
		return validationEntryInfo{}, fmt.Errorf("validation seed 0x%08X selected missing tag 0x%02X", seed, tag)
	}
	decoded := append([]byte(nil), selected.Payload...)
	validationTEA(decoded, key, true)
	for range repeatCount {
		validationTEA(decoded, key, false)
	}
	validationTEA(decoded, key, true)
	if len(decoded) < 8 {
		return validationEntryInfo{}, fmt.Errorf("decoded validation entry too small: 0x%X", len(decoded))
	}
	trailerA := binary.LittleEndian.Uint32(decoded[len(decoded)-8:])
	trailerB := binary.LittleEndian.Uint32(decoded[len(decoded)-4:])
	complement := trailerA == ^trailerB
	if !complement {
		return validationEntryInfo{}, fmt.Errorf("decoded validation entry trailer 0x%08X/0x%08X is not complemented", trailerA, trailerB)
	}
	if err := os.WriteFile(outputPath, decoded, 0o644); err != nil {
		return validationEntryInfo{}, err
	}
	return validationEntryInfo{
		OutputPath:  outputPath,
		Seed:        seed,
		Tag:         tag,
		EntryIndex:  selected.Index,
		EntryOffset: selected.Offset,
		EntrySize:   len(selected.Payload),
		RepeatCount: repeatCount,
		Shift:       shift,
		Mutations:   mutationCount,
		MD5:         hex.EncodeToString(digest[:]),
		TrailerA:    trailerA,
		TrailerB:    trailerB,
		Complement:  complement,
	}, nil
}

type mapperValidationEntry struct {
	Index   int
	Offset  int
	Tag     byte
	Payload []byte
}

func readMapperMetadataAndData1(packedPath string) ([]byte, []byte, error) {
	pdata, _, err := readPDATASection(packedPath)
	if err != nil {
		return nil, nil, err
	}
	data1, err := readSectionData(packedPath, ".data1")
	if err != nil {
		return nil, nil, err
	}
	tailOffset, err := findPDATATailOffset(pdata)
	if err != nil {
		return nil, nil, err
	}
	windowOffset := tailOffset + mapperTailSelectedOffset
	if windowOffset+mapperWindowSize > len(pdata) {
		return nil, nil, fmt.Errorf("mapper window 0x%X..0x%X exceeds .pdata size 0x%X", windowOffset, windowOffset+mapperWindowSize, len(pdata))
	}
	window := append([]byte(nil), pdata[windowOffset:windowOffset+mapperWindowSize]...)
	expectedMagic, err := mapperMagicFromData1(data1)
	if err != nil {
		return nil, nil, err
	}
	seed, err := recoverMapperWindowSeed(window, expectedMagic)
	if err != nil {
		return nil, nil, err
	}
	generatedXORDwordsWithPRNG(window, seed)
	metadataSize := int(binary.LittleEndian.Uint32(window[4:8]))
	compressedSize := int(binary.LittleEndian.Uint32(window[8:12]))
	compressedStart := mapperExpectedZlibOffset
	compressedEnd := compressedStart + compressedSize
	if compressedEnd > len(window) {
		return nil, nil, errors.New("mapper metadata compressed range exceeds window")
	}
	reader, err := zlib.NewReader(bytes.NewReader(window[compressedStart:compressedEnd]))
	if err != nil {
		return nil, nil, err
	}
	metadata, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil {
		return nil, nil, readErr
	}
	if closeErr != nil {
		return nil, nil, closeErr
	}
	if len(metadata) != metadataSize {
		return nil, nil, fmt.Errorf("mapper metadata size 0x%X, expected 0x%X", len(metadata), metadataSize)
	}
	return metadata, data1, nil
}

func mapperValidationEntries(metadata []byte) ([]mapperValidationEntry, error) {
	var entries []mapperValidationEntry
	for cursor := validationMetadataDataOffset; cursor+5 <= len(metadata); {
		size := int(binary.LittleEndian.Uint32(metadata[cursor:]))
		if size == 0 {
			break
		}
		end := cursor + 5 + size
		if end > len(metadata) {
			return nil, fmt.Errorf("validation entry at 0x%X exceeds metadata size 0x%X", cursor, len(metadata))
		}
		entries = append(entries, mapperValidationEntry{
			Index:   len(entries),
			Offset:  cursor,
			Tag:     metadata[cursor+4],
			Payload: append([]byte(nil), metadata[cursor+5:end]...),
		})
		cursor = end
	}
	return entries, nil
}

func deriveValidationKey(seed, data1Field38, data1Field3c uint32, longCountFlag bool) ([4]uint32, byte, int, int, int, [16]byte) {
	local20 := data1Field38 ^ seed
	local28 := seed
	local60 := local20*10 + 1
	words := make([]uint32, 256)
	for index := range words {
		a := generatedPRNGDwordSigned(&local60)
		b := generatedPRNGDwordSigned(&local28)
		c := generatedPRNGDwordSigned(&local20)
		words[index] = a ^ b ^ c ^ seed
	}
	shift, mutationCount := mutateValidationWords(words, &local20, &local28, data1Field3c, longCountFlag)
	buffer := make([]byte, len(words)*4)
	for index, word := range words {
		binary.LittleEndian.PutUint32(buffer[index*4:], word)
	}
	digest := md5.Sum(buffer)
	key := [4]uint32{
		binary.LittleEndian.Uint32(digest[0:4]),
		binary.LittleEndian.Uint32(digest[4:8]),
		binary.LittleEndian.Uint32(digest[8:12]),
		binary.LittleEndian.Uint32(digest[12:16]),
	}
	local44 := ^seed ^ data1Field38
	local44 = (generatedPRNGMulMod100MSigned(local44, generatedPRNGMultiplier) + 1) % 100000000
	repeatCount := int(((local44 / 10000) * 400 / 10000) + 0x321)
	return key, digest[0] & 7, repeatCount, shift, mutationCount, digest
}

func mutateValidationWords(words []uint32, local20, local28 *uint32, data1Field3c uint32, longCountFlag bool) (int, int) {
	*local20 = (generatedPRNGMulMod100MSigned(*local20, generatedPRNGMultiplier) + 1) % 100000000
	*local28 = (generatedPRNGMulMod100MSigned(*local28, generatedPRNGMultiplier) + 1) % 100000000
	shift := int((((*local28/10000)<<4)/10000)+(((*local20/10000)<<4)/10000)) & 0x1F
	mask := uint32(0x7FFF)
	base := 5000
	if longCountFlag {
		mask = 0xFFFF
		base = 10000
	}
	count := int(data1Field3c&mask) + base
	index := 0
	for iteration := range count {
		if iteration&0xFF == 0 {
			index = 0
		}
		selector := (words[index] >> shift) & 3
		switch selector {
		case 0:
			words[index] |= generatedPRNGDwordSigned(local20)
			index++
			_ = generatedPRNGDwordSigned(local28)
		case 1:
			words[index] &= generatedPRNGDwordSigned(local28)
			index++
			_ = generatedPRNGDwordSigned(local20)
		default:
			words[index] ^= generatedPRNGDwordSigned(local28) ^ generatedPRNGDwordSigned(local20)
			index++
		}
	}
	return shift, count
}

func validationTEA(data []byte, key [4]uint32, chaining bool) {
	carry0 := key[1]
	carry1 := key[3]
	end := ((len(data) >> 2) & 0x3FFFFFFE) * 4
	for offset := 0; offset < end; offset += 8 {
		v0 := binary.LittleEndian.Uint32(data[offset:])
		v1 := binary.LittleEndian.Uint32(data[offset+4:])
		sum := uint32(0xC6EF3720)
		for range 32 {
			v1 -= (((v0 >> 5) + carry1) ^ ((v0 << 4) + key[2]) ^ (sum + v0))
			combined := sum + v1
			sum += generatedTEADelta
			v0 -= (((v1 >> 5) + carry0) ^ ((v1 << 4) + key[0]) ^ combined)
		}
		binary.LittleEndian.PutUint32(data[offset:], v0)
		binary.LittleEndian.PutUint32(data[offset+4:], v1)
		if chaining {
			carry0 = v0
			carry1 = v1
		}
	}
}
