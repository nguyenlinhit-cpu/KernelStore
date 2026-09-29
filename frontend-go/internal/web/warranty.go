package web

import (
	"net/http"

	"github.com/KernelStore/frontend-go/internal/i18n"
	"github.com/KernelStore/frontend-go/internal/views"
)

func (a *App) warrantyPage(w http.ResponseWriter, r *http.Request) {
	list, err := a.api.ListMyWarranty(r.Context(), session(r).Token)
	page(w, r, views.WarrantyPage(list, errString(err)))
}

func (a *App) warrantyManagePage(w http.ResponseWriter, r *http.Request) {
	filter := r.URL.Query().Get("status")
	list, err := a.api.ListShopWarranty(r.Context(), session(r).Token, filter)
	page(w, r, views.WarrantyManagePage(list, errString(err), filter))
}

// Toast khi xử lý thành công, theo hành động.
var warrantyOKKeys = map[string]string{
	"cancel":   "warranty.cancelled_ok",
	"approve":  "warranty.approved_ok",
	"reject":   "warranty.rejected_ok",
	"process":  "warranty.processing_ok",
	"complete": "warranty.completed_ok",
}

// warrantyAction thực hiện hành động rồi nạp lại danh sách (khách hoặc quản lý).
func (a *App) warrantyAction(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, token, id := r.Context(), session(r).Token, r.PathValue("id")
		var body any
		switch action {
		case "approve":
			body = map[string]string{"resolution": r.FormValue("resolution"), "note": r.FormValue("note")}
		case "reject", "complete":
			body = map[string]string{"note": r.FormValue("note")}
		}
		if _, err := a.api.WarrantyAction(ctx, token, id, action, body); err != nil {
			toast(w, toastError, err.Error())
		} else {
			toast(w, toastOK, i18n.Tc(ctx, warrantyOKKeys[action]))
		}

		manage, filter := r.FormValue("manage") != "", r.FormValue("filter")
		if manage {
			list, err := a.api.ListShopWarranty(ctx, token, filter)
			fragment(w, r, views.ClaimList(list, errString(err), true, filter))
			return
		}
		list, err := a.api.ListMyWarranty(ctx, token)
		fragment(w, r, views.ClaimList(list, errString(err), false, ""))
	}
}
