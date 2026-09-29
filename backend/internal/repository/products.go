package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/KernelStore/backend/internal/models"
	"github.com/KernelStore/backend/internal/money"
)

// ProductRow = Product kèm tên shop và tên category (C#: Include(Shop), Include(Category)).
type ProductRow struct {
	models.Product
	ShopName     *string `db:"ShopName"`
	CategoryName *string `db:"CategoryName"`
}

const productRowSelect = `SELECT p."Id", p."Name", p."Slug", p."Description", p."Price", p."SalePrice",
	p."StockQuantity", p."Sku", p."WarrantyMonths", p."IsActive", p."CreatedAt", p."CategoryId", p."ShopId",
	s."Name" AS "ShopName", c."Name" AS "CategoryName"
	FROM "Products" p
	LEFT JOIN "Shops" s ON s."Id" = p."ShopId"
	LEFT JOIN "Categories" c ON c."Id" = p."CategoryId"`

// ProductFilter là tham số của GET /api/products.
type ProductFilter struct {
	CategoryIDs []uuid.UUID // nil = không lọc theo category
	ShopID      *uuid.UUID
	MinPrice    *money.Money
	MaxPrice    *money.Money
	Search      string // đã trim + lower; "" = không tìm
	Sort        string
	Offset      int
	Limit       int
}

// ListActiveProducts trả một trang sản phẩm đang bán + tổng số khớp bộ lọc.
// Giá lọc/sắp xếp là giá hiệu lực COALESCE(SalePrice, Price) như C# (SalePrice ?? Price).
func ListActiveProducts(ctx context.Context, db DBTX, f ProductFilter) ([]ProductRow, int, error) {
	where := []string{`p."IsActive"`}
	var args []any
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	if f.CategoryIDs != nil {
		where = append(where, `p."CategoryId" = ANY(`+arg(f.CategoryIDs)+`)`)
	}
	if f.ShopID != nil {
		where = append(where, `p."ShopId" = `+arg(*f.ShopID))
	}
	if f.MinPrice != nil {
		where = append(where, `COALESCE(p."SalePrice", p."Price") >= `+arg(*f.MinPrice)+`::numeric`)
	}
	if f.MaxPrice != nil {
		where = append(where, `COALESCE(p."SalePrice", p."Price") <= `+arg(*f.MaxPrice)+`::numeric`)
	}
	if f.Search != "" {
		// strpos = Contains của C#: so khớp chuỗi thuần, không coi % hay _ là ký tự đại diện.
		t := arg(f.Search)
		where = append(where, `(strpos(lower(p."Name"), `+t+`) > 0 OR strpos(lower(p."Description"), `+t+`) > 0)`)
	}
	cond := " WHERE " + strings.Join(where, " AND ")

	var total int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM "Products" p`+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	order := `p."CreatedAt" DESC`
	switch f.Sort {
	case "price_asc":
		order = `COALESCE(p."SalePrice", p."Price")`
	case "price_desc":
		order = `COALESCE(p."SalePrice", p."Price") DESC`
	case "name":
		order = `p."Name"`
	}
	// Thêm Id làm khoá phụ để phân trang ổn định khi trùng giá trị sắp xếp.
	sql := productRowSelect + cond + ` ORDER BY ` + order + `, p."Id" OFFSET ` + arg(f.Offset) + ` LIMIT ` + arg(f.Limit)
	items, err := collectProducts(ctx, db, sql, args...)
	return items, total, err
}

// FeaturedProducts: sản phẩm đang bán mới nhất.
func FeaturedProducts(ctx context.Context, db DBTX, take int) ([]ProductRow, error) {
	return collectProducts(ctx, db, productRowSelect+` WHERE p."IsActive" ORDER BY p."CreatedAt" DESC, p."Id" LIMIT $1`, take)
}

// ListShopProducts: toàn bộ sản phẩm của shop (kể cả đang ẩn), mới nhất trước.
func ListShopProducts(ctx context.Context, db DBTX, shopID uuid.UUID) ([]ProductRow, error) {
	return collectProducts(ctx, db, productRowSelect+` WHERE p."ShopId" = $1 ORDER BY p."CreatedAt" DESC, p."Id"`, shopID)
}

// FindActiveProductBySlugOrID: trang chi tiết nhận slug hoặc id.
func FindActiveProductBySlugOrID(ctx context.Context, db DBTX, slug string, id uuid.UUID) (*ProductRow, error) {
	return one[ProductRow](ctx, db, productRowSelect+` WHERE p."IsActive" AND (p."Slug" = $1 OR p."Id" = $2) LIMIT 1`, slug, id)
}

func FindProductRow(ctx context.Context, db DBTX, id uuid.UUID) (*ProductRow, error) {
	return one[ProductRow](ctx, db, productRowSelect+` WHERE p."Id" = $1`, id)
}

// FindShopProduct: sản phẩm id thuộc shopID (dùng kiểm quyền sở hữu).
func FindShopProduct(ctx context.Context, db DBTX, id, shopID uuid.UUID) (*ProductRow, error) {
	return one[ProductRow](ctx, db, productRowSelect+` WHERE p."Id" = $1 AND p."ShopId" = $2`, id, shopID)
}

func collectProducts(ctx context.Context, db DBTX, sql string, args ...any) ([]ProductRow, error) {
	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[ProductRow])
}

func CountActiveShopProducts(ctx context.Context, db DBTX, shopID uuid.UUID) (int, error) {
	var n int
	err := db.QueryRow(ctx, `SELECT count(*) FROM "Products" WHERE "ShopId" = $1 AND "IsActive"`, shopID).Scan(&n)
	return n, err
}

func ProductSlugTaken(ctx context.Context, db DBTX, slug string) (bool, error) {
	return exists(ctx, db, `SELECT 1 FROM "Products" WHERE "Slug" = $1`, slug)
}

func InsertProduct(ctx context.Context, db DBTX, p *models.Product) error {
	_, err := db.Exec(ctx, `INSERT INTO "Products" ("Id", "Name", "Slug", "Description", "Price", "SalePrice",
		"StockQuantity", "Sku", "WarrantyMonths", "IsActive", "CreatedAt", "CategoryId", "ShopId")
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		p.ID, p.Name, p.Slug, p.Description, p.Price, p.SalePrice,
		p.StockQuantity, p.Sku, p.WarrantyMonths, p.IsActive, p.CreatedAt, p.CategoryID, p.ShopID)
	return err
}

func UpdateProduct(ctx context.Context, db DBTX, p *models.Product) error {
	_, err := db.Exec(ctx, `UPDATE "Products" SET "Name" = $2, "Slug" = $3, "Description" = $4, "Price" = $5,
		"SalePrice" = $6, "StockQuantity" = $7, "Sku" = $8, "WarrantyMonths" = $9, "CategoryId" = $10, "IsActive" = $11
		WHERE "Id" = $1`,
		p.ID, p.Name, p.Slug, p.Description, p.Price, p.SalePrice,
		p.StockQuantity, p.Sku, p.WarrantyMonths, p.CategoryID, p.IsActive)
	return err
}

// DeleteProduct: nếu sản phẩm đã nằm trong đơn/bảo hành, FK RESTRICT làm lệnh lỗi
// (C# cũng ném exception → 500).
func DeleteProduct(ctx context.Context, db DBTX, id uuid.UUID) error {
	_, err := db.Exec(ctx, `DELETE FROM "Products" WHERE "Id" = $1`, id)
	return err
}

// ─── Images ──────────────────────────────────────────────────────────────────

// LoadProductImages nạp ảnh của nhiều sản phẩm trong một truy vấn, sắp theo DisplayOrder.
func LoadProductImages(ctx context.Context, db DBTX, productIDs []uuid.UUID) (map[uuid.UUID][]models.ProductImage, error) {
	out := make(map[uuid.UUID][]models.ProductImage, len(productIDs))
	if len(productIDs) == 0 {
		return out, nil
	}
	rows, err := db.Query(ctx, `SELECT "Id", "Url", "AltText", "IsPrimary", "DisplayOrder", "ProductId"
		FROM "ProductImages" WHERE "ProductId" = ANY($1) ORDER BY "DisplayOrder", "Id"`, productIDs)
	if err != nil {
		return nil, err
	}
	images, err := pgx.CollectRows(rows, pgx.RowToStructByName[models.ProductImage])
	if err != nil {
		return nil, err
	}
	for _, img := range images {
		out[img.ProductID] = append(out[img.ProductID], img)
	}
	return out, nil
}

func CountProductImages(ctx context.Context, db DBTX, productID uuid.UUID) (int, error) {
	var n int
	err := db.QueryRow(ctx, `SELECT count(*) FROM "ProductImages" WHERE "ProductId" = $1`, productID).Scan(&n)
	return n, err
}

func DeleteProductImages(ctx context.Context, db DBTX, productID uuid.UUID) error {
	_, err := db.Exec(ctx, `DELETE FROM "ProductImages" WHERE "ProductId" = $1`, productID)
	return err
}

func InsertProductImages(ctx context.Context, db DBTX, images []models.ProductImage) error {
	for _, img := range images {
		if _, err := db.Exec(ctx, `INSERT INTO "ProductImages" ("Id", "Url", "AltText", "IsPrimary", "DisplayOrder", "ProductId")
			VALUES ($1, $2, $3, $4, $5, $6)`, img.ID, img.Url, img.AltText, img.IsPrimary, img.DisplayOrder, img.ProductID); err != nil {
			return err
		}
	}
	return nil
}
