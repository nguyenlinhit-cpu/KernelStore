package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/KernelStore/backend/internal/dto"
	"github.com/KernelStore/backend/internal/httpx"
	"github.com/KernelStore/backend/internal/middleware"
	"github.com/KernelStore/backend/internal/models"
	"github.com/KernelStore/backend/internal/repository"
	"github.com/KernelStore/backend/internal/services"
)

// WarrantyController: /api/warranty — khách gửi yêu cầu bảo hành, shop/admin xử lý.
// Vòng đời: Pending → Approved → Processing → Completed; Pending → Rejected / Cancelled.
func (h *Handler) registerWarranty(rt *httpx.Router) {
	manage := httpx.Roles("Seller", "Admin")
	rt.Handle("POST /api/warranty", httpx.Authenticated, h.createWarranty)
	rt.Handle("GET /api/warranty/mine", httpx.Authenticated, h.myWarranty)
	rt.Handle("GET /api/warranty/shop", manage, h.shopWarranty)
	rt.Handle("GET /api/warranty/{id}", httpx.Authenticated, h.getWarranty)
	rt.Handle("POST /api/warranty/{id}/cancel", httpx.Authenticated, h.cancelWarranty)
	rt.Handle("POST /api/warranty/{id}/approve", manage, h.approveWarranty)
	rt.Handle("POST /api/warranty/{id}/reject", manage, h.rejectWarranty)
	rt.Handle("POST /api/warranty/{id}/process", manage, h.processWarranty)
	rt.Handle("POST /api/warranty/{id}/complete", manage, h.completeWarranty)
}

// createWarranty: chỉ khi đơn đã Delivered, sản phẩm có bảo hành, còn hạn
// (tính từ PaidAt ?? CreatedAt) và dòng hàng chưa có yêu cầu nào đang mở.
func (h *Handler) createWarranty(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateWarrantyClaimRequest
	if !httpx.BindJSON(w, r, &req) {
		return
	}
	ctx, uid := r.Context(), userID(r)

	d, err := repository.FindWarrantyDetail(ctx, h.db, req.OrderDetailID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if d == nil {
		dto.NotFound(w, "Không tìm thấy sản phẩm trong đơn hàng")
		return
	}
	if d.OrderUserID != uid {
		dto.Forbidden(w, "Bạn không có quyền bảo hành sản phẩm này")
		return
	}
	if d.OrderStatus != models.OrderStatusDelivered {
		dto.BadRequest(w, "Chỉ bảo hành khi đơn đã nhận hàng (Delivered)")
		return
	}
	if d.WarrantyMonths <= 0 {
		dto.BadRequest(w, "Sản phẩm này không có chính sách bảo hành")
		return
	}
	expires := addMonths(deliveredAt(d.OrderPaidAt, d.OrderCreatedAt), d.WarrantyMonths)
	if services.Now().After(expires) {
		dto.BadRequest(w, fmt.Sprintf("Sản phẩm đã hết hạn bảo hành (%s)", expires.Format("02/01/2006")))
		return
	}
	if open, err := repository.HasOpenWarrantyClaim(ctx, h.db, d.DetailID); err != nil {
		serverError(w, r, err)
		return
	} else if open {
		dto.BadRequest(w, "Đã có một yêu cầu bảo hành đang xử lý cho sản phẩm này")
		return
	}

	claim := &models.WarrantyClaim{
		ID: services.NewUUID(), Description: strings.TrimSpace(req.Description),
		ImageUrl: strings.TrimSpace(req.ImageUrl), Status: models.WarrantyStatusPending,
		Resolution: models.WarrantyResolutionNone, CreatedAt: services.Now(),
		OrderDetailID: d.DetailID, UserID: uid, ProductID: d.ProductID, ShopID: d.ShopID,
	}
	err = repository.InTx(ctx, h.db, func(tx pgx.Tx) error {
		code, err := generateCode("WR", func(c string) (bool, error) { return repository.WarrantyCodeExists(ctx, tx, c) })
		if err != nil {
			return err
		}
		claim.ClaimCode = code
		return repository.InsertWarrantyClaim(ctx, tx, claim)
	})
	if err != nil {
		serverError(w, r, err)
		return
	}
	row, err := repository.FindWarrantyClaim(ctx, h.db, claim.ID, false)
	if err != nil || row == nil {
		serverError(w, r, fmt.Errorf("đọc lại yêu cầu bảo hành: %w", err))
		return
	}
	h.writeWarranty(w, r, row, false, "Đã gửi yêu cầu bảo hành")
}

func (h *Handler) myWarranty(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	h.writeWarrantyList(w, r, repository.WarrantyScope{UserID: &uid}, false)
}

// shopWarranty: seller thấy yêu cầu gửi tới shop mình, admin thấy toàn hệ thống.
func (h *Handler) shopWarranty(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var scope repository.WarrantyScope
	if !middleware.IsInRole(ctx, "Admin") {
		shopID, err := h.warrantyShopID(ctx, userID(r))
		if err != nil {
			serverError(w, r, err)
			return
		}
		if shopID == nil {
			dto.NotFound(w, "Bạn chưa có shop")
			return
		}
		scope.ShopID = shopID
	}
	if s, ok := httpx.NewQuery(r).Get("status"); ok && strings.TrimSpace(s) != "" {
		if st, ok := models.ParseWarrantyStatus(s); ok && st.IsDefined() {
			scope.Status = &st
		}
	}
	h.writeWarrantyList(w, r, scope, true)
}

func (h *Handler) getWarranty(w http.ResponseWriter, r *http.Request) {
	ctx, uid := r.Context(), userID(r)
	claim, err := repository.FindWarrantyClaim(ctx, h.db, httpx.PathGUID(r, "id"), false)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if claim == nil {
		dto.NotFound(w, "Không tìm thấy yêu cầu bảo hành")
		return
	}
	isAdmin := middleware.IsInRole(ctx, "Admin")
	shopID, err := h.warrantyShopID(ctx, uid)
	if err != nil {
		serverError(w, r, err)
		return
	}
	isOwnerSeller := shopID != nil && claim.ShopID == *shopID
	if claim.UserID != uid && !isAdmin && !isOwnerSeller {
		dto.Forbidden(w, "Bạn không có quyền xem yêu cầu này")
		return
	}
	h.writeWarranty(w, r, claim, isAdmin || isOwnerSeller, "OK")
}

// cancelWarranty: khách tự huỷ khi yêu cầu còn Pending.
func (h *Handler) cancelWarranty(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	h.warrantyTransition(w, r, false, "Đã hủy yêu cầu bảo hành", func(c *repository.WarrantyRow) error {
		if c.UserID != uid {
			return fail(http.StatusForbidden, "Bạn không có quyền hủy yêu cầu này")
		}
		if c.Status != models.WarrantyStatusPending {
			return fail(http.StatusBadRequest, "Chỉ hủy được khi yêu cầu đang chờ xử lý")
		}
		c.Status = models.WarrantyStatusCancelled
		return nil
	})
}

// approveWarranty: Pending → Approved kèm hình thức xử lý (Repair/Replace/Refund).
func (h *Handler) approveWarranty(w http.ResponseWriter, r *http.Request) {
	var req dto.ApproveWarrantyRequest
	if !httpx.BindJSON(w, r, &req) {
		return
	}
	resolution, ok := models.ParseWarrantyResolution(req.Resolution)
	if !ok || resolution == models.WarrantyResolutionNone {
		dto.BadRequest(w, "Hình thức xử lý không hợp lệ (Repair/Replace/Refund)")
		return
	}
	h.warrantyTransition(w, r, true, "Đã chấp nhận bảo hành", func(c *repository.WarrantyRow) error {
		if c.Status != models.WarrantyStatusPending {
			return fail(http.StatusBadRequest, "Chỉ duyệt được yêu cầu đang chờ xử lý")
		}
		c.Status, c.Resolution, c.ResolutionNote = models.WarrantyStatusApproved, resolution, strings.TrimSpace(req.Note)
		return nil
	})
}

// rejectWarranty: Pending → Rejected.
func (h *Handler) rejectWarranty(w http.ResponseWriter, r *http.Request) {
	var req dto.WarrantyNoteRequest
	if !httpx.BindJSON(w, r, &req) {
		return
	}
	h.warrantyTransition(w, r, true, "Đã từ chối yêu cầu bảo hành", func(c *repository.WarrantyRow) error {
		if c.Status != models.WarrantyStatusPending {
			return fail(http.StatusBadRequest, "Chỉ từ chối được yêu cầu đang chờ xử lý")
		}
		c.Status, c.Resolution, c.ResolutionNote = models.WarrantyStatusRejected, models.WarrantyResolutionNone, strings.TrimSpace(req.Note)
		now := services.Now()
		c.ResolvedAt = &now
		return nil
	})
}

// processWarranty: Approved → Processing.
func (h *Handler) processWarranty(w http.ResponseWriter, r *http.Request) {
	h.warrantyTransition(w, r, true, "Đã bắt đầu xử lý bảo hành", func(c *repository.WarrantyRow) error {
		if c.Status != models.WarrantyStatusApproved {
			return fail(http.StatusBadRequest, "Chỉ chuyển xử lý khi yêu cầu đã được chấp nhận")
		}
		c.Status = models.WarrantyStatusProcessing
		return nil
	})
}

// completeWarranty: Approved/Processing → Completed.
func (h *Handler) completeWarranty(w http.ResponseWriter, r *http.Request) {
	var req dto.WarrantyNoteRequest
	if !httpx.BindJSON(w, r, &req) {
		return
	}
	h.warrantyTransition(w, r, true, "Đã hoàn tất bảo hành", func(c *repository.WarrantyRow) error {
		if c.Status != models.WarrantyStatusApproved && c.Status != models.WarrantyStatusProcessing {
			return fail(http.StatusBadRequest, "Chỉ hoàn tất khi yêu cầu đã được chấp nhận")
		}
		c.Status = models.WarrantyStatusCompleted
		if strings.TrimSpace(req.Note) != "" {
			c.ResolutionNote = strings.TrimSpace(req.Note)
		}
		now := services.Now()
		c.ResolvedAt = &now
		return nil
	})
}

// warrantyTransition: khoá yêu cầu {id} trong transaction, (nếu manage) kiểm quyền shop/admin,
// áp dụng thay đổi, đặt UpdatedAt rồi lưu.
func (h *Handler) warrantyTransition(w http.ResponseWriter, r *http.Request, manage bool, okMsg string,
	apply func(*repository.WarrantyRow) error) {
	ctx, uid := r.Context(), userID(r)
	var claim *repository.WarrantyRow
	err := repository.InTx(ctx, h.db, func(tx pgx.Tx) error {
		c, err := repository.FindWarrantyClaim(ctx, tx, httpx.PathGUID(r, "id"), true)
		if err != nil {
			return err
		}
		if c == nil {
			return fail(http.StatusNotFound, "Không tìm thấy yêu cầu bảo hành")
		}
		if manage && !middleware.IsInRole(ctx, "Admin") {
			shopID, err := h.warrantyShopID(ctx, uid)
			if err != nil {
				return err
			}
			if shopID == nil || c.ShopID != *shopID {
				return fail(http.StatusForbidden, "Bạn không có quyền xử lý yêu cầu này")
			}
		}
		if err := apply(c); err != nil {
			return err
		}
		now := services.Now()
		c.UpdatedAt = &now
		claim = c
		return repository.UpdateWarrantyState(ctx, tx, &c.WarrantyClaim)
	})
	if err != nil {
		writeTxError(w, r, err)
		return
	}
	h.writeWarranty(w, r, claim, manage, okMsg)
}

// warrantyShopID: shop của người gọi nếu token có role Seller.
func (h *Handler) warrantyShopID(ctx context.Context, uid uuid.UUID) (*uuid.UUID, error) {
	if !middleware.IsInRole(ctx, "Seller") {
		return nil, nil
	}
	shop, err := repository.FindShopByOwner(ctx, h.db, uid)
	if err != nil || shop == nil {
		return nil, err
	}
	return &shop.ID, nil
}

func (h *Handler) writeWarrantyList(w http.ResponseWriter, r *http.Request, scope repository.WarrantyScope, canManage bool) {
	ctx := r.Context()
	claims, err := repository.ListWarrantyClaims(ctx, h.db, scope)
	if err != nil {
		serverError(w, r, err)
		return
	}
	images, err := h.warrantyImages(ctx, claims)
	if err != nil {
		serverError(w, r, err)
		return
	}
	out := make([]dto.WarrantyClaimDto, 0, len(claims))
	for i := range claims {
		out = append(out, warrantyDto(&claims[i], images, canManage))
	}
	dto.OK(w, out, "OK")
}

func (h *Handler) writeWarranty(w http.ResponseWriter, r *http.Request, c *repository.WarrantyRow, canManage bool, msg string) {
	images, err := h.warrantyImages(r.Context(), []repository.WarrantyRow{*c})
	if err != nil {
		serverError(w, r, err)
		return
	}
	dto.OK(w, warrantyDto(c, images, canManage), msg)
}

func (h *Handler) warrantyImages(ctx context.Context, claims []repository.WarrantyRow) (map[uuid.UUID][]models.ProductImage, error) {
	ids := make([]uuid.UUID, len(claims))
	for i, c := range claims {
		ids[i] = c.ProductID
	}
	return repository.LoadProductImages(ctx, h.db, ids)
}

func warrantyDto(c *repository.WarrantyRow, images map[uuid.UUID][]models.ProductImage, canManage bool) dto.WarrantyClaimDto {
	months := 0
	if c.ProductWarranty != nil {
		months = *c.ProductWarranty
	}
	var expires *time.Time
	if c.OrderCreatedAt != nil && months > 0 {
		e := addMonths(deliveredAt(c.OrderPaidAt, *c.OrderCreatedAt), months)
		expires = &e
	}
	var imageURL *string
	if c.ImageUrl != "" {
		imageURL = &c.ImageUrl
	}
	var orderID uuid.UUID
	if c.OrderID != nil {
		orderID = *c.OrderID
	}
	quantity := 0
	if c.DetailQuantity != nil {
		quantity = *c.DetailQuantity
	}
	return dto.WarrantyClaimDto{
		ID: c.ID, ClaimCode: c.ClaimCode, Status: c.Status.String(), Resolution: c.Resolution.String(),
		ResolutionNote: c.ResolutionNote, Description: c.Description, ImageUrl: imageURL,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt, ResolvedAt: c.ResolvedAt,
		OrderDetailID: c.OrderDetailID, OrderID: orderID, OrderCode: deref(c.OrderCode),
		ProductID: c.ProductID, ProductName: deref(c.ProductName), ProductSlug: deref(c.ProductSlug),
		ProductImageUrl: primaryImageURL(images[c.ProductID]), Quantity: quantity,
		WarrantyMonths: months, WarrantyExpiresAt: expires,
		ShopID: c.ShopID, ShopName: c.ShopName, UserID: c.UserID, UserName: deref(c.UserFullName),
		CanManage: canManage,
	}
}

// deliveredAt = PaidAt ?? CreatedAt (mốc tính hạn bảo hành).
func deliveredAt(paidAt *time.Time, createdAt time.Time) time.Time {
	if paidAt != nil {
		return *paidAt
	}
	return createdAt
}

// addMonths giống DateTime.AddMonths của .NET: nếu ngày không tồn tại ở tháng đích thì
// lấy ngày cuối tháng (31/01 + 1 tháng = 28/02), thay vì tràn sang tháng sau như time.AddDate.
func addMonths(t time.Time, months int) time.Time {
	y, m, d := t.Date()
	firstOfTarget := time.Date(y, m+time.Month(months), 1, 0, 0, 0, 0, t.Location())
	lastDay := firstOfTarget.AddDate(0, 1, -1).Day()
	hh, mm, ss := t.Clock()
	return time.Date(firstOfTarget.Year(), firstOfTarget.Month(), min(d, lastDay), hh, mm, ss, t.Nanosecond(), t.Location())
}
