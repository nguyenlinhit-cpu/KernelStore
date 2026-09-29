package middleware

import (
	"net/http"
	"strings"

	"github.com/KernelStore/backend-go/internal/config"
)

// CORS trả về middleware tương đương policy "FrontendPolicy" của bản C#:
// WithOrigins(cfg) + AllowAnyMethod + AllowAnyHeader + AllowCredentials.
//
// Giống ASP.NET CORS middleware:
//   - Chỉ coi là preflight khi có Origin + Access-Control-Request-Method;
//     OPTIONS thường vẫn đi tiếp xuống router.
//   - Preflight luôn trả 204; origin không được phép thì không có header CORS
//     (trình duyệt tự chặn).
//   - AllowAny* nghĩa là phản hồi lại đúng method/header mà client yêu cầu.
func CORS(cfg *config.Config) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(cfg.CORSAllowedOrigins))
	for _, o := range cfg.CORSAllowedOrigins {
		allowed[strings.TrimRight(o, "/")] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			h := w.Header()
			h.Add("Vary", "Origin")
			ok := allowed[strings.TrimRight(origin, "/")]
			if ok {
				h.Set("Access-Control-Allow-Origin", origin)
				h.Set("Access-Control-Allow-Credentials", "true")
			}

			reqMethod := r.Header.Get("Access-Control-Request-Method")
			if r.Method == http.MethodOptions && reqMethod != "" {
				if ok {
					h.Set("Access-Control-Allow-Methods", reqMethod)
					if reqHeaders := r.Header.Get("Access-Control-Request-Headers"); reqHeaders != "" {
						h.Set("Access-Control-Allow-Headers", reqHeaders)
					}
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
