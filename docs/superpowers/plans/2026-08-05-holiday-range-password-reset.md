# 节假日范围管理 + 管理员密码重置实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 节假日支持范围批量添加（含补班 workday）、管理员重置用户密码（可选须首登改密）、系统配置页紧凑化。

**Architecture:** 扩展 `POST /api/calendar` 接受起止日期逐日展开；新增 `POST /api/admin/users/{id}/reset-password` 复用随机密码/邮件/明文回退模式；前端配置页改范围输入 + 紧凑网格布局，用户管理页加重置密码弹窗。排程引擎 `IsWorkDay` 已支持 workday，零改动。

**Tech Stack:** Go 1.22+（chi/SQLite）/ React 18 TypeScript / Vite

## Global Constraints

- 不引入新依赖；注释、提交信息使用简体中文
- 改符号前 `gitnexus_impact({target, direction:"upstream"})`，HIGH/CRITICAL 风险先告知用户
- 提交前 `gitnexus_detect_changes()` 验证影响范围
- 后端每步跑 `go build ./...` + `go test ./...`；前端每步跑 `npx tsc --noEmit`
- 每个逻辑变更单独提交（中文提交信息）
- 设计依据：`docs/superpowers/specs/2026-08-05-holiday-range-password-reset-design.md`（用户确认 6 条决策）
- 随机密码不落日志；邮件失败不阻塞主流程
- `POST /api/calendar` 单日调用（无 end_date）必须与现有行为兼容

---

### Task 1: 后端节假日范围添加 API + 补班单测

**Files:**
- Modify: `backend/internal/api/calendar.go`（Create 扩展 + 范围展开）
- Test: `backend/internal/scheduler/calendar_test.go`（补班显式用例，若文件不存在则创建）

**Interfaces:**
- Produces: `POST /api/calendar` 接受 `{start_date, end_date?, type?, label?}` → `{message: "已添加 N 天", count: N}`；校验：start 必填、end 缺省=start、start>end→400、范围>366 天→400

- [ ] **Step 1: 改造 Create handler**

`backend/internal/api/calendar.go` 替换 Create（第 67-87 行）：

```go
func (h *CalendarHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
		Type      string `json:"type"`
		Label     string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.StartDate == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "缺少 start_date 字段")
		return
	}
	// 兼容单日调用：end_date 缺省 = start_date
	end := req.EndDate
	if end == "" {
		end = req.StartDate
	}
	if req.StartDate > end {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "结束日期不能早于开始日期")
		return
	}
	// 单次范围不能超过一年（366 天）
	days := countDays(req.StartDate, end)
	if days > 366 {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "单次范围不能超过一年")
		return
	}
	if req.Type == "" {
		req.Type = "holiday"
	}
	if req.Type != "holiday" && req.Type != "workday" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "type 仅支持 holiday 或 workday")
		return
	}
	// 按日展开写入
	for d := req.StartDate; d <= end; d = nextDay(d) {
		if _, err := h.db.Exec("INSERT OR REPLACE INTO calendar (date, type, label) VALUES (?, ?, ?)",
			d, req.Type, req.Label); err != nil {
			writeError(w, http.StatusInternalServerError, "DB_ERROR", "添加日历条目失败")
			return
		}
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message": fmt.Sprintf("已添加 %d 天", days),
		"count":   days,
	})
}

// countDays 计算 start~end 含首尾的天数（YYYY-MM-DD 字符串）
func countDays(start, end string) int {
	var sy, sm, sd, ey, em, ed int
	fmt.Sscanf(start, "%d-%d-%d", &sy, &sm, &sd)
	fmt.Sscanf(end, "%d-%d-%d", &ey, &em, &ed)
	days := 0
	for !(sy == ey && sm == em && sd == ed) {
		days++
		sd++
		dim := daysInMonth(sy, sm)
		if sd > dim {
			sd = 1
			sm++
			if sm > 12 {
				sm = 1
				sy++
			}
		}
	}
	return days + 1
}

// nextDay 返回 date 的次日（YYYY-MM-DD）
func nextDay(date string) string {
	var y, m, d int
	fmt.Sscanf(date, "%d-%d-%d", &y, &m, &d)
	d++
	dim := daysInMonth(y, m)
	if d > dim {
		d = 1
		m++
		if m > 12 {
			m = 1
			y++
		}
	}
	return fmt.Sprintf("%04d-%02d-%02d", y, m, d)
}

// daysInMonth 返回某月天数（用于范围展开）
func daysInMonth(y, m int) int {
	switch m {
	case 1, 3, 5, 7, 8, 10, 12:
		return 31
	case 4, 6, 9, 11:
		return 30
	}
	// 2 月：闰年判断
	if (y%4 == 0 && y%100 != 0) || y%400 == 0 {
		return 29
	}
	return 28
}
```

（import 补充：`fmt`。注意：`daysInMonth` 与 scheduler 包同名函数是不同包，无冲突。）

- [ ] **Step 2: 补班单测**

`backend/internal/scheduler/calendar_test.go`（若已存在则追加）：

```go
func TestIsWorkDayWorkdayType(t *testing.T) {
	// 2026-08-08 是周六，标记补班后应为工作日
	cal := map[string]string{"2026-08-08": "workday", "2026-08-09": "holiday"}
	if !IsWorkDay(cal, "2026-08-08") {
		t.Error("补班日(workday)应视为工作日")
	}
	if IsWorkDay(cal, "2026-08-09") {
		t.Error("假日(holiday)不应视为工作日")
	}
	if !IsWorkDay(cal, "2026-08-07") {
		t.Error("普通周五应为工作日")
	}
	if !IsWorkDay(cal, "2026-08-10") {
		t.Error("普通周一应为工作日")
	}
}
```

- [ ] **Step 3: 编译 + 全量测试**

Run: `cd F:\projects\followITup\backend && go build ./... && go test ./...`
Expected: 编译通过、全部 PASS（含新增补班用例）

- [ ] **Step 4: 提交**

```bash
git add backend/internal/api/calendar.go backend/internal/scheduler/calendar_test.go
git commit -m "后端:节假日范围批量添加(起止日期逐日展开,兼容单日,上限366天)+补班workday单测"
```

---

### Task 2: 管理员重置密码端点 + 重置邮件

**Files:**
- Modify: `backend/internal/mail/mail.go`（SendPasswordReset）
- Modify: `backend/internal/api/auth.go`（路由 + ResetUserPassword handler）

**Interfaces:**
- Consumes: `auth.GenerateRandomPassword`、`settings.GetInt(db, KeyPasswordMinLen, 8)`、`mail.SendPasswordReset`
- Produces: `POST /api/admin/users/{id}/reset-password`（AdminOnly）`{must_change?: bool 默认 true}` → `{message, initial_password?, mail_sent}`；404「用户不存在」；400「不能重置自己的密码」？——不，管理员可重置自己？设计未禁止，但重置自己=改自己密码的场景走普通改密即可。**设计决策：允许重置任意用户（含自己）**，但保留「不能删除自己」的既有规则不动。handler 校验目标存在且 active。

- [ ] **Step 1: 重置邮件函数**

`backend/internal/mail/mail.go` 追加：

```go
// SendPasswordReset 发送密码重置通知（含新密码）
func SendPasswordReset(db *sql.DB, to, displayName, password string) error {
	subject := "FollowITup 密码已重置"
	body := fmt.Sprintf(`你好，%s：

你的 FollowITup 密码已被管理员重置，请使用以下信息登录：
  邮箱：%s
  新密码：%s

如果管理员勾选了"下次登录时须更改密码"，首次登录后系统会要求你修改。

—— FollowITup 系统通知`, displayName, to, password)
	return Send(db, to, subject, body)
}
```

- [ ] **Step 2: handler + 路由**

`backend/internal/api/auth.go`：

路由（AdminOnly 组内，DeleteUser 旁）：

```go
	r.Post("/api/admin/users/{id}/reset-password", withAuth(h.mid, h.AdminOnly(h.ResetUserPassword)))
```

handler：

```go
// ResetUserPassword 管理员重置用户密码（可选要求首登改密，默认要求）
func (h *AuthHandler) ResetUserPassword(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	var req struct {
		MustChange *bool `json:"must_change"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "请求格式错误")
		return
	}
	// 缺省 = 要求下次登录修改（与 Windows 账号管理一致）
	mustChange := true
	if req.MustChange != nil {
		mustChange = *req.MustChange
	}

	var displayName, email string
	err := h.svc.DB().QueryRow(
		`SELECT display_name, email FROM users WHERE id = ? AND is_active = 1`, id).Scan(&displayName, &email)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "用户不存在")
		return
	}

	minLen := settings.GetInt(h.svc.DB(), settings.KeyPasswordMinLen, 8)
	password := auth.GenerateRandomPassword(minLen)
	if err := h.svc.ChangePasswordDirect(id, password, mustChange); err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "重置密码失败")
		return
	}
	// 发送重置通知；失败不阻塞，明文回退给管理员
	mailErr := mail.SendPasswordReset(h.svc.DB(), email, displayName, password)
	if mailErr != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"message":          "密码已重置（邮件发送失败，请手动告知新密码）",
			"initial_password": password,
			"mail_sent":        false,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":   "密码已重置，新密码已发送至邮箱",
		"mail_sent": true,
	})
}
```

- [ ] **Step 3: service 直改密码方法**

`backend/internal/auth/auth.go` 追加：

```go
// ChangePasswordDirect 管理员直接设置用户密码（跳过旧密码校验）
func (s *Service) ChangePasswordDirect(userID int64, newPassword string, mustChange bool) error {
	if len(newPassword) < 8 {
		return errors.New("新密码长度不能少于 8 位")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), s.bcryptCost)
	if err != nil {
		return fmt.Errorf("密码哈希失败: %w", err)
	}
	_, err = s.db.Exec(
		"UPDATE users SET password_hash = ?, must_change_password = ?, updated_at = datetime('now') WHERE id = ?",
		string(hash), boolToInt(mustChange), userID)
	return err
}
```

- [ ] **Step 4: 编译 + 全量测试**

Run: `cd F:\projects\followITup\backend && go build ./... && go test ./...`
Expected: 编译通过、全部 PASS

- [ ] **Step 5: 提交**

```bash
git add backend/internal/mail/mail.go backend/internal/api/auth.go backend/internal/auth/auth.go
git commit -m "后端:管理员重置密码(随机密码+可选首登改密,默认勾选,邮件通知,失败回退明文)"
```

---

### Task 3: 前端系统配置页紧凑化 + 节假日范围管理

**Files:**
- Modify: `frontend/src/pages/SystemSettings.tsx`

**Interfaces:**
- Consumes: Task 1 的 `POST /api/calendar`（start_date/end_date/type/label → `{message, count}`）
- Produces: 紧凑布局 + 节假日范围添加区（起止日期 + 名称 + 类型下拉）+ 列表类型徽标

- [ ] **Step 1: 紧凑化布局**

`frontend/src/pages/SystemSettings.tsx` 调整：
- 外层容器 `maxWidth: 960` → `maxWidth: 1100`；两列 grid `gap: 20` → `gap: 14`
- 卡片统一内边距 `padding: 20` → `padding: 14`；标题 `marginBottom: 16` → `marginBottom: 10`，字号 `16 → 15`
- SMTP 卡片字段改 **2 列网格**（`display: grid; gridTemplateColumns: "1fr 1fr"; gap: 0 14px`）：
  - 行 1：服务器地址 + 端口
  - 行 2：发件人（占满一列）→ 改为与认证用户名同行：发件人 + 认证用户名
  - 行 3：认证密码（占满）
  - 操作行：保存 + 测试发送
- 财年与密码卡片：财年起始月 + 密码最小长度**并排一行**（`display: grid; gridTemplateColumns: "1fr 1fr"; gap: 0 14px`）
- `inputStyle` 不变（字段本身高度紧凑）

- [ ] **Step 2: 节假日卡片改范围添加**

替换节假日卡片内容（现有 日期/名称 单行 + 新增按钮 改为）：

```tsx
        {/* 节假日（排程自动排除/补班） */}
        <div style={{ background: "var(--card-bg)", border: "1px solid var(--card-border)", borderRadius: 8, padding: 14, marginTop: 14 }}>
          <h3 style={{ fontSize: 15, fontWeight: 600, marginBottom: 10 }}>节假日与补班（排程自动排除 / 周末补班计工作日）</h3>
          <div className="form-row" style={{ gap: 8 }}>
            <div className="form-group" style={{ marginBottom: 8 }}>
              <label style={{ fontSize: 13 }}>开始日期</label>
              <input style={inputStyle} type="date" value={holidayStart}
                onChange={(e) => setHolidayStart(e.target.value)} />
            </div>
            <div className="form-group" style={{ marginBottom: 8 }}>
              <label style={{ fontSize: 13 }}>结束日期</label>
              <input style={inputStyle} type="date" value={holidayEnd}
                onChange={(e) => setHolidayEnd(e.target.value)} />
            </div>
            <div className="form-group" style={{ marginBottom: 8 }}>
              <label style={{ fontSize: 13 }}>类型</label>
              <select style={inputStyle} value={holidayType}
                onChange={(e) => setHolidayType(e.target.value)}>
                <option value="holiday">假日（排除工作日）</option>
                <option value="workday">补班（周末计工作日）</option>
              </select>
            </div>
            <div className="form-group" style={{ marginBottom: 8 }}>
              <label style={{ fontSize: 13 }}>名称</label>
              <input style={inputStyle} placeholder="如：春节" value={holidayLabel}
                onChange={(e) => setHolidayLabel(e.target.value)} />
            </div>
            <div className="form-group" style={{ marginBottom: 8, alignSelf: "flex-end" }}>
              <button className="btn btn-primary" onClick={addHolidayRange}>添加</button>
            </div>
          </div>
          <table className="task-table" style={{ marginTop: 8 }}>
            <thead>
              <tr>
                <th>日期</th>
                <th>类型</th>
                <th>名称</th>
                <th style={{ width: 80 }}>操作</th>
              </tr>
            </thead>
            <tbody>
              {holidays.map((h) => (
                <tr key={h.id}>
                  <td>{h.date}</td>
                  <td>
                    <span className="status-badge"
                      style={h.type === "workday"
                        ? { background: "rgba(8, 145, 178, 0.1)", color: "var(--accent)" }
                        : { background: "var(--surface-alt)", color: "var(--text-secondary)" }}>
                      {h.type === "workday" ? "补班" : "假日"}
                    </span>
                  </td>
                  <td>{h.label || "—"}</td>
                  <td>
                    <button className="btn btn-ghost btn-sm" onClick={async () => {
                      try {
                        await api.delete(`/api/calendar/${h.id}`);
                        fetchHolidays();
                      } catch (err: any) {
                        setMessage(err?.response?.data?.error?.message || "删除失败");
                      }
                    }}>删除</button>
                  </td>
                </tr>
              ))}
              {holidays.length === 0 && (
                <tr><td colSpan={4} className="text-secondary">暂无节假日</td></tr>
              )}
            </tbody>
          </table>
        </div>
```

state 与 handler 调整：

```tsx
  const [holidayStart, setHolidayStart] = useState("");
  const [holidayEnd, setHolidayEnd] = useState("");
  const [holidayType, setHolidayType] = useState("holiday");
  // 删除原 holidayDate state

  const addHolidayRange = async () => {
    if (!holidayStart) { setMessage("请选择开始日期"); return; }
    try {
      const res = await api.post("/api/calendar", {
        start_date: holidayStart,
        end_date: holidayEnd || undefined,
        type: holidayType,
        label: holidayLabel,
      });
      setMessage(res.data?.data?.message || "已添加");
      setHolidayStart(""); setHolidayEnd(""); setHolidayLabel("");
      fetchHolidays();
    } catch (err: any) {
      setMessage(err?.response?.data?.error?.message || "添加失败");
    }
  };
```

（`Holiday` 接口已有 `type` 字段 ✓；删除原 holidayDate 相关代码。）

- [ ] **Step 3: 类型检查**

Run: `cd F:\projects\followITup\frontend && npx tsc --noEmit`
Expected: 无类型错误

- [ ] **Step 4: 提交**

```bash
git add frontend/src/pages/SystemSettings.tsx
git commit -m "前端:配置页紧凑化(1920一屏)+节假日范围添加(起止日期+假日/补班类型+徽标区分)"
```

---

### Task 4: 前端用户管理重置密码弹窗

**Files:**
- Modify: `frontend/src/pages/UserManagement.tsx`

**Interfaces:**
- Consumes: Task 2 的 `POST /api/admin/users/{id}/reset-password` `{must_change}`
- Produces: 每行「重置密码」按钮 + 勾选弹窗 + 结果展示

- [ ] **Step 1: 重置弹窗与按钮**

`frontend/src/pages/UserManagement.tsx`：

state 追加：

```tsx
  const [resetTarget, setResetTarget] = useState<User | null>(null);
  const [resetMustChange, setResetMustChange] = useState(true);
  const [resetting, setResetting] = useState(false);
```

操作列（`handleRole` 按钮旁）加按钮：

```tsx
                    <button className="btn btn-ghost btn-sm"
                      onClick={() => { setResetMustChange(true); setResetTarget(u); }}>
                      重置密码
                    </button>{" "}
```

handler：

```tsx
  const handleResetPassword = async () => {
    if (!resetTarget) return;
    setResetting(true);
    try {
      const res = await api.post(`/api/admin/users/${resetTarget.id}/reset-password`, {
        must_change: resetMustChange,
      });
      const d = res.data?.data;
      setMessage(d?.initial_password
        ? `${d.message}（新密码：${d.initial_password}）`
        : d?.message || "密码已重置");
      setResetTarget(null);
    } catch (err: any) {
      alert(err?.response?.data?.error?.message || "重置失败");
      setResetTarget(null);
    } finally {
      setResetting(false);
    }
  };
```

弹窗（JSX 末尾，仿 modal-overlay/modal-card 现有模式——若页面无弹窗先例，用内联绝对定位覆盖层；项目中 TaskDetailModal 有 modal-overlay 样式可直接复用）：

```tsx
      {resetTarget && (
        <div className="modal-overlay" onClick={() => setResetTarget(null)}>
          <div className="modal-card" onClick={(e) => e.stopPropagation()}>
            <div className="modal-title">
              <h2>重置密码 · {resetTarget.display_name || resetTarget.email}</h2>
              <button className="btn btn-ghost btn-sm" onClick={() => setResetTarget(null)}>×</button>
            </div>
            <div className="modal-body">
              <p className="text-secondary" style={{ fontSize: 13, marginBottom: 12 }}>
                将生成随机新密码，通过邮件发送；邮件不可达时会在下方显示新密码。
              </p>
              <label style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 14 }}>
                <input type="checkbox" checked={resetMustChange}
                  onChange={(e) => setResetMustChange(e.target.checked)} />
                用户下次登录时须更改密码
              </label>
            </div>
            <div className="modal-actions">
              <button className="btn btn-ghost" onClick={() => setResetTarget(null)}>取消</button>
              <button className="btn btn-primary" disabled={resetting}
                onClick={handleResetPassword}>
                {resetting ? "重置中..." : "确认重置"}
              </button>
            </div>
          </div>
        </div>
      )}
```

- [ ] **Step 2: 类型检查**

Run: `cd F:\projects\followITup\frontend && npx tsc --noEmit`
Expected: 无类型错误

- [ ] **Step 3: 提交**

```bash
git add frontend/src/pages/UserManagement.tsx
git commit -m "前端:用户管理重置密码(每行按钮+须更改密码勾选弹窗,默认勾选)"
```

---

### Task 5: 全量验证回归

**Files:** 无新增

- [ ] **Step 1: 后端全量测试 + 前端类型检查**

Run: `cd F:\projects\followITup\backend && go test ./... && cd ../frontend && npx tsc --noEmit`
Expected: 全部 PASS、无类型错误

- [ ] **Step 2: 影响范围检查**

Run: `gitnexus_detect_changes`（repo: followITup, scope: all）
Expected: 变更集中在 calendar.go/auth.go/mail.go/SystemSettings/UserManagement；无意外 HIGH/CRITICAL

- [ ] **Step 3: 构建 + 重启 + 验证**

```bash
cd frontend && npm run build
rm -rf ../backend/cmd/server/frontend-dist && cp -r dist ../backend/cmd/server/frontend-dist
cd ../backend && go build -o followitup.exe ./cmd/server/
```

重启服务器（查 8080 PID → `taskkill //F //PID <pid>` → 后台启动 → curl 登录确认）。

**API 验证**：
1. 范围添加：`POST /api/calendar` `{start_date:"2027-01-01", end_date:"2027-01-03", type:"holiday", label:"元旦"}` → count=3 → 列表 3 条 → 逐条删除清理
2. 单日兼容：`{start_date:"2026-12-31"}` → 1 条 → 清理
3. 边界：start>end → 400；2027 全年范围 → 400
4. 补班：加 2026-08-08（周六）workday → 列表可见（type=workday）；`IsWorkDay` 单测已覆盖排程侧
5. 重置密码：创建测试用户 → 管理员重置（must_change=true）→ 旧密码登录失败 → 新密码登录成功且 `must_change_password=true` → 登录后改密页；再重置（must_change=false）→ 新密码登录直接可用 → 删除测试用户

**浏览器验证**：
1. 配置页：范围添加 3 天 → 列表 3 条；加补班（周六）→ 蓝色「补班」徽标；删除清理
2. 1920 分辨率一屏无滚动条（页面总高低于视口）
3. 用户管理：管理员重置测试用户（勾选须更改）→ 提示 → 该用户新密码登录 → 跳改密页
4. 重置不勾选 → 登录直接进首页
5. 旧功能回归：配置页 SMTP 保存、财年保存、用户创建不报错

- [ ] **Step 4: 提交**

```bash
git add -A
git commit -m "验证:节假日范围+密码重置回归通过"
```

（若验证发现问题，按 bug 流程先修再提交，并记录 .wolf/buglog.json）
