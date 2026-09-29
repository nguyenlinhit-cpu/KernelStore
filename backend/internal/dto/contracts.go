package dto

import (
	"regexp"
	"time"

	"github.com/google/uuid"

	"github.com/KernelStore/backend/internal/money"
	"github.com/KernelStore/backend/internal/validate"
)

// Quy ước: mỗi request có Validate() mô tả đúng các DataAnnotations của class C# tương ứng,
// tên field trong lỗi là tên property C# (PascalCase) như ModelState của ASP.NET.

// ─── Auth ────────────────────────────────────────────────────────────────────

type RegisterRequest struct {
	FullName string `json:"fullName"`
	Email    string `json:"email"`
	UserName string `json:"userName"`
	Password string `json:"password"`
}

func (r *RegisterRequest) Validate(e *validate.Errors) {
	e.Required("FullName", r.FullName, "FullName là bắt buộc")
	e.StringLength("FullName", r.FullName, 2, 200, "")
	e.Required("Email", r.Email, "Email là bắt buộc")
	e.Email("Email", r.Email)
	e.Required("UserName", r.UserName, "UserName là bắt buộc")
	e.StringLength("UserName", r.UserName, 3, 50, "")
	e.Required("Password", r.Password, "Password là bắt buộc")
	e.MinLength("Password", r.Password, 6)
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r *LoginRequest) Validate(e *validate.Errors) {
	e.Required("Email", r.Email, "Email là bắt buộc")
	e.Email("Email", r.Email)
	e.Required("Password", r.Password, "Password là bắt buộc")
}

type RefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

func (r *RefreshRequest) Validate(e *validate.Errors) {
	e.Required("RefreshToken", r.RefreshToken, "RefreshToken là bắt buộc")
}

type AuthResponse struct {
	AccessToken  string      `json:"accessToken"`
	RefreshToken string      `json:"refreshToken"`
	ExpiresAt    time.Time   `json:"expiresAt"`
	User         UserInfoDto `json:"user"`
}

type UserInfoDto struct {
	ID        uuid.UUID `json:"id"`
	UserName  string    `json:"userName"`
	Email     string    `json:"email"`
	FullName  string    `json:"fullName"`
	AvatarUrl string    `json:"avatarUrl"`
	Role      string    `json:"role"`
	IsActive  bool      `json:"isActive"`
}

// ─── Shops ───────────────────────────────────────────────────────────────────

type CreateShopRequest struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

func (r *CreateShopRequest) Validate(e *validate.Errors) {
	validateShop(e, r.Name, r.Slug, r.Description)
}

type UpdateShopRequest struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

func (r *UpdateShopRequest) Validate(e *validate.Errors) {
	validateShop(e, r.Name, r.Slug, r.Description)
}

func validateShop(e *validate.Errors, name, slug, desc string) {
	e.Required("Name", name, "Tên shop là bắt buộc")
	e.StringLength("Name", name, 3, 200, "")
	e.Required("Slug", slug, "Slug là bắt buộc")
	e.Regex("Slug", slug, validate.Slug, validate.SlugMessage)
	e.StringLength("Slug", slug, 3, 200, "")
	e.StringLength("Description", desc, 0, 2000, "")
}

type ShopDto struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	LogoUrl     string    `json:"logoUrl"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	OwnerID     uuid.UUID `json:"ownerId"`
	OwnerName   string    `json:"ownerName"`
}

// ─── Categories ──────────────────────────────────────────────────────────────

type CreateCategoryRequest struct {
	Name        string     `json:"name"`
	Slug        string     `json:"slug"`
	Description string     `json:"description"`
	ParentID    *uuid.UUID `json:"parentId"`
}

func (r *CreateCategoryRequest) Validate(e *validate.Errors) {
	validateCategory(e, r.Name, r.Slug, r.Description)
}

type UpdateCategoryRequest struct {
	Name        string     `json:"name"`
	Slug        string     `json:"slug"`
	Description string     `json:"description"`
	ParentID    *uuid.UUID `json:"parentId"`
}

func (r *UpdateCategoryRequest) Validate(e *validate.Errors) {
	validateCategory(e, r.Name, r.Slug, r.Description)
}

func validateCategory(e *validate.Errors, name, slug, desc string) {
	e.Required("Name", name, "Tên danh mục là bắt buộc")
	e.StringLength("Name", name, 2, 150, "")
	e.Required("Slug", slug, "Slug là bắt buộc")
	e.Regex("Slug", slug, validate.Slug, validate.SlugMessage)
	e.StringLength("Slug", slug, 2, 150, "")
	e.StringLength("Description", desc, 0, 500, "")
}

// CategoryDto: Children = null ở API trả một category, = [] / danh sách ở API cây.
type CategoryDto struct {
	ID           uuid.UUID      `json:"id"`
	Name         string         `json:"name"`
	Slug         string         `json:"slug"`
	Description  string         `json:"description"`
	ParentID     *uuid.UUID     `json:"parentId"`
	ProductCount int            `json:"productCount"`
	Children     []*CategoryDto `json:"children"`
}

// ─── Products ────────────────────────────────────────────────────────────────

type CreateProductRequest struct {
	Name           string       `json:"name"`
	Slug           string       `json:"slug"`
	Description    string       `json:"description"`
	Price          money.Money  `json:"price"`
	SalePrice      *money.Money `json:"salePrice"`
	StockQuantity  int          `json:"stockQuantity"`
	Sku            string       `json:"sku"`
	WarrantyMonths int          `json:"warrantyMonths"`
	CategoryID     *uuid.UUID   `json:"categoryId"`
	Images         []string     `json:"images"`
}

func (r *CreateProductRequest) Validate(e *validate.Errors) {
	validateProduct(e, r.Name, r.Slug, r.Description, r.Price, r.SalePrice, r.StockQuantity, r.Sku, r.WarrantyMonths)
}

type UpdateProductRequest struct {
	Name           string       `json:"name"`
	Slug           string       `json:"slug"`
	Description    string       `json:"description"`
	Price          money.Money  `json:"price"`
	SalePrice      *money.Money `json:"salePrice"`
	StockQuantity  int          `json:"stockQuantity"`
	Sku            string       `json:"sku"`
	WarrantyMonths int          `json:"warrantyMonths"`
	CategoryID     *uuid.UUID   `json:"categoryId"`
	IsActive       bool         `json:"isActive"`
	Images         []string     `json:"images"`
}

// SetDefaults: C# khai báo `IsActive { get; set; } = true`.
func (r *UpdateProductRequest) SetDefaults() { r.IsActive = true }

func (r *UpdateProductRequest) Validate(e *validate.Errors) {
	validateProduct(e, r.Name, r.Slug, r.Description, r.Price, r.SalePrice, r.StockQuantity, r.Sku, r.WarrantyMonths)
}

func validateProduct(e *validate.Errors, name, slug, desc string, price money.Money, sale *money.Money, stock int, sku string, warranty int) {
	e.Required("Name", name, "Tên sản phẩm là bắt buộc")
	e.StringLength("Name", name, 3, 300, "")
	e.Required("Slug", slug, "Slug là bắt buộc")
	e.Regex("Slug", slug, validate.Slug, validate.SlugMessage)
	e.StringLength("Slug", slug, 3, 300, "")
	e.StringLength("Description", desc, 0, 4000, "")
	e.RangeMoney("Price", &price, 0, 99_999_999, "Giá không hợp lệ")
	e.RangeMoney("SalePrice", sale, 0, 99_999_999, "")
	e.RangeInt("StockQuantity", stock, 0, 1_000_000, "Tồn kho không hợp lệ")
	e.StringLength("Sku", sku, 0, 100, "")
	e.RangeInt("WarrantyMonths", warranty, 0, 120, "Thời hạn bảo hành từ 0 đến 120 tháng")
}

type ProductImageDto struct {
	ID           uuid.UUID `json:"id"`
	Url          string    `json:"url"`
	AltText      string    `json:"altText"`
	IsPrimary    bool      `json:"isPrimary"`
	DisplayOrder int       `json:"displayOrder"`
}

type ProductDto struct {
	ID             uuid.UUID         `json:"id"`
	Name           string            `json:"name"`
	Slug           string            `json:"slug"`
	Description    string            `json:"description"`
	Price          money.Money       `json:"price"`
	SalePrice      *money.Money      `json:"salePrice"`
	StockQuantity  int               `json:"stockQuantity"`
	Sku            string            `json:"sku"`
	WarrantyMonths int               `json:"warrantyMonths"`
	IsActive       bool              `json:"isActive"`
	CreatedAt      time.Time         `json:"createdAt"`
	ShopID         uuid.UUID         `json:"shopId"`
	ShopName       *string           `json:"shopName"`
	CategoryID     *uuid.UUID        `json:"categoryId"`
	CategoryName   *string           `json:"categoryName"`
	Images         []ProductImageDto `json:"images"`
}

type PagedResult[T any] struct {
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	Total      int `json:"total"`
	TotalPages int `json:"totalPages"`
	Items      []T `json:"items"`
}

type ReviewDto struct {
	ID        uuid.UUID `json:"id"`
	Rating    int       `json:"rating"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"createdAt"`
	UserID    uuid.UUID `json:"userId"`
	UserName  string    `json:"userName"`
}

type ShopSummaryDto struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	Description  string    `json:"description"`
	LogoUrl      string    `json:"logoUrl"`
	ProductCount int       `json:"productCount"`
}

type ProductDetailDto struct {
	ID             uuid.UUID         `json:"id"`
	Name           string            `json:"name"`
	Slug           string            `json:"slug"`
	Description    string            `json:"description"`
	Price          money.Money       `json:"price"`
	SalePrice      *money.Money      `json:"salePrice"`
	StockQuantity  int               `json:"stockQuantity"`
	Sku            string            `json:"sku"`
	WarrantyMonths int               `json:"warrantyMonths"`
	CreatedAt      time.Time         `json:"createdAt"`
	ShopID         uuid.UUID         `json:"shopId"`
	ShopName       *string           `json:"shopName"`
	CategoryID     *uuid.UUID        `json:"categoryId"`
	CategoryName   *string           `json:"categoryName"`
	Images         []ProductImageDto `json:"images"`
	Reviews        []ReviewDto       `json:"reviews"`
	Shop           ShopSummaryDto    `json:"shop"`
	AverageRating  float64           `json:"averageRating"`
	ReviewCount    int               `json:"reviewCount"`
}

// ─── Cart ────────────────────────────────────────────────────────────────────

type AddToCartRequest struct {
	ProductID uuid.UUID `json:"productId"`
	Quantity  int       `json:"quantity"`
}

// SetDefaults: C# khai báo `Quantity { get; set; } = 1`.
func (r *AddToCartRequest) SetDefaults() { r.Quantity = 1 }

// Validate: [Required] trên Guid không bao giờ lỗi ở C# (Guid.Empty vẫn "có giá trị").
func (r *AddToCartRequest) Validate(e *validate.Errors) {
	e.RangeInt("Quantity", r.Quantity, 1, 1000, "Số lượng phải từ 1 đến 1000")
}

type UpdateCartItemRequest struct {
	Quantity int `json:"quantity"`
}

func (r *UpdateCartItemRequest) Validate(e *validate.Errors) {
	e.RangeInt("Quantity", r.Quantity, 0, 1000, "Số lượng phải từ 0 đến 1000")
}

type CartItemDto struct {
	ID            uuid.UUID    `json:"id"`
	ProductID     uuid.UUID    `json:"productId"`
	Name          string       `json:"name"`
	Slug          string       `json:"slug"`
	Price         money.Money  `json:"price"`
	SalePrice     *money.Money `json:"salePrice"`
	UnitPrice     money.Money  `json:"unitPrice"`
	Quantity      int          `json:"quantity"`
	StockQuantity int          `json:"stockQuantity"`
	LineTotal     money.Money  `json:"lineTotal"`
	ImageUrl      *string      `json:"imageUrl"`
	ShopID        uuid.UUID    `json:"shopId"`
	ShopName      *string      `json:"shopName"`
}

type CartDto struct {
	Items      []CartItemDto `json:"items"`
	TotalItems int           `json:"totalItems"`
	Subtotal   money.Money   `json:"subtotal"`
}

// ─── Orders ──────────────────────────────────────────────────────────────────

type CreateOrderRequest struct {
	FullName string `json:"fullName"`
	Phone    string `json:"phone"`
	Street   string `json:"street"`
	Ward     string `json:"ward"`
	District string `json:"district"`
	City     string `json:"city"`
	Note     string `json:"note"`
}

var phonePattern = regexp.MustCompile(`^[0-9+\-\s]{8,20}$`)

func (r *CreateOrderRequest) Validate(e *validate.Errors) {
	e.Required("FullName", r.FullName, "Họ tên người nhận là bắt buộc")
	e.StringLength("FullName", r.FullName, 2, 200, "")
	e.Required("Phone", r.Phone, "Số điện thoại là bắt buộc")
	e.Regex("Phone", r.Phone, phonePattern, "Số điện thoại không hợp lệ")
	e.Required("Street", r.Street, "Địa chỉ là bắt buộc")
	e.StringLength("Street", r.Street, 2, 300, "")
	e.StringLength("Ward", r.Ward, 0, 150, "")
	e.StringLength("District", r.District, 0, 150, "")
	e.Required("City", r.City, "Tỉnh/Thành phố là bắt buộc")
	e.StringLength("City", r.City, 0, 150, "")
	e.StringLength("Note", r.Note, 0, 1000, "")
}

type UpdateOrderStatusRequest struct {
	Status string `json:"status"`
}

func (r *UpdateOrderStatusRequest) Validate(e *validate.Errors) {
	e.Required("Status", r.Status, "Trạng thái là bắt buộc")
}

type OrderAddressDto struct {
	FullName string `json:"fullName"`
	Phone    string `json:"phone"`
	Street   string `json:"street"`
	Ward     string `json:"ward"`
	District string `json:"district"`
	City     string `json:"city"`
}

type OrderItemDto struct {
	ID          uuid.UUID   `json:"id"`
	ProductID   uuid.UUID   `json:"productId"`
	ProductName string      `json:"productName"`
	ProductSlug string      `json:"productSlug"`
	ImageUrl    *string     `json:"imageUrl"`
	UnitPrice   money.Money `json:"unitPrice"`
	Quantity    int         `json:"quantity"`
	TotalPrice  money.Money `json:"totalPrice"`
	ShopID      uuid.UUID   `json:"shopId"`
	ShopName    *string     `json:"shopName"`
}

type OrderDto struct {
	ID          uuid.UUID       `json:"id"`
	OrderCode   string          `json:"orderCode"`
	Status      string          `json:"status"`
	TotalAmount money.Money     `json:"totalAmount"`
	ShippingFee money.Money     `json:"shippingFee"`
	Note        string          `json:"note"`
	CreatedAt   time.Time       `json:"createdAt"`
	PaidAt      *time.Time      `json:"paidAt"`
	Address     OrderAddressDto `json:"address"`
	Items       []OrderItemDto  `json:"items"`
	ItemCount   int             `json:"itemCount"`
	// True nếu người xem được quản lý trạng thái đơn (seller có hàng trong đơn, hoặc admin).
	CanManage bool `json:"canManage"`
}

// ─── Reviews ─────────────────────────────────────────────────────────────────

type ProductReviewsDto struct {
	Reviews       []ReviewDto `json:"reviews"`
	AverageRating float64     `json:"averageRating"`
	ReviewCount   int         `json:"reviewCount"`
}

type CreateReviewRequest struct {
	ProductID uuid.UUID `json:"productId"`
	Rating    int       `json:"rating"`
	Comment   string    `json:"comment"`
}

func (r *CreateReviewRequest) Validate(e *validate.Errors) {
	e.RangeInt("Rating", r.Rating, 1, 5, "Đánh giá phải từ 1 đến 5 sao")
	e.StringLength("Comment", r.Comment, 0, 1000, "Nhận xét tối đa 1000 ký tự")
}

// ─── Warranty ────────────────────────────────────────────────────────────────

// CreateWarrantyClaimRequest: khách gửi yêu cầu bảo hành cho một dòng sản phẩm đã mua.
type CreateWarrantyClaimRequest struct {
	OrderDetailID uuid.UUID `json:"orderDetailId"`
	Description   string    `json:"description"`
	ImageUrl      string    `json:"imageUrl"`
}

func (r *CreateWarrantyClaimRequest) Validate(e *validate.Errors) {
	e.Required("Description", r.Description, "Vui lòng mô tả tình trạng lỗi")
	e.StringLength("Description", r.Description, 10, 2000, "Mô tả từ 10 đến 2000 ký tự")
	e.StringLength("ImageUrl", r.ImageUrl, 0, 500, "")
}

// ApproveWarrantyRequest: shop/admin chấp nhận bảo hành và chọn hình thức xử lý.
type ApproveWarrantyRequest struct {
	Resolution string `json:"resolution"`
	Note       string `json:"note"`
}

func (r *ApproveWarrantyRequest) Validate(e *validate.Errors) {
	e.Required("Resolution", r.Resolution, "Chọn hình thức xử lý (Repair/Replace/Refund)")
	e.StringLength("Note", r.Note, 0, 1000, "")
}

// WarrantyNoteRequest: ghi chú kèm khi từ chối / hoàn tất.
type WarrantyNoteRequest struct {
	Note string `json:"note"`
}

func (r *WarrantyNoteRequest) Validate(e *validate.Errors) {
	e.StringLength("Note", r.Note, 0, 1000, "")
}

type WarrantyClaimDto struct {
	ID                uuid.UUID  `json:"id"`
	ClaimCode         string     `json:"claimCode"`
	Status            string     `json:"status"`
	Resolution        string     `json:"resolution"`
	ResolutionNote    string     `json:"resolutionNote"`
	Description       string     `json:"description"`
	ImageUrl          *string    `json:"imageUrl"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         *time.Time `json:"updatedAt"`
	ResolvedAt        *time.Time `json:"resolvedAt"`
	OrderDetailID     uuid.UUID  `json:"orderDetailId"`
	OrderID           uuid.UUID  `json:"orderId"`
	OrderCode         string     `json:"orderCode"`
	ProductID         uuid.UUID  `json:"productId"`
	ProductName       string     `json:"productName"`
	ProductSlug       string     `json:"productSlug"`
	ProductImageUrl   *string    `json:"productImageUrl"`
	Quantity          int        `json:"quantity"`
	WarrantyMonths    int        `json:"warrantyMonths"`
	WarrantyExpiresAt *time.Time `json:"warrantyExpiresAt"`
	ShopID            uuid.UUID  `json:"shopId"`
	ShopName          *string    `json:"shopName"`
	UserID            uuid.UUID  `json:"userId"`
	UserName          string     `json:"userName"`
	// Người xem là shop sở hữu / admin → được xử lý yêu cầu.
	CanManage bool `json:"canManage"`
}

// ─── Chat ────────────────────────────────────────────────────────────────────

type ConversationDto struct {
	ID        uuid.UUID `json:"id"`
	ShopID    uuid.UUID `json:"shopId"`
	ShopName  string    `json:"shopName"`
	BuyerID   uuid.UUID `json:"buyerId"`
	BuyerName string    `json:"buyerName"`
	// Tên phía đối thoại (với khách = tên shop, với seller = tên khách).
	OtherName     string    `json:"otherName"`
	LastMessage   *string   `json:"lastMessage"`
	LastMessageAt time.Time `json:"lastMessageAt"`
	UnreadCount   int       `json:"unreadCount"`
}

type ChatMessageDto struct {
	ID             uuid.UUID `json:"id"`
	ConversationID uuid.UUID `json:"conversationId"`
	SenderID       uuid.UUID `json:"senderId"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"createdAt"`
}

type StartConversationRequest struct {
	ShopID uuid.UUID `json:"shopId"`
}

func (r *StartConversationRequest) Validate(*validate.Errors) {}

type SendMessageRequest struct {
	Content string `json:"content"`
}

func (r *SendMessageRequest) Validate(e *validate.Errors) {
	e.Required("Content", r.Content, "Nội dung không được để trống")
	e.StringLength("Content", r.Content, 1, 2000, "")
}

// ─── Admin ───────────────────────────────────────────────────────────────────

type OrderStatusCount struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

type AdminOrderDto struct {
	ID            uuid.UUID   `json:"id"`
	OrderCode     string      `json:"orderCode"`
	Status        string      `json:"status"`
	TotalAmount   money.Money `json:"totalAmount"`
	ItemCount     int         `json:"itemCount"`
	CreatedAt     time.Time   `json:"createdAt"`
	CustomerID    uuid.UUID   `json:"customerId"`
	CustomerName  string      `json:"customerName"`
	CustomerEmail string      `json:"customerEmail"`
}

type AdminUserDto struct {
	ID        uuid.UUID `json:"id"`
	UserName  string    `json:"userName"`
	Email     string    `json:"email"`
	FullName  string    `json:"fullName"`
	Role      string    `json:"role"`
	IsActive  bool      `json:"isActive"`
	CreatedAt time.Time `json:"createdAt"`
}

type AdminDashboardDto struct {
	TotalUsers     int                `json:"totalUsers"`
	TotalShops     int                `json:"totalShops"`
	PendingShops   int                `json:"pendingShops"`
	ApprovedShops  int                `json:"approvedShops"`
	TotalProducts  int                `json:"totalProducts"`
	ActiveProducts int                `json:"activeProducts"`
	TotalOrders    int                `json:"totalOrders"`
	TotalRevenue   money.Money        `json:"totalRevenue"`
	OrdersByStatus []OrderStatusCount `json:"ordersByStatus"`
}

// ─── Seller ──────────────────────────────────────────────────────────────────

// TopProductStat: sản phẩm bán chạy của shop (theo doanh thu).
type TopProductStat struct {
	ProductID    uuid.UUID   `json:"productId"`
	Name         string      `json:"name"`
	QuantitySold int         `json:"quantitySold"`
	Revenue      money.Money `json:"revenue"`
}

// SellerDashboardDto: thống kê doanh thu + đơn hàng trong phạm vi shop của seller.
type SellerDashboardDto struct {
	TotalRevenue   money.Money        `json:"totalRevenue"`
	TotalOrders    int                `json:"totalOrders"`
	PendingOrders  int                `json:"pendingOrders"`
	ItemsSold      int                `json:"itemsSold"`
	TotalProducts  int                `json:"totalProducts"`
	ActiveProducts int                `json:"activeProducts"`
	OrdersByStatus []OrderStatusCount `json:"ordersByStatus"`
	TopProducts    []TopProductStat   `json:"topProducts"`
}
