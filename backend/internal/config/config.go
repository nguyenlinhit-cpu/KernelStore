// Package config cung cấp cấu hình ứng dụng, đọc từ biến môi trường hoặc dùng giá trị mặc định
// khớp với appsettings.json của bản C# gốc.
package config

import (
	"os"
	"strconv"
	"strings"
)

// Config chứa toàn bộ cấu hình ứng dụng.
type Config struct {
	// Server
	ServerAddr string // ":5000"

	// Database (PostgreSQL)
	DBHost     string
	DBPort     int
	DBName     string
	DBUser     string
	DBPassword string
	DBSSLMode  string

	// JWT
	JWTIssuer                 string
	JWTAudience               string
	JWTSecret                 string
	JWTAccessTokenExpiryMins  int
	JWTRefreshTokenExpiryDays int

	// CORS
	CORSAllowedOrigins []string

	// Upload
	UploadDir string // thư mục chứa ảnh upload; rỗng = backend/uploads
}

// Load đọc cấu hình từ biến môi trường. Giá trị mặc định khớp appsettings.json.
func Load() *Config {
	return &Config{
		ServerAddr: envOr("SERVER_ADDR", ":5000"),

		DBHost:     envOr("DB_HOST", "localhost"),
		DBPort:     envIntOr("DB_PORT", 5433),
		DBName:     envOr("DB_NAME", "kernelstore"),
		DBUser:     envOr("DB_USER", "admin"),
		DBPassword: envOr("DB_PASSWORD", "admin123"),
		DBSSLMode:  envOr("DB_SSLMODE", "disable"),

		JWTIssuer:                 envOr("JWT_ISSUER", "KernelStore.Api"),
		JWTAudience:               envOr("JWT_AUDIENCE", "KernelStore.Client"),
		JWTSecret:                 envOr("JWT_SECRET", "KernelStore_Dev_Super_Secret_Key_2026_At_Least_32_Chars_Long!"),
		JWTAccessTokenExpiryMins:  envIntOr("JWT_ACCESS_EXPIRY_MINS", 30),
		JWTRefreshTokenExpiryDays: envIntOr("JWT_REFRESH_EXPIRY_DAYS", 7),

		CORSAllowedOrigins: envListOr("CORS_ORIGINS", []string{"http://localhost:8080", "http://127.0.0.1:8080"}),

		UploadDir: envOr("UPLOAD_DIR", ""),
	}
}

// DatabaseURL trả connection string cho pgx.
func (c *Config) DatabaseURL() string {
	return "postgres://" + c.DBUser + ":" + c.DBPassword +
		"@" + c.DBHost + ":" + strconv.Itoa(c.DBPort) +
		"/" + c.DBName + "?sslmode=" + c.DBSSLMode
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOr(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envListOr(key string, fallback []string) []string {
	if v := os.Getenv(key); v != "" {
		parts := strings.Split(v, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			if t := strings.TrimSpace(p); t != "" {
				result = append(result, t)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return fallback
}
