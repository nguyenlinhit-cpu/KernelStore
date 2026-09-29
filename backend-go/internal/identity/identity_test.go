package identity

import (
	"slices"
	"strings"
	"testing"
)

// Hash thật do ASP.NET Identity (.NET 10) tạo cho admin@ks.com / Admin@12345,
// lấy từ database.sql.
const aspNetAdminHash = "AQAAAAIAAYagAAAAEGGnZj9QJ7GJNEw0RgDhcGYiEXX2FjCBWn3OLjIqf5AH6tUtKV8jpRALoQzAxDl+tA=="

func TestVerifyAspNetHash(t *testing.T) {
	if got := VerifyHashedPassword(aspNetAdminHash, "Admin@12345"); got != VerifySuccess {
		t.Fatalf("mật khẩu đúng: got %v, want VerifySuccess", got)
	}
	if got := VerifyHashedPassword(aspNetAdminHash, "admin@12345"); got != VerifyFailed {
		t.Fatalf("mật khẩu sai: got %v, want VerifyFailed", got)
	}
}

func TestHashRoundTrip(t *testing.T) {
	h := HashPassword("Passw0rd!")
	if !strings.HasPrefix(h, "AQAAAAIAAYag") {
		t.Fatalf("hash phải cùng định dạng V3/SHA512/100000 với .NET: %s", h)
	}
	if VerifyHashedPassword(h, "Passw0rd!") != VerifySuccess {
		t.Fatal("verify hash vừa tạo thất bại")
	}
	if VerifyHashedPassword(h, "Passw0rd?") != VerifyFailed {
		t.Fatal("mật khẩu sai vẫn verify được")
	}
	if VerifyHashedPassword("not-base64!", "x") != VerifyFailed || VerifyHashedPassword("", "x") != VerifyFailed {
		t.Fatal("hash hỏng phải trả VerifyFailed")
	}
}

func TestValidatePassword(t *testing.T) {
	if errs := ValidatePassword("Passw0rd!"); len(errs) != 0 {
		t.Fatalf("mật khẩu hợp lệ bị từ chối: %v", errs)
	}
	errs := ValidatePassword("abc")
	want := []string{
		"Passwords must be at least 6 characters.",
		"Passwords must have at least one non alphanumeric character.",
		"Passwords must have at least one digit ('0'-'9').",
		"Passwords must have at least one uppercase ('A'-'Z').",
	}
	if !slices.Equal(errs, want) {
		t.Fatalf("got %q\nwant %q", errs, want)
	}
}

func TestUserNameAndStamp(t *testing.T) {
	if _, ok := ValidateUserNameFormat("user_1.a@b+c-d"); !ok {
		t.Fatal("username hợp lệ bị từ chối")
	}
	if _, ok := ValidateUserNameFormat("có dấu"); ok {
		t.Fatal("username có khoảng trắng/dấu phải bị từ chối")
	}
	if s := NewSecurityStamp(); len(s) != 32 || strings.ToUpper(s) != s {
		t.Fatalf("security stamp sai định dạng: %q", s)
	}
}
