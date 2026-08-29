package utils

import (
	"encoding/json"
	"net/http"

	apperrors "github.com/dprince-03/Bibliomania/internal/errors"
)

type APIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

type APIError struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
	Code    int    `json:"code"`
}

func WriteJSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func Success(w http.ResponseWriter, statusCode int, message string, data any) {
	WriteJSON(w, statusCode, APIResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

func Error(w http.ResponseWriter, err *apperrors.AppError) {
	WriteJSON(w, err.Code, APIError{
		Success: false,
		Error:   err.Message,
		Code:    err.Code,
	})
}

// HandleError is the single point every handler routes service/repository
// errors through: known *apperrors.AppError values keep their status code and
// message, anything else is treated as an unexpected 500.
func HandleError(w http.ResponseWriter, err error) {
	if appErr, ok := err.(*apperrors.AppError); ok {
		Error(w, appErr)
		return
	}
	Error(w, apperrors.Internal(err))
}
