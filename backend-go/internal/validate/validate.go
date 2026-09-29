// Package validate mô phỏng DataAnnotations của .NET ([Required], [StringLength],
// [MinLength], [Range], [RegularExpression], [EmailAddress]) để lỗi validation
// trả về giống InvalidModelStateResponseFactory: danh sách "Field: message".
package validate

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf16"

	"github.com/KernelStore/backend-go/internal/money"
)

// Errors gom lỗi theo thứ tự thêm vào.
type Errors struct {
	list []string
}

// Add thêm một lỗi dạng "Field: message".
func (e *Errors) Add(field, msg string) {
	e.list = append(e.list, field+": "+msg)
}

func (e *Errors) Empty() bool    { return len(e.list) == 0 }
func (e *Errors) List() []string { return e.list }

// Required — [Required]: chuỗi rỗng hoặc chỉ có khoảng trắng là lỗi.
func (e *Errors) Required(field, v, msg string) {
	if strings.TrimFunc(v, unicode.IsSpace) == "" {
		e.Add(field, or(msg, fmt.Sprintf("The %s field is required.", field)))
	}
}

// StringLength — [StringLength(max, MinimumLength = min)]. .NET chỉ bỏ qua null,
// còn "" vẫn bị kiểm min; Go không phân biệt được null với "" nên luôn kiểm.
func (e *Errors) StringLength(field, v string, min, max int, msg string) {
	n := Len(v)
	if n >= min && n <= max {
		return
	}
	if msg == "" {
		if min > 0 {
			msg = fmt.Sprintf("The field %s must be a string with a minimum length of %d and a maximum length of %d.", field, min, max)
		} else {
			msg = fmt.Sprintf("The field %s must be a string with a maximum length of %d.", field, max)
		}
	}
	e.Add(field, msg)
}

// MinLength — [MinLength(min)].
func (e *Errors) MinLength(field, v string, min int) {
	if Len(v) < min {
		e.Add(field, fmt.Sprintf("The field %s must be a string or array type with a minimum length of '%d'.", field, min))
	}
}

// Email — [EmailAddress]: đúng một '@', không ở đầu/cuối (thuật toán của .NET).
// Chuỗi rỗng coi là null → hợp lệ ([Required] lo phần bắt buộc).
func (e *Errors) Email(field, v string) {
	if v == "" || IsEmail(v) {
		return
	}
	e.Add(field, fmt.Sprintf("The %s field is not a valid e-mail address.", field))
}

// Regex — [RegularExpression]: phải khớp toàn chuỗi; chuỗi rỗng bỏ qua.
func (e *Errors) Regex(field, v string, re *regexp.Regexp, msg string) {
	if v == "" {
		return
	}
	if loc := re.FindStringIndex(v); loc == nil || loc[0] != 0 || loc[1] != len(v) {
		e.Add(field, or(msg, fmt.Sprintf("The field %s must match the regular expression '%s'.", field, re.String())))
	}
}

// RangeInt — [Range(min, max)] cho int.
func (e *Errors) RangeInt(field string, v, min, max int, msg string) {
	if v < min || v > max {
		e.Add(field, or(msg, fmt.Sprintf("The field %s must be between %d and %d.", field, min, max)))
	}
}

// RangeMoney — [Range(min, max)] cho decimal; nil (decimal?) là hợp lệ.
func (e *Errors) RangeMoney(field string, v *money.Money, min, max int64, msg string) {
	if v == nil {
		return
	}
	if v.Cmp(money.FromInt(min)) < 0 || v.Cmp(money.FromInt(max)) > 0 {
		e.Add(field, or(msg, fmt.Sprintf("The field %s must be between %d and %d.", field, min, max)))
	}
}

// Len đếm độ dài theo đơn vị UTF-16 như string.Length của C#.
func Len(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}

// IsEmail là thuật toán của EmailAddressAttribute (.NET Core).
func IsEmail(s string) bool {
	if strings.ContainsAny(s, "\r\n") {
		return false
	}
	i := strings.IndexByte(s, '@')
	return i > 0 && i != len(s)-1 && i == strings.LastIndexByte(s, '@')
}

// Slug là regex dùng chung cho mọi trường Slug của C#.
var Slug = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

const SlugMessage = "Slug chỉ gồm chữ thường, số và dấu gạch ngang"

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
