@echo off
chcp 65001 >nul
cd /d "%~dp0"
echo ================================================
echo   KernelStore - khoi dong toan bo (Windows)
echo ================================================

echo.
echo [1/5] Database (Docker)...
docker compose up -d
if errorlevel 1 (
  echo.
  echo [LOI] Docker khong chay. Hay bat Docker Desktop roi thu lai.
  pause
  exit /b 1
)

echo.
echo [2/5] Doi PostgreSQL san sang...
:waitdb
docker compose exec -T postgres pg_isready -U admin -d kernelstore >nul 2>&1
if errorlevel 1 (
  timeout /t 2 >nul
  goto waitdb
)
echo       Postgres healthy.

echo.
echo [3/5] Backend -^> http://localhost:5000 (cua so moi)
start "KernelStore Backend" cmd /k "cd /d "%~dp0backend" ^&^& go run ./cmd/api"

echo.
echo [4/5] Doi Backend san sang (lan dau phai build, co the vai phut)...
:waitbackend
curl -s -o nul http://localhost:5000/api/categories
if errorlevel 1 (
  timeout /t 2 >nul
  goto waitbackend
)
echo       Backend dang lang nghe tai :5000.

echo.
echo [5/5] Frontend -^> http://localhost:8080 (cua so moi)
start "KernelStore Frontend" cmd /k "call "%~dp0win-frontend.bat""

echo.
echo Doi Frontend build xong (lan dau tai thu vien Go, co the vai phut)...
:waitfrontend
curl -s -o nul http://localhost:8080
if errorlevel 1 (
  timeout /t 2 >nul
  goto waitfrontend
)
echo       Frontend san sang tai :8080.

echo.
echo Mo trinh duyet...
start "" http://localhost:8080

echo.
echo ================================================
echo   Xong! Web da mo tai http://localhost:8080
echo   Neu localhost loi/khong mo duoc, thu: http://127.0.0.1:8080/
echo   (2 cua so Backend + Frontend van chay - dung Ctrl+C de tat)
echo ================================================
echo.
echo (Tuy chon) Seed du lieu mau: chay win-seed.bat sau khi backend da san sang.
pause
