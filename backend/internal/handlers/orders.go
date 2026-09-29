package handlers

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/KernelStore/backend/internal/dto"
	"github.com/KernelStore/backend/internal/httpx"
	"github.com/KernelStore/backend/internal/middleware"
	"github.com/KernelStore/backend/internal/models"
	"github.com/KernelStore/backend/internal/money"
	"github.com/KernelStore/backend/internal/repository"
	"github.com/KernelStore/backend/internal/services"
)

// OrdersController: /api/orders.
func (h *Handler) registerOrders(rt *httpx.Router) {
	sellerOrAdmin := httpx.Roles("Seller", "Admin")
	rt.Handle("POST /api/orders", httpx.Authenticated, h.createOrder)
	rt.Handle("GET /api/orders", httpx.Authenticated, h.listOrders)
	rt.Handle("GET /api/orders/sales", sellerOrAdmin, h.salesOrders)
	rt.Handle("GET /api/orders/{id}", httpx.Authenticated, h.getOrder)
	rt.Handle("PUT /api/orders/{id}/status", sellerOrAdmin, h.updateOrderStatus)
	rt.Handle("POST /api/orders/{id}/confirm-received", httpx.Authenticated, h.confirmReceived)
	rt.Handle("POST /api/orders/{id}/cancel", httpx.Authenticated, h.cancelOrder)
	rt.Handle("POST /api/orders/{id}/return", httpx.Authenticated, h.requestReturn)
	rt.Handle("POST /api/orders/{id}/return/{decision}", sellerOrAdmin, h.resolveReturn)
}

// apiError dùng trong transaction: trả về để rollback và ghi response lỗi nghiệp vụ.
type apiError struct {
	status int
	msg    string
	errs   []string
}

func (e *apiError) Error() string { return e.msg }

func fail(status int, msg string, errs ...string) error { return &apiError{status, msg, errs} }

// writeTxError: lỗi nghiệp vụ → response tương ứng; lỗi khác → 500.
func writeTxError(w http.ResponseWriter, r *http.Request, err error) {
	if ae, ok := errors.AsType[*apiError](err); ok {
		dto.Fail(w, ae.status, ae.msg, ae.errs...)
		return
	}
	serverError(w, r, err)
}

// createOrder: tạo địa chỉ + đơn + chi tiết, trừ kho và xoá giỏ trong MỘT transaction.
// Giá chốt = SalePrice ?? Price tại thời điểm đặt.
func (h *Handler) createOrder(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateOrderRequest
	if !httpx.BindJSON(w, r, &req) {
		return
	}
	ctx, uid := r.Context(), userID(r)

	cart, err := repository.ListCartRows(ctx, h.db, uid)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if len(cart) == 0 {
		dto.BadRequest(w, "Giỏ hàng trống")
		return
	}
	var problems []string
	for _, c := range cart {
		if !c.IsActive {
			problems = append(problems, "Sản phẩm không còn khả dụng")
			continue
		}
		if c.Quantity > c.StockQuantity {
			problems = append(problems, fmt.Sprintf("'%s' chỉ còn %d trong kho", c.Name, c.StockQuantity))
		}
	}
	if len(problems) > 0 {
		dto.BadRequest(w, "Không thể đặt hàng", problems...)
		return
	}

	now := services.Now()
	address := &models.Address{
		ID: services.NewUUID(), UserID: uid,
		FullName: strings.TrimSpace(req.FullName), Phone: strings.TrimSpace(req.Phone),
		Street: strings.TrimSpace(req.Street), Ward: strings.TrimSpace(req.Ward),
		District: strings.TrimSpace(req.District), City: strings.TrimSpace(req.City),
	}
	order := &models.Order{
		ID: services.NewUUID(), Status: models.OrderStatusPending,
		ShippingFee: money.Zero, Note: strings.TrimSpace(req.Note),
		CreatedAt: now, UserID: uid, AddressID: address.ID,
	}
	details := make([]models.OrderDetail, 0, len(cart))
	total := money.Zero
	for _, c := range cart {
		unit := effectivePrice(c.Price, c.SalePrice)
		line := unit.MulInt(c.Quantity)
		total = total.Add(line)
		details = append(details, models.OrderDetail{
			ID: services.NewUUID(), OrderID: order.ID, ProductID: c.ProductID,
			Quantity: c.Quantity, UnitPrice: unit, TotalPrice: line,
		})
	}
	order.TotalAmount = total.Add(order.ShippingFee)

	err = repository.InTx(ctx, h.db, func(tx pgx.Tx) error {
		if err := repository.InsertAddress(ctx, tx, address); err != nil {
			return err
		}
		code, err := generateCode("KS", func(c string) (bool, error) { return repository.OrderCodeExists(ctx, tx, c) })
		if err != nil {
			return err
		}
		order.OrderCode = code
		if err := repository.InsertOrder(ctx, tx, order); err != nil {
			return err
		}
		for i := range details {
			if err := repository.InsertOrderDetail(ctx, tx, &details[i]); err != nil {
				return err
			}
			// Trừ kho có điều kiện: nếu đơn khác vừa mua hết thì huỷ cả transaction.
			ok, err := repository.DecrementStock(ctx, tx, details[i].ProductID, details[i].Quantity)
			if err != nil {
				return err
			}
			if !ok {
				c := cart[i]
				fresh, err := repository.FindProductRow(ctx, tx, c.ProductID)
				if err != nil || fresh == nil {
					return cmp.Or(err, fail(http.StatusBadRequest, "Không thể đặt hàng", "Sản phẩm không còn khả dụng"))
				}
				return fail(http.StatusBadRequest, "Không thể đặt hàng",
					fmt.Sprintf("'%s' chỉ còn %d trong kho", c.Name, fresh.StockQuantity))
			}
		}
		for _, c := range cart {
			if err := repository.DeleteCartItem(ctx, tx, c.ID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		writeTxError(w, r, err)
		return
	}

	ids := make([]uuid.UUID, len(cart))
	for i, c := range cart {
		ids[i] = c.ProductID
	}
	images, err := repository.LoadProductImages(ctx, h.db, ids)
	if err != nil {
		serverError(w, r, err)
		return
	}
	items := make([]dto.OrderItemDto, len(details))
	for i, d := range details {
		c := cart[i]
		items[i] = dto.OrderItemDto{
			ID: d.ID, ProductID: d.ProductID, ProductName: c.Name, ProductSlug: c.Slug,
			ImageUrl: primaryImageURL(images[c.ProductID]), UnitPrice: d.UnitPrice,
			Quantity: d.Quantity, TotalPrice: d.TotalPrice, ShopID: c.ShopID, ShopName: c.ShopName,
		}
	}
	dto.OK(w, dto.OrderDto{
		ID: order.ID, OrderCode: order.OrderCode, Status: order.Status.String(),
		TotalAmount: order.TotalAmount, ShippingFee: order.ShippingFee, Note: order.Note,
		CreatedAt: order.CreatedAt, PaidAt: order.PaidAt, Address: addressDto(address),
		Items: items, ItemCount: sumQuantity(items),
		CanManage: false, // đơn vừa tạo là của người mua
	}, "Đặt hàng thành công")
}

// listOrders: admin thấy tất cả; seller thấy đơn mình mua + đơn bán của shop
// (đơn bán chỉ hiện phần hàng của shop); customer thấy đơn của mình.
func (h *Handler) listOrders(w http.ResponseWriter, r *http.Request) {
	ctx, uid := r.Context(), userID(r)
	isAdmin := middleware.IsInRole(ctx, "Admin")
	shopID, err := h.sellerShopID(ctx, isAdmin, uid)
	if err != nil {
		serverError(w, r, err)
		return
	}
	scope := repository.OrderScope{All: isAdmin, BuyerID: &uid, ShopID: shopID}
	orders, err := repository.LoadOrders(ctx, h.db, scope)
	if err != nil {
		serverError(w, r, err)
		return
	}
	out := make([]dto.OrderDto, 0, len(orders))
	for i := range orders {
		var sf *uuid.UUID
		if orders[i].UserID != uid {
			sf = shopID
		}
		out = append(out, orderDto(&orders[i], sf, isAdmin || sf != nil))
	}
	dto.OK(w, out, "OK")
}

// salesOrders: đơn BÁN của shop người gọi (chỉ phần hàng của shop), lọc theo trạng thái.
func (h *Handler) salesOrders(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	shop := h.myShop(w, r)
	if shop == nil {
		return
	}
	scope := repository.OrderScope{ShopID: &shop.ID}
	if s, ok := httpx.NewQuery(r).Get("status"); ok && strings.TrimSpace(s) != "" {
		if st, ok := models.ParseOrderStatus(s); ok && st.IsDefined() {
			scope.Status = &st
		}
	}
	orders, err := repository.LoadOrders(ctx, h.db, scope)
	if err != nil {
		serverError(w, r, err)
		return
	}
	out := make([]dto.OrderDto, 0, len(orders))
	for i := range orders {
		out = append(out, orderDto(&orders[i], &shop.ID, true))
	}
	dto.OK(w, out, "OK")
}

func (h *Handler) getOrder(w http.ResponseWriter, r *http.Request) {
	ctx, uid := r.Context(), userID(r)
	o, err := repository.LoadOrder(ctx, h.db, httpx.PathGUID(r, "id"), false)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if o == nil {
		dto.NotFound(w, "Không tìm thấy đơn hàng")
		return
	}
	isAdmin := middleware.IsInRole(ctx, "Admin")
	shopID, err := h.sellerShopID(ctx, isAdmin, uid)
	if err != nil {
		serverError(w, r, err)
		return
	}
	var sf *uuid.UUID
	switch {
	case isAdmin, o.UserID == uid: // xem đầy đủ
	case shopID != nil && orderHasShop(o, *shopID):
		sf = shopID // seller chỉ xem phần hàng của shop mình
	default:
		dto.Forbidden(w, "Bạn không có quyền xem đơn hàng này")
		return
	}
	dto.OK(w, orderDto(o, sf, isAdmin || sf != nil), "OK")
}

func (h *Handler) updateOrderStatus(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateOrderStatusRequest
	if !httpx.BindJSON(w, r, &req) {
		return
	}
	newStatus, ok := models.ParseOrderStatus(req.Status)
	if !ok || !newStatus.IsDefined() {
		dto.BadRequest(w, "Trạng thái không hợp lệ")
		return
	}
	ctx, uid := r.Context(), userID(r)
	isAdmin := middleware.IsInRole(ctx, "Admin")
	var result dto.OrderDto

	err := h.withOrder(r, func(tx pgx.Tx, o *repository.OrderFull) error {
		shopID, err := h.sellerShopID(ctx, isAdmin, uid)
		if err != nil {
			return err
		}
		var sf *uuid.UUID
		if !isAdmin {
			if shopID == nil || !orderHasShop(o, *shopID) {
				return fail(http.StatusForbidden, "Bạn không có quyền cập nhật đơn hàng này")
			}
			sf = shopID
		}
		// Đơn đã giao/kết thúc/đang trả hàng → không đổi qua dropdown
		// (Delivered do khách xác nhận; trả hàng đi qua các endpoint /return riêng).
		switch o.Status {
		case models.OrderStatusDelivered, models.OrderStatusCancelled,
			models.OrderStatusReturnRequested, models.OrderStatusReturned:
			return fail(http.StatusBadRequest, fmt.Sprintf("Đơn đã ở trạng thái '%s', không thể thay đổi", o.Status))
		}
		if !isAdmin && newStatus == models.OrderStatusDelivered {
			return fail(http.StatusBadRequest, "Trạng thái 'Delivered' do khách xác nhận khi nhận hàng")
		}
		if newStatus == models.OrderStatusReturnRequested || newStatus == models.OrderStatusReturned {
			return fail(http.StatusBadRequest, "Trạng thái trả hàng dùng các thao tác trả hàng riêng")
		}
		if newStatus == models.OrderStatusCancelled {
			if err := repository.RestoreOrderStock(ctx, tx, o.ID); err != nil {
				return err
			}
		}
		o.Status = newStatus
		if err := repository.SetOrderStatus(ctx, tx, o.ID, o.Status, o.PaidAt); err != nil {
			return err
		}
		result = orderDto(o, sf, isAdmin || sf != nil)
		return nil
	})
	if err != nil {
		writeTxError(w, r, err)
		return
	}
	dto.OK(w, result, "Đã cập nhật trạng thái đơn hàng")
}

// confirmReceived: khách xác nhận đã nhận hàng (Shipped → Delivered).
func (h *Handler) confirmReceived(w http.ResponseWriter, r *http.Request) {
	h.buyerTransition(w, r, "Bạn không có quyền xác nhận đơn hàng này", "Đã xác nhận nhận hàng",
		func(ctx context.Context, tx pgx.Tx, o *repository.OrderFull) error {
			if o.Status == models.OrderStatusDelivered {
				return fail(http.StatusBadRequest, "Đơn đã được xác nhận nhận hàng")
			}
			if o.Status != models.OrderStatusShipped {
				return fail(http.StatusBadRequest, "Chỉ xác nhận được khi đơn đã giao (Shipped)")
			}
			o.Status = models.OrderStatusDelivered
			if o.PaidAt == nil {
				now := services.Now()
				o.PaidAt = &now
			}
			return nil
		})
}

// cancelOrder: khách huỷ khi chưa giao (Pending/Confirmed/Processing) → hoàn kho.
func (h *Handler) cancelOrder(w http.ResponseWriter, r *http.Request) {
	h.buyerTransition(w, r, "Bạn không có quyền hủy đơn hàng này", "Đã hủy đơn hàng",
		func(ctx context.Context, tx pgx.Tx, o *repository.OrderFull) error {
			switch o.Status {
			case models.OrderStatusPending, models.OrderStatusConfirmed, models.OrderStatusProcessing:
			default:
				return fail(http.StatusBadRequest, "Chỉ hủy được đơn khi chưa giao (Pending/Confirmed/Processing)")
			}
			o.Status = models.OrderStatusCancelled
			return repository.RestoreOrderStock(ctx, tx, o.ID)
		})
}

// requestReturn: khách yêu cầu trả hàng sau khi đã nhận (Delivered → ReturnRequested).
func (h *Handler) requestReturn(w http.ResponseWriter, r *http.Request) {
	h.buyerTransition(w, r, "Bạn không có quyền trả đơn hàng này", "Đã gửi yêu cầu trả hàng",
		func(ctx context.Context, tx pgx.Tx, o *repository.OrderFull) error {
			if o.Status != models.OrderStatusDelivered {
				return fail(http.StatusBadRequest, "Chỉ yêu cầu trả hàng khi đơn đã nhận (Delivered)")
			}
			o.Status = models.OrderStatusReturnRequested
			return nil
		})
}

// buyerTransition: khung chung cho thao tác của chủ đơn — khoá đơn, kiểm chủ sở hữu,
// áp dụng thay đổi, lưu trạng thái, trả OrderDto (người mua không có quyền quản lý).
func (h *Handler) buyerTransition(w http.ResponseWriter, r *http.Request, forbiddenMsg, okMsg string,
	apply func(context.Context, pgx.Tx, *repository.OrderFull) error) {
	ctx, uid := r.Context(), userID(r)
	var result dto.OrderDto
	err := h.withOrder(r, func(tx pgx.Tx, o *repository.OrderFull) error {
		if o.UserID != uid {
			return fail(http.StatusForbidden, forbiddenMsg)
		}
		if err := apply(ctx, tx, o); err != nil {
			return err
		}
		if err := repository.SetOrderStatus(ctx, tx, o.ID, o.Status, o.PaidAt); err != nil {
			return err
		}
		result = orderDto(o, nil, false)
		return nil
	})
	if err != nil {
		writeTxError(w, r, err)
		return
	}
	dto.OK(w, result, okMsg)
}

// resolveReturn: seller/admin duyệt (→ Returned + hoàn kho) hoặc từ chối (→ Delivered).
func (h *Handler) resolveReturn(w http.ResponseWriter, r *http.Request) {
	decision := r.PathValue("decision")
	approve := strings.EqualFold(decision, "approve")
	if !approve && !strings.EqualFold(decision, "reject") {
		dto.BadRequest(w, "Hành động không hợp lệ (approve|reject)")
		return
	}
	ctx, uid := r.Context(), userID(r)
	isAdmin := middleware.IsInRole(ctx, "Admin")
	var result dto.OrderDto

	err := h.withOrder(r, func(tx pgx.Tx, o *repository.OrderFull) error {
		shopID, err := h.sellerShopID(ctx, isAdmin, uid)
		if err != nil {
			return err
		}
		if !isAdmin && (shopID == nil || !orderHasShop(o, *shopID)) {
			return fail(http.StatusForbidden, "Bạn không có quyền xử lý đơn hàng này")
		}
		if o.Status != models.OrderStatusReturnRequested {
			return fail(http.StatusBadRequest, "Đơn không ở trạng thái chờ trả hàng")
		}
		if approve {
			o.Status = models.OrderStatusReturned
			if err := repository.RestoreOrderStock(ctx, tx, o.ID); err != nil {
				return err
			}
		} else {
			o.Status = models.OrderStatusDelivered
		}
		if err := repository.SetOrderStatus(ctx, tx, o.ID, o.Status, o.PaidAt); err != nil {
			return err
		}
		var sf *uuid.UUID
		if !isAdmin {
			sf = shopID
		}
		result = orderDto(o, sf, true)
		return nil
	})
	if err != nil {
		writeTxError(w, r, err)
		return
	}
	msg := "Đã từ chối yêu cầu trả hàng"
	if approve {
		msg = "Đã duyệt trả hàng"
	}
	dto.OK(w, result, msg)
}

// withOrder mở transaction, khoá đơn {id} (FOR UPDATE) rồi gọi fn; không có đơn → 404.
// Khoá dòng giúp hai thao tác song song (vd. huỷ hai lần) không hoàn kho hai lần.
func (h *Handler) withOrder(r *http.Request, fn func(pgx.Tx, *repository.OrderFull) error) error {
	ctx := r.Context()
	return repository.InTx(ctx, h.db, func(tx pgx.Tx) error {
		o, err := repository.LoadOrder(ctx, tx, httpx.PathGUID(r, "id"), true)
		if err != nil {
			return err
		}
		if o == nil {
			return fail(http.StatusNotFound, "Không tìm thấy đơn hàng")
		}
		return fn(tx, o)
	})
}

// sellerShopID: shop của người gọi nếu token có role Seller (và không phải Admin).
func (h *Handler) sellerShopID(ctx context.Context, isAdmin bool, uid uuid.UUID) (*uuid.UUID, error) {
	if isAdmin || !middleware.IsInRole(ctx, "Seller") {
		return nil, nil
	}
	shop, err := repository.FindShopByOwner(ctx, h.db, uid)
	if err != nil || shop == nil {
		return nil, err
	}
	return &shop.ID, nil
}

func orderHasShop(o *repository.OrderFull, shopID uuid.UUID) bool {
	return slices.ContainsFunc(o.Details, func(d repository.OrderDetailRow) bool {
		return d.ProductShopID != nil && *d.ProductShopID == shopID
	})
}

// orderDto: shopFilter ≠ nil → chỉ phần hàng của shop, tổng tiền = tổng phần đó, phí ship = 0.
func orderDto(o *repository.OrderFull, shopFilter *uuid.UUID, canManage bool) dto.OrderDto {
	details := slices.Clone(o.Details)
	if shopFilter != nil {
		details = slices.DeleteFunc(details, func(d repository.OrderDetailRow) bool {
			return d.ProductShopID == nil || *d.ProductShopID != *shopFilter
		})
	}
	slices.SortStableFunc(details, func(a, b repository.OrderDetailRow) int {
		return strings.Compare(deref(a.ProductName), deref(b.ProductName))
	})

	items := make([]dto.OrderItemDto, 0, len(details))
	for _, d := range details {
		var shopID uuid.UUID
		if d.ProductShopID != nil {
			shopID = *d.ProductShopID
		}
		items = append(items, dto.OrderItemDto{
			ID: d.ID, ProductID: d.ProductID,
			ProductName: deref(d.ProductName), ProductSlug: deref(d.ProductSlug),
			ImageUrl: primaryImageURL(o.Images[d.ProductID]), UnitPrice: d.UnitPrice,
			Quantity: d.Quantity, TotalPrice: d.TotalPrice, ShopID: shopID, ShopName: d.ShopName,
		})
	}

	total, shipping := o.TotalAmount, o.ShippingFee
	if shopFilter != nil {
		total, shipping = money.Zero, money.Zero
		for _, it := range items {
			total = total.Add(it.TotalPrice)
		}
	}
	return dto.OrderDto{
		ID: o.ID, OrderCode: o.OrderCode, Status: o.Status.String(),
		TotalAmount: total, ShippingFee: shipping, Note: o.Note,
		CreatedAt: o.CreatedAt, PaidAt: o.PaidAt, Address: addressDto(o.Address),
		Items: items, ItemCount: sumQuantity(items), CanManage: canManage,
	}
}

func addressDto(a *models.Address) dto.OrderAddressDto {
	if a == nil {
		return dto.OrderAddressDto{}
	}
	return dto.OrderAddressDto{FullName: a.FullName, Phone: a.Phone, Street: a.Street,
		Ward: a.Ward, District: a.District, City: a.City}
}

func sumQuantity(items []dto.OrderItemDto) int {
	n := 0
	for _, it := range items {
		n += it.Quantity
	}
	return n
}

// generateCode sinh mã "<PREFIX>-yyyyMMdd-XXXX" (UTC, 4 số ngẫu nhiên), thử 10 lần cho
// khỏi trùng; cực hiếm mới rơi vào nhánh dự phòng dùng hậu tố từ UUID (dài 21 ký tự).
func generateCode(prefix string, taken func(string) (bool, error)) (string, error) {
	date := services.Now().Format("20060102")
	for range 10 {
		code := fmt.Sprintf("%s-%s-%04d", prefix, date, rand.IntN(10000))
		exists, err := taken(code)
		if err != nil {
			return "", err
		}
		if !exists {
			return code, nil
		}
	}
	return (prefix + "-" + date + "-" + strings.ReplaceAll(uuid.NewString(), "-", ""))[:21], nil
}
