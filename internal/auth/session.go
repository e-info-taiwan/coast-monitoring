package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
)

const MinTokenBytes = 32

func GenerateToken(byteCount int) (string, error) {
	if byteCount < MinTokenBytes {
		return "", errors.New("byteCount must be at least 32")
	}

	raw := make([]byte, byteCount)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
