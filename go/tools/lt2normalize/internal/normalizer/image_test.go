package normalizer

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestApplyBaseRelocationsHighLow(t *testing.T) {
	image := make([]byte, 0x3000)
	binary.LittleEndian.PutUint32(image[0x1010:0x1014], 0x10002000)
	binary.LittleEndian.PutUint32(image[0x2000:0x2004], 0x10001010)

	// Relocation block at RVA 0x2000: one HIGHLOW fixup for page 0x1000 + offset 0x10.
	binary.LittleEndian.PutUint32(image[0x2000:0x2004], 0x1000)
	binary.LittleEndian.PutUint32(image[0x2004:0x2008], 10)
	binary.LittleEndian.PutUint16(image[0x2008:0x200A], 0x3010)

	count, err := applyBaseRelocations(image, 0x10000000, 0x01190000, 0x2000, 10)
	if err != nil {
		t.Fatalf("applyBaseRelocations returned error: %v", err)
	}
	if count != 1 {
		t.Fatalf("relocation count = %d, want 1", count)
	}
	if got, want := binary.LittleEndian.Uint32(image[0x1010:0x1014]), uint32(0x01192000); got != want {
		t.Fatalf("relocated value = 0x%08X, want 0x%08X", got, want)
	}
}

func TestApplyBaseRelocationsRejectsBadRange(t *testing.T) {
	_, err := applyBaseRelocations(make([]byte, 0x100), 0x10000000, 0x01190000, 0x80, 0x100)
	if err == nil {
		t.Fatal("expected relocation range error")
	}
}

func TestGeneratedTEAMode1RoundTrip(t *testing.T) {
	data := []byte("12345678ABCDEFGH")
	original := append([]byte(nil), data...)

	generatedTEAMode1(data, 0xF1C62847)
	if string(data) == string(original) {
		t.Fatal("expected TEA mode1 to transform data")
	}
	inverseGeneratedTEAMode1(data, 0xF1C62847)
	if string(data) != string(original) {
		t.Fatalf("inverse TEA mode1 did not restore original: % X", data)
	}
}

func TestGeneratedPRNGDword(t *testing.T) {
	state := uint32(0x12345678)
	if got, want := generatedPRNGNext(&state), uint32(0xD3); got != want {
		t.Fatalf("first PRNG byte = 0x%X, want 0x%X", got, want)
	}
	if state != 0x04EBFD19 {
		t.Fatalf("state after first PRNG byte = 0x%08X, want 0x04EBFD19", state)
	}
	state = 0x12345678
	if got, want := generatedPRNGDword(&state), uint32(0xD3DE4D4F); got != want {
		t.Fatalf("PRNG dword = 0x%08X, want 0x%08X", got, want)
	}
}

func TestRecoverGeneratedPRNGSeed(t *testing.T) {
	seed, ok := recoverGeneratedPRNGSeed([]byte{0xD2, 0xF1, 0x98, 0x15, 0xEA, 0x4C, 0x84, 0x17})
	if !ok {
		t.Fatal("recoverGeneratedPRNGSeed failed")
	}
	if seed != mapperWindowExpectedSeed {
		t.Fatalf("recovered PRNG seed = 0x%08X, want 0x%08X", seed, mapperWindowExpectedSeed)
	}
}

func TestGeneratedSignedPRNGDword(t *testing.T) {
	state := uint32(0x7F885997) // (0x4030F656 ^ 0xCCF0580A) * 10 + 1, with 32-bit wrap.
	if got, want := generatedPRNGDwordSigned(&state), uint32(0x6230E039); got != want {
		t.Fatalf("signed PRNG dword = 0x%08X, want 0x%08X", got, want)
	}
	if state != 0x0156B8D3 {
		t.Fatalf("signed PRNG state = 0x%08X, want 0x0156B8D3", state)
	}
}

func TestDeriveValidationKeyKnownSeed(t *testing.T) {
	key, tag, repeatCount, shift, mutationCount, digest := deriveValidationKey(0xCCF0580A, 0x4030F656, 0xCAB1D923, false)
	if tag != validationExpectedEntryTag {
		t.Fatalf("validation tag = 0x%02X, want 0x%02X", tag, validationExpectedEntryTag)
	}
	if repeatCount != 1161 || shift != 10 || mutationCount != 27819 {
		t.Fatalf("repeat/shift/mutations = %d/%d/%d", repeatCount, shift, mutationCount)
	}
	if got, want := key, ([4]uint32{0xF37A12B4, 0x98CD3A79, 0xEE7D9E58, 0x368A604B}); got != want {
		t.Fatalf("validation key = %08X, want %08X", got, want)
	}
	if got, want := digest, ([16]byte{0xB4, 0x12, 0x7A, 0xF3, 0x79, 0x3A, 0xCD, 0x98, 0x58, 0x9E, 0x7D, 0xEE, 0x4B, 0x60, 0x8A, 0x36}); got != want {
		t.Fatalf("validation md5 = % X, want % X", got, want)
	}
}

func TestGeneratedCRC32AndPRNGXOR(t *testing.T) {
	if got, want := generatedCRC32([]byte("123456789"), 0xFFFFFFFF), uint32(0x340BC6D9); got != want {
		t.Fatalf("generated CRC32 = 0x%08X, want 0x%08X", got, want)
	}

	data := make([]byte, 8)
	binary.LittleEndian.PutUint32(data[0:4], 0x11223344)
	binary.LittleEndian.PutUint32(data[4:8], 0x55667788)
	decoded := append([]byte(nil), data...)
	if count := generatedXORDwordsWithPRNG(decoded, 0x12345678); count != 2 {
		t.Fatalf("xor count = %d, want 2", count)
	}
	if string(decoded) == string(data) {
		t.Fatal("expected PRNG xor to transform data")
	}
	generatedXORDwordsWithPRNG(decoded, 0x12345678)
	if string(decoded) != string(data) {
		t.Fatalf("second PRNG xor did not restore data: % X", decoded)
	}
}

func TestGeneratedRLEDecompress(t *testing.T) {
	stored, err := generatedRLEDecompress([]byte{0, 'p', 'l', 'a', 'i', 'n'}, 8)
	if err != nil {
		t.Fatalf("stored generatedRLEDecompress returned error: %v", err)
	}
	if string(stored) != "plain" {
		t.Fatalf("stored output = %q, want plain", stored)
	}

	encoded := []byte{1, 'A', 0xFF, 0x02, 'B', 0xFF, 0xFF, 'C'}
	decoded, err := generatedRLEDecompress(encoded, 16)
	if err != nil {
		t.Fatalf("RLE generatedRLEDecompress returned error: %v", err)
	}
	if string(decoded) != "AAAAAAB\xffC" {
		t.Fatalf("RLE output = %q", decoded)
	}
}

func TestGeneratedRLEDecompressRejectsOverflow(t *testing.T) {
	_, err := generatedRLEDecompress([]byte{1, 'A', 0xFF, 0x02}, 6)
	if err == nil {
		t.Fatal("expected generated RLE overflow error")
	}
}

func TestReadGeneratedChunk(t *testing.T) {
	const key = uint32(0x1DC06EA8)
	stream := make([]byte, 0, 32)
	header := make([]byte, 4)
	binary.LittleEndian.PutUint32(header, (uint32(3)|0x80000000)^key)
	stream = append(stream, header...)
	stream = append(stream, 'r', 'a', 'w')
	binary.LittleEndian.PutUint32(header, uint32(4)^key)
	stream = append(stream, header...)
	stream = append(stream, 0, 'x', 'y', 'z')

	first, err := readGeneratedChunk(stream, 0, key, 8)
	if err != nil {
		t.Fatalf("read stored chunk: %v", err)
	}
	if !first.Stored || string(first.Data) != "raw" || first.NextOffset != 7 {
		t.Fatalf("unexpected stored chunk: %+v %q", first, first.Data)
	}
	second, err := readGeneratedChunk(stream, first.NextOffset, key, 8)
	if err != nil {
		t.Fatalf("read compressed chunk: %v", err)
	}
	if second.Stored || string(second.Data) != "xyz" || second.NextOffset != len(stream) {
		t.Fatalf("unexpected compressed chunk: %+v %q", second, second.Data)
	}
}

func TestFindBestData1Selector(t *testing.T) {
	stream := make([]byte, 0, 64)
	header := make([]byte, 4)
	binary.LittleEndian.PutUint32(header, 1)
	stream = append(stream, header...)
	stream = append(stream, 0, 0)
	binary.LittleEndian.PutUint32(header, 0x1234)
	stream = append(stream, header...)
	binary.LittleEndian.PutUint32(header, 3|0x80000000)
	stream = append(stream, header...)
	stream = append(stream, 'a', 'b', 'c')
	binary.LittleEndian.PutUint32(header, 0)
	stream = append(stream, header...)

	candidate, err := findBestData1Selector(stream)
	if err != nil {
		t.Fatalf("findBestData1Selector returned error: %v", err)
	}
	if candidate.Offset != 0 || candidate.Count != 1 || string(candidate.Payload) != "abc" {
		t.Fatalf("unexpected selector candidate: %+v payload %q", candidate, candidate.Payload)
	}
}

func TestMapperStreamPostMetadataOffset(t *testing.T) {
	packedPath := defaultPackedPath
	if _, err := os.Stat(packedPath); err != nil {
		packedPath = filepath.Join("..", "..", defaultPackedPath)
	}
	if _, err := os.Stat(packedPath); err != nil {
		t.Skipf("packed Lemonade2.exe is not available: %v", err)
	}
	info, err := deriveMapperStream(packedPath, t.TempDir()+"/mapper_stream.bin")
	if err != nil {
		t.Fatalf("deriveMapperStream returned error: %v", err)
	}
	if info.PostMetadataOffset != 0xC8B {
		t.Fatalf("post-metadata offset = 0x%X, want 0xC8B", info.PostMetadataOffset)
	}
	if info.InitialDecodeLimit != 0xC8B || info.InitialDecodeSourceSize != 0xC7B || info.InitialDecodeHeaderOffset != 0x8 || info.InitialDecodeTerminatorOff != 0xC87 {
		t.Fatalf("initial mapper decode = limit 0x%X source 0x%X header 0x%X terminator 0x%X, want 0xC8B/0xC7B/0x8/0xC87",
			info.InitialDecodeLimit, info.InitialDecodeSourceSize, info.InitialDecodeHeaderOffset, info.InitialDecodeTerminatorOff)
	}
	if info.LoaderSelectedVA != 0x0052B52E || info.LoaderSectionVA != 0x00503000 || info.LoaderRawWindowSize != 0x487BA || info.LoaderEffectiveWindowSize != 0x2800 {
		t.Fatalf("loader window = selected 0x%08X section 0x%08X raw/effective 0x%X/0x%X, want 0x0052B52E/0x00503000/0x487BA/0x2800",
			info.LoaderSelectedVA, info.LoaderSectionVA, info.LoaderRawWindowSize, info.LoaderEffectiveWindowSize)
	}
	if info.LoaderSeed != 0x031E1692 || info.LoaderCRCMix != 0xF32EC9A2 {
		t.Fatalf("loader seed/crc-mix = 0x%08X/0x%08X, want 0x031E1692/0xF32EC9A2", info.LoaderSeed, info.LoaderCRCMix)
	}
	if info.LoaderOriginalTextRVA != 0x1000 || info.LoaderOriginalTextSize != 0x91000 || info.LoaderOriginalTextPages != 0x91 {
		t.Fatalf("loader text mapping = rva 0x%X size 0x%X pages 0x%X, want 0x1000/0x91000/0x91",
			info.LoaderOriginalTextRVA, info.LoaderOriginalTextSize, info.LoaderOriginalTextPages)
	}
	if info.LoaderOptionalCRCMask != 0 || info.LoaderChunkKey != 0x1DC06EA8 || info.LoaderChunkLimitKey != 0 {
		t.Fatalf("loader late keys = optional 0x%08X chunk 0x%08X limit 0x%08X, want 0/0x1DC06EA8/0",
			info.LoaderOptionalCRCMask, info.LoaderChunkKey, info.LoaderChunkLimitKey)
	}
	if info.PayloadKey != 0x64294F12 || info.PayloadTerminatorOffset != 0x472CB || len(info.PayloadEntries) != 3 {
		t.Fatalf("payload table = key 0x%08X entries %d terminator 0x%X, want 0x64294F12/3/0x472CB",
			info.PayloadKey, len(info.PayloadEntries), info.PayloadTerminatorOffset)
	}
	wantEntries := []struct {
		rva       uint32
		size      uint32
		headerOff int
		sourceLen int
	}{
		{0x1000, 0x91000, 0xC8B, 0x43B27},
		{0x92000, 0xA000, 0x447C2, 0x1E16},
		{0x9C000, 0x4000, 0x465E8, 0xCD3},
	}
	for index, want := range wantEntries {
		got := info.PayloadEntries[index]
		if got.RVA != want.rva || got.Size != want.size || got.HeaderOff != want.headerOff || got.SourceLen != want.sourceLen || len(got.Chunks) != 2 {
			t.Fatalf("payload entry %d = rva 0x%X size 0x%X header 0x%X source 0x%X chunks %d, want 0x%X/0x%X/0x%X/0x%X/2",
				index, got.RVA, got.Size, got.HeaderOff, got.SourceLen, len(got.Chunks), want.rva, want.size, want.headerOff, want.sourceLen)
		}
	}
	if info.Size != 0x48AD2 || info.SHA256 != "23d99952e520be0ffb86227abc437b6af50176f670bf1cd25c7d97619b75f4f4" {
		t.Fatalf("mapper stream size/hash = 0x%X/%s", info.Size, info.SHA256)
	}
}

func TestMapperMetadataHeaderFields(t *testing.T) {
	packedPath := defaultPackedPath
	if _, err := os.Stat(packedPath); err != nil {
		packedPath = filepath.Join("..", "..", defaultPackedPath)
	}
	if _, err := os.Stat(packedPath); err != nil {
		t.Skipf("packed Lemonade2.exe is not available: %v", err)
	}
	window, err := deriveMapperWindowBytes(packedPath)
	if err != nil {
		t.Fatalf("deriveMapperWindowBytes returned error: %v", err)
	}
	wantKeys := [4]uint32{0, 0, 3, 2}
	if window.Metadata.Flags != 0xA012 || window.Metadata.KeyDwords != wantKeys || window.Metadata.Checksum != 0x7A192EDC {
		t.Fatalf("metadata header = flags 0x%08X keys %08X %08X %08X %08X checksum 0x%08X, want 0xA012/[0 0 3 2]/0x7A192EDC",
			window.Metadata.Flags,
			window.Metadata.KeyDwords[0], window.Metadata.KeyDwords[1], window.Metadata.KeyDwords[2], window.Metadata.KeyDwords[3], window.Metadata.Checksum)
	}
}

func TestDeriveStaticPayload(t *testing.T) {
	packedPath := defaultPackedPath
	if _, err := os.Stat(packedPath); err != nil {
		packedPath = filepath.Join("..", "..", defaultPackedPath)
	}
	if _, err := os.Stat(packedPath); err != nil {
		t.Skipf("packed Lemonade2.exe is not available: %v", err)
	}
	info, err := deriveStaticPayload(packedPath, filepath.Join(t.TempDir(), "static_payload.bin"), staticPayloadModeCanonical)
	if err != nil {
		t.Fatalf("deriveStaticPayload returned error: %v", err)
	}
	if info.Mode != staticPayloadModeCanonical || info.Size != payloadCombinedSize || info.SHA256 != payloadCombinedSHA256 {
		t.Fatalf("static payload size/hash = 0x%X/%s, want 0x%X/%s", info.Size, info.SHA256, payloadCombinedSize, payloadCombinedSHA256)
	}
}

func TestDeriveStaticPayloadPortableDiffersFromCanonical(t *testing.T) {
	packedPath := defaultPackedPath
	if _, err := os.Stat(packedPath); err != nil {
		packedPath = filepath.Join("..", "..", defaultPackedPath)
	}
	if _, err := os.Stat(packedPath); err != nil {
		t.Skipf("packed Lemonade2.exe is not available: %v", err)
	}
	info, err := deriveStaticPayload(packedPath, filepath.Join(t.TempDir(), "static_payload_portable.bin"), staticPayloadModePortable)
	if err != nil {
		t.Fatalf("deriveStaticPayload portable returned error: %v", err)
	}
	if info.Mode != staticPayloadModePortable || info.Size != payloadCombinedSize {
		t.Fatalf("portable payload mode/size = %s/0x%X, want %s/0x%X", info.Mode, info.Size, staticPayloadModePortable, payloadCombinedSize)
	}
	if info.SHA256 == payloadCombinedSHA256 {
		t.Fatal("portable payload unexpectedly matched canonical dump-artifact hash")
	}
	want := portablePatchSummary{
		SkippedRDataRanges:        26,
		SkippedRDataBytes:         674,
		AppliedDataDwords:         369,
		AppliedDataBytes:          879,
		SkippedArtifactDataDwords: 111,
		SkippedArtifactDataBytes:  426,
	}
	if info.PortablePatches != want {
		t.Fatalf("portable patch summary = %+v, want %+v", info.PortablePatches, want)
	}
}

func TestDeriveStaticPayloadStrictHasNoPatchPolicy(t *testing.T) {
	packedPath := defaultPackedPath
	if _, err := os.Stat(packedPath); err != nil {
		packedPath = filepath.Join("..", "..", defaultPackedPath)
	}
	if _, err := os.Stat(packedPath); err != nil {
		t.Skipf("packed Lemonade2.exe is not available: %v", err)
	}
	info, err := deriveStaticPayload(packedPath, filepath.Join(t.TempDir(), "static_payload_strict.bin"), staticPayloadModeStrict)
	if err != nil {
		t.Fatalf("deriveStaticPayload strict returned error: %v", err)
	}
	if info.Mode != staticPayloadModeStrict || info.Size != payloadCombinedSize {
		t.Fatalf("strict payload mode/size = %s/0x%X, want %s/0x%X", info.Mode, info.Size, staticPayloadModeStrict, payloadCombinedSize)
	}
	if info.SHA256 == payloadCombinedSHA256 {
		t.Fatal("strict payload unexpectedly matched canonical dump-artifact hash")
	}
	if info.PortablePatches != (portablePatchSummary{}) {
		t.Fatalf("strict payload patch summary = %+v, want zero", info.PortablePatches)
	}
}

func TestRebuildCleanPEStripsProtectorSections(t *testing.T) {
	packedPath := defaultPackedPath
	if _, err := os.Stat(packedPath); err != nil {
		packedPath = filepath.Join("..", "..", defaultPackedPath)
	}
	if _, err := os.Stat(packedPath); err != nil {
		t.Skipf("packed Lemonade2.exe is not available: %v", err)
	}
	tmp := t.TempDir()
	payloadPath := filepath.Join(tmp, "strict_payload.bin")
	if _, err := deriveStaticPayload(packedPath, payloadPath, staticPayloadModeStrict); err != nil {
		t.Fatalf("deriveStaticPayload strict returned error: %v", err)
	}
	cleanPath := filepath.Join(tmp, "Lemonade2.clean.exe")
	info, err := rebuildCleanPE(packedPath, payloadPath, cleanPath, cleanDefaultEntryRVA)
	if err != nil {
		t.Fatalf("rebuildCleanPE returned error: %v", err)
	}
	if info.EntryRVA != cleanDefaultEntryRVA || len(info.Sections) != 5 {
		t.Fatalf("clean info entry/section count = 0x%X/%d, want 0x%X/5", info.EntryRVA, len(info.Sections), cleanDefaultEntryRVA)
	}
	peInfo, err := inspectPE(cleanPath)
	if err != nil {
		t.Fatalf("inspect clean PE: %v", err)
	}
	wantNames := []string{".text", ".rdata", ".data", ".rsrc", ".idata"}
	if len(peInfo.Sections) != len(wantNames) {
		t.Fatalf("clean PE sections = %d, want %d", len(peInfo.Sections), len(wantNames))
	}
	for index, want := range wantNames {
		if peInfo.Sections[index].Name != want {
			t.Fatalf("clean PE section %d = %s, want %s", index, peInfo.Sections[index].Name, want)
		}
	}
	if peInfo.EntryVA != originalImageBase+uint64(cleanDefaultEntryRVA) {
		t.Fatalf("clean PE entry VA = 0x%X, want 0x%X", peInfo.EntryVA, originalImageBase+uint64(cleanDefaultEntryRVA))
	}
	cleanBytes, err := os.ReadFile(cleanPath)
	if err != nil {
		t.Fatalf("read clean PE: %v", err)
	}
	peOffset := int(binary.LittleEndian.Uint32(cleanBytes[0x3C:0x40]))
	dataDir := peOffset + 24 + 96
	importRVA := binary.LittleEndian.Uint32(cleanBytes[dataDir+8 : dataDir+12])
	importSize := binary.LittleEndian.Uint32(cleanBytes[dataDir+12 : dataDir+16])
	iatRVA := binary.LittleEndian.Uint32(cleanBytes[dataDir+12*8 : dataDir+12*8+4])
	iatSize := binary.LittleEndian.Uint32(cleanBytes[dataDir+12*8+4 : dataDir+12*8+8])
	if importRVA == 0 || importSize == 0 {
		t.Fatal("clean PE import directory was not rebuilt")
	}
	if iatRVA != cleanIATRVA || iatSize != cleanIATSize {
		t.Fatalf("clean PE IAT directory = 0x%X/0x%X, want 0x%X/0x%X", iatRVA, iatSize, cleanIATRVA, cleanIATSize)
	}
	resourceRVA := binary.LittleEndian.Uint32(cleanBytes[dataDir+2*8 : dataDir+2*8+4])
	resourceSize := binary.LittleEndian.Uint32(cleanBytes[dataDir+2*8+4 : dataDir+2*8+8])
	if resourceRVA != cleanRsrcRVA || resourceSize == 0 {
		t.Fatalf("clean PE resource directory = 0x%X/0x%X, want RVA 0x%X", resourceRVA, resourceSize, cleanRsrcRVA)
	}
	resourceDataRVAs, resourceTypes, err := resourceDataRVAsAndTypes(cleanBytes, resourceRVA)
	if err != nil {
		t.Fatalf("inspect clean resources: %v", err)
	}
	if len(resourceDataRVAs) == 0 {
		t.Fatal("clean PE has no resource data entries")
	}
	for _, dataRVA := range resourceDataRVAs {
		if dataRVA < resourceRVA || dataRVA >= resourceRVA+resourceSize {
			t.Fatalf("resource data RVA 0x%X is outside relocated .rsrc range 0x%X..0x%X", dataRVA, resourceRVA, resourceRVA+resourceSize)
		}
	}
	if !resourceTypes[3] || !resourceTypes[14] {
		t.Fatalf("clean PE icon resource types missing: RT_ICON=%v RT_GROUP_ICON=%v", resourceTypes[3], resourceTypes[14])
	}
	assertTextBytes(t, cleanBytes, 0x0045749C, []byte{0x90, 0x90, 0x90, 0x90, 0x90, 0x90})
	assertTextBytes(t, cleanBytes, 0x00457C6B, []byte{0xE9, 0x00, 0x6D, 0xFD, 0xFF, 0x90, 0x90, 0x90})
	assertTextBytes(t, cleanBytes, 0x0042E970, []byte{0xC6, 0x05, 0x18, 0x07, 0x4A, 0x00, 0x01, 0x5F, 0x5E, 0x5D, 0x5B, 0x83, 0xC4, 0x2C, 0xC3})
	assertCleanVABytes(t, cleanBytes, 0x004A0718, []byte{0x00})
}

func assertTextBytes(t *testing.T, peBytes []byte, va uint32, want []byte) {
	t.Helper()
	offset := 0x1000 + int(uint64(va)-originalImageBase-uint64(cleanTextRVA))
	assertBytesAtOffset(t, peBytes, offset, va, want)
}

func assertCleanVABytes(t *testing.T, peBytes []byte, va uint32, want []byte) {
	t.Helper()
	rva := uint64(va) - originalImageBase
	var offset int
	switch {
	case rva >= uint64(cleanTextRVA) && rva < uint64(cleanTextRVA+payloadTextSize):
		offset = 0x1000 + int(rva-uint64(cleanTextRVA))
	case rva >= uint64(cleanRDataRVA) && rva < uint64(cleanRDataRVA+payloadRDataSize):
		offset = 0x92000 + int(rva-uint64(cleanRDataRVA))
	case rva >= uint64(cleanDataRVA) && rva < uint64(cleanDataRVA+payloadDataSize):
		offset = 0x9C000 + int(rva-uint64(cleanDataRVA))
	default:
		t.Fatalf("VA 0x%X is outside clean payload sections", va)
	}
	assertBytesAtOffset(t, peBytes, offset, va, want)
}

func assertBytesAtOffset(t *testing.T, peBytes []byte, offset int, va uint32, want []byte) {
	t.Helper()
	if offset < 0 || offset+len(want) > len(peBytes) {
		t.Fatalf("VA 0x%X maps outside clean PE", va)
	}
	if got := peBytes[offset : offset+len(want)]; !bytes.Equal(got, want) {
		t.Fatalf("bytes at VA 0x%X = % X, want % X", va, got, want)
	}
}

func resourceDataRVAsAndTypes(peBytes []byte, resourceRVA uint32) ([]uint32, map[uint16]bool, error) {
	peOffset := int(binary.LittleEndian.Uint32(peBytes[0x3C:0x40]))
	optionalSize := int(binary.LittleEndian.Uint16(peBytes[peOffset+20 : peOffset+22]))
	sectionTable := peOffset + 24 + optionalSize
	sectionCount := int(binary.LittleEndian.Uint16(peBytes[peOffset+6 : peOffset+8]))
	var resourceRaw uint32
	var resourceRawSize uint32
	for index := 0; index < sectionCount; index++ {
		header := sectionTable + index*40
		sectionRVA := binary.LittleEndian.Uint32(peBytes[header+12 : header+16])
		if sectionRVA != resourceRVA {
			continue
		}
		resourceRawSize = binary.LittleEndian.Uint32(peBytes[header+16 : header+20])
		resourceRaw = binary.LittleEndian.Uint32(peBytes[header+20 : header+24])
		break
	}
	if resourceRaw == 0 || int(resourceRaw+resourceRawSize) > len(peBytes) {
		return nil, nil, fmt.Errorf("resource section raw range is invalid")
	}
	resource := peBytes[resourceRaw : resourceRaw+resourceRawSize]
	types := map[uint16]bool{}
	var dataRVAs []uint32
	visited := map[uint32]bool{}
	var walk func(uint32) error
	walk = func(dirOffset uint32) error {
		if visited[dirOffset] {
			return nil
		}
		visited[dirOffset] = true
		if dirOffset > uint32(len(resource)) || uint32(len(resource))-dirOffset < 16 {
			return fmt.Errorf("directory offset 0x%X is out of range", dirOffset)
		}
		entryCount := int(binary.LittleEndian.Uint16(resource[dirOffset+12:dirOffset+14])) + int(binary.LittleEndian.Uint16(resource[dirOffset+14:dirOffset+16]))
		entriesOffset := dirOffset + 16
		if entriesOffset > uint32(len(resource)) || uint32(len(resource))-entriesOffset < uint32(entryCount*8) {
			return fmt.Errorf("directory entries at 0x%X exceed section", entriesOffset)
		}
		for index := 0; index < entryCount; index++ {
			entryOffset := entriesOffset + uint32(index*8)
			name := binary.LittleEndian.Uint32(resource[entryOffset : entryOffset+4])
			value := binary.LittleEndian.Uint32(resource[entryOffset+4 : entryOffset+8])
			if dirOffset == 0 && name&0x80000000 == 0 {
				types[uint16(name)] = true
			}
			childOffset := value & 0x7FFFFFFF
			if value&0x80000000 != 0 {
				if err := walk(childOffset); err != nil {
					return err
				}
				continue
			}
			if childOffset > uint32(len(resource)) || uint32(len(resource))-childOffset < 16 {
				return fmt.Errorf("data entry offset 0x%X is out of range", childOffset)
			}
			dataRVAs = append(dataRVAs, binary.LittleEndian.Uint32(resource[childOffset:childOffset+4]))
		}
		return nil
	}
	if err := walk(0); err != nil {
		return nil, nil, err
	}
	return dataRVAs, types, nil
}
