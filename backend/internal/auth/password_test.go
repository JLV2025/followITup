package auth

import (
	"strings"
	"testing"
)

func TestDeriveDisplayName(t *testing.T) {
	cases := []struct{ email, want string }{
		{"john.doe@qorvo.com", "John Doe"},
		{"mary.jane.smith@qorvo.com", "Mary Jane Smith"},
		{"jlv2025@qorvo.com", "Jlv2025"},
		{"boss@qorvo.com", "Boss"},
	}
	for _, c := range cases {
		if got := DeriveDisplayName(c.email); got != c.want {
			t.Errorf("DeriveDisplayName(%q) = %q, want %q", c.email, got, c.want)
		}
	}
}

func TestGenerateRandomPassword(t *testing.T) {
	p := GenerateRandomPassword(12)
	if len(p) != 12 {
		t.Errorf("长度 = %d, want 12", len(p))
	}
	classes := 0
	for _, set := range []string{lowerChars, upperChars, digitChars, symbolChars} {
		if strings.ContainsAny(p, set) {
			classes++
		}
	}
	if classes < 3 {
		t.Errorf("字符类别不足: %q (仅 %d 类)", p, classes)
	}
	// 两次生成不应相同
	if p == GenerateRandomPassword(12) {
		t.Error("两次生成相同密码，随机性异常")
	}
}
