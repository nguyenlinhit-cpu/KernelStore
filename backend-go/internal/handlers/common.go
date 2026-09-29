package handlers

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/KernelStore/backend-go/internal/dto"
	"github.com/KernelStore/backend-go/internal/middleware"
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
