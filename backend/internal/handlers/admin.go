package handlers

import (
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/KernelStore/backend/internal/dto"
	"github.com/KernelStore/backend/internal/httpx"
	"github.com/KernelStore/backend/internal/models"
	"github.com/KernelStore/backend/internal/repository"
)

// AdminDashboardController, AdminUsersController, AdminOrdersController, SellerDashboardController.
func (h *Handler) registerAdmin(rt *httpx.Router) {
	admin := httpx.Roles("Admin")
	rt.Handle("GET /api/admin/dashboard", admin, h.adminDashboard)
	rt.Handle("GET /api/admin/users", admin, h.adminListUsers)
	rt.Handle("POST /api/admin/users/{id}/ban", admin, h.adminBanUser)
	rt.Handle("POST /api/admin/users/{id}/unban", admin, h.adminUnbanUser)
	rt.Handle("GET /api/admin/orders", admin, h.adminListOrders)
	rt.Handle("GET /api/seller/dashboard", httpx.Roles("Seller", "Admin"), h.sellerDashboard)
}

// adminDashboard: doanh thu không tính đơn Cancelled/Returned; ordersByStatus đủ 8 trạng thái.
func (h *Handler) adminDashboard(w http.ResponseWriter, r *http.Request) {
	s, err := repository.LoadAdminStats(r.Context(), h.db)
	if err != nil {
		serverError(w, r, err)
		return
	}
	dto.OK(w, dto.AdminDashboardDto{
		TotalUsers: s.TotalUsers, TotalShops: s.TotalShops,
		PendingShops: s.PendingShops, ApprovedShops: s.ApprovedShops,
		TotalProducts: s.TotalProducts, ActiveProducts: s.ActiveProducts,
		TotalOrders: s.TotalOrders, TotalRevenue: s.TotalRevenue,
		OrdersByStatus: statusCounts(s.OrdersByStatus),
	}, "OK")
}

// sellerDashboard: thống kê trong phạm vi shop của người gọi.
func (h *Handler) sellerDashboard(w http.ResponseWriter, r *http.Request) {
	shop := h.myShop(w, r)
	if shop == nil {
		return
	}
	s, err := repository.LoadSellerStats(r.Context(), h.db, shop.ID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	totalOrders := 0
	for _, n := range s.OrdersByStatus {
		totalOrders += n
	}
	pending := s.OrdersByStatus[models.OrderStatusPending] +
		s.OrdersByStatus[models.OrderStatusConfirmed] + s.OrdersByStatus[models.OrderStatusProcessing]
	top := make([]dto.TopProductStat, 0, len(s.TopProducts))
	for _, t := range s.TopProducts {
		top = append(top, dto.TopProductStat{ProductID: t.ProductID, Name: deref(t.Name),
			QuantitySold: t.QuantitySold, Revenue: t.Revenue})
	}
	dto.OK(w, dto.SellerDashboardDto{
		TotalRevenue: s.TotalRevenue, TotalOrders: totalOrders, PendingOrders: pending,
		ItemsSold: s.ItemsSold, TotalProducts: s.TotalProducts, ActiveProducts: s.ActiveProducts,
		OrdersByStatus: statusCounts(s.OrdersByStatus), TopProducts: top,
	}, "OK")
}

// statusCounts liệt kê đủ mọi trạng thái (kèm 0) theo thứ tự enum.
func statusCounts(m map[models.OrderStatus]int) []dto.OrderStatusCount {
	out := make([]dto.OrderStatusCount, 0, len(models.AllOrderStatuses))
	for _, st := range models.AllOrderStatuses {
		out = append(out, dto.OrderStatusCount{Status: st.String(), Count: m[st]})
	}
	return out
}

func (h *Handler) adminListUsers(w http.ResponseWriter, r *http.Request) {
	q := httpx.NewQuery(r)
	var f repository.UserFilter
	if s := strings.TrimSpace(q.String("search")); s != "" {
		f.Search = strings.ToLower(s)
	}
	if s := q.String("role"); strings.TrimSpace(s) != "" {
		if role, ok := models.ParseUserRole(s); ok {
			f.Role = &role
		}
	}
	f.IsActive = q.Bool("isActive")
	if q.Failed(w) {
		return
	}
	users, err := repository.ListUsers(r.Context(), h.db, f)
	if err != nil {
		serverError(w, r, err)
		return
	}
	out := make([]dto.AdminUserDto, 0, len(users))
	for i := range users {
		out = append(out, adminUserDto(&users[i]))
	}
	dto.OK(w, out, "OK")
}

// adminBanUser: vô hiệu hoá tài khoản + thu hồi refresh token (phiên hiện tại không gia hạn được).
func (h *Handler) adminBanUser(w http.ResponseWriter, r *http.Request) {
	user := h.adminTargetUser(w, r)
	if user == nil {
		return
	}
	if user.Role == models.UserRoleAdmin {
		dto.BadRequest(w, "Không thể vô hiệu hóa tài khoản Admin")
		return
	}
	if !user.IsActive {
		dto.BadRequest(w, "Tài khoản đã bị vô hiệu hóa")
		return
	}
	ctx := r.Context()
	err := repository.InTx(ctx, h.db, func(tx pgx.Tx) error {
		if err := repository.SetUserActive(ctx, tx, user.ID, false); err != nil {
			return err
		}
		return repository.RevokeUserRefreshTokens(ctx, tx, user.ID)
	})
	if err != nil {
		serverError(w, r, err)
		return
	}
	user.IsActive = false
	dto.OK(w, adminUserDto(user), "Đã vô hiệu hóa người dùng")
}

func (h *Handler) adminUnbanUser(w http.ResponseWriter, r *http.Request) {
	user := h.adminTargetUser(w, r)
	if user == nil {
		return
	}
	if user.IsActive {
		dto.BadRequest(w, "Tài khoản đang hoạt động")
		return
	}
	if err := repository.SetUserActive(r.Context(), h.db, user.ID, true); err != nil {
		serverError(w, r, err)
		return
	}
	user.IsActive = true
	dto.OK(w, adminUserDto(user), "Đã kích hoạt lại người dùng")
}

// adminTargetUser: không cho thao tác trên chính mình (400), không có user → 404.
func (h *Handler) adminTargetUser(w http.ResponseWriter, r *http.Request) *models.ApplicationUser {
	id := httpx.PathGUID(r, "id")
	if id == userID(r) {
		dto.BadRequest(w, "Không thể thao tác trên chính tài khoản của bạn")
		return nil
	}
	user, err := repository.FindUserByID(r.Context(), h.db, id)
	if err != nil {
		serverError(w, r, err)
		return nil
	}
	if user == nil {
		dto.NotFound(w, "Không tìm thấy người dùng")
	}
	return user
}

func adminUserDto(u *models.ApplicationUser) dto.AdminUserDto {
	return dto.AdminUserDto{ID: u.ID, UserName: deref(u.UserName), Email: deref(u.Email),
		FullName: u.FullName, Role: u.Role.String(), IsActive: u.IsActive, CreatedAt: u.CreatedAt}
}

func (h *Handler) adminListOrders(w http.ResponseWriter, r *http.Request) {
	q := httpx.NewQuery(r)
	statusStr, search := q.String("status"), q.String("search")
	page, pageSize := q.Int("page", 1), q.Int("pageSize", 20)
	if q.Failed(w) {
		return
	}
	page = max(1, page)
	pageSize = min(max(pageSize, 1), 50)

	var status *models.OrderStatus
	if strings.TrimSpace(statusStr) != "" {
		if st, ok := models.ParseOrderStatus(statusStr); ok {
			status = &st
		}
	}
	rows, total, err := repository.ListAdminOrders(r.Context(), h.db, status,
		strings.ToLower(strings.TrimSpace(search)), (page-1)*pageSize, pageSize)
	if err != nil {
		serverError(w, r, err)
		return
	}
	items := make([]dto.AdminOrderDto, 0, len(rows))
	for _, o := range rows {
		items = append(items, dto.AdminOrderDto{
			ID: o.ID, OrderCode: o.OrderCode, Status: o.Status.String(), TotalAmount: o.TotalAmount,
			ItemCount: o.ItemCount, CreatedAt: o.CreatedAt, CustomerID: o.UserID,
			CustomerName: deref(o.CustomerName), CustomerEmail: deref(o.CustomerEmail),
		})
	}
	dto.OK(w, dto.PagedResult[dto.AdminOrderDto]{Page: page, PageSize: pageSize, Total: total,
		TotalPages: (total + pageSize - 1) / pageSize, Items: items}, "OK")
}
