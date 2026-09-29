package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/KernelStore/backend-go/internal/models"
)

// CategoryWithCount = Category kèm số sản phẩm (C#: Include(c => c.Products) → Products.Count,
// đếm cả sản phẩm đang ẩn).
type CategoryWithCount struct {
	models.Category
	ProductCount int `db:"ProductCount"`
}

const categoryWithCountSelect = `SELECT c."Id", c."Name", c."Slug", c."Description", c."ParentId", c."OwnerShopId",
	(SELECT count(*) FROM "Products" p WHERE p."CategoryId" = c."Id")::int AS "ProductCount"
	FROM "Categories" c`

// ListGlobalCategories: category do Admin quản lý (OwnerShopId IS NULL), sắp theo tên.
func ListGlobalCategories(ctx context.Context, db DBTX) ([]CategoryWithCount, error) {
	return collectCategories(ctx, db, categoryWithCountSelect+` WHERE c."OwnerShopId" IS NULL ORDER BY c."Name"`)
}

// ListShopCategories: category riêng của một shop, sắp theo tên.
func ListShopCategories(ctx context.Context, db DBTX, shopID uuid.UUID) ([]CategoryWithCount, error) {
	return collectCategories(ctx, db, categoryWithCountSelect+` WHERE c."OwnerShopId" = $1 ORDER BY c."Name"`, shopID)
}

func collectCategories(ctx context.Context, db DBTX, sql string, args ...any) ([]CategoryWithCount, error) {
	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[CategoryWithCount])
}

func FindCategoryBySlug(ctx context.Context, db DBTX, slug string) (*CategoryWithCount, error) {
	return one[CategoryWithCount](ctx, db, categoryWithCountSelect+` WHERE c."Slug" = $1 LIMIT 1`, slug)
}

func FindCategoryByID(ctx context.Context, db DBTX, id uuid.UUID) (*CategoryWithCount, error) {
	return one[CategoryWithCount](ctx, db, categoryWithCountSelect+` WHERE c."Id" = $1`, id)
}

func CategoryExists(ctx context.Context, db DBTX, id uuid.UUID) (bool, error) {
	return exists(ctx, db, `SELECT 1 FROM "Categories" WHERE "Id" = $1`, id)
}

// CategorySlugTaken: slug là duy nhất trên toàn bộ category (global lẫn của shop).
func CategorySlugTaken(ctx context.Context, db DBTX, slug string, exclude uuid.UUID) (bool, error) {
	return exists(ctx, db, `SELECT 1 FROM "Categories" WHERE "Slug" = $1 AND "Id" <> $2`, slug, exclude)
}

func CategoryHasChildren(ctx context.Context, db DBTX, id uuid.UUID) (bool, error) {
	return exists(ctx, db, `SELECT 1 FROM "Categories" WHERE "ParentId" = $1`, id)
}

// CategoryAndDescendantIDs trả id của category và mọi category con cháu (C# duyệt BFS).
func CategoryAndDescendantIDs(ctx context.Context, db DBTX, id uuid.UUID) ([]uuid.UUID, error) {
	rows, err := db.Query(ctx, `WITH RECURSIVE tree AS (
			SELECT "Id" FROM "Categories" WHERE "Id" = $1
			UNION
			SELECT c."Id" FROM "Categories" c JOIN tree t ON c."ParentId" = t."Id"
		) SELECT "Id" FROM tree`, id)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
}

func InsertCategory(ctx context.Context, db DBTX, c *models.Category) error {
	_, err := db.Exec(ctx, `INSERT INTO "Categories" ("Id", "Name", "Slug", "Description", "ParentId", "OwnerShopId")
		VALUES ($1, $2, $3, $4, $5, $6)`, c.ID, c.Name, c.Slug, c.Description, c.ParentID, c.OwnerShopID)
	return err
}

func UpdateCategory(ctx context.Context, db DBTX, c *models.Category) error {
	_, err := db.Exec(ctx, `UPDATE "Categories" SET "Name" = $2, "Slug" = $3, "Description" = $4, "ParentId" = $5
		WHERE "Id" = $1`, c.ID, c.Name, c.Slug, c.Description, c.ParentID)
	return err
}

func DeleteCategory(ctx context.Context, db DBTX, id uuid.UUID) error {
	_, err := db.Exec(ctx, `DELETE FROM "Categories" WHERE "Id" = $1`, id)
	return err
}

// DetachProductsFromCategory gỡ category khỏi sản phẩm (xoá category của shop không bị chặn).
func DetachProductsFromCategory(ctx context.Context, db DBTX, id uuid.UUID) error {
	_, err := db.Exec(ctx, `UPDATE "Products" SET "CategoryId" = NULL WHERE "CategoryId" = $1`, id)
	return err
}
