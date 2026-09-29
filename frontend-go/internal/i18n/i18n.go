// Package i18n: giao diện song ngữ EN/VI (chuyển từ frontend/src/i18n.rs).
// Ngôn ngữ đang chọn lưu ở cookie "ks_lang" (bản Rust lưu localStorage cùng tên key).
package i18n

import (
	"context"
	"net/http"
)

type Lang int

const (
	En Lang = iota // mặc định
	Vi
)

const CookieName = "ks_lang"

// Code dùng cho cookie và <html lang>.
func (l Lang) Code() string {
	if l == Vi {
		return "vi"
	}
	return "en"
}

// Label hiện trên nút đổi ngôn ngữ.
func (l Lang) Label() string {
	if l == Vi {
		return "VI"
	}
	return "EN"
}

// Toggle trả ngôn ngữ còn lại.
func (l Lang) Toggle() Lang {
	if l == Vi {
		return En
	}
	return Vi
}

// FromRequest đọc cookie; không có / không hợp lệ → EN.
func FromRequest(r *http.Request) Lang {
	if c, err := r.Cookie(CookieName); err == nil && c.Value == "vi" {
		return Vi
	}
	return En
}

type ctxKey struct{}

func WithLang(ctx context.Context, l Lang) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

func FromContext(ctx context.Context) Lang {
	l, _ := ctx.Value(ctxKey{}).(Lang)
	return l
}

// T dịch key theo ngôn ngữ; key thiếu hiện "??" cho dễ thấy (như bản Rust).
func T(l Lang, key string) string {
	if v, ok := table[key]; ok {
		return v[l]
	}
	return "??"
}

// Tc dịch theo ngôn ngữ trong context (tiện dùng trong templ).
func Tc(ctx context.Context, key string) string { return T(FromContext(ctx), key) }
