package views

import (
	"context"
	"strings"

	"github.com/KernelStore/frontend/internal/client"
)

// Session là thông tin của request hiện tại mà layout cần (người dùng, đường dẫn, tab).
type Session struct {
	Token string
	User  *client.UserInfo // nil = khách
	Path  string
	Tab   string // query ?tab= (để tô sáng menu seller)
}

type sessionKey struct{}

func WithSession(ctx context.Context, s *Session) context.Context {
	return context.WithValue(ctx, sessionKey{}, s)
}

func SessionFrom(ctx context.Context) *Session {
	if s, ok := ctx.Value(sessionKey{}).(*Session); ok {
		return s
	}
	return &Session{}
}

func currentUser(ctx context.Context) *client.UserInfo { return SessionFrom(ctx).User }

func userRole(ctx context.Context) string {
	if u := currentUser(ctx); u != nil {
		return u.Role
	}
	return ""
}

func userID(ctx context.Context) string {
	if u := currentUser(ctx); u != nil {
		return u.ID
	}
	return ""
}

// navItem là một mục menu: khoá i18n + href.
type navItem struct{ label, href string }

type navGroup struct {
	label string
	items []navItem
}

// menuGroups: nhóm chức năng theo vai trò (như nav.rs).
func menuGroups(role string) []navGroup {
	groups := []navGroup{{"nav.group.buyer", []navItem{
		{"nav.browse", "/products"},
		{"nav.cart", "/cart"},
		{"nav.orders", "/orders"},
		{"nav.warranty", "/warranty"},
		{"nav.chat", "/chat"},
	}}}
	switch role {
	case "Seller":
		groups = append(groups, navGroup{"nav.group.seller", []navItem{
			{"nav.dashboard", "/seller?tab=dashboard"},
			{"nav.products", "/seller?tab=products"},
			{"nav.categories", "/seller?tab=categories"},
			{"nav.sales", "/seller?tab=sales"},
			{"nav.warranty_manage", "/warranty/manage"},
			{"nav.settings", "/seller?tab=settings"},
		}})
	case "Admin":
		groups = append(groups, navGroup{"nav.group.admin", []navItem{
			{"nav.panel", "/admin"},
			{"nav.warranty_manage", "/warranty/manage"},
		}})
	default: // Customer: chưa có shop
		groups = append(groups, navGroup{"nav.group.seller", []navItem{{"nav.become_seller", "/seller"}}})
	}
	return groups
}

// isActive: href có phải trang đang mở (với link /seller?tab= thì so cả tab).
func isActive(href, path, tab string) bool {
	if base, want, ok := strings.Cut(href, "?tab="); ok {
		return path == base && (tab == want || (tab == "" && want == "dashboard"))
	}
	return path == href || strings.HasPrefix(path, href+"/")
}

func menuClass(active bool) string {
	if active {
		return "term-menu-item term-active px-2 py-0.5 shrink-0"
	}
	return "term-menu-item px-2 py-0.5 shrink-0"
}
