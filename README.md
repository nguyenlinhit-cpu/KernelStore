# KernelStore

Sàn thương mại điện tử đa nhà cung cấp (Multi-vendor E-commerce Marketplace) cho đồ công nghệ.

## Tech Stack

- **Backend:** Go 1.27 + `net/http` (ServeMux có method + wildcard) + pgx v5 + PostgreSQL 16 — `http://localhost:5000`
  - JWT HS256 (`golang-jwt`), mật khẩu PBKDF2-HMAC-SHA512, migration bằng `golang-migrate` (nhúng trong binary), WebSocket bằng `coder/websocket`, log bằng `log/slog`
- **Frontend:** Go (SSR) + [templ](https://templ.guide) + [HTMX](https://htmx.org) + Tailwind CSS — `http://localhost:8080`
  - Giao diện terminal: loading kiểu máy đánh chữ, KernelPanic 404/500, toast `[INFO]/[WARN]/[ERROR]/[OK]`, bảng & biểu đồ ASCII `[####  ]`, song ngữ EN/VI
- **Database:** PostgreSQL 16 (Docker Compose) — host `localhost:5433` → container `5432`
- **Môi trường dev:** Nix flake (`flake.nix` + `flake.lock`) — `nix develop`

## Cấu trúc

```
KernelStore/
├── backend/                → REST API + WebSocket (Go 1.27)
│   ├── cmd/api/            → main.go (khởi động :5000), seeder.go (roles + admin), seed_demo.go (dữ liệu mẫu)
│   ├── internal/
│   │   ├── handlers/       → Auth, Shops, AdminShops, Categories, SellerCategories, Products, Uploads,
│   │   │                     Cart, Orders, Reviews, Warranty, Admin (dashboard/users/orders), Seller dashboard, Chat
│   │   ├── repository/     → truy vấn SQL viết tay (pgx), transaction
│   │   ├── services/       → TokenService (JWT), tạo user / kiểm mật khẩu
│   │   ├── identity/       → băm mật khẩu PBKDF2, luật mật khẩu/username
│   │   ├── middleware/     → xác thực JWT, phân quyền, CORS, recovery, log
│   │   ├── httpx/          → router có chính sách quyền, đọc JSON/query + validation
│   │   ├── dto/ models/    → request/response JSON, bảng DB + enum
│   │   ├── ws/             → Hub WebSocket in-memory + endpoint /ws/chat
│   │   └── config/ money/ validate/
│   ├── migrations/         → 000001_initial_schema.{up,down}.sql (khớp database.sql), nhúng vào binary
│   └── uploads/            → ảnh sản phẩm (phục vụ tại /uploads/...)
├── frontend/               → web SSR (Go + templ + HTMX)
│   ├── cmd/web/            → main.go (khởi động :8080)
│   ├── internal/
│   │   ├── web/            → router trang + endpoint HTMX /x/*, phiên đăng nhập (cookie HttpOnly)
│   │   ├── views/          → giao diện .templ (layout, components, 12 trang)
│   │   ├── client/         → client gọi REST API backend
│   │   └── i18n/           → bảng dịch EN/VI (398 key)
│   ├── static/             → css (input.css → app.css), js (htmx.min.js, app.js), favicon
│   ├── tailwind.config.js  → cấu hình Tailwind (theme terminal)
│   └── .air.toml           → hot reload
├── test/wsclient/          → công cụ thử WebSocket (Go) cho test_chat_api.sh
├── flake.nix / flake.lock  → môi trường dev + build package (Nix)
├── docker-compose.yml      → PostgreSQL 16
├── database.sql            → dump schema + dữ liệu mẫu
├── run.sh                  → NixOS/Linux: DB + backend + frontend (1 lệnh)
├── seed.sh                 → seed dữ liệu mẫu (1 lệnh)
├── win-*.bat               → Windows: chạy từng service / cả stack
├── test_*_api.sh           → bộ test API (curl + jq)
├── README.md               → hướng dẫn chính (NixOS/Linux)
└── README-Windows.md       → hướng dẫn trên Windows
```

Chi tiết từng file: [KernelStore_CauTrucThuMuc.md](./KernelStore_CauTrucThuMuc.md).

## Chạy nhanh (TL;DR)

Cần **3 thành phần chạy cùng lúc**, đúng thứ tự: **Database → Backend → Frontend**.

```sh
# Docker — không cần cài Go/Nix, chỉ cần Docker. Build + chạy cả stack:
docker compose up -d --build
docker compose run --rm backend seed        # seed dữ liệu mẫu (tuỳ chọn)
# Cổng 8080 bị chiếm? Đổi cổng host: WEB_PORT=8081 docker compose up -d --build
# (API_PORT đổi được, nhưng ảnh của dữ liệu mẫu trỏ cố định tới localhost:5000)
# Tắt: docker compose down   (thêm -v để xoá cả DB và ảnh upload)
```

```sh
# NixOS/Linux — dựng cả stack, Ctrl+C để tắt
nix develop -c ./run.sh
```

```bat
:: Windows — double-click win-run-all.bat
win-run-all.bat
```

**Hoặc chạy tay từng terminal** (mỗi terminal đã `nix develop`):

```sh
docker compose up -d postgres         # Terminal 1 — Postgres 16 (:5433)
cd backend && go run ./cmd/api        # Terminal 2 — Backend  → http://localhost:5000
cd frontend && air                    # Terminal 3 — Frontend → http://localhost:8080 (tự reload)
```

Mở trình duyệt tại **`http://localhost:8080`**. Seed dữ liệu mẫu (tuỳ chọn): `./seed.sh`.

---

## Chạy trên NixOS (chi tiết)

Toàn bộ công cụ (Go 1.27.1, gopls, delve, golangci-lint, templ, tailwindcss, air, go-migrate, docker-compose, psql 16, jq, curl) khai báo trong `flake.nix` — **không cài gì global**.

### 0. Yêu cầu hệ thống (một lần, trong `configuration.nix`)

```nix
# /etc/nixos/configuration.nix
nix.settings.experimental-features = [ "nix-command" "flakes" ];   # bật flakes
virtualisation.docker.enable = true;                               # Docker daemon cho Postgres
users.users.<bạn>.extraGroups = [ "docker" ];
```

Rồi `sudo nixos-rebuild switch` và đăng nhập lại. Không dùng Docker? Xem [Postgres không dùng Docker](#postgres-không-dùng-docker).

### 1. Vào môi trường dev

```sh
cd KernelStore
nix develop        # lần đầu tải nixpkgs hơi lâu; in ra phiên bản go / templ / tailwindcss / air
```

**Mọi lệnh bên dưới chạy trong `nix develop`** (hoặc `nix develop -c <lệnh>`). Có [direnv](https://direnv.net)? `direnv allow` một lần là tự vào môi trường mỗi khi `cd` vào thư mục (file `.envrc` = `use flake`).

> Flake chỉ thấy file **đã được git track**. Thêm file mới mà `nix develop`/`nix build` báo không tìm thấy → `git add <file>` trước.

### 2. Database (PostgreSQL 16)

```sh
docker compose up -d postgres # postgres:16 tại localhost:5433 (db/user/pass: kernelstore/admin/admin123)
docker compose ps             # đợi healthy
```

Nạp sẵn dữ liệu mẫu (tuỳ chọn, thay cho bước 4): xem [HUONG_DAN_CAI_DAT.md](./HUONG_DAN_CAI_DAT.md).

### 3. Backend (Go API)

```sh
cd backend
go run ./cmd/api              # → Now listening on: http://localhost:5000
```

- Khi khởi động: chạy migration (SQL nhúng trong binary) → tạo roles `Customer/Seller/Admin` + tài khoản admin.
- Cấu hình qua biến môi trường (giá trị mặc định khớp dự án):

| Biến | Mặc định | Ý nghĩa |
|---|---|---|
| `SERVER_ADDR` | `:5000` | địa chỉ lắng nghe |
| `DB_HOST` / `DB_PORT` / `DB_NAME` | `localhost` / `5433` / `kernelstore` | Postgres |
| `DB_USER` / `DB_PASSWORD` / `DB_SSLMODE` | `admin` / `admin123` / `disable` | |
| `JWT_SECRET` | (chuỗi dev) | khoá ký JWT — **đổi khi triển khai thật** |
| `JWT_ACCESS_EXPIRY_MINS` / `JWT_REFRESH_EXPIRY_DAYS` | `30` / `7` | hạn token |
| `CORS_ORIGINS` | `http://localhost:8080,http://127.0.0.1:8080` | origin frontend |
| `UPLOAD_DIR` | `backend/uploads` | thư mục ảnh upload |
| `PUBLIC_URL` | (theo Host của request) | địa chỉ backend trình duyệt truy cập được, dùng dựng URL ảnh upload (Docker: `http://localhost:5000`) |

### 4. Seed dữ liệu mẫu (tuỳ chọn, 1 lệnh)

```sh
./seed.sh                     # = cd backend && go run ./cmd/api seed
```

Tạo **10 danh mục, 7 shop (Approved) + 57 sản phẩm** (kèm ảnh). Idempotent — chạy lại báo `already present — skipping`.

| Chuyên ngành | Shop | Ví dụ sản phẩm |
|---|---|---|
| IoT & Embedded | IoT Depot | Raspberry Pi 5, ESP32, Arduino R4, LoRa Gateway, Zigbee Hub |
| AI & Machine Learning | Neural Forge | RTX 4090, Jetson Orin, Google Coral, A100 80GB |
| Cybersecurity | SecOps Armory | YubiKey 5, Flipper Zero, WiFi Pineapple, Proxmark3 |
| SysAdmin & DevOps | OpsCenter | UniFi Dream Machine, 1U Server, Synology NAS, UPS rack |
| Developer Tools | DevTools Hub | Keychron Q1, màn 4K, Stream Deck, license JetBrains/Copilot |

Ảnh nằm ở `backend/uploads/`, phục vụ tại `http://localhost:5000/uploads/<slug>.jpg`.

### 5. Frontend (Go + templ + HTMX)

Terminal thứ 2 (cũng `nix develop`):

```sh
cd frontend
air                           # templ generate → tailwind → build → chạy :8080, tự reload khi sửa code
```

Không dùng air: `templ generate && tailwindcss -c tailwind.config.js -i static/css/input.css -o static/css/app.css --minify && go run ./cmd/web`.
Biến môi trường: `WEB_ADDR` (`:8080`), `API_BASE` (`http://localhost:5000/api`), `WS_BASE` (`ws://localhost:5000/ws/chat`), `STATIC_DIR`.

### Chạy cả stack một lệnh

```sh
nix develop -c ./run.sh
```

`docker compose up -d postgres` → đợi Postgres healthy → build + chạy backend nền (log `/tmp/kernelstore-backend.log`) → đợi `:5000` → chạy frontend bằng `air`. **Ctrl+C** dừng frontend và tắt backend (Postgres vẫn chạy; `docker compose down` để tắt hẳn).

### Nâng phiên bản công cụ

```sh
nix flake update          # cập nhật nixpkgs, ghi lại flake.lock (commit cả flake.lock)
nix develop -c go version # kiểm tra vẫn là go1.27.x
```

### Build một lần (không chạy dev server)

```sh
nix build                 # backend  → ./result/bin/kernelstore-api
nix build .#frontend      # frontend → ./result/bin/kernelstore-web (kèm static)
# hoặc bằng go:
(cd backend && go build ./...) && (cd frontend && templ generate && go build ./...)
```

### Tài khoản mặc định

| Vai trò   | Email               | Mật khẩu       | Nguồn            |
|-----------|---------------------|----------------|------------------|
| Admin     | `admin@ks.com`      | `Admin@12345`  | tạo tự động khi backend khởi động |
| Seller    | `seller1@demo.ks`   | `Seller@12345` | `./seed.sh` — TechWorld Store |
| Seller    | `seller2@demo.ks`   | `Seller@12345` | `./seed.sh` — GadgetHub |
| Seller    | `iot@demo.ks`       | `Seller@12345` | `./seed.sh` — IoT Depot |
| Seller    | `ai@demo.ks`        | `Seller@12345` | `./seed.sh` — Neural Forge |
| Seller    | `security@demo.ks`  | `Seller@12345` | `./seed.sh` — SecOps Armory |
| Seller    | `sysadmin@demo.ks`  | `Seller@12345` | `./seed.sh` — OpsCenter |
| Seller    | `developer@demo.ks` | `Seller@12345` | `./seed.sh` — DevTools Hub |
| Customer  | tự đăng ký ở `/auth/register` | —    | —                |

### Chạy test (API đang chạy trên :5000)

```sh
nix develop -c ./test_phase1_api.sh     # Auth                (21 checks)
nix develop -c ./test_phase2_api.sh     # Shop & Seller       (14 checks)
nix develop -c ./test_phase3_api.sh     # Product & Category  (27 checks)
nix develop -c ./test_phase4_api.sh     # Cart & Checkout     (22 checks)
nix develop -c ./test_phase5_api.sh     # Review & Rating     (15 checks)
nix develop -c ./test_phase6_api.sh     # Admin Panel         (32 checks)
nix develop -c ./test_chat_api.sh       # Chat realtime       (27 checks, tự build test/wsclient)
nix develop -c ./test_full_api.sh       # End-to-end mọi vai trò (52 checks)
nix develop -c ./test_extra_api.sh      # Các chức năng còn lại  (66 checks)
nix develop -c ./test_warranty_api.sh   # Bảo hành            (23 checks)
```

`test_chat`, `test_full`, `test_extra`, `test_warranty` cần dữ liệu mẫu (`./seed.sh`). Unit test Go: `cd backend && go test ./...`.

### Kết quả test gần nhất (2026-09-29, bản Go)

Chạy lại trên backend Go (Postgres + `:5000`, sau `./seed.sh`) — **276/276 checks PASS trên 9 bộ test**, cộng bộ bảo hành 23/23:

| Bộ test | Kết quả | Phạm vi |
|---------|---------|---------|
| `test_phase1_api.sh` | ✅ 21/21 PASS | Auth: register/login/sai mật khẩu→401/me/refresh + rotation (reuse→401) |
| `test_phase2_api.sh` | ✅ 14/14 PASS | Shop & Seller: mở shop→Pending→admin approve→Approved, cập nhật shop, phân quyền |
| `test_phase3_api.sh` | ✅ 27/27 PASS | Product & Category: CRUD, cross-seller 404, lọc/sắp xếp/phân trang, ẩn sản phẩm inactive |
| `test_phase4_api.sh` | ✅ 22/22 PASS | Cart & Checkout: add/update, vượt stock→lỗi, tạo đơn trừ stock, seller đổi status |
| `test_phase5_api.sh` | ✅ 15/15 PASS | Review: chưa nhận hàng→403, Delivered mới review, điểm trung bình, chống trùng |
| `test_phase6_api.sh` | ✅ 32/32 PASS | Admin: dashboard (8 trạng thái đơn), ban/unban user, category CRUD + chặn xoá khi có con |
| `test_chat_api.sh` | ✅ 27/27 PASS | Auth 401 (REST+WS), hội thoại idempotent/chặn shop mình/404, tin rỗng/>2000→400, non-participant→403, unread, **WebSocket realtime** 2 chiều |
| `test_full_api.sh` | ✅ 52/52 PASS | End-to-end mọi vai trò: browse công khai, giỏ, đơn, seller, admin, vòng đời mua→giao→xác nhận→đánh giá, RBAC |
| `test_extra_api.sh` | ✅ 66/66 PASS | Category CRUD, Customer→Seller, guard sản phẩm theo trạng thái shop, ban tạm thời/vĩnh viễn, huỷ đơn hoàn kho, trả hàng, refresh-token reuse, **upload ảnh** (jpg/png/svg, nosniff) |
| `test_warranty_api.sh` | ✅ 23/23 PASS | Bảo hành: điều kiện gửi (Delivered, còn hạn, không trùng), vòng đời Pending→Approved→Processing→Completed, huỷ, phân quyền |

### Xử lý sự cố

- **`error: experimental Nix feature 'flakes' is disabled`** (hoặc `'nix-command'`) → chưa bật flakes. Thêm `nix.settings.experimental-features = [ "nix-command" "flakes" ];` vào `configuration.nix` rồi `sudo nixos-rebuild switch`. Tạm thời: `nix --extra-experimental-features 'nix-command flakes' develop`.
- **`nix develop`/`nix build` báo không thấy file vừa tạo** (`No such file`, `path ... does not exist`) → flake chỉ thấy file đã git track: `git add <file>` rồi chạy lại.
- **Sai phiên bản Go** (`go.mod requires go >= 1.27` hoặc `go` tự tải toolchain) → đang dùng Go ngoài flake. Chạy trong `nix develop` (đặt sẵn `GOTOOLCHAIN=local`); kiểm tra `go version` phải là `go1.27.x`.
- **Frontend lỗi build `undefined: views.XxxPage`** hoặc giao diện không đổi sau khi sửa `.templ` → chưa sinh code: `cd frontend && templ generate` (air tự làm việc này).
- **Trang hiện nhưng không có style** → chưa build CSS: `tailwindcss -c tailwind.config.js -i static/css/input.css -o static/css/app.css`.
- **Trang báo lỗi `network error: ... connection refused`** → backend chưa chạy ở `:5000`. Kiểm tra: `curl http://localhost:5000/api/categories` phải trả JSON.
- **`Cannot connect to the Docker daemon`** → chưa bật `virtualisation.docker.enable` hoặc chưa vào group `docker`.
- **Backend không nối được DB** → `docker compose ps` (postgres healthy) và biến `DB_PORT` (mặc định `5433`).
- **`listen tcp :8080: bind: address already in use`** → cổng đang bị chương trình khác dùng: tắt nó (`ss -ltnp | grep 8080`) hoặc chạy frontend ở cổng khác: `WEB_ADDR=:8081` (frontend gọi API từ phía server nên không cần sửa CORS).

### Postgres không dùng Docker

`nix develop` có sẵn `postgresql_16` (gồm cả `initdb`/`pg_ctl`). Chạy Postgres cục bộ thay cho Docker:

```sh
initdb -D .pgdata
pg_ctl -D .pgdata -o "-p 5433" -l .pgdata/log start
createuser -p 5433 -s admin
psql -p 5433 -d postgres -c "ALTER USER admin PASSWORD 'admin123';"
createdb -p 5433 -O admin kernelstore
```

Cổng khác `5433`? Đặt `DB_PORT=<cổng>` khi chạy backend.

## API endpoints

Ký hiệu quyền: **—** công khai · **Bearer** cần đăng nhập · **Seller** / **Admin** / **S/A** (Seller hoặc Admin).

| Method | Path | Quyền | Mô tả |
|--------|------|-------|-------|
| POST | /api/auth/register | — | Đăng ký (Customer) |
| POST | /api/auth/login | — | Đăng nhập → JWT + refresh token |
| POST | /api/auth/refresh | — | Đổi refresh token (dùng một lần) |
| GET | /api/auth/me | Bearer | Thông tin user + roles |
| POST | /api/shops | Bearer | Mở shop (Pending), Customer → Seller |
| GET / PUT | /api/shops/me | Bearer | Xem / cập nhật shop của mình |
| GET | /api/admin/shops?status= | Admin | Danh sách shop |
| POST | /api/admin/shops/{id}/approve \| reject \| ban \| unban | Admin | Duyệt / từ chối / ban tạm thời / gỡ ban |
| DELETE | /api/admin/shops/{id} | Admin | Ban vĩnh viễn: xoá cứng (chưa có đơn) / xoá mềm `Deleted` (có đơn) |
| GET | /api/admin/users?search=&role=&isActive= | Admin | Danh sách user |
| POST | /api/admin/users/{id}/ban \| unban | Admin | Vô hiệu hoá (thu hồi refresh token) / kích hoạt lại |
| GET | /api/admin/orders?status=&search=&page=&pageSize= | Admin | Đơn toàn hệ thống (phân trang) |
| GET | /api/admin/dashboard | Admin | Thống kê hệ thống |
| GET | /api/seller/dashboard | S/A | Thống kê shop (doanh thu, top 5 sản phẩm) |
| GET | /api/categories | — | Cây danh mục chung |
| GET | /api/categories/{slug} | — | Chi tiết danh mục |
| POST / PUT / DELETE | /api/categories[/{id}] | Admin | Tạo / sửa / xoá (chặn khi có con/sản phẩm) |
| GET / POST | /api/seller/categories | Seller | Danh mục riêng của shop |
| PUT / DELETE | /api/seller/categories/{id} | Seller | Sửa / xoá (gỡ khỏi sản phẩm) |
| GET | /api/products?category=&shop=&minPrice=&maxPrice=&search=&sort=&page=&pageSize= | — | Danh sách sản phẩm (lọc, sắp xếp, phân trang) |
| GET | /api/products/featured?take= | — | Sản phẩm mới |
| GET | /api/products/{slug\|id} | — | Chi tiết + review + shop |
| GET | /api/products/my | Bearer | Sản phẩm của shop mình |
| POST | /api/products | Seller | Tạo sản phẩm (shop phải Approved) |
| PUT / DELETE | /api/products/{id} | Seller | Sửa / xoá sản phẩm của shop mình |
| POST | /api/uploads/image | Bearer | Upload ảnh jpg/png/svg (≤ 5MB) |
| GET / POST | /api/cart | Bearer | Xem giỏ / thêm vào giỏ |
| PUT / DELETE | /api/cart/{productId} | Bearer | Đổi số lượng (≤0 = xoá) / xoá |
| POST | /api/orders | Bearer | Đặt hàng từ giỏ (1 transaction, trừ kho) |
| GET | /api/orders | Bearer | Lịch sử đơn (theo vai trò) |
| GET | /api/orders/sales?status= | S/A | Đơn bán của shop |
| GET | /api/orders/{id} | Bearer | Chi tiết đơn |
| PUT | /api/orders/{id}/status | S/A | Đổi trạng thái đơn |
| POST | /api/orders/{id}/confirm-received \| cancel \| return | Bearer | Khách: xác nhận nhận hàng / huỷ (hoàn kho) / yêu cầu trả |
| POST | /api/orders/{id}/return/{approve\|reject} | S/A | Duyệt / từ chối trả hàng |
| GET | /api/reviews?productId= | — | Review của sản phẩm + điểm trung bình |
| POST | /api/reviews | Bearer | Đánh giá (chỉ sau khi đơn Delivered, 1 lần) |
| POST | /api/warranty | Bearer | Gửi yêu cầu bảo hành |
| GET | /api/warranty/mine | Bearer | Yêu cầu của tôi |
| GET | /api/warranty/shop?status= | S/A | Yêu cầu gửi tới shop / toàn hệ thống |
| GET | /api/warranty/{id} | Bearer | Chi tiết yêu cầu |
| POST | /api/warranty/{id}/cancel | Bearer | Khách huỷ (khi Pending) |
| POST | /api/warranty/{id}/approve \| reject \| process \| complete | S/A | Xử lý yêu cầu |
| GET / POST | /api/chat/conversations | Bearer | Danh sách hội thoại / mở hội thoại với shop |
| GET / POST | /api/chat/conversations/{id}/messages | Bearer | Lịch sử (đánh dấu đã đọc) / gửi tin (đẩy realtime) |
| WS | /ws/chat?access_token=JWT | token | Nhận tin realtime |
| GET | /uploads/{file} | — | Ảnh sản phẩm (`X-Content-Type-Options: nosniff`) |

Response format: `{ "success": bool, "data": ..., "message": string, "errors": [] }` (JSON camelCase).

## Ghi chú kỹ thuật

- **JWT:** HS256, access token 30 phút, refresh token 7 ngày. Claims `nameid`, `unique_name`, `email`, `role`; role có hiệu lực gồm cả cột `AspNetUsers.Role` lẫn role đã gán.
- **Refresh token rotation:** mỗi refresh token chỉ dùng một lần — được đánh dấu `IsUsed` bằng một câu `UPDATE` có điều kiện, nên dùng lại (kể cả hai request song song) đều bị `401`. Ban user sẽ thu hồi mọi refresh token.
- **Mật khẩu:** PBKDF2-HMAC-SHA512, 100.000 vòng, salt 16 byte (hash dạng base64 bắt đầu bằng `AQAAAAIAAYag`) — tài khoản có sẵn trong `database.sql` đăng nhập được bình thường.
- **Đơn hàng:** tạo đơn trong một transaction; trừ kho có điều kiện (`StockQuantity >= qty`) nên đặt hàng song song không bán vượt kho; đổi trạng thái khoá dòng đơn (`FOR UPDATE`). Mã đơn `KS-yyyyMMdd-XXXX`, giá chốt `SalePrice ?? Price`.
- **Chat realtime:** REST để ghi/lấy tin (`/api/chat/*`), WebSocket `/ws/chat` chỉ đẩy tin tới người nhận (token qua query `access_token`). Kết nối giữ **in-memory** — chỉ chạy **một instance** backend.
- **Frontend:** render ở server, gọi API backend; phiên đăng nhập lưu trong cookie HttpOnly (`ks_access`, `ks_refresh`), ngôn ngữ trong cookie `ks_lang`. Trang chat nhúng access token để trình duyệt mở WebSocket tới `:5000`.
- **Schema:** giữ nguyên `database.sql` (tên bảng/cột PascalCase); migration Go khớp 100% `pg_dump --schema-only`.
