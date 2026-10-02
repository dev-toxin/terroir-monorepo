// Package domain contains Terroir core types and pure business logic.
// It has no dependencies on storage, HTTP or LLM code.
package domain

import (
	"errors"
	"fmt"
)

// Sentinel errors. Transport maps them to HTTP status codes.
var (
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrOutOfStock   = errors.New("no bottles left")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrBudget       = errors.New("llm budget exceeded")
)

// ValidationError describes invalid user input. Field is the JSON field name.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Invalid is a shortcut for constructing a ValidationError.
func Invalid(field, format string, args ...any) error {
	return &ValidationError{Field: field, Message: fmt.Sprintf(format, args...)}
}

// IsValidation reports whether err is (or wraps) a ValidationError.
func IsValidation(err error) bool {
	var v *ValidationError
	return errors.As(err, &v)
}
