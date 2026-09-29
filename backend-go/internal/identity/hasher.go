// Package identity thay thế các phần của ASP.NET Core Identity mà dự án dùng:
// băm mật khẩu (PasswordHasher), luật mật khẩu/username (PasswordValidator,
// UserValidator), chuẩn hoá tên (UpperInvariantLookupNormalizer), security stamp.
//
// Định dạng hash giữ nguyên như ASP.NET Identity nên tài khoản cũ đăng nhập được,
// và backend C# lẫn Go có thể chạy chung một database.
package identity

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"hash"
)

// Tham số mặc định của PasswordHasher V3 trong .NET 8+ (hash bắt đầu bằng "AQAAAAIAAYag").
const (
	v3DefaultPRF   = prfHMACSHA512
	v3Iterations   = 100_000
	saltSize       = 16
	subkeyLength   = 32
	v2Iterations   = 1_000
	formatMarkerV2 = 0x00
	formatMarkerV3 = 0x01
)

// KeyDerivationPrf của .NET.
const (
	prfHMACSHA1   uint32 = 0
	prfHMACSHA256 uint32 = 1
	prfHMACSHA512 uint32 = 2
)

// VerifyResult tương đương PasswordVerificationResult.
type VerifyResult int

const (
	VerifyFailed VerifyResult = iota
	VerifySuccess
	// Mật khẩu đúng nhưng hash dùng định dạng/tham số cũ → nên băm lại
	// (CheckPasswordAsync của C# tự làm việc này).
	VerifySuccessRehashNeeded
)

// HashPassword tạo hash định dạng V3:
// [0x01][prf uint32 BE][iter uint32 BE][saltLen uint32 BE][salt][subkey], base64.
func HashPassword(password string) string {
	salt := make([]byte, saltSize)
	rand.Read(salt) // từ Go 1.24 không bao giờ trả lỗi
	subkey := derive(v3DefaultPRF, password, salt, v3Iterations, subkeyLength)

	out := make([]byte, 13+len(salt)+len(subkey))
	out[0] = formatMarkerV3
	binary.BigEndian.PutUint32(out[1:], v3DefaultPRF)
	binary.BigEndian.PutUint32(out[5:], v3Iterations)
	binary.BigEndian.PutUint32(out[9:], saltSize)
	copy(out[13:], salt)
	copy(out[13+saltSize:], subkey)
	return base64.StdEncoding.EncodeToString(out)
}

// VerifyHashedPassword kiểm mật khẩu với hash V2 hoặc V3 (mọi PRF / số vòng lặp).
func VerifyHashedPassword(hashed, password string) VerifyResult {
	raw, err := base64.StdEncoding.DecodeString(hashed)
	if err != nil || len(raw) == 0 {
		return VerifyFailed
	}
	switch raw[0] {
	case formatMarkerV2:
		// V2: PBKDF2-HMAC-SHA1, 1000 vòng, salt 16, subkey 32 → luôn cần băm lại.
		if len(raw) != 1+saltSize+subkeyLength {
			return VerifyFailed
		}
		salt, expected := raw[1:1+saltSize], raw[1+saltSize:]
		if equal(derive(prfHMACSHA1, password, salt, v2Iterations, subkeyLength), expected) {
			return VerifySuccessRehashNeeded
		}
		return VerifyFailed

	case formatMarkerV3:
		if len(raw) < 13 {
			return VerifyFailed
		}
		prf := binary.BigEndian.Uint32(raw[1:])
		iter := binary.BigEndian.Uint32(raw[5:])
		saltLen := int(binary.BigEndian.Uint32(raw[9:]))
		// .NET yêu cầu salt ≥ 128 bit và subkey ≥ 128 bit.
		if prf > prfHMACSHA512 || iter == 0 || saltLen < 16 || len(raw) < 13+saltLen+16 {
			return VerifyFailed
		}
		salt := raw[13 : 13+saltLen]
		expected := raw[13+saltLen:]
		if !equal(derive(prf, password, salt, int(iter), len(expected)), expected) {
			return VerifyFailed
		}
		if prf != v3DefaultPRF || iter < v3Iterations {
			return VerifySuccessRehashNeeded
		}
		return VerifySuccess
	}
	return VerifyFailed
}

func derive(prf uint32, password string, salt []byte, iter, keyLen int) []byte {
	var h func() hash.Hash
	switch prf {
	case prfHMACSHA1:
		h = sha1.New
	case prfHMACSHA256:
		h = sha256.New
	default:
		h = sha512.New
	}
	key, err := pbkdf2.Key(h, password, salt, iter, keyLen)
	if err != nil {
		// Chỉ lỗi khi tham số vượt giới hạn FIPS — trả key rỗng để so sánh thất bại.
		return nil
	}
	return key
}

func equal(a, b []byte) bool {
	return len(a) == len(b) && subtle.ConstantTimeCompare(a, b) == 1
}
