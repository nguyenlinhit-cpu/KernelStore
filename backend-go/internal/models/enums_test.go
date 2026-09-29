package models

import "testing"

// Hành vi phải khớp Enum.TryParse(ignoreCase: true) / Enum.ToString() của .NET.
func TestParseOrderStatus(t *testing.T) {
	cases := []struct {
		in   string
		want OrderStatus
		ok   bool
	}{
		{"Shipped", OrderStatusShipped, true},
		{"shipped", OrderStatusShipped, true},
		{"  Delivered ", OrderStatusDelivered, true},
		{"3", OrderStatusShipped, true},
		{"99", OrderStatus(99), true}, // parse được nhưng không defined
		{"Confirmed, Processing", OrderStatus(1 | 2), true},
		{"Shipping", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		got, ok := ParseOrderStatus(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("ParseOrderStatus(%q) = %v,%v; want %v,%v", c.in, got, ok, c.want, c.ok)
		}
	}
	if OrderStatus(99).IsDefined() || !OrderStatusReturned.IsDefined() {
		t.Error("IsDefined sai")
	}
	if s := OrderStatus(99).String(); s != "99" {
		t.Errorf("ToString giá trị lạ = %q, want \"99\"", s)
	}
}
