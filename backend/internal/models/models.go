// Package models chứa các struct tương ứng với bảng PostgreSQL.
// Giữ nguyên tên bảng/cột PascalCase từ schema C# (EF Core Identity + domain).
package models

import (
	"time"

	"github.com/KernelStore/backend/internal/money"
	"github.com/google/uuid"
)

// ─── Entities ────────────────────────────────────────────────────────────────
// Tên trường khớp PascalCase cột trong database.sql.

// ApplicationUser — bảng "AspNetUsers".
type ApplicationUser struct {
	ID                   uuid.UUID  `db:"Id"`
	FullName             string     `db:"FullName"`
	AvatarUrl            string     `db:"AvatarUrl"`
	CreatedAt            time.Time  `db:"CreatedAt"`
	IsActive             bool       `db:"IsActive"`
	Role                 UserRole   `db:"Role"`
	UserName             *string    `db:"UserName"`
	NormalizedUserName   *string    `db:"NormalizedUserName"`
	Email                *string    `db:"Email"`
	NormalizedEmail      *string    `db:"NormalizedEmail"`
	EmailConfirmed       bool       `db:"EmailConfirmed"`
	PasswordHash         *string    `db:"PasswordHash"`
	SecurityStamp        *string    `db:"SecurityStamp"`
	ConcurrencyStamp     *string    `db:"ConcurrencyStamp"`
	PhoneNumber          *string    `db:"PhoneNumber"`
	PhoneNumberConfirmed bool       `db:"PhoneNumberConfirmed"`
	TwoFactorEnabled     bool       `db:"TwoFactorEnabled"`
	LockoutEnd           *time.Time `db:"LockoutEnd"`
	LockoutEnabled       bool       `db:"LockoutEnabled"`
	AccessFailedCount    int        `db:"AccessFailedCount"`
}

// Shop — bảng "Shops".
type Shop struct {
	ID          uuid.UUID  `db:"Id"`
	Name        string     `db:"Name"`
	Slug        string     `db:"Slug"`
	Description string     `db:"Description"`
	LogoUrl     string     `db:"LogoUrl"`
	Status      ShopStatus `db:"Status"`
	CreatedAt   time.Time  `db:"CreatedAt"`
	OwnerID     uuid.UUID  `db:"OwnerId"`
}

// Category — bảng "Categories".
type Category struct {
	ID          uuid.UUID  `db:"Id"`
	Name        string     `db:"Name"`
	Slug        string     `db:"Slug"`
	Description string     `db:"Description"`
	ParentID    *uuid.UUID `db:"ParentId"`
	OwnerShopID *uuid.UUID `db:"OwnerShopId"`
}

// Product — bảng "Products".
type Product struct {
	ID             uuid.UUID    `db:"Id"`
	Name           string       `db:"Name"`
	Slug           string       `db:"Slug"`
	Description    string       `db:"Description"`
	Price          money.Money  `db:"Price"`
	SalePrice      *money.Money `db:"SalePrice"`
	StockQuantity  int          `db:"StockQuantity"`
	Sku            string       `db:"Sku"`
	WarrantyMonths int          `db:"WarrantyMonths"`
	IsActive       bool         `db:"IsActive"`
	CreatedAt      time.Time    `db:"CreatedAt"`
	CategoryID     *uuid.UUID   `db:"CategoryId"`
	ShopID         uuid.UUID    `db:"ShopId"`
}

// ProductImage — bảng "ProductImages".
type ProductImage struct {
	ID           uuid.UUID `db:"Id"`
	Url          string    `db:"Url"`
	AltText      string    `db:"AltText"`
	IsPrimary    bool      `db:"IsPrimary"`
	DisplayOrder int       `db:"DisplayOrder"`
	ProductID    uuid.UUID `db:"ProductId"`
}

// Address — bảng "Addresses".
type Address struct {
	ID        uuid.UUID `db:"Id"`
	FullName  string    `db:"FullName"`
	Phone     string    `db:"Phone"`
	Street    string    `db:"Street"`
	Ward      string    `db:"Ward"`
	District  string    `db:"District"`
	City      string    `db:"City"`
	IsDefault bool      `db:"IsDefault"`
	UserID    uuid.UUID `db:"UserId"`
}

// Order — bảng "Orders".
type Order struct {
	ID          uuid.UUID   `db:"Id"`
	OrderCode   string      `db:"OrderCode"`
	Status      OrderStatus `db:"Status"`
	TotalAmount money.Money `db:"TotalAmount"`
	ShippingFee money.Money `db:"ShippingFee"`
	Note        string      `db:"Note"`
	CreatedAt   time.Time   `db:"CreatedAt"`
	PaidAt      *time.Time  `db:"PaidAt"`
	UserID      uuid.UUID   `db:"UserId"`
	AddressID   uuid.UUID   `db:"AddressId"`
}

// OrderDetail — bảng "OrderDetails".
type OrderDetail struct {
	ID         uuid.UUID   `db:"Id"`
	Quantity   int         `db:"Quantity"`
	UnitPrice  money.Money `db:"UnitPrice"`
	TotalPrice money.Money `db:"TotalPrice"`
	OrderID    uuid.UUID   `db:"OrderId"`
	ProductID  uuid.UUID   `db:"ProductId"`
}

// Review — bảng "Reviews".
type Review struct {
	ID        uuid.UUID `db:"Id"`
	Rating    int       `db:"Rating"`
	Comment   string    `db:"Comment"`
	CreatedAt time.Time `db:"CreatedAt"`
	ProductID uuid.UUID `db:"ProductId"`
	UserID    uuid.UUID `db:"UserId"`
}

// CartItem — bảng "CartItems".
type CartItem struct {
	ID        uuid.UUID `db:"Id"`
	Quantity  int       `db:"Quantity"`
	UserID    uuid.UUID `db:"UserId"`
	ProductID uuid.UUID `db:"ProductId"`
}

// RefreshToken — bảng "RefreshTokens".
type RefreshToken struct {
	ID        uuid.UUID `db:"Id"`
	Token     string    `db:"Token"`
	ExpiresAt time.Time `db:"ExpiresAt"`
	CreatedAt time.Time `db:"CreatedAt"`
	IsRevoked bool      `db:"IsRevoked"`
	IsUsed    bool      `db:"IsUsed"`
	UserID    uuid.UUID `db:"UserId"`
}

// Conversation — bảng "Conversations" (chat giữa buyer và shop).
type Conversation struct {
	ID            uuid.UUID `db:"Id"`
	BuyerID       uuid.UUID `db:"BuyerId"`
	ShopID        uuid.UUID `db:"ShopId"`
	CreatedAt     time.Time `db:"CreatedAt"`
	LastMessageAt time.Time `db:"LastMessageAt"`
}

// ChatMessage — bảng "ChatMessages".
type ChatMessage struct {
	ID             uuid.UUID `db:"Id"`
	ConversationID uuid.UUID `db:"ConversationId"`
	SenderID       uuid.UUID `db:"SenderId"`
	Content        string    `db:"Content"`
	IsRead         bool      `db:"IsRead"`
	CreatedAt      time.Time `db:"CreatedAt"`
}

// WarrantyClaim — bảng "WarrantyClaims".
type WarrantyClaim struct {
	ID             uuid.UUID          `db:"Id"`
	ClaimCode      string             `db:"ClaimCode"`
	Description    string             `db:"Description"`
	ImageUrl       string             `db:"ImageUrl"`
	Status         WarrantyStatus     `db:"Status"`
	Resolution     WarrantyResolution `db:"Resolution"`
	ResolutionNote string             `db:"ResolutionNote"`
	CreatedAt      time.Time          `db:"CreatedAt"`
	UpdatedAt      *time.Time         `db:"UpdatedAt"`
	ResolvedAt     *time.Time         `db:"ResolvedAt"`
	OrderDetailID  uuid.UUID          `db:"OrderDetailId"`
	UserID         uuid.UUID          `db:"UserId"`
	ProductID      uuid.UUID          `db:"ProductId"`
	ShopID         uuid.UUID          `db:"ShopId"`
}

// ─── ASP.NET Identity tables (chỉ dùng bảng Roles + UserRoles) ──────────────

// AspNetRole — bảng "AspNetRoles".
type AspNetRole struct {
	ID               uuid.UUID `db:"Id"`
	Name             *string   `db:"Name"`
	NormalizedName   *string   `db:"NormalizedName"`
	ConcurrencyStamp *string   `db:"ConcurrencyStamp"`
}

// AspNetUserRole — bảng "AspNetUserRoles".
type AspNetUserRole struct {
	UserID uuid.UUID `db:"UserId"`
	RoleID uuid.UUID `db:"RoleId"`
}
