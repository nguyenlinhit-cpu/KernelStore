package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/KernelStore/backend/internal/dto"
)

// Recovery bắt panic trong handler và trả JSON 500 thay vì crash server.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"stack", string(debug.Stack()),
				)
				dto.InternalError(w, "Lỗi hệ thống")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
