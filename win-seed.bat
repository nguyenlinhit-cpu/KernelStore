@echo off
chcp 65001 >nul
cd /d "%~dp0backend"
echo === KernelStore: Seed du lieu mau ===
echo (Can Postgres dang chay. Idempotent - chay lai se bao "already present".)
echo.
go run ./cmd/api seed
echo.
echo Da seed: 10 danh muc, 7 shop (Approved), 57 san pham.
pause
