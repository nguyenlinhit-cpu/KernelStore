package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/KernelStore/backend-go/internal/models"
)

// ReviewRow = Review kèm FullName người viết (ReviewDto.UserName là FullName ở bản C#).
type ReviewRow struct {
	models.Review
	UserFullName *string `db:"UserFullName"`
}

// ListProductReviews: review của sản phẩm, mới nhất trước.
func ListProductReviews(ctx context.Context, db DBTX, productID uuid.UUID) ([]ReviewRow, error) {
	rows, err := db.Query(ctx, `SELECT r."Id", r."Rating", r."Comment", r."CreatedAt", r."ProductId", r."UserId",
		u."FullName" AS "UserFullName"
		FROM "Reviews" r LEFT JOIN "AspNetUsers" u ON u."Id" = r."UserId"
		WHERE r."ProductId" = $1 ORDER BY r."CreatedAt" DESC`, productID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[ReviewRow])
}
