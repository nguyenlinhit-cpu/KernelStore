// Package dto chứa các struct request/response JSON (camelCase) giữ nguyên hợp đồng API.
package dto

import (
	"net/http"
)

// ─── ApiResponse ─────────────────────────────────────────────────────────────
// Format: { "success": bool, "data": ..., "message": string, "errors": [] }

type ApiResponse struct {
	Success bool     `json:"success"`
	Data    any      `json:"data"`
	Message string   `json:"message"`
	Errors  []string `json:"errors"`
}

// OK ghi JSON 200 OK.
func OK(w http.ResponseWriter, data any, message ...string) {
	msg := "OK"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	writeJSON(w, http.StatusOK, ApiResponse{
		Success: true,
		Data:    data,
		Message: msg,
		Errors:  []string{},
	})
}

// Created ghi JSON 201 Created.
func Created(w http.ResponseWriter, data any, message ...string) {
	msg := "Created"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	writeJSON(w, http.StatusCreated, ApiResponse{
		Success: true,
		Data:    data,
		Message: msg,
		Errors:  []string{},
	})
}

// Fail ghi JSON với status code chỉ định.
func Fail(w http.ResponseWriter, statusCode int, message string, errors ...string) {
	if errors == nil {
		errors = []string{}
	}
	writeJSON(w, statusCode, ApiResponse{
		Success: false,
		Data:    nil,
		Message: message,
		Errors:  errors,
	})
}

// BadRequest → 400.
func BadRequest(w http.ResponseWriter, message string, errors ...string) {
	Fail(w, http.StatusBadRequest, message, errors...)
}

// Unauthorized → 401.
func Unauthorized(w http.ResponseWriter, message string, errors ...string) {
	Fail(w, http.StatusUnauthorized, message, errors...)
}

// Forbidden → 403.
func Forbidden(w http.ResponseWriter, message string, errors ...string) {
	Fail(w, http.StatusForbidden, message, errors...)
}

// NotFound → 404.
func NotFound(w http.ResponseWriter, message string, errors ...string) {
	Fail(w, http.StatusNotFound, message, errors...)
}

// InternalError → 500.
func InternalError(w http.ResponseWriter, message string, errors ...string) {
	Fail(w, http.StatusInternalServerError, message, errors...)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	body, err := MarshalJSON(v)
	if err != nil {
		status, body = http.StatusInternalServerError, []byte(`{"success":false,"data":null,"message":"L\u1ED7i h\u1EC7 th\u1ED1ng","errors":[]}`)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
