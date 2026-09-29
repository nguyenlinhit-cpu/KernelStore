package handlers

import (
	"context"
	"math"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/KernelStore/backend/internal/dto"
	"github.com/KernelStore/backend/internal/httpx"
	"github.com/KernelStore/backend/internal/models"
	"github.com/KernelStore/backend/internal/repository"
	"github.com/KernelStore/backend/internal/services"
)

// ProductsController: /api/products.
func (h *Handler) registerProducts(rt *httpx.Router) {
	rt.Handle("GET /api/products", httpx.Anonymous, h.listProducts)
	rt.Handle("GET /api/products/featured", httpx.Anonymous, h.featuredProducts)
	rt.Handle("GET /api/products/my", httpx.Authenticated, h.myProducts)
	rt.Handle("GET /api/products/{slug}", httpx.Anonymous, h.productDetail)
	rt.Handle("POST /api/products", httpx.Roles("Seller"), h.createProduct)
	rt.Handle("PUT /api/products/{id}", httpx.Roles("Seller"), h.updateProduct)
	rt.Handle("DELETE /api/products/{id}", httpx.Roles("Seller"), h.deleteProduct)
}

func (h *Handler) listProducts(w http.ResponseWriter, r *http.Request) {
	q := httpx.NewQuery(r)
	category, shop := q.String("category"), q.String("shop")
	minPrice, maxPrice := q.Money("minPrice"), q.Money("maxPrice")
	search, sort := q.String("search"), q.String("sort")
	page, pageSize := q.Int("page", 1), q.Int("pageSize", 12)
	if q.Failed(w) {
		return
	}
	page = max(1, page)
	pageSize = min(max(pageSize, 1), 50)
	ctx := r.Context()

	empty := func() {
		dto.OK(w, dto.PagedResult[dto.ProductDto]{Page: page, PageSize: pageSize, Items: []dto.ProductDto{}}, "OK")
	}
	f := repository.ProductFilter{MinPrice: minPrice, MaxPrice: maxPrice, Sort: sort,
		Offset: (page - 1) * pageSize, Limit: pageSize}

	if strings.TrimSpace(category) != "" {
		cat, err := h.resolveCategory(ctx, category)
		if err != nil {
			serverError(w, r, err)
			return
		}
		if cat == nil {
			empty()
			return
		}
		if f.CategoryIDs, err = repository.CategoryAndDescendantIDs(ctx, h.db, cat.ID); err != nil {
			serverError(w, r, err)
			return
		}
	}
	if strings.TrimSpace(shop) != "" {
		s, err := repository.FindShopBySlug(ctx, h.db, shop, models.ShopStatusApproved)
		if err != nil {
			serverError(w, r, err)
			return
		}
		if s == nil {
			empty()
			return
		}
		f.ShopID = &s.ID
	}
	if strings.TrimSpace(search) != "" {
		f.Search = strings.ToLower(strings.TrimSpace(search))
	}

	rows, total, err := repository.ListActiveProducts(ctx, h.db, f)
	if err != nil {
		serverError(w, r, err)
		return
	}
	items, err := h.productDtos(ctx, rows)
	if err != nil {
		serverError(w, r, err)
		return
	}
	dto.OK(w, dto.PagedResult[dto.ProductDto]{
		Page: page, PageSize: pageSize, Total: total,
		TotalPages: (total + pageSize - 1) / pageSize,
		Items:      items,
	}, "OK")
}

// resolveCategory: tham số category là id hoặc slug.
func (h *Handler) resolveCategory(ctx context.Context, s string) (*repository.CategoryWithCount, error) {
	if id, ok := httpx.ParseGUID(s); ok {
		return repository.FindCategoryByID(ctx, h.db, id)
	}
	return repository.FindCategoryBySlug(ctx, h.db, s)
}

func (h *Handler) featuredProducts(w http.ResponseWriter, r *http.Request) {
	q := httpx.NewQuery(r)
	take := q.Int("take", 8)
	if q.Failed(w) {
		return
	}
	rows, err := repository.FeaturedProducts(r.Context(), h.db, min(max(take, 1), 50))
	if err != nil {
		serverError(w, r, err)
		return
	}
	items, err := h.productDtos(r.Context(), rows)
	if err != nil {
		serverError(w, r, err)
		return
	}
	dto.OK(w, items, "OK")
}

func (h *Handler) productDetail(w http.ResponseWriter, r *http.Request) {
	ctx, slug := r.Context(), r.PathValue("slug")
	slugID, _ := httpx.ParseGUID(slug)

	p, err := repository.FindActiveProductBySlugOrID(ctx, h.db, slug, slugID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if p == nil {
		dto.NotFound(w, "Không tìm thấy sản phẩm")
		return
	}
	reviews, err := repository.ListProductReviews(ctx, h.db, p.ID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	productCount, err := repository.CountActiveShopProducts(ctx, h.db, p.ShopID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	shop, err := repository.FindShopByID(ctx, h.db, p.ShopID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	images, err := repository.LoadProductImages(ctx, h.db, []uuid.UUID{p.ID})
	if err != nil {
		serverError(w, r, err)
		return
	}

	reviewDtos, avg := reviewSummary(reviews)
	summary := dto.ShopSummaryDto{ID: p.ShopID, ProductCount: productCount}
	if shop != nil {
		summary.Name, summary.Slug, summary.Description, summary.LogoUrl = shop.Name, shop.Slug, shop.Description, shop.LogoUrl
	}
	dto.OK(w, dto.ProductDetailDto{
		ID: p.ID, Name: p.Name, Slug: p.Slug, Description: p.Description,
		Price: p.Price, SalePrice: p.SalePrice, StockQuantity: p.StockQuantity,
		Sku: p.Sku, WarrantyMonths: p.WarrantyMonths, CreatedAt: p.CreatedAt,
		ShopID: p.ShopID, ShopName: p.ShopName, CategoryID: p.CategoryID, CategoryName: p.CategoryName,
		Images:  imageDtos(images[p.ID]),
		Reviews: reviewDtos, Shop: summary, AverageRating: avg, ReviewCount: len(reviewDtos),
	}, "OK")
}

// reviewSummary: danh sách ReviewDto + điểm trung bình làm tròn 2 chữ số theo
// Math.Round của .NET (làm tròn về số chẵn khi đúng nửa).
func reviewSummary(reviews []repository.ReviewRow) ([]dto.ReviewDto, float64) {
	out := make([]dto.ReviewDto, 0, len(reviews))
	sum := 0
	for _, rv := range reviews {
		out = append(out, dto.ReviewDto{ID: rv.ID, Rating: rv.Rating, Comment: rv.Comment,
			CreatedAt: rv.CreatedAt, UserID: rv.UserID, UserName: deref(rv.UserFullName)})
		sum += rv.Rating
	}
	if len(out) == 0 {
		return out, 0
	}
	return out, math.RoundToEven(float64(sum)/float64(len(out))*100) / 100
}

// myProducts: sản phẩm của shop mình (kể cả shop chưa duyệt); chưa có shop → [].
func (h *Handler) myProducts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	shop, err := repository.FindShopByOwner(ctx, h.db, userID(r))
	if err != nil {
		serverError(w, r, err)
		return
	}
	if shop == nil {
		dto.OK(w, []dto.ProductDto{}, "Chưa có shop")
		return
	}
	rows, err := repository.ListShopProducts(ctx, h.db, shop.ID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	items, err := h.productDtos(ctx, rows)
	if err != nil {
		serverError(w, r, err)
		return
	}
	dto.OK(w, items, "OK")
}

func (h *Handler) createProduct(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateProductRequest
	if !httpx.BindJSON(w, r, &req) {
		return
	}
	ctx := r.Context()

	// Guard: chỉ shop đã Approved mới được thêm sản phẩm.
	shop, err := repository.FindShopByOwner(ctx, h.db, userID(r))
	if err != nil {
		serverError(w, r, err)
		return
	}
	if shop == nil || shop.Status != models.ShopStatusApproved {
		dto.BadRequest(w, "Bạn chưa có shop hoặc shop chưa được duyệt")
		return
	}
	if taken, err := repository.ProductSlugTaken(ctx, h.db, req.Slug); err != nil {
		serverError(w, r, err)
		return
	} else if taken {
		dto.BadRequest(w, "Slug đã được sử dụng")
		return
	}
	if !h.checkCategory(w, r, req.CategoryID) {
		return
	}

	p := &models.Product{
		ID: services.NewUUID(), Name: req.Name, Slug: req.Slug, Description: req.Description,
		Price: req.Price, SalePrice: req.SalePrice, StockQuantity: req.StockQuantity,
		Sku: req.Sku, WarrantyMonths: req.WarrantyMonths, CategoryID: req.CategoryID,
		ShopID: shop.ID, IsActive: true, CreatedAt: services.Now(),
	}
	err = repository.InTx(ctx, h.db, func(tx pgx.Tx) error {
		if err := repository.InsertProduct(ctx, tx, p); err != nil {
			return err
		}
		return repository.InsertProductImages(ctx, tx, buildImages(p, req.Images))
	})
	if err != nil {
		serverError(w, r, err)
		return
	}
	h.writeProduct(w, r, p.ID, "Đã thêm sản phẩm")
}

func (h *Handler) updateProduct(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateProductRequest
	if !httpx.BindJSON(w, r, &req) {
		return
	}
	existing := h.loadOwnProduct(w, r)
	if existing == nil {
		return
	}
	ctx := r.Context()
	if existing.Slug != req.Slug {
		if taken, err := repository.ProductSlugTaken(ctx, h.db, req.Slug); err != nil {
			serverError(w, r, err)
			return
		} else if taken {
			dto.BadRequest(w, "Slug đã được sử dụng")
			return
		}
	}
	if !h.checkCategory(w, r, req.CategoryID) {
		return
	}

	p := existing.Product
	p.Name, p.Slug, p.Description = req.Name, req.Slug, req.Description
	p.Price, p.SalePrice, p.StockQuantity = req.Price, req.SalePrice, req.StockQuantity
	p.Sku, p.WarrantyMonths, p.CategoryID, p.IsActive = req.Sku, req.WarrantyMonths, req.CategoryID, req.IsActive

	err := repository.InTx(ctx, h.db, func(tx pgx.Tx) error {
		if err := repository.UpdateProduct(ctx, tx, &p); err != nil {
			return err
		}
		// C#: có ảnh cũ hoặc gửi ảnh mới → thay toàn bộ gallery (gửi [] = xoá hết ảnh).
		oldCount, err := repository.CountProductImages(ctx, tx, p.ID)
		if err != nil || (oldCount == 0 && len(req.Images) == 0) {
			return err
		}
		if err := repository.DeleteProductImages(ctx, tx, p.ID); err != nil {
			return err
		}
		return repository.InsertProductImages(ctx, tx, buildImages(&p, req.Images))
	})
	if err != nil {
		serverError(w, r, err)
		return
	}
	h.writeProduct(w, r, p.ID, "Đã cập nhật sản phẩm")
}

func (h *Handler) deleteProduct(w http.ResponseWriter, r *http.Request) {
	p := h.loadOwnProduct(w, r)
	if p == nil {
		return
	}
	if err := repository.DeleteProduct(r.Context(), h.db, p.ID); err != nil {
		serverError(w, r, err)
		return
	}
	dto.OK(w, nil, "Đã xóa sản phẩm")
}

// loadOwnProduct: sản phẩm {id} phải thuộc shop đã Approved của người gọi.
func (h *Handler) loadOwnProduct(w http.ResponseWriter, r *http.Request) *repository.ProductRow {
	ctx := r.Context()
	shop, err := repository.FindShopByOwner(ctx, h.db, userID(r))
	if err != nil {
		serverError(w, r, err)
		return nil
	}
	if shop == nil {
		dto.BadRequest(w, "Bạn chưa có shop")
		return nil
	}
	if shop.Status != models.ShopStatusApproved {
		dto.BadRequest(w, "Bạn chưa có shop hoặc shop chưa được duyệt")
		return nil
	}
	p, err := repository.FindShopProduct(ctx, h.db, httpx.PathGUID(r, "id"), shop.ID)
	if err != nil {
		serverError(w, r, err)
		return nil
	}
	if p == nil {
		dto.NotFound(w, "Không tìm thấy sản phẩm")
	}
	return p
}

// checkCategory: categoryId (nếu có) phải tồn tại — bất kỳ category nào, global hay của shop.
func (h *Handler) checkCategory(w http.ResponseWriter, r *http.Request, id *uuid.UUID) bool {
	if id == nil {
		return true
	}
	ok, err := repository.CategoryExists(r.Context(), h.db, *id)
	if err != nil {
		serverError(w, r, err)
		return false
	}
	if !ok {
		dto.BadRequest(w, "Danh mục không tồn tại")
	}
	return ok
}

// buildImages: bỏ URL rỗng, tối đa 10 ảnh, ảnh đầu là primary, AltText = tên sản phẩm.
func buildImages(p *models.Product, urls []string) []models.ProductImage {
	var out []models.ProductImage
	for _, u := range urls {
		if u = strings.TrimSpace(u); u == "" {
			continue
		}
		if len(out) == 10 {
			break
		}
		out = append(out, models.ProductImage{
			ID: services.NewUUID(), Url: u, AltText: p.Name,
			IsPrimary: len(out) == 0, DisplayOrder: len(out), ProductID: p.ID,
		})
	}
	return out
}

// writeProduct đọc lại sản phẩm từ DB rồi trả ProductDto (C#: ToDtoAsync).
func (h *Handler) writeProduct(w http.ResponseWriter, r *http.Request, id uuid.UUID, msg string) {
	row, err := repository.FindProductRow(r.Context(), h.db, id)
	if err == nil && row == nil {
		err = pgx.ErrNoRows
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	items, err := h.productDtos(r.Context(), []repository.ProductRow{*row})
	if err != nil {
		serverError(w, r, err)
		return
	}
	dto.OK(w, items[0], msg)
}

// productDtos gắn ảnh (nạp một lượt) vào danh sách sản phẩm.
func (h *Handler) productDtos(ctx context.Context, rows []repository.ProductRow) ([]dto.ProductDto, error) {
	ids := make([]uuid.UUID, len(rows))
	for i, p := range rows {
		ids[i] = p.ID
	}
	images, err := repository.LoadProductImages(ctx, h.db, ids)
	if err != nil {
		return nil, err
	}
	out := make([]dto.ProductDto, 0, len(rows))
	for _, p := range rows {
		out = append(out, dto.ProductDto{
			ID: p.ID, Name: p.Name, Slug: p.Slug, Description: p.Description,
			Price: p.Price, SalePrice: p.SalePrice, StockQuantity: p.StockQuantity,
			Sku: p.Sku, WarrantyMonths: p.WarrantyMonths, IsActive: p.IsActive, CreatedAt: p.CreatedAt,
			ShopID: p.ShopID, ShopName: p.ShopName, CategoryID: p.CategoryID, CategoryName: p.CategoryName,
			Images: imageDtos(images[p.ID]),
		})
	}
	return out, nil
}

func imageDtos(images []models.ProductImage) []dto.ProductImageDto {
	out := make([]dto.ProductImageDto, 0, len(images))
	for _, i := range images {
		out = append(out, dto.ProductImageDto{ID: i.ID, Url: i.Url, AltText: i.AltText,
			IsPrimary: i.IsPrimary, DisplayOrder: i.DisplayOrder})
	}
	return out
}

// primaryImageURL: ảnh đại diện = ảnh primary, rồi theo DisplayOrder (null nếu không có ảnh).
func primaryImageURL(images []models.ProductImage) *string {
	var best *models.ProductImage
	for i := range images {
		img := &images[i]
		if best == nil || (img.IsPrimary && !best.IsPrimary) ||
			(img.IsPrimary == best.IsPrimary && img.DisplayOrder < best.DisplayOrder) {
			best = img
		}
	}
	if best == nil {
		return nil
	}
	return &best.Url
}
