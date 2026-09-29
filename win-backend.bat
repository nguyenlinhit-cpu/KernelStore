@echo off
chcp 65001 >nul
cd /d "%~dp0backend"
echo === KernelStore: Backend API (Go) -^> http://localhost:5000 ===
echo (De cua so nay chay. Dung bang Ctrl + C.)
echo.
go run ./cmd/api
pause
