package normalizer

import (
	"fmt"
	"os"
)

type mapperStreamInfo struct {
	OutputPath                 string
	TailOffset                 int
	StreamOffset               int
	Size                       int
	DecodedWindowSize          int
	LoaderSelectedVA           uint32
	LoaderSectionVA            uint32
	LoaderRawWindowSize        int
	LoaderEffectiveWindowSize  int
	LoaderSeed                 uint32
	LoaderCRCMix               uint32
	LoaderOriginalTextRVA      uint32
	LoaderOriginalTextSize     uint32
	LoaderOriginalTextPages    uint32
	LoaderOptionalCRCMask      uint32
	LoaderChunkKey             uint32
	LoaderChunkLimitKey        uint32
	InitialDecodeLimit         int
	InitialDecodeHeaderOffset  int
	InitialDecodeSourceSize    int
	InitialDecodeTerminatorOff int
	PostMetadataOffset         int
	PayloadKey                 uint32
	PayloadEntries             []mapperPayloadEntry
	PayloadTerminatorOffset    int
	SHA256                     string
}

type mapperPayloadEntry struct {
	RVA       uint32
	Size      uint32
	HeaderOff int
	Chunks    []mapperPayloadChunk
	SourceLen int
}

type mapperPayloadChunk struct {
	HeaderOff int
	SourceOff int
	SourceLen int
	Stored    bool
}

func deriveMapperStream(packedPath, outputPath string) (mapperStreamInfo, error) {
	stream, info, err := buildMapperStream(packedPath)
	if err != nil {
		return mapperStreamInfo{}, err
	}
	if err := os.WriteFile(outputPath, stream, 0o644); err != nil {
		return mapperStreamInfo{}, err
	}
	info.OutputPath = outputPath
	return info, nil
}

func buildMapperStream(packedPath string) ([]byte, mapperStreamInfo, error) {
	pdata, _, err := readPDATASection(packedPath)
	if err != nil {
		return nil, mapperStreamInfo{}, err
	}
	tailOffset, err := findPDATATailOffset(pdata)
	if err != nil {
		return nil, mapperStreamInfo{}, err
	}
	streamOffset := tailOffset + mapperTailSelectedOffset
	if streamOffset+mapperWindowSize > len(pdata) {
		return nil, mapperStreamInfo{}, fmt.Errorf("mapper selected stream 0x%X..0x%X exceeds .pdata size 0x%X", streamOffset, streamOffset+mapperWindowSize, len(pdata))
	}

	windowInfo, err := deriveMapperWindowBytes(packedPath)
	if err != nil {
		return nil, mapperStreamInfo{}, err
	}
	data1, err := readSectionData(packedPath, ".data1")
	if err != nil {
		return nil, mapperStreamInfo{}, err
	}
	pdataVA, err := readSectionVA(packedPath, ".pdata")
	if err != nil {
		return nil, mapperStreamInfo{}, err
	}
	stream := append([]byte(nil), windowInfo.Window...)
	stream = append(stream, pdata[streamOffset+mapperWindowSize:]...)
	selectedVA := pdataVA + uint32(streamOffset)
	loaderRawWindowSize, loaderEffectiveWindowSize, err := deriveLoaderWindowSize(data1, selectedVA, pdataVA)
	if err != nil {
		return nil, mapperStreamInfo{}, err
	}
	if loaderEffectiveWindowSize != mapperWindowSize {
		return nil, mapperStreamInfo{}, fmt.Errorf("mapper loader effective window size 0x%X, expected 0x%X", loaderEffectiveWindowSize, mapperWindowSize)
	}
	initialDecodeLimit := int(littleEndianUint32(stream[4:8]))
	initialDecodeHeaderOffset := 8
	initialDecodeSourceSize := int(littleEndianUint32(stream[initialDecodeHeaderOffset:]))
	if initialDecodeLimit != windowInfo.Metadata.Size || initialDecodeSourceSize != windowInfo.MetadataCompressedSize {
		return nil, mapperStreamInfo{}, fmt.Errorf("mapper initial decode mismatch: limit/source 0x%X/0x%X, metadata 0x%X/0x%X",
			initialDecodeLimit, initialDecodeSourceSize, windowInfo.Metadata.Size, windowInfo.MetadataCompressedSize)
	}
	postMetadataOffset := mapperExpectedZlibOffset + initialDecodeSourceSize + 4
	if postMetadataOffset > mapperWindowSize {
		return nil, mapperStreamInfo{}, fmt.Errorf("mapper post-metadata offset 0x%X exceeds decoded window 0x%X", postMetadataOffset, mapperWindowSize)
	}
	terminatorOffset := postMetadataOffset - 4
	if littleEndianUint32(stream[terminatorOffset:postMetadataOffset]) != 0 {
		return nil, mapperStreamInfo{}, fmt.Errorf("mapper metadata chunk terminator at 0x%X is not zero", terminatorOffset)
	}
	payloadKey := derivePayloadEntryKey(data1)
	payloadEntries, payloadTerminatorOffset, err := parseMapperPayloadEntries(stream, postMetadataOffset, payloadKey)
	if err != nil {
		return nil, mapperStreamInfo{}, err
	}
	return stream, mapperStreamInfo{
		TailOffset:                 tailOffset,
		StreamOffset:               streamOffset,
		Size:                       len(stream),
		DecodedWindowSize:          mapperWindowSize,
		LoaderSelectedVA:           selectedVA,
		LoaderSectionVA:            pdataVA,
		LoaderRawWindowSize:        loaderRawWindowSize,
		LoaderEffectiveWindowSize:  loaderEffectiveWindowSize,
		LoaderSeed:                 windowInfo.Seed,
		LoaderCRCMix:               deriveLoaderCRCMix(data1, windowInfo.Seed),
		LoaderOriginalTextRVA:      deriveLoaderOriginalTextRVA(data1),
		LoaderOriginalTextSize:     deriveLoaderOriginalTextSize(data1),
		LoaderOriginalTextPages:    deriveLoaderOriginalTextPages(data1),
		LoaderOptionalCRCMask:      deriveLoaderOptionalCRCMask(data1),
		LoaderChunkKey:             deriveLoaderChunkKey(data1),
		LoaderChunkLimitKey:        deriveLoaderChunkLimitKey(data1),
		InitialDecodeLimit:         initialDecodeLimit,
		InitialDecodeHeaderOffset:  initialDecodeHeaderOffset,
		InitialDecodeSourceSize:    initialDecodeSourceSize,
		InitialDecodeTerminatorOff: terminatorOffset,
		PostMetadataOffset:         postMetadataOffset,
		PayloadKey:                 payloadKey,
		PayloadEntries:             payloadEntries,
		PayloadTerminatorOffset:    payloadTerminatorOffset,
		SHA256:                     sha256Hex(stream),
	}, nil
}

func derivePayloadEntryKey(data1 []byte) uint32 {
	base := mapperData1ContextOffset
	return littleEndianUint32(data1[base+0x18:]) ^ littleEndianUint32(data1[base+0x24:])
}

func parseMapperPayloadEntries(stream []byte, offset int, key uint32) ([]mapperPayloadEntry, int, error) {
	entries := []mapperPayloadEntry{}
	cursor := offset
	for {
		if cursor+4 > len(stream) {
			return nil, 0, fmt.Errorf("mapper payload entry header at 0x%X exceeds stream size 0x%X", cursor, len(stream))
		}
		entryHeader := cursor
		rva := littleEndianUint32(stream[cursor:]) ^ key
		cursor += 4
		if rva == 0 {
			return entries, entryHeader, nil
		}
		if cursor+4 > len(stream) {
			return nil, 0, fmt.Errorf("mapper payload size at 0x%X exceeds stream size 0x%X", cursor, len(stream))
		}
		size := littleEndianUint32(stream[cursor:]) ^ key
		cursor += 4
		entry := mapperPayloadEntry{RVA: rva, Size: size, HeaderOff: entryHeader}
		for {
			if cursor+4 > len(stream) {
				return nil, 0, fmt.Errorf("mapper payload chunk header at 0x%X exceeds stream size 0x%X", cursor, len(stream))
			}
			chunkHeader := cursor
			rawLength := littleEndianUint32(stream[cursor:]) ^ key
			cursor += 4
			stored := rawLength&0x80000000 != 0
			sourceLen := int(rawLength & 0x7FFFFFFF)
			chunk := mapperPayloadChunk{HeaderOff: chunkHeader, SourceOff: cursor, SourceLen: sourceLen, Stored: stored}
			entry.Chunks = append(entry.Chunks, chunk)
			if sourceLen == 0 {
				break
			}
			if cursor+sourceLen < cursor || cursor+sourceLen > len(stream) {
				return nil, 0, fmt.Errorf("mapper payload chunk body 0x%X..0x%X exceeds stream size 0x%X", cursor, cursor+sourceLen, len(stream))
			}
			entry.SourceLen += sourceLen
			cursor += sourceLen
		}
		entries = append(entries, entry)
	}
}

func deriveLoaderWindowSize(data1 []byte, selectedVA, sectionVA uint32) (int, int, error) {
	if len(data1) < mapperData1ContextOffset+0x70 {
		return 0, 0, fmt.Errorf(".data1 too small for loader window fields: 0x%X", len(data1))
	}
	base := mapperData1ContextOffset
	windowExpr := littleEndianUint32(data1[base+0x6C:]) ^
		littleEndianUint32(data1[base+0x68:]) ^
		littleEndianUint32(data1[base+0x24:]) ^
		littleEndianUint32(data1[base+0x18:])
	rawSize := int(uint32(windowExpr - selectedVA + sectionVA))
	effectiveSize := rawSize
	if effectiveSize > mapperWindowSize {
		effectiveSize = mapperWindowSize
	}
	return rawSize, effectiveSize, nil
}

func deriveLoaderCRCMix(data1 []byte, seed uint32) uint32 {
	base := mapperData1ContextOffset
	return seed ^
		littleEndianUint32(data1[base+0x6C:]) ^
		littleEndianUint32(data1[base+0x24:]) ^
		littleEndianUint32(data1[base+0x18:])
}

func deriveLoaderOriginalTextRVA(data1 []byte) uint32 {
	base := mapperData1ContextOffset
	return littleEndianUint32(data1[base+0x74:]) ^
		littleEndianUint32(data1[base+0x54:]) ^
		littleEndianUint32(data1[base+0x14:])
}

func deriveLoaderOriginalTextSize(data1 []byte) uint32 {
	base := mapperData1ContextOffset
	return littleEndianUint32(data1[base+0x6C:]) ^
		littleEndianUint32(data1[base+0x58:]) ^
		littleEndianUint32(data1[base+0x18:])
}

func deriveLoaderOriginalTextPages(data1 []byte) uint32 {
	return (deriveLoaderOriginalTextSize(data1) + 0xFFF) >> 12
}

func deriveLoaderOptionalCRCMask(data1 []byte) uint32 {
	base := mapperData1ContextOffset
	return littleEndianUint32(data1[base+0x64:]) ^
		littleEndianUint32(data1[base+0x14:]) ^
		littleEndianUint32(data1[base+0x6C:])
}

func deriveLoaderChunkKey(data1 []byte) uint32 {
	base := mapperData1ContextOffset
	return littleEndianUint32(data1[base+0x14:]) ^
		littleEndianUint32(data1[base+0x6C:])
}

func deriveLoaderChunkLimitKey(data1 []byte) uint32 {
	base := mapperData1ContextOffset
	return littleEndianUint32(data1[base+0x60:]) ^
		littleEndianUint32(data1[base+0x3C:]) ^
		littleEndianUint32(data1[base+0x14:])
}

type mapperWindowBytes struct {
	Window                 []byte
	TailOffset             int
	Magic                  uint32
	Seed                   uint32
	Metadata               mapperMetadata
	MetadataCompressedSize int
	SHA256                 string
}

func deriveMapperWindowBytes(packedPath string) (mapperWindowBytes, error) {
	pdata, _, err := readPDATASection(packedPath)
	if err != nil {
		return mapperWindowBytes{}, err
	}
	data1, err := readSectionData(packedPath, ".data1")
	if err != nil {
		return mapperWindowBytes{}, err
	}
	tailOffset, err := findPDATATailOffset(pdata)
	if err != nil {
		return mapperWindowBytes{}, err
	}
	windowOffset := tailOffset + mapperTailSelectedOffset
	if windowOffset+mapperWindowSize > len(pdata) {
		return mapperWindowBytes{}, fmt.Errorf("mapper window 0x%X..0x%X exceeds .pdata size 0x%X", windowOffset, windowOffset+mapperWindowSize, len(pdata))
	}

	expectedMagic, err := mapperMagicFromData1(data1)
	if err != nil {
		return mapperWindowBytes{}, err
	}
	window := append([]byte(nil), pdata[windowOffset:windowOffset+mapperWindowSize]...)
	seed, err := recoverMapperWindowSeed(window, expectedMagic)
	if err != nil {
		return mapperWindowBytes{}, err
	}
	generatedXORDwordsWithPRNG(window, seed)
	magic := littleEndianUint32(window[:4])
	if magic != expectedMagic {
		return mapperWindowBytes{}, fmt.Errorf("mapper magic mismatch after PRNG decode: got 0x%08X, expected 0x%08X", magic, expectedMagic)
	}
	if len(window) <= mapperExpectedZlibOffset+1 || !isZlibMagicAt(window, mapperExpectedZlibOffset) {
		return mapperWindowBytes{}, fmt.Errorf("mapper window missing zlib member at +0x%X", mapperExpectedZlibOffset)
	}
	compressedSize := int(littleEndianUint32(window[8:12]))
	metadata, err := parseMapperMetadata(window)
	if err != nil {
		return mapperWindowBytes{}, err
	}
	digest := sha256Hex(window)
	if digest != mapperWindowExpectedSHA256 {
		return mapperWindowBytes{}, fmt.Errorf("mapper window sha256 %s, expected %s", digest, mapperWindowExpectedSHA256)
	}
	return mapperWindowBytes{
		Window:                 window,
		TailOffset:             tailOffset,
		Magic:                  magic,
		Seed:                   seed,
		Metadata:               metadata,
		MetadataCompressedSize: compressedSize,
		SHA256:                 digest,
	}, nil
}
