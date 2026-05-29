package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

const (
	segmentTableOffset = 0x1038
	bitmapSegmentType  = 2

	formatRGB565     = 0x0660
	formatRGB565Mask = 0x8760
	formatGray8      = 0xC300
)

type rbSegment struct {
	typ        uint32
	offset     int
	nextOffset int
	count      uint32
}

type bitmapRecord struct {
	index  int
	offset int
	width  int
	height int
	format uint32
	flags  uint32
	data   []byte
}

func extractBitmapPNGs(rb []byte, outputDir string, transparency bool) (int, error) {
	records, err := parseBitmapRecords(rb)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return 0, fmt.Errorf("create image output directory: %w", err)
	}

	for _, record := range records {
		img, err := decodeBitmapRecord(record, transparency)
		if err != nil {
			return 0, fmt.Errorf("decode bitmap %03d at 0x%X: %w", record.index, record.offset, err)
		}

		name := fmt.Sprintf("bitmap_%03d_%dx%d_%s.png", record.index, record.width, record.height, bitmapFormatName(record.format))
		path := filepath.Join(outputDir, name)
		output, err := os.Create(path)
		if err != nil {
			return 0, fmt.Errorf("create %s: %w", path, err)
		}
		encodeErr := png.Encode(output, img)
		closeErr := output.Close()
		if encodeErr != nil {
			return 0, fmt.Errorf("encode %s: %w", path, encodeErr)
		}
		if closeErr != nil {
			return 0, fmt.Errorf("close %s: %w", path, closeErr)
		}
	}

	return len(records), nil
}

func parseBitmapRecords(rb []byte) ([]bitmapRecord, error) {
	segment, err := findSegment(rb, bitmapSegmentType)
	if err != nil {
		return nil, err
	}

	records := make([]bitmapRecord, 0, segment.count)
	offset := segment.offset + 12
	for offset < segment.nextOffset {
		if offset+12 > segment.nextOffset {
			return nil, fmt.Errorf("truncated bitmap header at 0x%X", offset)
		}

		width := int(binary.LittleEndian.Uint16(rb[offset:]))
		height := int(binary.LittleEndian.Uint16(rb[offset+2:]))
		format := binary.LittleEndian.Uint32(rb[offset+4:])
		flags := binary.LittleEndian.Uint32(rb[offset+8:])
		if width == 0 || height == 0 {
			return nil, fmt.Errorf("bad bitmap dimensions %dx%d at 0x%X", width, height, offset)
		}

		bpp, err := bytesPerPixel(format)
		if err != nil {
			return nil, fmt.Errorf("bitmap %d at 0x%X: %w", len(records), offset, err)
		}
		dataSize := int64(width) * int64(height) * int64(bpp)
		if dataSize > int64(segment.nextOffset-(offset+12)) {
			return nil, fmt.Errorf("bitmap %d at 0x%X exceeds segment bounds", len(records), offset)
		}

		dataStart := offset + 12
		dataEnd := dataStart + int(dataSize)
		records = append(records, bitmapRecord{
			index:  len(records),
			offset: offset,
			width:  width,
			height: height,
			format: format,
			flags:  flags,
			data:   rb[dataStart:dataEnd],
		})
		offset = dataEnd
	}

	if uint32(len(records)) != segment.count {
		return nil, fmt.Errorf("bitmap count mismatch: parsed %d, segment says %d", len(records), segment.count)
	}
	return records, nil
}

func findSegment(rb []byte, typ uint32) (rbSegment, error) {
	segments, err := parseSegments(rb)
	if err != nil {
		return rbSegment{}, err
	}
	for _, segment := range segments {
		if segment.typ == typ {
			return segment, nil
		}
	}
	return rbSegment{}, fmt.Errorf("segment type %d not found", typ)
}

func parseSegments(rb []byte) ([]rbSegment, error) {
	if len(rb) < segmentTableOffset+12 {
		return nil, errors.New("rb input is too small for the segment table")
	}

	segments := make([]rbSegment, 0)
	offset := segmentTableOffset
	for {
		if offset+12 > len(rb) {
			return nil, fmt.Errorf("truncated segment header at 0x%X", offset)
		}

		nextOffset := int(binary.LittleEndian.Uint32(rb[offset+4:]))
		if nextOffset <= offset || nextOffset > len(rb) {
			return nil, fmt.Errorf("bad segment chain at 0x%X: next offset 0x%X", offset, nextOffset)
		}

		segments = append(segments, rbSegment{
			typ:        binary.LittleEndian.Uint32(rb[offset:]),
			offset:     offset,
			nextOffset: nextOffset,
			count:      binary.LittleEndian.Uint32(rb[offset+8:]),
		})

		offset = nextOffset
		if offset == len(rb) {
			break
		}
	}

	return segments, nil
}

func bytesPerPixel(format uint32) (int, error) {
	switch format & 0xFFFF {
	case formatRGB565:
		return 2, nil
	case formatRGB565Mask:
		return 3, nil
	case formatGray8:
		return 1, nil
	default:
		return 0, fmt.Errorf("unknown bitmap format 0x%X", format)
	}
}

func bitmapFormatName(format uint32) string {
	switch format & 0xFFFF {
	case formatRGB565:
		return "rgb565"
	case formatRGB565Mask:
		return "rgb565a8"
	case formatGray8:
		return "gray8"
	default:
		return fmt.Sprintf("fmt%X", format)
	}
}

func decodeBitmapRecord(record bitmapRecord, transparency bool) (*image.NRGBA, error) {
	img := image.NewNRGBA(image.Rect(0, 0, record.width, record.height))
	pixels := record.width * record.height

	switch record.format & 0xFFFF {
	case formatRGB565Mask:
		for i := range pixels {
			base := i * 3
			value := binary.LittleEndian.Uint16(record.data[base:])
			c := colorFromRGB565(value)
			if transparency {
				c.A = record.data[base+2]
			}
			setNRGBA(img, i, c)
		}
	case formatRGB565:
		key := colorFromRGBFlag(record.flags)
		for i := range pixels {
			base := i * 2
			value := binary.LittleEndian.Uint16(record.data[base:])
			c := colorFromRGB565(value)
			if transparency && record.flags != 0 && sameRGB(c, key) {
				c.A = 0
			}
			setNRGBA(img, i, c)
		}
	case formatGray8:
		for i := range pixels {
			value := record.data[i]
			setNRGBA(img, i, color.NRGBA{R: value, G: value, B: value, A: 255})
		}
	default:
		return nil, fmt.Errorf("unknown bitmap format 0x%X", record.format)
	}

	return img, nil
}

func setNRGBA(img *image.NRGBA, pixel int, c color.NRGBA) {
	base := pixel * 4
	img.Pix[base] = c.R
	img.Pix[base+1] = c.G
	img.Pix[base+2] = c.B
	img.Pix[base+3] = c.A
}

func colorFromRGB565(value uint16) color.NRGBA {
	r := (value >> 11) & 0x1F
	g := (value >> 5) & 0x3F
	b := value & 0x1F
	return color.NRGBA{
		R: byte((r << 3) | (r >> 2)),
		G: byte((g << 2) | (g >> 4)),
		B: byte((b << 3) | (b >> 2)),
		A: 255,
	}
}

func colorFromRGBFlag(value uint32) color.NRGBA {
	return color.NRGBA{
		R: byte(value >> 16),
		G: byte(value >> 8),
		B: byte(value),
		A: 255,
	}
}

func sameRGB(left color.NRGBA, right color.NRGBA) bool {
	return left.R == right.R && left.G == right.G && left.B == right.B
}
