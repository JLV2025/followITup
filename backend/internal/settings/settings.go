package settings

import (
	"database/sql"
	"fmt"
	"strconv"
	"sync"
)

// 预置配置 key（白名单）
const (
	KeySMTPHost         = "smtp_host"
	KeySMTPPort         = "smtp_port"
	KeySMTPUsername     = "smtp_username"
	KeySMTPPassword     = "smtp_password"
	KeySMTPSender       = "smtp_sender"
	KeyPasswordMinLen   = "password_min_length"
	KeyFiscalStartMonth = "fiscal_start_month"
	KeyDueReminderOn    = "due_reminder_enabled"
	KeyDueReminderDays  = "due_reminder_days"
	KeyBaseURL          = "base_url"
)

// Defaults 默认值（首次使用时写入）
var Defaults = map[string]string{
	KeySMTPHost:         "smtprelay-west.corp.qorvo.com",
	KeySMTPPort:         "25",
	KeySMTPUsername:     "",
	KeySMTPPassword:     "",
	KeySMTPSender:       "FollowITup@qorvo.com",
	KeyPasswordMinLen:   "8",
	KeyFiscalStartMonth: "4",
	KeyDueReminderOn:    "0",
	KeyDueReminderDays:  "3",
	KeyBaseURL:          "",
}

// AllKeys 所有合法 key（PUT 白名单）
var AllKeys = []string{
	KeySMTPHost, KeySMTPPort, KeySMTPUsername, KeySMTPPassword,
	KeySMTPSender, KeyPasswordMinLen, KeyFiscalStartMonth,
	KeyDueReminderOn, KeyDueReminderDays, KeyBaseURL,
}

// 每个 db 实例的默认值只写入一次，避免每次 Get/GetAll 都发一遍 INSERT OR IGNORE
var defaultsDone sync.Map // *sql.DB → struct{}

// ensureDefaults 首次访问时写入默认值
func ensureDefaults(db *sql.DB) {
	if _, loaded := defaultsDone.LoadOrStore(db, struct{}{}); loaded {
		return
	}
	for k, v := range Defaults {
		db.Exec(`INSERT OR IGNORE INTO settings (key, value) VALUES (?, ?)`, k, v)
	}
}

// Get 读取单个配置值
func Get(db *sql.DB, key string) (string, error) {
	ensureDefaults(db)
	var v string
	err := db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if err != nil {
		return "", fmt.Errorf("读取配置 %s 失败: %w", key, err)
	}
	return v, nil
}

// GetInt 读取整型配置，失败或非法时返回默认值
func GetInt(db *sql.DB, key string, def int) int {
	v, err := Get(db, key)
	if err != nil {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// GetAll 读取全部配置
func GetAll(db *sql.DB) (map[string]string, error) {
	ensureDefaults(db)
	rows, err := db.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return nil, fmt.Errorf("读取配置失败: %w", err)
	}
	defer rows.Close()
	m := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err == nil {
			m[k] = v
		}
	}
	return m, nil
}

// Set 写入单个配置
func Set(db *sql.DB, key, value string) error {
	_, err := db.Exec(
		`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, datetime('now'))
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = datetime('now')`,
		key, value)
	if err != nil {
		return fmt.Errorf("写入配置 %s 失败: %w", key, err)
	}
	return nil
}
