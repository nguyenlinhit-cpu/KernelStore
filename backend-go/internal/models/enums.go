package models

import (
	"strconv"
	"strings"
)

// enumNames[i] là tên của giá trị i (các enum trong dự án đều liên tiếp từ 0).
type enumNames []string

// name trả tên như C# Enum.ToString(): giá trị không định nghĩa → in số.
func (n enumNames) name(v int) string {
	if v >= 0 && v < len(n) {
		return n[v]
	}
	return strconv.Itoa(v)
}

// parse mô phỏng Enum.TryParse(value, ignoreCase: true):
//   - bỏ khoảng trắng hai đầu, không phân biệt hoa thường;
//   - nhận chuỗi số ("3", "-1", "99") kể cả giá trị không định nghĩa;
//   - nhận danh sách "A, B" → OR các giá trị (hành vi flags của .NET).
func (n enumNames) parse(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if c := s[0]; c == '-' || c == '+' || (c >= '0' && c <= '9') {
		v, err := strconv.ParseInt(s, 10, 32)
		return int(v), err == nil
	}
	result := 0
	for part := range strings.SplitSeq(s, ",") {
		part = strings.TrimSpace(part)
		idx := -1
		for i, name := range n {
			if strings.EqualFold(name, part) {
				idx = i
				break
			}
		}
		if idx < 0 {
			return 0, false
		}
		result |= idx
	}
	return result, true
}

func (n enumNames) defined(v int) bool { return v >= 0 && v < len(n) }

// ─── UserRole ────────────────────────────────────────────────────────────────

type UserRole int

const (
	UserRoleCustomer UserRole = iota
	UserRoleSeller
	UserRoleAdmin
)

var userRoleNames = enumNames{"Customer", "Seller", "Admin"}

func (r UserRole) String() string  { return userRoleNames.name(int(r)) }
func (r UserRole) IsDefined() bool { return userRoleNames.defined(int(r)) }

func ParseUserRole(s string) (UserRole, bool) {
	v, ok := userRoleNames.parse(s)
	return UserRole(v), ok
}

// ─── ShopStatus ──────────────────────────────────────────────────────────────

type ShopStatus int

const (
	ShopStatusPending ShopStatus = iota
	ShopStatusApproved
	ShopStatusRejected
	// Ban tạm thời (vi phạm) — có thể gỡ ban để hoạt động lại.
	ShopStatusBanned
	// Ban vĩnh viễn (xóa shop) — ẩn shop & sản phẩm, không thể khôi phục.
	ShopStatusDeleted
)

var shopStatusNames = enumNames{"Pending", "Approved", "Rejected", "Banned", "Deleted"}

func (s ShopStatus) String() string  { return shopStatusNames.name(int(s)) }
func (s ShopStatus) IsDefined() bool { return shopStatusNames.defined(int(s)) }

func ParseShopStatus(s string) (ShopStatus, bool) {
	v, ok := shopStatusNames.parse(s)
	return ShopStatus(v), ok
}

// ─── OrderStatus ─────────────────────────────────────────────────────────────

type OrderStatus int

const (
	OrderStatusPending OrderStatus = iota
	OrderStatusConfirmed
	OrderStatusProcessing
	OrderStatusShipped
	OrderStatusDelivered
	OrderStatusCancelled
	// Khách yêu cầu trả hàng sau khi đã nhận (Delivered → ReturnRequested).
	OrderStatusReturnRequested
	// Seller/Admin duyệt trả hàng → hoàn kho (ReturnRequested → Returned).
	OrderStatusReturned
)

var orderStatusNames = enumNames{
	"Pending", "Confirmed", "Processing", "Shipped",
	"Delivered", "Cancelled", "ReturnRequested", "Returned",
}

// AllOrderStatuses theo thứ tự enum (dùng cho thống kê ordersByStatus).
var AllOrderStatuses = []OrderStatus{
	OrderStatusPending, OrderStatusConfirmed, OrderStatusProcessing, OrderStatusShipped,
	OrderStatusDelivered, OrderStatusCancelled, OrderStatusReturnRequested, OrderStatusReturned,
}

func (o OrderStatus) String() string  { return orderStatusNames.name(int(o)) }
func (o OrderStatus) IsDefined() bool { return orderStatusNames.defined(int(o)) }

func ParseOrderStatus(s string) (OrderStatus, bool) {
	v, ok := orderStatusNames.parse(s)
	return OrderStatus(v), ok
}

// ─── WarrantyStatus ──────────────────────────────────────────────────────────

type WarrantyStatus int

const (
	// Khách vừa gửi yêu cầu, chờ shop/admin xử lý.
	WarrantyStatusPending WarrantyStatus = iota
	// Shop/Admin chấp nhận bảo hành (kèm hình thức xử lý).
	WarrantyStatusApproved
	// Shop/Admin từ chối.
	WarrantyStatusRejected
	// Đang tiến hành sửa/đổi/hoàn tiền.
	WarrantyStatusProcessing
	// Đã hoàn tất bảo hành cho khách.
	WarrantyStatusCompleted
	// Khách tự hủy khi còn Pending.
	WarrantyStatusCancelled
)

var warrantyStatusNames = enumNames{"Pending", "Approved", "Rejected", "Processing", "Completed", "Cancelled"}

func (w WarrantyStatus) String() string  { return warrantyStatusNames.name(int(w)) }
func (w WarrantyStatus) IsDefined() bool { return warrantyStatusNames.defined(int(w)) }

func ParseWarrantyStatus(s string) (WarrantyStatus, bool) {
	v, ok := warrantyStatusNames.parse(s)
	return WarrantyStatus(v), ok
}

// ─── WarrantyResolution ──────────────────────────────────────────────────────

type WarrantyResolution int

const (
	WarrantyResolutionNone WarrantyResolution = iota
	WarrantyResolutionRepair
	WarrantyResolutionReplace
	WarrantyResolutionRefund
)

var warrantyResolutionNames = enumNames{"None", "Repair", "Replace", "Refund"}

func (w WarrantyResolution) String() string  { return warrantyResolutionNames.name(int(w)) }
func (w WarrantyResolution) IsDefined() bool { return warrantyResolutionNames.defined(int(w)) }

func ParseWarrantyResolution(s string) (WarrantyResolution, bool) {
	v, ok := warrantyResolutionNames.parse(s)
	return WarrantyResolution(v), ok
}
