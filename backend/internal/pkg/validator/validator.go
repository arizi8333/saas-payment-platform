package validator

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// ValidateStruct validates a struct based on its validation tags.
func ValidateStruct(s interface{}) error {
	return validate.Struct(s)
}

// FormatValidationErrors formats validation errors into readable messages.
func FormatValidationErrors(err error) []string {
	var messages []string

	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		return []string{err.Error()}
	}

	for _, e := range validationErrors {
		messages = append(messages, formatFieldError(e))
	}

	return messages
}

// formatFieldError formats a single field validation error.
func formatFieldError(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", e.Field())
	case "email":
		return fmt.Sprintf("%s must be a valid email address", e.Field())
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", e.Field(), e.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", e.Field(), e.Param())
	case "uuid":
		return fmt.Sprintf("%s must be a valid UUID", e.Field())
	case "url":
		return fmt.Sprintf("%s must be a valid URL", e.Field())
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", e.Field(), e.Param())
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", e.Field(), e.Param())
	default:
		return fmt.Sprintf("%s failed validation: %s", e.Field(), e.Tag())
	}
}
