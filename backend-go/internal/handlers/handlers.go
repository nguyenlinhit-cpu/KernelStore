// Package handlers chứa các HTTP handler — mỗi file tương ứng một Controller của bản C#.
package handlers

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KernelStore/backend-go/internal/config"
	"github.com/KernelStore/backend-go/internal/httpx"
	"github.com/KernelStore/backend-go/internal/services"
)

// Handler giữ các phụ thuộc dùng chung (tương đương DI container của ASP.NET).
type Handler struct {
	db        *pgxpool.Pool
	cfg       *config.Config
	tokens    *services.TokenService
	uploadDir string
}

func New(db *pgxpool.Pool, cfg *config.Config, tokens *services.TokenService, uploadDir string) *Handler {
	return &Handler{db: db, cfg: cfg, tokens: tokens, uploadDir: uploadDir}
}

// Register đăng ký toàn bộ endpoint /api/* (tương đương app.MapControllers()).
// Các nhóm endpoint được thêm dần theo từng giai đoạn chuyển đổi.
func (h *Handler) Register(rt *httpx.Router) {
	h.registerAuth(rt)
	h.registerShops(rt)
	h.registerAdminShops(rt)
	h.registerCategories(rt)
	h.registerSellerCategories(rt)
	h.registerProducts(rt)
	h.registerUploads(rt)
	h.registerCart(rt)
	h.registerOrders(rt)
}
