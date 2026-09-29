package validate

import (
	"slices"
	"testing"
)

func TestRules(t *testing.T) {
	var e Errors
	e.Required("Name", "   ", "Tên là bắt buộc")
	e.StringLength("Name", "ab", 3, 200, "")
	e.Email("Email", "a@b@c")
	e.Email("Email", "") // rỗng: để [Required] xử lý
	e.Regex("Slug", "Bad Slug", Slug, SlugMessage)
	e.Regex("Slug", "ok-slug-1", Slug, SlugMessage)
	e.RangeInt("Quantity", 0, 1, 1000, "")
	want := []string{
		"Name: Tên là bắt buộc",
		"Name: The field Name must be a string with a minimum length of 3 and a maximum length of 200.",
		"Email: The Email field is not a valid e-mail address.",
		"Slug: " + SlugMessage,
		"Quantity: The field Quantity must be between 1 and 1000.",
	}
	if !slices.Equal(e.List(), want) {
		t.Fatalf("got  %q\nwant %q", e.List(), want)
	}
}

func TestLenUTF16(t *testing.T) {
	if Len("Việt") != 4 || Len("😀") != 2 {
		t.Fatal("độ dài phải tính theo UTF-16 như C#")
	}
}
