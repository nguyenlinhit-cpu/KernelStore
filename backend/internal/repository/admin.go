package repository

import (
	"context"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/KernelStore/backend/internal/models"
	"github.com/KernelStore/backend/internal/money"
)

// Trạng thái đơn không tính doanh thu (đơn bị huỷ / đã trả hàng).
var nonRevenueStatuses = []models.OrderStatus{models.OrderStatusCancelled, models.OrderStatusReturned}

// AdminStats là các con số của dashboard admin.
type AdminStats struct {
	TotalUsers, TotalShops, PendingShops, ApprovedShops int
	TotalProducts, ActiveProducts, TotalOrders          int
	TotalRevenue                                        money.Money
	OrdersByStatus                                      map[models.OrderStatus]int
}

func LoadAdminStats(ctx context.Context, db DBTX) (*AdminStats, error) {
	s := &AdminStats{}
	var revenue *money.Money
	err := db.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM "AspNetUsers")::int,
		(SELECT count(*) FROM "Shops")::int,
		(SELECT count(*) FROM "Shops" WHERE "Status" = $1)::int,
		(SELECT count(*) FROM "Shops" WHERE "Status" = $2)::int,
		(SELECT count(*) FROM "Products")::int,
		(SELECT count(*) FROM "Products" WHERE "IsActive")::int,
		(SELECT count(*) FROM "Orders")::int,
		(SELECT sum("TotalAmount") FROM "Orders" WHERE NOT ("Status" = ANY($3)))`,
		models.ShopStatusPending, models.ShopStatusApproved, nonRevenueStatuses,
	).Scan(&s.TotalUsers, &s.TotalShops, &s.PendingShops, &s.ApprovedShops,
		&s.TotalProducts, &s.ActiveProducts, &s.TotalOrders, &revenue)
	if err != nil {
		return nil, err
	}
	s.TotalRevenue = orZero(revenue)
	s.OrdersByStatus, err = countByStatus(ctx, db, `SELECT "Status", count(*)::int FROM "Orders" GROUP BY "Status"`)
	return s, err
}

// SellerStats là các con số của dashboard seller (chỉ trong phạm vi shop).
type SellerStats struct {
	TotalRevenue                  money.Money
	ItemsSold                     int
	TotalProducts, ActiveProducts int
	OrdersByStatus                map[models.OrderStatus]int
	TopProducts                   []TopProduct
}

type TopProduct struct {
	ProductID    uuid.UUID   `db:"ProductId"`
	Name         *string     `db:"Name"`
	QuantitySold int         `db:"QuantitySold"`
	Revenue      money.Money `db:"Revenue"`
}

func LoadSellerStats(ctx context.Context, db DBTX, shopID uuid.UUID) (*SellerStats, error) {
	s := &SellerStats{}
	var revenue *money.Money
	var itemsSold *int
	// Doanh thu / số lượng bán: chỉ các dòng đơn chứa hàng của shop, đơn không bị huỷ/trả.
	err := db.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM "Products" WHERE "ShopId" = $1)::int,
		(SELECT count(*) FROM "Products" WHERE "ShopId" = $1 AND "IsActive")::int,
		sum(d."TotalPrice"), sum(d."Quantity")::int
		FROM "OrderDetails" d
		JOIN "Products" p ON p."Id" = d."ProductId"
		JOIN "Orders" o ON o."Id" = d."OrderId"
		WHERE p."ShopId" = $1 AND NOT (o."Status" = ANY($2))`, shopID, nonRevenueStatuses,
	).Scan(&s.TotalProducts, &s.ActiveProducts, &revenue, &itemsSold)
	if err != nil {
		return nil, err
	}
	s.TotalRevenue = orZero(revenue)
	if itemsSold != nil {
		s.ItemsSold = *itemsSold
	}

	// Mỗi đơn (có chứa hàng của shop) đếm một lần theo trạng thái.
	s.OrdersByStatus, err = countByStatus(ctx, db, `SELECT o."Status", count(*)::int FROM "Orders" o
		WHERE EXISTS (SELECT 1 FROM "OrderDetails" d JOIN "Products" p ON p."Id" = d."ProductId"
			WHERE d."OrderId" = o."Id" AND p."ShopId" = $1)
		GROUP BY o."Status"`, shopID)
	if err != nil {
		return nil, err
	}

	// Top 5 sản phẩm theo doanh thu (đơn không huỷ/trả).
	rows, err := db.Query(ctx, `SELECT d."ProductId", max(p."Name") AS "Name",
		sum(d."Quantity")::int AS "QuantitySold", sum(d."TotalPrice") AS "Revenue"
		FROM "OrderDetails" d
		JOIN "Products" p ON p."Id" = d."ProductId"
		JOIN "Orders" o ON o."Id" = d."OrderId"
		WHERE p."ShopId" = $1 AND NOT (o."Status" = ANY($2))
		GROUP BY d."ProductId"
		ORDER BY sum(d."TotalPrice") DESC, d."ProductId"
		LIMIT 5`, shopID, nonRevenueStatuses)
	if err != nil {
		return nil, err
	}
	s.TopProducts, err = pgx.CollectRows(rows, pgx.RowToStructByName[TopProduct])
	return s, err
}

func countByStatus(ctx context.Context, db DBTX, sql string, args ...any) (map[models.OrderStatus]int, error) {
	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	out := map[models.OrderStatus]int{}
	var st models.OrderStatus
	var n int
	_, err = pgx.ForEachRow(rows, []any{&st, &n}, func() error {
		out[st] = n
		return nil
	})
	return out, err
}

func orZero(m *money.Money) money.Money {
	if m == nil {
		return money.Zero
	}
	return *m
}

// ─── Users ───────────────────────────────────────────────────────────────────

// UserFilter cho GET /api/admin/users.
type UserFilter struct {
	Search   string // đã trim + lower
	Role     *models.UserRole
	IsActive *bool
}

func ListUsers(ctx context.Context, db DBTX, f UserFilter) ([]models.ApplicationUser, error) {
	sql := `SELECT ` + userColumns + ` FROM "AspNetUsers" WHERE TRUE`
	var args []any
	next := func(v any) string { args = append(args, v); return "$" + strconv.Itoa(len(args)) }
	if f.Search != "" {
		t := next(f.Search)
		sql += ` AND (strpos(lower(coalesce("UserName", '')), ` + t + `) > 0
			OR strpos(lower(coalesce("Email", '')), ` + t + `) > 0
			OR strpos(lower("FullName"), ` + t + `) > 0)`
	}
	if f.Role != nil {
		sql += ` AND "Role" = ` + next(*f.Role)
	}
	if f.IsActive != nil {
		sql += ` AND "IsActive" = ` + next(*f.IsActive)
	}
	rows, err := db.Query(ctx, sql+` ORDER BY "CreatedAt" DESC, "Id"`, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[models.ApplicationUser])
}

func SetUserActive(ctx context.Context, db DBTX, id uuid.UUID, active bool) error {
	_, err := db.Exec(ctx, `UPDATE "AspNetUsers" SET "IsActive" = $2 WHERE "Id" = $1`, id, active)
	return err
}

// ─── Admin orders ────────────────────────────────────────────────────────────

type AdminOrderRow struct {
	models.Order
	ItemCount     int     `db:"ItemCount"`
	CustomerName  *string `db:"CustomerName"`
	CustomerEmail *string `db:"CustomerEmail"`
}

// ListAdminOrders: phân trang, lọc trạng thái + tìm theo mã đơn.
func ListAdminOrders(ctx context.Context, db DBTX, status *models.OrderStatus, search string, offset, limit int) ([]AdminOrderRow, int, error) {
	cond := ` WHERE TRUE`
	var args []any
	next := func(v any) string { args = append(args, v); return "$" + strconv.Itoa(len(args)) }
	if status != nil {
		cond += ` AND o."Status" = ` + next(*status)
	}
	if search != "" {
		cond += ` AND strpos(lower(o."OrderCode"), ` + next(search) + `) > 0`
	}
	var total int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM "Orders" o`+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	sql := `SELECT o."Id", o."OrderCode", o."Status", o."TotalAmount", o."ShippingFee", o."Note",
		o."CreatedAt", o."PaidAt", o."UserId", o."AddressId",
		coalesce((SELECT sum(d."Quantity") FROM "OrderDetails" d WHERE d."OrderId" = o."Id"), 0)::int AS "ItemCount",
		u."FullName" AS "CustomerName", u."Email" AS "CustomerEmail"
		FROM "Orders" o LEFT JOIN "AspNetUsers" u ON u."Id" = o."UserId"` + cond +
		` ORDER BY o."CreatedAt" DESC, o."Id" OFFSET ` + next(offset) + ` LIMIT ` + next(limit)
	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, err
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[AdminOrderRow])
	return items, total, err
}
