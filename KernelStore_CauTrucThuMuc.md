# KernelStore — Cấu trúc thư mục & File

> Đề tài: Sàn thương mại điện tử công nghệ đa nhà cung cấp
> Sinh viên: Nguyễn Huệ Thùy Linh · D24CNA12149 · Lớp D24CN03

---

## 📁 Cấu trúc tổng quan

```
KernelStore/
├── backend/                    ← Tầng Backend (Go 1.27: REST API + WebSocket, :5000)
│   ├── cmd/api/
│   │   ├── main.go             → Entry point: config, DB, migration, seed admin, router, :5000
│   │   ├── seeder.go           → Tạo roles Customer/Seller/Admin + tài khoản admin
│   │   └── seed_demo.go        → `go run ./cmd/api seed`: 10 danh mục, 7 shop, 57 sản phẩm
│   ├── internal/
│   │   ├── handlers/           → HTTP handler theo nhóm chức năng (Auth, Shops, Products, Orders, Chat...)
│   │   ├── repository/         → Truy vấn SQL viết tay (pgx), transaction
│   │   ├── services/           → JWT (TokenService), tạo user / kiểm mật khẩu
│   │   ├── identity/           → Băm mật khẩu PBKDF2, luật mật khẩu/username, security stamp
│   │   ├── middleware/         → Xác thực JWT, phân quyền, CORS, recovery, log
│   │   ├── httpx/              → Router có chính sách quyền, đọc JSON/query string + validation
│   │   ├── validate/           → Luật kiểm dữ liệu (bắt buộc, độ dài, khoảng, regex, email)
│   │   ├── dto/                → Request/Response JSON, wrapper {success,data,message,errors}
│   │   ├── models/             → Struct bảng DB + enum (trạng thái shop/đơn/bảo hành, vai trò)
│   │   ├── money/              → Kiểu tiền tệ chính xác (không sai số float)
│   │   ├── ws/                 → Hub WebSocket in-memory + endpoint /ws/chat
│   │   └── config/             → Cấu hình từ biến môi trường
│   ├── migrations/             → SQL migration (khớp database.sql), nhúng vào binary
│   ├── uploads/                → Ảnh sản phẩm (phục vụ tại /uploads/...)
│   └── go.mod / go.sum         → Module Go + thư viện
│
├── frontend/                   ← Tầng Frontend (Go SSR + templ + HTMX + Tailwind, :8080)
│   ├── cmd/web/main.go         → Entry point: server web :8080
│   ├── internal/
│   │   ├── web/                → Router trang + endpoint HTMX /x/*, phiên đăng nhập (cookie)
│   │   ├── views/              → Giao diện .templ (layout, component, 12 trang) + *_templ.go sinh ra
│   │   ├── client/             → Client gọi REST API backend (:5000)
│   │   └── i18n/               → Song ngữ EN/VI (398 key)
│   ├── static/
│   │   ├── css/input.css       → Theme terminal (nguồn) → css/app.css (Tailwind build)
│   │   ├── js/htmx.min.js      → HTMX 2
│   │   ├── js/app.js           → Toast, gallery, slug, chọn sao, upload ảnh, chat WebSocket
│   │   └── favicon.svg
│   ├── tailwind.config.js      → Cấu hình Tailwind CSS (màu, font, bo góc)
│   ├── .air.toml               → Hot reload (templ generate → tailwind → build)
│   └── go.mod / go.sum
│
├── test/wsclient/              ← Công cụ thử WebSocket (Go), test_chat_api.sh tự build
├── flake.nix                   ← Môi trường dev Nix (Go 1.27, templ, tailwind, air...) + package build
├── flake.lock                  ← Khoá phiên bản nixpkgs (sinh bằng `nix flake lock`)
├── .envrc                      ← direnv: `use flake`
├── docker-compose.yml          ← PostgreSQL 16 container (port 5433)
├── database.sql                ← Dump schema + dữ liệu mẫu
├── run.sh                      ← Chạy toàn bộ stack (1 lệnh)
├── seed.sh                     ← Seed dữ liệu mẫu
├── win-run-all.bat             ← Windows: DB + backend + frontend (tự mở trình duyệt)
├── win-db.bat                  ← Windows: chạy Docker DB
├── win-backend.bat             ← Windows: chạy backend
├── win-frontend.bat            ← Windows: chạy frontend
├── win-seed.bat                ← Windows: seed dữ liệu
├── test_phase1_api.sh          → Test Auth (21 checks)
├── test_phase2_api.sh          → Test Shop & Seller (14 checks)
├── test_phase3_api.sh          → Test Product & Category (27 checks)
├── test_phase4_api.sh          → Test Cart & Order (22 checks)
├── test_phase5_api.sh          → Test Review (15 checks)
├── test_phase6_api.sh          → Test Admin Panel (32 checks)
├── test_chat_api.sh            → Test Chat realtime REST + WS (27 checks)
├── test_full_api.sh            → Smoke test E2E toàn luồng (52 checks)
├── test_extra_api.sh           → Test nâng cao (upload, ban, return...) (66 checks)
├── test_warranty_api.sh        → Test bảo hành (23 checks)
├── README.md                   → Hướng dẫn chính (NixOS/Linux)
├── README-Windows.md           → Hướng dẫn chạy trên Windows
└── HUONG_DAN_CAI_DAT.md        → Cài đặt / phục hồi database
```

---

## 🔧 1. BACKEND (`backend/`)

### `internal/handlers/` — API Endpoints

| File | Chức năng |
|------|-----------|
| `auth.go` | Đăng ký, đăng nhập, refresh token (dùng một lần), `/me` |
| `shops.go` | Mở shop (Customer → Seller), xem/sửa shop của mình |
| `admin_shops.go` | Admin: duyệt / từ chối / ban tạm thời / gỡ ban / ban vĩnh viễn shop |
| `categories.go` | Cây danh mục chung (public) + CRUD (Admin) |
| `seller_categories.go` | Seller: danh mục riêng của shop |
| `products.go` | Danh sách (lọc/sắp xếp/phân trang), chi tiết, featured; Seller CRUD (shop phải Approved) |
| `uploads.go` | Upload ảnh jpg/png/svg ≤ 5MB |
| `cart.go` | Giỏ hàng (xem/thêm/đổi số lượng/xoá), không vượt tồn kho |
| `orders.go` | Đặt hàng (1 transaction), lịch sử, đổi trạng thái, huỷ/hoàn kho, trả hàng |
| `reviews.go` | Đánh giá: chỉ sau khi nhận hàng, mỗi người 1 lần |
| `warranty.go` | Bảo hành: gửi yêu cầu, duyệt / từ chối / xử lý / hoàn tất |
| `admin.go` | Dashboard admin, quản lý user (ban/unban), đơn toàn hệ thống, dashboard seller |
| `chat.go` | REST chat: hội thoại, tin nhắn, đẩy realtime qua WebSocket |
| `handlers.go` / `common.go` | Khai báo phụ thuộc + đăng ký route, hàm dùng chung |

### `internal/repository/` — Truy cập dữ liệu (SQL viết tay, pgx)

| File | Nội dung |
|------|----------|
| `db.go` | Pool kết nối, transaction (`InTx`), đọc timestamptz ra UTC |
| `migrate.go` | Chạy golang-migrate từ file SQL nhúng |
| `users.go` / `refresh_tokens.go` | Tài khoản, vai trò, refresh token |
| `shops.go` / `categories.go` / `products.go` | Shop, danh mục (cây, con cháu), sản phẩm + ảnh |
| `cart.go` / `orders.go` | Giỏ hàng; đơn hàng (trừ kho có điều kiện, hoàn kho, khoá dòng) |
| `reviews.go` / `warranty.go` / `chat.go` / `admin.go` | Đánh giá, bảo hành, chat, thống kê |

### Nhóm hạ tầng

| Thư mục | Chức năng |
|---------|-----------|
| `services/token.go` | Tạo/kiểm JWT HS256 (access 30 phút), sinh refresh token (7 ngày) |
| `services/users.go` | Tạo user theo đúng thứ tự kiểm (mật khẩu → username → email), kiểm mật khẩu |
| `identity/` | Băm mật khẩu PBKDF2-HMAC-SHA512 (100.000 vòng, tương thích tài khoản trong `database.sql`), luật mật khẩu, username |
| `middleware/` | `Authenticate` / `RequireAuth` (401) / `RequireRoles` (403), CORS, recovery, log |
| `httpx/` | Router (`{id}` sai GUID → 404), `BindJSON` (415/400 + validation), đọc query string |
| `ws/` | `Hub` giữ kết nối theo user (đa tab), endpoint `/ws/chat?access_token=JWT` |
| `dto/json.go` | Mã hoá JSON (escape Unicode giống bản cũ) |

### `migrations/` & dữ liệu

| File | Mô tả |
|------|-------|
| `000001_initial_schema.up.sql` | Tạo toàn bộ schema (21 bảng, giống hệt `database.sql`) |
| `000001_initial_schema.down.sql` | Xoá schema |
| `embed.go` | Nhúng file SQL vào binary |
| `../uploads/` | 57 ảnh sản phẩm mẫu + ảnh upload |

### Bảng dữ liệu chính

| Bảng | Mô tả |
|------|-------|
| `AspNetUsers` / `AspNetRoles` / `AspNetUserRoles` | Người dùng, vai trò |
| `Shops` | Gian hàng (Pending / Approved / Rejected / Banned / Deleted) |
| `Categories` | Danh mục cây cha-con; `OwnerShopId` = danh mục riêng của shop |
| `Products` / `ProductImages` | Sản phẩm + gallery |
| `CartItems` | Giỏ hàng |
| `Orders` / `OrderDetails` / `Addresses` | Đơn hàng, chi tiết, địa chỉ giao |
| `Reviews` | Đánh giá |
| `WarrantyClaims` | Yêu cầu bảo hành |
| `Conversations` / `ChatMessages` | Chat (duy nhất theo cặp khách + shop) |
| `RefreshTokens` | Refresh token (IsUsed, IsRevoked) |

---

## 🎨 2. FRONTEND (`frontend/`)

### `internal/web/` — Router & xử lý

| File | Chức năng |
|------|-----------|
| `app.go` | Server, phiên đăng nhập (cookie HttpOnly `ks_access`/`ks_refresh`), chặn trang cần đăng nhập/role, toast |
| `routes.go` / `actions.go` | Danh sách trang + endpoint hành động HTMX `/x/*` |
| `auth.go` | Đăng nhập, đăng ký, đăng xuất, đổi ngôn ngữ, trang chủ, 404 |
| `products.go` / `cart.go` / `orders.go` | Sản phẩm, giỏ + thanh toán, đơn hàng + đánh giá + bảo hành |
| `warranty.go` / `seller.go` / `admin.go` / `chat.go` | Bảo hành, trang seller, trang admin, chat |

### `internal/views/` — Trang & component (templ)

| File | Route | Vai trò | Chức năng |
|------|-------|---------|-----------|
| `layout.templ` | — | — | Khung trang, header, menu theo vai trò, toast, Loading, KernelPanic, TermInput |
| `home.templ` | `/` | Đăng nhập | Hero, chuyên ngành IT, sản phẩm nổi bật |
| `products.templ` | `/products` | Public | Lọc, gợi ý tìm kiếm, sắp xếp, cây danh mục, phân trang |
| `product_detail.templ` | `/products/{slug}` | Public | Gallery, giá, tồn kho, bảo hành, review, thêm giỏ, chat với shop |
| `auth.templ` | `/auth/login`, `/auth/register` | Public | Đăng nhập / đăng ký kiểu terminal |
| `cart.templ` | `/cart`, `/checkout` | Đăng nhập | Bảng giỏ hàng, form địa chỉ, xác nhận đặt hàng |
| `orders.templ` | `/orders`, `/orders/{id}` | Đăng nhập | Lịch sử, chi tiết, xác nhận/huỷ/trả hàng, đánh giá, bảo hành, đổi trạng thái |
| `warranty.templ` | `/warranty`, `/warranty/manage` | Đăng nhập / S-A | Yêu cầu bảo hành của khách; xử lý của shop/admin |
| `seller.templ` | `/seller?tab=` | Đăng nhập | Mở shop; dashboard / đơn bán / sản phẩm / danh mục / cài đặt |
| `admin.templ` | `/admin?tab=` | Admin | Dashboard, duyệt shop, quản lý danh mục |
| `chat.templ` | `/chat` | Đăng nhập | Danh sách hội thoại + khung chat realtime |
| `format.go` / `session.go` | — | — | Định dạng tiền/ngày/sao/thanh ASCII, menu theo vai trò |

### Khác

| File | Chức năng |
|------|-----------|
| `internal/client/` | Gọi REST API backend (`client.go`, `endpoints.go`, `types.go`) |
| `internal/i18n/` | `table.go` (398 key EN/VI), `i18n.go` (đọc cookie `ks_lang`) |
| `static/js/app.js` | Toast `[INFO]/[WARN]/[ERROR]/[OK]`, ẩn/hiện mật khẩu, gallery, slug tự sinh, chọn sao, upload ảnh, chat WebSocket |
| `tailwind.config.js` / `static/css/input.css` | Theme terminal (glass, neon, backdrop, typewriter) |
| `.air.toml` | Hot reload khi phát triển |

---

## 🧪 3. TEST

### Shell scripts (curl + jq)

| File | Phạm vi | Checks |
|------|---------|--------|
| `test_phase1_api.sh` | Auth: register/login/refresh/RBAC | 21 |
| `test_phase2_api.sh` | Shop & Seller: mở shop/duyệt/phân quyền | 14 |
| `test_phase3_api.sh` | Product & Category: CRUD, filter, pagination | 27 |
| `test_phase4_api.sh` | Cart & Order: add/update, stock, checkout | 22 |
| `test_phase5_api.sh` | Review: chỉ khi Delivered, chống trùng | 15 |
| `test_phase6_api.sh` | Admin: dashboard, ban/unban, category CRUD | 32 |
| `test_chat_api.sh` | Chat realtime: REST + WebSocket 2 chiều | 27 |
| `test_full_api.sh` | Smoke test E2E toàn bộ luồng nghiệp vụ | 52 |
| `test_extra_api.sh` | Upload ảnh, ban tạm/vĩnh viễn, return flow | 66 |
| `test_warranty_api.sh` | Bảo hành: điều kiện gửi, vòng đời xử lý | 23 |

Kết quả gần nhất (2026-09-29): **276/276** trên 9 bộ đầu + **23/23** bảo hành.

### Unit test Go

`cd backend && go test ./...` — băm mật khẩu (so với hash thật), enum, tiền tệ, validation, router/phân quyền, JSON, hạn bảo hành.

### WebSocket probe

| Thư mục | Chức năng |
|---------|-----------|
| `test/wsclient/` | Chương trình Go nhỏ, `test_chat_api.sh` tự build để thử WebSocket |

---

## 🐳 4. DEVOPS & CẤU HÌNH (Root level)

| File | Chức năng |
|------|-----------|
| `flake.nix` | Môi trường dev (`nix develop`): Go 1.27.1, gopls, delve, golangci-lint, templ, tailwindcss, air, go-migrate, docker-compose, psql 16, jq, curl; package `nix build` (backend) / `nix build .#frontend` |
| `flake.lock` | Khoá phiên bản nixpkgs — cập nhật bằng `nix flake update` |
| `docker-compose.yml` | Chạy PostgreSQL 16 container, map port `5433:5432` |
| `run.sh` | **Một lệnh chạy cả stack**: Docker → Backend → Frontend (`nix develop -c ./run.sh`) |
| `seed.sh` | Seed dữ liệu mẫu: 10 categories, 7 shops, 57 products — idempotent |
| `win-run-all.bat` | Windows: DB + backend + frontend + mở trình duyệt |
| `win-db.bat` / `win-backend.bat` / `win-frontend.bat` / `win-seed.bat` | Windows: từng service |
| `README.md` / `README-Windows.md` / `HUONG_DAN_CAI_DAT.md` | Hướng dẫn NixOS/Linux, Windows, cài database |

---

## 🔄 Sơ đồ tương tác tổng thể

```
┌──────────────────────────────────────────────────────────────┐
│  BROWSER (User)                                              │
│  HTML + HTMX + app.js ──────────────┐                        │
└────────┬───────────────────────────┼─────────────────────────┘
         │ HTTP (trang, /x/* HTMX)    │ WebSocket /ws/chat?access_token=
         ▼                            │ (nhận tin chat realtime)
┌─────────────────────────────┐       │
│  FRONTEND  localhost:8080   │       │
│  Go + templ (render HTML)   │       │
│  cookie phiên, i18n EN/VI   │       │
└────────┬────────────────────┘       │
         │ HTTP REST (JSON, Bearer)   │
         ▼                            ▼
┌──────────────────────────────────────────────────────────────┐
│  BACKEND  localhost:5000  (Go 1.27)                          │
│  middleware (JWT, role, CORS) → handlers → repository (pgx)  │
│  services (Token, User)        ws.Hub (in-memory)            │
└────────────────────────────────┬─────────────────────────────┘
                                 │ SQL
                                 ▼
                     ┌───────────────────────┐
                     │ PostgreSQL 16         │
                     │ (Docker :5433)        │
                     │ Users/Shops/Products  │
                     │ Orders/Reviews/Chat   │
                     └───────────────────────┘
```

---

## 💡 Tóm tắt ý nghĩa từng tầng

| Tầng | Thư mục/File chính | Vai trò |
|------|-------------------|---------|
| **Presentation** | `frontend/internal/views/` + `static/` | Giao diện người dùng (HTML render ở server, HTMX cập nhật từng phần) |
| **Web / BFF** | `frontend/internal/web/` + `client/` | Định tuyến trang, phiên đăng nhập, gọi API backend |
| **API** | `backend/internal/handlers/` + `httpx/` + `middleware/` | Tiếp nhận request, phân quyền JWT, validation |
| **Business Logic** | `backend/internal/handlers/` + `services/` + `identity/` | Nghiệp vụ, bảo mật, transaction |
| **Data Access** | `backend/internal/repository/` + `models/` + `migrations/` | Truy vấn SQL, schema |
| **Database** | PostgreSQL 16 (Docker) | Lưu trữ dữ liệu |
| **DevOps** | `flake.nix`, `docker-compose.yml`, `*.sh`, `*.bat` | Môi trường & chạy dự án |
| **Testing** | `test_*.sh`, `test/wsclient/`, `go test` | Kiểm thử tự động API + WebSocket + unit |

---

*Nguyễn Huệ Thùy Linh · D24CNA12149 · Lớp D24CN03*
