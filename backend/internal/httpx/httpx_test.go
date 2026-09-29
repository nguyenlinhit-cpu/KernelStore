package httpx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/KernelStore/backend/internal/config"
	"github.com/KernelStore/backend/internal/dto"
	"github.com/KernelStore/backend/internal/middleware"
	"github.com/KernelStore/backend/internal/models"
	"github.com/KernelStore/backend/internal/services"
)

func newServer(t *testing.T) (http.Handler, *services.TokenService) {
	t.Helper()
	cfg := config.Load()
	tokens := services.NewTokenService(cfg)
	mux := http.NewServeMux()
	rt := NewRouter(mux)
	ok := func(w http.ResponseWriter, r *http.Request) { dto.OK(w, r.PathValue("id")) }
	rt.Handle("GET /api/open", Anonymous, ok)
	rt.Handle("GET /api/me", Authenticated, ok)
	rt.Handle("POST /api/admin/shops/{id}/approve", Roles("Admin"), ok)
	rt.Handle("POST /api/cart", Authenticated, func(w http.ResponseWriter, r *http.Request) {
		var req dto.AddToCartRequest
		if !BindJSON(w, r, &req) {
			return
		}
		dto.OK(w, req.Quantity)
	})
	return middleware.Authenticate(tokens)(mux), tokens
}

func token(t *testing.T, tokens *services.TokenService, role models.UserRole, roles ...string) string {
	t.Helper()
	name := "u"
	tok, _, err := tokens.CreateAccessToken(&models.ApplicationUser{ID: services.NewUUID(), UserName: &name, Role: role}, roles)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func do(h http.Handler, method, path, tok, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestPolicies(t *testing.T) {
	h, tokens := newServer(t)
	customer := token(t, tokens, models.UserRoleCustomer, "Customer")
	admin := token(t, tokens, models.UserRoleAdmin, "Admin")
	// Cột Role = Seller nhưng chưa có role Identity → vẫn là Seller (giống C#).
	sellerByColumn := token(t, tokens, models.UserRoleSeller, "Customer")
	id := "019ff6c1-7d21-7479-9325-962a9cdf6490"

	cases := []struct {
		name, method, path, tok string
		want                    int
	}{
		{"anonymous ok", "GET", "/api/open", "", 200},
		{"no token → 401", "GET", "/api/me", "", 401},
		{"garbage token → 401", "GET", "/api/me", "not.a.jwt", 401},
		{"token ok", "GET", "/api/me", customer, 200},
		{"wrong role → 403", "POST", "/api/admin/shops/" + id + "/approve", customer, 403},
		{"admin → 200", "POST", "/api/admin/shops/" + id + "/approve", admin, 200},
		{"guid sai → 404 trước cả auth", "POST", "/api/admin/shops/not-a-guid/approve", "", 404},
		{"guid dạng N", "POST", "/api/admin/shops/019ff6c17d2174799325962a9cdf6490/approve", admin, 200},
		{"sai method → 405", "GET", "/api/admin/shops/" + id + "/approve", admin, 405},
		{"không có route → 404", "GET", "/api/nothing", "", 404},
	}
	for _, c := range cases {
		if got := do(h, c.method, c.path, c.tok, "").Code; got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
	claims, err := tokens.ParseAccessToken(sellerByColumn)
	if err != nil || !claims.HasRole("Seller") || !claims.HasRole("Customer") {
		t.Error("role từ cột AspNetUsers.Role phải có hiệu lực cùng role Identity")
	}
}

func TestBindJSON(t *testing.T) {
	h, tokens := newServer(t)
	tok := token(t, tokens, models.UserRoleCustomer, "Customer")
	cases := []struct {
		name, body string
		want       int
		data       string
	}{
		{"mặc định Quantity = 1", `{"productId":"019ff6c1-7d21-7479-9325-962a9cdf6490"}`, 200, "1"},
		{"tên field không phân biệt hoa thường", `{"Quantity":5}`, 200, "5"},
		{"vi phạm Range", `{"quantity":0}`, 400, ""},
		{"body rỗng", ``, 400, ""},
		{"null", `null`, 400, ""},
		{"JSON hỏng", `{"quantity":`, 400, ""},
		{"sai kiểu", `{"quantity":"x"}`, 400, ""},
		{"guid sai", `{"productId":"abc"}`, 400, ""},
		{"dữ liệu thừa", `{"quantity":2} {}`, 400, ""},
	}
	for _, c := range cases {
		req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/cart", strings.NewReader(c.body))
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != c.want {
			t.Errorf("%s: got %d, want %d (%s)", c.name, rec.Code, c.want, rec.Body)
			continue
		}
		var resp struct {
			Success bool            `json:"success"`
			Data    json.RawMessage `json:"data"`
			Errors  []string        `json:"errors"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if c.want == 200 && string(resp.Data) != c.data {
			t.Errorf("%s: data = %s, want %s", c.name, resp.Data, c.data)
		}
		if c.want == 400 && (resp.Success || len(resp.Errors) == 0) {
			t.Errorf("%s: phải có success=false và errors: %s", c.name, rec.Body)
		}
	}
	// Content-Type không phải JSON → 415.
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/cart", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("text/plain: got %d, want 415", rec.Code)
	}
}
