package normalizer

import (
	"bytes"
	"testing"
)

func TestFinalizeText1LoaderSection(t *testing.T) {
	plainPrefix := bytes.Repeat([]byte{0x41}, text1FinalXORByteCount)
	tail := bytes.Repeat([]byte{0x99}, 0x800)
	section := append([]byte{}, plainPrefix...)
	section = append(section, tail...)
	for i := range plainPrefix {
		section[i] ^= text1FinalXORKey
	}

	info, err := finalizeText1LoaderSection(section)
	if err != nil {
		t.Fatalf("finalizeText1LoaderSection returned error: %v", err)
	}
	if info.Key != text1FinalXORKey {
		t.Fatalf("key = 0x%02X, want 0x%02X", info.Key, text1FinalXORKey)
	}
	if info.ByteCount != text1FinalXORByteCount {
		t.Fatalf("byte count = 0x%X, want 0x%X", info.ByteCount, text1FinalXORByteCount)
	}
	if !bytes.Equal(section[:text1FinalXORByteCount], plainPrefix) {
		t.Fatal("decoded .text1 prefix does not match expected bytes")
	}
	if !bytes.Equal(section[text1FinalXORByteCount:], tail) {
		t.Fatal(".text1 tail should be left unchanged")
	}
}

func TestFinalizeText1LoaderSectionRejectsShortSection(t *testing.T) {
	_, err := finalizeText1LoaderSection(make([]byte, text1FinalXORByteCount-1))
	if err == nil {
		t.Fatal("expected short .text1 error")
	}
}
