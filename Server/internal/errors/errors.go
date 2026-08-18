package errors

import (
	"fmt"
	"net/http"
)

type AppError struct {
	Code    int
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}

	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func BadRequest(msg string, err error) *AppError {
	return &AppError{
		Code:    http.StatusBadRequest,
		Message: msg,
		Err:     err,
	}
}

func Unauthorized(msg string) *AppError {
	return &AppError{
		Code:    http.StatusUnauthorized,
		Message: msg,
	}
}

func Forbidden(msg string) *AppError {
	return &AppError{
		Code:    http.StatusForbidden,
		Message: msg,
	}
}

func NotFound(resource string) *AppError {
	return &AppError{
		Code:    http.StatusNotFound,
		Message: fmt.Sprintf("%s not found", resource),
	}
}

func Conflict(msg string) *AppError {
	return &AppError{
		Code:    http.StatusConflict,
		Message: msg,
	}
}

func UnprocessableEntity(msg string) *AppError {
	return &AppError{
		Code:    http.StatusUnprocessableEntity,
		Message: msg,
	}
}

func Internal(err error) *AppError {
	return &AppError{
		Code:    http.StatusInternalServerError,
		Message: "An internal server error occurred",
		Err:     err,
	}
}

func TooManyRequests(msg string) *AppError {
	return &AppError{
		Code: http.StatusTooManyRequests,
		Message: msg,
	}
}