package auth

import "testing"

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
}

func TestGenerateToken(t *testing.T) {
	token, err := GenerateToken(32)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	if len(token) < 40 {
		t.Fatalf("token length = %d, want at least 40", len(token))
	}
}
