package handlers

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/KernelStore/backend-go/internal/dto"
	"github.com/KernelStore/backend-go/internal/httpx"
	"github.com/KernelStore/backend-go/internal/identity"
	"github.com/KernelStore/backend-go/internal/models"
	"github.com/KernelStore/backend-go/internal/repository"
	"github.com/KernelStore/backend-go/internal/services"
)

// AuthController: /api/auth.
func (h *Handler) registerAuth(rt *httpx.Router) {
	rt.Handle("POST /api/auth/register", httpx.Anonymous, h.register)
	rt.Handle("POST /api/auth/login", httpx.Anonymous, h.login)
	rt.Handle("POST /api/auth/refresh", httpx.Anonymous, h.refresh)
	rt.Handle("GET /api/auth/me", httpx.Authenticated, h.me)
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req dto.RegisterRequest
	if !httpx.BindJSON(w, r, &req) {
		return
	}
	ctx := r.Context()

	existing, err := repository.FindUserByEmail(ctx, h.db, identity.Normalize(req.Email))
	if err != nil {
		serverError(w, r, err)
		return
	}
	if existing != nil {
		dto.BadRequest(w, "Email đã được sử dụng", "Email đã tồn tại trong hệ thống")
		return
	}

	user := &models.ApplicationUser{
		ID:        services.NewUUID(),
		UserName:  strPtr(req.UserName),
		Email:     strPtr(req.Email),
		FullName:  req.FullName,
		AvatarUrl: "",
		Role:      models.UserRoleCustomer,
		IsActive:  true,
		CreatedAt: services.Now(),
	}

	// Tạo user + gán role Customer trong một transaction (C# làm 2 bước rời;
	// gộp lại để không bao giờ có user thiếu role).
	var failures []string
	err = repository.InTx(ctx, h.db, func(tx pgx.Tx) error {
		errs, err := services.CreateUser(ctx, tx, user, req.Password)
		if err != nil || len(errs) > 0 {
			failures = errs
			return err
		}
		_, err = repository.AddToRole(ctx, tx, user.ID, identity.Normalize(models.UserRoleCustomer.String()))
		return err
	})
	if err != nil {
		serverError(w, r, err)
		return
	}
	if len(failures) > 0 {
		dto.BadRequest(w, "Đăng ký thất bại", failures...)
		return
	}

	resp, err := h.buildAuthResponse(ctx, user)
	if err != nil {
		serverError(w, r, err)
		return
	}
	dto.OK(w, resp, "Đăng ký thành công")
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest
	if !httpx.BindJSON(w, r, &req) {
		return
	}
	ctx := r.Context()

	user, err := repository.FindUserByEmail(ctx, h.db, identity.Normalize(req.Email))
	if err != nil {
		serverError(w, r, err)
		return
	}
	if user == nil {
		dto.Unauthorized(w, "Email hoặc mật khẩu không đúng")
		return
	}
	if !user.IsActive {
		dto.Unauthorized(w, "Tài khoản đã bị vô hiệu hóa")
		return
	}
	ok, err := services.CheckPassword(ctx, h.db, user, req.Password)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if !ok {
		dto.Unauthorized(w, "Email hoặc mật khẩu không đúng")
		return
	}

	resp, err := h.buildAuthResponse(ctx, user)
	if err != nil {
		serverError(w, r, err)
		return
	}
	dto.OK(w, resp, "Đăng nhập thành công")
}

// refresh: refresh token dùng một lần (single-use rotation). Token đã dùng/thu hồi → 401.
func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var req dto.RefreshRequest
	if !httpx.BindJSON(w, r, &req) {
		return
	}
	ctx := r.Context()

	stored, err := repository.FindRefreshToken(ctx, h.db, req.RefreshToken)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if stored == nil || stored.IsRevoked || stored.IsUsed {
		dto.Unauthorized(w, "Refresh token không hợp lệ")
		return
	}
	if stored.ExpiresAt.Before(services.Now()) {
		dto.Unauthorized(w, "Refresh token đã hết hạn")
		return
	}
	user, err := repository.FindUserByID(ctx, h.db, stored.UserID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if user == nil || !user.IsActive {
		dto.Unauthorized(w, "Tài khoản không hợp lệ")
		return
	}

	// Đánh dấu đã dùng có điều kiện: nếu request song song đã dùng trước thì coi là tái sử dụng.
	marked, err := repository.MarkRefreshTokenUsed(ctx, h.db, stored.ID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if !marked {
		dto.Unauthorized(w, "Refresh token không hợp lệ")
		return
	}

	resp, err := h.buildAuthResponse(ctx, user)
	if err != nil {
		serverError(w, r, err)
		return
	}
	dto.OK(w, resp, "Refresh token thành công")
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, err := repository.FindUserByID(ctx, h.db, userID(r))
	if err != nil {
		serverError(w, r, err)
		return
	}
	if user == nil {
		dto.NotFound(w, "Không tìm thấy người dùng")
		return
	}
	roles, err := repository.GetUserRoles(ctx, h.db, user.ID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	// C#: new { info, roles }
	dto.OK(w, struct {
		Info  dto.UserInfoDto `json:"info"`
		Roles []string        `json:"roles"`
	}{userInfo(user), roles}, "OK")
}

// buildAuthResponse cấp access token + refresh token mới (lưu DB) cho user.
func (h *Handler) buildAuthResponse(ctx context.Context, user *models.ApplicationUser) (*dto.AuthResponse, error) {
	roles, err := repository.GetUserRoles(ctx, h.db, user.ID)
	if err != nil {
		return nil, err
	}
	access, expiresAt, err := h.tokens.CreateAccessToken(user, roles)
	if err != nil {
		return nil, err
	}
	now := services.Now()
	rt := &models.RefreshToken{
		ID:        services.NewUUID(),
		Token:     h.tokens.GenerateRefreshToken(),
		UserID:    user.ID,
		CreatedAt: now,
		ExpiresAt: h.tokens.RefreshExpiry(now),
	}
	if err := repository.InsertRefreshToken(ctx, h.db, rt); err != nil {
		return nil, err
	}
	return &dto.AuthResponse{
		AccessToken:  access,
		RefreshToken: rt.Token,
		ExpiresAt:    expiresAt,
		User:         userInfo(user),
	}, nil
}

func userInfo(u *models.ApplicationUser) dto.UserInfoDto {
	return dto.UserInfoDto{
		ID:        u.ID,
		UserName:  deref(u.UserName),
		Email:     deref(u.Email),
		FullName:  u.FullName,
		AvatarUrl: u.AvatarUrl,
		Role:      u.Role.String(),
		IsActive:  u.IsActive,
	}
}
