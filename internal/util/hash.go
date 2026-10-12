package util

import (
	"crypto/sha256"
	"encoding/hex"
)

func SHA256(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
