# 节假日范围管理 + 管理员密码重置设计

日期：2026-08-05
状态：待用户复审

## 背景与目标

两处改进：
1. 系统配置页节假日管理：当前只能一天一天添加，改为**范围添加**（起止日期），并支持**补班**（长假期间周末调休为工作日，type=`workday`）。排程引擎 `IsWorkDay` 已完整支持 workday（日历条目存在时按 type 判断：`holiday`=非工作日、`workday`=工作日），缺前端入口与范围添加 API。页面整体紧凑化（1920 分辨率一屏放下）。
2. 用户密码：管理员可**重置任意用户密码**，并可选择**是否要求下次登录时修改**（类似 Windows 账号管理「用户下次登录时须更改密码」）。

## 用户决策记录（已确认）

1. 节假日支持指定**时间范围**（起止日期 + 名称 + 类型：假日/补班）批量添加，而非逐日添加
2. 补班（type=`workday`）用于长假调休：周末补班日按工作日参与排程；引擎已支持，补齐 API 与前端
3. 范围添加跨年/长范围照常逐日展开（如 2027 春节）
4. 管理员可重置用户密码；重置弹窗带「用户下次登录时须更改密码」**勾选框（默认勾选）**，取消勾选 = 密码长期有效
5. 重置流程与建号一致：随机密码 → SMTP 邮件通知；发送失败时明文密码回退给管理员（不落日志）
6. 系统设置页整体**紧凑化**：字段多列网格、缩小间距，1920 屏一屏放下

## 现状（保持不变的部分）

- `calendar` 表：`id/date/type/label`，`date` UNIQUE，`INSERT OR REPLACE` 已用于单日添加
- 排程引擎 `scheduler/calendar.go`：`LoadCalendar` 返回 `map[date]type`；`IsWorkDay(cal, date)` 对日历条目按 `type=="workday"` 判断——**holiday 条目=非工作日、workday 条目=工作日**，引擎无需改动
- `POST /api/calendar`（RequireAuth）现有单日：`{date, type, label}`，type 缺省 `holiday`
- `DELETE /api/calendar/{id}` 已有
- 创建用户逻辑（随机密码/邮件/明文回退）在 `api/auth.go CreateUser`，重置复用同模式

## 后端设计

### 1. 节假日范围添加（扩展 `POST /api/calendar`）

请求体扩展：

```json
{
  "start_date": "2027-01-01",
  "end_date": "2027-01-03",   // 可缺省：缺省时等效单日添加（兼容现有调用）
  "type": "holiday",           // "holiday" | "workday"，缺省 "holiday"
  "label": "元旦"
}
```

- 校验：`start_date` 必填；`end_date` 缺省 = `start_date`；`start_date <= end_date` 否则 400；范围超 366 天 → 400「单次范围不能超过一年」
- 实现：按日循环（参照 `CountWorkDays` 的日期推进模式），逐日 `INSERT OR REPLACE INTO calendar (date, type, label)`；单日场景与现有行为一致
- 响应：`{message: "已添加 N 天", count: N}`

### 2. 管理员重置密码（新增 `POST /api/admin/users/{id}/reset-password`）

- 权限：AdminOnly（`r.Post("/api/admin/users/{id}/reset-password", withAuth(h.mid, h.AdminOnly(h.ResetUserPassword)))`）
- 请求：`{must_change: bool}`——缺省/省略按 `true`（要求下次登录修改）
- 逻辑：
  1. 查询目标 `id` 且 `is_active=1`，不存在 → 404「用户不存在」
  2. 随机密码（`settings.KeyPasswordMinLen` 长度，同建号）
  3. `UPDATE users SET password_hash=?, must_change_password=?, updated_at=datetime('now') WHERE id=?`
  4. 发重置通知邮件（`mail.SendPasswordReset(db, to, displayName, password)` 新函数，文案「密码已重置」）；失败 → 记日志，响应带 `initial_password` 明文
- 响应：`{message: "密码已重置，初始密码已发送至邮箱", mail_sent: true}` 或 `{message: "...（邮件发送失败，请手动告知初始密码）", initial_password: "...", mail_sent: false}`
- 已有会话 token 不主动失效（随机密码已使旧密码登录失效；JWT 时效 8 小时自然过期，可接受）

### 3. 邮件函数

`backend/internal/mail/mail.go` 新增：

```go
// SendPasswordReset 发送密码重置通知（含新密码）
func SendPasswordReset(db *sql.DB, to, displayName, password string) error
```

文案：「你的 FollowITup 密码已被管理员重置…新密码…（是否需修改由管理员设置）」。

## 前端设计

### 1. 系统配置页紧凑化 + 节假日范围管理（SystemSettings.tsx）

**紧凑化**（全局）：
- 卡片 padding `20 → 14`，卡片间距 `20 → 14`，标题下边距 `16 → 10`
- SMTP 5 个字段改 **2 列网格**（host+port 一行、username+password 一行、sender 占满），label 字号 13
- 财年与密码卡片：两字段并排一行
- 目标：1920×1080 一屏放下全部三张卡片

**节假日卡片改造**：
- 添加区改为一行：起止日期（2 个 date input）+ 名称 + 类型下拉（`假日` / `补班`）+「添加」按钮
- 列表：日期 + 类型徽标（假日=灰、补班=蓝）+ 名称 + 删除按钮；按日期升序
- 空态：「暂无节假日」
- 调用 `POST /api/calendar`（带 start_date/end_date/type/label）→ 成功提示「已添加 N 天」

### 2. 用户管理页重置密码（UserManagement.tsx）

- 每行操作区加「重置密码」按钮（仅管理员视角，`btn-ghost btn-sm`）
- 点击 → 弹窗（复用 modal-overlay/modal-card 样式）：
  - 标题「重置密码 · 显示名」
  - 勾选框「**用户下次登录时须更改密码**」（默认勾选）
  - 「取消 / 确认重置」按钮
- 确认 → `POST /api/admin/users/{id}/reset-password {must_change}` → 结果：
  - 邮件成功：页面 message「密码已重置，新密码已发送至邮箱」
  - 邮件失败：alert/页面 message 展示 `initial_password` 明文（管理员手动传达）
- 行内展示当前 `must_change_password` 状态？——用户列表接口不返回该字段，YAGNI 不做（重置弹窗本身就是明确操作）

## 测试计划

### 后端

- scheduler 单测：确认 `IsWorkDay` 对 `workday`（周末补班）返回 true、对 `holiday` 返回 false——若已有覆盖则补一个周末补班显式用例（`cal={"2026-08-08":"workday"}`（周六）→ true）
- 编译 + `go test ./...` 全过

### API 验证

1. `POST /api/calendar` 范围（2027-01-01 ~ 2027-01-03 holiday）→ 返回 count=3 → 列表 3 条 → 单日删除逐条清理
2. `POST /api/calendar` 单日（兼容旧调用）→ 1 条
3. 范围超 366 天 → 400；start > end → 400
4. 补班：加 2026-08-08（周六）workday → `GET /api/calendar` 可见；排程侧（单测验证 IsWorkDay）
5. 重置密码：管理员重置 test 用户（must_change=true）→ 旧密码登录失败 → 新密码登录成功且 `must_change_password=true` → 改密后可正常；再重置（must_change=false）→ 新密码登录直接可用

### 浏览器验证

1. 配置页：范围添加 3 天假日 → 列表出现 3 条；添加补班（周末）→ 徽标区分；删除清理
2. 配置页 1920 分辨率一屏无滚动条（或基本无滚动）
3. 用户管理：管理员重置某用户密码（勾选须更改）→ 提示邮件/明文 → 该用户用新密码登录 → 跳改密页
4. 重置不勾选 → 登录直接进首页

## 风险与边界

- 范围添加用 `INSERT OR REPLACE`：已存在的同日期条目被覆盖（含类型变更——补班覆盖假日等），符合直觉
- 日历表无体积风险（年范围最多 366 行）
- 重置密码不失效旧 token：8 小时自然过期，内网可接受
- 页面紧凑化只动 SystemSettings 布局，不影响其他页面

## 明确不做（YAGNI）

- 节假日批量删除/跨页分页（单条删除足够）
- 重复日期检测提示（OR REPLACE 静默覆盖）
- 重置密码操作审计日志（activity_log 是项目维度，用户操作不落）
- 周期性假日规则（如「每年 5 月 1 日」）——逐日展开即可
