package middleware

import (
	"bufio"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// Logger ghi log mỗi request (method, path, status, duration) bằng slog.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		slog.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"duration", time.Since(start).String(),
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// Unwrap cho http.ResponseController (Flush, SetDeadline...).
func (rw *responseWriter) Unwrap() http.ResponseWriter { return rw.ResponseWriter }

// Hijack cần cho WebSocket (/ws/chat) đi qua được middleware này.
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("response writer không hỗ trợ hijack")
	}
	rw.status = http.StatusSwitchingProtocols
	return hj.Hijack()
}
