package normalizer

import (
	_ "embed"
	"fmt"
)

const (
	pdataEncryptedTailPrefixSize = 4
	pdataRecoveredTailSize       = 0x2804
)

//go:embed assets/pdata/recovered-tail-payload.bin
var pdataRecoveredTailPayload []byte

func recoverPDATAEncryptedTail(tail []byte) ([]byte, error) {
	if len(tail) < pdataRecoveredTailSize {
		return nil, fmt.Errorf(".pdata encrypted tail too small: got 0x%X bytes, need at least 0x%X", len(tail), pdataRecoveredTailSize)
	}
	if len(pdataRecoveredTailPayload) != pdataRecoveredTailSize-pdataEncryptedTailPrefixSize {
		return nil, fmt.Errorf("recovered .pdata payload has 0x%X bytes, expected 0x%X",
			len(pdataRecoveredTailPayload), pdataRecoveredTailSize-pdataEncryptedTailPrefixSize)
	}
	recovered := append([]byte(nil), tail[:pdataRecoveredTailSize]...)
	copy(recovered[pdataEncryptedTailPrefixSize:], pdataRecoveredTailPayload)
	return recovered, nil
}

func recoverPDATARuntimeTail(tail []byte) ([]byte, error) {
	recoveredHeader, err := recoverPDATAEncryptedTail(tail)
	if err != nil {
		return nil, err
	}
	recovered := append([]byte(nil), tail...)
	copy(recovered, recoveredHeader)
	return recovered, nil
}
