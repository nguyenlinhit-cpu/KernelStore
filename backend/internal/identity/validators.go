package identity

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"github.com/KernelStore/backend/internal/validate"
)

// Tuỳ chọn Identity trong Program.cs của bản C#.
const requiredPasswordLength = 6

// AllowedUserNameCharacters mặc định của IdentityOptions.User.
const allowedUserNameCharacters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._@+"

// ValidatePassword mô phỏng PasswordValidator với cấu hình của dự án
// (RequireDigit, RequireLowercase, RequireUppercase, RequireNonAlphanumeric, RequiredLength = 6).
// Trả danh sách mô tả lỗi tiếng Anh y như IdentityErrorDescriber.
func ValidatePassword(password string) []string {
	var errs []string
	if strings.TrimFunc(password, unicode.IsSpace) == "" || validate.Len(password) < requiredPasswordLength {
		errs = append(errs, fmt.Sprintf("Passwords must be at least %d characters.", requiredPasswordLength))
	}
	var hasNonAlnum, hasDigit, hasLower, hasUpper bool
	for _, c := range password {
		switch {
		case c >= '0' && c <= '9':
			hasDigit = true
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		}
		if !unicode.IsLetter(c) && !unicode.IsDigit(c) {
			hasNonAlnum = true
		}
	}
	if !hasNonAlnum {
		errs = append(errs, "Passwords must have at least one non alphanumeric character.")
	}
	if !hasDigit {
		errs = append(errs, "Passwords must have at least one digit ('0'-'9').")
	}
	if !hasLower {
		errs = append(errs, "Passwords must have at least one lowercase ('a'-'z').")
	}
	if !hasUpper {
		errs = append(errs, "Passwords must have at least one uppercase ('A'-'Z').")
	}
	return errs
}

// ValidateUserNameFormat là phần kiểm ký tự của UserValidator (phần trùng lặp cần DB).
func ValidateUserNameFormat(userName string) (string, bool) {
	if strings.TrimFunc(userName, unicode.IsSpace) == "" ||
		strings.ContainsFunc(userName, func(r rune) bool { return !strings.ContainsRune(allowedUserNameCharacters, r) }) {
		return InvalidUserName(userName), false
	}
	return "", true
}

// ValidateEmailFormat là phần kiểm định dạng email của UserValidator.
func ValidateEmailFormat(email string) (string, bool) {
	if strings.TrimFunc(email, unicode.IsSpace) == "" || !validate.IsEmail(email) {
		return fmt.Sprintf("Email '%s' is invalid.", email), false
	}
	return "", true
}

func InvalidUserName(userName string) string {
	return fmt.Sprintf("Username '%s' is invalid, can only contain letters or digits.", userName)
}

func DuplicateUserName(userName string) string {
	return fmt.Sprintf("Username '%s' is already taken.", userName)
}

func DuplicateEmail(email string) string {
	return fmt.Sprintf("Email '%s' is already taken.", email)
}

// Normalize tương đương UpperInvariantLookupNormalizer (NormalizedEmail/UserName/Role).
func Normalize(s string) string { return strings.ToUpper(s) }

// NewSecurityStamp: 20 byte ngẫu nhiên mã hoá Base32 (32 ký tự) như Identity.
func NewSecurityStamp() string {
	b := make([]byte, 20)
	rand.Read(b)
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
}

// NewConcurrencyStamp: Guid.NewGuid().ToString().
func NewConcurrencyStamp() string { return uuid.NewString() }
