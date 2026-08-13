package storage

import (
	"crypto/rand"
	"encoding/hex"
)

func NewID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic("storage: crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(buf[:])
}
