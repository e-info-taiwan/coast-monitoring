package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func TestTokenHashIsStableAndNotRawToken(t *testing.T) {
	token := "session-token"
	hash1 := HashToken(token)
	hash2 := HashToken(token)
	if string(hash1) != string(hash2) {
		t.Fatal("token hash should be stable")
	}
	if string(hash1) == token {
		t.Fatal("token hash must not equal raw token")
	}
	if len(hash1) != sha256.Size {
		t.Fatalf("token hash length = %d, want %d", len(hash1), sha256.Size)
	}
	expected := sha256.Sum256([]byte(token))
	if string(hash1) != string(expected[:]) {
		t.Fatal("token hash should equal SHA-256 sum")
	}
}

func TestGenerateToken(t *testing.T) {
	token, err := GenerateToken(32)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	if len(token) != base64.RawURLEncoding.EncodedLen(32) {
		t.Fatalf("token length = %d, want %d", len(token), base64.RawURLEncoding.EncodedLen(32))
	}
	if _, err := base64.RawURLEncoding.DecodeString(token); err != nil {
		t.Fatalf("token should decode as raw URL base64: %v", err)
	}
	otherToken, err := GenerateToken(32)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	if token == otherToken {
		t.Fatal("two generated tokens should differ")
	}
}

func TestGenerateTokenRejectsZeroByteCount(t *testing.T) {
	token, err := GenerateToken(0)
	if err == nil {
		t.Fatal("expected error for zero byte count")
	}
	if token != "" {
		t.Fatalf("token = %q, want empty string", token)
	}
}

func TestGenerateTokenRejectsNegativeByteCount(t *testing.T) {
	token, err := GenerateToken(-1)
	if err == nil {
		t.Fatal("expected error for negative byte count")
	}
	if token != "" {
		t.Fatalf("token = %q, want empty string", token)
	}
}
