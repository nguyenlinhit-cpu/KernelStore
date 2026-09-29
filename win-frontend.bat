@echo off
chcp 65001 >nul
cd /d "%~dp0frontend"
echo === KernelStore: Frontend (Go + templ + HTMX) -^> http://localhost:8080 ===
echo (De cua so nay chay. Dung bang Ctrl + C.)
echo.
templ generate
if errorlevel 1 goto fail
tailwindcss -c tailwind.config.js -i static/css/input.css -o static/css/app.css --minify
if errorlevel 1 goto fail
go run ./cmd/web
pause
exit /b 0
:fail
echo.
echo [LOI] Thieu templ hoac tailwindcss. Xem README-Windows.md.
pause
exit /b 1
