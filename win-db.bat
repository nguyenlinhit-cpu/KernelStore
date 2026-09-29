@echo off
chcp 65001 >nul
cd /d "%~dp0"
echo === KernelStore: Database (PostgreSQL 16 qua Docker) ===
docker compose up -d postgres
if errorlevel 1 (
  echo.
  echo [LOI] Docker khong chay. Hay bat Docker Desktop roi thu lai.
  pause
  exit /b 1
)
docker compose ps
echo.
echo Postgres dang chay tai localhost:5433 (db/user/pass: kernelstore/admin/admin123)
pause
