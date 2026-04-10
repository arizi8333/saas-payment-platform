package validator

import (
	"errors"
	"testing"
)

type validStruct struct {
	Name  string `validate:"required"`
	Email string `validate:"required,email"`
}

func TestValidateStruct_ValidStructPasses(t *testing.T) {
	s := validStruct{Name: "Alice", Email: "alice@example.com"}

	if err := ValidateStruct(s); err != nil {
		t.Fatalf("expected no error for valid struct, got %v", err)
	}
}

func TestValidateStruct_MissingRequiredFieldFails(t *testing.T) {
	s := validStruct{Name: "", Email: "alice@example.com"}

	err := ValidateStruct(s)

	if err == nil {
		t.Fatal("expected error for missing required field, got nil")
	}
}

func TestFormatValidationErrors_ReturnsReadableMessages(t *testing.T) {
	s := validStruct{Name: "", Email: "not-an-email"}

	err := ValidateStruct(s)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}

	messages := FormatValidationErrors(err)

	if len(messages) == 0 {
		t.Fatal("expected at least one formatted message")
	}

	// Verify messages are human-readable (not raw error strings).
	foundName := false
	foundEmail := false
	for _, msg := range messages {
		if msg == "Name is required" {
			foundName = true
		}
		if msg == "Email must be a valid email address" {
			foundEmail = true
		}
	}
	if !foundName {
		t.Errorf("expected 'Name is required' in messages, got %v", messages)
	}
	if !foundEmail {
		t.Errorf("expected 'Email must be a valid email address' in messages, got %v", messages)
	}
}

func TestFormatValidationErrors_NonValidationError_ReturnsFallback(t *testing.T) {
	err := errors.New("some generic error")

	messages := FormatValidationErrors(err)

	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0] != "some generic error" {
		t.Errorf("expected fallback message, got %q", messages[0])
	}
}
