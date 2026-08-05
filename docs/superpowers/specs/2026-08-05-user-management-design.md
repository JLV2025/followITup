# 用户管理升级设计（邮件通知 / 首登改密 / 权限模型 / 系统配置页）

日期：2026-08-05
状态：待用户复审

## 背景与目标

现状：创建用户仅管理员（AdminOnly）、手动填密码+显示名；`must_change_password` 字段已存在但未启用；assignee 为自由文本+datalist；财年设置在前端本地存储；节假日 CRUD 后端已有（`/api/calendar`）但无前端入口；无邮件通知能力。

目标：创建账号（邮箱即用户名 + 随机密码 + 邮件通知 + 首次登录强制改密）、显示名从邮箱自动推导、assignee 改为用户下拉、权限模型（全员可建号、管理员管删与角色）、统一系统配置页（SMTP/财年/节假日/密码策略）。

## 用户决策记录（已确认）

1. 账号创建：用户名必须是邮箱；密码随机生成；通过**内部 SMTP**（`smtprelay-west.corp.qorvo.com`，无需登录，发件人 `FollowITup@qorvo.com`）发送通知邮件；首次登录强制修改密码
2. **SMTP 配置不硬编码**——新增系统配置页面（仅管理员）：SMTP 设置、财年起始月设置、节假日新增/删除、以及其他必要的系统设置
3. 显示名推导：`first.last@qorvo.com` → `First Last`（点号拆分、各段首字母大写、空格拼接）；创建时可手动覆盖
4. 任务 owner（assignee）改为下拉列表选择用户
5. 权限：**所有登录用户可创建用户**；仅管理员可删除用户/提升降级管理员
6. 管理员规则：可有多个管理员；**管理员不可被删除**（先取消管理员身份）；管理员可提升/降级（降级需保证至少一名管理员）；**系统必须始终至少有一名管理员**；普通用户创建的用户固定为普通用户
7. 删除用户：软删（`is_active=0`，列表/下拉自动消失）；`project_members` 记录一并清理；**历史项目和任务上的用户名字符串保留**（assignee 存文本，留底备查）
8. 随机密码邮件明文发送可接受（内网 + 首登即改）；明文密码不落日志
9. 被删用户的已登录会话：登录时校验 `is_active=1`（现已是），JWT 短时效自然过期，不做黑名单
10. 不做 AD 同步增强（预期用户不多，本地创建足够；LDAP 登录能力保留现状）

## 现状（保持不变的部分）

- `users` 表：`login/email/display_name/password_hash/auth_source/is_admin/is_active/must_change_password` 字段齐全，`email` UNIQUE
- 登录走 `email` + 密码；`auth_source` 区分 `local`/`ldap`；LDAP 用户不受本地随机密码/首登改密机制影响
- 节假日 CRUD：`GET/POST /api/calendar`、`DELETE /api/calendar/{id}`（排程引擎已排除节假日），仅缺前端入口
- Dashboard 财年起始月：目前前端 store 本地存储（`fiscalStartMonth`），需迁移到系统配置

## 后端设计

### 1. settings 配置表 + API

```sql
CREATE TABLE IF NOT EXISTS settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```

预置 key（默认值）：

| key | 默认 | 说明 |
|---|---|---|
| `smtp_host` | `smtprelay-west.corp.qorvo.com` | SMTP 服务器 |
| `smtp_port` | `25` | 端口 |
| `smtp_sender` | `FollowITup@qorvo.com` | 发件人 |
| `smtp_username` | `` | 认证用户名（可空 = 无需登录） |
| `smtp_password` | `` | 认证密码 |
| `password_min_length` | `8` | 密码最小长度 |
| `fiscal_start_month` | `4` | 财年起始月份（1-12） |

API：
- `GET /api/settings`（RequireAuth 任意登录用户）→ 公开子集：`fiscal_start_month`、`password_min_length`（创建用户表单需要）
- `GET /api/settings/admin`（AdminOnly）→ 全量
- `PUT /api/settings`（AdminOnly）→ 批量更新，**仅接受预置 key 白名单**（未知 key 忽略/报错），响应全量
- `POST /api/settings/test-email`（AdminOnly）`{to: email}` → 用当前 SMTP 配置试发一封测试邮件，失败返回错误信息（便于排障）

### 2. 邮件服务 `backend/internal/mail/`

- 标准库 `net/smtp`，无新依赖
- `Send(to, subject, body)`：读 settings 的 SMTP 配置；`smtp_username` 为空 → 无认证 `smtp.SendMail`；非空 → `smtp.PlainAuth`
- 发送失败仅记日志，不影响主流程（创建用户仍成功，见下）
- 发送内容（纯文本）：账号创建通知（含初始密码、首登需改密提示）、SMTP 测试邮件

### 3. 创建用户改造（`POST /api/admin/users` → 权限放开 + 新逻辑）

- 路由：去掉 `AdminOnly` → 所有登录用户（withAuth 即可）
- 请求：`email`、`display_name`（可选，空则推导）、`is_admin`（可选）
- 服务端逻辑：
  1. 邮箱格式校验（基本格式正则：`x@y.z`）
  2. 显示名推导（空时）：local 部分按 `.` 拆分 → 各段首字母大写 → 空格拼接；local 无 `.` 时用原样；推导为空则用 email
  3. `is_admin`：**仅当当前用户 `is_admin` 且显式传 `true` 才设管理员**；普通用户传 `true` 强制忽略为 `false`
  4. 随机密码：`crypto/rand` 生成 12 位（大小写字母 + 数字 + 符号，至少含 3 类），**不落日志**
  5. 入库：`must_change_password=1`、`auth_source='local'`、`is_active=1`
  6. 发邮件（SMTP 配置存在时）；发送失败 → 记日志，响应携带 `initial_password` 明文（供管理员手动传达）；发送成功 → 响应不带明文
- 响应：`{message: "用户创建成功", initial_password?: string, mail_sent: boolean}`

### 4. 删除用户（`DELETE /api/admin/users/{id}`，AdminOnly 新增）

- 目标不存在或已停用 → 404「用户不存在」
- **目标 `is_admin=1` → 400「请先取消其管理员身份」**（管理员不可直接删除）
- 目标为操作者本人 → 400「不能删除自己」（管理员必被上一条拦截；普通用户无权限调用，规则兜底）
- 执行：`UPDATE users SET is_active=0, updated_at=datetime('now') WHERE id=?` + `DELETE FROM project_members WHERE user_id=?`
- 历史任务 assignee 为文本，天然保留（不做任何清理）
- 响应 200

### 5. 提升/降级管理员（`PUT /api/admin/users/{id}/role`，AdminOnly 新增）

- 请求：`{is_admin: bool}`
- 降级（`false`）保护：**若目标为当前唯一 active 管理员 → 400「系统至少保留一名管理员」**（保证始终有管理员）
- 提升（`true`）：任意 active 用户
- 执行：`UPDATE users SET is_admin=? WHERE id=? AND is_active=1`，影响 0 行 → 404
- 响应更新后的用户对象

### 6. 首登强制改密

- `service.Login`：查询增加 `must_change_password` 字段 → `LoginResponse` 增加 `must_change_password: bool`
- **JWT claims 增加 `must_change_password`**（登录签发时写入，避免每请求查库）
- auth 中间件（`withAuth`/`RequireAuth`）：解析出 claims 后，若 `must_change_password=true` 且请求路径不是 `POST /api/auth/change-password` → `403 FORCE_PASSWORD_CHANGE`「首次登录需先修改密码」
- `ChangePassword`：成功后 `UPDATE users SET must_change_password=0` + **重新签发 JWT**（无 flag）返回给前端（前端替换存储，避免旧 token 仍带 flag）
- 会话中改密成功 → 后续请求使用新 token，无拦截

### 7. 财年设置迁移

- `fiscal_start_month` 写入 settings（配置页修改）
- Dashboard 财年逻辑从 settings 读取（后端 `GET /api/settings` 公开子集），替换前端本地存储

## 前端设计

### 1. 系统配置页（`/admin/settings`，导航新增「系统设置」，管理员可见）

- SMTP 卡片：host/port/sender/username/password 表单 + 保存 + 「测试发送」按钮（弹出输入框填测试邮箱 → `POST /api/settings/test-email` → alert 结果）
- 财年卡片：起始月份下拉 → 保存（即时生效）
- 节假日卡片：现有 `/api/calendar` 接入——日期列表 + 新增（日期输入 + 名称）+ 删除
- 密码策略卡片：最小长度数字输入 → 保存
- 样式复用现有卡片/表单样式（btn/card/form-*）

### 2. 用户管理页改造（`/admin/users`）

- **创建用户表单**：对全部登录用户开放页面访问（去掉前端 AdminOnly 限制？——页面路由保持 `/admin/users`，但入口对所有登录用户可见，表单内「设为管理员」勾选框仅管理员可见）
- 用户列表每行（管理员视角）：「设为管理员/取消管理员」按钮（目标为最后一名管理员时后端 400 提示）+ 「删除」按钮（confirm 确认，文案「删除后历史任务留名备查」）
- 普通用户视角：列表只读 + 可创建用户

### 3. 首登强制改密流程

- 登录接口返回 `must_change_password: true` → 登录成功后跳转**改密页**（`/change-password`，新页面或复用登录页内嵌）
- 改密表单：旧密码（=初始随机密码）+ 新密码×2（校验最小长度）
- 成功后用响应中的新 token 更新 store → 跳转首页
- 后端 403 `FORCE_PASSWORD_CHANGE` 时前端也跳转改密页（兜底：直接携带旧 token 访问其他页时）

### 4. assignee 下拉

- **新增 `GET /api/users`**（RequireAuth）→ `{data: [{id, display_name, email}]}`（active 用户，不含 is_admin 等敏感字段）——现有 `/api/admin/users` 保持 AdminOnly 不动
- TaskDetailModal：datalist 改 select，选项 = 用户显示名（+「未指派」空选项），仍存显示名文本
- 被删用户从下拉消失；历史任务的 assignee 文本保留显示

## 测试计划

### 后端

- auth service 单测：显示名推导（`john.doe@qorvo.com`→`John Doe`、无点 local、空段）、随机密码长度/字符集、is_admin 权限校验（普通用户传 true 被忽略）、降级最后一名管理员被拒、删除管理员被拒
- 编译 + `go test ./...` 全过

### 浏览器验证清单

1. 配置页：SMTP 填写 + 测试发送（qorvo 内网 smtprelay 应可达，验证收到邮件）；财年起始月修改生效（Dashboard 财年标签跟随）
2. 创建用户（普通用户身份）：填 `new.user@qorvo.com` → 显示名自动「New User」→ 收到邮件（含初始密码）→ 用初始密码登录 → 强制跳改密页 → 改密后正常进入
3. 未改密用户访问其他页面 → 403 拦截（直接输 URL 验证）
4. 普通用户创建用户时无「设为管理员」勾选；创建的用户非管理员
5. 管理员：提升普通用户为管理员 → 该用户获得管理权限；降级最后一名管理员 → 400 提示
6. 删除：删除普通用户 → 列表消失、assignee 下拉消失、历史任务留名、project_members 清理（项目成员页无该用户）
7. 删除管理员 → 400「请先取消其管理员身份」
8. assignee 下拉：任务弹窗可从用户列表选择
9. 节假日：配置页新增/删除节假日 → 排程日期计算排除该日

## 风险与边界

- SMTP 在开发机可能不可达（内网限制）——测试发送失败时 alert 返回错误信息，不影响其他功能；上线配置页可再调
- 财年迁移影响 Dashboard 统计口径——保持默认 4 月不变，行为一致
- 中间件拦截白名单配置要准确（仅放行 `POST /api/auth/change-password`），漏放行会锁死新用户——浏览器验证覆盖
- 随机密码发送失败时明文返回给创建者（非落日志），管理员手动传达
- LDAP 用户（若启用）不受本地随机密码/首登改密机制影响（认证走 AD）
- 删除用户为软删且无回收站（`is_active=0` 不可恢复，管理员操作前有 confirm）——与任务/项目回收站不同，账号删除是一次性操作，需确认后执行

## 明确不做（YAGNI）

- AD/LDAP 同步增强（用户决策 10：本地创建足够）
- 用户回收站（账号删除不可恢复）
- 密码过期策略/复杂度分级（仅最小长度）
- 多语言/主题等系统配置（需要时再加）
- 邮件模板自定义（固定纯文本模板）
