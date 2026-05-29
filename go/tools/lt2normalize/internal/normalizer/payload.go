package normalizer

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
)

const (
	payloadTextSize       = 0x91000
	payloadRDataSize      = 0x0A000
	payloadDataSize       = 0x07000
	payloadCombinedSize   = payloadTextSize + payloadRDataSize + payloadDataSize
	payloadCombinedSHA256 = "0a14f853214920d91abbb596a369efbb2a3a6ff5bc9e93e8c41500aa5c0d1f7f"
)

type payloadReconstructInfo struct {
	OutputPath string
	PayloadSHA string
	Sections   []payloadSectionWrite
}

type payloadSectionWrite struct {
	Name      string
	RawOffset uint32
	RawSize   uint32
}

func reconstructWithPayload(packedPath, payloadPath, outputPath string) (payloadReconstructInfo, error) {
	return reconstructWithPayloadHash(packedPath, payloadPath, outputPath, payloadCombinedSHA256)
}

func reconstructWithPayloadHash(packedPath, payloadPath, outputPath string, expectedSHA string) (payloadReconstructInfo, error) {
	packed, err := os.ReadFile(packedPath)
	if err != nil {
		return payloadReconstructInfo{}, err
	}
	payload, err := os.ReadFile(payloadPath)
	if err != nil {
		return payloadReconstructInfo{}, err
	}
	if len(payload) != payloadCombinedSize {
		return payloadReconstructInfo{}, fmt.Errorf("payload has size 0x%X, expected 0x%X", len(payload), payloadCombinedSize)
	}
	payloadSHA := sha256Hex(payload)
	if expectedSHA != "" && payloadSHA != expectedSHA {
		return payloadReconstructInfo{}, fmt.Errorf("payload sha256 %s, expected %s", payloadSHA, expectedSHA)
	}

	out := append([]byte(nil), packed...)
	sectionWrites := []payloadSectionWrite{
		{Name: ".text", RawOffset: uint32(len(packed)), RawSize: payloadTextSize},
		{Name: ".rdata", RawOffset: uint32(len(packed) + payloadTextSize), RawSize: payloadRDataSize},
		{Name: ".data", RawOffset: uint32(len(packed) + payloadTextSize + payloadRDataSize), RawSize: payloadDataSize},
	}
	for _, section := range sectionWrites {
		if err := patchSectionRawRange(out, section.Name, section.RawOffset, section.RawSize); err != nil {
			return payloadReconstructInfo{}, err
		}
	}
	out = append(out, payload...)
	if err := writeFileAtomic(outputPath, out); err != nil {
		return payloadReconstructInfo{}, err
	}
	return payloadReconstructInfo{OutputPath: outputPath, PayloadSHA: payloadSHA, Sections: sectionWrites}, nil
}

func patchSectionRawRange(peData []byte, sectionName string, rawOffset uint32, rawSize uint32) error {
	sectionOffset, err := sectionHeaderOffset(peData, sectionName)
	if err != nil {
		return err
	}
	binary.LittleEndian.PutUint32(peData[sectionOffset+16:sectionOffset+20], rawSize)
	binary.LittleEndian.PutUint32(peData[sectionOffset+20:sectionOffset+24], rawOffset)
	return nil
}

func sectionHeaderOffset(peData []byte, sectionName string) (int, error) {
	if len(peData) < 0x40 || !bytes.Equal(peData[:2], []byte("MZ")) {
		return 0, fmt.Errorf("not a PE/MZ file")
	}
	peOffset := int(binary.LittleEndian.Uint32(peData[0x3C:0x40]))
	if peOffset < 0 || peOffset+24 > len(peData) || !bytes.Equal(peData[peOffset:peOffset+4], []byte("PE\x00\x00")) {
		return 0, fmt.Errorf("invalid PE header")
	}
	sectionCount := int(binary.LittleEndian.Uint16(peData[peOffset+6 : peOffset+8]))
	optionalSize := int(binary.LittleEndian.Uint16(peData[peOffset+20 : peOffset+22]))
	sectionTable := peOffset + 24 + optionalSize
	for i := 0; i < sectionCount; i++ {
		offset := sectionTable + i*40
		if offset+40 > len(peData) {
			return 0, fmt.Errorf("section table extends beyond file")
		}
		name := string(bytes.TrimRight(peData[offset:offset+8], "\x00"))
		if name == sectionName {
			return offset, nil
		}
	}
	return 0, fmt.Errorf("section %s not found", sectionName)
}
