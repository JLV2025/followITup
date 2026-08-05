package auth

import (
	"crypto/rand"
	"math/big"
	"strings"
)

const (
	lowerChars  = "abcdefghijklmnopqrstuvwxyz"
	upperChars  = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digitChars  = "0123456789"
	symbolChars = "!@#$%^&*"
)

// GenerateRandomPassword 生成随机密码：大小写字母+数字+符号混合，至少含 3 类字符
func GenerateRandomPassword(length int) string {
	if length < 8 {
		length = 8
	}
	all := lowerChars + upperChars + digitChars + symbolChars
	// 先各取一类保证多样性（不足 4 位时循环覆盖）
	seeds := []string{lowerChars, upperChars, digitChars, symbolChars}
	var sb strings.Builder
	for i := 0; i < 4 && i < length; i++ {
		sb.WriteByte(seeds[i%4][randInt(len(seeds[i%4]))])
	}
	for sb.Len() < length {
		sb.WriteByte(all[randInt(len(all))])
	}
	// 打乱顺序（Fisher-Yates，用 crypto/rand）
	b := []byte(sb.String())
	for i := len(b) - 1; i > 0; i-- {
		j := randInt(i + 1)
		b[i], b[j] = b[j], b[i]
	}
	return string(b)
}

// DeriveDisplayName 从邮箱推导显示名：local 部分按点拆分、各段首字母大写、空格拼接
func DeriveDisplayName(email string) string {
	at := strings.Index(email, "@")
	local := email
	if at >= 0 {
		local = email[:at]
	}
	parts := strings.Split(local, ".")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	name := strings.Join(parts, " ")
	if name == "" {
		return email
	}
	return name
}

// randInt 返回 [0, max) 的随机整数（crypto/rand）
func randInt(max int) int {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0
	}
	return int(n.Int64())
}
