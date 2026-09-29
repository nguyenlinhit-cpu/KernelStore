package handlers

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/KernelStore/backend/internal/dto"
	"github.com/KernelStore/backend/internal/httpx"
	"github.com/KernelStore/backend/internal/models"
	"github.com/KernelStore/backend/internal/repository"
	"github.com/KernelStore/backend/internal/services"
)

// CategoriesController: /api/categories.
func (h *Handler) registerCategories(rt *httpx.Router) {
	rt.Handle("GET /api/categories", httpx.Anonymous, h.listCategories)
	rt.Handle("GET /api/categories/{slug}", httpx.Anonymous, h.getCategoryBySlug)
	rt.Handle("POST /api/categories", httpx.Roles("Admin"), h.createCategory)
	rt.Handle("PUT /api/categories/{id}", httpx.Roles("Admin"), h.updateCategory)
	rt.Handle("DELETE /api/categories/{id}", httpx.Roles("Admin"), h.deleteCategory)
}

// listCategories: cây category global (do Admin quản lý); category của shop không hiện ở đây.
func (h *Handler) listCategories(w http.ResponseWriter, r *http.Request) {
	all, err := repository.ListGlobalCategories(r.Context(), h.db)
	if err != nil {
		serverError(w, r, err)
		return
	}
	dto.OK(w, buildCategoryTree(all), "OK")
}

// buildCategoryTree: node có cha nằm trong danh sách thì gắn vào cha, còn lại là gốc.
// Thứ tự con giữ theo thứ tự tên như danh sách đầu vào.
func buildCategoryTree(all []repository.CategoryWithCount) []*dto.CategoryDto {
	nodes := make(map[uuid.UUID]*dto.CategoryDto, len(all))
	for _, c := range all {
		nodes[c.ID] = &dto.CategoryDto{
			ID: c.ID, Name: c.Name, Slug: c.Slug, Description: c.Description,
			ParentID: c.ParentID, ProductCount: c.ProductCount, Children: []*dto.CategoryDto{},
		}
	}
	roots := []*dto.CategoryDto{}
	for _, c := range all {
		node := nodes[c.ID]
		if c.ParentID != nil {
			if parent, ok := nodes[*c.ParentID]; ok {
				parent.Children = append(parent.Children, node)
				continue
			}
		}
		roots = append(roots, node)
	}
	return roots
}

func (h *Handler) getCategoryBySlug(w http.ResponseWriter, r *http.Request) {
	c, err := repository.FindCategoryBySlug(r.Context(), h.db, r.PathValue("slug"))
	if err != nil {
		serverError(w, r, err)
		return
	}
	if c == nil {
		dto.NotFound(w, "Không tìm thấy danh mục")
		return
	}
	dto.OK(w, categoryDto(&c.Category, c.ProductCount), "OK")
}

func (h *Handler) createCategory(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateCategoryRequest
	if !httpx.BindJSON(w, r, &req) {
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
	if req.ParentID != nil {
		if ok, err := repository.CategoryExists(ctx, h.db, *req.ParentID); err != nil {
			serverError(w, r, err)
			return
		} else if !ok {
			dto.BadRequest(w, "Danh mục cha không tồn tại")
			return
		}
	}
	c := &models.Category{
		ID: services.NewUUID(), Name: req.Name, Slug: req.Slug,
		Description: req.Description, ParentID: req.ParentID,
	}
	if err := repository.InsertCategory(ctx, h.db, c); err != nil {
		serverError(w, r, err)
		return
	}
	dto.OK(w, categoryDto(c, 0), "Đã tạo danh mục.")
}

func (h *Handler) updateCategory(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateCategoryRequest
	if !httpx.BindJSON(w, r, &req) {
		return
	}
	ctx, id := r.Context(), httpx.PathGUID(r, "id")

	existing, err := repository.FindCategoryByID(ctx, h.db, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if existing == nil {
		dto.NotFound(w, "Không tìm thấy danh mục")
		return
	}
	if taken, err := repository.CategorySlugTaken(ctx, h.db, req.Slug, id); err != nil {
		serverError(w, r, err)
		return
	} else if taken {
		dto.BadRequest(w, "Slug đã được sử dụng")
		return
	}
	if req.ParentID != nil && *req.ParentID == id {
		dto.BadRequest(w, "Danh mục không thể là cha của chính nó")
		return
	}
	if req.ParentID != nil {
		if ok, err := repository.CategoryExists(ctx, h.db, *req.ParentID); err != nil {
			serverError(w, r, err)
			return
		} else if !ok {
			dto.BadRequest(w, "Danh mục cha không tồn tại")
			return
		}
	}
	c := existing.Category
	c.Name, c.Slug, c.Description, c.ParentID = req.Name, req.Slug, req.Description, req.ParentID
	if err := repository.UpdateCategory(ctx, h.db, &c); err != nil {
		serverError(w, r, err)
		return
	}
	dto.OK(w, categoryDto(&c, 0), "Đã cập nhật danh mục.")
}

func (h *Handler) deleteCategory(w http.ResponseWriter, r *http.Request) {
	ctx, id := r.Context(), httpx.PathGUID(r, "id")
	c, err := repository.FindCategoryByID(ctx, h.db, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if c == nil {
		dto.NotFound(w, "Không tìm thấy danh mục")
		return
	}
	if has, err := repository.CategoryHasChildren(ctx, h.db, id); err != nil {
		serverError(w, r, err)
		return
	} else if has {
		dto.BadRequest(w, "Danh mục có danh mục con. Không thể xóa.")
		return
	}
	if c.ProductCount > 0 {
		dto.BadRequest(w, "Danh mục đang chứa sản phẩm. Không thể xóa.")
		return
	}
	if err := repository.DeleteCategory(ctx, h.db, id); err != nil {
		serverError(w, r, err)
		return
	}
	dto.OK(w, nil, "Đã xóa danh mục.")
}

// categoryDto: API trả một category có Children = null (như ToDto của C#).
func categoryDto(c *models.Category, productCount int) dto.CategoryDto {
	return dto.CategoryDto{
		ID: c.ID, Name: c.Name, Slug: c.Slug, Description: c.Description,
		ParentID: c.ParentID, ProductCount: productCount,
	}
}
