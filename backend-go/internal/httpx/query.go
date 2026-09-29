package httpx

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/KernelStore/backend-go/internal/dto"
	"github.com/KernelStore/backend-go/internal/money"
	"github.com/KernelStore/backend-go/internal/validate"
)

// Query đọc tham số query string như [FromQuery] của ASP.NET:
// tên tham số không phân biệt hoa thường, lấy giá trị đầu tiên,
// giá trị sai kiểu → lỗi ModelState (400) chứ không âm thầm bỏ qua.
type Query struct {
	values url.Values
	errs   validate.Errors
}

func NewQuery(r *http.Request) *Query { return &Query{values: r.URL.Query()} }

// Get trả (giá trị, có mặt hay không).
func (q *Query) Get(name string) (string, bool) {
	for k, v := range q.values {
		if strings.EqualFold(k, name) && len(v) > 0 {
			return v[0], true
		}
	}
	return "", false
}

// String trả chuỗi hoặc "" nếu không có.
func (q *Query) String(name string) string {
	v, _ := q.Get(name)
	return v
}

// Int cho tham số int có mặc định (int page = 1).
func (q *Query) Int(name string, def int) int {
	v, ok := q.Get(name)
	if !ok {
		return def
	}
	n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 32)
	if err != nil {
		q.invalid(name, v)
		return def
	}
	return int(n)
}

// Money cho tham số decimal? (rỗng/không có → nil).
func (q *Query) Money(name string) *money.Money {
	v, ok := q.Get(name)
	if !ok || strings.TrimSpace(v) == "" {
		return nil
	}
	m, err := money.Parse(strings.TrimSpace(v))
	if err != nil {
		q.invalid(name, v)
		return nil
	}
	return &m
}

// Bool cho tham số bool? (rỗng/không có → nil).
func (q *Query) Bool(name string) *bool {
	v, ok := q.Get(name)
	if !ok || strings.TrimSpace(v) == "" {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true":
		b := true
		return &b
	case "false":
		b := false
		return &b
	}
	q.invalid(name, v)
	return nil
}

// GUID cho tham số Guid không nullable (không có → Guid.Empty).
func (q *Query) GUID(name string) uuid.UUID {
	v, ok := q.Get(name)
	if !ok || v == "" {
		return uuid.Nil
	}
	id, ok := ParseGUID(strings.TrimSpace(v))
	if !ok {
		q.invalid(name, v)
	}
	return id
}

func (q *Query) invalid(name, value string) {
	q.errs.Add(name, fmt.Sprintf("The value '%s' is not valid for %s.", value, name))
}

// Failed ghi 400 nếu có tham số sai kiểu; handler return khi true.
func (q *Query) Failed(w http.ResponseWriter) bool {
	if q.errs.Empty() {
		return false
	}
	dto.BadRequest(w, "Dữ liệu không hợp lệ", q.errs.List()...)
	return true
}
