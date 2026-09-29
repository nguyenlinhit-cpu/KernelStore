package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/KernelStore/frontend/internal/client"
	"github.com/KernelStore/frontend/internal/i18n"
	"github.com/KernelStore/frontend/internal/views"
)

func (a *App) ordersPage(w http.ResponseWriter, r *http.Request) {
	list, err := a.api.ListOrders(r.Context(), session(r).Token)
	page(w, r, views.OrdersPage(list, errString(err)))
}

func (a *App) orderDetailPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	o, err := a.api.GetOrder(ctx, session(r).Token, r.PathValue("id"))
	switch {
	case client.IsNotFound(err):
		render(w, r, http.StatusNotFound, layoutWith(views.OrderPanic("404",
			i18n.Tc(ctx, "orders.not_found"), i18n.Tc(ctx, "orders.not_found_detail"))))
	case err != nil:
		render(w, r, http.StatusBadGateway, layoutWith(views.OrderPanic("500",
			i18n.Tc(ctx, "orders.detail_failed"), err.Error())))
	default:
		page(w, r, views.OrderDetailPage(o))
	}
}

// Thông báo toast khi hành động của người mua / seller thành công.
var orderActionToasts = map[string]struct{ level, key string }{
	"confirm-received": {toastOK, "orders.received_toast"},
	"cancel":           {toastInfo, "orders.cancel_toast"},
	"return":           {toastInfo, "orders.return_toast"},
	"return/approve":   {toastInfo, "orders.return_approved"},
	"return/reject":    {toastInfo, "orders.return_rejected"},
}

// orderAction: confirm-received | cancel | return | return/approve | return/reject.
func (a *App) orderAction(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, token, id := r.Context(), session(r).Token, r.PathValue("id")
		updated, err := a.api.OrderAction(ctx, token, id, action)
		if err != nil {
			toast(w, toastError, err.Error())
			a.writeOrder(w, r, id, "")
			return
		}
		ta := orderActionToasts[action]
		toast(w, ta.level, i18n.Tc(ctx, ta.key))
		fragment(w, r, views.OrderDetailSection(updated, ""))
	}
}

// orderStatus: seller/admin đổi trạng thái qua dropdown.
func (a *App) orderStatus(w http.ResponseWriter, r *http.Request) {
	ctx, token, id := r.Context(), session(r).Token, r.PathValue("id")
	current, err := a.api.GetOrder(ctx, token, id)
	if err != nil {
		toast(w, toastError, err.Error())
		w.WriteHeader(http.StatusOK)
		return
	}
	target := r.FormValue("status")
	if target == current.Status {
		fragment(w, r, views.OrderDetailSection(current, i18n.Tc(ctx, "orders.status_unchanged")))
		return
	}
	updated, err := a.api.UpdateOrderStatus(ctx, token, id, target)
	if err != nil {
		toast(w, toastError, err.Error())
		fragment(w, r, views.OrderDetailSection(current, i18n.Tc(ctx, "products.error")+err.Error()))
		return
	}
	toast(w, toastInfo, i18n.Tc(ctx, "orders.status_arrow")+updated.Status)
	fragment(w, r, views.OrderDetailSection(updated, i18n.Tc(ctx, "orders.status_updated")))
}

// writeOrder render lại khung chi tiết với dữ liệu mới nhất (sau lỗi).
func (a *App) writeOrder(w http.ResponseWriter, r *http.Request, id, statusMsg string) {
	o, err := a.api.GetOrder(r.Context(), session(r).Token, id)
	if err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}
	fragment(w, r, views.OrderDetailSection(o, statusMsg))
}

func (a *App) submitReview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pid, name, comment := r.FormValue("productId"), r.FormValue("productName"), r.FormValue("comment")
	rating, _ := strconv.Atoi(r.FormValue("rating"))
	ratingStr := strconv.Itoa(rating)
	if _, err := a.api.CreateReview(ctx, session(r).Token, pid, rating, strings.TrimSpace(comment)); err != nil {
		toast(w, toastError, err.Error())
		fragment(w, r, views.ReviewForm(pid, name, ratingStr, comment, i18n.Tc(ctx, "products.error")+err.Error(), false))
		return
	}
	toast(w, toastOK, i18n.Tc(ctx, "orders.review_submitted"))
	fragment(w, r, views.ReviewForm(pid, name, ratingStr, "", i18n.Tc(ctx, "orders.review_thanks"), true))
}

func (a *App) submitWarranty(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	detailID, name := r.FormValue("orderDetailId"), r.FormValue("productName")
	s := views.WarrantyFormState{Open: true, Description: r.FormValue("description"), ImageUrl: r.FormValue("imageUrl")}
	desc := strings.TrimSpace(s.Description)
	// Rust kiểm desc.len() (số byte UTF-8) < 10.
	if len(desc) < 10 {
		s.Msg = i18n.Tc(ctx, "warranty.desc_too_short")
		fragment(w, r, views.WarrantyForm(detailID, name, s))
		return
	}
	claim, err := a.api.CreateWarranty(ctx, session(r).Token, client.WarrantyPayload{
		OrderDetailID: detailID, Description: desc, ImageUrl: strings.TrimSpace(s.ImageUrl),
	})
	if err != nil {
		s.Msg = i18n.Tc(ctx, "products.error") + err.Error()
		toast(w, toastError, err.Error())
		fragment(w, r, views.WarrantyForm(detailID, name, s))
		return
	}
	toast(w, toastOK, i18n.Tc(ctx, "warranty.submitted"))
	fragment(w, r, views.WarrantyForm(detailID, name, views.WarrantyFormState{
		Done: true, Msg: i18n.Tc(ctx, "warranty.submitted_code") + claim.ClaimCode,
	}))
}
