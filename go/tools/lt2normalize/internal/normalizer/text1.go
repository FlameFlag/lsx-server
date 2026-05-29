package normalizer

import "fmt"

const (
	text1SectionName       = ".text1"
	text1FinalXORKey       = byte(0x5E)
	text1FinalXORByteCount = 0x2F800
)

type text1FinalizeInfo struct {
	Key       byte
	ByteCount int
}

func finalizeText1LoaderSection(sectionData []byte) (text1FinalizeInfo, error) {
	if len(sectionData) < text1FinalXORByteCount {
		return text1FinalizeInfo{}, fmt.Errorf(".text1 too small for final XOR: got 0x%X bytes, need at least 0x%X",
			len(sectionData), text1FinalXORByteCount)
	}
	xorBytes(sectionData[:text1FinalXORByteCount], text1FinalXORKey)
	return text1FinalizeInfo{
		Key:       text1FinalXORKey,
		ByteCount: text1FinalXORByteCount,
	}, nil
}
