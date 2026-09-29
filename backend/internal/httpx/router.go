package httpx

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/KernelStore/backend/internal/middleware"
)

// Policy mô tả quyền của một endpoint, tương đương attribute trên action C#.
type Policy struct {
	auth  bool
	roles []string
}

var (
	// Anonymous = [AllowAnonymous] / không có [Authorize].
	Anonymous = Policy{}
	// Authenticated = [Authorize].
	Authenticated = Policy{auth: true}
)

// Roles = [Authorize(Roles = "A,B")].
func Roles(roles ...string) Policy { return Policy{auth: true, roles: roles} }

// Router bọc http.ServeMux (Go 1.22+: pattern có method + wildcard).
type Router struct {
	mux *http.ServeMux
}

func NewRouter(mux *http.ServeMux) *Router { return &Router{mux: mux} }

var wildcardRe = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// Handle đăng ký endpoint với chính sách quyền.
//
// Wildcard tên "id" hoặc kết thúc bằng "Id" được coi là route constraint {x:guid}
// của ASP.NET: giá trị không phải GUID → 404 ngay (route không khớp), trước cả
// bước kiểm tra xác thực — đúng thứ tự routing → authorization của ASP.NET.
func (rt *Router) Handle(pattern string, p Policy, h http.HandlerFunc) {
	var handler http.Handler = h
	switch {
	case len(p.roles) > 0:
		handler = middleware.RequireRoles(p.roles...)(handler)
	case p.auth:
		handler = middleware.RequireAuth(handler)
	}

	var guidParams []string
	for _, m := range wildcardRe.FindAllStringSubmatch(pattern, -1) {
		if name := m[1]; name == "id" || strings.HasSuffix(name, "Id") {
			guidParams = append(guidParams, name)
		}
	}
	if len(guidParams) > 0 {
		inner := handler
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, name := range guidParams {
				if _, ok := ParseGUID(r.PathValue(name)); !ok {
					http.NotFound(w, r)
					return
				}
			}
			inner.ServeHTTP(w, r)
		})
	}

	rt.mux.Handle(pattern, handler)
}

// PathGUID đọc wildcard GUID đã được Router kiểm tra trước.
func PathGUID(r *http.Request, name string) uuid.UUID {
	id, _ := ParseGUID(r.PathValue(name))
	return id
}

// ParseGUID chấp nhận các định dạng mà Guid.TryParse của .NET nhận:
// D (có gạch), N (32 hex), B ({...}), P ((...)).
func ParseGUID(s string) (uuid.UUID, bool) {
	if len(s) == 38 && s[0] == '(' && s[37] == ')' {
		s = s[1:37]
	}
	if strings.HasPrefix(strings.ToLower(s), "urn:") {
		return uuid.Nil, false // uuid.Parse nhận urn:uuid: nhưng .NET thì không
	}
	id, err := uuid.Parse(s)
	return id, err == nil
}
