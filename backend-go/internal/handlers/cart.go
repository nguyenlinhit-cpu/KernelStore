package handlers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/KernelStore/backend-go/internal/dto"
	"github.com/KernelStore/backend-go/internal/httpx"
	"github.com/KernelStore/backend-go/internal/models"
	"github.com/KernelStore/backend-go/internal/money"
	"github.com/KernelStore/backend-go/internal/repository"
	"github.com/KernelStore/backend-go/internal/services"
)

// CartController: /api/cart.
func (h *Handler) registerCart(rt *httpx.Router) {
	rt.Handle("GET /api/cart", httpx.Authenticated, h.getCart)
	rt.Handle("POST /api/cart", httpx.Authenticated, h.addToCart)
	rt.Handle("PUT /api/cart/{productId}", httpx.Authenticated, h.updateCartItem)
	rt.Handle("DELETE /api/cart/{productId}", httpx.Authenticated, h.deleteCartItem)
}

func (h *Handler) getCart(w http.ResponseWriter, r *http.Request) {
	h.writeCart(w, r, "OK")
}

func (h *Handler) addToCart(w http.ResponseWriter, r *http.Request) {
	var req dto.AddToCartRequest
	if !httpx.BindJSON(w, r, &req) {
		return
	}
	ctx, uid := r.Context(), userID(r)

	product := h.activeProduct(w, r, req.ProductID)
	if product == nil {
		return
	}
	item, err := repository.FindCartItem(ctx, h.db, uid, product.ID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	newQty := req.Quantity
	if item != nil {
		newQty += item.Quantity
	}
	if newQty > product.StockQuantity {
		dto.BadRequest(w, fmt.Sprintf("Số lượng vượt quá tồn kho (còn %d)", product.StockQuantity))
		return
	}
	if item == nil {
		err = repository.InsertCartItem(ctx, h.db, &models.CartItem{
			ID: services.NewUUID(), UserID: uid, ProductID: product.ID, Quantity: req.Quantity,
		})
	} else {
		err = repository.UpdateCartItemQuantity(ctx, h.db, item.ID, newQty)
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	h.writeCart(w, r, "Đã thêm vào giỏ hàng")
}

// updateCartItem: quantity ≤ 0 → xoá khỏi giỏ.
func (h *Handler) updateCartItem(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateCartItemRequest
	if !httpx.BindJSON(w, r, &req) {
		return
	}
	ctx, productID := r.Context(), httpx.PathGUID(r, "productId")
	item := h.cartItem(w, r, productID)
	if item == nil {
		return
	}
	if req.Quantity <= 0 {
		if err := repository.DeleteCartItem(ctx, h.db, item.ID); err != nil {
			serverError(w, r, err)
			return
		}
		h.writeCart(w, r, "Đã xóa khỏi giỏ hàng")
		return
	}
	product := h.activeProduct(w, r, productID)
	if product == nil {
		return
	}
	if req.Quantity > product.StockQuantity {
		dto.BadRequest(w, fmt.Sprintf("Số lượng vượt quá tồn kho (còn %d)", product.StockQuantity))
		return
	}
	if err := repository.UpdateCartItemQuantity(ctx, h.db, item.ID, req.Quantity); err != nil {
		serverError(w, r, err)
		return
	}
	h.writeCart(w, r, "Đã cập nhật giỏ hàng")
}

func (h *Handler) deleteCartItem(w http.ResponseWriter, r *http.Request) {
	item := h.cartItem(w, r, httpx.PathGUID(r, "productId"))
	if item == nil {
		return
	}
	if err := repository.DeleteCartItem(r.Context(), h.db, item.ID); err != nil {
		serverError(w, r, err)
		return
	}
	h.writeCart(w, r, "Đã xóa khỏi giỏ hàng")
}

// cartItem: dòng giỏ hàng của người gọi cho sản phẩm; không có → 404.
func (h *Handler) cartItem(w http.ResponseWriter, r *http.Request, productID uuid.UUID) *models.CartItem {
	item, err := repository.FindCartItem(r.Context(), h.db, userID(r), productID)
	if err != nil {
		serverError(w, r, err)
		return nil
	}
	if item == nil {
		dto.NotFound(w, "Sản phẩm không có trong giỏ hàng")
	}
	return item
}

// activeProduct: sản phẩm đang bán; không có hoặc đã ẩn → 404.
func (h *Handler) activeProduct(w http.ResponseWriter, r *http.Request, id uuid.UUID) *repository.ProductRow {
	p, err := repository.FindProductRow(r.Context(), h.db, id)
	if err != nil {
		serverError(w, r, err)
		return nil
	}
	if p == nil || !p.IsActive {
		dto.NotFound(w, "Không tìm thấy sản phẩm")
		return nil
	}
	return p
}

func (h *Handler) writeCart(w http.ResponseWriter, r *http.Request, msg string) {
	cart, err := h.buildCart(r.Context(), userID(r))
	if err != nil {
		serverError(w, r, err)
		return
	}
	dto.OK(w, cart, msg)
}

// buildCart: đơn giá = SalePrice ?? Price, thành tiền = đơn giá × số lượng.
func (h *Handler) buildCart(ctx context.Context, uid uuid.UUID) (*dto.CartDto, error) {
	rows, err := repository.ListCartRows(ctx, h.db, uid)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(rows))
	for i, c := range rows {
		ids[i] = c.ProductID
	}
	images, err := repository.LoadProductImages(ctx, h.db, ids)
	if err != nil {
		return nil, err
	}

	cart := &dto.CartDto{Items: make([]dto.CartItemDto, 0, len(rows)), Subtotal: money.Zero}
	for _, c := range rows {
		unit := effectivePrice(c.Price, c.SalePrice)
		line := unit.MulInt(c.Quantity)
		cart.Items = append(cart.Items, dto.CartItemDto{
			ID: c.ID, ProductID: c.ProductID, Name: c.Name, Slug: c.Slug,
			Price: c.Price, SalePrice: c.SalePrice, UnitPrice: unit,
			Quantity: c.Quantity, StockQuantity: c.StockQuantity, LineTotal: line,
			ImageUrl: primaryImageURL(images[c.ProductID]), ShopID: c.ShopID, ShopName: c.ShopName,
		})
		cart.TotalItems += c.Quantity
		cart.Subtotal = cart.Subtotal.Add(line)
	}
	return cart, nil
}

// effectivePrice = SalePrice ?? Price.
func effectivePrice(price money.Money, sale *money.Money) money.Money {
	if sale != nil {
		return *sale
	}
	return price
}
