package web

import (
	"net/http"
	"strings"

	"github.com/KernelStore/frontend/internal/client"
	"github.com/KernelStore/frontend/internal/i18n"
	"github.com/KernelStore/frontend/internal/views"
)

func (a *App) loginPage(w http.ResponseWriter, r *http.Request) {
	page(w, r, views.LoginPage(views.LoginForm{}))
}

func (a *App) registerPage(w http.ResponseWriter, r *http.Request) {
	page(w, r, views.RegisterPage(views.RegisterForm{}))
}

// loginSubmit: lỗi → trả lại form kèm lỗi (giữ giá trị đã nhập); thành công → lưu cookie, về "/".
func (a *App) loginSubmit(w http.ResponseWriter, r *http.Request) {
	f := views.LoginForm{Email: r.FormValue("email"), Password: r.FormValue("password")}
	if f.Email == "" || f.Password == "" {
		f.Error = i18n.Tc(r.Context(), "login.required")
		fragment(w, r, views.LoginFormView(f))
		return
	}
	auth, err := a.api.Login(r.Context(), f.Email, f.Password)
	if err != nil {
		f.Error = err.Error()
		fragment(w, r, views.LoginFormView(f))
		return
	}
	setAuthCookies(w, auth)
	redirect(w, r, "/")
}

func (a *App) registerSubmit(w http.ResponseWriter, r *http.Request) {
	f := views.RegisterForm{
		FullName: r.FormValue("fullName"), UserName: r.FormValue("userName"),
		Email: r.FormValue("email"), Password: r.FormValue("password"),
	}
	if f.FullName == "" || f.UserName == "" || f.Email == "" || f.Password == "" {
		f.Error = i18n.Tc(r.Context(), "register.required")
		fragment(w, r, views.RegisterFormView(f))
		return
	}
	auth, err := a.api.Register(r.Context(), client.RegisterPayload{
		FullName: f.FullName, Email: f.Email, UserName: f.UserName, Password: f.Password,
	})
	if err != nil {
		f.Error = err.Error()
		fragment(w, r, views.RegisterFormView(f))
		return
	}
	setAuthCookies(w, auth)
	redirect(w, r, "/")
}

// logout: xoá phiên rồi về "/" (trang chủ cần đăng nhập nên sẽ chuyển tiếp tới /auth/login).
func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	clearAuthCookies(w)
	redirect(w, r, "/")
}

// toggleLang: đổi EN ⇄ VI, lưu cookie rồi tải lại trang.
func (a *App) toggleLang(w http.ResponseWriter, r *http.Request) {
	next := i18n.FromRequest(r).Toggle()
	http.SetCookie(w, &http.Cookie{Name: i18n.CookieName, Value: next.Code(), Path: "/",
		MaxAge: 365 * 24 * 3600, SameSite: http.SameSiteLaxMode})
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

func (a *App) notFound(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	render(w, r, http.StatusNotFound, layoutWith(views.KernelPanic("404",
		i18n.Tc(ctx, "panic.route_not_found"), i18n.Tc(ctx, "panic.route_detail"), "", "")))
}

func (a *App) homePage(w http.ResponseWriter, r *http.Request) {
	page(w, r, views.HomePage())
}

// homeFeatured: nạp "featured" sau khi trang hiện (giữ skeleton/loading như bản Rust).
func (a *App) homeFeatured(w http.ResponseWriter, r *http.Request) {
	products, err := a.api.FeaturedProducts(r.Context(), 8)
	fragment(w, r, views.HomeFeatured(products, errString(err)))
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func trimmed(r *http.Request, key string) string { return strings.TrimSpace(r.FormValue(key)) }
