package normalizer

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

const (
	mapperWindowExpectedSeed   = uint32(0x031E1692)
	mapperWindowSize           = 0x2800
	mapperTailSelectedOffset   = 4
	mapperData1ContextOffset   = 0x310
	mapperExpectedZlibOffset   = 0x0C
	mapperWindowExpectedSHA256 = "198c22b83d188f2aedf2b152822bcb51fe3707d306ec2e166bdc69c56dd97b67"
)

type mapperWindowInfo struct {
	OutputPath string
	Seed       uint32
	TailOffset int
	Magic      uint32
	SHA256     string
	Metadata   mapperMetadata
}

type mapperMetadata struct {
	Flags        uint32
	KeyDwords    [4]uint32
	Checksum     uint32
	ProductName  string
	Dependencies []string
	Records      []mapperMetadataRecord
	DataEntries  []mapperMetadataDataEntry
	RecordOffset int
	DataOffset   int
	Size         int
}

type mapperMetadataRecord struct {
	Kind    byte
	ID      uint32
	Payload []byte
}

type mapperMetadataDataEntry struct {
	Offset int
	Size   uint32
	Tag    byte
}

func deriveMapperWindow(packedPath, outputPath string) (mapperWindowInfo, error) {
	windowInfo, err := deriveMapperWindowBytes(packedPath)
	if err != nil {
		return mapperWindowInfo{}, err
	}
	if err := os.WriteFile(outputPath, windowInfo.Window, 0o644); err != nil {
		return mapperWindowInfo{}, err
	}
	return mapperWindowInfo{OutputPath: outputPath, Seed: windowInfo.Seed, TailOffset: windowInfo.TailOffset, Magic: windowInfo.Magic, SHA256: windowInfo.SHA256, Metadata: windowInfo.Metadata}, nil
}

func recoverMapperWindowSeed(encryptedWindow []byte, expectedMagic uint32) (uint32, error) {
	if len(encryptedWindow) < 4 {
		return 0, fmt.Errorf("mapper window too small for seed recovery: 0x%X", len(encryptedWindow))
	}
	keyDword := littleEndianUint32(encryptedWindow[:4]) ^ expectedMagic
	prngBytes := []byte{byte(keyDword >> 24), byte(keyDword >> 16), byte(keyDword >> 8), byte(keyDword)}
	candidates := recoverGeneratedPRNGSeedCandidates(prngBytes)
	if len(candidates) == 0 {
		return 0, errors.New("failed to recover mapper PRNG seed from first dword")
	}
	for _, seed := range candidates {
		window := append([]byte(nil), encryptedWindow...)
		generatedXORDwordsWithPRNG(window, seed)
		if littleEndianUint32(window[:4]) != expectedMagic || len(window) <= mapperExpectedZlibOffset+1 || !isZlibMagicAt(window, mapperExpectedZlibOffset) {
			continue
		}
		if _, err := parseMapperMetadata(window); err != nil {
			continue
		}
		if seed != mapperWindowExpectedSeed {
			return 0, fmt.Errorf("mapper PRNG seed 0x%08X, expected 0x%08X", seed, mapperWindowExpectedSeed)
		}
		return seed, nil
	}
	return 0, errors.New("no mapper PRNG seed candidate produced valid metadata")
}

func recoverGeneratedPRNGSeed(outputs []byte) (uint32, bool) {
	candidates := recoverGeneratedPRNGSeedCandidates(outputs)
	if len(candidates) == 0 {
		return 0, false
	}
	return candidates[0], true
}

func recoverGeneratedPRNGSeedCandidates(outputs []byte) []uint32 {
	if len(outputs) == 0 {
		return nil
	}
	lo, hi := generatedPRNGByteInterval(outputs[0])
	candidates := []uint32{}
	for firstState := lo; firstState <= hi; firstState++ {
		state := firstState
		matched := true
		for _, expected := range outputs[1:] {
			if got := byte(generatedPRNGNext(&state)); got != expected {
				matched = false
				break
			}
		}
		if matched {
			seed := generatedPRNGMulMod100M((firstState+generatedPRNGModulus-1)%generatedPRNGModulus, generatedPRNGMultiplierInverse)
			candidates = append(candidates, seed)
		}
	}
	return candidates
}

func generatedPRNGByteInterval(value byte) (uint32, uint32) {
	lo := (uint64(value)*uint64(generatedPRNGModulus) + 255) / 256
	hi := ((uint64(value)+1)*uint64(generatedPRNGModulus)+255)/256 - 1
	return uint32(lo), uint32(hi)
}

func parseMapperMetadata(window []byte) (mapperMetadata, error) {
	if len(window) < mapperExpectedZlibOffset+4 {
		return mapperMetadata{}, fmt.Errorf("mapper window too small: 0x%X", len(window))
	}
	metadataSize := int(littleEndianUint32(window[4:8]))
	compressedSize := int(littleEndianUint32(window[8:12]))
	compressedStart := mapperExpectedZlibOffset
	compressedEnd := compressedStart + compressedSize
	if metadataSize <= 0 || compressedSize <= 0 || compressedEnd > len(window) {
		return mapperMetadata{}, fmt.Errorf("invalid mapper metadata sizes out=0x%X compressed=0x%X", metadataSize, compressedSize)
	}
	reader, err := zlib.NewReader(bytes.NewReader(window[compressedStart:compressedEnd]))
	if err != nil {
		return mapperMetadata{}, fmt.Errorf("open mapper metadata zlib stream: %w", err)
	}
	metadata, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil {
		return mapperMetadata{}, readErr
	}
	if closeErr != nil {
		return mapperMetadata{}, closeErr
	}
	if len(metadata) != metadataSize {
		return mapperMetadata{}, fmt.Errorf("mapper metadata size 0x%X, expected 0x%X", len(metadata), metadataSize)
	}
	if len(metadata) < 0x1A {
		return mapperMetadata{}, fmt.Errorf("mapper metadata too short: 0x%X", len(metadata))
	}

	result := mapperMetadata{
		Flags: littleEndianUint32(metadata[:4]),
		KeyDwords: [4]uint32{
			littleEndianUint32(metadata[4:8]),
			littleEndianUint32(metadata[8:12]),
			littleEndianUint32(metadata[12:16]),
			littleEndianUint32(metadata[16:20]),
		},
		Size: len(metadata),
	}
	cursor := 0x14
	if cursor+4 > len(metadata) {
		return mapperMetadata{}, errors.New("mapper metadata missing checksum field")
	}
	result.Checksum = littleEndianUint32(metadata[cursor:])
	cursor += 4
	product, next, err := readLengthPrefixedString(metadata, cursor)
	if err != nil {
		return mapperMetadata{}, err
	}
	result.ProductName = product
	cursor = next
	for cursor < len(metadata) && metadata[cursor] != 0 {
		length := int(metadata[cursor])
		cursor++
		if cursor+length > len(metadata) {
			return mapperMetadata{}, fmt.Errorf("mapper dependency at 0x%X exceeds metadata size 0x%X", cursor-1, len(metadata))
		}
		result.Dependencies = append(result.Dependencies, string(metadata[cursor:cursor+length]))
		cursor += length
	}
	if cursor >= len(metadata) {
		return mapperMetadata{}, errors.New("mapper dependency list is unterminated")
	}
	cursor++
	dependencyBlobSize, next, err := readLengthPrefixedBlobSize(metadata, cursor)
	if err != nil {
		return mapperMetadata{}, err
	}
	cursor = next
	if cursor+dependencyBlobSize > len(metadata) {
		return mapperMetadata{}, fmt.Errorf("mapper dependency blob 0x%X..0x%X exceeds metadata size 0x%X", cursor, cursor+dependencyBlobSize, len(metadata))
	}
	for item := range bytes.SplitSeq(metadata[cursor:cursor+dependencyBlobSize], []byte{0}) {
		if len(item) != 0 {
			result.Dependencies = append(result.Dependencies, string(item))
		}
	}
	cursor += dependencyBlobSize
	result.RecordOffset = cursor
	for cursor+2 <= len(metadata) && metadata[cursor+1] != 0 {
		length := int(metadata[cursor+1])
		if cursor+6+length > len(metadata) {
			return mapperMetadata{}, fmt.Errorf("mapper record at 0x%X exceeds metadata size 0x%X", cursor, len(metadata))
		}
		result.Records = append(result.Records, mapperMetadataRecord{
			Kind:    metadata[cursor],
			ID:      binary.LittleEndian.Uint32(metadata[cursor+2 : cursor+6]),
			Payload: append([]byte(nil), metadata[cursor+6:cursor+6+length]...),
		})
		cursor += 6 + length
	}
	if cursor+2 > len(metadata) {
		return result, nil
	}
	result.DataOffset = cursor + 2
	cursor = result.DataOffset
	for cursor+5 <= len(metadata) {
		size := binary.LittleEndian.Uint32(metadata[cursor : cursor+4])
		if size == 0 {
			break
		}
		end := cursor + 5 + int(size)
		if end > len(metadata) {
			return mapperMetadata{}, fmt.Errorf("mapper data entry at 0x%X exceeds metadata size 0x%X", cursor, len(metadata))
		}
		result.DataEntries = append(result.DataEntries, mapperMetadataDataEntry{Offset: cursor, Size: size, Tag: metadata[cursor+4]})
		cursor = end
	}
	return result, nil
}

func readLengthPrefixedString(data []byte, offset int) (string, int, error) {
	size, next, err := readLengthPrefixedBlobSize(data, offset)
	if err != nil {
		return "", 0, err
	}
	if next+size > len(data) {
		return "", 0, fmt.Errorf("length-prefixed string at 0x%X exceeds metadata size 0x%X", offset, len(data))
	}
	return string(data[next : next+size]), next + size, nil
}

func readLengthPrefixedBlobSize(data []byte, offset int) (int, int, error) {
	if offset+2 > len(data) {
		return 0, 0, fmt.Errorf("length field at 0x%X exceeds metadata size 0x%X", offset, len(data))
	}
	return int(binary.LittleEndian.Uint16(data[offset : offset+2])), offset + 2, nil
}

func mapperMagicFromData1(data1 []byte) (uint32, error) {
	if len(data1) < mapperData1ContextOffset+0x3C {
		return 0, fmt.Errorf(".data1 too small for mapper magic context: 0x%X", len(data1))
	}
	base := mapperData1ContextOffset
	return littleEndianUint32(data1[base+0x38:]) ^
		littleEndianUint32(data1[base+0x24:]) ^
		littleEndianUint32(data1[base+0x10:]), nil
}

func findPDATATailOffset(sectionData []byte) (int, error) {
	firstStreamOffset := len(pdataSignature) + pdataFirstMetadataSize
	if len(sectionData) < firstStreamOffset+2 || !bytes.HasPrefix(sectionData, pdataSignature) {
		return 0, errors.New(".pdata signature mismatch or section too small")
	}
	lastEnd := firstStreamOffset
	for searchOffset := firstStreamOffset; searchOffset < len(sectionData)-1; {
		streamOffset := searchOffset
		if streamOffset != firstStreamOffset {
			streamOffset = findNextZlibMagic(sectionData, searchOffset)
			if streamOffset < 0 {
				break
			}
		}
		_, compressedSize, err := inflatePDATAStream(sectionData, streamOffset)
		if err != nil {
			if streamOffset == firstStreamOffset {
				return 0, fmt.Errorf("inflate first .pdata stream: %w", err)
			}
			searchOffset = streamOffset + 1
			continue
		}
		lastEnd = streamOffset + compressedSize
		searchOffset = lastEnd
	}
	return lastEnd, nil
}
