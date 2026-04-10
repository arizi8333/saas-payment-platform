package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const testSecret = "test-secret-key-for-jwt-testing"

func TestGenerateToken_ReturnsValidTokenString(t *testing.T) {
	userID := uuid.New()

	token, err := GenerateToken(userID, "developer", testSecret, 24)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token string")
	}
}

func TestValidateToken_ValidToken_ReturnsCorrectClaims(t *testing.T) {
	userID := uuid.New()
	role := "admin"

	tokenStr, err := GenerateToken(userID, role, testSecret, 24)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	claims, err := ValidateToken(tokenStr, testSecret)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if claims.UserID != userID {
		t.Errorf("expected UserID %s, got %s", userID, claims.UserID)
	}
	if claims.Role != role {
		t.Errorf("expected Role %q, got %q", role, claims.Role)
	}
	if claims.Issuer != "saas-payment-platform" {
		t.Errorf("expected Issuer 'saas-payment-platform', got %q", claims.Issuer)
	}
}

func TestValidateToken_ExpiredToken_ReturnsError(t *testing.T) {
	userID := uuid.New()

	// Create a token that is already expired by crafting claims manually.
	claims := Claims{
		UserID: userID,
		Role:   "developer",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			Issuer:    "saas-payment-platform",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("failed to sign expired token: %v", err)
	}

	_, err = ValidateToken(tokenStr, testSecret)

	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestValidateToken_InvalidSecret_ReturnsError(t *testing.T) {
	userID := uuid.New()

	tokenStr, err := GenerateToken(userID, "developer", testSecret, 24)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	_, err = ValidateToken(tokenStr, "wrong-secret")

	if err == nil {
		t.Fatal("expected error for invalid secret, got nil")
	}
}

func TestValidateToken_MalformedToken_ReturnsError(t *testing.T) {
	_, err := ValidateToken("not.a.valid.jwt.token", testSecret)

	if err == nil {
		t.Fatal("expected error for malformed token, got nil")
	}
}
