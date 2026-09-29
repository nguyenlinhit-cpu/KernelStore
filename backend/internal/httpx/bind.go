// Package httpx gom các tiện ích HTTP dùng chung cho handlers: đọc JSON body
// kèm validation (thay model binding của ASP.NET) và router có chính sách quyền.
package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/KernelStore/backend/internal/dto"
	"github.com/KernelStore/backend/internal/validate"
)

// maxBodyBytes = giới hạn mặc định của Kestrel (30MB).
const maxBodyBytes = 30 << 20

// Validatable là request có luật validation (tương đương DataAnnotations).
type Validatable interface {
	Validate(*validate.Errors)
}

// defaulter cho các request có giá trị mặc định khác zero value (ví dụ Quantity = 1).
type defaulter interface {
	SetDefaults()
}

// BindJSON đọc body JSON vào dst rồi validate, giống [FromBody] + [ApiController]:
//   - Content-Type không phải JSON → 415;
//   - body rỗng, `null`, JSON hỏng, sai kiểu → 400 "Dữ liệu không hợp lệ";
//   - vi phạm validation → 400 kèm danh sách lỗi.
//
// Trả false nếu đã ghi response lỗi; handler chỉ việc return.
func BindJSON(w http.ResponseWriter, r *http.Request, dst Validatable) bool {
	if !isJSON(r.Header.Get("Content-Type")) {
		dto.Fail(w, http.StatusUnsupportedMediaType, "Content-Type phải là application/json")
		return false
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		invalid(w, "request: không đọc được body")
		return false
	}
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		invalid(w, ": A non-empty request body is required.")
		return false
	}
	if bytes.Equal(body, []byte("null")) {
		invalid(w, "request: The request field is required.")
		return false
	}

	if d, ok := dst.(defaulter); ok {
		d.SetDefaults()
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	if err := dec.Decode(dst); err != nil {
		invalid(w, jsonErrorMessage(err))
		return false
	}
	// System.Text.Json từ chối dữ liệu thừa sau giá trị JSON đầu tiên.
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		invalid(w, "$: The JSON value could not be converted: trailing data.")
		return false
	}

	var errs validate.Errors
	dst.Validate(&errs)
	if !errs.Empty() {
		dto.BadRequest(w, "Dữ liệu không hợp lệ", errs.List()...)
		return false
	}
	return true
}

func invalid(w http.ResponseWriter, detail string) {
	dto.BadRequest(w, "Dữ liệu không hợp lệ", detail)
}

func isJSON(contentType string) bool {
	mt, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return mt == "application/json" || (strings.HasPrefix(mt, "application/") && strings.HasSuffix(mt, "+json"))
}

// jsonErrorMessage đổi lỗi decode thành dạng "$.field: ..." gần với System.Text.Json.
func jsonErrorMessage(err error) string {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return fmt.Sprintf("$.%s: The JSON value could not be converted to %s.", typeErr.Field, typeErr.Type)
	}
	return "$: " + err.Error()
}
