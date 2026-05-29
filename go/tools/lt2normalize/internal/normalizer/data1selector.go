package normalizer

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
)

const (
	data1SelectorMaxCount      = 8
	data1SelectorChunkLimit    = 0x20000
	data1SelectorExpectedOff   = 0x4489
	data1SelectorExpectedSize  = 0x2188
	data1SelectorExpectedCount = 1
)

type data1SelectorInfo struct {
	OutputPath  string
	Offset      int
	Count       int
	Names       []string
	Selected    int
	PayloadSize int
	SHA256      string
}

type data1SelectorCandidate struct {
	Offset  int
	Count   int
	Names   []string
	Entries []data1SelectorEntry
	End     int
	Payload []byte
	Score   int
}

type data1SelectorEntry struct {
	Offset      int
	Limit       uint32
	PayloadSize int
	ChunkCount  int
	End         int
}

func deriveData1SelectorPayload(packedPath, outputPath string) (data1SelectorInfo, error) {
	data1, err := readSectionData(packedPath, ".data1")
	if err != nil {
		return data1SelectorInfo{}, err
	}
	candidate, err := findBestData1Selector(data1)
	if err != nil {
		return data1SelectorInfo{}, err
	}
	if candidate.Offset != data1SelectorExpectedOff || candidate.Count != data1SelectorExpectedCount || len(candidate.Payload) != data1SelectorExpectedSize {
		return data1SelectorInfo{}, fmt.Errorf("unexpected .data1 selector offset/count/size: 0x%X/%d/0x%X", candidate.Offset, candidate.Count, len(candidate.Payload))
	}
	if err := os.WriteFile(outputPath, candidate.Payload, 0o644); err != nil {
		return data1SelectorInfo{}, err
	}
	sum := sha256.Sum256(candidate.Payload)
	return data1SelectorInfo{
		OutputPath:  outputPath,
		Offset:      candidate.Offset,
		Count:       candidate.Count,
		Names:       candidate.Names,
		Selected:    0,
		PayloadSize: len(candidate.Payload),
		SHA256:      hex.EncodeToString(sum[:]),
	}, nil
}

func findBestData1Selector(data1 []byte) (data1SelectorCandidate, error) {
	best := data1SelectorCandidate{}
	found := false
	for offset := 0; offset+8 <= len(data1); offset++ {
		candidate, ok := parseData1SelectorAt(data1, offset)
		if !ok {
			continue
		}
		if !found || candidate.Score > best.Score || (candidate.Score == best.Score && candidate.Offset < best.Offset) {
			best = candidate
			found = true
		}
	}
	if !found {
		return data1SelectorCandidate{}, fmt.Errorf("no .data1 selector table candidate found")
	}
	return best, nil
}

func parseData1SelectorAt(data1 []byte, offset int) (data1SelectorCandidate, bool) {
	count := int(littleEndianUint32(data1[offset:]))
	if count < 1 || count > data1SelectorMaxCount {
		return data1SelectorCandidate{}, false
	}
	cursor := offset + 4
	names := make([]string, 0, count)
	for range count {
		name, next, ok := readData1SelectorString(data1, cursor)
		if !ok {
			return data1SelectorCandidate{}, false
		}
		names = append(names, name)
		cursor = next
	}
	entries := make([]data1SelectorEntry, 0, count)
	var selectedPayload []byte
	for index := range count {
		if cursor+4 > len(data1) {
			return data1SelectorCandidate{}, false
		}
		entryOffset := cursor
		limit := littleEndianUint32(data1[cursor:])
		cursor += 4
		chunkCount := 0
		payloadSize := 0
		payload := []byte{}
		for {
			chunk, err := readGeneratedChunk(data1, cursor, 0, data1SelectorChunkLimit)
			if err != nil {
				return data1SelectorCandidate{}, false
			}
			cursor = chunk.NextOffset
			chunkCount++
			if len(chunk.Data) == 0 {
				break
			}
			payloadSize += len(chunk.Data)
			if index == 0 {
				payload = append(payload, chunk.Data...)
			}
			if chunkCount > 32 {
				return data1SelectorCandidate{}, false
			}
		}
		if index == 0 {
			selectedPayload = payload
		}
		entries = append(entries, data1SelectorEntry{Offset: entryOffset, Limit: limit, PayloadSize: payloadSize, ChunkCount: chunkCount, End: cursor})
	}
	score := len(selectedPayload)
	for _, name := range names {
		score += len(name)
	}
	return data1SelectorCandidate{Offset: offset, Count: count, Names: names, Entries: entries, End: cursor, Payload: selectedPayload, Score: score}, true
}

func readData1SelectorString(data []byte, offset int) (string, int, bool) {
	if offset+2 > len(data) {
		return "", 0, false
	}
	size := int(binary.LittleEndian.Uint16(data[offset:]))
	if size > 256 || offset+2+size > len(data) {
		return "", 0, false
	}
	for _, item := range data[offset+2 : offset+2+size] {
		if item != 9 && item != 10 && item != 13 && (item < 32 || item >= 127) {
			return "", 0, false
		}
	}
	return string(data[offset+2 : offset+2+size]), offset + 2 + size, true
}
