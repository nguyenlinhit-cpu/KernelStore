package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/KernelStore/backend/internal/models"
)

func InsertRefreshToken(ctx context.Context, db DBTX, t *models.RefreshToken) error {
	_, err := db.Exec(ctx, `INSERT INTO "RefreshTokens"
		("Id", "Token", "ExpiresAt", "CreatedAt", "IsRevoked", "IsUsed", "UserId")
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		t.ID, t.Token, t.ExpiresAt, t.CreatedAt, t.IsRevoked, t.IsUsed, t.UserID)
	return err
}

func FindRefreshToken(ctx context.Context, db DBTX, token string) (*models.RefreshToken, error) {
	return one[models.RefreshToken](ctx, db, `SELECT "Id", "Token", "ExpiresAt", "CreatedAt",
		"IsRevoked", "IsUsed", "UserId" FROM "RefreshTokens" WHERE "Token" = $1`, token)
}

// MarkRefreshTokenUsed đánh dấu đã dùng một cách nguyên tử: trả false nếu request khác
// đã dùng token này trước (chặn hai lần refresh song song cùng một token).
func MarkRefreshTokenUsed(ctx context.Context, db DBTX, id uuid.UUID) (bool, error) {
	tag, err := db.Exec(ctx, `UPDATE "RefreshTokens" SET "IsUsed" = TRUE
		WHERE "Id" = $1 AND NOT "IsUsed" AND NOT "IsRevoked"`, id)
	return tag.RowsAffected() == 1, err
}

// RevokeUserRefreshTokens thu hồi mọi refresh token còn hiệu lực của user (khi bị ban).
func RevokeUserRefreshTokens(ctx context.Context, db DBTX, userID uuid.UUID) error {
	_, err := db.Exec(ctx, `UPDATE "RefreshTokens" SET "IsRevoked" = TRUE
		WHERE "UserId" = $1 AND NOT "IsRevoked"`, userID)
	return err
}
