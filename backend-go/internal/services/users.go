package services

import (
	"context"
	"time"

	"github.com/KernelStore/backend-go/internal/identity"
	"github.com/KernelStore/backend-go/internal/models"
	"github.com/KernelStore/backend-go/internal/repository"
)

// Now trả giờ UTC làm tròn tới micro giây — đúng độ chính xác của timestamptz,
// để giá trị trả về ngay sau khi tạo khớp với giá trị đọc lại từ DB.
func Now() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }

// CreateUser mô phỏng UserManager.CreateAsync(user, password):
//  1. kiểm luật mật khẩu — lỗi thì trả ngay, chưa kiểm user;
//  2. băm mật khẩu, sinh SecurityStamp/ConcurrencyStamp, chuẩn hoá tên + email;
//  3. UserValidator: username (ký tự hợp lệ, không trùng) rồi email (định dạng, không trùng);
//  4. ghi DB.
//
// validationErrs khác rỗng nghĩa là "IdentityResult.Failed" (→ 400); err là lỗi hệ thống.
func CreateUser(ctx context.Context, db repository.DBTX, u *models.ApplicationUser, password string) (validationErrs []string, err error) {
	if errs := identity.ValidatePassword(password); len(errs) > 0 {
		return errs, nil
	}

	hash := identity.HashPassword(password)
	stamp := identity.NewSecurityStamp()
	concurrency := identity.NewConcurrencyStamp()
	u.PasswordHash, u.SecurityStamp, u.ConcurrencyStamp = &hash, &stamp, &concurrency

	userName, email := deref(u.UserName), deref(u.Email)
	normUser, normEmail := identity.Normalize(userName), identity.Normalize(email)
	u.NormalizedUserName, u.NormalizedEmail = &normUser, &normEmail
	u.LockoutEnabled = true // mặc định của IdentityOptions.Lockout.AllowedForNewUsers

	var errs []string
	if msg, ok := identity.ValidateUserNameFormat(userName); !ok {
		errs = append(errs, msg)
	} else if owner, err := repository.FindUserByName(ctx, db, normUser); err != nil {
		return nil, err
	} else if owner != nil {
		errs = append(errs, identity.DuplicateUserName(userName))
	}
	if msg, ok := identity.ValidateEmailFormat(email); !ok {
		errs = append(errs, msg)
	} else if owner, err := repository.FindUserByEmail(ctx, db, normEmail); err != nil {
		return nil, err
	} else if owner != nil {
		errs = append(errs, identity.DuplicateEmail(email))
	}
	if len(errs) > 0 {
		return errs, nil
	}

	return nil, repository.InsertUser(ctx, db, u)
}

// CheckPassword mô phỏng UserManager.CheckPasswordAsync: nếu hash dùng định dạng
// cũ (V2 hoặc ít vòng lặp) thì băm lại và cập nhật, giống Identity.
func CheckPassword(ctx context.Context, db repository.DBTX, u *models.ApplicationUser, password string) (bool, error) {
	if u.PasswordHash == nil {
		return false, nil
	}
	switch identity.VerifyHashedPassword(*u.PasswordHash, password) {
	case identity.VerifySuccess:
		return true, nil
	case identity.VerifySuccessRehashNeeded:
		err := repository.UpdatePasswordHash(ctx, db, u.ID, identity.HashPassword(password),
			identity.NewSecurityStamp(), identity.NewConcurrencyStamp())
		return err == nil, err
	default:
		return false, nil
	}
}
