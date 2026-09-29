// cmd/api/main.go — entry point cho KernelStore Go backend.
//
// Tương đương Program.cs trong bản C#:
//   - Đọc config
//   - Kết nối DB + chạy migrations
//   - Seed roles + admin
//   - Đăng ký middleware (CORS, Recovery, Logger)
//   - Đăng ký routes
//   - Phục vụ static files (uploads), đăng ký routes qua handlers.Register
//   - Lắng nghe :5000
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"syscall"
	"time"

	"github.com/KernelStore/backend-go/internal/config"
	"github.com/KernelStore/backend-go/internal/handlers"
	"github.com/KernelStore/backend-go/internal/httpx"
	"github.com/KernelStore/backend-go/internal/middleware"
	"github.com/KernelStore/backend-go/internal/repository"
	"github.com/KernelStore/backend-go/internal/services"
	"github.com/KernelStore/backend-go/internal/ws"
)

func main() {
	// Structured logging (JSON ở production, text ở dev).
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg := config.Load()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── Database ───────────────────────────────────────────────────────────
	// Xác định thư mục migrations (tương đối so với thư mục chạy hoặc đường dẫn tuyệt đối).
	migrationsDir, err := findMigrationsDir()
	if err != nil {
		slog.Error("Cannot find migrations directory", "err", err)
		os.Exit(1)
	}

	if err := repository.RunMigrations(cfg, migrationsDir); err != nil {
		slog.Error("Migration failed", "err", err)
		os.Exit(1)
	}

	pool, err := repository.NewPool(ctx, cfg)
	if err != nil {
		slog.Error("Database connection failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	// ── Seeder: tạo roles + admin (như DatabaseSeeder.SeedAsync trong C#) ──
	if err := seedRolesAndAdmin(ctx, pool, cfg); err != nil {
		slog.Error("Seed failed", "err", err)
		os.Exit(1)
	}

	// `go run ./cmd/api seed` — seed demo data rồi thoát.
	if slices.ContainsFunc(os.Args[1:], func(a string) bool { return a == "seed" || a == "--seed" }) {
		if err := seedDemoData(ctx, pool); err != nil {
			slog.Error("Seed demo data failed", "err", err)
			os.Exit(1)
		}
		return
	}

	// ── Services ──────────────────────────────────────────────────────────
	tokenSvc := services.NewTokenService(cfg)

	// ── Upload dir ────────────────────────────────────────────────────────
	uploadDir := cfg.UploadDir
	if uploadDir == "" {
		// Mặc định: backend/KernelStore.Api/wwwroot/uploads (dùng chung ảnh với bản C#).
		uploadDir = filepath.Join("..", "backend", "KernelStore.Api", "wwwroot", "uploads")
	}
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		slog.Error("Cannot create upload dir", "dir", uploadDir, "err", err)
		os.Exit(1)
	}

	// ── Router ────────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	// Ảnh upload tại /uploads/ — khớp UseStaticFiles (wwwroot/uploads) của C#.
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", addSecurityHeaders(http.FileServer(http.Dir(uploadDir)))))

	// Chat realtime: kết nối WebSocket giữ in-memory (chỉ chạy 1 instance).
	hub := ws.NewHub()
	mux.Handle("/ws/chat", ws.Handler(hub, tokenSvc))

	h := handlers.New(pool, cfg, tokenSvc, uploadDir, hub)
	h.Register(httpx.NewRouter(mux))

	// ── Middleware stack (ngoài → trong): CORS → Recovery → Logger → Authentication ──
	var handler http.Handler = mux
	handler = middleware.Authenticate(tokenSvc)(handler)
	handler = middleware.Logger(handler)
	handler = middleware.Recovery(handler)
	handler = middleware.CORS(cfg)(handler)

	// ── Server ───────────────────────────────────────────────────────────
	server := &http.Server{
		Addr:         cfg.ServerAddr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		slog.Info("Shutting down server...")
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutCancel()
		_ = server.Shutdown(shutCtx)
	}()

	slog.Info("Now listening on: http://localhost" + cfg.ServerAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("Server error", "err", err)
		os.Exit(1)
	}
}

// addSecurityHeaders thêm X-Content-Type-Options: nosniff + sandbox CSP cho static files
// (khớp C# StaticFileOptions.OnPrepareResponse).
func addSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
		next.ServeHTTP(w, r)
	})
}

// findMigrationsDir tìm thư mục migrations tương đối so với executable hoặc cwd.
func findMigrationsDir() (string, error) {
	// Thử relative paths phổ biến
	candidates := []string{
		"migrations",
		"backend-go/migrations",
		filepath.Join("..", "migrations"),
	}
	for _, c := range candidates {
		abs, _ := filepath.Abs(c)
		if info, err := os.Stat(abs); err == nil && info.IsDir() {
			return abs, nil
		}
	}
	// Fallback: dùng cwd + migrations
	abs, _ := filepath.Abs("migrations")
	return abs, nil
}
