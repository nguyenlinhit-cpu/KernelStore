package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/KernelStore/backend-go/internal/models"
)

// ConversationRow = Conversation kèm shop, người mua, tin cuối và số tin chưa đọc của người xem.
type ConversationRow struct {
	models.Conversation
	ShopName    *string   `db:"ShopName"`
	ShopOwnerID uuid.UUID `db:"ShopOwnerId"`
	BuyerName   *string   `db:"BuyerName"`
	LastMessage *string   `db:"LastMessage"`
	UnreadCount int       `db:"UnreadCount"`
}

// $1 luôn là người xem (để đếm tin chưa đọc do phía kia gửi).
const conversationSelect = `SELECT c."Id", c."BuyerId", c."ShopId", c."CreatedAt", c."LastMessageAt",
	s."Name" AS "ShopName", s."OwnerId" AS "ShopOwnerId", u."FullName" AS "BuyerName",
	(SELECT m."Content" FROM "ChatMessages" m WHERE m."ConversationId" = c."Id"
		ORDER BY m."CreatedAt" DESC LIMIT 1) AS "LastMessage",
	(SELECT count(*) FROM "ChatMessages" m WHERE m."ConversationId" = c."Id"
		AND m."SenderId" <> $1 AND NOT m."IsRead")::int AS "UnreadCount"
	FROM "Conversations" c
	JOIN "Shops" s ON s."Id" = c."ShopId"
	LEFT JOIN "AspNetUsers" u ON u."Id" = c."BuyerId"`

// ListConversations: hội thoại mà user là người mua hoặc chủ shop, mới hoạt động trước.
func ListConversations(ctx context.Context, db DBTX, userID uuid.UUID) ([]ConversationRow, error) {
	rows, err := db.Query(ctx, conversationSelect+` WHERE c."BuyerId" = $1 OR s."OwnerId" = $1
		ORDER BY c."LastMessageAt" DESC, c."Id"`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[ConversationRow])
}

func FindConversation(ctx context.Context, db DBTX, viewer, id uuid.UUID) (*ConversationRow, error) {
	return one[ConversationRow](ctx, db, conversationSelect+` WHERE c."Id" = $2`, viewer, id)
}

// GetOrCreateConversation: mỗi cặp (người mua, shop) chỉ có một hội thoại.
func GetOrCreateConversation(ctx context.Context, db DBTX, buyerID, shopID uuid.UUID, now time.Time) (uuid.UUID, error) {
	_, err := db.Exec(ctx, `INSERT INTO "Conversations" ("Id", "BuyerId", "ShopId", "CreatedAt", "LastMessageAt")
		VALUES ($1, $2, $3, $4, $4) ON CONFLICT ("BuyerId", "ShopId") DO NOTHING`,
		uuid.Must(uuid.NewV7()), buyerID, shopID, now)
	if err != nil {
		return uuid.Nil, err
	}
	var id uuid.UUID
	err = db.QueryRow(ctx, `SELECT "Id" FROM "Conversations" WHERE "BuyerId" = $1 AND "ShopId" = $2`,
		buyerID, shopID).Scan(&id)
	return id, err
}

// ListMessages: tin nhắn theo thời gian tăng dần.
func ListMessages(ctx context.Context, db DBTX, conversationID uuid.UUID) ([]models.ChatMessage, error) {
	rows, err := db.Query(ctx, `SELECT "Id", "ConversationId", "SenderId", "Content", "IsRead", "CreatedAt"
		FROM "ChatMessages" WHERE "ConversationId" = $1 ORDER BY "CreatedAt", "Id"`, conversationID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[models.ChatMessage])
}

// MarkMessagesRead đánh dấu đã đọc các tin do phía kia gửi.
func MarkMessagesRead(ctx context.Context, db DBTX, conversationID, reader uuid.UUID) error {
	_, err := db.Exec(ctx, `UPDATE "ChatMessages" SET "IsRead" = TRUE
		WHERE "ConversationId" = $1 AND "SenderId" <> $2 AND NOT "IsRead"`, conversationID, reader)
	return err
}

// InsertMessage lưu tin và cập nhật LastMessageAt của hội thoại.
func InsertMessage(ctx context.Context, db DBTX, m *models.ChatMessage) error {
	if _, err := db.Exec(ctx, `INSERT INTO "ChatMessages" ("Id", "ConversationId", "SenderId", "Content", "IsRead", "CreatedAt")
		VALUES ($1, $2, $3, $4, $5, $6)`, m.ID, m.ConversationID, m.SenderID, m.Content, m.IsRead, m.CreatedAt); err != nil {
		return err
	}
	_, err := db.Exec(ctx, `UPDATE "Conversations" SET "LastMessageAt" = $2 WHERE "Id" = $1`, m.ConversationID, m.CreatedAt)
	return err
}
