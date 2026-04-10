package hash

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword hashes a password using bcrypt with the default cost.
// The plaintext password MUST NOT be logged.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(bytes), nil
}

// CheckPassword compares a plaintext password with a bcrypt hash.
// Returns nil if they match, or an error otherwise.
func CheckPassword(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
