package web

import (
	"context"
	"net/http"
	"strings"

	"github.com/a-h/templ"

	"github.com/KernelStore/frontend/internal/client"
	"github.com/KernelStore/frontend/internal/i18n"
	"github.com/KernelStore/frontend/internal/views"
)

// adminPage: /admin?tab=dashboard|shops|categories (bản Rust chuyển tab ở client).
func (a *App) adminPage(w http.ResponseWriter, r *http.Request) {
	ctx, token := r.Context(), session(r).Token
	tab := r.URL.Query().Get("tab")
	var body templ.Component
	switch tab {
	case "shops":
		filter := r.URL.Query().Get("status")
		if filter == "" {
			filter = "Pending" // mặc định như bản Rust
		}
		body = views.ShopModeration(a.shopModeration(ctx, token, filter))
	case "categories":
		body = views.CategoryManagement(a.categoryMgmt(ctx, views.CategoryFormState{}))
	default:
		tab = "dashboard"
		s, err := a.api.AdminDashboard(ctx, token)
		body = views.AdminDashboard(s, errString(err))
	}
	page(w, r, views.AdminPage(tab, body))
}

func (a *App) shopModeration(ctx context.Context, token, filter string) views.ShopModerationVM {
	status := filter
	if filter == "All" {
		status = ""
	}
	vm := views.ShopModerationVM{Filter: filter, RowErrors: map[string]string{}}
	shops, err := a.api.ListShops(ctx, token, status)
	vm.Shops, vm.Error = shops, errString(err)
	return vm
}

// Thông báo khi duyệt / từ chối / ban / gỡ ban thành công.
var shopActionFlash = map[string]string{
	"approve": "admin.approved_toast",
	"reject":  "admin.rejected_toast",
	"ban":     "admin.banned_toast",
	"unban":   "admin.unbanned_toast",
}

// adminShopAction: approve | reject | ban | unban | delete.
func (a *App) adminShopAction(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, token, id := r.Context(), session(r).Token, r.PathValue("id")
		filter, name := r.FormValue("filter"), r.FormValue("name")
		var flash string
		var err error
		if action == "delete" {
			flash, err = a.api.AdminDeleteShop(ctx, token, id) // hiện message của server
		} else if _, err = a.api.AdminShopAction(ctx, token, id, action); err == nil {
			flash = strings.Replace(i18n.Tc(ctx, shopActionFlash[action]), "{}", name, 1)
		}
		vm := a.shopModeration(ctx, token, filter)
		if err != nil {
			vm.RowErrors[id] = err.Error()
		} else {
			vm.Flash = flash
		}
		fragment(w, r, views.ShopModeration(vm))
	}
}

func (a *App) categoryMgmt(ctx context.Context, form views.CategoryFormState) views.CategoryMgmtVM {
	tree, err := a.api.ListCategories(ctx)
	return views.CategoryMgmtVM{Tree: tree, Form: form, LoadErr: errString(err)}
}

// adminCategoriesForm: GET — ?edit=<id> điền sẵn form sửa; không có → form tạo mới.
func (a *App) adminCategoriesForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vm := a.categoryMgmt(ctx, views.CategoryFormState{})
	if id := r.URL.Query().Get("edit"); id != "" {
		for _, fc := range views.Flatten(vm.Tree) {
			if fc.Node.ID == id {
				vm.Form = views.CategoryFormState{EditID: id, Name: fc.Node.Name, Slug: fc.Node.Slug,
					Description: fc.Node.Description, ParentID: deref(fc.Node.ParentID)}
			}
		}
	}
	fragment(w, r, views.CategoryManagement(vm))
}

func (a *App) adminCategorySave(w http.ResponseWriter, r *http.Request) {
	ctx, token := r.Context(), session(r).Token
	form := views.CategoryFormState{EditID: r.FormValue("id"), Name: r.FormValue("name"), Slug: r.FormValue("slug"),
		Description: r.FormValue("description"), ParentID: r.FormValue("parentId")}
	if strings.TrimSpace(form.Name) == "" || strings.TrimSpace(form.Slug) == "" {
		vm := a.categoryMgmt(ctx, form)
		vm.Error = i18n.Tc(ctx, "admin.name_slug_required")
		fragment(w, r, views.CategoryManagement(vm))
		return
	}
	payload := client.CategoryPayload{Name: strings.TrimSpace(form.Name), Slug: strings.TrimSpace(form.Slug),
		Description: strings.TrimSpace(form.Description)}
	if form.ParentID != "" {
		payload.ParentID = &form.ParentID
	}
	var c *client.CategoryNode
	var err error
	key := "admin.created"
	if form.EditID != "" {
		c, err = a.api.UpdateCategory(ctx, token, form.EditID, payload)
		key = "admin.updated"
	} else {
		c, err = a.api.CreateCategory(ctx, token, payload)
	}
	if err != nil {
		vm := a.categoryMgmt(ctx, form)
		vm.Error = err.Error()
		fragment(w, r, views.CategoryManagement(vm))
		return
	}
	vm := a.categoryMgmt(ctx, views.CategoryFormState{})
	vm.Flash = strings.Replace(i18n.Tc(ctx, key), "{}", c.Name, 1)
	fragment(w, r, views.CategoryManagement(vm))
}

func (a *App) adminCategoryDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	err := a.api.DeleteCategory(ctx, session(r).Token, r.PathValue("id"))
	vm := a.categoryMgmt(ctx, views.CategoryFormState{})
	if err != nil {
		vm.Error = err.Error()
	} else {
		vm.Flash = strings.Replace(i18n.Tc(ctx, "admin.deleted"), "{}", r.FormValue("name"), 1)
	}
	fragment(w, r, views.CategoryManagement(vm))
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
