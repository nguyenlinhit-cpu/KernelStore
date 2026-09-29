package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/KernelStore/backend/internal/models"
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

// UserReceivedProduct: user có đơn Delivered chứa sản phẩm này không (điều kiện để đánh giá).
func UserReceivedProduct(ctx context.Context, db DBTX, userID, productID uuid.UUID) (bool, error) {
	return exists(ctx, db, `SELECT 1 FROM "OrderDetails" d JOIN "Orders" o ON o."Id" = d."OrderId"
		WHERE d."ProductId" = $1 AND o."UserId" = $2 AND o."Status" = $3`,
		productID, userID, models.OrderStatusDelivered)
}

func UserReviewedProduct(ctx context.Context, db DBTX, userID, productID uuid.UUID) (bool, error) {
	return exists(ctx, db, `SELECT 1 FROM "Reviews" WHERE "ProductId" = $1 AND "UserId" = $2`, productID, userID)
}

func InsertReview(ctx context.Context, db DBTX, rv *models.Review) error {
	_, err := db.Exec(ctx, `INSERT INTO "Reviews" ("Id", "Rating", "Comment", "CreatedAt", "ProductId", "UserId")
		VALUES ($1, $2, $3, $4, $5, $6)`, rv.ID, rv.Rating, rv.Comment, rv.CreatedAt, rv.ProductID, rv.UserID)
	return err
}
