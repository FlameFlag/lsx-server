package normalizer

import (
	"encoding/binary"
	"fmt"
)

const (
	generatedTEADelta              = uint32(0x61C88647)
	generatedBootstrapSeed         = uint32(0xF1C62847)
	generatedBootstrapOffset       = 0x1F56A
	generatedBootstrapLength       = 0x7EF
	generatedBootstrapInverseMode1 = 1
	generatedSeedCRCInitial        = uint32(0xCE194941)
	generatedRangeOffsetTable      = 0x352BC
	generatedRangeLengthTable      = 0x354BC
	generatedRangePrefixTable      = 0x356BC
	generatedRangeCount            = 38
)

type generatedMutationInfo struct {
	BootstrapInverseRegions int
	StaticGlobalDwords      int
	StaticFillDwords        int
	ReplayedTEARows         int
}

type generatedTEARow struct {
	Seq          int
	SourceIndex  int
	TargetIndex  int
	Mode         int
	ExpectedSeed uint32
}

var generatedTEARows = []generatedTEARow{
	{Seq: 2, SourceIndex: 0, TargetIndex: 1, Mode: 0, ExpectedSeed: 0x6E5E2602},
	{Seq: 3, SourceIndex: 0, TargetIndex: 1, Mode: 1, ExpectedSeed: 0x6E5E2602},
	{Seq: 4, SourceIndex: 1, TargetIndex: 2, Mode: 0, ExpectedSeed: 0x4436423B},
	{Seq: 5, SourceIndex: 13, TargetIndex: 14, Mode: 0, ExpectedSeed: 0x2221B449},
	{Seq: 6, SourceIndex: 13, TargetIndex: 14, Mode: 1, ExpectedSeed: 0x2221B449},
	{Seq: 7, SourceIndex: 1, TargetIndex: 2, Mode: 1, ExpectedSeed: 0x4436423B},
	{Seq: 8, SourceIndex: 2, TargetIndex: 3, Mode: 0, ExpectedSeed: 0x48BBAB1E},
	{Seq: 9, SourceIndex: 2, TargetIndex: 3, Mode: 1, ExpectedSeed: 0x48BBAB1E},
	{Seq: 10, SourceIndex: 3, TargetIndex: 4, Mode: 0, ExpectedSeed: 0x1AAD84B0},
	{Seq: 11, SourceIndex: 5, TargetIndex: 6, Mode: 0, ExpectedSeed: 0x01007A6B},
	{Seq: 12, SourceIndex: 5, TargetIndex: 6, Mode: 1, ExpectedSeed: 0x01007A6B},
	{Seq: 13, SourceIndex: 5, TargetIndex: 6, Mode: 0, ExpectedSeed: 0x01007A6B},
	{Seq: 14, SourceIndex: 5, TargetIndex: 6, Mode: 1, ExpectedSeed: 0x01007A6B},
	{Seq: 15, SourceIndex: 3, TargetIndex: 4, Mode: 1, ExpectedSeed: 0x1AAD84B0},
	{Seq: 16, SourceIndex: 4, TargetIndex: 5, Mode: 0, ExpectedSeed: 0x31FF16DA},
	{Seq: 17, SourceIndex: 4, TargetIndex: 5, Mode: 1, ExpectedSeed: 0x31FF16DA},
	{Seq: 18, SourceIndex: 6, TargetIndex: 7, Mode: 0, ExpectedSeed: 0x0EB34D00},
	{Seq: 19, SourceIndex: 14, TargetIndex: 15, Mode: 0, ExpectedSeed: 0xA17996F6},
	{Seq: 20, SourceIndex: 14, TargetIndex: 15, Mode: 1, ExpectedSeed: 0xA17996F6},
	{Seq: 21, SourceIndex: 6, TargetIndex: 7, Mode: 1, ExpectedSeed: 0x0EB34D00},
	{Seq: 22, SourceIndex: 7, TargetIndex: 8, Mode: 0, ExpectedSeed: 0x5B4F5100},
	{Seq: 26, SourceIndex: 7, TargetIndex: 8, Mode: 1, ExpectedSeed: 0x5B4F5100},
	{Seq: 27, SourceIndex: 8, TargetIndex: 9, Mode: 0, ExpectedSeed: 0x09E41EF7},
	{Seq: 29, SourceIndex: 8, TargetIndex: 9, Mode: 1, ExpectedSeed: 0x09E41EF7},
	{Seq: 30, SourceIndex: 10, TargetIndex: 11, Mode: 0, ExpectedSeed: 0xE16252C5},
	{Seq: 31, SourceIndex: 17, TargetIndex: 18, Mode: 0, ExpectedSeed: 0x610AF46E},
	{Seq: 32, SourceIndex: 17, TargetIndex: 18, Mode: 1, ExpectedSeed: 0x610AF46E},
	{Seq: 33, SourceIndex: 17, TargetIndex: 18, Mode: 0, ExpectedSeed: 0x610AF46E},
	{Seq: 34, SourceIndex: 17, TargetIndex: 18, Mode: 1, ExpectedSeed: 0x610AF46E},
	{Seq: 35, SourceIndex: 17, TargetIndex: 18, Mode: 0, ExpectedSeed: 0x610AF46E},
	{Seq: 36, SourceIndex: 17, TargetIndex: 18, Mode: 1, ExpectedSeed: 0x610AF46E},
	{Seq: 37, SourceIndex: 17, TargetIndex: 18, Mode: 0, ExpectedSeed: 0x610AF46E},
	{Seq: 38, SourceIndex: 17, TargetIndex: 18, Mode: 1, ExpectedSeed: 0x610AF46E},
	{Seq: 39, SourceIndex: 17, TargetIndex: 18, Mode: 0, ExpectedSeed: 0x610AF46E},
	{Seq: 40, SourceIndex: 17, TargetIndex: 18, Mode: 1, ExpectedSeed: 0x610AF46E},
	{Seq: 41, SourceIndex: 17, TargetIndex: 18, Mode: 0, ExpectedSeed: 0x610AF46E},
	{Seq: 42, SourceIndex: 17, TargetIndex: 18, Mode: 1, ExpectedSeed: 0x610AF46E},
	{Seq: 43, SourceIndex: 10, TargetIndex: 11, Mode: 1, ExpectedSeed: 0xE16252C5},
	{Seq: 44, SourceIndex: 11, TargetIndex: 12, Mode: 0, ExpectedSeed: 0x2F5B9D75},
	{Seq: 45, SourceIndex: 11, TargetIndex: 12, Mode: 1, ExpectedSeed: 0x2F5B9D75},
	{Seq: 46, SourceIndex: 12, TargetIndex: 13, Mode: 0, ExpectedSeed: 0x8440189C},
	{Seq: 47, SourceIndex: 12, TargetIndex: 13, Mode: 1, ExpectedSeed: 0x8440189C},
	{Seq: 48, SourceIndex: 32, TargetIndex: 33, Mode: 0, ExpectedSeed: 0x024B0E26},
	{Seq: 49, SourceIndex: 32, TargetIndex: 33, Mode: 1, ExpectedSeed: 0x024B0E26},
	{Seq: 50, SourceIndex: 32, TargetIndex: 33, Mode: 0, ExpectedSeed: 0x024B0E26},
	{Seq: 51, SourceIndex: 32, TargetIndex: 33, Mode: 1, ExpectedSeed: 0x024B0E26},
	{Seq: 52, SourceIndex: 32, TargetIndex: 33, Mode: 0, ExpectedSeed: 0x024B0E26},
	{Seq: 53, SourceIndex: 32, TargetIndex: 33, Mode: 1, ExpectedSeed: 0x024B0E26},
	{Seq: 54, SourceIndex: 24, TargetIndex: 25, Mode: 0, ExpectedSeed: 0x0C89766E},
	{Seq: 55, SourceIndex: 24, TargetIndex: 25, Mode: 1, ExpectedSeed: 0x0C89766E},
	{Seq: 56, SourceIndex: 9, TargetIndex: 10, Mode: 0, ExpectedSeed: 0xCE892714},
	{Seq: 57, SourceIndex: 9, TargetIndex: 10, Mode: 1, ExpectedSeed: 0xCE892714},
	{Seq: 58, SourceIndex: 28, TargetIndex: 29, Mode: 0, ExpectedSeed: 0x5D3AE86C},
	{Seq: 59, SourceIndex: 28, TargetIndex: 29, Mode: 1, ExpectedSeed: 0x5D3AE86C},
	{Seq: 60, SourceIndex: 35, TargetIndex: 36, Mode: 0, ExpectedSeed: 0x65F56D61},
	{Seq: 61, SourceIndex: 35, TargetIndex: 36, Mode: 1, ExpectedSeed: 0x65F56D61},
}

func normalizeGeneratedInitialImage(image []byte, loadBase uint32) generatedMutationInfo {
	mutations := generatedMutationInfo{}
	mutations.StaticGlobalDwords = applyGeneratedStaticGlobals(image, loadBase)
	mutations.StaticFillDwords = applyGeneratedStaticFills(image)
	if generatedBootstrapOffset+generatedBootstrapLength <= len(image) {
		inverseGeneratedTEAMode1(image[generatedBootstrapOffset:generatedBootstrapOffset+generatedBootstrapLength], generatedBootstrapSeed)
		mutations.BootstrapInverseRegions = generatedBootstrapInverseMode1
	}
	return mutations
}

func applyGeneratedStaticGlobals(image []byte, loadBase uint32) int {
	globals := map[int]uint32{
		0x352B8: loadBase + 0x35CC0,
		0x38100: 0x004A5EB8,
		0x38104: 0x004B8369,
		0x38108: 0x004A5ED2,
		0x3810C: 0x004BA588,
		0x38110: 0x004BA917,
		0x38114: 0x004A6F90,
		0x38118: 0x004A57D0,
		0x3811C: 0x004A5F2B,
		0x38120: 0x004A632E,
		0x38144: loadBase + 0x38418,
		0x3841C: loadBase + 0x38130,
		0x3DE4C: loadBase + 0x3D328,
		0x3E6CC: loadBase + 0x356BC,
		0x3E6D0: loadBase,
		0x3E6D4: 0x00400000,
		0x3E6D8: 0x004E3310,
		0x3EB9C: 0x00400000,
		0x3EC50: 0x00400000,
		0x3EC58: loadBase,
		0x3EC5C: loadBase + 0x43270,
	}
	count := 0
	for offset, value := range globals {
		if offset+4 > len(image) {
			continue
		}
		binary.LittleEndian.PutUint32(image[offset:offset+4], value)
		count++
	}
	return count
}

func applyGeneratedStaticFills(image []byte) int {
	count := 0
	count += putGeneratedDword(image, 0x380E0, 0xFFFFFFFF)
	count += putGeneratedDword(image, 0x380E4, 0xFFFFFFFF)
	for offset := 0x38530; offset <= 0x39124; offset += 0x0C {
		count += putGeneratedDword(image, offset, 0xFFFFFFFF)
	}
	count += putGeneratedDword(image, 0x3E2C8, 0xFFFFFFFF)
	count += putGeneratedDword(image, 0x3E2E8, 0xFFFFFFFF)
	count += putGeneratedDword(image, 0x3E2EC, 0xFFFFFFFF)
	count += putGeneratedDword(image, 0x3E4A8, 0xFFFFFFFF)
	count += putGeneratedDword(image, 0x3E4AC, 0xFFFFFFFF)
	return count
}

func putGeneratedDword(image []byte, offset int, value uint32) int {
	if offset+4 > len(image) {
		return 0
	}
	binary.LittleEndian.PutUint32(image[offset:offset+4], value)
	return 1
}

func generatedTEAKeyWords(seed uint32) (uint32, uint32, uint32, uint32) {
	k1 := (seed << 24) | (seed >> 8)
	k2 := ((seed >> 8) << 24) | (k1 >> 8)
	k3 := ((k1 >> 8) << 24) | (k2 >> 8)
	return seed, k1, k2, k3
}

func generatedTEAMode1(data []byte, seed uint32) {
	generatedTEARegion(data, seed, 1)
}

func generatedTEARegion(data []byte, seed uint32, mode int) {
	k0, k1, k2, k3 := generatedTEAKeyWords(seed)
	end := ((len(data) >> 2) & 0x3FFFFFFE) * 4
	for offset := 0; offset < end; offset += 8 {
		v0 := binary.LittleEndian.Uint32(data[offset : offset+4])
		v1 := binary.LittleEndian.Uint32(data[offset+4 : offset+8])
		if mode <= 0 {
			sum := uint32(0xC6EF3720)
			for range 32 {
				v1 -= (((v0 >> 5) + k3) ^ ((v0 << 4) + k2) ^ (sum + v0))
				combined := sum + v1
				sum += generatedTEADelta
				v0 -= (((v1 >> 5) + k1) ^ ((v1 << 4) + k0) ^ combined)
			}
		} else {
			sum := uint32(0)
			for range 32 {
				sum -= generatedTEADelta
				v0 += (((v1 >> 5) + k1) ^ ((v1 << 4) + k0) ^ (sum + v1))
				v1 += (((v0 >> 5) + k3) ^ ((v0 << 4) + k2) ^ (sum + v0))
			}
		}
		binary.LittleEndian.PutUint32(data[offset:offset+4], v0)
		binary.LittleEndian.PutUint32(data[offset+4:offset+8], v1)
	}
}

func replayGeneratedTEARows(image []byte, loadBase uint32) (int, error) {
	ranges, err := readGeneratedDecryptRanges(image)
	if err != nil {
		return 0, err
	}
	relocations, err := readGeneratedRelocationRVAs(image)
	if err != nil {
		return 0, err
	}
	baseDelta := loadBase - generatedPreferredBase
	count := 0
	for _, row := range generatedTEARows {
		seed, err := deriveGeneratedTEASeed(image, ranges, relocations, baseDelta, row.SourceIndex)
		if err != nil {
			return count, fmt.Errorf("derive generated TEA seed for row %d: %w", row.Seq, err)
		}
		if seed != row.ExpectedSeed {
			return count, fmt.Errorf("generated TEA row %d seed 0x%08X, expected 0x%08X", row.Seq, seed, row.ExpectedSeed)
		}
		target, err := generatedRangeByIndex(ranges, row.TargetIndex)
		if err != nil {
			return count, fmt.Errorf("target for generated TEA row %d: %w", row.Seq, err)
		}
		if target.Offset+target.Length > len(image) {
			return count, fmt.Errorf("generated TEA row %d target 0x%X..0x%X exceeds image size 0x%X", row.Seq, target.Offset, target.Offset+target.Length, len(image))
		}
		generatedTEARegion(image[target.Offset:target.Offset+target.Length], seed, row.Mode)
		count++
	}
	return count, nil
}

type generatedDecryptRange struct {
	Offset int
	Length int
	Prefix int
}

func readGeneratedDecryptRanges(image []byte) ([]generatedDecryptRange, error) {
	if generatedRangeOffsetTable+generatedRangeCount*4 > len(image) ||
		generatedRangeLengthTable+generatedRangeCount*4 > len(image) ||
		generatedRangePrefixTable+generatedRangeCount*2 > len(image) {
		return nil, fmt.Errorf("generated range tables exceed image size 0x%X", len(image))
	}
	ranges := make([]generatedDecryptRange, generatedRangeCount)
	for index := range ranges {
		ranges[index] = generatedDecryptRange{
			Offset: int(binary.LittleEndian.Uint32(image[generatedRangeOffsetTable+index*4:])),
			Length: int(binary.LittleEndian.Uint32(image[generatedRangeLengthTable+index*4:])),
			Prefix: int(binary.LittleEndian.Uint16(image[generatedRangePrefixTable+index*2:])),
		}
	}
	return ranges, nil
}

func readGeneratedRelocationRVAs(image []byte) ([]int, error) {
	if len(image) < 0x40 || string(image[:2]) != "MZ" {
		return nil, fmt.Errorf("generated image missing MZ header")
	}
	peOffset := int(binary.LittleEndian.Uint32(image[0x3C:]))
	optionalOffset := peOffset + 0x18
	dataDirectoryOffset := optionalOffset + 0x60
	relocationDirectory := dataDirectoryOffset + 5*8
	if peOffset < 0 || relocationDirectory+8 > len(image) || string(image[peOffset:peOffset+4]) != "PE\x00\x00" {
		return nil, fmt.Errorf("generated image missing PE relocation directory")
	}
	relocRVA := int(binary.LittleEndian.Uint32(image[relocationDirectory:]))
	relocSize := int(binary.LittleEndian.Uint32(image[relocationDirectory+4:]))
	if relocRVA == 0 || relocSize == 0 {
		return nil, nil
	}
	end := relocRVA + relocSize
	if relocRVA < 0 || end < relocRVA || end > len(image) {
		return nil, fmt.Errorf("generated relocation directory 0x%X..0x%X exceeds image size 0x%X", relocRVA, end, len(image))
	}
	var relocations []int
	for pos := relocRVA; pos+8 <= end; {
		page := int(binary.LittleEndian.Uint32(image[pos:]))
		blockSize := int(binary.LittleEndian.Uint32(image[pos+4:]))
		pos += 8
		if blockSize < 8 || pos+blockSize-8 > end {
			return nil, fmt.Errorf("invalid generated relocation block size 0x%X", blockSize)
		}
		entryCount := (blockSize - 8) / 2
		for range entryCount {
			entry := binary.LittleEndian.Uint16(image[pos:])
			pos += 2
			if entry>>12 == 3 {
				relocations = append(relocations, page+int(entry&0x0FFF))
			}
		}
		if blockSize%2 != 0 {
			pos++
		}
	}
	return relocations, nil
}

func deriveGeneratedTEASeed(image []byte, ranges []generatedDecryptRange, relocations []int, baseDelta uint32, sourceIndex int) (uint32, error) {
	source, err := generatedRangeByIndex(ranges, sourceIndex)
	if err != nil {
		return 0, err
	}
	start := source.Offset - source.Prefix
	end := source.Offset + source.Length
	if start < 0 || end < start || end > len(image) {
		return 0, fmt.Errorf("source range %d 0x%X..0x%X exceeds image size 0x%X", sourceIndex, start, end, len(image))
	}
	buffer := append([]byte(nil), image[start:end]...)
	if sourceIndex == 0 {
		bootstrapStart := source.Offset - start
		bootstrapEnd := bootstrapStart + source.Length
		if bootstrapEnd > len(buffer) {
			return 0, fmt.Errorf("bootstrap source range exceeds seed buffer")
		}
		generatedTEAMode1(buffer[bootstrapStart:bootstrapEnd], generatedBootstrapSeed)
	}
	for _, relocation := range relocations {
		if relocation < start || relocation+4 > end {
			continue
		}
		offset := relocation - start
		value := binary.LittleEndian.Uint32(buffer[offset:])
		binary.LittleEndian.PutUint32(buffer[offset:], value-baseDelta)
	}
	return generatedCRC32(buffer, generatedSeedCRCInitial), nil
}

func generatedRangeByIndex(ranges []generatedDecryptRange, index int) (generatedDecryptRange, error) {
	if index < 0 || index >= len(ranges) {
		return generatedDecryptRange{}, fmt.Errorf("range index %d out of bounds", index)
	}
	return ranges[index], nil
}

func inverseGeneratedTEAMode1(data []byte, seed uint32) {
	k0, k1, k2, k3 := generatedTEAKeyWords(seed)
	end := ((len(data) >> 2) & 0x3FFFFFFE) * 4
	for offset := 0; offset < end; offset += 8 {
		v0 := binary.LittleEndian.Uint32(data[offset : offset+4])
		v1 := binary.LittleEndian.Uint32(data[offset+4 : offset+8])
		var sums [32]uint32
		sum := uint32(0)
		for i := range sums {
			sum -= generatedTEADelta
			sums[i] = sum
		}
		for i := len(sums) - 1; i >= 0; i-- {
			sum = sums[i]
			v1 -= (((v0 >> 5) + k3) ^ ((v0 << 4) + k2) ^ (sum + v0))
			v0 -= (((v1 >> 5) + k1) ^ ((v1 << 4) + k0) ^ (sum + v1))
		}
		binary.LittleEndian.PutUint32(data[offset:offset+4], v0)
		binary.LittleEndian.PutUint32(data[offset+4:offset+8], v1)
	}
}
