package normalizer

import (
	"bytes"
	"testing"
)

func TestDecodeADATAFirstStage(t *testing.T) {
	key := byte(0xAF)
	plainPayload := []byte("decoded first-stage bytes")
	section := bytes.Repeat([]byte{0xCC}, adataFirstPayloadOffset+adataFirstPayloadSize)
	section[adataFirstPatchOffset] = 0xEB
	copy(section[adataFirstMarkerOffset:adataFirstMarkerOffset+4], []byte{0x78, 0x54, 0x00, 0x00})
	copy(section[adataFirstPayloadOffset:], plainPayload)

	xorDword(section[adataFirstMarkerOffset:adataFirstMarkerOffset+4], key)
	xorBytes(section[adataFirstPayloadOffset:adataFirstPayloadOffset+adataFirstPayloadSize], key)

	info, err := decodeADATAFirstStage(section)
	if err != nil {
		t.Fatalf("decodeADATAFirstStage returned error: %v", err)
	}
	if info.Key != key {
		t.Fatalf("key = 0x%02X, want 0x%02X", info.Key, key)
	}
	if !info.PatchedRetByte {
		t.Fatal("expected patched ret byte")
	}
	if section[adataFirstPatchOffset] != 0x90 {
		t.Fatalf("patch byte = 0x%02X, want NOP", section[adataFirstPatchOffset])
	}
	if got := littleEndianUint32(section[adataFirstMarkerOffset : adataFirstMarkerOffset+4]); got != adataFirstMarker {
		t.Fatalf("marker = 0x%08X, want 0x%08X", got, adataFirstMarker)
	}
	if got := section[adataFirstPayloadOffset : adataFirstPayloadOffset+len(plainPayload)]; !bytes.Equal(got, plainPayload) {
		t.Fatalf("payload prefix = % X, want % X", got, plainPayload)
	}
}

func TestDecodeADATAFirstStageRejectsUnknownKey(t *testing.T) {
	section := bytes.Repeat([]byte{0}, adataFirstPayloadOffset+adataFirstPayloadSize)
	_, err := decodeADATAFirstStage(section)
	if err == nil {
		t.Fatal("expected unknown key error")
	}
}
