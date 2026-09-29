// Package views chứa giao diện templ (chuyển từ các component Leptos của bản Rust)
// và các hàm định dạng dùng chung.
package views

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/KernelStore/frontend-go/internal/client"
	"github.com/KernelStore/frontend-go/internal/i18n"
)

// t dịch key theo ngôn ngữ trong context.
func t(ctx context.Context, key string) string { return i18n.Tc(ctx, key) }

// tf dịch key rồi thay "{}" đầu tiên bằng v (như .replacen("{}", v, 1) của Rust).
func tf(ctx context.Context, key, v string) string {
	return strings.Replace(t(ctx, key), "{}", v, 1)
}

// money = format!("${:.2}", v).
func money(v float64) string { return fmt.Sprintf("$%.2f", v) }

// debugFloat = format!("{:?}", f64) của Rust: luôn có phần thập phân ("100.0", "19.9").
func debugFloat(v float64) string {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return s
}

// plainFloat = f64.to_string() của Rust ("100", "19.9") — dùng để điền lại ô nhập.
func plainFloat(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

// shortDate lấy 10 ký tự đầu của timestamp ISO ("2026-08-09").
func shortDate(raw string) string { return firstRunes(raw, 10) }

// shortTime: "2026-08-09T08:51:43Z" → "08-09 08:51" (như chat.rs).
func shortTime(raw string) string {
	if len(raw) < 16 {
		return " "
	}
	return raw[5:10] + " " + raw[11:16]
}

func firstRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// stars: n sao đặc trên 5.
func stars(n int) string {
	n = min(max(n, 0), 5)
	return strings.Repeat("★", n) + strings.Repeat("☆", 5-n)
}

// roundStars = (avg.round() as i32): làm tròn nửa ra xa 0 như Rust.
func roundStars(avg float64) int { return int(math.Round(avg)) }

// bar: thanh ASCII 10 ký tự theo tỉ lệ, ví dụ "[####      ]".
func bar(ratio float64) string {
	filled := int(min(max(math.Round(ratio*10), 0), 10))
	return "[" + strings.Repeat("#", filled) + strings.Repeat(" ", 10-filled) + "]"
}

// meter: thanh ASCII rộng width theo value/max.
func meter(value, maxV, width int) string {
	filled := 0
	if maxV > 0 {
		filled = int(math.Round(float64(value) / float64(maxV) * float64(width)))
	}
	filled = min(max(filled, 0), width)
	return "[" + strings.Repeat("#", filled) + strings.Repeat(" ", width-filled) + "]"
}

// pct = (value/max*100).round().
func pct(value, maxV int) int {
	if maxV <= 0 {
		return 0
	}
	return int(math.Round(float64(value) / float64(maxV) * 100))
}

// padRight = format!("{:<10}", s).
func padRight(s string, w int) string {
	if n := utf8.RuneCountInString(s); n < w {
		return s + strings.Repeat(" ", w-n)
	}
	return s
}

// orderStatusClass: màu terminal cho trạng thái đơn.
func orderStatusClass(status string) string {
	switch status {
	case "Delivered":
		return "term-info"
	case "Cancelled", "Returned":
		return "term-error"
	case "Shipped", "Processing", "Confirmed", "ReturnRequested":
		return "term-warn"
	default:
		return "term-muted"
	}
}

// shopBadge: màu badge trạng thái shop.
func shopBadge(status string) string {
	switch status {
	case "Approved":
		return "term-info"
	case "Pending":
		return "term-warn"
	case "Rejected", "Banned", "Deleted":
		return "term-error"
	default:
		return "term-muted"
	}
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func orDefault(p *string, def string) string {
	if p == nil {
		return def
	}
	return *p
}

// effectivePrice = sale_price.unwrap_or(price).
func effectivePrice(p *client.Product) float64 {
	if p.SalePrice != nil {
		return *p.SalePrice
	}
	return p.Price
}

func itoa(n int) string { return strconv.Itoa(n) }

// FlatCategory là một dòng của cây category đã làm phẳng.
type FlatCategory struct {
	Node  *client.CategoryNode
	Depth int
}

// Flatten duyệt cây theo thứ tự trước (như flatten() của admin.rs / seller.rs).
func Flatten(nodes []client.CategoryNode) []FlatCategory {
	var out []FlatCategory
	var walk func([]*client.CategoryNode, int)
	walk = func(ns []*client.CategoryNode, d int) {
		for _, n := range ns {
			out = append(out, FlatCategory{n, d})
			walk(n.Children, d+1)
		}
	}
	roots := make([]*client.CategoryNode, len(nodes))
	for i := range nodes {
		roots[i] = &nodes[i]
	}
	walk(roots, 0)
	return out
}

// loadingStyle: độ rộng + số bước animation theo độ dài chữ (như loading.rs).
func loadingStyle(text string) string {
	n := max(utf8.RuneCountInString(text), 1)
	return fmt.Sprintf("--tw: %dch; --steps: %d", n, n)
}

// panicTrace dựng khối "Call Trace" giả của KernelPanic.
func panicTrace(ctx context.Context, code, detail string) string {
	if detail == "" {
		detail = t(ctx, "panic.default_detail")
	}
	return fmt.Sprintf("  [ 0.000000] segfault at 0x%s ip pc:store\n  [ 0.000001] %s\n  [ 0.000002] Call Trace:\n  [ 0.000003]   kernelstore_render+0x1f\n  [ 0.000004]   route_dispatch+0x0\n  [ 0.000005] ---[ end trace ]---", code, detail)
}

func orStr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// categoryIndent = padding-left: {0.5 + depth*0.75}rem (như CategoryItem).
func categoryIndent(depth int) string {
	return fmt.Sprintf("padding-left: %grem", 0.5+float64(depth)*0.75)
}

// reviewsWord: "review" / "reviews".
func reviewsWord(ctx context.Context, n int) string {
	if n == 1 {
		return t(ctx, "pd.reviews_word")
	}
	return t(ctx, "pd.reviews_word_plural")
}

// ratingDistribution đếm số review theo số sao (chỉ số 0 = 1★).
func ratingDistribution(reviews []client.Review) [5]int {
	var d [5]int
	for _, r := range reviews {
		d[min(max(r.Rating, 1), 5)-1]++
	}
	return d
}

// jsonAttr mã hoá map thành JSON cho thuộc tính hx-vals.
func jsonAttr(m map[string]string) string {
	b, _ := json.Marshal(m)
	return string(b)
}
