package repository

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/KernelStore/backend-go/internal/models"
)

// WarrantyRow = WarrantyClaim kèm thông tin người mua, shop, sản phẩm, dòng đơn và đơn hàng
// (C#: BaseQuery() của WarrantyController).
type WarrantyRow struct {
	models.WarrantyClaim
	UserFullName    *string    `db:"UserFullName"`
	ShopName        *string    `db:"ShopName"`
	ProductName     *string    `db:"ProductName"`
	ProductSlug     *string    `db:"ProductSlug"`
	ProductWarranty *int       `db:"ProductWarranty"`
	DetailQuantity  *int       `db:"DetailQuantity"`
	OrderID         *uuid.UUID `db:"OrderId"`
	OrderCode       *string    `db:"OrderCode"`
	OrderPaidAt     *time.Time `db:"OrderPaidAt"`
	OrderCreatedAt  *time.Time `db:"OrderCreatedAt"`
}

const warrantySelect = `SELECT w."Id", w."ClaimCode", w."Description", w."ImageUrl", w."Status", w."Resolution",
	w."ResolutionNote", w."CreatedAt", w."UpdatedAt", w."ResolvedAt", w."OrderDetailId", w."UserId",
	w."ProductId", w."ShopId",
	u."FullName" AS "UserFullName", s."Name" AS "ShopName",
	p."Name" AS "ProductName", p."Slug" AS "ProductSlug", p."WarrantyMonths" AS "ProductWarranty",
	d."Quantity" AS "DetailQuantity", o."Id" AS "OrderId", o."OrderCode" AS "OrderCode",
	o."PaidAt" AS "OrderPaidAt", o."CreatedAt" AS "OrderCreatedAt"
	FROM "WarrantyClaims" w
	LEFT JOIN "AspNetUsers" u ON u."Id" = w."UserId"
	LEFT JOIN "Shops" s ON s."Id" = w."ShopId"
	LEFT JOIN "Products" p ON p."Id" = w."ProductId"
	LEFT JOIN "OrderDetails" d ON d."Id" = w."OrderDetailId"
	LEFT JOIN "Orders" o ON o."Id" = d."OrderId"`

// WarrantyScope chọn tập yêu cầu bảo hành cần nạp.
type WarrantyScope struct {
	ClaimID   *uuid.UUID
	UserID    *uuid.UUID
	ShopID    *uuid.UUID
	Status    *models.WarrantyStatus
	ForUpdate bool
}

// ListWarrantyClaims: mới nhất trước.
func ListWarrantyClaims(ctx context.Context, db DBTX, sc WarrantyScope) ([]WarrantyRow, error) {
	sql := warrantySelect + ` WHERE TRUE`
	var args []any
	add := func(cond string, v any) {
		args = append(args, v)
		sql += ` AND ` + cond + ` = $` + strconv.Itoa(len(args))
	}
	if sc.ClaimID != nil {
		add(`w."Id"`, *sc.ClaimID)
	}
	if sc.UserID != nil {
		add(`w."UserId"`, *sc.UserID)
	}
	if sc.ShopID != nil {
		add(`w."ShopId"`, *sc.ShopID)
	}
	if sc.Status != nil {
		add(`w."Status"`, *sc.Status)
	}
	sql += ` ORDER BY w."CreatedAt" DESC, w."Id"`
	if sc.ForUpdate {
		sql += ` FOR UPDATE OF w`
	}
	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[WarrantyRow])
}

func FindWarrantyClaim(ctx context.Context, db DBTX, id uuid.UUID, forUpdate bool) (*WarrantyRow, error) {
	rows, err := ListWarrantyClaims(ctx, db, WarrantyScope{ClaimID: &id, ForUpdate: forUpdate})
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}

// WarrantyDetail là dòng đơn hàng mà khách muốn bảo hành, kèm đơn và sản phẩm.
type WarrantyDetail struct {
	DetailID       uuid.UUID          `db:"DetailId"`
	ProductID      uuid.UUID          `db:"ProductId"`
	OrderUserID    uuid.UUID          `db:"OrderUserId"`
	OrderStatus    models.OrderStatus `db:"OrderStatus"`
	OrderPaidAt    *time.Time         `db:"OrderPaidAt"`
	OrderCreatedAt time.Time          `db:"OrderCreatedAt"`
	WarrantyMonths int                `db:"WarrantyMonths"`
	ShopID         uuid.UUID          `db:"ShopId"`
}

func FindWarrantyDetail(ctx context.Context, db DBTX, detailID uuid.UUID) (*WarrantyDetail, error) {
	return one[WarrantyDetail](ctx, db, `SELECT d."Id" AS "DetailId", d."ProductId",
		o."UserId" AS "OrderUserId", o."Status" AS "OrderStatus", o."PaidAt" AS "OrderPaidAt",
		o."CreatedAt" AS "OrderCreatedAt", p."WarrantyMonths", p."ShopId"
		FROM "OrderDetails" d
		JOIN "Orders" o ON o."Id" = d."OrderId"
		JOIN "Products" p ON p."Id" = d."ProductId"
		WHERE d."Id" = $1`, detailID)
}

// HasOpenWarrantyClaim: dòng hàng đang có yêu cầu chưa kết thúc (Pending/Approved/Processing).
func HasOpenWarrantyClaim(ctx context.Context, db DBTX, detailID uuid.UUID) (bool, error) {
	return exists(ctx, db, `SELECT 1 FROM "WarrantyClaims" WHERE "OrderDetailId" = $1 AND "Status" = ANY($2)`,
		detailID, []models.WarrantyStatus{models.WarrantyStatusPending, models.WarrantyStatusApproved, models.WarrantyStatusProcessing})
}

func WarrantyCodeExists(ctx context.Context, db DBTX, code string) (bool, error) {
	return exists(ctx, db, `SELECT 1 FROM "WarrantyClaims" WHERE "ClaimCode" = $1`, code)
}

func InsertWarrantyClaim(ctx context.Context, db DBTX, w *models.WarrantyClaim) error {
	_, err := db.Exec(ctx, `INSERT INTO "WarrantyClaims" ("Id", "ClaimCode", "Description", "ImageUrl", "Status",
		"Resolution", "ResolutionNote", "CreatedAt", "UpdatedAt", "ResolvedAt", "OrderDetailId", "UserId", "ProductId", "ShopId")
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		w.ID, w.ClaimCode, w.Description, w.ImageUrl, w.Status, w.Resolution, w.ResolutionNote,
		w.CreatedAt, w.UpdatedAt, w.ResolvedAt, w.OrderDetailID, w.UserID, w.ProductID, w.ShopID)
	return err
}

// UpdateWarrantyState lưu các trường thay đổi trong vòng đời xử lý.
func UpdateWarrantyState(ctx context.Context, db DBTX, w *models.WarrantyClaim) error {
	_, err := db.Exec(ctx, `UPDATE "WarrantyClaims" SET "Status" = $2, "Resolution" = $3, "ResolutionNote" = $4,
		"UpdatedAt" = $5, "ResolvedAt" = $6 WHERE "Id" = $1`,
		w.ID, w.Status, w.Resolution, w.ResolutionNote, w.UpdatedAt, w.ResolvedAt)
	return err
}
