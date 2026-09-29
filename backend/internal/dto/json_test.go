package dto

import (
	"encoding/json"
	"strings"
	"testing"
)

// esc dựng chuỗi escape kiểu JSON: esc("00F3") → backslash + "u00F3".
// (Không viết thẳng literal backslash-u trong file để tránh công cụ soạn thảo tự giải mã.)
func esc(hex string) string { return `\` + "u" + hex }

func TestMarshalLikeDotNet(t *testing.T) {
	content := "có chứ bạn <a href='x'>&+`\"\\\n\b😀"
	got, err := MarshalJSON(map[string]string{"content": content})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"content":"c` + esc("00F3") + ` ch` + esc("1EE9") + ` b` + esc("1EA1") + `n ` +
		esc("003C") + `a href=` + esc("0027") + `x` + esc("0027") + esc("003E") +
		esc("0026") + esc("002B") + esc("0060") + esc("0022") + `\\\n\b` + esc("D83D") + esc("DE00") + `"}`
	if string(got) != want {
		t.Fatalf("\ngot  %s\nwant %s", got, want)
	}
	var back map[string]string
	if err := json.Unmarshal(got, &back); err != nil || back["content"] != content {
		t.Fatalf("round-trip hỏng: %v %q", err, back["content"])
	}
	// Ngoài chuỗi không bị đụng tới.
	if got, _ := MarshalJSON(map[string]any{"a": 1.5, "b": []int{1}, "c": nil}); strings.Contains(string(got), esc("")) {
		t.Fatalf("escape nhầm ngoài chuỗi: %s", got)
	}
}
