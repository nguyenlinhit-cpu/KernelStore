package money

import (
	"encoding/json"
	"testing"
)

func TestScaleLikeCSharpDecimal(t *testing.T) {
	cases := []struct{ got, want string }{
		{MustParse("24.50").String(), "24.50"},
		{MustParse("100").String(), "100"},
		{Zero.String(), "0"},
		{MustParse("1906.19").MulInt(3).String(), "5718.57"},
		{MustParse("0.10").Add(MustParse("0.2")).String(), "0.30"},
		{MustParse("100").Add(MustParse("0.00")).String(), "100.00"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("got %s, want %s", c.got, c.want)
		}
	}
}

func TestJSON(t *testing.T) {
	var v struct {
		A Money  `json:"a"`
		B *Money `json:"b"`
		C Money  `json:"c"`
	}
	if err := json.Unmarshal([]byte(`{"a":24.50,"b":null,"c":"7.5"}`), &v); err != nil {
		t.Fatal(err)
	}
	out, _ := json.Marshal(v)
	if string(out) != `{"a":24.50,"b":null,"c":7.5}` {
		t.Fatalf("got %s", out)
	}
	if json.Unmarshal([]byte(`{"a":"abc"}`), &v) == nil {
		t.Fatal("chuỗi không phải số phải lỗi")
	}
}
