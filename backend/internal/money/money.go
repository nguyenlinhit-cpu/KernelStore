// Package money cung cấp kiểu tiền tệ Money, tương đương `decimal` của C#.
//
// Khác biệt quan trọng so với float64: không sai số khi cộng/nhân, và giữ
// nguyên số chữ số thập phân (scale) như System.Text.Json khi ghi JSON:
// giá trị đọc từ cột numeric(18,2) ra "24.50", còn literal 0m ra "0".
package money

import (
	"bytes"
	"database/sql/driver"
	"fmt"
	"strconv"

	"github.com/shopspring/decimal"
)

// Money bọc decimal.Decimal; zero value = 0.
type Money struct {
	d decimal.Decimal
}

// Zero là 0 (scale 0), tương đương `0m` trong C#.
var Zero = Money{}

// FromInt tạo Money từ số nguyên.
func FromInt(v int64) Money { return Money{decimal.NewFromInt(v)} }

// Parse đọc chuỗi số thập phân, giữ nguyên scale ("24.50" → scale 2).
func Parse(s string) (Money, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return Money{}, err
	}
	return Money{d}, nil
}

// MustParse như Parse nhưng panic khi lỗi — chỉ dùng cho hằng số (seed data).
func MustParse(s string) Money {
	m, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return m
}

// Add, Sub, MulInt: scale kết quả theo quy tắc của C# decimal
// (cộng/trừ → scale lớn hơn, nhân → tổng scale).
func (m Money) Add(o Money) Money        { return Money{m.d.Add(o.d)} }
func (m Money) Sub(o Money) Money        { return Money{m.d.Sub(o.d)} }
func (m Money) MulInt(n int) Money       { return Money{m.d.Mul(decimal.NewFromInt(int64(n)))} }
func (m Money) Cmp(o Money) int          { return m.d.Cmp(o.d) }
func (m Money) Decimal() decimal.Decimal { return m.d }

// String in giữ nguyên scale: 24.50 → "24.50", 100 → "100".
func (m Money) String() string {
	if exp := m.d.Exponent(); exp < 0 {
		return m.d.StringFixed(-exp)
	}
	return m.d.String()
}

// MarshalJSON ghi số JSON (không có ngoặc kép), giữ scale.
func (m Money) MarshalJSON() ([]byte, error) { return []byte(m.String()), nil }

// UnmarshalJSON nhận số JSON; cũng nhận chuỗi số vì ASP.NET (JsonSerializerDefaults.Web)
// cho phép đọc số từ chuỗi.
func (m *Money) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '"' {
		s, err := strconv.Unquote(string(b))
		if err != nil {
			return err
		}
		b = []byte(s)
	}
	d, err := decimal.NewFromString(string(b))
	if err != nil {
		return fmt.Errorf("giá trị tiền không hợp lệ: %s", b)
	}
	m.d = d
	return nil
}

// Scan đọc cột numeric (pgx trả dạng text, giữ scale).
func (m *Money) Scan(src any) error { return m.d.Scan(src) }

// Value ghi xuống DB dưới dạng chuỗi thập phân chính xác.
func (m Money) Value() (driver.Value, error) { return m.String(), nil }
