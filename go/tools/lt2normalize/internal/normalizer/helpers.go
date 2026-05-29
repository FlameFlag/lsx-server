package normalizer

import (
	"crypto/sha256"
	"debug/pe"
	"encoding/hex"
	"fmt"
	"strings"
)

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func sha256HexUpper(data []byte) string {
	return strings.ToUpper(sha256Hex(data))
}

func peSection(file *pe.File, name string) *pe.Section {
	if section := file.Section(name); section != nil {
		return section
	}
	for _, section := range file.Sections {
		if peSectionName(section) == name {
			return section
		}
	}
	return nil
}

func peSectionName(section *pe.Section) string {
	return strings.TrimRight(section.Name, "\x00")
}

func readSectionData(path, name string) ([]byte, error) {
	file, err := pe.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	section := peSection(file, name)
	if section == nil {
		return nil, fmt.Errorf("%s section not found", name)
	}
	return section.Data()
}

func readSectionVA(path, name string) (uint32, error) {
	file, err := pe.Open(path)
	if err != nil {
		return 0, err
	}
	defer func() { _ = file.Close() }()
	optional, ok := file.OptionalHeader.(*pe.OptionalHeader32)
	if !ok {
		return 0, fmt.Errorf("%s is not a PE32 image", path)
	}
	section := peSection(file, name)
	if section == nil {
		return 0, fmt.Errorf("%s section not found", name)
	}
	return optional.ImageBase + section.VirtualAddress, nil
}
