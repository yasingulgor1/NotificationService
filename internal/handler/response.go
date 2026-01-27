package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/insider-one/notification-service/internal/domain"
)

// Response represents a standard API response
type Response struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

// Error represents an API error
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// JSON writes a JSON response
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	response := Response{
		Success: status >= 200 && status < 300,
		Data:    data,
	}

	json.NewEncoder(w).Encode(response)
}

// JSONError writes an error response
func JSONError(w http.ResponseWriter, status int, code, message string, details any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	response := Response{
		Success: false,
		Error: &Error{
			Code:    code,
			Message: message,
			Details: details,
		},
	}

	json.NewEncoder(w).Encode(response)
}

// HandleError handles common domain errors and writes appropriate responses
func HandleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		JSONError(w, http.StatusNotFound, "NOT_FOUND", "Resource not found", nil)

	case errors.Is(err, domain.ErrAlreadyExists):
		JSONError(w, http.StatusConflict, "ALREADY_EXISTS", "Resource already exists", nil)

	case errors.Is(err, domain.ErrCannotCancel):
		JSONError(w, http.StatusBadRequest, "CANNOT_CANCEL", "Notification cannot be cancelled", nil)

	case errors.Is(err, domain.ErrBatchSizeExceeded):
		JSONError(w, http.StatusBadRequest, "BATCH_SIZE_EXCEEDED", "Batch size exceeds maximum limit of 1000", nil)

	case errors.Is(err, domain.ErrTemplateNotFound):
		JSONError(w, http.StatusBadRequest, "TEMPLATE_NOT_FOUND", "Template not found", nil)

	case errors.Is(err, domain.ErrMissingVariables):
		JSONError(w, http.StatusBadRequest, "MISSING_VARIABLES", err.Error(), nil)

	case errors.Is(err, domain.ErrIdempotencyConflict):
		JSONError(w, http.StatusConflict, "IDEMPOTENCY_CONFLICT", "Idempotency key already used", nil)

	default:
		var validationErr domain.ValidationError
		if errors.As(err, &validationErr) {
			JSONError(w, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message, map[string]string{
				"field": validationErr.Field,
			})
			return
		}

		var validationErrs domain.ValidationErrors
		if errors.As(err, &validationErrs) {
			JSONError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Validation failed", validationErrs.Errors)
			return
		}

		// Log internal errors
		JSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred", nil)
	}
}

// DecodeJSON decodes JSON request body
func DecodeJSON(r *http.Request, v any) error {
	if r.Body == nil {
		return domain.NewValidationError("body", "request body is required")
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(v); err != nil {
		return domain.NewValidationError("body", "invalid JSON: "+err.Error())
	}

	return nil
}
