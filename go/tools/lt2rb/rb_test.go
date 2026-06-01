package main

import (
	"encoding/binary"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestParseAndDecodeBitmapRecords(t *testing.T) {
	rb := testRB(
		testBitmap{width: 2, height: 1, format: formatRGB565Mask, data: []byte{
			0x00, 0xF8, 0x80,
			0xFF, 0xFF, 0x00,
		}},
		testBitmap{width: 2, height: 1, format: 0x20660, flags: 0x00FF00FF, data: []byte{
			0xE0, 0x07,
			0x1F, 0xF8,
		}},
		testBitmap{width: 1, height: 1, format: formatGray8, data: []byte{0x80}},
	)

	records, err := parseBitmapRecords(rb)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 {
		t.Fatalf("len(records) = %d, want 3", len(records))
	}

	masked, err := decodeBitmapRecord(records[0], true)
	if err != nil {
		t.Fatal(err)
	}
	assertNRGBA(t, masked.NRGBAAt(0, 0), color.NRGBA{R: 255, G: 0, B: 0, A: 128})
	assertNRGBA(t, masked.NRGBAAt(1, 0), color.NRGBA{R: 255, G: 255, B: 255, A: 0})

	rgb565, err := decodeBitmapRecord(records[1], true)
	if err != nil {
		t.Fatal(err)
	}
	assertNRGBA(t, rgb565.NRGBAAt(0, 0), color.NRGBA{R: 0, G: 255, B: 0, A: 255})
	assertNRGBA(t, rgb565.NRGBAAt(1, 0), color.NRGBA{R: 255, G: 0, B: 255, A: 0})

	gray, err := decodeBitmapRecord(records[2], true)
	if err != nil {
		t.Fatal(err)
	}
	assertNRGBA(t, gray.NRGBAAt(0, 0), color.NRGBA{R: 128, G: 128, B: 128, A: 255})
}

func TestExtractBitmapPNGs(t *testing.T) {
	dir := t.TempDir()
	rb := testRB(testBitmap{width: 1, height: 1, format: formatRGB565Mask, data: []byte{0x00, 0xF8, 0xFF}})

	count, err := extractBitmapPNGs(rb, dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}

	output := filepath.Join(dir, "bitmap_000_1x1_rgb565a8.png")
	file, err := os.Open(output)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()

	decoded, err := png.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	got := color.NRGBAModel.Convert(decoded.At(0, 0)).(color.NRGBA)
	assertNRGBA(t, got, color.NRGBA{R: 255, G: 0, B: 0, A: 255})
}

type testBitmap struct {
	width  uint16
	height uint16
	format uint32
	flags  uint32
	data   []byte
}

func testRB(bitmaps ...testBitmap) []byte {
	payloadSize := 0
	for _, bitmap := range bitmaps {
		payloadSize += 12 + len(bitmap.data)
	}

	rb := make([]byte, segmentTableOffset+12+payloadSize)
	segment := rb[segmentTableOffset:]
	binary.LittleEndian.PutUint32(segment, bitmapSegmentType)
	binary.LittleEndian.PutUint32(segment[4:], uint32(len(rb)))
	binary.LittleEndian.PutUint32(segment[8:], uint32(len(bitmaps)))

	offset := segmentTableOffset + 12
	for _, bitmap := range bitmaps {
		binary.LittleEndian.PutUint16(rb[offset:], bitmap.width)
		binary.LittleEndian.PutUint16(rb[offset+2:], bitmap.height)
		binary.LittleEndian.PutUint32(rb[offset+4:], bitmap.format)
		binary.LittleEndian.PutUint32(rb[offset+8:], bitmap.flags)
		copy(rb[offset+12:], bitmap.data)
		offset += 12 + len(bitmap.data)
	}

	return rb
}

func assertNRGBA(t *testing.T, got color.NRGBA, want color.NRGBA) {
	t.Helper()
	if got != want {
		t.Fatalf("color = %#v, want %#v", got, want)
	}
}
