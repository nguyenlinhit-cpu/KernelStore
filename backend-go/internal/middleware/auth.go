// Package middleware cung cấp các HTTP middleware cho backend Go.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/KernelStore/backend-go/internal/dto"
	"github.com/KernelStore/backend-go/internal/services"
)

type contextKey struct{}

// Claims trả thông tin người dùng đã xác thực (nil nếu request ẩn danh).
func Claims(ctx context.Context) *services.AccessClaims {
	c, _ := ctx.Value(contextKey{}).(*services.AccessClaims)
	return c
}

// UserID trả ID người dùng đã xác thực.
func UserID(ctx context.Context) (uuid.UUID, bool) {
	if c := Claims(ctx); c != nil {
		return c.UserID, true
	}
	return uuid.Nil, false
}

// IsInRole tương đương User.IsInRole trong controller C#.
func IsInRole(ctx context.Context, role string) bool {
	c := Claims(ctx)
	return c != nil && c.HasRole(role)
}

// Authenticate tương đương app.UseAuthentication(): đọc "Authorization: Bearer <jwt>",
// nếu hợp lệ thì gắn claims vào context. Không tự chặn request — việc chặn do
// RequireAuth/RequireRoles đảm nhiệm, nên endpoint ẩn danh vẫn chạy khi token sai.
func Authenticate(tokens *services.TokenService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if tok, ok := bearerToken(r); ok {
				if claims, err := tokens.ParseAccessToken(tok); err == nil {
					r = r.WithContext(context.WithValue(r.Context(), contextKey{}, claims))
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAuth tương đương [Authorize]: chưa xác thực → 401.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if Claims(r.Context()) == nil {
			w.Header().Set("WWW-Authenticate", "Bearer")
			dto.Unauthorized(w, "Chưa xác thực hoặc token không hợp lệ")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireRoles tương đương [Authorize(Roles = "A,B")]: chưa xác thực → 401,
// không có role nào trong danh sách → 403.
func RequireRoles(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c := Claims(r.Context())
			for _, role := range roles {
				if c.HasRole(role) {
					next.ServeHTTP(w, r)
					return
				}
			}
			dto.Forbidden(w, "Không có quyền truy cập")
		}))
	}
}

// bearerToken lấy token từ header; scheme "Bearer" không phân biệt hoa thường như ASP.NET.
func bearerToken(r *http.Request) (string, bool) {
	scheme, tok, found := strings.Cut(r.Header.Get("Authorization"), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	tok = strings.TrimSpace(tok)
	return tok, tok != ""
}
