package client

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/url"
	"strconv"
	"strings"
)

// ─── Auth ────────────────────────────────────────────────────────────────────

func (c *Client) Register(ctx context.Context, p RegisterPayload) (*AuthData, error) {
	return one[AuthData](c, ctx, req{method: "POST", path: "/auth/register", body: p})
}

func (c *Client) Login(ctx context.Context, email, password string) (*AuthData, error) {
	body := map[string]string{"email": email, "password": password}
	return one[AuthData](c, ctx, req{method: "POST", path: "/auth/login", body: body})
}

func (c *Client) Refresh(ctx context.Context, refreshToken string) (*AuthData, error) {
	body := map[string]string{"refreshToken": refreshToken}
	return one[AuthData](c, ctx, req{method: "POST", path: "/auth/refresh", body: body})
}

func (c *Client) Me(ctx context.Context, token string) (*UserInfo, error) {
	var me struct {
		Info  *UserInfo `json:"info"`
		Roles []string  `json:"roles"`
	}
	if _, err := c.do(ctx, req{method: "GET", path: "/auth/me", token: token}, &me); err != nil {
		return nil, err
	}
	if me.Info == nil {
		return nil, noData
	}
	return me.Info, nil
}

// ─── Upload ──────────────────────────────────────────────────────────────────

// UploadImage chuyển tiếp file ảnh người dùng chọn lên backend, trả URL công khai.
func (c *Client) UploadImage(ctx context.Context, token, filename string, file io.Reader) (string, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", filename)
	if err == nil {
		_, err = io.Copy(part, file)
	}
	if err == nil {
		err = mw.Close()
	}
	if err != nil {
		return "", &Error{Kind: ErrNetwork, Msg: err.Error()}
	}
	var out struct {
		Url string `json:"url"`
	}
	if _, err := c.do(ctx, req{method: "POST", path: "/uploads/image", token: token,
		raw: buf.Bytes(), contentType: mw.FormDataContentType()}, &out); err != nil {
		return "", err
	}
	if out.Url == "" {
		return "", &Error{Kind: ErrServer, Msg: "no url"}
	}
	return out.Url, nil
}

// ─── Shops ───────────────────────────────────────────────────────────────────

func (c *Client) CreateShop(ctx context.Context, token string, p ShopPayload) (*ShopInfo, error) {
	return one[ShopInfo](c, ctx, req{method: "POST", path: "/shops", token: token, body: p})
}

// GetMyShop: chưa có shop → (nil, nil).
func (c *Client) GetMyShop(ctx context.Context, token string) (*ShopInfo, error) {
	var s *ShopInfo
	_, err := c.do(ctx, req{method: "GET", path: "/shops/me", token: token}, &s)
	return s, err
}

func (c *Client) UpdateShop(ctx context.Context, token string, p ShopPayload) (*ShopInfo, error) {
	return one[ShopInfo](c, ctx, req{method: "PUT", path: "/shops/me", token: token, body: p})
}

func (c *Client) ListShops(ctx context.Context, token, status string) ([]ShopInfo, error) {
	path := "/admin/shops"
	if status != "" {
		path += "?status=" + url.QueryEscape(status)
	}
	return many[ShopInfo](c, ctx, req{method: "GET", path: path, token: token})
}

// AdminShopAction: approve | reject | ban | unban.
func (c *Client) AdminShopAction(ctx context.Context, token, id, action string) (*ShopInfo, error) {
	return one[ShopInfo](c, ctx, req{method: "POST", path: "/admin/shops/" + id + "/" + action, token: token})
}

// AdminDeleteShop trả message của server (khác nhau giữa xoá cứng và xoá mềm).
func (c *Client) AdminDeleteShop(ctx context.Context, token, id string) (string, error) {
	return c.do(ctx, req{method: "DELETE", path: "/admin/shops/" + id, token: token}, nil)
}

// ─── Dashboards ──────────────────────────────────────────────────────────────

func (c *Client) AdminDashboard(ctx context.Context, token string) (*DashboardStats, error) {
	return one[DashboardStats](c, ctx, req{method: "GET", path: "/admin/dashboard", token: token})
}

func (c *Client) SellerDashboard(ctx context.Context, token string) (*SellerDashboard, error) {
	return one[SellerDashboard](c, ctx, req{method: "GET", path: "/seller/dashboard", token: token, map404: true})
}

// ─── Products ────────────────────────────────────────────────────────────────

func (c *Client) CreateProduct(ctx context.Context, token string, p ProductPayload) (*Product, error) {
	return one[Product](c, ctx, req{method: "POST", path: "/products", token: token, body: p})
}

func (c *Client) UpdateProduct(ctx context.Context, token, id string, p ProductPayload) (*Product, error) {
	return one[Product](c, ctx, req{method: "PUT", path: "/products/" + id, token: token, body: p})
}

func (c *Client) DeleteProduct(ctx context.Context, token, id string) error {
	_, err := c.do(ctx, req{method: "DELETE", path: "/products/" + id, token: token}, nil)
	return err
}

func (c *Client) ListMyProducts(ctx context.Context, token string) ([]Product, error) {
	return many[Product](c, ctx, req{method: "GET", path: "/products/my", token: token})
}

// ListProducts ghép query giống api.rs (chỉ gửi tham số có giá trị).
func (c *Client) ListProducts(ctx context.Context, q ProductQuery) (*PagedResult[Product], error) {
	var parts []string
	add := func(k, v string) { parts = append(parts, k+"="+url.QueryEscape(v)) }
	if q.Category != "" {
		add("category", q.Category)
	}
	if q.Shop != "" {
		add("shop", q.Shop)
	}
	if q.MinPrice != nil {
		add("minPrice", strconv.FormatFloat(*q.MinPrice, 'f', -1, 64))
	}
	if q.MaxPrice != nil {
		add("maxPrice", strconv.FormatFloat(*q.MaxPrice, 'f', -1, 64))
	}
	if q.Search != "" {
		add("search", q.Search)
	}
	if q.Sort != "" {
		add("sort", q.Sort)
	}
	if q.Page > 0 {
		add("page", strconv.Itoa(q.Page))
	}
	if q.PageSize > 0 {
		add("pageSize", strconv.Itoa(q.PageSize))
	}
	path := "/products"
	if len(parts) > 0 {
		path += "?" + strings.Join(parts, "&")
	}
	return one[PagedResult[Product]](c, ctx, req{method: "GET", path: path})
}

func (c *Client) FeaturedProducts(ctx context.Context, take int) ([]Product, error) {
	return many[Product](c, ctx, req{method: "GET", path: "/products/featured?take=" + strconv.Itoa(take)})
}

func (c *Client) GetProduct(ctx context.Context, slug string) (*ProductDetail, error) {
	return one[ProductDetail](c, ctx, req{method: "GET", path: "/products/" + url.PathEscape(slug), map404: true})
}

// ─── Categories ──────────────────────────────────────────────────────────────

func (c *Client) ListCategories(ctx context.Context) ([]CategoryNode, error) {
	return many[CategoryNode](c, ctx, req{method: "GET", path: "/categories"})
}

func (c *Client) CreateCategory(ctx context.Context, token string, p CategoryPayload) (*CategoryNode, error) {
	return one[CategoryNode](c, ctx, req{method: "POST", path: "/categories", token: token, body: p})
}

func (c *Client) UpdateCategory(ctx context.Context, token, id string, p CategoryPayload) (*CategoryNode, error) {
	return one[CategoryNode](c, ctx, req{method: "PUT", path: "/categories/" + id, token: token, body: p})
}

func (c *Client) DeleteCategory(ctx context.Context, token, id string) error {
	_, err := c.do(ctx, req{method: "DELETE", path: "/categories/" + id, token: token}, nil)
	return err
}

func (c *Client) ListMyCategories(ctx context.Context, token string) ([]CategoryNode, error) {
	return many[CategoryNode](c, ctx, req{method: "GET", path: "/seller/categories", token: token})
}

func (c *Client) CreateMyCategory(ctx context.Context, token string, p CategoryPayload) (*CategoryNode, error) {
	return one[CategoryNode](c, ctx, req{method: "POST", path: "/seller/categories", token: token, body: p})
}

func (c *Client) UpdateMyCategory(ctx context.Context, token, id string, p CategoryPayload) (*CategoryNode, error) {
	return one[CategoryNode](c, ctx, req{method: "PUT", path: "/seller/categories/" + id, token: token, body: p})
}

func (c *Client) DeleteMyCategory(ctx context.Context, token, id string) error {
	_, err := c.do(ctx, req{method: "DELETE", path: "/seller/categories/" + id, token: token}, nil)
	return err
}

// ─── Reviews ─────────────────────────────────────────────────────────────────

func (c *Client) CreateReview(ctx context.Context, token, productID string, rating int, comment string) (*Review, error) {
	body := map[string]any{"productId": productID, "rating": rating, "comment": comment}
	return one[Review](c, ctx, req{method: "POST", path: "/reviews", token: token, body: body})
}

func (c *Client) GetReviews(ctx context.Context, productID string) (*ProductReviews, error) {
	return one[ProductReviews](c, ctx, req{method: "GET", path: "/reviews?productId=" + url.QueryEscape(productID)})
}

// ─── Cart ────────────────────────────────────────────────────────────────────

func (c *Client) GetCart(ctx context.Context, token string) (*Cart, error) {
	return one[Cart](c, ctx, req{method: "GET", path: "/cart", token: token})
}

func (c *Client) AddToCart(ctx context.Context, token, productID string, qty int) (*Cart, error) {
	body := map[string]any{"productId": productID, "quantity": qty}
	return one[Cart](c, ctx, req{method: "POST", path: "/cart", token: token, body: body})
}

func (c *Client) UpdateCartItem(ctx context.Context, token, productID string, qty int) (*Cart, error) {
	return one[Cart](c, ctx, req{method: "PUT", path: "/cart/" + productID, token: token, body: map[string]int{"quantity": qty}})
}

func (c *Client) DeleteCartItem(ctx context.Context, token, productID string) (*Cart, error) {
	return one[Cart](c, ctx, req{method: "DELETE", path: "/cart/" + productID, token: token})
}

// ─── Orders ──────────────────────────────────────────────────────────────────

func (c *Client) CreateOrder(ctx context.Context, token string, p OrderPayload) (*Order, error) {
	return one[Order](c, ctx, req{method: "POST", path: "/orders", token: token, body: p})
}

func (c *Client) ListOrders(ctx context.Context, token string) ([]Order, error) {
	return many[Order](c, ctx, req{method: "GET", path: "/orders", token: token})
}

func (c *Client) ListSellerSales(ctx context.Context, token, status string) ([]Order, error) {
	path := "/orders/sales"
	if status != "" {
		path += "?status=" + url.QueryEscape(status)
	}
	return many[Order](c, ctx, req{method: "GET", path: path, token: token, map404: true})
}

func (c *Client) GetOrder(ctx context.Context, token, id string) (*Order, error) {
	return one[Order](c, ctx, req{method: "GET", path: "/orders/" + id, token: token, map404: true})
}

func (c *Client) UpdateOrderStatus(ctx context.Context, token, id, status string) (*Order, error) {
	return one[Order](c, ctx, req{method: "PUT", path: "/orders/" + id + "/status", token: token,
		body: map[string]string{"status": status}, map404: true})
}

// OrderAction: confirm-received | cancel | return | return/approve | return/reject.
func (c *Client) OrderAction(ctx context.Context, token, id, action string) (*Order, error) {
	return one[Order](c, ctx, req{method: "POST", path: "/orders/" + id + "/" + action, token: token, map404: true})
}

// ─── Warranty ────────────────────────────────────────────────────────────────

func (c *Client) CreateWarranty(ctx context.Context, token string, p WarrantyPayload) (*WarrantyClaim, error) {
	return one[WarrantyClaim](c, ctx, req{method: "POST", path: "/warranty", token: token, body: p, map404: true})
}

func (c *Client) ListMyWarranty(ctx context.Context, token string) ([]WarrantyClaim, error) {
	return many[WarrantyClaim](c, ctx, req{method: "GET", path: "/warranty/mine", token: token})
}

func (c *Client) ListShopWarranty(ctx context.Context, token, status string) ([]WarrantyClaim, error) {
	path := "/warranty/shop"
	if status != "" {
		path += "?status=" + url.QueryEscape(status)
	}
	return many[WarrantyClaim](c, ctx, req{method: "GET", path: path, token: token})
}

// WarrantyAction: cancel | approve | reject | process | complete (body nil = không gửi body).
func (c *Client) WarrantyAction(ctx context.Context, token, id, action string, body any) (*WarrantyClaim, error) {
	return one[WarrantyClaim](c, ctx, req{method: "POST", path: "/warranty/" + id + "/" + action, token: token,
		body: body, map404: true})
}

// ─── Chat ────────────────────────────────────────────────────────────────────

func (c *Client) ListConversations(ctx context.Context, token string) ([]Conversation, error) {
	return many[Conversation](c, ctx, req{method: "GET", path: "/chat/conversations", token: token})
}

func (c *Client) StartConversation(ctx context.Context, token, shopID string) (*Conversation, error) {
	return one[Conversation](c, ctx, req{method: "POST", path: "/chat/conversations", token: token,
		body: map[string]string{"shopId": shopID}})
}

func (c *Client) GetMessages(ctx context.Context, token, conversationID string) ([]ChatMessage, error) {
	return many[ChatMessage](c, ctx, req{method: "GET", path: "/chat/conversations/" + conversationID + "/messages", token: token})
}

func (c *Client) SendMessage(ctx context.Context, token, conversationID, content string) (*ChatMessage, error) {
	return one[ChatMessage](c, ctx, req{method: "POST", path: "/chat/conversations/" + conversationID + "/messages",
		token: token, body: map[string]string{"content": content}})
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// one: gọi API cần data kiểu T; data null → lỗi "no data" (như .ok_or_else của api.rs).
func one[T any](c *Client, ctx context.Context, r req) (*T, error) {
	var out *T
	if _, err := c.do(ctx, r, &out); err != nil {
		return nil, err
	}
	if out == nil {
		return nil, noData
	}
	return out, nil
}

// many: gọi API trả danh sách; data null → lỗi "no data".
func many[T any](c *Client, ctx context.Context, r req) ([]T, error) {
	var out []T
	if _, err := c.do(ctx, r, &out); err != nil {
		return nil, err
	}
	if out == nil {
		return nil, noData
	}
	return out, nil
}
