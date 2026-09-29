package repository

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/KernelStore/backend/internal/models"
)

// OrderDetailRow = OrderDetail kèm thông tin sản phẩm + tên shop.
type OrderDetailRow struct {
	models.OrderDetail
	ProductName   *string    `db:"ProductName"`
	ProductSlug   *string    `db:"ProductSlug"`
	ProductShopID *uuid.UUID `db:"ProductShopId"`
	ShopName      *string    `db:"ShopName"`
}

// OrderFull = Order + Address + Details + ảnh sản phẩm (C#: BaseQuery() của OrdersController).
type OrderFull struct {
	models.Order
	Address *models.Address
	Details []OrderDetailRow
	Images  map[uuid.UUID][]models.ProductImage
}

// OrderScope chọn tập đơn cần nạp.
type OrderScope struct {
	OrderID   *uuid.UUID
	All       bool       // admin: mọi đơn
	BuyerID   *uuid.UUID // đơn do user này đặt
	ShopID    *uuid.UUID // đơn có chứa hàng của shop này (OR với BuyerID)
	Status    *models.OrderStatus
	ForUpdate bool // khoá dòng đơn (dùng trong transaction đổi trạng thái)
}

// LoadOrders nạp đơn theo scope, mới nhất trước, kèm địa chỉ/chi tiết/ảnh.
func LoadOrders(ctx context.Context, db DBTX, sc OrderScope) ([]OrderFull, error) {
	sql := `SELECT o."Id", o."OrderCode", o."Status", o."TotalAmount", o."ShippingFee", o."Note",
		o."CreatedAt", o."PaidAt", o."UserId", o."AddressId" FROM "Orders" o WHERE `
	var args []any
	switch {
	case sc.OrderID != nil:
		args = append(args, *sc.OrderID)
		sql += `o."Id" = $1`
	case sc.All:
		sql += `TRUE`
	default:
		args = append(args, sc.BuyerID, sc.ShopID)
		sql += `(o."UserId" = $1 OR EXISTS (SELECT 1 FROM "OrderDetails" d JOIN "Products" p ON p."Id" = d."ProductId"
			WHERE d."OrderId" = o."Id" AND p."ShopId" = $2))`
	}
	if sc.Status != nil {
		args = append(args, *sc.Status)
		sql += ` AND o."Status" = $` + strconv.Itoa(len(args))
	}
	sql += ` ORDER BY o."CreatedAt" DESC, o."Id"`
	if sc.ForUpdate {
		sql += ` FOR UPDATE`
	}

	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	orders, err := pgx.CollectRows(rows, pgx.RowToStructByName[models.Order])
	if err != nil || len(orders) == 0 {
		return []OrderFull{}, err
	}

	ids := make([]uuid.UUID, len(orders))
	addrIDs := make([]uuid.UUID, len(orders))
	for i, o := range orders {
		ids[i], addrIDs[i] = o.ID, o.AddressID
	}

	rows, err = db.Query(ctx, `SELECT "Id", "FullName", "Phone", "Street", "Ward", "District", "City", "IsDefault", "UserId"
		FROM "Addresses" WHERE "Id" = ANY($1)`, addrIDs)
	if err != nil {
		return nil, err
	}
	addrs, err := pgx.CollectRows(rows, pgx.RowToStructByName[models.Address])
	if err != nil {
		return nil, err
	}
	addrByID := make(map[uuid.UUID]*models.Address, len(addrs))
	for i := range addrs {
		addrByID[addrs[i].ID] = &addrs[i]
	}

	rows, err = db.Query(ctx, `SELECT d."Id", d."Quantity", d."UnitPrice", d."TotalPrice", d."OrderId", d."ProductId",
		p."Name" AS "ProductName", p."Slug" AS "ProductSlug", p."ShopId" AS "ProductShopId", s."Name" AS "ShopName"
		FROM "OrderDetails" d
		LEFT JOIN "Products" p ON p."Id" = d."ProductId"
		LEFT JOIN "Shops" s ON s."Id" = p."ShopId"
		WHERE d."OrderId" = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	details, err := pgx.CollectRows(rows, pgx.RowToStructByName[OrderDetailRow])
	if err != nil {
		return nil, err
	}
	detailsByOrder := make(map[uuid.UUID][]OrderDetailRow, len(orders))
	productIDs := make([]uuid.UUID, 0, len(details))
	for _, d := range details {
		detailsByOrder[d.OrderID] = append(detailsByOrder[d.OrderID], d)
		productIDs = append(productIDs, d.ProductID)
	}
	images, err := LoadProductImages(ctx, db, productIDs)
	if err != nil {
		return nil, err
	}

	out := make([]OrderFull, len(orders))
	for i, o := range orders {
		out[i] = OrderFull{Order: o, Address: addrByID[o.AddressID], Details: detailsByOrder[o.ID], Images: images}
	}
	return out, nil
}

// LoadOrder nạp một đơn; không có → nil.
func LoadOrder(ctx context.Context, db DBTX, id uuid.UUID, forUpdate bool) (*OrderFull, error) {
	orders, err := LoadOrders(ctx, db, OrderScope{OrderID: &id, ForUpdate: forUpdate})
	if err != nil || len(orders) == 0 {
		return nil, err
	}
	return &orders[0], nil
}

func OrderCodeExists(ctx context.Context, db DBTX, code string) (bool, error) {
	return exists(ctx, db, `SELECT 1 FROM "Orders" WHERE "OrderCode" = $1`, code)
}

func InsertAddress(ctx context.Context, db DBTX, a *models.Address) error {
	_, err := db.Exec(ctx, `INSERT INTO "Addresses" ("Id", "FullName", "Phone", "Street", "Ward", "District", "City", "IsDefault", "UserId")
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		a.ID, a.FullName, a.Phone, a.Street, a.Ward, a.District, a.City, a.IsDefault, a.UserID)
	return err
}

func InsertOrder(ctx context.Context, db DBTX, o *models.Order) error {
	_, err := db.Exec(ctx, `INSERT INTO "Orders" ("Id", "OrderCode", "Status", "TotalAmount", "ShippingFee", "Note",
		"CreatedAt", "PaidAt", "UserId", "AddressId") VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		o.ID, o.OrderCode, o.Status, o.TotalAmount, o.ShippingFee, o.Note, o.CreatedAt, o.PaidAt, o.UserID, o.AddressID)
	return err
}

func InsertOrderDetail(ctx context.Context, db DBTX, d *models.OrderDetail) error {
	_, err := db.Exec(ctx, `INSERT INTO "OrderDetails" ("Id", "Quantity", "UnitPrice", "TotalPrice", "OrderId", "ProductId")
		VALUES ($1, $2, $3, $4, $5, $6)`, d.ID, d.Quantity, d.UnitPrice, d.TotalPrice, d.OrderID, d.ProductID)
	return err
}

// DecrementStock trừ kho có điều kiện; trả false nếu không đủ hàng (chống bán vượt kho khi đặt song song).
func DecrementStock(ctx context.Context, db DBTX, productID uuid.UUID, qty int) (bool, error) {
	tag, err := db.Exec(ctx, `UPDATE "Products" SET "StockQuantity" = "StockQuantity" - $2
		WHERE "Id" = $1 AND "StockQuantity" >= $2`, productID, qty)
	return tag.RowsAffected() == 1, err
}

// RestoreOrderStock hoàn lại tồn kho cho mọi dòng của đơn (khi huỷ / trả hàng).
func RestoreOrderStock(ctx context.Context, db DBTX, orderID uuid.UUID) error {
	_, err := db.Exec(ctx, `UPDATE "Products" p SET "StockQuantity" = p."StockQuantity" + d."Quantity"
		FROM "OrderDetails" d WHERE d."OrderId" = $1 AND p."Id" = d."ProductId"`, orderID)
	return err
}

func SetOrderStatus(ctx context.Context, db DBTX, id uuid.UUID, status models.OrderStatus, paidAt *time.Time) error {
	_, err := db.Exec(ctx, `UPDATE "Orders" SET "Status" = $2, "PaidAt" = $3 WHERE "Id" = $1`, id, status, paidAt)
	return err
}
