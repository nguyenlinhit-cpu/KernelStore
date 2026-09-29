// Package migrations nhúng các file SQL migration vào binary, nên backend chạy được
// ở bất kỳ thư mục nào (kể cả bản build bằng Nix `buildGoModule`).
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
