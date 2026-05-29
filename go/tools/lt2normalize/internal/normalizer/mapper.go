package normalizer

import (
	"encoding/binary"
	"math/bits"
)

var generatedCRC32Table = buildGeneratedCRC32Table()

func generatedCRC32(data []byte, initial uint32) uint32 {
	value := initial
	for _, item := range data {
		value = generatedCRC32Table[(uint32(item)^value)&0xFF] ^ (value >> 8)
	}
	return value
}

func buildGeneratedCRC32Table() [256]uint32 {
	var table [256]uint32
	for item := range table {
		// FUN_1001F498 builds this through bit reversals around the normal CRC32 polynomial.
		value := uint32(bits.Reverse8(byte(item))) << 24
		for range 8 {
			if value&0x80000000 != 0 {
				value = (value << 1) ^ 0x04C11DB7
			} else {
				value <<= 1
			}
		}
		table[item] = bits.Reverse32(value)
	}
	return table
}

func generatedXORDwordsWithPRNG(data []byte, seed uint32) int {
	count := 0
	for offset := 0; offset+4 <= len(data); offset += 4 {
		value := binary.LittleEndian.Uint32(data[offset : offset+4])
		binary.LittleEndian.PutUint32(data[offset:offset+4], value^generatedPRNGDword(&seed))
		count++
	}
	return count
}
