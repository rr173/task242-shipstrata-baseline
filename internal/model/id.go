package model

import (
	"crypto/rand"
	"encoding/hex"
)

// NewID 生成带前缀的稳定随机标识，用于实体主键。
func NewID(prefix string) string {
	buf := make([]byte, 9)
	if _, err := rand.Read(buf); err != nil {
		buf = []byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9}
	}
	return prefix + "_" + hex.EncodeToString(buf)
}
