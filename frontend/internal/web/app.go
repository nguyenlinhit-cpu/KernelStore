// Package web là server frontend SSR (thay app Leptos CSR): định tuyến trang,
// phiên đăng nhập bằng cookie, và các endpoint HTMX cho hành động trên trang.
package web

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/a-h/templ"

	"github.com/KernelStore/frontend/internal/client"
	"github.com/KernelStore/frontend/internal/i18n"
	"github.com/KernelStore/frontend/internal/views"
)

// Cookie phiên — HttpOnly nên JS không đọc được token (khác bản Rust dùng localStorage).
const (
	accessCookie  = "ks_access"
	refreshCookie = "ks_refresh"
	cookieMaxAge  = 7 * 24 * 3600 // bằng hạn refresh token
)

type App struct {
	api       *client.Client
	staticDir string
	wsBase    string // ws://localhost:5000/ws/chat — trang chat mở WebSocket trực tiếp tới backend
}

func New(api *client.Client, staticDir, wsBase string) *App {
	return &App{api: api, staticDir: staticDir, wsBase: wsBase}
}

// Handler dựng router + middleware.
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir(a.staticDir))))
	a.routes(mux)
	return a.withSession(mux)
}

// withSession: đọc ngôn ngữ + token từ cookie; có token thì gọi /auth/me để lấy user.
// Token hỏng/hết hạn → xoá cookie (như restore_user() → logout() của bản Rust).
func (a *App) withSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := i18n.WithLang(r.Context(), i18n.FromRequest(r))
		s := &views.Session{Path: r.URL.Path, Tab: r.URL.Query().Get("tab")}
		if c, err := r.Cookie(accessCookie); err == nil && c.Value != "" && !isStatic(r) {
			if u, err := a.api.Me(ctx, c.Value); err == nil {
				s.Token, s.User = c.Value, u
			} else {
				clearAuthCookies(w)
			}
		}
		next.ServeHTTP(w, r.WithContext(views.WithSession(ctx, s)))
	})
}

func isStatic(r *http.Request) bool { return len(r.URL.Path) >= 8 && r.URL.Path[:8] == "/static/" }

func session(r *http.Request) *views.Session { return views.SessionFrom(r.Context()) }

// requireLogin = ProtectedRoute(condition: có token) → chưa đăng nhập thì về /auth/login.
func (a *App) requireLogin(h http.HandlerFunc) http.HandlerFunc { return a.requireRole(h) }

// requireRole: cần đăng nhập và (nếu có danh sách) một trong các vai trò.
func (a *App) requireRole(h http.HandlerFunc, roles ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := session(r).User
		ok := u != nil
		if ok && len(roles) > 0 {
			ok = false
			for _, role := range roles {
				ok = ok || u.Role == role
			}
		}
		if !ok {
			redirect(w, r, "/auth/login")
			return
		}
		h(w, r)
	}
}

// redirect: request HTMX → header HX-Redirect; request thường → 302.
func redirect(w http.ResponseWriter, r *http.Request, to string) {
	if r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Boosted") != "true" {
		w.Header().Set("HX-Redirect", to)
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, to, http.StatusSeeOther)
}

// page render trang đầy đủ (layout + nội dung).
func page(w http.ResponseWriter, r *http.Request, body templ.Component) {
	render(w, r, http.StatusOK, layoutWith(body))
}

// fragment render một phần trang (response của hành động HTMX).
func fragment(w http.ResponseWriter, r *http.Request, c templ.Component) {
	render(w, r, http.StatusOK, c)
}

func render(w http.ResponseWriter, r *http.Request, status int, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := c.Render(r.Context(), w); err != nil {
		slog.Error("render", "path", r.URL.Path, "err", err)
	}
}

// Toast levels (khớp ToastLevel của toast.rs).
const (
	toastInfo  = "INFO"
	toastWarn  = "WARN"
	toastError = "ERROR"
	toastOK    = "OK"
)

// toast gắn header HX-Trigger để app.js hiện toast "[LEVEL] message".
// Phải gọi trước khi ghi body.
func toast(w http.ResponseWriter, level, msg string) {
	b, _ := json.Marshal(map[string]any{"ks-toast": map[string]string{"level": level, "message": msg}})
	w.Header().Set("HX-Trigger", string(b))
}

func setAuthCookies(w http.ResponseWriter, auth *client.AuthData) {
	for name, val := range map[string]string{accessCookie: auth.AccessToken, refreshCookie: auth.RefreshToken} {
		http.SetCookie(w, &http.Cookie{Name: name, Value: val, Path: "/", MaxAge: cookieMaxAge,
			HttpOnly: true, SameSite: http.SameSiteLaxMode})
	}
}

func clearAuthCookies(w http.ResponseWriter) {
	for _, name := range []string{accessCookie, refreshCookie} {
		http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	}
}

// ctxTimeout giới hạn thời gian gọi backend cho một request.
func ctxTimeout(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), 30*time.Second)
}

// layoutWith bọc nội dung bằng layout (dùng khi cần status khác 200, ví dụ 404).
func layoutWith(body templ.Component) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, wr io.Writer) error {
		return views.Layout().Render(templ.WithChildren(ctx, body), wr)
	})
}
