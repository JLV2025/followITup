package mail

import (
	"database/sql"
	"fmt"
	"net/smtp"

	"followitup/internal/settings"
)

// Send 发送纯文本邮件。smtp_username 为空时无需认证，否则用 PlainAuth。
func Send(db *sql.DB, to, subject, body string) error {
	host, err := settings.Get(db, settings.KeySMTPHost)
	if err != nil || host == "" {
		return fmt.Errorf("SMTP 未配置")
	}
	port, _ := settings.Get(db, settings.KeySMTPPort)
	if port == "" {
		port = "25"
	}
	username, _ := settings.Get(db, settings.KeySMTPUsername)
	password, _ := settings.Get(db, settings.KeySMTPPassword)
	sender, _ := settings.Get(db, settings.KeySMTPSender)
	if sender == "" {
		return fmt.Errorf("发件人未配置")
	}

	msg := []byte("To: " + to + "\r\n" +
		"From: " + sender + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" + body + "\r\n")

	addr := fmt.Sprintf("%s:%s", host, port)
	if username == "" {
		return smtp.SendMail(addr, nil, sender, []string{to}, msg)
	}
	auth := smtp.PlainAuth("", username, password, host)
	return smtp.SendMail(addr, auth, sender, []string{to}, msg)
}

// SendTemporaryPassword 发送账号创建通知（含初始密码）
func SendTemporaryPassword(db *sql.DB, to, displayName, password string) error {
	subject := "FollowITup 账号已创建"
	body := fmt.Sprintf(`你好，%s：

你的 FollowITup 账号已创建，请使用以下信息登录：
  邮箱：%s
  初始密码：%s

首次登录后系统会要求你修改密码。

—— FollowITup 系统通知`, displayName, to, password)
	return Send(db, to, subject, body)
}
