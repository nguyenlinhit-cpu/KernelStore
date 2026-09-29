package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/KernelStore/frontend/internal/client"
	"github.com/KernelStore/frontend/internal/i18n"
	"github.com/KernelStore/frontend/internal/views"
)

func (a *App) cartPage(w http.ResponseWriter, r *http.Request) {
	c, err := a.api.GetCart(r.Context(), session(r).Token)
	page(w, r, views.CartPage(c, errString(err)))
}

// cartSetQty: đặt số lượng (≤ 0 thì backend xoá khỏi giỏ).
func (a *App) cartSetQty(w http.ResponseWriter, r *http.Request) {
	qty, _ := strconv.Atoi(r.FormValue("quantity"))
	c, err := a.api.UpdateCartItem(r.Context(), session(r).Token, r.PathValue("pid"), qty)
	a.writeCart(w, r, c, err, "")
}

func (a *App) cartRemove(w http.ResponseWriter, r *http.Request) {
	c, err := a.api.DeleteCartItem(r.Context(), session(r).Token, r.PathValue("pid"))
	a.writeCart(w, r, c, err, i18n.Tc(r.Context(), "cart.removed"))
}

// writeCart: thành công → giỏ mới (+ toast info nếu có); lỗi → giỏ hiện tại + [ERROR] + toast.
func (a *App) writeCart(w http.ResponseWriter, r *http.Request, c *client.Cart, err error, okMsg string) {
	if err != nil {
		toast(w, toastError, err.Error())
		current, _ := a.api.GetCart(r.Context(), session(r).Token)
		fragment(w, r, views.CartSection(current, err.Error()))
		return
	}
	if okMsg != "" {
		toast(w, toastInfo, okMsg)
	}
	fragment(w, r, views.CartSection(c, ""))
}

func (a *App) checkoutPage(w http.ResponseWriter, r *http.Request) {
	s := session(r)
	c, err := a.api.GetCart(r.Context(), s.Token)
	// Điền sẵn tên người nhận từ tài khoản đang đăng nhập.
	page(w, r, views.CheckoutPage(c, errString(err), views.CheckoutForm{FullName: s.User.FullName}))
}

// checkoutSubmit: kiểm các trường bắt buộc như bản Rust rồi tạo đơn.
func (a *App) checkoutSubmit(w http.ResponseWriter, r *http.Request) {
	ctx, s := r.Context(), session(r)
	f := views.CheckoutForm{
		FullName: r.FormValue("fullName"), Phone: r.FormValue("phone"), Street: r.FormValue("street"),
		Ward: r.FormValue("ward"), District: r.FormValue("district"), City: r.FormValue("city"), Note: r.FormValue("note"),
	}
	cart, _ := a.api.GetCart(ctx, s.Token)
	if cart == nil {
		cart = &client.Cart{}
	}
	if strings.TrimSpace(f.FullName) == "" || strings.TrimSpace(f.Phone) == "" ||
		strings.TrimSpace(f.Street) == "" || strings.TrimSpace(f.City) == "" {
		f.Error = i18n.Tc(ctx, "checkout.missing")
		fragment(w, r, views.CheckoutBody(cart, f))
		return
	}
	order, err := a.api.CreateOrder(ctx, s.Token, client.OrderPayload{
		FullName: strings.TrimSpace(f.FullName), Phone: strings.TrimSpace(f.Phone),
		Street: strings.TrimSpace(f.Street), Ward: strings.TrimSpace(f.Ward),
		District: strings.TrimSpace(f.District), City: strings.TrimSpace(f.City), Note: strings.TrimSpace(f.Note),
	})
	if err != nil {
		f.Error = err.Error()
		toast(w, toastError, err.Error())
		fragment(w, r, views.CheckoutBody(cart, f))
		return
	}
	toast(w, toastOK, strings.Replace(i18n.Tc(ctx, "checkout.placed_toast"), "{}", order.OrderCode, 1))
	fragment(w, r, views.OrderPlaced(order))
}
