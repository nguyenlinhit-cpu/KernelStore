// Package services chứa business logic chung (JWT, chat, v.v.).
package services

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/KernelStore/backend-go/internal/config"
	"github.com/KernelStore/backend-go/internal/models"
)

// Tên claim ngắn — đúng như JwtSecurityTokenHandler của C# ghi ra
// (OutboundClaimTypeMap: NameIdentifier→nameid, Name→unique_name, Email→email, Role→role).
const (
	ClaimUserID   = "nameid"
	ClaimUserName = "unique_name"
	ClaimEmail    = "email"
	ClaimRole     = "role"
)

// ClockSkew mặc định của ASP.NET JwtBearer: token còn được chấp nhận 5 phút sau exp.
const ClockSkew = 5 * time.Minute

// TokenService tạo và kiểm tra JWT access token, sinh refresh token.
type TokenService struct {
	cfg *config.Config
}

func NewTokenService(cfg *config.Config) *TokenService {
	return &TokenService{cfg: cfg}
}

// AccessClaims là thông tin đọc ra từ access token hợp lệ.
type AccessClaims struct {
	UserID   uuid.UUID
	UserName string
	Email    string
	// Roles gồm cả claim "role" từ cột AspNetUsers.Role lẫn các role Identity,
	// giống ClaimsPrincipal.IsInRole của C# (cả hai đều map vào ClaimTypes.Role).
	Roles []string
}

// HasRole so sánh phân biệt hoa thường như ClaimsIdentity.HasClaim của .NET.
func (c *AccessClaims) HasRole(role string) bool {
	for _, r := range c.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// CreateAccessToken sinh JWT HS256. Payload khớp bản C#:
// nameid, unique_name, email, role (enum của user + từng role Identity), exp, iss, aud.
func (s *TokenService) CreateAccessToken(user *models.ApplicationUser, roles []string) (string, time.Time, error) {
	// JWT lưu exp theo giây → cắt phần lẻ để ExpiresAt trả về khớp exp (C# đọc lại jwt.ValidTo).
	expiresAt := time.Now().UTC().Add(time.Duration(s.cfg.JWTAccessTokenExpiryMins) * time.Minute).Truncate(time.Second)

	roleValues := append([]string{user.Role.String()}, roles...)
	var roleClaim any = roleValues
	if len(roleValues) == 1 {
		roleClaim = roleValues[0]
	}

	claims := jwt.MapClaims{
		ClaimUserID:   user.ID.String(),
		ClaimUserName: deref(user.UserName),
		ClaimEmail:    deref(user.Email),
		ClaimRole:     roleClaim,
		"exp":         expiresAt.Unix(),
		"iss":         s.cfg.JWTIssuer,
		"aud":         s.cfg.JWTAudience,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiresAt, nil
}

// ErrInvalidToken gộp mọi lý do token không dùng được (sai chữ ký, hết hạn, thiếu claim...).
var ErrInvalidToken = errors.New("invalid token")

// ParseAccessToken kiểm tra chữ ký HS256, issuer, audience, thời hạn (kèm ClockSkew).
// Dùng chung cho middleware HTTP và endpoint WebSocket.
func (s *TokenService) ParseAccessToken(tokenStr string) (*AccessClaims, error) {
	token, err := jwt.Parse(tokenStr,
		func(*jwt.Token) (any, error) { return []byte(s.cfg.JWTSecret), nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(s.cfg.JWTIssuer),
		jwt.WithAudience(s.cfg.JWTAudience),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(ClockSkew),
	)
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	mc, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidToken
	}

	idStr, _ := mc[ClaimUserID].(string)
	userID, err := uuid.Parse(idStr)
	if err != nil {
		return nil, ErrInvalidToken
	}
	userName, _ := mc[ClaimUserName].(string)
	email, _ := mc[ClaimEmail].(string)

	return &AccessClaims{
		UserID:   userID,
		UserName: userName,
		Email:    email,
		Roles:    claimStrings(mc[ClaimRole]),
	}, nil
}

// GenerateRefreshToken sinh 64 byte ngẫu nhiên, mã hóa base64 (khớp C#).
func (s *TokenService) GenerateRefreshToken() string {
	b := make([]byte, 64)
	rand.Read(b) // từ Go 1.24 crypto/rand.Read không bao giờ trả lỗi
	return base64.StdEncoding.EncodeToString(b)
}

// RefreshExpiry là thời điểm hết hạn cho refresh token mới cấp.
func (s *TokenService) RefreshExpiry(now time.Time) time.Time {
	return now.AddDate(0, 0, s.cfg.JWTRefreshTokenExpiryDays)
}

func claimStrings(v any) []string {
	switch val := v.(type) {
	case string:
		return []string{val}
	case []any:
		out := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func deref(p *string) string {
	if p != nil {
		return *p
	}
	return ""
}

// NewUUID sinh UUID v7 (theo thời gian) — .NET 10 Identity/EF cũng dùng UUIDv7.
func NewUUID() uuid.UUID {
	return uuid.Must(uuid.NewV7())
}
