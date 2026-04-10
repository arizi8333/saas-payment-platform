package hash

import (
	"testing"
)

func TestHashPassword_ReturnsNonEmptyHashDifferentFromInput(t *testing.T) {
	password := "mysecurepassword"

	hashed, err := HashPassword(password)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if hashed == "" {
		t.Fatal("expected non-empty hash")
	}
	if hashed == password {
		t.Fatal("hash should differ from plaintext password")
	}
}

func TestCheckPassword_SucceedsWithCorrectPassword(t *testing.T) {
	password := "correctpassword"

	hashed, err := HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if err := CheckPassword(password, hashed); err != nil {
		t.Fatalf("expected nil error for correct password, got %v", err)
	}
}

func TestCheckPassword_FailsWithWrongPassword(t *testing.T) {
	password := "correctpassword"

	hashed, err := HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if err := CheckPassword("wrongpassword", hashed); err == nil {
		t.Fatal("expected error for wrong password, got nil")
	}
}
