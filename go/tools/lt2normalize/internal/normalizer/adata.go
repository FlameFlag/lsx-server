package normalizer

import (
	"bytes"
	"debug/pe"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	adataSectionName        = ".adata"
	adataFirstPatchOffset   = 0x10A
	adataFirstMarkerOffset  = 0x17B
	adataFirstPayloadOffset = adataFirstMarkerOffset + 4
	adataFirstPayloadSize   = 0x9B4F
	adataFirstMarker        = uint32(0x00005478)
)

type adataDecodeInfo struct {
	Path           string
	Key            byte
	MarkerOffset   int
	PayloadOffset  int
	PayloadSize    int
	PatchedRetByte bool
}

func decodeADATA(path, outputPath string) (adataDecodeInfo, error) {
	data, sectionOffset, sectionSize, err := readSectionForPatch(path, adataSectionName)
	if err != nil {
		return adataDecodeInfo{}, err
	}
	sectionData := data[sectionOffset : sectionOffset+sectionSize]
	info, err := decodeADATAFirstStage(sectionData)
	if err != nil {
		return adataDecodeInfo{}, err
	}
	info.Path = outputPath
	if err := writeFileAtomic(outputPath, data); err != nil {
		return adataDecodeInfo{}, err
	}
	return info, nil
}

func readSectionForPatch(path, name string) ([]byte, int, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, 0, err
	}
	file, err := pe.Open(path)
	if err != nil {
		return nil, 0, 0, err
	}
	defer func() { _ = file.Close() }()

	section := peSection(file, name)
	if section != nil {
		start := int(section.Offset)
		size := int(section.Size)
		if start < 0 || size < 0 || start+size > len(data) {
			return nil, 0, 0, fmt.Errorf("%s section raw range 0x%X..0x%X is outside file size 0x%X",
				name, start, start+size, len(data))
		}
		return data, start, size, nil
	}
	return nil, 0, 0, fmt.Errorf("%s section not found", name)
}

func decodeADATAFirstStage(sectionData []byte) (adataDecodeInfo, error) {
	if len(sectionData) < adataFirstPayloadOffset+adataFirstPayloadSize {
		return adataDecodeInfo{}, fmt.Errorf(".adata too small for first-stage decode: got 0x%X bytes, need at least 0x%X",
			len(sectionData), adataFirstPayloadOffset+adataFirstPayloadSize)
	}

	key, ok := findADATAFirstStageKey(sectionData[adataFirstMarkerOffset : adataFirstMarkerOffset+4])
	if !ok {
		return adataDecodeInfo{}, errors.New("unable to find .adata first-stage XOR key")
	}

	xorDword(sectionData[adataFirstMarkerOffset:adataFirstMarkerOffset+4], key)
	if got := littleEndianUint32(sectionData[adataFirstMarkerOffset : adataFirstMarkerOffset+4]); got != adataFirstMarker {
		return adataDecodeInfo{}, fmt.Errorf(".adata marker decode produced 0x%08X, expected 0x%08X", got, adataFirstMarker)
	}
	xorBytes(sectionData[adataFirstPayloadOffset:adataFirstPayloadOffset+adataFirstPayloadSize], key)

	patchedRet := false
	if adataFirstPatchOffset < len(sectionData) && sectionData[adataFirstPatchOffset] != 0x90 {
		sectionData[adataFirstPatchOffset] = 0x90
		patchedRet = true
	}

	return adataDecodeInfo{
		Key:            key,
		MarkerOffset:   adataFirstMarkerOffset,
		PayloadOffset:  adataFirstPayloadOffset,
		PayloadSize:    adataFirstPayloadSize,
		PatchedRetByte: patchedRet,
	}, nil
}

func findADATAFirstStageKey(marker []byte) (byte, bool) {
	if len(marker) != 4 {
		return 0, false
	}
	original := littleEndianUint32(marker)
	for key := 1; key <= 0xFF; key++ {
		mask := repeatedByteDword(byte(key))
		if original^mask == adataFirstMarker {
			return byte(key), true
		}
	}
	return 0, false
}

func littleEndianUint32(data []byte) uint32 {
	return uint32(data[0]) |
		uint32(data[1])<<8 |
		uint32(data[2])<<16 |
		uint32(data[3])<<24
}

func repeatedByteDword(key byte) uint32 {
	return uint32(key) * 0x01010101
}

func xorDword(data []byte, key byte) {
	for i := range 4 {
		data[i] ^= key
	}
}

func xorBytes(data []byte, key byte) {
	for i := range data {
		data[i] ^= key
	}
}

func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tempPath)
		}
	}()

	if _, err := bytes.NewReader(data).WriteTo(temp); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	removeTemp = false
	return nil
}
