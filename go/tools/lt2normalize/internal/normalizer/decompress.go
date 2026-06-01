package normalizer

import (
	"errors"
	"fmt"
)

type generatedChunk struct {
	Data       []byte
	Stored     bool
	SourceSize int
	NextOffset int
}

func readGeneratedChunk(stream []byte, offset int, lengthKey uint32, outputLimit int) (generatedChunk, error) {
	if offset < 0 || offset+4 > len(stream) {
		return generatedChunk{}, fmt.Errorf("generated chunk header at 0x%X exceeds stream size 0x%X", offset, len(stream))
	}
	rawLength := littleEndianUint32(stream[offset:offset+4]) ^ lengthKey
	stored := rawLength&0x80000000 != 0
	sourceSize := int(rawLength & 0x7FFFFFFF)
	sourceStart := offset + 4
	sourceEnd := sourceStart + sourceSize
	if sourceSize == 0 {
		return generatedChunk{Stored: stored, SourceSize: sourceSize, NextOffset: sourceEnd}, nil
	}
	if sourceSize < 0 || sourceEnd < sourceStart || sourceEnd > len(stream) {
		return generatedChunk{}, fmt.Errorf("generated chunk body 0x%X..0x%X exceeds stream size 0x%X", sourceStart, sourceEnd, len(stream))
	}
	source := stream[sourceStart:sourceEnd]
	var data []byte
	var err error
	if stored {
		if sourceSize > outputLimit {
			return generatedChunk{}, fmt.Errorf("stored generated chunk size 0x%X exceeds limit 0x%X", sourceSize, outputLimit)
		}
		data = append([]byte(nil), source...)
	} else {
		data, err = generatedRLEDecompress(source, outputLimit)
		if err != nil {
			return generatedChunk{}, err
		}
	}
	return generatedChunk{Data: data, Stored: stored, SourceSize: sourceSize, NextOffset: sourceEnd}, nil
}

func generatedRLEDecompress(source []byte, outputLimit int) ([]byte, error) {
	if len(source) == 0 {
		return nil, errors.New("empty generated RLE source")
	}
	if source[0] == 0 {
		plainSize := len(source) - 1
		if plainSize > outputLimit {
			return nil, fmt.Errorf("stored generated block expands to 0x%X bytes, limit 0x%X", plainSize, outputLimit)
		}
		return append([]byte(nil), source[1:]...), nil
	}

	out := make([]byte, 0, outputLimit)
	last := byte(0xFF)
	for cursor := 1; cursor < len(source); {
		item := source[cursor]
		cursor++
		if item != 0xFF {
			if len(out) >= outputLimit {
				return nil, fmt.Errorf("generated RLE output exceeds limit 0x%X", outputLimit)
			}
			out = append(out, item)
			last = item
			continue
		}
		if cursor >= len(source) {
			return nil, errors.New("truncated generated RLE escape")
		}
		escape := source[cursor]
		cursor++
		if escape == 0xFF {
			if len(out) >= outputLimit {
				return nil, fmt.Errorf("generated RLE output exceeds limit 0x%X", outputLimit)
			}
			out = append(out, 0xFF)
			last = 0xFF
			continue
		}
		repeat := int(escape) + 3
		if len(out)+repeat >= outputLimit {
			return nil, fmt.Errorf("generated RLE repeat exceeds limit 0x%X", outputLimit)
		}
		for range repeat {
			out = append(out, last)
		}
	}
	return out, nil
}
