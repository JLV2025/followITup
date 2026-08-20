package mail

import (
	"database/sql"
	"fmt"
	"net/smtp"

	"followitup/internal/settings"
)

// Send 发送纯文本邮件。smtp_username 为空时无需认证，否则用 PlainAuth。
// 部署网址未配置时拒绝发送——邮件必须带服务器地址，否则收件人无法得知登录入口。
func Send(db *sql.DB, to, subject, body string) error {
	baseURL, err := settings.Get(db, settings.KeyBaseURL)
	if err != nil || baseURL == "" {
		return fmt.Errorf("部署网址未配置")
	}
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

	body = body + "\n\nSign in at: " + baseURL + "\n"

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

// SendPasswordReset 发送密码重置通知（含新密码）
func SendPasswordReset(db *sql.DB, to, displayName, password string) error {
	subject := "FollowITup Password Reset"
	body := fmt.Sprintf(`Hi %s,

Your FollowITup password has been reset by an administrator. Sign in with:
  Email: %s
  New password: %s

If the administrator enabled "must change password on next login", you will be asked to set a new one after signing in.

— FollowITup`, displayName, to, password)
	return Send(db, to, subject, body)
}

// SendTemporaryPassword 发送账号创建通知（含初始密码）
func SendTemporaryPassword(db *sql.DB, to, displayName, password string) error {
	subject := "FollowITup Account Created"
	body := fmt.Sprintf(`Hi %s,

Your FollowITup account has been created. Sign in with:
  Email: %s
  Initial password: %s

You will be asked to change your password after your first sign-in.

— FollowITup`, displayName, to, password)
	return Send(db, to, subject, body)
}
