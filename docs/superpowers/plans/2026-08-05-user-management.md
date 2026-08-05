# 用户管理升级实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 账号创建（邮箱+随机密码+邮件通知+首登强制改密）、显示名自动推导、assignee 下拉、权限模型（全员建号/管理员管删与角色）、统一系统配置页（SMTP/财年/节假日/密码策略）。

**Architecture:** 新表 `settings`（key-value）+ 标准库 `net/smtp` 邮件服务（无新依赖）；JWT claims 加 `must_change_password` 标记驱动首登改密拦截；创建/删除/角色端点改造；前端新增 `/admin/settings` 配置页与 `/change-password` 改密页，用户管理页与 TaskDetailModal 改造。

**Tech Stack:** Go 1.22+（chi/SQLite/net/smtp）/ React 18 TypeScript / Vite

## Global Constraints

- 不引入新依赖（`net/smtp`、`crypto/rand` 均为标准库；前端不新增包）
- 所有注释、提交信息使用简体中文；专业术语保留英文
- 改符号前 `gitnexus_impact({target, direction:"upstream"})`，HIGH/CRITICAL 风险先告知用户
- 提交前 `gitnexus_detect_changes()` 验证影响范围
- 后端每步跑 `go build ./...` + `go test ./...`；前端每步跑 `npx tsc --noEmit`
- 每个逻辑变更单独提交（中文提交信息）
- 设计依据：`docs/superpowers/specs/2026-08-05-user-management-design.md`（用户确认决策 10 条）
- 随机密码**不落日志**；邮件发送失败不阻塞主流程
- 系统必须始终至少有一名管理员；管理员不可被删除（先降级）

---

### Task 1: settings 配置表 + 读写包 + Settings API

**Files:**
- Modify: `backend/internal/db/sqlite.go`（建表）
- Create: `backend/internal/settings/settings.go`（读写包，供 mail/auth/api 复用，避免循环依赖）
- Create: `backend/internal/api/settings.go`（handler + 路由）
- Modify: `backend/internal/api/helpers.go`（RegisterRoutes 挂载）

**Interfaces:**
- Consumes: `auth.Middleware`（RequireAuth/AdminOnly 模式）
- Produces:
  - `settings.Get(db, key) (string, error)` / `settings.GetInt(db, key string, def int) int` / `settings.GetAll(db) (map[string]string, error)` / `settings.Set(db, key, value string) error`
  - `GET /api/settings`（RequireAuth）→ `{data: {fiscal_start_month, password_min_length}}`（公开子集）
  - `GET /api/settings/admin`（AdminOnly）→ `{data: 全量}`
  - `PUT /api/settings`（AdminOnly）→ 白名单更新，响应全量

- [ ] **Step 1: sqlite.go 建表**

`backend/internal/db/sqlite.go` 的 schema 执行区追加（放在现有 CREATE TABLE 之后）：

```go
	// 系统配置表（key-value）
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS settings (
		key        TEXT PRIMARY KEY,
		value      TEXT NOT NULL,
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`)
	if err != nil {
		return nil, fmt.Errorf("创建 settings 表失败: %w", err)
	}
```

（具体插入位置以 sqlite.go 现有 schema 执行结构为准，参照相邻表创建语句的错误处理模式。）

- [ ] **Step 2: 创建 settings 读写包**

`backend/internal/settings/settings.go`：

```go
package settings

import (
	"database/sql"
	"fmt"
	"strconv"
)

// 预置配置 key（白名单）
const (
	KeySMTPHost        = "smtp_host"
	KeySMTPPort        = "smtp_port"
	KeySMTPUsername    = "smtp_username"
	KeySMTPPassword    = "smtp_password"
	KeySMTPSender      = "smtp_sender"
	KeyPasswordMinLen  = "password_min_length"
	KeyFiscalStartMonth = "fiscal_start_month"
)

// Defaults 默认值（首次使用时写入）
var Defaults = map[string]string{
	KeySMTPHost:          "smtprelay-west.corp.qorvo.com",
	KeySMTPPort:          "25",
	KeySMTPUsername:      "",
	KeySMTPPassword:      "",
	KeySMTPSender:        "FollowITup@qorvo.com",
	KeyPasswordMinLen:    "8",
	KeyFiscalStartMonth:  "4",
}

// AllKeys 所有合法 key（PUT 白名单）
var AllKeys = []string{
	KeySMTPHost, KeySMTPPort, KeySMTPUsername, KeySMTPPassword,
	KeySMTPSender, KeyPasswordMinLen, KeyFiscalStartMonth,
}

// ensureDefaults 首次访问时写入默认值
func ensureDefaults(db *sql.DB) {
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
```

- [ ] **Step 3: SettingsHandler**

`backend/internal/api/settings.go`：

```go
package api

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"followitup/internal/auth"
	"followitup/internal/settings"
)

type SettingsHandler struct {
	db  *sql.DB
	mid *auth.Middleware
}

func NewSettingsHandler(db *sql.DB, mid *auth.Middleware) *SettingsHandler {
	return &SettingsHandler{db: db, mid: mid}
}

func (h *SettingsHandler) RegisterRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(h.mid.RequireAuth)
		r.Get("/api/settings", h.GetPublic)
	})
	r.Group(func(r chi.Router) {
		r.Use(h.mid.RequireAuth)
		r.Use(h.mid.AdminOnly)
		r.Get("/api/settings/admin", h.GetAll)
		r.Put("/api/settings", h.Put)
	})
}

// GetPublic 公开子集（创建用户表单、财年展示需要）
func (h *SettingsHandler) GetPublic(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"fiscal_start_month":  settings.GetInt(h.db, settings.KeyFiscalStartMonth, 4),
		"password_min_length": settings.GetInt(h.db, settings.KeyPasswordMinLen, 8),
	})
}

// GetAll 全量配置（仅管理员）
func (h *SettingsHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	m, err := settings.GetAll(h.db)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "读取系统配置失败")
		return
	}
	writeJSON(w, http.StatusOK, m)
}

// Put 批量更新（仅管理员，白名单 key）
func (h *SettingsHandler) Put(w http.ResponseWriter, r *http.Request) {
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "请求格式错误")
		return
	}
	allowed := make(map[string]bool)
	for _, k := range settings.AllKeys {
		allowed[k] = true
	}
	for k, v := range req {
		if !allowed[k] {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "未知配置项: "+k)
			return
		}
		value := ""
		switch val := v.(type) {
		case string:
			value = val
		case float64:
			value = strconv.FormatFloat(val, 'f', -1, 64)
		default:
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "配置值格式错误: "+k)
			return
		}
		if k == settings.KeyPasswordMinLen {
			n, err := strconv.Atoi(value)
			if err != nil || n < 6 || n > 32 {
				writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "密码最小长度需在 6-32 之间")
				return
			}
		}
		if err := settings.Set(h.db, k, value); err != nil {
			writeError(w, http.StatusInternalServerError, "DB_ERROR", "保存配置失败")
			return
		}
	}
	m, _ := settings.GetAll(h.db)
	writeJSON(w, http.StatusOK, m)
}
```

（需 import `strconv`、`github.com/go-chi/chi/v5`。）

- [ ] **Step 4: 挂载路由**

`backend/internal/api/helpers.go` 的 `RegisterRoutes` 追加：

```go
func RegisterRoutes(r chi.Router, authHandler *AuthHandler, settingsHandler *SettingsHandler) {
	authHandler.RegisterRoutes(r)
	settingsHandler.RegisterRoutes(r)
}
```

同步修改 `backend/internal/server/server.go`（或对应组装处）：构造 `SettingsHandler` 并传入。参照现有 AuthHandler 的组装方式。

- [ ] **Step 5: 编译 + 全量测试**

Run: `cd F:\projects\followITup\backend && go build ./... && go test ./...`
Expected: 编译通过、全部 PASS

- [ ] **Step 6: 提交**

```bash
git add backend/internal/db/sqlite.go backend/internal/settings/settings.go backend/internal/api/settings.go backend/internal/api/helpers.go backend/internal/server/
git commit -m "后端:系统配置表settings+读写包+API(公开子集/全量/白名单更新)"
```

---

### Task 2: 邮件服务 + SMTP 测试发送

**Files:**
- Create: `backend/internal/mail/mail.go`
- Modify: `backend/internal/api/settings.go`（加 test-email 端点）

**Interfaces:**
- Consumes: Task 1 的 `settings.Get`（smtp_host/smtp_port/smtp_username/smtp_password/smtp_sender）
- Produces:
  - `mail.Send(db *sql.DB, to, subject, body string) error`（无认证 / PlainAuth 自适应）
  - `mail.SendTemporaryPassword(db *sql.DB, to, displayName, password string) error`（账号创建通知）
  - `POST /api/settings/test-email`（AdminOnly）`{to}` → 200「测试邮件已发送」/ 400 失败原因

- [ ] **Step 1: mail 包**

`backend/internal/mail/mail.go`：

```go
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
```

- [ ] **Step 2: test-email 端点**

`backend/internal/api/settings.go` 的 RegisterRoutes 管理组内追加（Put 之后）：

```go
		r.Post("/api/settings/test-email", h.TestEmail)
```

及 handler：

```go
// TestEmail 测试 SMTP 配置（仅管理员）
func (h *SettingsHandler) TestEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		To string `json:"to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.To == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "缺少收件人地址")
		return
	}
	body := "这是一封来自 FollowITup 的测试邮件。如果你收到这封邮件，说明 SMTP 配置正确。"
	if err := mail.Send(h.db, req.To, "FollowITup SMTP 测试", body); err != nil {
		writeError(w, http.StatusBadRequest, "MAIL_FAILED", "邮件发送失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "测试邮件已发送"})
}
```

（import `followitup/internal/mail`。）

- [ ] **Step 3: 编译 + 全量测试**

Run: `cd F:\projects\followITup\backend && go build ./... && go test ./...`
Expected: 编译通过、全部 PASS

- [ ] **Step 4: 提交**

```bash
git add backend/internal/mail/mail.go backend/internal/api/settings.go
git commit -m "后端:邮件服务(标准库net/smtp,无认证自适应)+SMTP测试发送端点"
```

---

### Task 3: 创建用户改造（随机密码/显示名推导/权限放开）+ 用户列表端点

**Files:**
- Modify: `backend/internal/auth/auth.go`（service：CreateUser 签名、随机密码、显示名推导）
- Modify: `backend/internal/api/auth.go`（handler：去 AdminOnly、is_admin 校验、发邮件、响应）
- Create: `backend/internal/auth/password.go`（随机密码生成，独立文件便于单测）

**Interfaces:**
- Consumes: Task 1 的 `settings.GetInt(db, KeyPasswordMinLen, 8)`、Task 2 的 `mail.SendTemporaryPassword`
- Produces:
  - `auth.GenerateRandomPassword(length int) string`（12 位：大小写+数字+符号，至少 3 类）
  - `auth.DeriveDisplayName(email string) string`（`john.doe@` → `John Doe`；无点用原样）
  - `Service.CreateUser(email, password, displayName, authSource string, isAdmin, mustChange bool) error`
  - `POST /api/users`（全部登录用户）→ 创建；响应 `{message, initial_password?, mail_sent}`
  - `GET /api/users`（RequireAuth）→ `{data: [{id, display_name, email}]}`（active 用户，assignee 下拉数据源）

- [ ] **Step 1: 随机密码 + 显示名推导**

`backend/internal/auth/password.go`：

```go
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
	// 先各取一类保证多样性（4 类取 4 位，若 length 不足则循环覆盖）
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
```

- [ ] **Step 2: service.CreateUser 加 mustChange 参数**

`backend/internal/auth/auth.go` 修改（第 48-65 行）：

```go
// CreateUser 创建用户
func (s *Service) CreateUser(email, password, displayName, authSource string, isAdmin, mustChange bool) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.bcryptCost)
	if err != nil {
		return fmt.Errorf("密码哈希失败: %w", err)
	}

	login := email // 邮箱即登录名
	_, err = s.db.Exec(
		`INSERT INTO users (login, email, display_name, password_hash, auth_source, is_admin, must_change_password)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		login, email, displayName, string(hash), authSource, boolToInt(isAdmin), boolToInt(mustChange),
	)
	if err != nil {
		return fmt.Errorf("创建用户失败: %w", err)
	}
	return nil
}
```

同步修改调用处：`InitAdmin`（第 45 行）→ `s.CreateUser(email, password, displayName, "local", true, false)`。

- [ ] **Step 3: handler 改造**

`backend/internal/api/auth.go`：

1. 路由（第 30-31 行）——去掉 AdminOnly，创建放开全部登录用户；新增 GET /api/users：

```go
	r.Get("/api/admin/users", withAuth(h.mid, h.AdminOnly(h.ListUsers)))
	r.Post("/api/admin/users", withAuth(h.mid, h.CreateUser))          // 权限放开：全部登录用户可创建
	r.Get("/api/users", withAuth(h.mid, h.PublicUsers))                // assignee 下拉数据源
```

2. CreateUser handler 重写（替换第 89-115 行）：

```go
// CreateUser 创建本地用户（全部登录用户可创建；仅管理员可设置管理员角色）
func (h *AuthHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
		IsAdmin     bool   `json:"is_admin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "请求格式错误")
		return
	}
	if req.Email == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "邮箱不能为空")
		return
	}
	if !validEmailFormat(req.Email) {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "邮箱格式不正确")
		return
	}
	// 显示名：未提供时从邮箱推导
	if req.DisplayName == "" {
		req.DisplayName = auth.DeriveDisplayName(req.Email)
	}
	// 仅管理员可创建管理员；普通用户传 true 强制忽略
	if req.IsAdmin && !auth.GetIsAdmin(r.Context()) {
		req.IsAdmin = false
	}
	minLen := settings.GetInt(h.svc.DB(), settings.KeyPasswordMinLen, 8)
	password := auth.GenerateRandomPassword(minLen)
	if err := h.svc.CreateUser(req.Email, password, req.DisplayName, "local", req.IsAdmin, true); err != nil {
		writeError(w, http.StatusConflict, "DUPLICATE", err.Error())
		return
	}
	// 发送通知邮件；失败不阻塞创建，明文密码退回给创建者手动传达
	mailErr := mail.SendTemporaryPassword(h.svc.DB(), req.Email, req.DisplayName, password)
	if mailErr != nil {
		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"message":          "用户创建成功（邮件发送失败，请手动告知初始密码）",
			"initial_password": password,
			"mail_sent":        false,
		})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message":   "用户创建成功，初始密码已发送至邮箱",
		"mail_sent": true,
	})
}

// validEmailFormat 简单邮箱格式校验
func validEmailFormat(email string) bool {
	at := strings.Index(email, "@")
	if at <= 0 || at == len(email)-1 {
		return false
	}
	return strings.Index(email[at+1:], ".") > 0
}
```

3. PublicUsers handler：

```go
// PublicUsers 返回活跃用户精简列表（assignee 下拉数据源）
func (h *AuthHandler) PublicUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.DB().Query(
		`SELECT id, display_name, email FROM users WHERE is_active = 1 ORDER BY display_name`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "查询用户失败")
		return
	}
	defer rows.Close()
	type Item struct {
		ID          int64  `json:"id"`
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
	}
	var items []Item
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.DisplayName, &it.Email); err == nil {
			items = append(items, it)
		}
	}
	if items == nil {
		items = []Item{}
	}
	writeJSON(w, http.StatusOK, items)
}
```

4. service 暴露 DB 访问：`backend/internal/auth/auth.go` 加：

```go
// DB 返回数据库句柄（供 api 层读取配置等）
func (s *Service) DB() *sql.DB { return s.db }
```

（import 补充：`strings`、`followitup/internal/settings`、`followitup/internal/mail`。）

- [ ] **Step 4: 单测**

`backend/internal/auth/password_test.go`：

```go
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
```

- [ ] **Step 5: 编译 + 全量测试**

Run: `cd F:\projects\followITup\backend && go build ./... && go test ./...`
Expected: 编译通过、全部 PASS（含新增单测）

- [ ] **Step 6: 提交**

```bash
git add backend/internal/auth/
git commit -m "后端:创建用户改造(随机密码+首登改密标记+显示名推导+全员可建号,仅管理员可设管理员)+GET /api/users"
```

---

### Task 4: 删除用户 + 提升/降级管理员端点

**Files:**
- Modify: `backend/internal/api/auth.go`（路由 + 2 handler）
- Modify: `backend/internal/auth/auth.go`（service：DeleteUser、SetUserRole）

**Interfaces:**
- Consumes: `auth.GetIsAdmin`、`auth.GetUserID`
- Produces:
  - `DELETE /api/admin/users/{id}`（AdminOnly）→ 200；400「请先取消其管理员身份」/「不能删除自己」；404「用户不存在」
  - `PUT /api/admin/users/{id}/role`（AdminOnly）`{is_admin}` → 200 更新后用户；400「系统至少保留一名管理员」；404
  - `Service.DeleteUser(id int64) error`、`Service.SetUserRole(id int64, isAdmin bool) error`

- [ ] **Step 1: service 实现**

`backend/internal/auth/auth.go` 追加：

```go
// DeleteUser 软删用户并清理项目成员关系。调用前需校验目标非管理员且非本人。
func (s *Service) DeleteUser(userID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(
		`UPDATE users SET is_active = 0, updated_at = datetime('now') WHERE id = ? AND is_active = 1`,
		userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM project_members WHERE user_id = ?`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

// SetUserRole 提升/降级管理员。降级时校验系统至少保留一名管理员。
func (s *Service) SetUserRole(userID int64, isAdmin bool) error {
	if !isAdmin {
		var adminCount int
		if err := s.db.QueryRow(
			`SELECT COUNT(*) FROM users WHERE is_active = 1 AND is_admin = 1`).Scan(&adminCount); err != nil {
			return err
		}
		var targetAdmin int
		s.db.QueryRow(`SELECT is_admin FROM users WHERE id = ? AND is_active = 1`, userID).Scan(&targetAdmin)
		if targetAdmin == 1 && adminCount <= 1 {
			return errors.New("系统至少保留一名管理员")
		}
	}
	res, err := s.db.Exec(
		`UPDATE users SET is_admin = ?, updated_at = datetime('now') WHERE id = ? AND is_active = 1`,
		boolToInt(isAdmin), userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New("用户不存在")
	}
	return nil
}
```

- [ ] **Step 2: handler 实现**

`backend/internal/api/auth.go` 路由追加（AdminOnly 组内）：

```go
	r.Delete("/api/admin/users/{id}", withAuth(h.mid, h.AdminOnly(h.DeleteUser)))
	r.Put("/api/admin/users/{id}/role", withAuth(h.mid, h.AdminOnly(h.SetUserRole)))
```

handler：

```go
// DeleteUser 删除用户（软删，仅管理员；管理员需先降级）
func (h *AuthHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	currentID, _ := auth.GetUserID(r.Context())
	if id == currentID {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "不能删除自己")
		return
	}
	var isAdmin int
	err := h.svc.DB().QueryRow(
		`SELECT is_admin FROM users WHERE id = ? AND is_active = 1`, id).Scan(&isAdmin)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "用户不存在")
		return
	}
	if isAdmin == 1 {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "请先取消其管理员身份")
		return
	}
	if err := h.svc.DeleteUser(id); err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "删除用户失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "用户已删除"})
}

// SetUserRole 提升/降级管理员（仅管理员）
func (h *AuthHandler) SetUserRole(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	var req struct {
		IsAdmin bool `json:"is_admin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "请求格式错误")
		return
	}
	if err := h.svc.SetUserRole(id, req.IsAdmin); err != nil {
		if strings.Contains(err.Error(), "至少保留") {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
			return
		}
		writeError(w, http.StatusNotFound, "NOT_FOUND", "用户不存在")
		return
	}
	user, err := h.svc.GetUserByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "用户不存在")
		return
	}
	writeJSON(w, http.StatusOK, user)
}
```

（import 补充：`strconv`、`github.com/go-chi/chi/v5`。）

- [ ] **Step 3: 编译 + 全量测试**

Run: `cd F:\projects\followITup\backend && go build ./... && go test ./...`
Expected: 编译通过、全部 PASS

- [ ] **Step 4: 提交**

```bash
git add backend/internal/auth/auth.go backend/internal/api/auth.go
git commit -m "后端:删除用户(软删+清理成员关系,管理员需先降级,不能删自己)+提升/降级管理员(至少保留一名)"
```

---

### Task 5: 首登强制改密（登录标记 + JWT + 中间件拦截 + 改密重签）

**Files:**
- Modify: `backend/internal/auth/auth.go`（Login 查询、GenerateToken、ChangePassword 重签辅助）
- Modify: `backend/internal/auth/middleware.go`（拦截逻辑）
- Modify: `backend/internal/api/auth.go`（ChangePassword handler 重签响应）
- Modify: `backend/internal/models/models.go`（LoginResponse 加字段）

**Interfaces:**
- Produces: `LoginResponse.must_change_password`、JWT claim `must_change_password`、403 `FORCE_PASSWORD_CHANGE`、`ChangePassword` 响应 `{token}`

- [ ] **Step 1: 模型 + Login + Token**

`backend/internal/models/models.go`：

```go
// LoginResponse 登录响应
type LoginResponse struct {
	Token              string `json:"token"`
	User               User   `json:"user"`
	MustChangePassword bool   `json:"must_change_password"`
}
```

`backend/internal/auth/auth.go`：

1. Login 查询加字段（第 76-84 行）：

```go
	var mustChange int
	err := s.db.QueryRow(
		`SELECT id, login, email, display_name, auth_source, is_admin, is_active,
		        password_hash, failed_attempts, locked_until, must_change_password
		 FROM users WHERE email = ? AND is_active = 1`,
		email,
	).Scan(&u.ID, &u.Login, &u.Email, &u.DisplayName, &u.AuthSource,
		&isAdmin, &isActive, &passwordHash, &failedAttempts, &lockedUntil, &mustChange)
```

2. 签发与响应（第 116-121 行）：

```go
	token, err := s.GenerateToken(&u, mustChange != 0)
	if err != nil {
		return nil, fmt.Errorf("生成 token 失败: %w", err)
	}

	return &models.LoginResponse{Token: token, User: u, MustChangePassword: mustChange != 0}, nil
```

3. GenerateToken 加参数（第 125-136 行）：

```go
// GenerateToken 为用户生成 JWT
func (s *Service) GenerateToken(user *models.User, mustChangePassword bool) (string, error) {
	claims := jwt.MapClaims{
		"user_id":              user.ID,
		"email":                user.Email,
		"is_admin":             user.IsAdmin,
		"must_change_password": mustChangePassword,
		"exp":                  time.Now().Add(time.Duration(s.sessionHours) * time.Hour).Unix(),
		"iat":                  time.Now().Unix(),
	}
	...
}
```

4. ChangePassword 后重签：service 增加方法：

```go
// ChangePasswordAndRetoken 修改密码并返回新 token（清首登标记后重签）
func (s *Service) ChangePasswordAndRetoken(userID int64, oldPassword, newPassword string) (string, error) {
	if err := s.ChangePassword(userID, oldPassword, newPassword); err != nil {
		return "", err
	}
	u, err := s.GetUserByID(userID)
	if err != nil {
		return "", err
	}
	return s.GenerateToken(u, false)
}
```

（ChangePassword 已清 `must_change_password=0`，无需改 SQL。）

- [ ] **Step 2: 中间件拦截**

`backend/internal/auth/middleware.go`：RequireAuth 与 OptionalAuth 解析成功后、写 context 后追加：

```go
		// 首登强制改密：未改密用户只能访问改密接口
		if mcp, _ := claims["must_change_password"].(bool); mcp && r.URL.Path != "/api/auth/change-password" {
			http.Error(w, `{"error":{"code":"FORCE_PASSWORD_CHANGE","message":"首次登录需先修改密码"}}`, http.StatusForbidden)
			return
		}
```

（两处都加，放在 `next.ServeHTTP` 之前。）

- [ ] **Step 3: ChangePassword handler 返回新 token**

`backend/internal/api/auth.go` 的 ChangePassword handler（第 129-144 行起）改为：

```go
// ChangePassword 修改密码（首登改密成功后返回新 token）
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录")
		return
	}

	var req models.ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "请求格式错误")
		return
	}

	token, err := h.svc.ChangePasswordAndRetoken(userID, req.OldPassword, req.NewPassword)
	if err != nil {
		writeError(w, http.StatusBadRequest, "PASSWORD_CHANGE_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}
```

- [ ] **Step 4: 编译 + 全量测试**

Run: `cd F:\projects\followITup\backend && go build ./... && go test ./...`
Expected: 编译通过、全部 PASS

- [ ] **Step 5: 提交**

```bash
git add backend/internal/models/models.go backend/internal/auth/ backend/internal/api/auth.go
git commit -m "后端:首登强制改密(登录带标记+JWT拦截非改密接口+改密成功重签token)"
```

---

### Task 6: 财年设置迁移到 settings

**Files:**
- Modify: `backend/internal/api/projects.go`（fiscalStartMonth 字段改动态读 settings）
- Modify: `backend/internal/server/server.go`（构造调整）

**Interfaces:**
- Consumes: Task 1 的 `settings.GetInt(db, settings.KeyFiscalStartMonth, 4)`
- Produces: `ProjectHandler` 不再持有启动时固化的 fiscalStartMonth，每次计算财年时读 settings（配置页修改即时生效）

- [ ] **Step 1: projects.go 改造**

`backend/internal/api/projects.go`：

1. 删除字段与构造参数（第 21、25-26 行）：

```go
type ProjectHandler struct {
	db  *sql.DB
	mid *auth.Middleware
	// fiscalStartMonth 已迁移至 settings 表（Task 6），动态读取
}

func NewProjectHandler(db *sql.DB, mid *auth.Middleware) *ProjectHandler {
	return &ProjectHandler{db: db, mid: mid}
}
```

2. 使用处（第 182 行附近，buildTimeFilter 内）：

```go
		fiscalMonth := settings.GetInt(h.db, settings.KeyFiscalStartMonth, 4)
		start, end, err := util.FiscalYearRange(fyInt, fiscalMonth)
```

（import 补充 `followitup/internal/settings`；若还有其他 `h.fiscalStartMonth` 使用点，一并替换。）

- [ ] **Step 2: server.go 构造同步**

`backend/internal/server/server.go`（或对应组装文件）：`NewProjectHandler(h.db, h.mid, fiscalStartMonth)` → `NewProjectHandler(h.db, h.mid)`；删除 config 读取 fiscalStartMonth 的传参（config.yaml 的 `fiscal:` 段保留不删，忽略即可；若 server.go 有对应校验逻辑，注释说明已迁移）。

- [ ] **Step 3: 编译 + 全量测试**

Run: `cd F:\projects\followITup\backend && go build ./... && go test ./...`
Expected: 编译通过、全部 PASS

- [ ] **Step 4: 提交**

```bash
git add backend/internal/api/projects.go backend/internal/server/
git commit -m "后端:财年起始月迁移至settings表(动态读取,配置页修改即时生效)"
```

---

### Task 7: 前端系统配置页（/admin/settings）+ 财年读取迁移

**Files:**
- Create: `frontend/src/pages/SystemSettings.tsx`
- Modify: `frontend/src/App.tsx`（路由）
- Modify: `frontend/src/components/Navbar.tsx`（「系统设置」入口，仅管理员）
- Modify: `frontend/src/stores/settingsStore.ts`（fiscalStartMonth 改从 /api/settings 读取）
- Modify: `frontend/src/styles/components.css`（配置页所需轻量样式，如可复用则不改）

**Interfaces:**
- Consumes: Task 1 的 `GET /api/settings`、`GET /api/settings/admin`、`PUT /api/settings`、`POST /api/settings/test-email`；既有 `GET/POST /api/calendar`、`DELETE /api/calendar/{id}`
- Produces: `/admin/settings` 页面（SMTP/财年/节假日/密码策略）

- [ ] **Step 1: settingsStore 财年迁移**

`frontend/src/stores/settingsStore.ts`：`fiscalStartMonth` 初始值改为从 `/api/settings` 拉取（登录后）：

```ts
// 财年起始月从系统配置读取（管理员在系统设置页修改），不再本地存储
async function loadFiscalStartMonth(): Promise<number> {
  try {
    const res = await api.get("/api/settings");
    const m = res.data?.data?.fiscal_start_month;
    return typeof m === "number" && m >= 1 && m <= 12 ? m : 4;
  } catch {
    return 4;
  }
}
```

在 store 初始化（或 Dashboard 挂载时）调用并 set；移除 localStorage 中 fiscalStartMonth 的读写（保留 displayMode 的本地存储）。具体接入方式以 settingsStore.ts 现有结构为准，最小改动：初始化时 `loadFiscalStartMonth().then((m) => set({ fiscalStartMonth: m }))`，`setFiscalStartMonth` 逻辑保留（页面内切换立即生效，刷新后按配置还原）。

- [ ] **Step 1b: Dashboard 移除财年起始月修改入口**

`frontend/src/pages/Dashboard.tsx`：删除「财年起始月份」select（约第 206-217 行，`displayMode === "fiscal"` 条件内的 `fiscal-month-select`），**保留**「📅 财年/🗓 自然年」模式切换按钮（第 199-205 行）。同时第 16 行 destructure 移除 `setFiscalStartMonth`（`fiscalStartMonth` 读取保留——财年计算仍用它）；若 settingsStore 的 `setFiscalStartMonth` 不再被任何组件使用则一并移除（仅保留配置页通过 PUT /api/settings 修改）。

- [ ] **Step 2: SystemSettings 页面**

`frontend/src/pages/SystemSettings.tsx`（完整代码）：

```tsx
import { useEffect, useState } from "react";
import api from "../api/client";

interface Holiday {
  id: number;
  date: string;
  type: string;
  label: string;
}

export default function SystemSettings() {
  // SMTP 配置
  const [smtp, setSmtp] = useState({ smtp_host: "", smtp_port: "25", smtp_username: "", smtp_password: "", smtp_sender: "" });
  // 财年 + 密码策略
  const [fiscalStartMonth, setFiscalStartMonth] = useState(4);
  const [passwordMinLength, setPasswordMinLength] = useState(8);
  // 节假日
  const [holidays, setHolidays] = useState<Holiday[]>([]);
  const [holidayDate, setHolidayDate] = useState("");
  const [holidayLabel, setHolidayLabel] = useState("");
  const [message, setMessage] = useState("");

  useEffect(() => {
    api.get("/api/settings/admin").then((res) => {
      const d = res.data?.data || {};
      setSmtp({
        smtp_host: d.smtp_host || "",
        smtp_port: d.smtp_port || "25",
        smtp_username: d.smtp_username || "",
        smtp_password: d.smtp_password || "",
        smtp_sender: d.smtp_sender || "",
      });
      setFiscalStartMonth(Number(d.fiscal_start_month) || 4);
      setPasswordMinLength(Number(d.password_min_length) || 8);
    }).catch(() => setMessage("加载配置失败"));
    fetchHolidays();
  }, []);

  const fetchHolidays = async () => {
    try {
      const res = await api.get("/api/calendar");
      setHolidays(res.data?.data || []);
    } catch {
      setMessage("加载节假日失败");
    }
  };

  const saveSettings = async (patch: Record<string, any>, okMsg: string) => {
    try {
      await api.put("/api/settings", patch);
      setMessage(okMsg);
    } catch (err: any) {
      setMessage(err?.response?.data?.error?.message || "保存失败");
    }
  };

  const testEmail = async () => {
    const to = window.prompt("输入测试收件邮箱：");
    if (!to) return;
    try {
      await api.post("/api/settings/test-email", { to });
      setMessage("测试邮件已发送");
    } catch (err: any) {
      setMessage("发送失败: " + (err?.response?.data?.error?.message || ""));
    }
  };

  return (
    <div className="page-container">
      <h1 className="page-title">系统设置</h1>
      <div className="settings-grid">
        {/* SMTP */}
        <div className="card">
          <h2 className="card-title">邮件通知（SMTP）</h2>
          <div className="form-group">
            <label className="form-label">服务器地址</label>
            <input className="form-input" value={smtp.smtp_host}
              onChange={(e) => setSmtp({ ...smtp, smtp_host: e.target.value })} />
          </div>
          <div className="form-group">
            <label className="form-label">端口</label>
            <input className="form-input" value={smtp.smtp_port}
              onChange={(e) => setSmtp({ ...smtp, smtp_port: e.target.value })} />
          </div>
          <div className="form-group">
            <label className="form-label">发件人</label>
            <input className="form-input" value={smtp.smtp_sender}
              onChange={(e) => setSmtp({ ...smtp, smtp_sender: e.target.value })} />
          </div>
          <div className="form-group">
            <label className="form-label">认证用户名（留空 = 无需登录）</label>
            <input className="form-input" value={smtp.smtp_username}
              onChange={(e) => setSmtp({ ...smtp, smtp_username: e.target.value })} />
          </div>
          <div className="form-group">
            <label className="form-label">认证密码</label>
            <input className="form-input" type="password" value={smtp.smtp_password}
              onChange={(e) => setSmtp({ ...smtp, smtp_password: e.target.value })} />
          </div>
          <div className="form-actions">
            <button className="btn btn-primary" onClick={() => saveSettings(smtp, "SMTP 配置已保存")}>保存</button>
            <button className="btn btn-ghost" onClick={testEmail}>测试发送</button>
          </div>
        </div>

        {/* 财年 + 密码策略 */}
        <div className="card">
          <h2 className="card-title">财年与密码</h2>
          <div className="form-group">
            <label className="form-label">财年起始月份</label>
            <select className="form-input" value={fiscalStartMonth}
              onChange={(e) => setFiscalStartMonth(Number(e.target.value))}>
              {[1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12].map((m) => (
                <option key={m} value={m}>{m} 月起始</option>
              ))}
            </select>
          </div>
          <div className="form-group">
            <label className="form-label">密码最小长度</label>
            <input className="form-input" type="number" min={6} max={32} value={passwordMinLength}
              onChange={(e) => setPasswordMinLength(Number(e.target.value))} />
          </div>
          <div className="form-actions">
            <button className="btn btn-primary"
              onClick={() => saveSettings({ fiscal_start_month: fiscalStartMonth, password_min_length: passwordMinLength }, "财年与密码设置已保存")}>
              保存
            </button>
          </div>
        </div>

        {/* 节假日 */}
        <div className="card">
          <h2 className="card-title">节假日（排程自动排除）</h2>
          <div className="form-group form-row">
            <input className="form-input" type="date" value={holidayDate}
              onChange={(e) => setHolidayDate(e.target.value)} />
            <input className="form-input" placeholder="名称（如：春节）" value={holidayLabel}
              onChange={(e) => setHolidayLabel(e.target.value)} />
            <button className="btn btn-primary" onClick={async () => {
              if (!holidayDate) { setMessage("请选择日期"); return; }
              try {
                await api.post("/api/calendar", { date: holidayDate, type: "holiday", label: holidayLabel });
                setHolidayDate(""); setHolidayLabel("");
                fetchHolidays();
                setMessage("节假日已添加");
              } catch (err: any) {
                setMessage(err?.response?.data?.error?.message || "添加失败");
              }
            }}>新增</button>
          </div>
          <ul className="holiday-list">
            {holidays.map((h) => (
              <li key={h.id} className="dep-item">
                <span className="dep-item-main">
                  <span className="dep-item-name">{h.date}{h.label ? ` · ${h.label}` : ""}</span>
                </span>
                <button className="btn btn-ghost btn-sm" onClick={async () => {
                  try {
                    await api.delete(`/api/calendar/${h.id}`);
                    fetchHolidays();
                  } catch (err: any) {
                    setMessage(err?.response?.data?.error?.message || "删除失败");
                  }
                }}>删除</button>
              </li>
            ))}
            {holidays.length === 0 && <li className="text-secondary">暂无节假日</li>}
          </ul>
        </div>
      </div>
      {message && <div className="form-error">{message}</div>}
    </div>
  );
}
```

（`page-container`/`page-title`/`card`/`card-title`/`form-*`/`dep-item` 类按现有样式命名对齐；若实际类名不同以现有页面为准。）

- [ ] **Step 3: 路由 + 导航**

`frontend/src/App.tsx`：

```tsx
import SystemSettings from "./pages/SystemSettings";
// 路由表中加（仅管理员可见由 Navbar 入口控制 + 页面内若需要可再校验）：
// <Route path="/admin/settings" element={<SystemSettings />} />
```

`frontend/src/components/Navbar.tsx`（管理员导航区，用户管理链接旁）：

```tsx
        {isLoggedIn && user?.is_admin && (
          <Link to="/admin/users" className="btn btn-link">
            用户管理
          </Link>
        )}
        {isLoggedIn && user?.is_admin && (
          <Link to="/admin/settings" className="btn btn-link">
            系统设置
          </Link>
        )}
```

（放在右侧 navbar-actions 组内「用户管理」之后、「退出」之前。）

- [ ] **Step 4: 类型检查**

Run: `cd F:\projects\followITup\frontend && npx tsc --noEmit`
Expected: 无类型错误

- [ ] **Step 5: 提交**

```bash
git add frontend/src/pages/SystemSettings.tsx frontend/src/App.tsx frontend/src/components/Navbar.tsx frontend/src/stores/settingsStore.ts frontend/src/pages/Dashboard.tsx
git commit -m "前端:系统配置页(邮件SMTP+测试发送/财年起始月/节假日管理/密码长度)+导航入口+财年读取迁移(首页仅可切换财年,不可修改起始月)"
```

---

### Task 8: 前端用户管理页改造（创建表单 + 删除 + 角色按钮 + 权限放开）

**Files:**
- Modify: `frontend/src/pages/UserManagement.tsx`
- Modify: `frontend/src/components/Navbar.tsx`（「用户管理」入口对所有登录用户可见）

**Interfaces:**
- Consumes: Task 3 的 `POST /api/users`（响应 `{message, initial_password?, mail_sent}`）、Task 4 的 `DELETE /api/admin/users/{id}`、`PUT /api/admin/users/{id}/role`、`GET /api/settings`（password_min_length）
- Produces: 改造后的用户管理页

- [ ] **Step 1: 创建表单改造**

`frontend/src/pages/UserManagement.tsx`：
1. 移除 `password` state 与输入框
2. 创建请求改：

```tsx
      const res = await api.post("/api/admin/users", {
        email: email.trim(),
        display_name: displayName.trim() || undefined,
        is_admin: isAdminChecked,
      });
      setMessage(res.data?.data?.initial_password
        ? `${res.data.data.message}（初始密码：${res.data.data.initial_password}）`
        : res.data.data.message);
```

3. 「设为管理员」勾选框（仅当前用户 is_admin 时渲染）：

```tsx
        {isAdmin && (
          <label className="form-label checkbox-label">
            <input type="checkbox" checked={isAdminChecked}
              onChange={(e) => setIsAdminChecked(e.target.checked)} />
            设为管理员
          </label>
        )}
```

4. 表单顶部提示：密码由系统随机生成，会发送至用户邮箱。

- [ ] **Step 2: 列表加删除与角色按钮（仅管理员）**

用户列表每行（现有 `<td>{u.is_admin ? "管理员" : "成员"}</td>` 之后）追加：

```tsx
                {isAdmin && (
                  <td className="actions-cell">
                    <button className="btn btn-ghost btn-sm"
                      onClick={() => handleRole(u)}>
                      {u.is_admin ? "取消管理员" : "设为管理员"}
                    </button>
                    <button className="btn btn-ghost btn-sm danger-text"
                      onClick={() => handleDelete(u)}>
                      删除
                    </button>
                  </td>
                )}
```

及 handler：

```tsx
  const handleRole = async (u: User) => {
    try {
      await api.put(`/api/admin/users/${u.id}/role`, { is_admin: !u.is_admin });
      fetchUsers();
    } catch (err: any) {
      alert(err?.response?.data?.error?.message || "操作失败");
    }
  };

  const handleDelete = async (u: User) => {
    if (!confirm(`确认删除用户「${u.display_name || u.email}」？\n\n历史项目和任务上的名字保留备查。`)) return;
    try {
      await api.delete(`/api/admin/users/${u.id}`);
      fetchUsers();
    } catch (err: any) {
      alert(err?.response?.data?.error?.message || "删除失败");
    }
  };
```

（`fetchUsers` 为页面现有刷新函数名，按实际命名对齐。）

- [ ] **Step 3: 导航放开**

`frontend/src/components/Navbar.tsx`：`用户管理` 链接条件 `user?.is_admin` → `isLoggedIn`（所有登录用户可见；页面内创建表单对所有人开放，删除/角色按钮按 isAdmin 条件渲染——Step 1/2 已处理）。

- [ ] **Step 4: 类型检查**

Run: `cd F:\projects\followITup\frontend && npx tsc --noEmit`
Expected: 无类型错误

- [ ] **Step 5: 提交**

```bash
git add frontend/src/pages/UserManagement.tsx frontend/src/components/Navbar.tsx
git commit -m "前端:用户管理页改造(创建表单去密码+仅管理员可设管理员,列表加删除/角色按钮,页面入口放开)"
```

---

### Task 9: 前端首登强制改密流程

**Files:**
- Create: `frontend/src/pages/ChangePassword.tsx`
- Modify: `frontend/src/pages/Login.tsx`（登录响应 must_change_password → 跳转改密页）
- Modify: `frontend/src/App.tsx`（/change-password 路由）
- Modify: `frontend/src/api/client.ts`（403 FORCE_PASSWORD_CHANGE 统一跳转兜底）

**Interfaces:**
- Consumes: Task 5 的登录响应 `must_change_password`、`POST /api/auth/change-password` 响应 `{token}`
- Produces: 改密页 + 登录后跳转 + 403 兜底

- [ ] **Step 1: 改密页**

`frontend/src/pages/ChangePassword.tsx`：

```tsx
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import api from "../api/client";
import { useAuthStore } from "../stores/authStore";

export default function ChangePassword() {
  const navigate = useNavigate();
  const setToken = useAuthStore((s) => s.setToken);
  const [oldPassword, setOldPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (newPassword !== confirmPassword) {
      setError("两次输入的新密码不一致");
      return;
    }
    setSubmitting(true);
    try {
      const res = await api.post("/api/auth/change-password", {
        old_password: oldPassword,
        new_password: newPassword,
      });
      // 用新 token 替换（原 token 带首登标记）
      setToken(res.data?.data?.token);
      alert("密码修改成功");
      navigate("/");
    } catch (err: any) {
      setError(err?.response?.data?.error?.message || "修改失败");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="login-page">
      <div className="login-card">
        <h1 className="login-title">首次登录：请修改密码</h1>
        <p className="text-secondary">为保障账号安全，请设置你的新密码。</p>
        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label className="form-label">初始密码</label>
            <input className="form-input" type="password" required
              value={oldPassword} onChange={(e) => setOldPassword(e.target.value)} />
          </div>
          <div className="form-group">
            <label className="form-label">新密码</label>
            <input className="form-input" type="password" required
              value={newPassword} onChange={(e) => setNewPassword(e.target.value)} />
          </div>
          <div className="form-group">
            <label className="form-label">确认新密码</label>
            <input className="form-input" type="password" required
              value={confirmPassword} onChange={(e) => setConfirmPassword(e.target.value)} />
          </div>
          {error && <div className="form-error">{error}</div>}
          <button className="btn btn-primary btn-block" type="submit" disabled={submitting}>
            {submitting ? "提交中..." : "修改密码"}
          </button>
        </form>
      </div>
    </div>
  );
}
```

（`login-page`/`login-card`/`login-title` 等类按 Login.tsx 现有样式命名对齐；`setToken` 为 authStore 现有方法名，按实际命名对齐。）

- [ ] **Step 2: 登录跳转**

`frontend/src/pages/Login.tsx`：登录成功后处理：

```tsx
      if (res.data?.data?.must_change_password) {
        navigate("/change-password", { replace: true });
        return;
      }
```

（放在现有 token/user 存入 store 之后。）

- [ ] **Step 3: 路由**

`frontend/src/App.tsx`：

```tsx
import ChangePassword from "./pages/ChangePassword";
// <Route path="/change-password" element={<ChangePassword />} />
```

- [ ] **Step 4: 403 兜底（api client 拦截）**

`frontend/src/api/client.ts`：响应拦截器中，403 且 `error.code === "FORCE_PASSWORD_CHANGE"` 时跳转 `/change-password`（排除改密接口自身）：

```ts
      if (
        status === 403 &&
        error?.response?.data?.error?.code === "FORCE_PASSWORD_CHANGE" &&
        !config.url?.includes("/api/auth/change-password")
      ) {
        window.location.href = "/change-password";
      }
```

（放在现有错误处理逻辑处，按 client.ts 现有拦截器结构对齐；拦截器内导出的相关函数名以现有代码为准。）

- [ ] **Step 5: 类型检查**

Run: `cd F:\projects\followITup\frontend && npx tsc --noEmit`
Expected: 无类型错误

- [ ] **Step 6: 提交**

```bash
git add frontend/src/pages/ChangePassword.tsx frontend/src/pages/Login.tsx frontend/src/App.tsx frontend/src/api/client.ts
git commit -m "前端:首登强制改密(登录跳转改密页+403统一兜底+改密后新token续会话)"
```

---

### Task 10: assignee 下拉（TaskDetailModal）

**Files:**
- Modify: `frontend/src/components/TaskDetailModal.tsx`
- Modify: `frontend/src/pages/TaskListView.tsx`（若存在同类 assignee 输入则同步改，否则跳过）

**Interfaces:**
- Consumes: Task 3 的 `GET /api/users` → `{data: [{id, display_name, email}]}`
- Produces: assignee 从 datalist 改 select

- [ ] **Step 1: 加载用户列表**

`TaskDetailModal.tsx` 组件内：

```tsx
  const [userOptions, setUserOptions] = useState<{ id: number; display_name: string }[]>([]);

  useEffect(() => {
    api.get("/api/users").then((res) => {
      setUserOptions(res.data?.data || []);
    }).catch(() => {});
  }, []);
```

- [ ] **Step 2: datalist 改 select**

替换现有 assignee 输入（第 476-482 行附近 datalist）：

```tsx
            <select
              className="form-input"
              value={assignee}
              onChange={(e) => setAssignee(e.target.value)}
            >
              <option value="">未指派</option>
              {userOptions.map((u) => (
                <option key={u.id} value={u.display_name}>{u.display_name}</option>
              ))}
            </select>
```

（删除原 `<datalist id="assignee-list">` 及相关代码；value 仍存显示名文本，历史数据兼容。）

- [ ] **Step 3: 类型检查**

Run: `cd F:\projects\followITup\frontend && npx tsc --noEmit`
Expected: 无类型错误

- [ ] **Step 4: 提交**

```bash
git add frontend/src/components/TaskDetailModal.tsx
git commit -m "前端:任务负责人改为用户下拉(数据源GET /api/users,存显示名兼容历史)"
```

---

### Task 11: 全量验证回归

**Files:** 无新增

- [ ] **Step 1: 后端全量测试 + 前端类型检查**

Run: `cd F:\projects\followITup\backend && go test ./... && cd ../frontend && npx tsc --noEmit`
Expected: 全部 PASS、无类型错误

- [ ] **Step 2: 影响范围检查**

Run: `gitnexus_detect_changes`（repo: followITup, scope: all）
Expected: 变更集中在 settings/mail/auth 相关文件与前端配置页/用户管理/登录流程；无意外 HIGH/CRITICAL

- [ ] **Step 3: 构建 + 浏览器验证**

```bash
cd frontend && npm run build
rm -rf ../backend/cmd/server/frontend-dist && cp -r dist ../backend/cmd/server/frontend-dist
cd ../backend && go build -o followitup.exe ./cmd/server/
```

重启服务器（查 8080 PID → `taskkill //F //PID <pid>` → 后台启动新 exe → curl 登录确认）。

浏览器验证清单：
1. 配置页（管理员）：SMTP 填写 + 测试发送（qorvo 内网 smtprelay 可达则应收到邮件，失败则 alert 错误信息）；财年起始月修改 → Dashboard 财年标签跟随；**首页已无财年起始月选择器**（仅「📅 财年/🗓 自然年」切换按钮）
2. 创建用户（普通用户身份）：`new.user@qorvo.com` → 显示名自动「New User」→ 收到邮件（含初始密码）→ 用初始密码登录 → 强制跳改密页 → 改密后进入首页
3. 未改密用户直接访问其他页面 → 403 拦截（直接输 URL 验证）
4. 普通用户创建用户时无「设为管理员」勾选；创建的用户非管理员
5. 管理员：提升普通用户为管理员 → 获得管理权限；降级最后一名管理员 → 400「系统至少保留一名管理员」
6. 删除普通用户 → 列表消失、assignee 下拉消失、历史任务留名、项目成员页无该用户
7. 删除管理员 → 400「请先取消其管理员身份」；删除自己 → 400
8. assignee 下拉：任务弹窗从用户列表选择
9. 节假日：配置页新增/删除 → 排程排除该日
10. 旧功能回归：登录、看板、甘特图、任务增删改不报错

- [ ] **Step 4: 提交**

```bash
git add -A
git commit -m "验证:用户管理升级浏览器回归通过"
```

（若验证发现问题，按 bug 流程先修再提交，并记录 .wolf/buglog.json）
