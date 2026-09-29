@echo off
chcp 65001 >nul
cd /d "%~dp0"
echo ================================================
echo   KernelStore - chay toan bo bang Docker (Windows)
echo   Chi can Docker Desktop, khong can cai Go/templ/Tailwind
echo ================================================

echo.
echo [1/2] Build image + chay Postgres, Backend, Frontend (lan dau mat vai phut)...
docker compose up -d --build
if errorlevel 1 (
  echo.
  echo [LOI] Docker khong chay hoac build loi. Hay bat Docker Desktop roi thu lai.
  pause
  exit /b 1
)

if "%WEB_PORT%"=="" set WEB_PORT=8080

echo.
echo [2/2] Doi Frontend san sang tai :%WEB_PORT%...
:waitfrontend
curl -s -o nul http://localhost:%WEB_PORT%/auth/login
if errorlevel 1 (
  timeout /t 2 >nul
  goto waitfrontend
)

start "" http://localhost:%WEB_PORT%

echo.
echo ================================================
echo   Xong! Web: http://localhost:%WEB_PORT%   (admin@ks.com / Admin@12345)
echo   Seed du lieu mau : docker compose run --rm backend seed
echo   Xem log          : docker compose logs -f backend frontend
echo   Tat              : docker compose down
echo ================================================
pause
