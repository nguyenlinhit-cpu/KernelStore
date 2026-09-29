package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/KernelStore/backend/internal/models"
)

// ShopWithOwner = Shop kèm Owner.UserName (C#: Include(s => s.Owner)).
type ShopWithOwner struct {
	models.Shop
	OwnerName *string `db:"OwnerName"`
}

const shopWithOwnerSelect = `SELECT s."Id", s."Name", s."Slug", s."Description", s."LogoUrl",
	s."Status", s."CreatedAt", s."OwnerId", u."UserName" AS "OwnerName"
	FROM "Shops" s LEFT JOIN "AspNetUsers" u ON u."Id" = s."OwnerId"`

func FindShopByOwner(ctx context.Context, db DBTX, ownerID uuid.UUID) (*ShopWithOwner, error) {
	return one[ShopWithOwner](ctx, db, shopWithOwnerSelect+` WHERE s."OwnerId" = $1 LIMIT 1`, ownerID)
}

func FindShopByID(ctx context.Context, db DBTX, id uuid.UUID) (*ShopWithOwner, error) {
	return one[ShopWithOwner](ctx, db, shopWithOwnerSelect+` WHERE s."Id" = $1`, id)
}

// FindShopBySlug trả shop có slug + trạng thái chỉ định (dùng cho lọc sản phẩm theo shop).
func FindShopBySlug(ctx context.Context, db DBTX, slug string, status models.ShopStatus) (*ShopWithOwner, error) {
	return one[ShopWithOwner](ctx, db, shopWithOwnerSelect+` WHERE s."Slug" = $1 AND s."Status" = $2 LIMIT 1`, slug, status)
}

// ListShops: status = nil → tất cả; sắp xếp mới nhất trước.
func ListShops(ctx context.Context, db DBTX, status *models.ShopStatus) ([]ShopWithOwner, error) {
	rows, err := db.Query(ctx, shopWithOwnerSelect+` WHERE ($1::int IS NULL OR s."Status" = $1)
		ORDER BY s."CreatedAt" DESC`, status)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[ShopWithOwner])
}

func ShopOwnedBy(ctx context.Context, db DBTX, ownerID uuid.UUID) (bool, error) {
	return exists(ctx, db, `SELECT 1 FROM "Shops" WHERE "OwnerId" = $1`, ownerID)
}

// ShopSlugTaken kiểm slug đã dùng bởi shop khác (exclude = uuid.Nil khi tạo mới).
func ShopSlugTaken(ctx context.Context, db DBTX, slug string, exclude uuid.UUID) (bool, error) {
	return exists(ctx, db, `SELECT 1 FROM "Shops" WHERE "Slug" = $1 AND "Id" <> $2`, slug, exclude)
}

func InsertShop(ctx context.Context, db DBTX, s *models.Shop) error {
	_, err := db.Exec(ctx, `INSERT INTO "Shops"
		("Id", "Name", "Slug", "Description", "LogoUrl", "Status", "CreatedAt", "OwnerId")
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		s.ID, s.Name, s.Slug, s.Description, s.LogoUrl, s.Status, s.CreatedAt, s.OwnerID)
	return err
}

func UpdateShopInfo(ctx context.Context, db DBTX, id uuid.UUID, name, slug, description string) error {
	_, err := db.Exec(ctx, `UPDATE "Shops" SET "Name" = $2, "Slug" = $3, "Description" = $4 WHERE "Id" = $1`,
		id, name, slug, description)
	return err
}

func SetShopStatus(ctx context.Context, db DBTX, id uuid.UUID, status models.ShopStatus) error {
	_, err := db.Exec(ctx, `UPDATE "Shops" SET "Status" = $2 WHERE "Id" = $1`, id, status)
	return err
}

// SetShopProductsActive bật/tắt hiển thị toàn bộ sản phẩm của shop.
func SetShopProductsActive(ctx context.Context, db DBTX, shopID uuid.UUID, active bool) error {
	_, err := db.Exec(ctx, `UPDATE "Products" SET "IsActive" = $2 WHERE "ShopId" = $1`, shopID, active)
	return err
}

// ShopHasOrders: đã có dòng đơn hàng nào chứa sản phẩm của shop chưa.
func ShopHasOrders(ctx context.Context, db DBTX, shopID uuid.UUID) (bool, error) {
	return exists(ctx, db, `SELECT 1 FROM "OrderDetails" d JOIN "Products" p ON p."Id" = d."ProductId"
		WHERE p."ShopId" = $1`, shopID)
}

// DeleteShopHard xoá sản phẩm (DB cascade ảnh/review/giỏ hàng) rồi xoá shop
// (cascade hội thoại chat). Category riêng của shop không có FK nên được giữ lại như bản C#.
func DeleteShopHard(ctx context.Context, db DBTX, shopID uuid.UUID) error {
	if _, err := db.Exec(ctx, `DELETE FROM "Products" WHERE "ShopId" = $1`, shopID); err != nil {
		return err
	}
	_, err := db.Exec(ctx, `DELETE FROM "Shops" WHERE "Id" = $1`, shopID)
	return err
}
