package auth

import (
	"encoding/hex"
	"testing"
)

func TestPasswordHashingAndVerification(t *testing.T) {
	password := "correct-horse-battery"
	hash, err := Hash(password)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if hash == password || !Check(hash, password) {
		t.Fatal("expected non-plaintext hash that verifies")
	}
	if Check(hash, "incorrect-password") {
		t.Fatal("wrong password unexpectedly verified")
	}
}

func TestTokenIsUniqueHex(t *testing.T) {
	first, err := Token()
	if err != nil {
		t.Fatal(err)
	}
	second, err := Token()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 64 || first == second {
		t.Fatal("token must be a unique 256-bit hex value")
	}
	if _, err = hex.DecodeString(first); err != nil {
		t.Fatalf("token is not hex: %v", err)
	}
}
