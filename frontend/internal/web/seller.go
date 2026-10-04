package web

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/a-h/templ"

	"github.com/KernelStore/frontend/internal/client"
	"github.com/KernelStore/frontend/internal/i18n"
	"github.com/KernelStore/frontend/internal/views"
)

// sellerPage: /seller?tab=dashboard|sales|products|categories|settings.
// Chưa có shop → form mở shop.
func (a *App) sellerPage(w http.ResponseWriter, r *http.Request) {
	ctx, token := r.Context(), session(r).Token
	shop, err := a.api.GetMyShop(ctx, token)
	switch {
	case err != nil:
		page(w, r, views.SellerPage(views.SellerError(err.Error())))
		return
	case shop == nil:
		page(w, r, views.SellerPage(views.CreateShopForm(views.ShopFormState{})))
		return
	}

	section := r.URL.Query().Get("tab")
	if !slices.Contains([]string{"dashboard", "sales", "products", "categories", "settings"}, section) {
		section = "dashboard"
	}
	approved := shop.Status == "Approved"
	var content templ.Component
	switch {
	case section == "settings":
		content = views.ShopSettings(views.ShopFormState{Name: shop.Name, Slug: shop.Slug, Description: shop.Description}, "")
	case section == "products" && approved:
		content = views.ProductManager(a.productManager(ctx, token))
	case section == "products":
		content = views.NotApproved("seller.not_approved_products")
	case section == "sales" && approved:
		orders, err := a.api.ListSellerSales(ctx, token, "")
		content = views.SalesSection(orders, errString(err), "")
	case section == "sales":
		content = views.NotApproved("seller.not_approved_sales")
	case section == "categories" && approved:
		content = views.SellerCategoryManager(a.sellerCats(ctx, token, views.CategoryFormState{}))
	case section == "categories":
		content = views.NotApproved("seller.not_approved_cats")
	default:
		content = views.SellerDashboardSection(shop)
	}
	page(w, r, views.SellerPage(views.SellerLayout(shop, section, content)))
}

func (a *App) sellerDashboard(w http.ResponseWriter, r *http.Request) {
	s, err := a.api.SellerDashboard(r.Context(), session(r).Token)
	fragment(w, r, views.RevenueDashboard(s, errString(err)))
}

func (a *App) sellerSales(w http.ResponseWriter, r *http.Request) {
	orders, err := a.api.ListSellerSales(r.Context(), session(r).Token, r.URL.Query().Get("status"))
	fragment(w, r, views.SalesList(orders, errString(err)))
}

// createShop: mở shop xong thì làm mới token (role Customer → Seller) rồi về dashboard.
func (a *App) createShop(w http.ResponseWriter, r *http.Request) {
	ctx, token := r.Context(), session(r).Token
	f := views.ShopFormState{Name: r.FormValue("name"), Slug: r.FormValue("slug"), Description: r.FormValue("description")}
	if strings.TrimSpace(f.Name) == "" || strings.TrimSpace(f.Slug) == "" {
		f.Error = i18n.Tc(ctx, "seller.name_slug_required")
		fragment(w, r, views.CreateShopForm(f))
		return
	}
	if _, err := a.api.CreateShop(ctx, token, client.ShopPayload{Name: f.Name, Slug: f.Slug, Description: f.Description}); err != nil {
		f.Error = err.Error()
		fragment(w, r, views.CreateShopForm(f))
		return
	}
	if c, err := r.Cookie(refreshCookie); err == nil {
		if auth, err := a.api.Refresh(ctx, c.Value); err == nil {
			setAuthCookies(w, auth)
		}
	}
	redirect(w, r, "/seller?tab=dashboard")
}

func (a *App) updateShop(w http.ResponseWriter, r *http.Request) {
	ctx, token := r.Context(), session(r).Token
	f := views.ShopFormState{Name: r.FormValue("name"), Slug: r.FormValue("slug"), Description: r.FormValue("description")}
	if strings.TrimSpace(f.Name) == "" || strings.TrimSpace(f.Slug) == "" {
		f.Error = i18n.Tc(ctx, "seller.name_slug_required")
		fragment(w, r, views.ShopSettings(f, ""))
		return
	}
	shop, err := a.api.UpdateShop(ctx, token, client.ShopPayload{Name: f.Name, Slug: f.Slug, Description: f.Description})
	if err != nil {
		f.Error = err.Error()
		fragment(w, r, views.ShopSettings(f, ""))
		return
	}
	fragment(w, r, views.ShopSettings(views.ShopFormState{Name: shop.Name, Slug: shop.Slug, Description: shop.Description},
		i18n.Tc(ctx, "seller.saved")))
}

// ─── Sản phẩm ────────────────────────────────────────────────────────────────

func (a *App) productManager(ctx context.Context, token string) views.ProductManagerVM {
	vm := views.ProductManagerVM{EditForms: map[string]*views.ProductFormState{}, RowErrors: map[string]string{}}
	products, err := a.api.ListMyProducts(ctx, token)
	vm.Products, vm.Error = products, errString(err)
	// Category chung (dạng cây, thụt lề) + category riêng của shop (đánh dấu ★).
	if tree, err := a.api.ListCategories(ctx); err == nil {
		for _, fc := range views.Flatten(tree) {
			vm.Categories = append(vm.Categories, views.CategoryOption{ID: fc.Node.ID,
				Label: strings.Repeat("  ", fc.Depth) + fc.Node.Name})
		}
	}
	if mine, err := a.api.ListMyCategories(ctx, token); err == nil {
		for _, c := range mine {
			vm.Categories = append(vm.Categories, views.CategoryOption{ID: c.ID, Label: "★ " + c.Name})
		}
	}
	return vm
}

// saveProduct: tạo mới (không có {id}) hoặc cập nhật. Lỗi → mở lại đúng form kèm lỗi.
func (a *App) saveProduct(w http.ResponseWriter, r *http.Request) {
	ctx, token, id := r.Context(), session(r).Token, r.PathValue("id")
	f := views.ProductFormState{
		EditID: id, Name: r.FormValue("name"), Slug: r.FormValue("slug"), Description: r.FormValue("description"),
		Price: r.FormValue("price"), Sale: r.FormValue("sale"), Stock: r.FormValue("stock"), Sku: r.FormValue("sku"),
		Warranty: r.FormValue("warranty"), CategoryID: r.FormValue("categoryId"), Images: r.FormValue("images"),
		IsActive: r.FormValue("isActive") != "",
	}
	fail := func(msg string) {
		vm := a.productManager(ctx, token)
		f.Error = msg
		if id == "" {
			vm.NewForm = &f
		} else {
			vm.EditForms[id] = &f
		}
		fragment(w, r, views.ProductManager(vm))
	}
	if strings.TrimSpace(f.Name) == "" || strings.TrimSpace(f.Slug) == "" {
		fail(i18n.Tc(ctx, "seller.name_slug_required"))
		return
	}

	// Chuyển kiểu như bản Rust: số sai → 0, giá khuyến mãi rỗng/sai → không có.
	price, _ := strconv.ParseFloat(strings.TrimSpace(f.Price), 64)
	stock, _ := strconv.Atoi(strings.TrimSpace(f.Stock))
	warranty, _ := strconv.Atoi(strings.TrimSpace(f.Warranty))
	payload := client.ProductPayload{
		Name: f.Name, Slug: f.Slug, Description: f.Description, Price: price,
		SalePrice: parseFloat(strings.TrimSpace(f.Sale)), StockQuantity: stock, Sku: f.Sku,
		WarrantyMonths: warranty, IsActive: f.IsActive, Images: []string{},
	}
	if f.CategoryID != "" {
		payload.CategoryID = &f.CategoryID
	}
	for _, l := range strings.Split(f.Images, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			payload.Images = append(payload.Images, l)
		}
	}

	var err error
	if id == "" {
		_, err = a.api.CreateProduct(ctx, token, payload)
	} else {
		_, err = a.api.UpdateProduct(ctx, token, id, payload)
	}
	if err != nil {
		fail(err.Error())
		return
	}
	action := "update"
	if id == "" {
		action = "create"
	}
	if a.events != nil {
		a.events.Broadcast(Event{Type: "product-updated", Action: action, Slug: payload.Slug, ID: id})
	}
	w.Header().Set("HX-Trigger", "product-updated")
	fragment(w, r, views.ProductManager(a.productManager(ctx, token)))
}

func (a *App) deleteProduct(w http.ResponseWriter, r *http.Request) {
	ctx, token, id := r.Context(), session(r).Token, r.PathValue("id")
	err := a.api.DeleteProduct(ctx, token, id)
	vm := a.productManager(ctx, token)
	if err != nil {
		vm.RowErrors[id] = err.Error()
	} else if a.events != nil {
		a.events.Broadcast(Event{Type: "product-updated", Action: "delete", ID: id})
	}
	w.Header().Set("HX-Trigger", "product-updated")
	fragment(w, r, views.ProductManager(vm))
}

func (a *App) productManagerFragment(w http.ResponseWriter, r *http.Request) {
	ctx, token := r.Context(), session(r).Token
	fragment(w, r, views.ProductManager(a.productManager(ctx, token)))
}

// uploadImage: chuyển file ảnh lên backend; trả JSON {url} hoặc {error} cho app.js.
func (a *App) uploadImage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	file, header, err := r.FormFile("file")
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "network error: cannot attach file"})
		return
	}
	defer file.Close()
	url, err := a.api.UploadImage(r.Context(), session(r).Token, header.Filename, file)
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"url": url})
}

// ─── Category của shop ───────────────────────────────────────────────────────

func (a *App) sellerCats(ctx context.Context, token string, form views.CategoryFormState) views.SellerCategoriesVM {
	cats, err := a.api.ListMyCategories(ctx, token)
	return views.SellerCategoriesVM{Cats: cats, Form: form, LoadErr: errString(err)}
}

func (a *App) sellerCatsForm(w http.ResponseWriter, r *http.Request) {
	ctx, token := r.Context(), session(r).Token
	vm := a.sellerCats(ctx, token, views.CategoryFormState{})
	if id := r.URL.Query().Get("edit"); id != "" {
		for _, c := range vm.Cats {
			if c.ID == id {
				vm.Form = views.CategoryFormState{EditID: id, Name: c.Name, Slug: c.Slug, Description: c.Description}
			}
		}
	}
	fragment(w, r, views.SellerCategoryManager(vm))
}

func (a *App) sellerCatSave(w http.ResponseWriter, r *http.Request) {
	ctx, token := r.Context(), session(r).Token
	form := views.CategoryFormState{EditID: r.FormValue("id"), Name: r.FormValue("name"),
		Slug: r.FormValue("slug"), Description: r.FormValue("description")}
	if strings.TrimSpace(form.Name) == "" || strings.TrimSpace(form.Slug) == "" {
		vm := a.sellerCats(ctx, token, form)
		vm.Error = i18n.Tc(ctx, "admin.name_slug_required")
		fragment(w, r, views.SellerCategoryManager(vm))
		return
	}
	payload := client.CategoryPayload{Name: strings.TrimSpace(form.Name), Slug: strings.TrimSpace(form.Slug),
		Description: strings.TrimSpace(form.Description)}
	var c *client.CategoryNode
	var err error
	key := "admin.created"
	if form.EditID != "" {
		c, err = a.api.UpdateMyCategory(ctx, token, form.EditID, payload)
		key = "admin.updated"
	} else {
		c, err = a.api.CreateMyCategory(ctx, token, payload)
	}
	if err != nil {
		vm := a.sellerCats(ctx, token, form)
		vm.Error = err.Error()
		fragment(w, r, views.SellerCategoryManager(vm))
		return
	}
	vm := a.sellerCats(ctx, token, views.CategoryFormState{})
	vm.Flash = strings.Replace(i18n.Tc(ctx, key), "{}", c.Name, 1)
	fragment(w, r, views.SellerCategoryManager(vm))
}

func (a *App) sellerCatDelete(w http.ResponseWriter, r *http.Request) {
	ctx, token := r.Context(), session(r).Token
	err := a.api.DeleteMyCategory(ctx, token, r.PathValue("id"))
	vm := a.sellerCats(ctx, token, views.CategoryFormState{})
	if err != nil {
		vm.Error = err.Error()
	} else {
		vm.Flash = strings.Replace(i18n.Tc(ctx, "admin.deleted"), "{}", r.FormValue("name"), 1)
	}
	fragment(w, r, views.SellerCategoryManager(vm))
}

// productRow: mở/đóng form sửa của một sản phẩm (chỉ render lại dòng đó).
func (a *App) productRow(w http.ResponseWriter, r *http.Request) {
	ctx, token, id := r.Context(), session(r).Token, r.PathValue("id")
	vm := a.productManager(ctx, token)
	for i := range vm.Products {
		if p := &vm.Products[i]; p.ID == id {
			var edit *views.ProductFormState
			if r.URL.Query().Get("edit") == "1" {
				f := views.EditProductForm(p)
				edit = &f
			}
			fragment(w, r, views.ProductRow(p, vm.Categories, edit, ""))
			return
		}
	}
	fragment(w, r, views.ProductManager(vm)) // sản phẩm không còn → render lại cả danh sách
}
