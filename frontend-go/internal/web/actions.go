package web

import "net/http"

// actionRoutes: endpoint HTMX cho các hành động trên trang (trả về một phần HTML).
// Endpoint cần đăng nhập đi qua requireLogin như trang tương ứng.
func (a *App) actionRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /x/home/featured", a.homeFeatured)
	mux.HandleFunc("GET /x/products/suggest", a.suggest)
	mux.HandleFunc("GET /x/products/{id}/reviews", a.productReviews)
	mux.HandleFunc("POST /x/cart/add", a.addToCart)
	mux.HandleFunc("POST /x/chat/start", a.startChat)

	mux.HandleFunc("POST /x/cart/{pid}/qty", a.requireLogin(a.cartSetQty))
	mux.HandleFunc("POST /x/cart/{pid}/remove", a.requireLogin(a.cartRemove))
	mux.HandleFunc("POST /x/checkout", a.requireLogin(a.checkoutSubmit))

	for _, action := range []string{"confirm-received", "cancel", "return", "return/approve", "return/reject"} {
		mux.HandleFunc("POST /x/orders/{id}/"+action, a.requireLogin(a.orderAction(action)))
	}
	mux.HandleFunc("POST /x/orders/{id}/status", a.requireLogin(a.orderStatus))
	mux.HandleFunc("POST /x/reviews", a.requireLogin(a.submitReview))
	mux.HandleFunc("POST /x/warranty", a.requireLogin(a.submitWarranty))
	for _, action := range []string{"cancel", "approve", "reject", "process", "complete"} {
		mux.HandleFunc("POST /x/warranty/{id}/"+action, a.requireLogin(a.warrantyAction(action)))
	}

	admin := func(h http.HandlerFunc) http.HandlerFunc { return a.requireRole(h, "Admin") }
	for _, action := range []string{"approve", "reject", "ban", "unban", "delete"} {
		mux.HandleFunc("POST /x/admin/shops/{id}/"+action, admin(a.adminShopAction(action)))
	}
	mux.HandleFunc("GET /x/admin/categories", admin(a.adminCategoriesForm))
	mux.HandleFunc("POST /x/admin/categories", admin(a.adminCategorySave))
	mux.HandleFunc("POST /x/admin/categories/{id}/delete", admin(a.adminCategoryDelete))

	login := a.requireLogin
	mux.HandleFunc("GET /x/seller/dashboard", login(a.sellerDashboard))
	mux.HandleFunc("GET /x/seller/sales", login(a.sellerSales))
	mux.HandleFunc("POST /x/seller/shop", login(a.createShop))
	mux.HandleFunc("POST /x/seller/shop/update", login(a.updateShop))
	mux.HandleFunc("POST /x/seller/products", login(a.saveProduct))
	mux.HandleFunc("POST /x/seller/products/{id}", login(a.saveProduct))
	mux.HandleFunc("POST /x/seller/products/{id}/delete", login(a.deleteProduct))
	mux.HandleFunc("GET /x/seller/products/{id}/row", login(a.productRow))
	mux.HandleFunc("POST /x/seller/upload", login(a.uploadImage))
	mux.HandleFunc("GET /x/seller/categories", login(a.sellerCatsForm))
	mux.HandleFunc("POST /x/seller/categories", login(a.sellerCatSave))
	mux.HandleFunc("POST /x/seller/categories/{id}/delete", login(a.sellerCatDelete))

	mux.HandleFunc("GET /x/chat/list", login(a.chatList))
	mux.HandleFunc("POST /x/chat/{id}/send", login(a.chatSend))
}
