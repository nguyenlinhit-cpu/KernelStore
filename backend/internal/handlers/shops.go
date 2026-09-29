package handlers

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/KernelStore/backend/internal/dto"
	"github.com/KernelStore/backend/internal/httpx"
	"github.com/KernelStore/backend/internal/identity"
	"github.com/KernelStore/backend/internal/models"
	"github.com/KernelStore/backend/internal/repository"
	"github.com/KernelStore/backend/internal/services"
)

// ShopsController: /api/shops.
func (h *Handler) registerShops(rt *httpx.Router) {
	rt.Handle("POST /api/shops", httpx.Authenticated, h.createShop)
	rt.Handle("GET /api/shops/me", httpx.Authenticated, h.getMyShop)
	rt.Handle("PUT /api/shops/me", httpx.Authenticated, h.updateMyShop)
}

func (h *Handler) createShop(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateShopRequest
	if !httpx.BindJSON(w, r, &req) {
		return
	}
	ctx, uid := r.Context(), userID(r)

	user, err := repository.FindUserByID(ctx, h.db, uid)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if user == nil {
		dto.NotFound(w, "Không tìm thấy người dùng")
		return
	}
	if owned, err := repository.ShopOwnedBy(ctx, h.db, uid); err != nil {
		serverError(w, r, err)
		return
	} else if owned {
		dto.BadRequest(w, "Bạn đã có shop rồi")
		return
	}
	if taken, err := repository.ShopSlugTaken(ctx, h.db, req.Slug, uuid.Nil); err != nil {
		serverError(w, r, err)
		return
	} else if taken {
		dto.BadRequest(w, "Slug đã được sử dụng")
		return
	}

	shop := &models.Shop{
		ID:          services.NewUUID(),
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
		LogoUrl:     "",
		Status:      models.ShopStatusPending,
		OwnerID:     uid,
		CreatedAt:   services.Now(),
	}
	err = repository.InTx(ctx, h.db, func(tx pgx.Tx) error {
		if err := repository.InsertShop(ctx, tx, shop); err != nil {
			return err
		}
		// Mở shop = trở thành seller: Customer → Seller (cột Role + role Identity).
		// Token hiện tại vẫn mang role cũ; client phải refresh/đăng nhập lại.
		if user.Role != models.UserRoleCustomer {
			return nil
		}
		if err := repository.UpdateUserRoleColumn(ctx, tx, uid, models.UserRoleSeller, identity.NewConcurrencyStamp()); err != nil {
			return err
		}
		if _, err := repository.AddToRole(ctx, tx, uid, "SELLER"); err != nil {
			return err
		}
		return repository.RemoveFromRole(ctx, tx, uid, "CUSTOMER")
	})
	if err != nil {
		serverError(w, r, err)
		return
	}

	dto.OK(w, shopDto(shop, user.UserName), "Đã gửi yêu cầu mở shop. Chờ admin duyệt.")
}

func (h *Handler) getMyShop(w http.ResponseWriter, r *http.Request) {
	shop, err := repository.FindShopByOwner(r.Context(), h.db, userID(r))
	if err != nil {
		serverError(w, r, err)
		return
	}
	if shop == nil {
		dto.OK(w, nil, "Chưa có shop")
		return
	}
	dto.OK(w, shopDtoOf(shop), "OK")
}

func (h *Handler) updateMyShop(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateShopRequest
	if !httpx.BindJSON(w, r, &req) {
		return
	}
	ctx := r.Context()

	shop, err := repository.FindShopByOwner(ctx, h.db, userID(r))
	if err != nil {
		serverError(w, r, err)
		return
	}
	if shop == nil {
		dto.NotFound(w, "Bạn chưa có shop")
		return
	}
	if taken, err := repository.ShopSlugTaken(ctx, h.db, req.Slug, shop.ID); err != nil {
		serverError(w, r, err)
		return
	} else if taken {
		dto.BadRequest(w, "Slug đã được sử dụng")
		return
	}
	if err := repository.UpdateShopInfo(ctx, h.db, shop.ID, req.Name, req.Slug, req.Description); err != nil {
		serverError(w, r, err)
		return
	}
	shop.Name, shop.Slug, shop.Description = req.Name, req.Slug, req.Description
	dto.OK(w, shopDtoOf(shop), "Đã cập nhật thông tin shop.")
}

func shopDtoOf(s *repository.ShopWithOwner) dto.ShopDto {
	return shopDto(&s.Shop, s.OwnerName)
}

func shopDto(s *models.Shop, ownerName *string) dto.ShopDto {
	return dto.ShopDto{
		ID:          s.ID,
		Name:        s.Name,
		Slug:        s.Slug,
		Description: s.Description,
		LogoUrl:     s.LogoUrl,
		Status:      s.Status.String(),
		CreatedAt:   s.CreatedAt,
		OwnerID:     s.OwnerID,
		OwnerName:   deref(ownerName),
	}
}
