package web

import "net/http"

// routes: cùng tập URL với router Leptos (lib.rs) + các endpoint hành động HTMX dưới /x/.
func (a *App) routes(mux *http.ServeMux) {
	// Trang
	mux.HandleFunc("GET /{$}", a.requireLogin(a.homePage)) // "/" là ProtectedRoute ở bản Rust
	mux.HandleFunc("GET /products", a.productsPage)
	mux.HandleFunc("GET /products/{slug}", a.productDetailPage)
	mux.HandleFunc("GET /auth/login", a.loginPage)
	mux.HandleFunc("GET /auth/register", a.registerPage)
	mux.HandleFunc("GET /cart", a.requireLogin(a.cartPage))
	mux.HandleFunc("GET /checkout", a.requireLogin(a.checkoutPage))
	mux.HandleFunc("GET /orders", a.requireLogin(a.ordersPage))
	mux.HandleFunc("GET /orders/{id}", a.requireLogin(a.orderDetailPage))
	mux.HandleFunc("GET /warranty", a.requireLogin(a.warrantyPage))
	mux.HandleFunc("GET /warranty/manage", a.requireRole(a.warrantyManagePage, "Seller", "Admin"))
	mux.HandleFunc("GET /seller", a.requireLogin(a.sellerPage))
	mux.HandleFunc("GET /admin", a.requireRole(a.adminPage, "Admin"))
	mux.HandleFunc("GET /chat", a.requireLogin(a.chatPage))

	// Phiên + ngôn ngữ
	mux.HandleFunc("POST /auth/login", a.loginSubmit)
	mux.HandleFunc("POST /auth/register", a.registerSubmit)
	mux.HandleFunc("POST /auth/logout", a.logout)
	mux.HandleFunc("POST /lang", a.toggleLang)

	a.actionRoutes(mux)

	// Còn lại → KernelPanic 404 (fallback của <Routes>).
	mux.HandleFunc("/", a.notFound)
}
