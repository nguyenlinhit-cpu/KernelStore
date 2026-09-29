// cmd/web/main.go — frontend KernelStore (Go SSR + templ + HTMX), chạy ở :8080.
// Thay app Rust/Leptos CSR: server gọi REST API backend (:5000) rồi trả HTML.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/KernelStore/frontend-go/internal/client"
	"github.com/KernelStore/frontend-go/internal/web"
)

func main() {
	addr := envOr("WEB_ADDR", ":8080")
	apiBase := envOr("API_BASE", "http://localhost:5000/api")
	wsBase := envOr("WS_BASE", "ws://localhost:5000/ws/chat")
	staticDir := envOr("STATIC_DIR", "static")

	app := web.New(client.New(apiBase), staticDir, wsBase)
	server := &http.Server{
		Addr:              addr,
		Handler:           app.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()

	slog.Info("KernelStore frontend listening", "addr", "http://localhost"+addr, "api", apiBase)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
