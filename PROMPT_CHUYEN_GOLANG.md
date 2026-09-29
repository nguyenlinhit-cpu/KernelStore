# Prompt: Chuyển KernelStore sang Golang (backend + frontend + Nix flake + README)

> Dán toàn bộ nội dung từ mục "PROMPT" bên dưới vào Antigravity (chọn Claude Opus 4.6 Thinking).
> Không cho chạy một mạch: duyệt bước 0 (bảng ánh xạ) xong mới cho làm tiếp.

---

## PROMPT

Bạn là senior engineer. Hãy chuyển toàn bộ dự án KernelStore (marketplace đa nhà cung cấp) sang Golang: cả backend (ASP.NET Core 10 + EF Core + PostgreSQL) lẫn frontend (Rust + Leptos CSR + Trunk + Tailwind). Đồng thời chuyển môi trường dev từ `shell.nix` sang Nix flake (có `flake.lock`) và viết lại toàn bộ tài liệu. Đọc README.md và KernelStore_CauTrucThuMuc.md trước.

### 1. Phiên bản Go (bắt buộc)
- Dùng Go stable mới nhất: **Go 1.27.1** (hoặc bản vá mới hơn của 1.27 nếu có). Trong `go.mod` ghi `go 1.27` (kèm `toolchain go1.27.1` nếu cần).
- Trước khi bắt đầu, chạy `go version`. Nếu thấp hơn 1.27 thì dừng lại và hướng dẫn tôi cài từ https://go.dev/dl/, không tự hạ phiên bản.
- Dùng thư viện bản mới nhất còn tương thích Go 1.27: kiểm tra bằng `go list -m -u all`, không copy phiên bản cũ từ trí nhớ.
- Ưu tiên tính năng chuẩn của Go hiện đại (net/http ServeMux có method + wildcard, log/slog, generics, iterators, min/max, slices/maps) thay vì thêm thư viện khi không cần.

### 2. Ràng buộc bắt buộc
1. GIỮ NGUYÊN hợp đồng API: mọi endpoint, path, method, phân quyền, status code, và response format `{ "success", "data", "message", "errors" }` (JSON camelCase). Danh sách endpoint ở README.
2. GIỮ NGUYÊN schema PostgreSQL 16 (`database.sql`, `docker-compose.yml`, port 5433, db/user/pass: kernelstore/admin/admin123). Không đổi tên bảng/cột.
3. Backend chạy ở `:5000`, frontend ở `:8080` để 9 bộ test hiện có (`test_phase1..6`, `test_chat`, `test_full`, `test_extra`) chạy lại được mà không sửa logic. Mục tiêu: **276/276 checks PASS**.
4. GIỮ NGUYÊN hành vi nghiệp vụ:
   - JWT HS256 (access 30 phút, refresh 7 ngày, single-use rotation, reuse → 401)
   - Roles Customer/Seller/Admin, seed admin `admin@ks.com` / `Admin@12345`
   - Guard: shop phải Approved mới được tạo/sửa/xóa sản phẩm
   - Ban tạm thời (Banned, ẩn sản phẩm) và ban vĩnh viễn (xóa cứng nếu không có đơn, xóa mềm `Deleted` nếu có đơn, giữ lịch sử đơn)
   - Cancel khôi phục stock, return flow (Shipped → ReturnRequested → Returned)
   - Tạo đơn trong 1 transaction, OrderCode `KS-yyyyMMdd-XXXX` (unique), giá chốt SalePrice ?? Price
   - Review chỉ khi đơn Delivered, chống trùng
   - Upload ảnh (jpg/png/svg, header `X-Content-Type-Options: nosniff`)
   - Chat REST + WebSocket `/ws/chat?access_token=JWT`, `ChatConnectionManager` in-memory

### 3. Stack đề xuất
- **Backend**: Go 1.27, `net/http` (ServeMux) hoặc chi, pgx + sqlc, golang-migrate, golang-jwt, bcrypt/argon2, coder/websocket (hoặc gorilla), CORS cho `http://localhost:8080`.
- **Frontend**: Go SSR với templ + HTMX + Tailwind CSS. Giữ nguyên giao diện terminal: typewriter loading, KernelPanic 404/500, toast `[INFO]/[WARN]/[ERROR]/[OK]`, bảng ASCII, progress bar `[#### ]`. Thay Trunk bằng air (hot reload) + tailwind CLI.
- **Cấu trúc**:
  ```
  backend/   → cmd/api, internal/{handlers,services,repository,models,dto,middleware,ws}
  frontend/  → cmd/web, internal/{pages,components,client}, static/
  ```

### 4. Các giai đoạn (mỗi giai đoạn phải build + test xong rồi commit)
0. Tạo branch `go-migration`. Đọc toàn bộ code C# và Rust, lập **bảng ánh xạ** C# → Go (Entities, DTO, Controllers, Services, Seeder) và Leptos page/component → templ. **Chưa viết code**, đưa bảng cho tôi duyệt.
1. Nền tảng: go.mod, config, kết nối DB, middleware (JWT auth, role, CORS, lỗi → ApiResponse), migrations khớp schema hiện tại.
2. Auth + TokenService (register/login/refresh/me, seed roles + admin). Chạy `test_phase1`.
3. Shops + Admin shops. Chạy `test_phase2`.
4. Categories + Products + upload ảnh. Chạy `test_phase3`.
5. Cart + Orders (transaction, trừ stock, cancel/return). Chạy `test_phase4`.
6. Reviews. Chạy `test_phase5`.
7. Admin dashboard/users/orders. Chạy `test_phase6`.
8. Chat REST + WebSocket. Viết lại `test/wsclient` bằng Go (thay probe C#). Chạy `test_chat_api.sh`.
9. Seeder demo (10 categories, 7 shop, 57 sản phẩm, ảnh trong uploads/), `./seed.sh` gọi `go run ./cmd/api seed`, idempotent.
10. Frontend: chuyển lần lượt home, products, product_detail, login/register, cart, checkout, orders, seller (dashboard/products/settings), admin (dashboard/shops/categories), chat. Auth dùng cookie/session hoặc giữ JWT qua HTMX; protected routes redirect `/auth/login`; responsive như bản cũ.
11. Dọn dẹp: chạy lại toàn bộ `test_full` + `test_extra`, rồi mới xóa code C#/Rust cũ.
12. Chuyển Nix sang flake (mục 5).
13. Viết lại tài liệu (mục 6).

### 5. Chuyển Nix sang flake (giai đoạn 12)
1. Xóa `shell.nix`, tạo `flake.nix` ở gốc repo:
   - `inputs`: nixpkgs (nixos-unstable); flake-utils hoặc tự viết `forAllSystems` (ưu tiên không thêm phụ thuộc).
   - `outputs`: `devShells.default` (dùng cho `nix develop`) gồm: Go 1.27.x, gopls, delve, golangci-lint, air, templ, sqlc, golang-migrate, tailwindcss, nodejs (nếu cần), docker-compose, postgresql_16 (client), jq, curl. **BỎ** dotnet, rust, trunk, wasm target vì không còn dùng.
   - Kiểm tra `nix eval nixpkgs#go_1_27.version`. Nếu nixpkgs chưa có Go 1.27 thì pin nixpkgs mới hơn hoặc override bản Go từ source (đặt version + hash) và ghi rõ lý do trong comment. **Không được lặng lẽ hạ xuống bản thấp hơn.**
   - `shellHook`: in version go/templ/tailwind, đặt GOFLAGS/GOPATH hợp lý.
   - Thêm `packages.default` build backend bằng `buildGoModule` (nếu làm được; vendorHash lấy từ lỗi build lần đầu).
2. Sinh `flake.lock` bằng `nix flake lock` (**KHÔNG viết tay**), commit cả `flake.nix` và `flake.lock`. Flake chỉ thấy file đã git-tracked nên phải `git add flake.nix flake.lock` trước khi chạy `nix develop`.
3. Kiểm tra: `nix flake check`; `nix develop -c go version` phải in `go1.27.x`.
4. Cập nhật mọi chỗ dùng `nix-shell`:
   - `run.sh`, `seed.sh`, các `test_*.sh`: `nix-shell --run X` → `nix develop -c X`.
   - Ghi chú trong script: cần bật `nix.settings.experimental-features = [ "nix-command" "flakes" ];` trong configuration.nix.
   - `.gitignore`: giữ `flake.lock` được track; thêm `.direnv/` nếu dùng direnv.
   - Tuỳ chọn: thêm `.envrc` với `use flake`.

### 6. Cập nhật tài liệu (giai đoạn 13)
Viết lại `README.md`, `README-Windows.md`, `HUONG_DAN_CAI_DAT.md`, `KernelStore_CauTrucThuMuc.md` cho khớp bản Go:
- **Tech Stack mới**: Go 1.27 + (net/http hoặc chi) + pgx/sqlc + PostgreSQL 16; frontend Go + templ + HTMX + Tailwind. Bỏ mọi nhắc tới ASP.NET, EF Core, Rust, Leptos, Trunk, wasm32, dotnet.
- **Cây thư mục** mới đúng với cấu trúc thật sau khi chuyển.
- **Chạy nhanh / Chạy trên NixOS**: dùng `nix develop` và `nix develop -c ./run.sh`; giải thích yêu cầu bật flakes; `nix flake update` khi muốn nâng phiên bản.
- **Windows**: hướng dẫn cài Go 1.27 (.msi), templ, tailwind; cập nhật các file `win-*.bat` (`go run` thay `dotnet run`, `air` thay `trunk serve`).
- **Bảng API, tài khoản mặc định, kết quả test**: giữ nội dung, chỉ cập nhật lệnh chạy và ngày verify mới (tự chạy lại rồi ghi số liệu thật, không copy số cũ).
- **Xử lý sự cố**: bỏ mục `cc not found` / `wasm target`; thêm: flake không thấy file (chưa `git add`), lỗi "experimental feature flakes", sai phiên bản Go, templ chưa generate (`templ generate`).
- **Ghi chú kỹ thuật**: JWT, refresh rotation, chat WebSocket in-memory (chỉ 1 instance) giữ nguyên.

### 7. Quy tắc làm việc
- Sau mỗi giai đoạn: chạy `go build ./... && go vet ./...` và bộ test tương ứng, báo kết quả PASS/FAIL, rồi mới sang bước tiếp.
- Không xóa code C#/Rust cho đến khi toàn bộ test PASS; đặt code mới ở `backend-go/` và `frontend-go/` trước, sau đó mới thay thế.
- Không tự ý đổi hành vi, không bỏ tính năng, không thêm tính năng mới. Chỗ nào mơ hồ thì hỏi tôi.
- Mật khẩu: hash ASP.NET Identity (PBKDF2) không tương thích bcrypt. Seed lại dữ liệu là đủ, hoặc hỗ trợ verify hash cũ nếu cần giữ dữ liệu hiện có.
- Code Go idiomatic, xử lý lỗi rõ ràng, comment tiếng Việt ở chỗ nghiệp vụ khó.

**Bắt đầu với bước 0.**

---

## Ghi chú cho người dùng (không đưa vào prompt)
- Nếu Antigravity bị giới hạn ngữ cảnh, mỗi lần chỉ dán "Thực hiện giai đoạn N" kèm mục 1 và 2.
- Muốn nhẹ hơn: chỉ chuyển backend sang Go, giữ frontend Rust → bỏ bước 10, phần frontend ở mục 3, và các phần liên quan Rust/trunk trong mục 5, 6.
- Phiên bản Go stable mới nhất lúc soạn (29/09/2026): go1.27.1 (theo https://go.dev/dl/).
- `flake.lock` phải do `nix flake lock` sinh ra, không viết tay.
