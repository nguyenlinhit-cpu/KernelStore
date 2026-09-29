package dto

import (
	"bytes"
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

// MarshalJSON mã hoá v giống System.Text.Json với JavaScriptEncoder.Default (mặc định
// của ASP.NET Core): trong chuỗi, mọi ký tự ngoài ASCII và các ký tự " & ' + < > `
// được ghi thành \uXXXX (hex HOA), ví dụ "có" → "có".
// Client đọc JSON không thấy khác biệt, nhưng output trùng byte với bản C#
// (test chat grep trực tiếp chuỗi đã escape trong payload WebSocket).
func MarshalJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return escapeLikeDotNet(bytes.TrimRight(buf.Bytes(), "\n")), nil
}

// escapeLikeDotNet chuyển output của encoding/json sang quy tắc escape của .NET.
// Chỉ đụng tới nội dung bên trong chuỗi JSON.
func escapeLikeDotNet(in []byte) []byte {
	out := make([]byte, 0, len(in)+len(in)/8)
	inString := false
	for i := 0; i < len(in); {
		c := in[i]
		if !inString {
			if c == '"' {
				inString = true
			}
			out = append(out, c)
			i++
			continue
		}
		switch {
		case c == '"':
			inString = false
			out = append(out, c)
			i++
		case c == '\\':
			// Escape sẵn có của Go: \" \\ \n \r \t \b \f \uXXXX.
			next := in[i+1]
			switch next {
			case '"':
				out = append(out, `\`+"u0022"...)
				i += 2
			case 'u':
				hex := string(in[i+2 : i+6])
				switch hex {
				case "0008":
					out = append(out, `\b`...)
				case "000c":
					out = append(out, `\f`...)
				default:
					out = append(out, `\u`...)
					out = append(out, bytes.ToUpper(in[i+2:i+6])...)
				}
				i += 6
			default:
				out = append(out, c, next)
				i += 2
			}
		case c == '&' || c == '\'' || c == '+' || c == '<' || c == '>' || c == '`' || c == 0x7F:
			out = fmt.Appendf(out, `\u%04X`, c)
			i++
		case c < utf8.RuneSelf:
			out = append(out, c)
			i++
		default:
			r, size := utf8.DecodeRune(in[i:])
			if r > 0xFFFF { // ngoài BMP → cặp surrogate như UTF-16
				r -= 0x10000
				out = fmt.Appendf(out, `\u%04X\u%04X`, 0xD800+(r>>10), 0xDC00+(r&0x3FF))
			} else {
				out = fmt.Appendf(out, `\u%04X`, r)
			}
			i += size
		}
	}
	return out
}
