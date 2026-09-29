package client

// Kiểu dữ liệu khớp JSON backend (tương đương các struct trong api.rs).
// Giá tiền dùng float64 như bản Rust (f64) vì chỉ để hiển thị / gửi lên.

type UserInfo struct {
	ID        string `json:"id"`
	UserName  string `json:"userName"`
	Email     string `json:"email"`
	FullName  string `json:"fullName"`
	AvatarUrl string `json:"avatarUrl"`
	Role      string `json:"role"`
	IsActive  bool   `json:"isActive"`
}

type AuthData struct {
	AccessToken  string   `json:"accessToken"`
	RefreshToken string   `json:"refreshToken"`
	ExpiresAt    string   `json:"expiresAt"`
	User         UserInfo `json:"user"`
}

type RegisterPayload struct {
	FullName string `json:"fullName"`
	Email    string `json:"email"`
	UserName string `json:"userName"`
	Password string `json:"password"`
}

type ShopPayload struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

type ShopInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	LogoUrl     string `json:"logoUrl"`
	Status      string `json:"status"`
	CreatedAt   string `json:"createdAt"`
	OwnerID     string `json:"ownerId"`
	OwnerName   string `json:"ownerName"`
}

type OrderStatusCount struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

type DashboardStats struct {
	TotalUsers     int                `json:"totalUsers"`
	TotalShops     int                `json:"totalShops"`
	PendingShops   int                `json:"pendingShops"`
	ApprovedShops  int                `json:"approvedShops"`
	TotalProducts  int                `json:"totalProducts"`
	ActiveProducts int                `json:"activeProducts"`
	TotalOrders    int                `json:"totalOrders"`
	TotalRevenue   float64            `json:"totalRevenue"`
	OrdersByStatus []OrderStatusCount `json:"ordersByStatus"`
}

type TopProductStat struct {
	ProductID    string  `json:"productId"`
	Name         string  `json:"name"`
	QuantitySold int     `json:"quantitySold"`
	Revenue      float64 `json:"revenue"`
}

type SellerDashboard struct {
	TotalRevenue   float64            `json:"totalRevenue"`
	TotalOrders    int                `json:"totalOrders"`
	PendingOrders  int                `json:"pendingOrders"`
	ItemsSold      int                `json:"itemsSold"`
	TotalProducts  int                `json:"totalProducts"`
	ActiveProducts int                `json:"activeProducts"`
	OrdersByStatus []OrderStatusCount `json:"ordersByStatus"`
	TopProducts    []TopProductStat   `json:"topProducts"`
}

type ProductPayload struct {
	Name           string   `json:"name"`
	Slug           string   `json:"slug"`
	Description    string   `json:"description"`
	Price          float64  `json:"price"`
	SalePrice      *float64 `json:"salePrice"`
	StockQuantity  int      `json:"stockQuantity"`
	Sku            string   `json:"sku"`
	WarrantyMonths int      `json:"warrantyMonths"`
	CategoryID     *string  `json:"categoryId,omitempty"`
	IsActive       bool     `json:"isActive"`
	Images         []string `json:"images"`
}

type ProductImage struct {
	ID           string `json:"id"`
	Url          string `json:"url"`
	AltText      string `json:"altText"`
	IsPrimary    bool   `json:"isPrimary"`
	DisplayOrder int    `json:"displayOrder"`
}

// Product dùng cho cả ProductCard, ProductInfo của api.rs (cùng JSON ProductDto).
type Product struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Slug           string         `json:"slug"`
	Description    string         `json:"description"`
	Price          float64        `json:"price"`
	SalePrice      *float64       `json:"salePrice"`
	StockQuantity  int            `json:"stockQuantity"`
	Sku            string         `json:"sku"`
	WarrantyMonths int            `json:"warrantyMonths"`
	IsActive       bool           `json:"isActive"`
	CreatedAt      string         `json:"createdAt"`
	ShopID         string         `json:"shopId"`
	ShopName       *string        `json:"shopName"`
	CategoryID     *string        `json:"categoryId"`
	CategoryName   *string        `json:"categoryName"`
	Images         []ProductImage `json:"images"`
}

// PrimaryImage: ảnh primary, không có thì ảnh đầu (như ProductCard::primary_image).
func PrimaryImage(images []ProductImage) string {
	for _, i := range images {
		if i.IsPrimary {
			return i.Url
		}
	}
	if len(images) > 0 {
		return images[0].Url
	}
	return ""
}

type PagedResult[T any] struct {
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	Total      int `json:"total"`
	TotalPages int `json:"totalPages"`
	Items      []T `json:"items"`
}

type ProductQuery struct {
	Category, Shop, Search, Sort string
	MinPrice, MaxPrice           *float64
	Page, PageSize               int
}

type CategoryNode struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Slug         string          `json:"slug"`
	Description  string          `json:"description"`
	ParentID     *string         `json:"parentId"`
	ProductCount int             `json:"productCount"`
	Children     []*CategoryNode `json:"children"`
}

type CategoryPayload struct {
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Description string  `json:"description"`
	ParentID    *string `json:"parentId,omitempty"`
}

type Review struct {
	ID        string `json:"id"`
	Rating    int    `json:"rating"`
	Comment   string `json:"comment"`
	CreatedAt string `json:"createdAt"`
	UserID    string `json:"userId"`
	UserName  string `json:"userName"`
}

type ShopSummary struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	Description  string `json:"description"`
	LogoUrl      string `json:"logoUrl"`
	ProductCount int    `json:"productCount"`
}

type ProductDetail struct {
	Product
	Reviews       []Review    `json:"reviews"`
	Shop          ShopSummary `json:"shop"`
	AverageRating float64     `json:"averageRating"`
	ReviewCount   int         `json:"reviewCount"`
}

type ProductReviews struct {
	Reviews       []Review `json:"reviews"`
	AverageRating float64  `json:"averageRating"`
	ReviewCount   int      `json:"reviewCount"`
}

type CartItem struct {
	ID            string   `json:"id"`
	ProductID     string   `json:"productId"`
	Name          string   `json:"name"`
	Slug          string   `json:"slug"`
	Price         float64  `json:"price"`
	SalePrice     *float64 `json:"salePrice"`
	UnitPrice     float64  `json:"unitPrice"`
	Quantity      int      `json:"quantity"`
	StockQuantity int      `json:"stockQuantity"`
	LineTotal     float64  `json:"lineTotal"`
	ImageUrl      *string  `json:"imageUrl"`
	ShopID        string   `json:"shopId"`
	ShopName      *string  `json:"shopName"`
}

type Cart struct {
	Items      []CartItem `json:"items"`
	TotalItems int        `json:"totalItems"`
	Subtotal   float64    `json:"subtotal"`
}

type OrderPayload struct {
	FullName string `json:"fullName"`
	Phone    string `json:"phone"`
	Street   string `json:"street"`
	Ward     string `json:"ward"`
	District string `json:"district"`
	City     string `json:"city"`
	Note     string `json:"note"`
}

type OrderAddress struct {
	FullName string `json:"fullName"`
	Phone    string `json:"phone"`
	Street   string `json:"street"`
	Ward     string `json:"ward"`
	District string `json:"district"`
	City     string `json:"city"`
}

type OrderItem struct {
	ID          string  `json:"id"`
	ProductID   string  `json:"productId"`
	ProductName string  `json:"productName"`
	ProductSlug string  `json:"productSlug"`
	ImageUrl    *string `json:"imageUrl"`
	UnitPrice   float64 `json:"unitPrice"`
	Quantity    int     `json:"quantity"`
	TotalPrice  float64 `json:"totalPrice"`
	ShopID      string  `json:"shopId"`
	ShopName    *string `json:"shopName"`
}

type Order struct {
	ID          string       `json:"id"`
	OrderCode   string       `json:"orderCode"`
	Status      string       `json:"status"`
	TotalAmount float64      `json:"totalAmount"`
	ShippingFee float64      `json:"shippingFee"`
	Note        string       `json:"note"`
	CreatedAt   string       `json:"createdAt"`
	PaidAt      *string      `json:"paidAt"`
	Address     OrderAddress `json:"address"`
	Items       []OrderItem  `json:"items"`
	ItemCount   int          `json:"itemCount"`
	CanManage   bool         `json:"canManage"`
}

type WarrantyPayload struct {
	OrderDetailID string `json:"orderDetailId"`
	Description   string `json:"description"`
	ImageUrl      string `json:"imageUrl"`
}

type WarrantyClaim struct {
	ID                string  `json:"id"`
	ClaimCode         string  `json:"claimCode"`
	Status            string  `json:"status"`
	Resolution        string  `json:"resolution"`
	ResolutionNote    string  `json:"resolutionNote"`
	Description       string  `json:"description"`
	ImageUrl          *string `json:"imageUrl"`
	CreatedAt         string  `json:"createdAt"`
	UpdatedAt         *string `json:"updatedAt"`
	ResolvedAt        *string `json:"resolvedAt"`
	OrderDetailID     string  `json:"orderDetailId"`
	OrderID           string  `json:"orderId"`
	OrderCode         string  `json:"orderCode"`
	ProductID         string  `json:"productId"`
	ProductName       string  `json:"productName"`
	ProductSlug       string  `json:"productSlug"`
	ProductImageUrl   *string `json:"productImageUrl"`
	Quantity          int     `json:"quantity"`
	WarrantyMonths    int     `json:"warrantyMonths"`
	WarrantyExpiresAt *string `json:"warrantyExpiresAt"`
	ShopID            string  `json:"shopId"`
	ShopName          *string `json:"shopName"`
	UserID            string  `json:"userId"`
	UserName          string  `json:"userName"`
	CanManage         bool    `json:"canManage"`
}

type Conversation struct {
	ID            string  `json:"id"`
	ShopID        string  `json:"shopId"`
	ShopName      string  `json:"shopName"`
	BuyerID       string  `json:"buyerId"`
	BuyerName     string  `json:"buyerName"`
	OtherName     string  `json:"otherName"`
	LastMessage   *string `json:"lastMessage"`
	LastMessageAt string  `json:"lastMessageAt"`
	UnreadCount   int     `json:"unreadCount"`
}

type ChatMessage struct {
	ID             string `json:"id"`
	ConversationID string `json:"conversationId"`
	SenderID       string `json:"senderId"`
	Content        string `json:"content"`
	CreatedAt      string `json:"createdAt"`
}
