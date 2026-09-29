package handlers

import (
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/KernelStore/backend-go/internal/dto"
	"github.com/KernelStore/backend-go/internal/httpx"
	"github.com/KernelStore/backend-go/internal/identity"
	"github.com/KernelStore/backend-go/internal/models"
	"github.com/KernelStore/backend-go/internal/repository"
)

// AdminShopsController: /api/admin/shops (chỉ Admin).
func (h *Handler) registerAdminShops(rt *httpx.Router) {
	admin := httpx.Roles("Admin")
	rt.Handle("GET /api/admin/shops", admin, h.adminListShops)
	rt.Handle("POST /api/admin/shops/{id}/approve", admin, h.adminApproveShop)
	rt.Handle("POST /api/admin/shops/{id}/reject", admin, h.adminRejectShop)
	rt.Handle("POST /api/admin/shops/{id}/ban", admin, h.adminBanShop)
	rt.Handle("POST /api/admin/shops/{id}/unban", admin, h.adminUnbanShop)
	rt.Handle("DELETE /api/admin/shops/{id}", admin, h.adminDeleteShop)
}

func (h *Handler) adminListShops(w http.ResponseWriter, r *http.Request) {
	var filter *models.ShopStatus
	if s := r.URL.Query().Get("status"); s != "" {
		if st, ok := models.ParseShopStatus(s); ok {
			filter = &st
		}
	}
	shops, err := repository.ListShops(r.Context(), h.db, filter)
	if err != nil {
		serverError(w, r, err)
		return
	}
	out := make([]dto.ShopDto, 0, len(shops))
	for i := range shops {
		out = append(out, shopDtoOf(&shops[i]))
	}
	dto.OK(w, out, "OK")
}

// loadShop đọc shop theo {id}; không có → 404 và trả nil.
func (h *Handler) loadShop(w http.ResponseWriter, r *http.Request) *repository.ShopWithOwner {
	shop, err := repository.FindShopByID(r.Context(), h.db, httpx.PathGUID(r, "id"))
	if err != nil {
		serverError(w, r, err)
		return nil
	}
	if shop == nil {
		dto.NotFound(w, "Không tìm thấy shop")
	}
	return shop
}

func (h *Handler) adminApproveShop(w http.ResponseWriter, r *http.Request) {
	shop := h.loadShop(w, r)
	if shop == nil {
		return
	}
	if shop.Status == models.ShopStatusApproved {
		dto.BadRequest(w, "Shop đã được duyệt rồi")
		return
	}
	ctx := r.Context()
	err := repository.InTx(ctx, h.db, func(tx pgx.Tx) error {
		if err := repository.SetShopStatus(ctx, tx, shop.ID, models.ShopStatusApproved); err != nil {
			return err
		}
		// C#: owner.Role = Seller; AddToRoleAsync(owner, "Seller"). Cột Role chỉ được lưu
		// khi AddToRole thành công (đã có role Seller → Identity báo lỗi, không lưu gì).
		added, err := repository.AddToRole(ctx, tx, shop.OwnerID, "SELLER")
		if err != nil || !added {
			return err
		}
		return repository.UpdateUserRoleColumn(ctx, tx, shop.OwnerID, models.UserRoleSeller, identity.NewConcurrencyStamp())
	})
	if err != nil {
		serverError(w, r, err)
		return
	}
	shop.Status = models.ShopStatusApproved
	dto.OK(w, shopDtoOf(shop), "Đã duyệt shop. Người dùng trở thành Seller.")
}

func (h *Handler) adminRejectShop(w http.ResponseWriter, r *http.Request) {
	shop := h.loadShop(w, r)
	if shop == nil {
		return
	}
	if shop.Status != models.ShopStatusPending {
		dto.BadRequest(w, "Chỉ shop đang chờ duyệt mới có thể từ chối")
		return
	}
	if err := repository.SetShopStatus(r.Context(), h.db, shop.ID, models.ShopStatusRejected); err != nil {
		serverError(w, r, err)
		return
	}
	shop.Status = models.ShopStatusRejected
	dto.OK(w, shopDtoOf(shop), "Đã từ chối shop")
}

// adminBanShop: ban tạm thời shop vi phạm → Banned, ẩn toàn bộ sản phẩm.
func (h *Handler) adminBanShop(w http.ResponseWriter, r *http.Request) {
	shop := h.loadShop(w, r)
	if shop == nil {
		return
	}
	switch shop.Status {
	case models.ShopStatusDeleted:
		dto.BadRequest(w, "Shop đã bị xóa vĩnh viễn")
		return
	case models.ShopStatusBanned:
		dto.BadRequest(w, "Shop đang bị ban")
		return
	}
	if err := h.setShopStatusAndProducts(r, shop, models.ShopStatusBanned, false); err != nil {
		serverError(w, r, err)
		return
	}
	dto.OK(w, shopDtoOf(shop), "Đã tạm ban shop vi phạm. Sản phẩm đã bị ẩn khỏi cửa hàng.")
}

// adminUnbanShop: gỡ ban tạm thời → Approved, hiện lại sản phẩm.
func (h *Handler) adminUnbanShop(w http.ResponseWriter, r *http.Request) {
	shop := h.loadShop(w, r)
	if shop == nil {
		return
	}
	if shop.Status != models.ShopStatusBanned {
		dto.BadRequest(w, "Chỉ gỡ ban được shop đang bị ban")
		return
	}
	if err := h.setShopStatusAndProducts(r, shop, models.ShopStatusApproved, true); err != nil {
		serverError(w, r, err)
		return
	}
	dto.OK(w, shopDtoOf(shop), "Đã gỡ ban shop. Sản phẩm đã hiển thị trở lại.")
}

// adminDeleteShop: ban vĩnh viễn. Chưa có đơn → xoá cứng shop + sản phẩm;
// đã có đơn → xoá mềm (Deleted) + ẩn sản phẩm, giữ nguyên lịch sử đơn hàng.
func (h *Handler) adminDeleteShop(w http.ResponseWriter, r *http.Request) {
	shop := h.loadShop(w, r)
	if shop == nil {
		return
	}
	if shop.Status == models.ShopStatusDeleted {
		dto.BadRequest(w, "Shop đã bị xóa vĩnh viễn")
		return
	}
	ctx := r.Context()
	hasOrders, err := repository.ShopHasOrders(ctx, h.db, shop.ID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if !hasOrders {
		if err := repository.InTx(ctx, h.db, func(tx pgx.Tx) error {
			return repository.DeleteShopHard(ctx, tx, shop.ID)
		}); err != nil {
			serverError(w, r, err)
			return
		}
		dto.OK(w, nil, "Đã xóa vĩnh viễn shop và toàn bộ sản phẩm.")
		return
	}
	if err := h.setShopStatusAndProducts(r, shop, models.ShopStatusDeleted, false); err != nil {
		serverError(w, r, err)
		return
	}
	dto.OK(w, shopDtoOf(shop), "Đã ban vĩnh viễn (xóa) shop. Sản phẩm bị gỡ; lịch sử đơn hàng được giữ lại.")
}

// setShopStatusAndProducts đổi trạng thái shop và bật/tắt sản phẩm trong một transaction.
func (h *Handler) setShopStatusAndProducts(r *http.Request, shop *repository.ShopWithOwner, status models.ShopStatus, active bool) error {
	ctx := r.Context()
	err := repository.InTx(ctx, h.db, func(tx pgx.Tx) error {
		if err := repository.SetShopStatus(ctx, tx, shop.ID, status); err != nil {
			return err
		}
		return repository.SetShopProductsActive(ctx, tx, shop.ID, active)
	})
	if err == nil {
		shop.Status = status
	}
	return err
}
