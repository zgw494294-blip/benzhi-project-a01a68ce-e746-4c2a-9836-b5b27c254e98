package domain

import (
	"errors"
	"fmt"
)

type ErrorCode string

const (
	CodeValidation ErrorCode = "validation"
	CodeConflict   ErrorCode = "conflict"
	CodeForbidden  ErrorCode = "forbidden"
	CodeNotFound   ErrorCode = "not_found"
	CodeState      ErrorCode = "invalid_state"
)

type DomainError struct {
	Code    ErrorCode     `json:"code"`
	Message string        `json:"message"`
	Details []ErrorDetail `json:"details,omitempty"`
}

type ErrorDetail struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *DomainError) Error() string { return e.Message }

func NewError(code ErrorCode, format string, args ...any) error {
	return &DomainError{Code: code, Message: fmt.Sprintf(format, args...)}
}

func NewDetailedError(code ErrorCode, message string, details []ErrorDetail) error {
	return &DomainError{Code: code, Message: message, Details: append([]ErrorDetail(nil), details...)}
}

func ErrorCodeOf(err error) ErrorCode {
	var e *DomainError
	if errors.As(err, &e) {
		return e.Code
	}
	return "internal"
}
