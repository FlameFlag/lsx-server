package normalizer

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"os"
)

const (
	generatedPayloadTEASeed = uint32(0x1019B2C9)
	generatedPayloadAuxSeed = uint32(0x04E27787)
)

type staticPayloadMode string

const (
	staticPayloadModeCanonical staticPayloadMode = "canonical"
	staticPayloadModePortable  staticPayloadMode = "portable"
	staticPayloadModeStrict    staticPayloadMode = "strict"
)

func parseStaticPayloadMode(mode string) (staticPayloadMode, error) {
	switch staticPayloadMode(mode) {
	case "", staticPayloadModeCanonical:
		return staticPayloadModeCanonical, nil
	case staticPayloadModePortable:
		return staticPayloadModePortable, nil
	case staticPayloadModeStrict:
		return staticPayloadModeStrict, nil
	default:
		return "", fmt.Errorf("invalid static mode %q; expected canonical, portable, or strict", mode)
	}
}

type staticPayloadInfo struct {
	OutputPath      string
	Size            int
	Seed            uint32
	Mode            staticPayloadMode
	SHA256          string
	Sections        []staticPayloadSection
	PortablePatches portablePatchSummary
}

type portablePatchSummary struct {
	SkippedRDataRanges        int
	SkippedRDataBytes         int
	AppliedDataDwords         int
	AppliedDataBytes          int
	SkippedArtifactDataDwords int
	SkippedArtifactDataBytes  int
}

type staticPayloadSection struct {
	Name         string
	RVA          uint32
	InflatedSize int
	PaddedSize   int
	SHA256       string
}

func deriveStaticPayload(packedPath, outputPath string, mode staticPayloadMode) (staticPayloadInfo, error) {
	stream, streamInfo, err := buildMapperStream(packedPath)
	if err != nil {
		return staticPayloadInfo{}, err
	}

	sections := []struct {
		name       string
		rva        uint32
		paddedSize int
	}{
		{name: ".text", rva: 0x1000, paddedSize: payloadTextSize},
		{name: ".rdata", rva: 0x92000, paddedSize: payloadRDataSize},
		{name: ".data", rva: 0x9C000, paddedSize: payloadDataSize},
	}

	entryByRVA := make(map[uint32]mapperPayloadEntry, len(streamInfo.PayloadEntries))
	for _, entry := range streamInfo.PayloadEntries {
		entryByRVA[entry.RVA] = entry
	}

	payload := make([]byte, 0, payloadCombinedSize)
	sectionInfos := make([]staticPayloadSection, 0, len(sections))
	for _, section := range sections {
		entry, ok := entryByRVA[section.rva]
		if !ok {
			return staticPayloadInfo{}, fmt.Errorf("mapper payload entry for %s rva 0x%X not found", section.name, section.rva)
		}
		inflated, err := inflateStaticPayloadEntry(stream, entry, generatedPayloadTEASeed, generatedPayloadAuxSeed)
		if err != nil {
			return staticPayloadInfo{}, fmt.Errorf("inflate %s payload entry: %w", section.name, err)
		}
		if len(inflated) > section.paddedSize {
			return staticPayloadInfo{}, fmt.Errorf("%s inflated size 0x%X exceeds padded raw size 0x%X", section.name, len(inflated), section.paddedSize)
		}
		padded := make([]byte, section.paddedSize)
		copy(padded, inflated)
		payload = append(payload, padded...)
		sectionInfos = append(sectionInfos, staticPayloadSection{
			Name:         section.name,
			RVA:          entry.RVA,
			InflatedSize: len(inflated),
			PaddedSize:   len(padded),
			SHA256:       sha256Hex(padded),
		})
	}
	var portablePatches portablePatchSummary
	switch mode {
	case staticPayloadModeCanonical:
		if err := canonicalizeStaticPayload(payload); err != nil {
			return staticPayloadInfo{}, err
		}
	case staticPayloadModePortable:
		summary, err := portableCanonicalizeStaticPayload(payload)
		if err != nil {
			return staticPayloadInfo{}, err
		}
		portablePatches = summary
	}
	for index := range sectionInfos {
		start := 0
		switch sectionInfos[index].Name {
		case ".text":
			start = 0
		case ".rdata":
			start = payloadTextSize
		case ".data":
			start = payloadTextSize + payloadRDataSize
		}
		sectionInfos[index].SHA256 = sha256Hex(payload[start : start+sectionInfos[index].PaddedSize])
	}

	if err := os.WriteFile(outputPath, payload, 0o644); err != nil {
		return staticPayloadInfo{}, err
	}
	return staticPayloadInfo{
		OutputPath:      outputPath,
		Size:            len(payload),
		Seed:            generatedPayloadTEASeed,
		Mode:            mode,
		SHA256:          sha256Hex(payload),
		Sections:        sectionInfos,
		PortablePatches: portablePatches,
	}, nil
}

func canonicalizeStaticPayload(payload []byte) error {
	if len(payload) != payloadCombinedSize {
		return fmt.Errorf("static payload has size 0x%X, expected 0x%X", len(payload), payloadCombinedSize)
	}
	if err := applyStaticPayloadPatches(payload[payloadTextSize:payloadTextSize+payloadRDataSize], staticPayloadCanonicalRDataPatches); err != nil {
		return fmt.Errorf("canonicalize .rdata: %w", err)
	}
	if err := applyStaticPayloadPatches(payload[payloadTextSize+payloadRDataSize:], staticPayloadCanonicalDataPatches); err != nil {
		return fmt.Errorf("canonicalize .data: %w", err)
	}
	return nil
}

func portableCanonicalizeStaticPayload(payload []byte) (portablePatchSummary, error) {
	if len(payload) != payloadCombinedSize {
		return portablePatchSummary{}, fmt.Errorf("static payload has size 0x%X, expected 0x%X", len(payload), payloadCombinedSize)
	}
	summary, err := summarizeSkippedRDataPatches()
	if err != nil {
		return portablePatchSummary{}, fmt.Errorf("portable summarize .rdata: %w", err)
	}
	dataSummary, err := applyPortableDataPatches(payload[payloadTextSize+payloadRDataSize:])
	if err != nil {
		return portablePatchSummary{}, fmt.Errorf("portable canonicalize .data: %w", err)
	}
	summary.AppliedDataDwords = dataSummary.AppliedDwords
	summary.AppliedDataBytes = dataSummary.AppliedBytes
	summary.SkippedArtifactDataDwords = dataSummary.SkippedArtifactDwords
	summary.SkippedArtifactDataBytes = dataSummary.SkippedArtifactBytes
	return summary, nil
}

func inflateStaticPayloadEntry(stream []byte, entry mapperPayloadEntry, seed uint32, auxSeed uint32) ([]byte, error) {
	var out bytes.Buffer
	aux := generatedPayloadAuxTable(auxSeed)
	for _, chunk := range entry.Chunks {
		if chunk.SourceLen == 0 {
			break
		}
		if chunk.SourceOff+chunk.SourceLen < chunk.SourceOff || chunk.SourceOff+chunk.SourceLen > len(stream) {
			return nil, fmt.Errorf("payload chunk body 0x%X..0x%X exceeds stream size 0x%X", chunk.SourceOff, chunk.SourceOff+chunk.SourceLen, len(stream))
		}
		data := append([]byte(nil), stream[chunk.SourceOff:chunk.SourceOff+chunk.SourceLen]...)
		generatedAuxXOR(data, aux)
		generatedTEARegion(data, seed, 0)
		inflated, err := zlibInflate(data)
		if err != nil {
			return nil, err
		}
		out.Write(inflated)
	}
	return out.Bytes(), nil
}

func generatedPayloadAuxTable(seed uint32) []byte {
	aux := make([]byte, 0x1004)
	aux[0] = 0x20
	for offset := 0; offset < 0x1000; offset++ {
		aux[2+offset] = byte(generatedPRNGNext(&seed))
	}
	aux[0x1002] = byte(generatedPRNGNext(&seed))
	return aux
}

func generatedAuxXOR(data []byte, aux []byte) {
	stateByte := uint32(aux[0x1002])
	indexA := 0
	indexB := 0
	for offset := range data {
		indexA += int(stateByte)
		nextIndexB := indexB
		if indexA > 0xFFF {
			nextIndexB = indexB + 1
			if nextIndexB > 0xFFF {
				nextIndexB = indexB - 0xFFF
			}
			indexA -= 0x1000
		}
		data[offset] ^= aux[2+indexA] ^ aux[2+nextIndexB]
		stateByte = uint32(data[offset])
		if stateByte == 0 {
			stateByte = 0x100
		}
		indexB = nextIndexB
	}
}

func zlibInflate(data []byte) (out []byte, err error) {
	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := reader.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	return io.ReadAll(reader)
}
