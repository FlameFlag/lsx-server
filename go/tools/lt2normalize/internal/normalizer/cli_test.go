package normalizer

import "testing"

func TestLooksPackedLike(t *testing.T) {
	info := peInfo{
		Sections: []sectionInfo{
			{Name: ".text", VA: 0x00401000, VirtualSize: 0x902d6, RawSize: 0, Entropy: 0},
			{Name: ".rdata", VA: 0x00492000, VirtualSize: 0x9aee, RawSize: 0, Entropy: 0},
			{Name: ".data", VA: 0x0049c000, VirtualSize: 0x6578, RawSize: 0, Entropy: 0},
			{Name: ".pdata", VA: 0x00503000, VirtualSize: 0x80000, RawSize: 0x71000, Entropy: 7.98},
		},
	}
	if !looksPackedLike(info) {
		t.Fatal("expected virtual original sections plus high entropy payload to look packed")
	}
}

func TestHasRawBytesForVA(t *testing.T) {
	info := peInfo{
		Sections: []sectionInfo{
			{Name: ".text", VA: 0x00401000, RawSize: 0x2000},
		},
	}
	if !info.hasRawBytesForVA(0x00401000) {
		t.Fatal("expected raw bytes at section start")
	}
	if info.hasRawBytesForVA(0x00405000) {
		t.Fatal("did not expect raw bytes outside section")
	}
}
