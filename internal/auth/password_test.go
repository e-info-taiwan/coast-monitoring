package auth

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("CorrectHorseBatteryStaple1!")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if !VerifyPassword(hash, "CorrectHorseBatteryStaple1!") {
		t.Fatal("expected password to verify")
	}
	if VerifyPassword(hash, "wrong-password") {
		t.Fatal("wrong password verified")
	}
}

func TestHashPasswordUsesSalt(t *testing.T) {
	hash1, err := HashPassword("CorrectHorseBatteryStaple1!")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	hash2, err := HashPassword("CorrectHorseBatteryStaple1!")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if hash1 == hash2 {
		t.Fatal("expected hashes for the same password to differ")
	}
}
