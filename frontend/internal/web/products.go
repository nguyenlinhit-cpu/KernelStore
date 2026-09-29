package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/KernelStore/frontend/internal/client"
	"github.com/KernelStore/frontend/internal/i18n"
	"github.com/KernelStore/frontend/internal/views"
)

// Số sản phẩm mỗi trang (PAGE_SIZE của products.rs).
const productsPageSize = 12

func (a *App) productsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Giữ tham số có giá trị (đã trim), bỏ tham số rỗng — như apply() của bản Rust.
	q := url.Values{}
	for _, k := range []string{"category", "shop", "minPrice", "maxPrice", "search", "sort", "page"} {
		if v := strings.TrimSpace(r.URL.Query().Get(k)); v != "" {
			q.Set(k, v)
		}
	}
	pq := client.ProductQuery{
		Category: q.Get("category"), Shop: q.Get("shop"), Search: q.Get("search"), Sort: q.Get("sort"),
		MinPrice: parseFloat(q.Get("minPrice")), MaxPrice: parseFloat(q.Get("maxPrice")),
		Page: 1, PageSize: productsPageSize,
	}
	if n, err := strconv.ParseUint(q.Get("page"), 10, 32); err == nil {
		pq.Page = int(n)
	}

	vm := views.ProductsVM{Query: q}
	var err error
	if vm.Result, err = a.api.ListProducts(ctx, pq); err != nil {
		vm.Error = err.Error()
	}
	if vm.Categories, err = a.api.ListCategories(ctx); err != nil {
		vm.CatError = err.Error()
	}
	page(w, r, views.ProductsPage(vm))
}

// suggest: gõ ≥ 2 ký tự → tối đa 6 sản phẩm khớp.
func (a *App) suggest(w http.ResponseWriter, r *http.Request) {
	term := r.URL.Query().Get("search")
	if len(strings.TrimSpace(term)) < 2 {
		fragment(w, r, views.Suggestions(nil))
		return
	}
	res, err := a.api.ListProducts(r.Context(), client.ProductQuery{Search: term, Page: 1, PageSize: 6})
	if err != nil {
		fragment(w, r, views.Suggestions(nil))
		return
	}
	fragment(w, r, views.Suggestions(res.Items))
}

func (a *App) productDetailPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p, err := a.api.GetProduct(ctx, r.PathValue("slug"))
	switch {
	case client.IsNotFound(err):
		render(w, r, http.StatusNotFound, layoutWith(views.ProductNotFound("404",
			i18n.Tc(ctx, "pd.not_found"), i18n.Tc(ctx, "pd.not_found_detail"))))
	case err != nil:
		render(w, r, http.StatusBadGateway, layoutWith(views.ProductNotFound("500",
			i18n.Tc(ctx, "pd.load_failed"), err.Error())))
	default:
		page(w, r, views.ProductDetailPage(p))
	}
}

func (a *App) productReviews(w http.ResponseWriter, r *http.Request) {
	rv, err := a.api.GetReviews(r.Context(), r.PathValue("id"))
	fragment(w, r, views.Reviews(rv, errString(err)))
}

// addToCart: chưa đăng nhập → sang trang login; xong thì báo "(N items)" + toast.
func (a *App) addToCart(w http.ResponseWriter, r *http.Request) {
	ctx, s := r.Context(), session(r)
	if s.User == nil {
		redirect(w, r, "/auth/login")
		return
	}
	cart, err := a.api.AddToCart(ctx, s.Token, r.FormValue("productId"), 1)
	if err != nil {
		toast(w, toastError, err.Error())
		fragment(w, r, views.CartFlash(i18n.Tc(ctx, "products.error")+err.Error()))
		return
	}
	added, items := i18n.Tc(ctx, "pd.added_flash"), i18n.Tc(ctx, "common.items")
	toast(w, toastOK, fmt.Sprintf("%s — %d %s", added, cart.TotalItems, items))
	fragment(w, r, views.CartFlash(fmt.Sprintf("%s (%d %s)", added, cart.TotalItems, items)))
}

// startChat: mở (hoặc lấy lại) hội thoại với shop rồi chuyển tới /chat?c=<id>.
func (a *App) startChat(w http.ResponseWriter, r *http.Request) {
	ctx, s := r.Context(), session(r)
	if s.User == nil {
		redirect(w, r, "/auth/login")
		return
	}
	convo, err := a.api.StartConversation(ctx, s.Token, r.FormValue("shopId"))
	if err != nil {
		toast(w, toastError, i18n.Tc(ctx, "pd.chat_failed")+err.Error())
		w.WriteHeader(http.StatusOK)
		return
	}
	redirect(w, r, "/chat?c="+url.QueryEscape(convo.ID))
}

// parseFloat như s.parse::<f64>().ok() — lỗi → nil.
func parseFloat(s string) *float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || s == "" {
		return nil
	}
	return &v
}
