# Bảng ánh xạ C# / Rust → Go (bước 0)

> Branch: `go-migration`. Tài liệu này dùng để duyệt trước khi viết code; các giai đoạn sau bám theo nó.
> Code mới nằm ở `backend-go/` và `frontend-go/`; code C#/Rust giữ nguyên tới giai đoạn 11.
> **Cập nhật sau giai đoạn 11:** `backend-go/` → `backend/`, `frontend-go/` → `frontend/`; code C#/Rust đã xoá. Bảng dưới giữ nguyên làm tài liệu đối chiếu.

---

## A. Backend

### A1. Cấu trúc & hạ tầng

| C# (ASP.NET Core 10) | Go | Ghi chú |
|---|---|---|
| `Program.cs` | `cmd/api/main.go` | Đã có. `go run ./cmd/api seed` = `dotnet run seed`. |
| `appsettings.json` | `internal/config/config.go` | Đã có; đọc env, mặc định khớp appsettings. |
| `Data/ApplicationDbContext.cs` + `Migrations/*` | `migrations/000001_initial_schema.{up,down}.sql` + `internal/repository/*` | Schema đã so `pg_dump` = `database.sql` (100%). |
| EF Core LINQ | `pgx/v5` + SQL viết tay trong `internal/repository/<domain>.go` | Xem **Quyết định Q2** (sqlc hay không). |
| `Common/ApiResponse.cs` | `internal/dto/response.go` | Đã có. |
| `InvalidModelStateResponseFactory` + DataAnnotations (`[Required]`, `[StringLength]`, `[Range]`, `[RegularExpression]`, `[EmailAddress]`) | `internal/validate/validate.go` (hàm `Validate()` trên từng request struct) | Trả 400 `"Dữ liệu không hợp lệ"` + `errors: ["Field: msg"]`. Body rỗng/JSON hỏng/Guid sai → 400. |
| `AddJwtBearer` | `middleware.JWTAuth` | Đã có. 401 khi thiếu/sai token. |
| `[Authorize]`, `[Authorize(Roles="A,B")]`, `[AllowAnonymous]` | `middleware.RequireAuth`, `middleware.RequireRole("A","B")` bọc từng route | Role lấy **từ token** như C# (token cũ vẫn mang role cũ cho tới khi refresh). 401 trước, 403 sau. |
| `AddCors("FrontendPolicy")` | `middleware.CORS` | Đã sửa để giống AllowAny*. |
| `UseStaticFiles` + nosniff/CSP | `http.FileServer` + `addSecurityHeaders` | Đã có. |
| Route constraint `{id:guid}` | ServeMux `{id}` + `uuid.Parse`; sai định dạng → **404** | Giống ASP.NET (constraint không khớp = không có route). |
| `UserManager` / `RoleManager` (Identity) | `internal/identity/` | Chuẩn hoá `NormalizedEmail/UserName` (UPPER), chính sách mật khẩu (≥6, có số, thường, HOA, ký tự đặc biệt), ký tự username hợp lệ (`a-zA-Z0-9-._@+`), username/email không trùng, `SecurityStamp`, gán role. Xem **Q1** (hash mật khẩu). |
| `Services/TokenService.cs` | `internal/services/token.go` | Đã có, claims khớp. |
| `Services/ChatConnectionManager.cs` | `internal/ws/hub.go` | `map[userID]map[connID]*websocket.Conn` + `sync.RWMutex`. |
| `Common/ChatEndpoints.cs` | `internal/ws/endpoint.go` | `/ws/chat?access_token=`: không phải WS → 400, token sai → 401. Thư viện `coder/websocket`. |
| `Common/DatabaseSeeder.SeedAsync` | `cmd/api/seeder.go` | Đã có (roles + admin). |
| `Common/DatabaseSeeder.SeedDemoDataAsync` | `cmd/api/seed_demo.go` | 10 categories, 7 seller/shop, 57 sản phẩm, ảnh `http://localhost:5000/uploads/<slug>.<ext>`; idempotent theo marker `github-copilot-1-year`. |
| `test/wsclient` (C#) | `test/wsclient/main.go` | Cùng CLI: `wsclient <token> <seconds>`, in `[ws] connected` / `[ws] <json>` / `[ws] done`. |

### A2. Entities → `internal/models`

Đã có đủ trong `models.go`: `ApplicationUser, Shop, Category, Product, ProductImage, Address, Order, OrderDetail, Review, CartItem, RefreshToken, Conversation, ChatMessage, WarrantyClaim` + enum `UserRole, ShopStatus, OrderStatus, WarrantyStatus, WarrantyResolution` (lưu `integer`, JSON trả **tên** enum như `.ToString()`).

### A3. Contracts → `internal/dto/contracts.go`

| C# | Go | Trạng thái |
|---|---|---|
| Auth, Shops, Categories, Products, Cart, Orders, Reviews, Chat, Admin, Seller | cùng tên | Đã có |
| `Warranty/*` (`CreateWarrantyClaimRequest`, `ApproveWarrantyRequest`, `WarrantyNoteRequest`, `WarrantyClaimDto`) | cùng tên | **Thiếu — sẽ thêm** |
| `decimal` (Price, SalePrice, TotalAmount, Revenue…) | hiện là `float64` | **Sẽ đổi sang `decimal.Decimal`** (`MarshalJSONWithoutQuotes = true`) để tiền không sai số khi nhân/cộng. |
| `List<T>` rỗng → `[]` | slice phải khởi tạo `[]T{}` | Go `nil` slice ra `null` → phải chú ý. |
| `DateTime` UTC | `time.Time` `.UTC()` | |
| `Guid?`/`string?`/`decimal?` | con trỏ `*T` (ra `null`) | |

### A4. Controllers → `internal/handlers` (endpoint giữ nguyên 100%)

Ký hiệu quyền: 🌐 ẩn danh · 🔑 cần đăng nhập · 👤S Seller · 👤A Admin · 👤S/A Seller hoặc Admin.

| C# Controller | Go file | Endpoint | Quyền | Nghiệp vụ cần giữ |
|---|---|---|---|---|
| `AuthController` | `auth.go` | `POST /api/auth/register` | 🌐 | Email trùng → 400; lỗi Identity → 400 kèm mô tả; role Customer; trả `AuthResponse` |
| | | `POST /api/auth/login` | 🌐 | Sai email/mật khẩu → 401; `IsActive=false` → 401 |
| | | `POST /api/auth/refresh` | 🌐 | Token không có/revoked/used → 401; hết hạn → 401; user bị ban → 401; đánh dấu `IsUsed` rồi cấp cặp mới (single-use rotation) |
| | | `GET /api/auth/me` | 🔑 | `data: { info, roles }` |
| `ShopsController` | `shops.go` | `POST /api/shops` | 🔑 | 1 user 1 shop; slug trùng → 400; tạo Pending; **Customer → Seller** (cả cột `Role` lẫn AspNetUserRoles) |
| | | `GET /api/shops/me` | 🔑 | Chưa có → 200, `data: null`, msg "Chưa có shop" |
| | | `PUT /api/shops/me` | 🔑 | |
| `AdminShopsController` | `admin_shops.go` | `GET /api/admin/shops?status=` | 👤A | |
| | | `POST /api/admin/shops/{id}/approve` | 👤A | Đã Approved → 400; owner thành Seller |
| | | `POST /api/admin/shops/{id}/reject` | 👤A | Chỉ Pending |
| | | `POST /api/admin/shops/{id}/ban` | 👤A | → Banned, ẩn toàn bộ sản phẩm |
| | | `POST /api/admin/shops/{id}/unban` | 👤A | Banned → Approved, hiện lại sản phẩm |
| | | `DELETE /api/admin/shops/{id}` | 👤A | Không có đơn → xoá cứng shop + sản phẩm; có đơn → `Deleted` + ẩn sản phẩm |
| `AdminUsersController` | `admin_users.go` | `GET /api/admin/users?search=&role=&isActive=` | 👤A | |
| | | `POST /api/admin/users/{id}/ban` | 👤A | Không tự ban mình, không ban Admin; revoke mọi refresh token |
| | | `POST /api/admin/users/{id}/unban` | 👤A | |
| `AdminDashboardController` | `admin_dashboard.go` | `GET /api/admin/dashboard` | 👤A | Doanh thu bỏ Cancelled/Returned; `ordersByStatus` đủ 8 trạng thái theo thứ tự enum |
| `AdminOrdersController` | `admin_orders.go` | `GET /api/admin/orders?status=&search=&page=&pageSize=` | 👤A | pageSize kẹp 1..50 |
| `SellerDashboardController` | `seller_dashboard.go` | `GET /api/seller/dashboard` | 👤S/A | Top 5 theo doanh thu |
| `CategoriesController` | `categories.go` | `GET /api/categories` | 🌐 | Cây, chỉ category global (`OwnerShopId IS NULL`), sắp theo Name |
| | | `GET /api/categories/{slug}` | 🌐 | |
| | | `POST /api/categories`, `PUT /{id}`, `DELETE /{id}` | 👤A | Xoá bị chặn nếu có con/sản phẩm |
| `SellerCategoriesController` | `seller_categories.go` | `GET/POST /api/seller/categories`, `PUT/DELETE /api/seller/categories/{id}` | 👤S | Category phẳng của shop; xoá → gỡ `CategoryId` khỏi sản phẩm |
| `ProductsController` | `products.go` | `GET /api/products?category=&shop=&minPrice=&maxPrice=&search=&sort=&page=&pageSize=` | 🌐 | Category gồm cả con (BFS); giá lọc theo `COALESCE(SalePrice, Price)` |
| | | `GET /api/products/featured?take=` | 🌐 | |
| | | `GET /api/products/{slug}` | 🌐 | Nhận slug **hoặc** id; kèm reviews, shop summary, điểm TB làm tròn 2 số |
| | | `GET /api/products/my` | 🔑 | (C# chỉ cần đăng nhập) |
| | | `POST /api/products` | 👤S | **Guard shop Approved**; tối đa 10 ảnh, ảnh đầu là primary |
| | | `PUT /api/products/{id}`, `DELETE /api/products/{id}` | 👤S | Guard Approved + sở hữu |
| `UploadsController` | `uploads.go` | `POST /api/uploads/image` (multipart `file`) | 🔑 | ≤5MB; `.jpg/.jpeg/.png/.svg`; tên `<uuid-N><ext>`; URL `scheme://host/uploads/...` |
| `CartController` | `cart.go` | `GET/POST /api/cart`, `PUT/DELETE /api/cart/{productId}` | 🔑 | Không vượt tồn kho; qty ≤0 khi PUT = xoá |
| `OrdersController` | `orders.go` | `POST /api/orders` | 🔑 | **1 transaction**: tạo Address + Order + Details, trừ stock, xoá giỏ; giá chốt `SalePrice ?? Price`; `KS-yyyyMMdd-XXXX` unique |
| | | `GET /api/orders` | 🔑 | Admin: tất cả; Seller: đơn mình mua + đơn bán (chỉ phần hàng shop) |
| | | `GET /api/orders/sales?status=` | 👤S/A | |
| | | `GET /api/orders/{id}` | 🔑 | Buyer/Admin xem đủ; seller xem phần của shop; khác → 403 |
| | | `PUT /api/orders/{id}/status` | 👤S/A | Chặn đổi từ Delivered/Cancelled/ReturnRequested/Returned; Cancelled → hoàn kho |
| | | `POST /api/orders/{id}/confirm-received` | 🔑 | Shipped → Delivered, set `PaidAt` |
| | | `POST /api/orders/{id}/cancel` | 🔑 | Pending/Confirmed/Processing → Cancelled + hoàn kho |
| | | `POST /api/orders/{id}/return` | 🔑 | Delivered → ReturnRequested |
| | | `POST /api/orders/{id}/return/{decision}` | 👤S/A | approve → Returned + hoàn kho; reject → Delivered |
| `ReviewsController` | `reviews.go` | `GET /api/reviews?productId=` | 🌐 | Thiếu productId → 400 |
| | | `POST /api/reviews` | 🔑 | Chưa có đơn Delivered → 403; đánh giá trùng → 400 |
| `WarrantyController` | `warranty.go` | `POST /api/warranty` | 🔑 | Delivered + còn hạn (`PaidAt ?? CreatedAt` + N tháng); không trùng yêu cầu đang mở; mã `WR-yyyyMMdd-XXXX` |
| | | `GET /api/warranty/mine` | 🔑 | |
| | | `GET /api/warranty/shop?status=` | 👤S/A | |
| | | `GET /api/warranty/{id}` | 🔑 | |
| | | `POST /api/warranty/{id}/cancel` | 🔑 | Chỉ Pending |
| | | `POST /api/warranty/{id}/approve` (body `resolution`, `note`) | 👤S/A | Pending → Approved |
| | | `POST /api/warranty/{id}/reject`, `/process`, `/complete` | 👤S/A | |
| `ChatController` | `chat.go` | `GET/POST /api/chat/conversations` | 🔑 | Không chat với shop của mình; 1 hội thoại / cặp buyer-shop |
| | | `GET /api/chat/conversations/{id}/messages` | 🔑 | Đánh dấu đã đọc tin của phía kia |
| | | `POST /api/chat/conversations/{id}/messages` | 🔑 | Lưu DB + đẩy WS tới người nhận (JSON camelCase `ChatMessageDto`) |
| `ChatEndpoints` | `internal/ws/endpoint.go` | `GET /ws/chat?access_token=JWT` | token | |

### A5. Hành vi "ngầm" của ASP.NET cần mô phỏng

- Model binding: JSON không phân biệt hoa thường ở tên field. Thiếu field Guid bắt buộc → 400.
- `User.IsInRole("X")` đọc role từ token, không đọc từ DB.
- Exception chưa bắt → 500. Ví dụ C# xoá sản phẩm đã có trong đơn → FK Restrict → 500. **Giữ nguyên** (không tự thêm chặn).
- Route/method không khớp → 404/405 (ServeMux làm được giống).

---

## B. Frontend (Rust + Leptos CSR → Go SSR + templ + HTMX + Tailwind)

### B1. Hạ tầng

| Rust | Go (`frontend-go/`) | Ghi chú |
|---|---|---|
| `main.rs`, `lib.rs::App` (Router) | `cmd/web/main.go` (ServeMux, `:8080`) | Cùng tập URL. |
| `index.html` (font, backdrop `.app-bg`) | `internal/components/layout.templ` | |
| `Trunk.toml` + tailwind của trunk | `tailwindcss` CLI (`static/css/app.css`) + `air` hot reload + `templ generate` | |
| `src/input.css`, `tailwind.config.js` | `static/css/input.css`, `tailwind.config.js` (content → `**/*.templ`) | Giữ nguyên theme terminal. |
| `api.rs` (gloo-net, gọi thẳng :5000 từ trình duyệt) | `internal/client/*.go` (server Go gọi :5000) | Kiểu dữ liệu tái dùng cấu trúc JSON. |
| `auth.rs` (JWT trong localStorage) | `internal/session/` — cookie **HttpOnly** `ks_access`, `ks_refresh` | Xem **Q3**. |
| `ProtectedRoute` | middleware `requireLogin` → 302 `/auth/login`; `requireRole` cho `/admin`, `/warranty/manage` | `/` hiện **đang là route protected** → giữ nguyên. |
| `i18n.rs` (EN/VI, ~800 key, lưu localStorage `ks_lang`) | `internal/i18n/` (`map[key]{en,vi}`, cookie `ks_lang`, `POST /lang`) | **Prompt không nhắc** nhưng là tính năng hiện có → giữ. |
| `components/toast.rs` | `components/toast.templ` + HTMX OOB swap vào `#toasts` + tự ẩn 4s (JS nhỏ) | `[INFO]/[WARN]/[ERROR]/[OK]`. |
| `components/loading.rs` (typewriter) | `components/loading.templ` + `hx-indicator` | CSS `.term-typing` giữ nguyên. |
| `components/error.rs` (KernelPanic) | `components/panic.templ` | Dùng cho 404 route và lỗi 500/mạng. |
| `components/input.rs` (TermInput, nút ẩn/hiện mật khẩu) | `components/input.templ` + JS toggle 3 dòng | |
| `components/nav.rs` (menu theo role, highlight tab) | `components/nav.templ` | |
| `web_sys::confirm` (xoá shop) | `hx-confirm` | |

### B2. Pages

| Rust page (component con) | URL | Go templ + handler | Tương tác HTMX |
|---|---|---|---|
| `home.rs` (FeaturedCard, DomainCard, skeleton) | `/` 🔑 | `pages/home.templ` | Featured load bằng `hx-get` + skeleton |
| `products.rs` (Grid card, CategoryItem, Pagination, skeleton) | `/products` | `pages/products.templ` | Lọc/sort/phân trang = `hx-get` + `hx-push-url` |
| `product_detail.rs` (ProductView, Reviews, bar/progress `[####  ]`, stars) | `/products/{slug}` | `pages/product_detail.templ` | Thêm vào giỏ, chat với shop |
| `login.rs`, `register.rs` | `/auth/login`, `/auth/register` | `pages/auth.templ` | Form POST → set cookie → redirect |
| `cart.rs` (CartRow) | `/cart` 🔑 | `pages/cart.templ` | Sửa số lượng/xoá: swap 1 dòng + tổng |
| `checkout.rs` (OrderPlaced) | `/checkout` 🔑 | `pages/checkout.templ` | |
| `orders.rs` (OrderRow, OrderDetailView, StatusManager, ConfirmReceipt, CancelOrder, ReturnRequest, ReturnResolver, ReviewForm, WarrantyForm) | `/orders`, `/orders/{id}` 🔑 | `pages/orders.templ`, `pages/order_detail.templ` | Mỗi action là 1 partial |
| `warranty.rs` (ClaimCard, ManageActions) | `/warranty` 🔑, `/warranty/manage` S/A | `pages/warranty.templ` | **Prompt không liệt kê** → thêm vào giai đoạn 10 |
| `seller.rs` (Sidebar, CreateShopForm, ShopStatus, RevenueDashboard, ProductManager/Form/Row + upload ảnh, SellerCategoryManager, SalesManager/Row, ShopSettings) | `/seller?tab=dashboard\|products\|categories\|sales\|settings` 🔑 | `pages/seller/*.templ` | Upload ảnh: form multipart → frontend → `/api/uploads/image` |
| `admin.rs` (AdminDashboard, StatCard, MeterRow, ShopModeration, AdminShopRow, CategoryManagement) | `/admin` A | `pages/admin/*.templ` | |
| `chat.rs` (WebSocket) | `/chat` 🔑 | `pages/chat.templ` + `static/js/chat.js` | Gửi qua HTMX POST; nhận realtime bằng WS trực tiếp tới `:5000` (xem Q3) |
| fallback | mọi URL khác | `KernelPanic 404` | |

---

## C. Số liệu test

Các bộ test chỉ gọi API ở `:5000`, không bộ nào gọi frontend `:8080`.
`test_warranty_api.sh` có trong repo nhưng **không nằm trong danh sách 9 bộ** của prompt → đề xuất chạy thêm sau giai đoạn Reviews (Q4).

## D. Thứ tự giai đoạn đề xuất (điều chỉnh nhỏ so với prompt)

1 Nền tảng (+ `validate`, `identity`, sửa decimal) → 2 Auth → 3 Shops → 4 Categories/Products/Upload → 5 Cart/Orders → 6 Reviews → **6b Warranty** → 7 Admin → 8 Chat + wsclient → 9 Seed demo → 10 Frontend (+ i18n, warranty) → 11 Dọn dẹp → 12 Nix flake → 13 Tài liệu.

## E. Quyết định đã chốt (29/09/2026)

- **Q1 — Hash mật khẩu:** PBKDF2 theo định dạng ASP.NET Identity v3, dùng `crypto/pbkdf2` của stdlib. Tài khoản cũ vẫn đăng nhập được, C# và Go dùng chung DB được. Seeder admin sẽ chuyển từ bcrypt sang cách này.
- **Q2 — Truy cập DB:** `pgx/v5` với SQL viết tay, không dùng sqlc.
- **Q3 — Phiên đăng nhập frontend:** cookie HttpOnly `ks_access`/`ks_refresh`. Trang `/chat` nhúng access token để JS mở WebSocket tới `:5000`.
- **Q4 — Warranty:** chuyển đầy đủ cả backend (giai đoạn 6b) lẫn frontend (giai đoạn 10), và chạy thêm `test_warranty_api.sh`.
