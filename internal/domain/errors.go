package domain

import "fmt"

type ErrorCode string

const (
	ErrValidation ErrorCode = "VALIDATION_ERROR"
	ErrState      ErrorCode = "INVALID_STATE"
	ErrConflict   ErrorCode = "REVISION_CONFLICT"
	ErrNotFound   ErrorCode = "NOT_FOUND"
	ErrForbidden  ErrorCode = "FORBIDDEN"
	ErrIntegrity  ErrorCode = "INTEGRITY_ERROR"
)

type DomainError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Field   string    `json:"field,omitempty"`
}

func (e *DomainError) Error() string { return e.Message }

func NewError(code ErrorCode, field, format string, args ...any) error {
	return &DomainError{Code: code, Field: field, Message: fmt.Sprintf(format, args...)}
}

func IsCode(err error, code ErrorCode) bool {
	e, ok := err.(*DomainError)
	return ok && e.Code == code
}
