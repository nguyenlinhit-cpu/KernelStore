package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/KernelStore/backend/internal/models"
	"github.com/KernelStore/backend/internal/money"
)

// CartRow = CartItem kèm thông tin sản phẩm + tên shop (C#: Include(Product).ThenInclude(Shop)).
type CartRow struct {
	models.CartItem
	Name          string       `db:"Name"`
	Slug          string       `db:"Slug"`
	Price         money.Money  `db:"Price"`
	SalePrice     *money.Money `db:"SalePrice"`
	StockQuantity int          `db:"StockQuantity"`
	IsActive      bool         `db:"IsActive"`
	ShopID        uuid.UUID    `db:"ShopId"`
	ShopName      *string      `db:"ShopName"`
}

// ListCartRows: giỏ hàng của user, sắp theo tên sản phẩm (gồm cả sản phẩm đã bị ẩn, như C#).
func ListCartRows(ctx context.Context, db DBTX, userID uuid.UUID) ([]CartRow, error) {
	rows, err := db.Query(ctx, `SELECT ci."Id", ci."Quantity", ci."UserId", ci."ProductId",
		p."Name", p."Slug", p."Price", p."SalePrice", p."StockQuantity", p."IsActive", p."ShopId",
		s."Name" AS "ShopName"
		FROM "CartItems" ci
		JOIN "Products" p ON p."Id" = ci."ProductId"
		LEFT JOIN "Shops" s ON s."Id" = p."ShopId"
		WHERE ci."UserId" = $1
		ORDER BY p."Name", ci."Id"`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[CartRow])
}

func FindCartItem(ctx context.Context, db DBTX, userID, productID uuid.UUID) (*models.CartItem, error) {
	return one[models.CartItem](ctx, db, `SELECT "Id", "Quantity", "UserId", "ProductId"
		FROM "CartItems" WHERE "UserId" = $1 AND "ProductId" = $2`, userID, productID)
}

func InsertCartItem(ctx context.Context, db DBTX, c *models.CartItem) error {
	_, err := db.Exec(ctx, `INSERT INTO "CartItems" ("Id", "Quantity", "UserId", "ProductId") VALUES ($1, $2, $3, $4)`,
		c.ID, c.Quantity, c.UserID, c.ProductID)
	return err
}

func UpdateCartItemQuantity(ctx context.Context, db DBTX, id uuid.UUID, qty int) error {
	_, err := db.Exec(ctx, `UPDATE "CartItems" SET "Quantity" = $2 WHERE "Id" = $1`, id, qty)
	return err
}

func DeleteCartItem(ctx context.Context, db DBTX, id uuid.UUID) error {
	_, err := db.Exec(ctx, `DELETE FROM "CartItems" WHERE "Id" = $1`, id)
	return err
}

func ClearCart(ctx context.Context, db DBTX, userID uuid.UUID) error {
	_, err := db.Exec(ctx, `DELETE FROM "CartItems" WHERE "UserId" = $1`, userID)
	return err
}
