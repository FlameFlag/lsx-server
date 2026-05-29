package normalizer

import (
	"bytes"
	"debug/pe"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	defaultPackedPath = "decompiled/local/lt2_install/Lemonade2.exe"
	defaultOutPath    = "decompiled/local/unpacked/Lemonade2.unpacked.exe"
	defaultReportPath = "docs/reverse-engineering/normalization.md"
	originalImageBase = uint64(0x00400000)
	originalTextVA    = uint64(0x00401000)
)

type config struct {
	packed        string
	unpacked      string
	out           string
	report        string
	pdataOut      string
	adataOut      string
	payload       string
	staticPayload string
	staticNorm    string
	staticMode    string
	cleanNorm     string
	cleanEntryRVA uint
	genOut        string
	mapOut        string
	streamOut     string
	valOut        string
	data1Out      string
	genBase       uint
	genTEA        bool
	write         bool
	check         bool
}

type peInfo struct {
	Path       string
	SHA256     string
	ImageBase  uint64
	EntryVA    uint64
	Sections   []sectionInfo
	StringHits []string
}

type sectionInfo struct {
	Name        string
	VA          uint64
	VirtualSize uint32
	RawOffset   uint32
	RawSize     uint32
	Entropy     float64
}

func Run(args []string, stdout io.Writer) error {
	cfg := config{}
	fs := flag.NewFlagSet("lt2normalize", flag.ContinueOnError)
	fs.StringVar(&cfg.packed, "packed", defaultPackedPath, "focused packed Lemonade2.exe path")
	fs.StringVar(&cfg.unpacked, "unpacked", "", "candidate unpacked/normalized Lemonade2.exe dump")
	fs.StringVar(&cfg.out, "out", defaultOutPath, "canonical normalized output path")
	fs.StringVar(&cfg.report, "report", defaultReportPath, "normalization report path")
	fs.StringVar(&cfg.pdataOut, "extract-pdata", "", "directory where .pdata zlib container entries should be written")
	fs.StringVar(&cfg.adataOut, "decode-adata", "", "write a copy of the packed EXE with the first .adata entry-stub layer decoded")
	fs.StringVar(&cfg.payload, "reconstruct-payload", "", "append a recovered .text/.rdata/.data payload blob and patch section raw ranges")
	fs.StringVar(&cfg.staticPayload, "derive-static-payload", "", "write the .text/.rdata/.data payload derived from the packed mapper stream")
	fs.StringVar(&cfg.staticNorm, "derive-static-normalized", "", "write the normalized EXE from the packed input using the static payload derivation")
	fs.StringVar(&cfg.staticMode, "static-mode", string(staticPayloadModeCanonical), "static output mode: canonical, portable, or strict")
	fs.StringVar(&cfg.cleanNorm, "derive-clean-normalized", "", "write a clean PE candidate with Armadillo carrier sections stripped")
	fs.UintVar(&cfg.cleanEntryRVA, "clean-entry-rva", uint(cleanDefaultEntryRVA), "entry RVA for -derive-clean-normalized candidate")
	fs.StringVar(&cfg.genOut, "derive-generated", "", "write the generated Armadillo PE image recovered from packed .pdata")
	fs.StringVar(&cfg.mapOut, "derive-mapper-window", "", "write the first post-PRNG mapper window recovered from packed .pdata")
	fs.StringVar(&cfg.streamOut, "derive-mapper-stream", "", "write the selected mapper stream with the decoded first window and raw tail continuation")
	fs.StringVar(&cfg.valOut, "derive-validation-entry", "", "write the decrypted mapper validation entry for the known naturally valid ShortV3 seed")
	fs.StringVar(&cfg.data1Out, "derive-data1-selector", "", "write the selected .data1 FUN_10028237 decoded payload")
	fs.UintVar(&cfg.genBase, "generated-base", 0x01190000, "load base used when relocating -derive-generated output")
	fs.BoolVar(&cfg.genTEA, "replay-generated-tea", false, "apply the validated generated PE TEA row table after -derive-generated initialization")
	fs.BoolVar(&cfg.write, "write", false, "copy the candidate unpacked file to the canonical output path")
	fs.BoolVar(&cfg.check, "check", false, "validate the canonical normalized output path")
	fs.Usage = func() {
		_, _ = fmt.Fprintf(fs.Output(), "Usage: %s [flags]\n\n", fs.Name())
		_, _ = fmt.Fprintln(fs.Output(), "Inspects the packed LT2 game EXE and stages a real unpacked dump for Ghidra.")
		_, _ = fmt.Fprintln(fs.Output())
		_, _ = fmt.Fprintln(fs.Output(), "Examples:")
		_, _ = fmt.Fprintln(fs.Output(), "  go -C go run ./tools/lt2normalize")
		_, _ = fmt.Fprintln(fs.Output(), "  go -C go run ./tools/lt2normalize -extract-pdata decompiled/local/pdata_assets")
		_, _ = fmt.Fprintln(fs.Output(), "  go -C go run ./tools/lt2normalize -derive-generated decompiled/local/generated_island_initial.bin")
		_, _ = fmt.Fprintln(fs.Output(), "  go -C go run ./tools/lt2normalize -derive-mapper-window decompiled/local/mapper_window.bin")
		_, _ = fmt.Fprintln(fs.Output(), "  go -C go run ./tools/lt2normalize -derive-mapper-stream decompiled/local/mapper_stream.bin")
		_, _ = fmt.Fprintln(fs.Output(), "  go -C go run ./tools/lt2normalize -derive-validation-entry decompiled/local/mapper_validation_entry.bin")
		_, _ = fmt.Fprintln(fs.Output(), "  go -C go run ./tools/lt2normalize -derive-data1-selector decompiled/local/data1_selector_payload.bin")
		_, _ = fmt.Fprintln(fs.Output(), "  go -C go run ./tools/lt2normalize -derive-static-payload decompiled/local/static_payload.bin")
		_, _ = fmt.Fprintln(fs.Output(), "  go -C go run ./tools/lt2normalize -derive-static-normalized decompiled/local/unpacked/Lemonade2.unpacked.exe")
		_, _ = fmt.Fprintln(fs.Output(), "  go -C go run ./tools/lt2normalize -static-mode portable -derive-static-normalized decompiled/local/unpacked/Lemonade2.portable.exe")
		_, _ = fmt.Fprintln(fs.Output(), "  go -C go run ./tools/lt2normalize -static-mode strict -derive-static-normalized decompiled/local/unpacked/Lemonade2.strict.exe")
		_, _ = fmt.Fprintln(fs.Output(), "  go -C go run ./tools/lt2normalize -static-mode strict -derive-clean-normalized decompiled/local/unpacked/Lemonade2.clean-candidate.exe")
		_, _ = fmt.Fprintln(fs.Output(), "  go -C go run ./tools/lt2normalize -decode-adata decompiled/local/Lemonade2.adata1.exe")
		_, _ = fmt.Fprintln(fs.Output(), "  go -C go run ./tools/lt2normalize -reconstruct-payload decompiled/analysis/lemonade2_static_unpacking/runs/latest/reconstruction/reconstructed_payload.bin -out decompiled/local/unpacked/Lemonade2.unpacked.exe")
		_, _ = fmt.Fprintln(fs.Output(), "  go -C go run ./tools/lt2normalize -unpacked /path/to/Lemonade2.dump.exe -write")
		_, _ = fmt.Fprintln(fs.Output(), "  go -C go run ./tools/lt2normalize -check")
		_, _ = fmt.Fprintln(fs.Output())
		_, _ = fmt.Fprintln(fs.Output(), "Flags:")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	staticMode, err := parseStaticPayloadMode(cfg.staticMode)
	if err != nil {
		return err
	}
	resolveConfigPaths(&cfg)

	packed, err := inspectPE(cfg.packed)
	if err != nil {
		return fmt.Errorf("inspect packed exe: %w", err)
	}

	if cfg.pdataOut != "" {
		assets, err := extractPDATA(cfg.packed, cfg.pdataOut)
		if err != nil {
			return fmt.Errorf("extract .pdata assets: %w", err)
		}
		for _, asset := range assets {
			if asset.Kind == "encrypted_tail" {
				_, _ = fmt.Fprintf(stdout, "wrote %s (%d raw bytes, tail at file offset 0x%X)\n",
					asset.Path, asset.DecompressedSize, asset.FileOffset)
				continue
			}
			if asset.Kind == "recovered_tail_header" || asset.Kind == "recovered_tail" {
				_, _ = fmt.Fprintf(stdout, "wrote %s (%d recovered bytes, tail at file offset 0x%X)\n",
					asset.Path, asset.DecompressedSize, asset.FileOffset)
				continue
			}
			_, _ = fmt.Fprintf(stdout, "wrote %s (%d bytes, stream at file offset 0x%X, compressed %d bytes)\n",
				asset.Path, asset.DecompressedSize, asset.FileOffset, asset.CompressedSize)
		}
	}

	if cfg.adataOut != "" {
		info, err := decodeADATA(cfg.packed, cfg.adataOut)
		if err != nil {
			return fmt.Errorf("decode .adata first stage: %w", err)
		}
		_, _ = fmt.Fprintf(stdout, "wrote %s (.adata first-stage key 0x%02X, marker at +0x%X, payload +0x%X..+0x%X",
			info.Path, info.Key, info.MarkerOffset, info.PayloadOffset, info.PayloadOffset+info.PayloadSize)
		if info.PatchedRetByte {
			_, _ = fmt.Fprintf(stdout, ", patched +0x%X to NOP", adataFirstPatchOffset)
		}
		_, _ = fmt.Fprintln(stdout, ")")
	}

	if cfg.genOut != "" {
		if cfg.genBase > math.MaxUint32 {
			return fmt.Errorf("generated base 0x%X exceeds 32-bit address space", cfg.genBase)
		}
		info, err := deriveGeneratedImage(cfg.packed, cfg.genOut, uint32(cfg.genBase), cfg.genTEA)
		if err != nil {
			return fmt.Errorf("derive generated PE image: %w", err)
		}
		_, _ = fmt.Fprintf(stdout, "wrote generated PE image %s (base 0x%08X, preferred 0x%08X, size 0x%X, relocations %d, static globals %d, static fills %d, bootstrap inversions %d, replayed tea rows %d)\n",
			info.OutputPath, info.LoadBase, info.PreferredBase, info.Size, info.Relocations, info.Mutations.StaticGlobalDwords, info.Mutations.StaticFillDwords, info.Mutations.BootstrapInverseRegions, info.Mutations.ReplayedTEARows)
	}

	if cfg.mapOut != "" {
		info, err := deriveMapperWindow(cfg.packed, cfg.mapOut)
		if err != nil {
			return fmt.Errorf("derive mapper window: %w", err)
		}
		_, _ = fmt.Fprintf(stdout, "wrote mapper window %s (tail offset 0x%X, seed 0x%08X, magic 0x%08X, sha256 %s)\n",
			info.OutputPath, info.TailOffset, info.Seed, info.Magic, info.SHA256)
		_, _ = fmt.Fprintf(stdout, "mapper metadata: size 0x%X, flags 0x%08X, keys [%08X %08X %08X %08X], checksum 0x%08X, product %q, dependencies %d, records %d, data entries %d\n",
			info.Metadata.Size, info.Metadata.Flags,
			info.Metadata.KeyDwords[0], info.Metadata.KeyDwords[1], info.Metadata.KeyDwords[2], info.Metadata.KeyDwords[3], info.Metadata.Checksum,
			info.Metadata.ProductName, len(info.Metadata.Dependencies), len(info.Metadata.Records), len(info.Metadata.DataEntries))
	}

	if cfg.streamOut != "" {
		info, err := deriveMapperStream(cfg.packed, cfg.streamOut)
		if err != nil {
			return fmt.Errorf("derive mapper stream: %w", err)
		}
		_, _ = fmt.Fprintf(stdout, "wrote mapper stream %s (tail offset 0x%X, stream offset 0x%X, size 0x%X, decoded window 0x%X, loader selected VA 0x%08X, loader raw/effective 0x%X/0x%X, seed 0x%08X, crc-mix 0x%08X, text rva/size/pages 0x%X/0x%X/%d, optional-crc mask 0x%08X, chunk-key 0x%08X, limit-key 0x%08X, initial decode limit 0x%X, source 0x%X at 0x%X, post-metadata offset 0x%X, payload key 0x%08X entries %d terminator 0x%X, sha256 %s)\n",
			info.OutputPath, info.TailOffset, info.StreamOffset, info.Size, info.DecodedWindowSize,
			info.LoaderSelectedVA, info.LoaderRawWindowSize, info.LoaderEffectiveWindowSize, info.LoaderSeed, info.LoaderCRCMix,
			info.LoaderOriginalTextRVA, info.LoaderOriginalTextSize, info.LoaderOriginalTextPages, info.LoaderOptionalCRCMask,
			info.LoaderChunkKey, info.LoaderChunkLimitKey,
			info.InitialDecodeLimit, info.InitialDecodeSourceSize, info.InitialDecodeHeaderOffset, info.PostMetadataOffset,
			info.PayloadKey, len(info.PayloadEntries), info.PayloadTerminatorOffset, info.SHA256)
		for index, entry := range info.PayloadEntries {
			_, _ = fmt.Fprintf(stdout, "mapper payload entry %d: rva 0x%X size 0x%X source 0x%X chunks %d header 0x%X\n",
				index, entry.RVA, entry.Size, entry.SourceLen, len(entry.Chunks)-1, entry.HeaderOff)
		}
	}

	if cfg.valOut != "" {
		info, err := deriveMapperValidationEntry(cfg.packed, cfg.valOut, validationDefaultSeed)
		if err != nil {
			return fmt.Errorf("derive mapper validation entry: %w", err)
		}
		_, _ = fmt.Fprintf(stdout, "wrote validation entry %s (seed 0x%08X, tag 0x%02X, entry %d at 0x%X, size 0x%X, repeat %d, shift %d, mutations %d, md5 %s, trailer 0x%08X/0x%08X)\n",
			info.OutputPath, info.Seed, info.Tag, info.EntryIndex, info.EntryOffset, info.EntrySize,
			info.RepeatCount, info.Shift, info.Mutations, info.MD5, info.TrailerA, info.TrailerB)
	}

	if cfg.data1Out != "" {
		info, err := deriveData1SelectorPayload(cfg.packed, cfg.data1Out)
		if err != nil {
			return fmt.Errorf("derive .data1 selector payload: %w", err)
		}
		_, _ = fmt.Fprintf(stdout, "wrote .data1 selector payload %s (offset 0x%X, count %d, selected %d, size 0x%X, sha256 %s)\n",
			info.OutputPath, info.Offset, info.Count, info.Selected, info.PayloadSize, info.SHA256)
	}

	if cfg.staticPayload != "" {
		info, err := deriveStaticPayload(cfg.packed, cfg.staticPayload, staticMode)
		if err != nil {
			return fmt.Errorf("derive static payload: %w", err)
		}
		_, _ = fmt.Fprintf(stdout, "wrote static payload %s (mode %s, size 0x%X, seed 0x%08X, sha256 %s)\n",
			info.OutputPath, info.Mode, info.Size, info.Seed, info.SHA256)
		writePortablePatchSummary(stdout, info)
		for _, section := range info.Sections {
			_, _ = fmt.Fprintf(stdout, "static payload section %s: rva 0x%X inflated 0x%X padded 0x%X sha256 %s\n",
				section.Name, section.RVA, section.InflatedSize, section.PaddedSize, section.SHA256)
		}
	}

	if cfg.payload != "" {
		info, err := reconstructWithPayload(cfg.packed, cfg.payload, cfg.out)
		if err != nil {
			return fmt.Errorf("reconstruct payload: %w", err)
		}
		_, _ = fmt.Fprintf(stdout, "wrote reconstructed normalized target %s (payload sha256 %s)\n", info.OutputPath, info.PayloadSHA)
		for _, section := range info.Sections {
			_, _ = fmt.Fprintf(stdout, "patched %s raw offset 0x%X size 0x%X\n", section.Name, section.RawOffset, section.RawSize)
		}
	}

	var candidate *peInfo
	if cfg.staticNorm != "" {
		tmp, err := os.CreateTemp("", "lt2-static-payload-*.bin")
		if err != nil {
			return err
		}
		tmpPath := tmp.Name()
		_ = tmp.Close()
		defer func() { _ = os.Remove(tmpPath) }()

		payloadInfo, err := deriveStaticPayload(cfg.packed, tmpPath, staticMode)
		if err != nil {
			return fmt.Errorf("derive static payload: %w", err)
		}
		expectedPayloadSHA := payloadCombinedSHA256
		if staticMode != staticPayloadModeCanonical {
			expectedPayloadSHA = ""
		}
		reconstructInfo, err := reconstructWithPayloadHash(cfg.packed, tmpPath, cfg.staticNorm, expectedPayloadSHA)
		if err != nil {
			return fmt.Errorf("reconstruct static normalized EXE: %w", err)
		}
		_, _ = fmt.Fprintf(stdout, "wrote static normalized target %s (mode %s, payload sha256 %s)\n", reconstructInfo.OutputPath, payloadInfo.Mode, payloadInfo.SHA256)
		writePortablePatchSummary(stdout, payloadInfo)
		for _, section := range reconstructInfo.Sections {
			_, _ = fmt.Fprintf(stdout, "patched %s raw offset 0x%X size 0x%X\n", section.Name, section.RawOffset, section.RawSize)
		}
		info, err := inspectPE(reconstructInfo.OutputPath)
		if err != nil {
			return fmt.Errorf("inspect static normalized output: %w", err)
		}
		candidate = &info
	}

	if cfg.cleanNorm != "" {
		if cfg.cleanEntryRVA > math.MaxUint32 {
			return fmt.Errorf("clean entry RVA 0x%X exceeds 32-bit address space", cfg.cleanEntryRVA)
		}
		tmp, err := os.CreateTemp("", "lt2-clean-payload-*.bin")
		if err != nil {
			return err
		}
		tmpPath := tmp.Name()
		_ = tmp.Close()
		defer func() { _ = os.Remove(tmpPath) }()

		payloadInfo, err := deriveStaticPayload(cfg.packed, tmpPath, staticMode)
		if err != nil {
			return fmt.Errorf("derive clean payload: %w", err)
		}
		cleanInfo, err := rebuildCleanPE(cfg.packed, tmpPath, cfg.cleanNorm, uint32(cfg.cleanEntryRVA))
		if err != nil {
			return fmt.Errorf("rebuild clean normalized EXE: %w", err)
		}
		_, _ = fmt.Fprintf(stdout, "wrote clean normalized candidate %s (mode %s, entry rva 0x%X, payload sha256 %s)\n", cleanInfo.OutputPath, payloadInfo.Mode, cleanInfo.EntryRVA, payloadInfo.SHA256)
		writePortablePatchSummary(stdout, payloadInfo)
		for _, section := range cleanInfo.Sections {
			_, _ = fmt.Fprintf(stdout, "clean section %s raw offset 0x%X size 0x%X\n", section.Name, section.RawOffset, section.RawSize)
		}
		for _, warning := range cleanInfo.Warnings {
			_, _ = fmt.Fprintf(stdout, "warning: %s\n", warning)
		}
		info, err := inspectPE(cleanInfo.OutputPath)
		if err != nil {
			return fmt.Errorf("inspect clean normalized output: %w", err)
		}
		candidate = &info
	}

	if cfg.unpacked != "" {
		info, err := inspectPE(cfg.unpacked)
		if err != nil {
			return fmt.Errorf("inspect candidate unpacked exe: %w", err)
		}
		candidate = &info
		if err := validateNormalizedCandidate(packed, info); err != nil {
			return err
		}
		if cfg.write {
			if err := copyFile(cfg.unpacked, cfg.out); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(stdout, "wrote normalized candidate %s\n", cfg.out)
		}
	}

	if cfg.check {
		info, err := inspectPE(cfg.out)
		if err != nil {
			return fmt.Errorf("inspect normalized output: %w", err)
		}
		if err := validateNormalizedCandidate(packed, info); err != nil {
			return err
		}
		candidate = &info
		_, _ = fmt.Fprintf(stdout, "validated normalized target %s\n", cfg.out)
	}

	if err := writeReport(cfg.report, cfg, packed, candidate); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "wrote %s\n", cfg.report)
	return nil
}

func writePortablePatchSummary(stdout io.Writer, info staticPayloadInfo) {
	if info.Mode == staticPayloadModeStrict {
		_, _ = fmt.Fprintln(stdout, "strict patch policy: skipped all canonical/runtime patches; payload bytes are only packed mapper stream -> aux xor -> TEA -> zlib -> zero padding")
		return
	}
	if info.Mode != staticPayloadModePortable {
		return
	}
	summary := info.PortablePatches
	_, _ = fmt.Fprintf(stdout, "portable patch policy: skipped .rdata canonical ranges %d bytes %d; applied .data initializer dwords %d bytes %d; skipped .data artifact dwords %d bytes %d\n",
		summary.SkippedRDataRanges, summary.SkippedRDataBytes,
		summary.AppliedDataDwords, summary.AppliedDataBytes,
		summary.SkippedArtifactDataDwords, summary.SkippedArtifactDataBytes)
}

func resolveConfigPaths(cfg *config) {
	cfg.packed = resolveWorkspacePath(cfg.packed)
	cfg.unpacked = resolveWorkspacePath(cfg.unpacked)
	cfg.out = resolveWorkspacePath(cfg.out)
	cfg.report = resolveWorkspacePath(cfg.report)
	cfg.pdataOut = resolveWorkspacePath(cfg.pdataOut)
	cfg.adataOut = resolveWorkspacePath(cfg.adataOut)
	cfg.payload = resolveWorkspacePath(cfg.payload)
	cfg.staticPayload = resolveWorkspacePath(cfg.staticPayload)
	cfg.staticNorm = resolveWorkspacePath(cfg.staticNorm)
	cfg.cleanNorm = resolveWorkspacePath(cfg.cleanNorm)
	cfg.genOut = resolveWorkspacePath(cfg.genOut)
	cfg.mapOut = resolveWorkspacePath(cfg.mapOut)
	cfg.streamOut = resolveWorkspacePath(cfg.streamOut)
	cfg.valOut = resolveWorkspacePath(cfg.valOut)
	cfg.data1Out = resolveWorkspacePath(cfg.data1Out)
}

func resolveWorkspacePath(path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	if _, err := os.Stat(path); err == nil {
		return path
	}
	cwd, err := os.Getwd()
	if err != nil {
		return path
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, path)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		if _, err := os.Stat(filepath.Dir(candidate)); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return path
}

func inspectPE(path string) (peInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return peInfo{}, err
	}
	file, err := pe.Open(path)
	if err != nil {
		return peInfo{}, err
	}
	defer func() { _ = file.Close() }()

	info := peInfo{Path: path, SHA256: sha256HexUpper(data)}
	switch optional := file.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		info.ImageBase = uint64(optional.ImageBase)
		info.EntryVA = info.ImageBase + uint64(optional.AddressOfEntryPoint)
	case *pe.OptionalHeader64:
		info.ImageBase = optional.ImageBase
		info.EntryVA = info.ImageBase + uint64(optional.AddressOfEntryPoint)
	default:
		return peInfo{}, errors.New("missing PE optional header")
	}

	for _, section := range file.Sections {
		sectionData, _ := section.Data()
		info.Sections = append(info.Sections, sectionInfo{
			Name:        peSectionName(section),
			VA:          info.ImageBase + uint64(section.VirtualAddress),
			VirtualSize: section.VirtualSize,
			RawOffset:   section.Offset,
			RawSize:     section.Size,
			Entropy:     entropy(sectionData),
		})
	}
	info.StringHits = findEvidenceStrings(data)
	return info, nil
}

func validateNormalizedCandidate(packed peInfo, candidate peInfo) error {
	if packed.SHA256 == candidate.SHA256 {
		return fmt.Errorf("%s is byte-identical to the packed source; need a runtime-unpacked dump", candidate.Path)
	}
	if looksPackedLike(candidate) {
		return fmt.Errorf("%s still looks packed/protected; need a dump with restored original code bytes", candidate.Path)
	}
	if !candidate.hasRawBytesForVA(originalTextVA) {
		return fmt.Errorf("%s does not map raw bytes at expected original .text VA 0x%08X", candidate.Path, originalTextVA)
	}
	return nil
}

func looksPackedLike(info peInfo) bool {
	text := info.section(".text")
	rdata := info.section(".rdata")
	data := info.section(".data")
	text1 := info.section(".text1")
	pdata := info.section(".pdata")
	virtualOriginals := text != nil && rdata != nil && data != nil &&
		text.RawSize == 0 && rdata.RawSize == 0 && data.RawSize == 0 &&
		text.VirtualSize != 0 && rdata.VirtualSize != 0 && data.VirtualSize != 0
	highEntropyPayload := (text1 != nil && text1.Entropy > 7.0) ||
		(pdata != nil && pdata.Entropy > 7.0)
	return virtualOriginals && highEntropyPayload
}

func (info peInfo) section(name string) *sectionInfo {
	for i := range info.Sections {
		if info.Sections[i].Name == name {
			return &info.Sections[i]
		}
	}
	return nil
}

func (info peInfo) hasRawBytesForVA(va uint64) bool {
	for _, section := range info.Sections {
		size := uint64(section.RawSize)
		if size == 0 {
			continue
		}
		if va >= section.VA && va < section.VA+size {
			return true
		}
	}
	return false
}

func findEvidenceStrings(data []byte) []string {
	needles := []string{
		"Cannot locate protected program data",
		"ARMDEBUG=",
		"IsDebuggerPresent",
		"SetFunctionAddresses",
		"deflate 1.1.4",
		"inflate 1.1.4",
		"DebugActiveProcess",
		"WriteProcessMemory",
		"GetThreadContext",
		"VirtualProtectEx",
		"CreateProcessW",
	}
	var hits []string
	for _, needle := range needles {
		if bytes.Contains(data, []byte(needle)) {
			hits = append(hits, needle)
		}
	}
	sort.Strings(hits)
	return hits
}

func entropy(data []byte) float64 {
	if len(data) == 0 {
		return 0
	}
	var counts [256]int
	for _, b := range data {
		counts[b]++
	}
	var e float64
	length := float64(len(data))
	for _, count := range counts {
		if count == 0 {
			continue
		}
		p := float64(count) / length
		e -= p * math.Log2(p)
	}
	return e
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	temp := dst + ".tmp"
	out, err := os.OpenFile(temp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(temp, dst)
}

func writeReport(path string, cfg config, packed peInfo, candidate *peInfo) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	outPath := displayWorkspacePath(cfg.out)
	var b strings.Builder
	fmt.Fprintf(&b, "# Lemonade2 Normalization Report\n\n")
	fmt.Fprintf(&b, "Generated by `tools/lt2normalize`.\n\n")
	fmt.Fprintf(&b, "## Verdict\n\n")
	fmt.Fprintf(&b, "`%s` is protected/packed. The exact commercial protector is not proven from these bytes alone, but the evidence strongly matches an Armadillo-style protected executable.\n\n", displayWorkspacePath(packed.Path))
	fmt.Fprintf(&b, "A useful Ghidra target needs the decrypted/decompressed normalized EXE staged at `%s`.\n\n", outPath)
	writePEInfo(&b, "Packed Source", packed)
	if candidate != nil {
		writePEInfo(&b, "Normalized Candidate", *candidate)
	} else {
		fmt.Fprintf(&b, "## Normalized Candidate\n\n")
		fmt.Fprintf(&b, "No candidate was provided. Derive the normalized target directly from the packed input with:\n\n")
		fmt.Fprintf(&b, "```sh\n")
		fmt.Fprintf(&b, "go -C go run ./tools/lt2normalize -derive-static-normalized %s -check\n", outPath)
		fmt.Fprintf(&b, "```\n\n")
	}
	fmt.Fprintf(&b, "## Ghidra Import\n\n")
	fmt.Fprintf(&b, "Once `%s` exists and passes `-check`, import it as the normalized analysis target, or run `decompiled/ghidra_scripts/build_unpacked_project.sh` to do both steps:\n\n", outPath)
	fmt.Fprintf(&b, "```sh\n")
	fmt.Fprintf(&b, "ghidra-analyzeHeadless decompiled/local/ghidra_projects LT2Unpacked \\\n")
	fmt.Fprintf(&b, "  -import %s \\\n", outPath)
	fmt.Fprintf(&b, "  -overwrite \\\n")
	fmt.Fprintf(&b, "  -analysisTimeoutPerFile 600 \\\n")
	fmt.Fprintf(&b, "  -scriptPath decompiled/ghidra_scripts/src/main/java \\\n")
	fmt.Fprintf(&b, "  -preScript PreRepairAntiDisassembly.java \\\n")
	fmt.Fprintf(&b, "  -postScript FullAnalysis.java \\\n")
	fmt.Fprintf(&b, "  -log decompiled/analysis/ghidra_unpacked.log\n")
	fmt.Fprintf(&b, "```\n\n")
	fmt.Fprintf(&b, "The findings INI already has placeholder sections for `Lemonade2.unpacked.exe` at the expected original `.text`, `.rdata`, and `.data` regions. Add concrete function sections there after the dump reveals real code.\n")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func writePEInfo(b *strings.Builder, title string, info peInfo) {
	fmt.Fprintf(b, "## %s\n\n", title)
	fmt.Fprintf(b, "- Path: `%s`\n", displayWorkspacePath(info.Path))
	fmt.Fprintf(b, "- SHA-256: `%s`\n", info.SHA256)
	fmt.Fprintf(b, "- Image base: `0x%08X`\n", info.ImageBase)
	fmt.Fprintf(b, "- Entry VA: `0x%08X`\n", info.EntryVA)
	if len(info.StringHits) > 0 {
		fmt.Fprintf(b, "- Protector/evidence strings: `%s`\n", strings.Join(info.StringHits, "`, `"))
	}
	fmt.Fprintf(b, "\n| Section | VA | Virtual Size | Raw Offset | Raw Size | Entropy |\n")
	fmt.Fprintf(b, "| --- | --- | ---: | ---: | ---: | ---: |\n")
	for _, section := range info.Sections {
		fmt.Fprintf(b, "| `%s` | `0x%08X` | `0x%X` | `0x%X` | `0x%X` | %.3f |\n",
			section.Name, section.VA, section.VirtualSize, section.RawOffset,
			section.RawSize, section.Entropy)
	}
	fmt.Fprintf(b, "\n")
}

func displayWorkspacePath(path string) string {
	if path == "" || !filepath.IsAbs(path) {
		return path
	}
	root := workspaceRoot()
	if root == "" {
		return path
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return path
	}
	return rel
}

func workspaceRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
	}
}
