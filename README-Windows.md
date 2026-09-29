# KernelStore — Hướng dẫn chạy trên Windows

Hướng dẫn cho **Windows 10/11** (thay cho phần "Chạy trên NixOS" trong [README.md](./README.md)). Trên Windows không dùng Nix — bạn cài trực tiếp công cụ rồi chạy từng service.

Ứng dụng cần **3 tiến trình chạy cùng lúc**, đúng thứ tự **Database → Backend → Frontend**:

| Service    | Công nghệ                          | URL                     |
|------------|------------------------------------|-------------------------|
| Database   | PostgreSQL 16                      | `localhost:5433`        |
| Backend    | Go 1.27 (REST API + WebSocket)     | `http://localhost:5000` |
| Frontend   | Go + templ + HTMX + Tailwind CSS   | `http://localhost:8080` |

> ⚠️ **Cổng Postgres là `5433`** (mặc định của backend, biến `DB_PORT`). Đừng nhầm sang `5432`.

---

## 📑 Mục lục

1. [Cài đặt công cụ (một lần)](#-bước-0--cài-đặt-công-cụ-một-lần)
2. [Bước 1 — Database: **Phương án A (Docker)** hoặc **Phương án B (PostgreSQL native)**](#-bước-1--database-chọn-1-trong-2-phương-án)
3. [Bước 2 — Backend](#-bước-2--backend-go-api)
4. [Bước 3 — Seed dữ liệu mẫu (tuỳ chọn)](#-bước-3--seed-dữ-liệu-mẫu-tuỳ-chọn)
5. [Bước 4 — Frontend](#-bước-4--frontend-go--templ--htmx)
6. [Tài khoản mặc định](#-tài-khoản-mặc-định) · [Chạy test](#-chạy-test-tuỳ-chọn) · [Xử lý sự cố](#-xử-lý-sự-cố-windows)

---

## 🔧 Bước 0 — Cài đặt công cụ (một lần)

### 1. Go 1.27

Tải bộ cài **`go1.27.1.windows-amd64.msi`** (hoặc bản vá 1.27.x mới hơn) tại **https://go.dev/dl/** rồi chạy, hoặc dùng winget:

```powershell
winget install GoLang.Go
```

Bộ cài tự thêm `C:\Program Files\Go\bin` và `%USERPROFILE%\go\bin` vào `PATH`. **Đóng và mở lại PowerShell**, rồi kiểm tra:

```powershell
go version        # phải là go1.27.x — thấp hơn thì cài lại bản mới
```

### 2. templ (sinh code Go từ file `.templ`)

```powershell
go install github.com/a-h/templ/cmd/templ@v0.3.1020
templ version     # v0.3.1020
```

### 3. Tailwind CSS CLI (bản v3, khớp `frontend\tailwind.config.js`)

Tải file **`tailwindcss-windows-x64.exe`** của bản **v3.4.17** tại
https://github.com/tailwindlabs/tailwindcss/releases/tag/v3.4.17 , đổi tên thành **`tailwindcss.exe`** và chép vào `%USERPROFILE%\go\bin` (đã có trong `PATH`):

```powershell
Invoke-WebRequest https://github.com/tailwindlabs/tailwindcss/releases/download/v3.4.17/tailwindcss-windows-x64.exe -OutFile "$env:USERPROFILE\go\bin\tailwindcss.exe"
tailwindcss --help      # in ra "tailwindcss v3.4.17"
```

> Không dùng bản v4: cấu hình Tailwind của dự án là định dạng v3.

### 4. Database engine (theo phương án ở Bước 1)

```powershell
winget install Docker.DockerDesktop          # Phương án A (Docker)
winget install PostgreSQL.PostgreSQL.16      # Phương án B (PostgreSQL native)
```

### 5. (Tuỳ chọn) air — tự reload frontend khi sửa code

```powershell
go install github.com/air-verse/air@latest
```

<details>
<summary><b>📥 Nguồn cài đặt chính thức (bấm để mở)</b></summary>

| Công cụ | Cách cài | Trang chính thức | Dùng cho |
|---|---|---|---|
| Go 1.27 | `.msi` hoặc `winget install GoLang.Go` | https://go.dev/dl/ | Backend + frontend (bắt buộc) |
| templ | `go install github.com/a-h/templ/cmd/templ@v0.3.1020` | https://templ.guide | Frontend (bắt buộc) |
| Tailwind CSS CLI v3 | file `.exe` standalone | https://github.com/tailwindlabs/tailwindcss/releases | Frontend (bắt buộc) |
| Docker Desktop | `winget install Docker.DockerDesktop` | https://www.docker.com/products/docker-desktop/ | **Phương án A** |
| PostgreSQL 16 | `winget install PostgreSQL.PostgreSQL.16` | https://www.postgresql.org/download/windows/ | **Phương án B** |
| air | `go install github.com/air-verse/air@latest` | https://github.com/air-verse/air | Hot reload (tuỳ chọn) |
| Git for Windows | `winget install Git.Git` | https://git-scm.com/download/win | Chạy test `.sh` (tuỳ chọn) |
| jq | `winget install jqlang.jq` | https://jqlang.github.io/jq/download/ | Chạy test `.sh` (tuỳ chọn) |

> ⚠️ Chỉ tải từ các domain trên, tránh trang mirror bên thứ ba.

</details>

Không cần Visual Studio / C++ Build Tools: dự án build thuần Go, không dùng cgo.

---

## 🗄️ Bước 1 — Database (chọn 1 trong 2 phương án)

Hai phương án **cho ra database y hệt nhau** — db `kernelstore`, user `admin`, mật khẩu `admin123`, cổng `5433`.

| | **Phương án A — Docker** | **Phương án B — PostgreSQL native** |
|---|---|---|
| Cần cài | Docker Desktop | PostgreSQL 16 |
| Ưu điểm | Có sẵn `win-run-all.bat` chạy tự động cả stack | Không cần Docker, nhẹ máy |

---

### Phương án A — Docker Desktop (khuyến nghị)

> Yêu cầu: đã cài và **bật Docker Desktop** (đợi trạng thái *Running*).

#### ⭐ Cách A1 — Tự động cả stack bằng `win-run-all.bat`

`win-run-all.bat` làm **toàn bộ**: bật DB → đợi healthy → mở Backend → mở Frontend → **tự mở trình duyệt**. Chạy file này thì **bỏ qua Bước 2–4**.

- **Double-click** `win-run-all.bat`, hoặc trong PowerShell: `.\win-run-all.bat`

```text
[1/5] Database (Docker)...              → bật Postgres
[2/5] Doi PostgreSQL san sang...        → đợi DB healthy
[3/5] Backend -> :5000                  → CỬA SỔ MỚI: go run ./cmd/api
[4/5] Doi Backend san sang...           → đợi build + lắng nghe :5000
[5/5] Frontend -> :8080                 → CỬA SỔ MỚI: win-frontend.bat (templ + tailwind + go run)
Mo trinh duyet...                       → TỰ mở http://localhost:8080
```

- **Lần đầu chờ lâu hơn** vì Go tải thư viện và biên dịch — script đứng đợi là bình thường.
- **Tắt:** `Ctrl + C` ở 2 cửa sổ Backend & Frontend. DB vẫn chạy nền → `docker compose down` để tắt hẳn.

> Cảnh báo "Windows protected your PC" khi double-click → **More info → Run anyway**.

#### Cách A2 — Thủ công

```powershell
cd C:\path\to\KernelStore
docker compose up -d      # postgres:16 tại localhost:5433
docker compose ps         # đợi STATUS = healthy
```

Nạp sẵn dữ liệu mẫu từ `database.sql` (tuỳ chọn, thay cho Bước 3):

```powershell
Get-Content database.sql | docker exec -i kernelstore-postgres psql -U admin -d kernelstore
```

#### Các file `.bat`

| File | Việc |
|------|------|
| `win-run-all.bat`  | Tự động cả stack (Cách A1) — cần Docker. |
| `win-db.bat`       | Chỉ bật Database (Docker). |
| `win-backend.bat`  | Chỉ chạy Backend (`go run ./cmd/api`, `:5000`). |
| `win-frontend.bat` | Chỉ chạy Frontend (`templ generate` → tailwind → `go run ./cmd/web`, `:8080`). |
| `win-seed.bat`     | Seed dữ liệu mẫu (`go run ./cmd/api seed`). |

`win-backend.bat`, `win-frontend.bat`, `win-seed.bat` dùng được cho cả Phương án B.

---

### Phương án B — PostgreSQL native (không cần Docker)

#### B1. Cho Postgres nghe cổng `5433`

Bản native mặc định nghe `5432`. Sửa `port = 5432` → `port = 5433` rồi restart:

```powershell
notepad "C:\Program Files\PostgreSQL\16\data\postgresql.conf"
Restart-Service postgresql-x64-16
```

> Muốn giữ `5432`? Thay vào đó đặt biến môi trường khi chạy backend: `$env:DB_PORT = "5432"` (trong cùng cửa sổ PowerShell trước `go run`).

#### B2. Tạo user + database

```powershell
psql -U postgres -p 5433 -c "CREATE USER admin WITH PASSWORD 'admin123' SUPERUSER;"
psql -U postgres -p 5433 -c "CREATE DATABASE kernelstore OWNER admin;"
```

#### B3. Dữ liệu — chọn 1

- **Để backend tự tạo schema:** sang thẳng Bước 2 (backend tự chạy migration + tạo admin), rồi Bước 3 (seed).
- **Nạp sẵn `database.sql`:** `psql -U admin -p 5433 -d kernelstore -f database.sql`

---

## ⚙️ Bước 2 — Backend (Go API)

> Đã chạy `win-run-all.bat`? Bỏ qua bước này.

```powershell
cd C:\path\to\KernelStore\backend
go run ./cmd/api
```

(hoặc double-click `win-backend.bat`)

- Khi khởi động: chạy migration + tạo roles `Customer/Seller/Admin` + tài khoản admin.
- Đợi log **`Now listening on: http://localhost:5000`**. Kiểm tra: `curl http://localhost:5000/api/categories` trả JSON.
- Cấu hình qua biến môi trường (`DB_PORT`, `DB_PASSWORD`, `JWT_SECRET`, `UPLOAD_DIR`, ...) — bảng đầy đủ ở [README.md](./README.md#3-backend-go-api). Ví dụ: `$env:DB_PORT = "5432"; go run ./cmd/api`.

---

## 🌱 Bước 3 — Seed dữ liệu mẫu (tuỳ chọn)

> Bỏ qua nếu đã nạp `database.sql`.

```powershell
cd C:\path\to\KernelStore\backend
go run ./cmd/api seed
```

(hoặc `win-seed.bat`) — tạo **10 danh mục, 7 shop (Approved) + 57 sản phẩm** kèm ảnh. Chạy lại báo `already present — skipping`.

| Chuyên ngành | Shop | Ví dụ sản phẩm |
|---|---|---|
| IoT & Embedded | IoT Depot | Raspberry Pi 5, ESP32, Arduino R4, LoRa Gateway |
| AI & Machine Learning | Neural Forge | RTX 4090, Jetson Orin, Google Coral, A100 80GB |
| Cybersecurity | SecOps Armory | YubiKey 5, Flipper Zero, WiFi Pineapple, Proxmark3 |
| SysAdmin & DevOps | OpsCenter | UniFi Dream Machine, 1U Server, Synology NAS |
| Developer Tools | DevTools Hub | Keychron Q1, màn 4K, Stream Deck, license JetBrains |

Ảnh ở `backend\uploads\`, phục vụ tại `http://localhost:5000/uploads/<slug>.jpg`.

---

## 🖥️ Bước 4 — Frontend (Go + templ + HTMX)

> Đã chạy `win-run-all.bat`? Bỏ qua bước này.

Double-click **`win-frontend.bat`**, hoặc:

```powershell
cd C:\path\to\KernelStore\frontend
templ generate
tailwindcss -c tailwind.config.js -i static/css/input.css -o static/css/app.css --minify
go run ./cmd/web
```

Mở **`http://localhost:8080`**. Sửa file `.templ`/`.css` thì chạy lại 3 lệnh trên (hoặc dùng `air`, xem [README.md](./README.md#5-frontend-go--templ--htmx)).

> `localhost:8080` không mở được? Thử **`http://127.0.0.1:8080/`** — backend cho phép cả hai origin.

---

## 👤 Tài khoản mặc định

Admin có sẵn khi backend khởi động. 7 seller chỉ có sau khi **seed** (Bước 3) hoặc **nạp `database.sql`**.

| Vai trò   | Email               | Mật khẩu       | Shop            |
|-----------|---------------------|----------------|-----------------|
| Admin     | `admin@ks.com`      | `Admin@12345`  | — (tự động)     |
| Seller    | `seller1@demo.ks`   | `Seller@12345` | TechWorld Store |
| Seller    | `seller2@demo.ks`   | `Seller@12345` | GadgetHub       |
| Seller    | `iot@demo.ks`       | `Seller@12345` | IoT Depot       |
| Seller    | `ai@demo.ks`        | `Seller@12345` | Neural Forge    |
| Seller    | `security@demo.ks`  | `Seller@12345` | SecOps Armory   |
| Seller    | `sysadmin@demo.ks`  | `Seller@12345` | OpsCenter       |
| Seller    | `developer@demo.ks` | `Seller@12345` | DevTools Hub    |
| Customer  | tự đăng ký ở `/auth/register` | —    | —               |

---

## ✅ Chạy test (tuỳ chọn)

Các file `test_*.sh` là script bash: cần **Git Bash** (hoặc WSL) có `jq`, `curl` và `go` trong `PATH`, backend đang chạy ở `:5000`, đã seed dữ liệu mẫu. Trong **Git Bash**:

```bash
./test_phase1_api.sh     # Auth               (21 checks)
./test_phase2_api.sh     # Shop & Seller      (14 checks)
./test_phase3_api.sh     # Product & Category (27 checks)
./test_phase4_api.sh     # Cart & Checkout    (22 checks)
./test_phase5_api.sh     # Review & Rating    (15 checks)
./test_phase6_api.sh     # Admin Panel        (32 checks)
./test_chat_api.sh       # Chat realtime      (27 checks, tự build test/wsclient bằng go)
./test_full_api.sh       # End-to-end         (52 checks)
./test_extra_api.sh      # Các chức năng còn lại (66 checks)
./test_warranty_api.sh   # Bảo hành           (23 checks)
```

> `test_chat_api.sh` ghi log vào `/tmp/opencode/` — trong Git Bash tạo trước: `mkdir -p /tmp/opencode`.

Kết quả verify gần nhất (2026-09-29): **276/276 PASS** trên 9 bộ test + bảo hành 23/23 (chi tiết trong [README.md](./README.md#kết-quả-test-gần-nhất-2026-09-29-bản-go)).

### Build một lần (không chạy dev server)

```powershell
cd backend;  go build -o kernelstore-api.exe ./cmd/api
cd ..\frontend;  templ generate;  go build -o kernelstore-web.exe ./cmd/web
```

---

## 🩺 Xử lý sự cố (Windows)

- **`go`/`templ`/`tailwindcss` không nhận lệnh** → chưa mở lại terminal sau khi cài, hoặc `%USERPROFILE%\go\bin` chưa có trong `PATH`.
- **`go: go.mod requires go >= 1.27`** → Go đang cài thấp hơn 1.27. Cài lại bản mới từ https://go.dev/dl/ và kiểm tra `go version`.
- **Frontend lỗi build `undefined: views.XxxPage`** hoặc sửa `.templ` mà giao diện không đổi → chưa chạy `templ generate` trong thư mục `frontend`.
- **Trang hiện nhưng mất style** → chưa build CSS (lệnh `tailwindcss ...` ở Bước 4), hoặc đang dùng Tailwind v4 thay vì v3.
- **Trang báo `network error: ... connection refused`** → backend chưa chạy ở `:5000` (Bước 2).
- **`Cannot connect to the Docker daemon`** (Phương án A) → chưa bật Docker Desktop.
- **Backend không nối được DB** → DB chưa chạy, hoặc cổng không khớp: mặc định `5433`, đổi bằng `$env:DB_PORT`.
- **Port bị chiếm (5000/5433/8080)** → tìm tiến trình: `netstat -ano | findstr :8080`. Đổi cổng frontend: `$env:WEB_ADDR = ":8081"`.
- **Reset sạch DB** → Docker: `docker compose down -v` rồi `up -d`; native: `DROP DATABASE kernelstore;` rồi tạo lại. Sau đó seed lại hoặc nạp `database.sql`.
