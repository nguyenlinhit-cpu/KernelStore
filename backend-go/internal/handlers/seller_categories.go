package handlers

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/KernelStore/backend-go/internal/dto"
	"github.com/KernelStore/backend-go/internal/httpx"
	"github.com/KernelStore/backend-go/internal/models"
	"github.com/KernelStore/backend-go/internal/repository"
	"github.com/KernelStore/backend-go/internal/services"
)

// SellerCategoriesController: /api/seller/categories — category phẳng riêng của shop.
func (h *Handler) registerSellerCategories(rt *httpx.Router) {
	seller := httpx.Roles("Seller")
	rt.Handle("GET /api/seller/categories", seller, h.listMyCategories)
	rt.Handle("POST /api/seller/categories", seller, h.createMyCategory)
	rt.Handle("PUT /api/seller/categories/{id}", seller, h.updateMyCategory)
	rt.Handle("DELETE /api/seller/categories/{id}", seller, h.deleteMyCategory)
}

// myShop trả shop của người gọi; chưa có → 404 "Bạn chưa có shop" và nil.
func (h *Handler) myShop(w http.ResponseWriter, r *http.Request) *repository.ShopWithOwner {
	shop, err := repository.FindShopByOwner(r.Context(), h.db, userID(r))
	if err != nil {
		serverError(w, r, err)
		return nil
	}
	if shop == nil {
		dto.NotFound(w, "Bạn chưa có shop")
	}
	return shop
}

func (h *Handler) listMyCategories(w http.ResponseWriter, r *http.Request) {
	shop := h.myShop(w, r)
	if shop == nil {
		return
	}
	cats, err := repository.ListShopCategories(r.Context(), h.db, shop.ID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	out := make([]dto.CategoryDto, 0, len(cats))
	for i := range cats {
		out = append(out, categoryDto(&cats[i].Category, cats[i].ProductCount))
	}
	dto.OK(w, out, "OK")
}

func (h *Handler) createMyCategory(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateCategoryRequest
	if !httpx.BindJSON(w, r, &req) {
		return
	}
	shop := h.myShop(w, r)
	if shop == nil {
		return
	}
	ctx := r.Context()
	if taken, err := repository.CategorySlugTaken(ctx, h.db, req.Slug, uuid.Nil); err != nil {
		serverError(w, r, err)
		return
	} else if taken {
		dto.BadRequest(w, "Slug đã được sử dụng")
		return
	}
	c := &models.Category{
		ID: services.NewUUID(), Name: req.Name, Slug: req.Slug, Description: req.Description,
		ParentID:    nil, // category của shop luôn phẳng
		OwnerShopID: &shop.ID,
	}
	if err := repository.InsertCategory(ctx, h.db, c); err != nil {
		serverError(w, r, err)
		return
	}
	dto.OK(w, categoryDto(c, 0), "Đã tạo danh mục.")
}

// loadMyCategory đọc category {id} và kiểm nó thuộc shop; sai → 404.
func (h *Handler) loadMyCategory(w http.ResponseWriter, r *http.Request, shopID uuid.UUID) *repository.CategoryWithCount {
	c, err := repository.FindCategoryByID(r.Context(), h.db, httpx.PathGUID(r, "id"))
	if err != nil {
		serverError(w, r, err)
		return nil
	}
	if c == nil || c.OwnerShopID == nil || *c.OwnerShopID != shopID {
		dto.NotFound(w, "Không tìm thấy danh mục của shop")
		return nil
	}
	return c
}

func (h *Handler) updateMyCategory(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateCategoryRequest
	if !httpx.BindJSON(w, r, &req) {
		return
	}
	shop := h.myShop(w, r)
	if shop == nil {
		return
	}
	existing := h.loadMyCategory(w, r, shop.ID)
	if existing == nil {
		return
	}
	ctx := r.Context()
	if taken, err := repository.CategorySlugTaken(ctx, h.db, req.Slug, existing.ID); err != nil {
		serverError(w, r, err)
		return
	} else if taken {
		dto.BadRequest(w, "Slug đã được sử dụng")
		return
	}
	c := existing.Category
	c.Name, c.Slug, c.Description = req.Name, req.Slug, req.Description // ParentId giữ nguyên
	if err := repository.UpdateCategory(ctx, h.db, &c); err != nil {
		serverError(w, r, err)
		return
	}
	dto.OK(w, categoryDto(&c, 0), "Đã cập nhật danh mục.")
}

// deleteMyCategory: gỡ category khỏi các sản phẩm thay vì chặn xoá.
func (h *Handler) deleteMyCategory(w http.ResponseWriter, r *http.Request) {
	shop := h.myShop(w, r)
	if shop == nil {
		return
	}
	c := h.loadMyCategory(w, r, shop.ID)
	if c == nil {
		return
	}
	ctx := r.Context()
	err := repository.InTx(ctx, h.db, func(tx pgx.Tx) error {
		if err := repository.DetachProductsFromCategory(ctx, tx, c.ID); err != nil {
			return err
		}
		return repository.DeleteCategory(ctx, tx, c.ID)
	})
	if err != nil {
		serverError(w, r, err)
		return
	}
	dto.OK(w, nil, "Đã xóa danh mục.")
}
