package main

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	uninstallerOverlayStart = 0x12000
	uninstallerTextsID      = 0x00007f7f
	uninstallerRegistryID   = 0x00011445
	uninstallTextBlockID    = 0x1234
	uninstallFilesBlockID   = 0x1235
	uninstallIconsBlockID   = 0x1236

	canonicalWindowsProfile = `C:\Users\Admin`
)

var finalizedUninstallerTerminator = []byte{0x7f, 0x7f, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00}

func finalizeUninstaller(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read uninstaller: %w", err)
	}
	if len(data) < uninstallerOverlayStart {
		return errors.New("uninstaller is too small to contain Clickteam overlay")
	}

	chunks, err := decodeUninstallerOverlay(data[uninstallerOverlayStart:])
	if err != nil {
		return err
	}

	textBlock, err := extractUninstallerBlock(chunks[uninstallerTextsID], uninstallTextBlockID)
	if err != nil {
		return err
	}
	registryBlocks := chunks[uninstallerRegistryID]
	if len(registryBlocks) == 0 {
		return errors.New("uninstaller registry metadata block is missing")
	}

	var overlay bytes.Buffer
	overlay.Write(textBlock)
	writeUninstallerBlock(&overlay, uninstallFilesBlockID, buildUninstallerFilePayload())
	writeUninstallerBlock(&overlay, uninstallIconsBlockID, buildUninstallerIconPayload(canonicalWindowsProfile))
	overlay.Write(registryBlocks)
	overlay.Write(finalizedUninstallerTerminator)

	finalized := append(append([]byte{}, data[:uninstallerOverlayStart]...), overlay.Bytes()...)
	if err := os.WriteFile(path, finalized, 0o755); err != nil {
		return fmt.Errorf("write finalized uninstaller: %w", err)
	}
	return nil
}

func decodeUninstallerOverlay(overlay []byte) (map[uint32][]byte, error) {
	chunks := map[uint32][]byte{}
	for pos := 0; pos+13 <= len(overlay); {
		id := binary.LittleEndian.Uint32(overlay[pos:])
		length := int(binary.LittleEndian.Uint32(overlay[pos+4:]))
		end := pos + 8 + length
		end = min(end, len(overlay))
		record := overlay[pos+8 : end]
		if len(record) < 5 {
			return nil, fmt.Errorf("uninstaller overlay chunk 0x%X is truncated", id)
		}
		if record[4] != 1 {
			return nil, fmt.Errorf("uninstaller overlay chunk 0x%X uses unsupported encoding %d", id, record[4])
		}

		zr, err := zlib.NewReader(bytes.NewReader(record[5:]))
		if err != nil {
			return nil, fmt.Errorf("open uninstaller overlay chunk 0x%X: %w", id, err)
		}
		decoded, readErr := io.ReadAll(zr)
		closeErr := zr.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read uninstaller overlay chunk 0x%X: %w", id, readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close uninstaller overlay chunk 0x%X: %w", id, closeErr)
		}
		chunks[id] = decoded

		pos = end
		if id == uninstallerTextsID {
			break
		}
	}
	return chunks, nil
}

func extractUninstallerBlock(blocks []byte, wantID uint32) ([]byte, error) {
	for pos := 0; pos+8 <= len(blocks); {
		id := binary.LittleEndian.Uint32(blocks[pos:])
		length := int(binary.LittleEndian.Uint32(blocks[pos+4:]))
		end := pos + 8 + length
		if end > len(blocks) {
			return nil, fmt.Errorf("uninstaller block 0x%X is truncated", id)
		}
		if id == wantID {
			return append([]byte{}, blocks[pos:end]...), nil
		}
		if length == 0 {
			break
		}
		pos = end
	}
	return nil, fmt.Errorf("uninstaller block 0x%X is missing", wantID)
}

func writeUninstallerBlock(dst *bytes.Buffer, id uint32, payload []byte) {
	var header [8]byte
	binary.LittleEndian.PutUint32(header[0:4], id)
	binary.LittleEndian.PutUint32(header[4:8], uint32(len(payload)))
	dst.Write(header[:])
	dst.Write(payload)
}

func buildUninstallerFilePayload() []byte {
	var payload bytes.Buffer
	entries := make([]installEntry, 0, len(installEntries))
	for _, entry := range installEntries {
		if strings.EqualFold(entry.name, "Uninstal.exe") {
			continue
		}
		entries = append(entries, entry)
	}
	writeUint32(&payload, uint32(len(entries)))

	for _, entry := range entries {
		dir, name := splitInstallEntry(entry.name)
		writeUint16(&payload, uint16(len(dir)))
		payload.WriteString(dir)
		writeUint16(&payload, uint16(len(name)))
		payload.WriteString(name)
		payload.WriteByte(0)
	}

	return payload.Bytes()
}

func buildUninstallerIconPayload(windowsProfile string) []byte {
	windowsProfile = strings.TrimRight(strings.ReplaceAll(windowsProfile, "/", `\`), `\`)
	programGroup := windowsProfile + `\AppData\Roaming\Microsoft\Windows\Start Menu\Programs\Lemonade Tycoon 2 - New York City`
	desktop := windowsProfile + `\Desktop`
	gameLink := "Lemonade Tycoon 2 - New York City.lnk"

	entries := []struct {
		container string
		marker    byte
		linkName  string
	}{
		{container: programGroup, marker: '/', linkName: "Uninstall Lemonade Tycoon 2 - New York City.lnk"},
		{container: programGroup, marker: '%', linkName: gameLink},
		{container: desktop, marker: '%', linkName: gameLink},
	}

	var payload bytes.Buffer
	writeUint32(&payload, uint32(len(entries)))
	for i, entry := range entries {
		if i == 0 {
			payload.WriteByte(0)
		}
		writeUint16(&payload, uint16(len(entry.container)))
		payload.WriteString(entry.container)
		payload.WriteByte(entry.marker)
		payload.WriteByte(0)
		payload.WriteString(entry.linkName)
		if i != len(entries)-1 {
			payload.WriteByte(0)
		}
	}
	return payload.Bytes()
}

func splitInstallEntry(name string) (string, string) {
	normalized := strings.ReplaceAll(name, "/", `\`)
	index := strings.LastIndex(normalized, `\`)
	if index == -1 {
		return ".", normalized
	}
	return `.\` + normalized[:index], normalized[index+1:]
}

func writeUint16(dst *bytes.Buffer, value uint16) {
	var buf [2]byte
	binary.LittleEndian.PutUint16(buf[:], value)
	dst.Write(buf[:])
}

func writeUint32(dst *bytes.Buffer, value uint32) {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], value)
	dst.Write(buf[:])
}
