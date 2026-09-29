// Package client gọi REST API của backend (:5000) — thay api.rs của bản Rust.
// Khác bản Rust: gọi từ server frontend (SSR) chứ không từ trình duyệt.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ErrKind tương ứng enum ApiError của api.rs.
type ErrKind int

const (
	ErrServer ErrKind = iota
	ErrNetwork
	ErrUnauthorized
	ErrNotFound
)

type Error struct {
	Kind ErrKind
	Msg  string
}

// Error in như Display của ApiError bên Rust (UI hiện nguyên chuỗi này).
func (e *Error) Error() string {
	switch e.Kind {
	case ErrNetwork:
		return "network error: " + e.Msg
	case ErrUnauthorized:
		return "unauthorized"
	case ErrNotFound:
		return "not found"
	default:
		return e.Msg
	}
}

// IsNotFound / IsUnauthorized tiện cho handler.
func IsNotFound(err error) bool     { e, ok := err.(*Error); return ok && e.Kind == ErrNotFound }
func IsUnauthorized(err error) bool { e, ok := err.(*Error); return ok && e.Kind == ErrUnauthorized }

type Client struct {
	Base string // ví dụ http://localhost:5000/api
	HTTP *http.Client
}

func New(base string) *Client {
	return &Client{Base: base, HTTP: &http.Client{Timeout: 30 * time.Second}}
}

type envelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Message string          `json:"message"`
	Errors  []string        `json:"errors"`
}

// req mô tả một lời gọi API.
type req struct {
	method, path, token string
	body                any    // nil = không gửi body
	raw                 []byte // body đã dựng sẵn (multipart)
	contentType         string
	map404              bool // 404 → ErrNotFound (chỉ những hàm api.rs có kiểm 404)
}

// do gửi request và giải envelope {success,data,message,errors}.
// out = nil → bỏ qua data. Trả message của envelope (một số hàm hiển thị nó).
func (c *Client) do(ctx context.Context, r req, out any) (string, error) {
	var body io.Reader
	ct := r.contentType
	switch {
	case r.raw != nil:
		body = bytes.NewReader(r.raw)
	case r.body != nil:
		b, err := json.Marshal(r.body)
		if err != nil {
			return "", &Error{Kind: ErrNetwork, Msg: err.Error()}
		}
		body, ct = bytes.NewReader(b), "application/json"
	}
	httpReq, err := http.NewRequestWithContext(ctx, r.method, c.Base+r.path, body)
	if err != nil {
		return "", &Error{Kind: ErrNetwork, Msg: err.Error()}
	}
	if ct != "" {
		httpReq.Header.Set("Content-Type", ct)
	}
	if r.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+r.token)
	}
	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return "", &Error{Kind: ErrNetwork, Msg: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return "", &Error{Kind: ErrUnauthorized}
	}
	if r.map404 && resp.StatusCode == http.StatusNotFound {
		return "", &Error{Kind: ErrNotFound}
	}
	var env envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return "", &Error{Kind: ErrNetwork, Msg: fmt.Sprintf("HTTP %d: %v", resp.StatusCode, err)}
	}
	if !env.Success {
		msg := env.Message
		if len(env.Errors) > 0 {
			msg = env.Errors[0]
		}
		return "", &Error{Kind: ErrServer, Msg: msg}
	}
	if out != nil && len(env.Data) > 0 && string(env.Data) != "null" {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return "", &Error{Kind: ErrNetwork, Msg: err.Error()}
		}
	}
	return env.Message, nil
}

// noData = lỗi "no data" của api.rs khi data null mà hàm cần giá trị.
var noData = &Error{Kind: ErrServer, Msg: "no data"}
