package domain

import (
	"errors"
	"fmt"
)

// Domain Const errors
var (
	ErrNotFound            = errors.New("resource not found")
	ErrAlreadyExists       = errors.New("resource already exists")
	ErrInvalidInput        = errors.New("invalid input")
	ErrInvalidStatus       = errors.New("invalid status transition")
	ErrCannotCancel        = errors.New("notification cannot be cancelled")
	ErrRateLimitExceeded   = errors.New("rate limit exceeded")
	ErrTemplateNotFound    = errors.New("template not found")
	ErrMissingVariables    = errors.New("missing template variables")
	ErrBatchSizeExceeded   = errors.New("batch size exceeded maximum limit")
	ErrIdempotencyConflict = errors.New("idempotency key conflict")
	ErrProviderError       = errors.New("external provider error")
)

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

type ValidationErrors struct {
	Errors []ValidationError `json:"errors"`
}

func (e ValidationErrors) Error() string {
	if len(e.Errors) == 0 {
		return "validation failed"
	}
	return fmt.Sprintf("validation failed: %s", e.Errors[0].Error())
}

func NewValidationError(field, message string) ValidationError {
	return ValidationError{Field: field, Message: message}
}

type ProviderError struct {
	StatusCode int
	Message    string
	Retryable  bool
}

func (e ProviderError) Error() string {
	return fmt.Sprintf("provider error (status %d): %s", e.StatusCode, e.Message)
}

func NewProviderError(statusCode int, message string, retryable bool) ProviderError {
	return ProviderError{
		StatusCode: statusCode,
		Message:    message,
		Retryable:  retryable,
	}
}
