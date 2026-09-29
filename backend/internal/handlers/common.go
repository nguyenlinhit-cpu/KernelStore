package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/KernelStore/backend/internal/dto"
	"github.com/KernelStore/backend/internal/middleware"
)

// userID lấy ID người dùng từ token (endpoint đã qua RequireAuth nên luôn có).
func userID(r *http.Request) uuid.UUID {
	id, _ := middleware.UserID(r.Context())
	return id
}

// serverError ghi log và trả 500 — tương đương exception không bắt ở C#.
func serverError(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("unhandled error", "method", r.Method, "path", r.URL.Path, "err", err)
	dto.InternalError(w, "Lỗi hệ thống")
}

func strPtr(s string) *string { return &s }

func deref(p *string) string {
	if p != nil {
		return *p
	}
	return ""
}

// cmpErr trả err nếu có, ngược lại tạo lỗi với thông điệp msg.
func cmpErr(err error, msg string) error {
	if err != nil {
		return err
	}
	return errors.New(msg)
}
