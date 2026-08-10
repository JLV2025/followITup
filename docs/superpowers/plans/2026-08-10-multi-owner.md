# 任务与项目多负责人(multi-owner)实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 任务与项目都支持多个负责人(关联表存储),我的待办支持「我负责的任务/我负责的项目」双视角,看板卡片改三行布局。

**Architecture:** 新建 `task_assignees` / `project_owners` 关联表(user_id 外键,主键去重)作为权威数据,`tasks.assignee` / `projects.owner` 列保留为快照(写路径同步维护,读路径以关联表为准)。写接口接受 `assignee_ids`/`owner_ids` 数组并兼容旧文本;`GetMyTasks` 加 `view=task|project` 参数;前端抽 MultiUserSelect 多选组件,各页面接入。

**Tech Stack:** Go 1.22+ / modernc.org/sqlite / React 18+TS / react-i18next / dhtmlx-gantt

## Global Constraints

- 编辑权限保持不变:任何登录用户可编辑任何任务,不引入权限体系
- 负责人分隔符统一为分号 `;`(展示、CSV、文本兼容解析全部一致)
- 去重由关联表主键兜底;重复选择同一用户只保留一个
- 快照列(`assignee`/`owner`)写路径同步维护为分号分隔显示名,读路径权威是关联表
- 软删除不清理关联表(回收站恢复后负责人自动回来)
- 项目 owner 变更 → 未开始(open/delayed)任务负责人自动改派为项目全部 owner(沿用现状语义)
- 所有对话、注释、提交信息使用简体中文
- 每次构建严格遵守:`cd frontend && npm run build` → 复制 dist 到 `backend/cmd/server/frontend-dist` → 杀进程 → `cd backend && go build`(见 Do-Not-Repeat)
- 后端测试:`cd backend && go test ./...`;前端类型:`cd frontend && npx tsc --noEmit`(分号分隔,勿用管道取退出码)

---

### Task 1: 迁移 v9(关联表 + 存量数据迁移)+ 测试

**Files:**
- Modify: `backend/internal/db/sqlite.go`(migrations 数组末尾追加)
- Test: `backend/internal/db/migration_test.go`(新建)

**Interfaces:**
- Produces: `task_assignees` / `project_owners` 两张表(主键去重),存量 `tasks.assignee` / `projects.owner` 文本解析为 user_id 并回填快照列

- [ ] **Step 1: 写失败测试**(断言 v9 迁移能把存量文本解析进关联表)

`backend/internal/db/migration_test.go`(新建):

```go
package db

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// TestMigrateV9MultiOwner 存量 assignee/owner 文本 → 关联表,email 优先于 display_name;重名取 id 最小;快照列回填分号文本
func TestMigrateV9MultiOwner(t *testing.T) {
	conn, err := sql.Open("sqlite", ":memory:?_journal_mode=WAL")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// 手动建出"迁移前"的 schema(users/tasks/projects 相关列)并插存量数据
	for _, stmt := range []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, email TEXT UNIQUE, display_name TEXT, is_active INTEGER DEFAULT 1)`,
		`CREATE TABLE tasks (id INTEGER PRIMARY KEY AUTOINCREMENT, project_id INTEGER, assignee TEXT, deleted_at TEXT, name TEXT, status TEXT)`,
		`CREATE TABLE projects (id INTEGER PRIMARY KEY AUTOINCREMENT, owner TEXT, deleted_at TEXT)`,
		`INSERT INTO users (email, display_name) VALUES ('a@x.com', '张三')`,
		`INSERT INTO users (email, display_name) VALUES ('b@x.com', '李四')`,
		`INSERT INTO users (email, display_name) VALUES ('c@x.com', '王五')`,
		`INSERT INTO users (email, display_name) VALUES ('d@x.com', '张三')`, // 重名:id=4
		`INSERT INTO tasks (id, project_id, assignee) VALUES (1, 1, '张三')`,   // display_name 命中,重名取 id 最小 → user 1
		`INSERT INTO tasks (id, project_id, assignee) VALUES (2, 1, 'b@x.com')`, // email 命中 → user 2
		`INSERT INTO tasks (id, project_id, assignee) VALUES (3, 1, '不存在的人')`, // 解析失败 → 不插
		`INSERT INTO projects (id, owner) VALUES (1, '张三;李四')`, // 分号分隔双值 → user 1、2
	} {
		if _, err := conn.Exec(stmt); err != nil {
			t.Fatalf("造数据失败: %v (%s)", err, stmt)
		}
	}

	// 找到 v9 迁移并执行(测试内执行不经过 Open,故手动跑)
	if err := applyMigrationOnly(conn, 9); err != nil {
		t.Fatalf("迁移 v9 失败: %v", err)
	}

	var n int
	conn.QueryRow(`SELECT COUNT(*) FROM task_assignees`).Scan(&n)
	if n != 2 {
		t.Fatalf("task_assignees 行数 = %d, want 2", n)
	}
	var uid int64
	conn.QueryRow(`SELECT user_id FROM task_assignees WHERE task_id = 1`).Scan(&uid)
	if uid != 1 {
		t.Errorf("任务1 assignee 解析 = %d, want 1(email 无命中取 display_name 重名最小 id)", uid)
	}
	conn.QueryRow(`SELECT user_id FROM task_assignees WHERE task_id = 2`).Scan(&uid)
	if uid != 2 {
		t.Errorf("任务2 assignee 解析 = %d, want 2(email 精确命中)", uid)
	}
	var owners []int64
	rows, _ := conn.Query(`SELECT user_id FROM project_owners WHERE project_id = 1 ORDER BY user_id`)
	for rows.Next() {
		var v int64
		rows.Scan(&v)
		owners = append(owners, v)
	}
	rows.Close()
	if len(owners) != 2 || owners[0] != 1 || owners[1] != 2 {
		t.Errorf("项目1 owners = %v, want [1 2]", owners)
	}
	// 快照列回填分号文本
	var snap string
	conn.QueryRow(`SELECT assignee FROM tasks WHERE id = 1`).Scan(&snap)
	if snap != "张三" {
		t.Errorf("任务1快照 = %q, want 张三", snap)
	}
	conn.QueryRow(`SELECT owner FROM projects WHERE id = 1`).Scan(&snap)
	if snap != "张三; 李四" {
		t.Errorf("项目1快照 = %q, want %q", snap, "张三; 李四")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd /f/projects/followITup/backend && go test ./internal/db/ -run TestMigrateV9MultiOwner -v`
Expected: FAIL,`applyMigrationOnly` 未定义

- [ ] **Step 3: 实现迁移 v9 + 测试辅助**

`backend/internal/db/sqlite.go` migrations 数组末尾追加(现有 v8 后):

```go
	{9, `
	-- 多负责人(v9):任务负责人与项目负责人支持多人,关联表为权威,原文本列保留为快照
	CREATE TABLE IF NOT EXISTS task_assignees (
		task_id INTEGER NOT NULL,
		user_id INTEGER NOT NULL,
		PRIMARY KEY (task_id, user_id)
	);
	CREATE TABLE IF NOT EXISTS project_owners (
		project_id INTEGER NOT NULL,
		user_id INTEGER NOT NULL,
		PRIMARY KEY (project_id, user_id)
	);

	-- 存量迁移:email 或 display_name 精确匹配活跃用户,重名取 id 最小;解析失败跳过(归未分配)。
	-- 不按 deleted_at 过滤:软删任务/项目恢复后负责人仍在
	INSERT OR IGNORE INTO task_assignees (task_id, user_id)
	SELECT t.id, MIN(u.id) FROM tasks t
	JOIN users u ON (u.email = t.assignee OR u.display_name = t.assignee) AND u.is_active = 1
	WHERE t.assignee != '' GROUP BY t.id;
	INSERT OR IGNORE INTO project_owners (project_id, user_id)
	SELECT p.id, MIN(u.id) FROM projects p
	JOIN users u ON (u.email = p.owner OR u.display_name = p.owner) AND u.is_active = 1
	WHERE p.owner != '' GROUP BY p.id;

	-- 快照列回填最新显示名(分号+空格分隔)
	UPDATE tasks SET assignee = (
		SELECT GROUP_CONCAT(u.display_name, '; ') FROM task_assignees ta JOIN users u ON u.id = ta.user_id
		WHERE ta.task_id = tasks.id)
	WHERE EXISTS (SELECT 1 FROM task_assignees ta WHERE ta.task_id = tasks.id);
	UPDATE projects SET owner = (
		SELECT GROUP_CONCAT(u.display_name, '; ') FROM project_owners po JOIN users u ON u.id = po.user_id
		WHERE po.project_id = projects.id)
	WHERE EXISTS (SELECT 1 FROM project_owners po WHERE po.project_id = projects.id);
	`},
```

`backend/internal/db/migration_test.go` 追加测试辅助(单条迁移按版本执行):

```go
// applyMigrationOnly 只执行指定版本的迁移(测试隔离用:目标库已手工建好迁移前 schema)
func applyMigrationOnly(conn *sql.DB, version int) error {
	for _, m := range migrations {
		if m.version == version {
			if _, err := conn.Exec(m.sql); err != nil {
				return err
			}
		}
	}
	return nil
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd /f/projects/followITup/backend && go test ./internal/db/ -v`
Expected: PASS(含 TestMigrateV9MultiOwner)

- [ ] **Step 5: 提交**

```bash
git add backend/internal/db/sqlite.go backend/internal/db/migration_test.go
git commit -m "feat:多负责人关联表迁移 v9(task_assignees/project_owners)"
```

---

### Task 2: 模型字段 + 负责人解析/保存辅助函数

**Files:**
- Modify: `backend/internal/models/models.go`(Task/Project 加 ids 数组字段)
- Modify: `backend/internal/api/tasks.go`(新增辅助函数,文件内任意位置)
- Test: `backend/internal/api/multi_owner_test.go`(新建,测试辅助函数)

**Interfaces:**
- Consumes: Task 1 的两张关联表
- Produces:
  - `models.Task.AssigneeIDs []int64`(json `assignee_ids`)、`models.Project.OwnerIDs []int64`(json `owner_ids`)
  - `splitOwnerNames(s string) []string` — 分号/逗号/空白拆分并去重
  - `resolveUserIDs(db *sql.DB, names []string) (ids []int64, missing []string)` — 逐名解析活跃用户,email/display_name 精确匹配;返回成功 ids 与失败名
  - `saveTaskAssignees(db *sql.DB, taskID int64, ids []int64) (snapshot string)` — 事务删旧插新(去重),返回分号分隔显示名快照
  - `saveProjectOwners(db *sql.DB, projectID int64, ids []int64) (snapshot string)` — 同上,项目侧
  - `loadTaskAssignees(db *sql.DB, taskID int64) (ids []int64, snapshot string)`
  - `loadProjectOwners(db *sql.DB, projectID int64) (ids []int64, snapshot string)`

- [ ] **Step 1: 写失败测试**

`backend/internal/api/multi_owner_test.go`(新建):

```go
package api

import (
	"testing"

	"followitup/internal/db"
)

func TestSplitOwnerNames(t *testing.T) {
	cases := []struct{ in string; want []string }{
		{"张三;李四", []string{"张三", "李四"}},
		{"张三; 李四", []string{"张三", "李四"}},
		{"张三,李四", []string{"张三", "李四"}},
		{"张三;张三;李四", []string{"张三", "李四"}},   // 去重
		{" 张三 ; 李四 ", []string{"张三", "李四"}},   // trim
		{"", nil},
		{"张三", []string{"张三"}},
	}
	for _, c := range cases {
		got := splitOwnerNames(c.in)
		if len(got) != len(c.want) {
			t.Errorf("splitOwnerNames(%q) = %v, want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("splitOwnerNames(%q) = %v, want %v", c.in, got, c.want)
				break
			}
		}
	}
}

// resolveUserIDs + saveTaskAssignees + loadTaskAssignees 全链路
func TestResolveAndSaveAssignees(t *testing.T) {
	conn, _ := testTaskHandler(t) // 复用已有 helper:db.Open(TempDir) 已跑全部迁移(含 v9)
	conn.Exec(`INSERT INTO users (login, email, display_name, password_hash, auth_source, is_active) VALUES ('a','a@x.com','张三','x','local',1), ('b','b@x.com','李四','x','local',1), ('c','c@x.com','王五','x','local',0)`)
	pid := setupProject(t, conn)
	tid := setupTask(t, conn, pid, "任务X", "", "")

	ids, missing := resolveUserIDs(conn, []string{"张三", "王五", "不存在"})
	if len(ids) != 1 || len(missing) != 2 {
		t.Fatalf("resolveUserIDs = (%v, %v), want 1 成功 2 失败(停用+不存在)", ids, missing)
	}
	// 重复 id 写入去重
	snap := saveTaskAssignees(conn, tid, append(ids, ids[0]))
	if snap != "张三" {
		t.Errorf("快照 = %q, want 张三", snap)
	}
	gotIDs, gotSnap := loadTaskAssignees(conn, tid)
	if len(gotIDs) != 1 || gotIDs[0] != ids[0] || gotSnap != "张三" {
		t.Errorf("loadTaskAssignees = (%v, %q)", gotIDs, gotSnap)
	}
	// 覆盖写:换成李四
	snap = saveTaskAssignees(conn, tid, []int64{})
	if snap != "" {
		t.Errorf("清空后快照 = %q, want 空", snap)
	}
	gotIDs, _ = loadTaskAssignees(conn, tid)
	if len(gotIDs) != 0 {
		t.Errorf("清空后 ids = %v, want 空", gotIDs)
	}
}

// saveProjectOwners + loadProjectOwners
func TestResolveAndSaveProjectOwners(t *testing.T) {
	conn, _ := testTaskHandler(t)
	conn.Exec(`INSERT INTO users (login, email, display_name, password_hash, auth_source, is_active) VALUES ('a','a@x.com','张三','x','local',1), ('b','b@x.com','李四','x','local',1)`)
	pid := setupProject(t, conn)

	ids, _ := resolveUserIDs(conn, []string{"张三", "李四"})
	snap := saveProjectOwners(conn, pid, ids)
	if snap != "张三; 李四" {
		t.Errorf("项目快照 = %q, want %q", snap, "张三; 李四")
	}
	got, _ := loadProjectOwners(conn, pid)
	if len(got) != 2 {
		t.Errorf("owners = %v, want 2 个", got)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd /f/projects/followITup/backend && go test ./internal/api/ -run 'TestSplitOwnerNames|TestResolveAndSave' -v`
Expected: FAIL,`splitOwnerNames` 未定义(编译错)

- [ ] **Step 3: 模型加字段**

`backend/internal/models/models.go`:

```go
// Task 中 Assignee 字段后追加:
	Assignee        string    `json:"assignee"`
	AssigneeIDs     []int64   `json:"assignee_ids"` // 多负责人:user_id 数组(权威在 task_assignees 表)
```

```go
// Project 中 Owner 字段后追加:
	Owner       string    `json:"owner"` // 项目所有者:分号分隔显示名快照(权威在 project_owners 表)
	OwnerIDs    []int64   `json:"owner_ids"`
```

- [ ] **Step 4: 实现辅助函数**

`backend/internal/api/tasks.go` 新增(建议放在 `fillActualDates` 之后):

```go
// splitOwnerNames 拆分负责人文本(分号/逗号/空白分隔),去重保序
func splitOwnerNames(s string) []string {
	raw := strings.FieldsFunc(s, func(r rune) bool { return r == ';' || r == ',' || r == '\n' })
	var out []string
	seen := map[string]bool{}
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// resolveUserIDs 逐名解析活跃用户(id 精确优先:email 命中才算;其次 display_name),返回成功 ids 与失败名
func resolveUserIDs(db *sql.DB, names []string) ([]int64, []string) {
	var ids []int64
	var missing []string
	seen := map[int64]bool{}
	for _, name := range names {
		var uid int64
		err := db.QueryRow(`SELECT id FROM users WHERE is_active = 1 AND (email = ? OR display_name = ?) ORDER BY id LIMIT 1`, name, name).Scan(&uid)
		if err != nil || seen[uid] {
			if err != nil {
				missing = append(missing, name)
			}
			continue
		}
		seen[uid] = true
		ids = append(ids, uid)
	}
	return ids, missing
}

// saveTaskAssignees 覆盖写任务负责人:事务删旧插新(去重),返回分号分隔显示名快照
func saveTaskAssignees(db *sql.DB, taskID int64, ids []int64) string {
	if len(ids) > 0 {
		seen := map[int64]bool{}
		var uniq []int64
		for _, id := range ids {
			if !seen[id] {
				seen[id] = true
				uniq = append(uniq, id)
			}
		}
		ids = uniq
	}
	tx, err := db.Begin()
	if err != nil {
		return ""
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM task_assignees WHERE task_id = ?`, taskID); err != nil {
		return ""
	}
	for _, id := range ids {
		tx.Exec(`INSERT OR IGNORE INTO task_assignees (task_id, user_id) VALUES (?, ?)`, taskID, id)
	}
	var snap string
	tx.QueryRow(`SELECT GROUP_CONCAT(u.display_name, '; ') FROM task_assignees ta JOIN users u ON u.id = ta.user_id WHERE ta.task_id = ?`, taskID).Scan(&snap)
	tx.Commit()
	return snap
}

// saveProjectOwners 项目负责人覆盖写(同上)
func saveProjectOwners(db *sql.DB, projectID int64, ids []int64) string {
	if len(ids) > 0 {
		seen := map[int64]bool{}
		var uniq []int64
		for _, id := range ids {
			if !seen[id] {
				seen[id] = true
				uniq = append(uniq, id)
			}
		}
		ids = uniq
	}
	tx, err := db.Begin()
	if err != nil {
		return ""
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM project_owners WHERE project_id = ?`, projectID); err != nil {
		return ""
	}
	for _, id := range ids {
		tx.Exec(`INSERT OR IGNORE INTO project_owners (project_id, user_id) VALUES (?, ?)`, projectID, id)
	}
	var snap string
	tx.QueryRow(`SELECT GROUP_CONCAT(u.display_name, '; ') FROM project_owners po JOIN users u ON u.id = po.user_id WHERE po.project_id = ?`, projectID).Scan(&snap)
	tx.Commit()
	return snap
}

// loadTaskAssignees 读取任务负责人:ids + 分号分隔显示名快照
func loadTaskAssignees(db *sql.DB, taskID int64) ([]int64, string) {
	rows, err := db.Query(`SELECT ta.user_id, u.display_name FROM task_assignees ta JOIN users u ON u.id = ta.user_id WHERE ta.task_id = ? ORDER BY ta.user_id`, taskID)
	if err != nil {
		return nil, ""
	}
	defer rows.Close()
	var ids []int64
	var names []string
	for rows.Next() {
		var id int64
		var name string
		if rows.Scan(&id, &name) == nil {
			ids = append(ids, id)
			names = append(names, name)
		}
	}
	return ids, strings.Join(names, "; ")
}

// loadProjectOwners 项目负责人(同上)
func loadProjectOwners(db *sql.DB, projectID int64) ([]int64, string) {
	rows, err := db.Query(`SELECT po.user_id, u.display_name FROM project_owners po JOIN users u ON u.id = po.user_id WHERE po.project_id = ? ORDER BY po.user_id`, projectID)
	if err != nil {
		return nil, ""
	}
	defer rows.Close()
	var ids []int64
	var names []string
	for rows.Next() {
		var id int64
		var name string
		if rows.Scan(&id, &name) == nil {
			ids = append(ids, id)
			names = append(names, name)
		}
	}
	return ids, strings.Join(names, "; ")
}
```

- [ ] **Step 5: 跑测试确认通过**

Run: `cd /f/projects/followITup/backend && go test ./internal/api/ -run 'TestSplitOwnerNames|TestResolveAndSave' -v`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add backend/internal/models/models.go backend/internal/api/tasks.go backend/internal/api/multi_owner_test.go
git commit -m "feat:负责人解析/保存辅助函数与模型 ids 字段"
```

---

### Task 3: 任务写路径接入多负责人(CreateTask/UpdateTask)

**Files:**
- Modify: `backend/internal/api/tasks.go`(CreateTask 233-311 行、UpdateTask 608-707 行)
- Test: `backend/internal/api/multi_owner_test.go`(追加)

**Interfaces:**
- Consumes: Task 2 的 `splitOwnerNames`/`resolveUserIDs`/`saveTaskAssignees`/`loadTaskAssignees`
- Produces: CreateTask/UpdateTask 接受 `assignee_ids: []int64`(兼容 `assignee` 文本),写入关联表并同步快照列;请求体未携带负责人时**保留 DB 旧关联**;CreateTask 未指定负责人时默认取项目全部 owner

- [ ] **Step 1: 写失败测试(追加到 multi_owner_test.go)**

```go
// CreateTask 传 assignee_ids 数组 → 关联表写入;assignee 列同步快照
func TestCreateTaskWithAssigneeIDs(t *testing.T) {
	conn, h := testTaskHandler(t)
	conn.Exec(`INSERT INTO users (login, email, display_name, password_hash, auth_source, is_active) VALUES ('a','a@x.com','张三','x','local',1), ('b','b@x.com','李四','x','local',1)`)
	pid := setupProject(t, conn)
	var uid1, uid2 int64
	conn.QueryRow(`SELECT id FROM users WHERE email='a@x.com'`).Scan(&uid1)
	conn.QueryRow(`SELECT id FROM users WHERE email='b@x.com'`).Scan(&uid2)

	body, _ := json.Marshal(map[string]interface{}{
		"name": "任务A", "task_type": "task", "status": "open",
		"start_date": "2026-08-03", "end_date": "2026-08-10", "duration_days": 5,
		"assignee_ids": []int64{uid1, uid2, uid1}, // 重复 id 去重
	})
	r := chi.NewRouter()
	r.Post("/api/projects/{id}/tasks", h.CreateTask)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/projects/%d/tasks", pid), bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateTask 状态码 = %d, body=%s", w.Code, w.Body.String())
	}
	var tid int64
	conn.QueryRow(`SELECT id FROM tasks WHERE project_id=? AND name='任务A'`, pid).Scan(&tid)
	var n int
	conn.QueryRow(`SELECT COUNT(*) FROM task_assignees WHERE task_id=?`, tid).Scan(&n)
	if n != 2 {
		t.Errorf("task_assignees 行数 = %d, want 2(去重)", n)
	}
	var snap string
	conn.QueryRow(`SELECT assignee FROM tasks WHERE id=?`, tid).Scan(&snap)
	if snap != "张三; 李四" {
		t.Errorf("快照列 = %q, want %q", snap, "张三; 李四")
	}
	// 响应体含 assignee_ids
	var resp struct {
		Data models.Task `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Data.AssigneeIDs) != 2 {
		t.Errorf("响应 assignee_ids = %v, want 2 个", resp.Data.AssigneeIDs)
	}
}

// UpdateTask:未携带负责人字段 → 保留 DB 旧关联;携带 assignee_ids → 覆盖写
func TestUpdateTaskPreservesOrOverwritesAssignees(t *testing.T) {
	conn, h := testTaskHandler(t)
	conn.Exec(`INSERT INTO users (login, email, display_name, password_hash, auth_source, is_active) VALUES ('a','a@x.com','张三','x','local',1), ('b','b@x.com','李四','x','local',1)`)
	pid := setupProject(t, conn)
	tid := setupTask(t, conn, pid, "任务B", "", "")
	var uid1 int64
	conn.QueryRow(`SELECT id FROM users WHERE email='a@x.com'`).Scan(&uid1)
	saveTaskAssignees(conn, tid, []int64{uid1})

	// 部分更新(不含 assignee_ids/assignee):旧关联保留
	body, _ := json.Marshal(map[string]interface{}{
		"name": "任务B改名", "task_type": "task", "status": "open",
		"start_date": "2026-08-03", "end_date": "2026-08-10", "duration_days": 5,
		"version": 1,
	})
	r := chi.NewRouter()
	r.Put("/api/projects/{id}/tasks/{taskID}", h.UpdateTask)
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/projects/%d/tasks/%d", pid, tid), bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("部分更新状态码 = %d, body=%s", w.Code, w.Body.String())
	}
	var n int
	conn.QueryRow(`SELECT COUNT(*) FROM task_assignees WHERE task_id=?`, tid).Scan(&n)
	if n != 1 {
		t.Errorf("部分更新后关联行数 = %d, want 1(保留旧值)", n)
	}
	// 携带 assignee_ids:[] → 清空
	body2, _ := json.Marshal(map[string]interface{}{
		"name": "任务B改名", "task_type": "task", "status": "open",
		"start_date": "2026-08-03", "end_date": "2026-08-10", "duration_days": 5,
		"assignee_ids": []int64{}, "version": 2,
	})
	req2 := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/projects/%d/tasks/%d", pid, tid), bytes.NewReader(body2))
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("覆盖写状态码 = %d, body=%s", w2.Code, w2.Body.String())
	}
	conn.QueryRow(`SELECT COUNT(*) FROM task_assignees WHERE task_id=?`, tid).Scan(&n)
	if n != 0 {
		t.Errorf("清空后关联行数 = %d, want 0", n)
	}
}

// UpdateTask 旧文本 assignee 兼容:分号分隔
func TestUpdateTaskLegacyAssigneeText(t *testing.T) {
	conn, h := testTaskHandler(t)
	conn.Exec(`INSERT INTO users (login, email, display_name, password_hash, auth_source, is_active) VALUES ('a','a@x.com','张三','x','local',1), ('b','b@x.com','李四','x','local',1)`)
	pid := setupProject(t, conn)
	tid := setupTask(t, conn, pid, "任务C", "", "")

	body, _ := json.Marshal(map[string]interface{}{
		"name": "任务C", "task_type": "task", "status": "open",
		"start_date": "2026-08-03", "end_date": "2026-08-10", "duration_days": 5,
		"assignee": "张三;李四", "version": 1,
	})
	r := chi.NewRouter()
	r.Put("/api/projects/{id}/tasks/{taskID}", h.UpdateTask)
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/projects/%d/tasks/%d", pid, tid), bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, body=%s", w.Code, w.Body.String())
	}
	var n int
	conn.QueryRow(`SELECT COUNT(*) FROM task_assignees WHERE task_id=?`, tid).Scan(&n)
	if n != 2 {
		t.Errorf("文本解析后关联行数 = %d, want 2", n)
	}
}

// CreateTask 未指定负责人 → 默认取项目全部 owner
func TestCreateTaskDefaultsToProjectOwners(t *testing.T) {
	conn, h := testTaskHandler(t)
	conn.Exec(`INSERT INTO users (login, email, display_name, password_hash, auth_source, is_active) VALUES ('a','a@x.com','张三','x','local',1), ('b','b@x.com','李四','x','local',1)`)
	pid := setupProject(t, conn)
	ids, _ := resolveUserIDs(conn, []string{"张三", "李四"})
	saveProjectOwners(conn, pid, ids)

	body, _ := json.Marshal(map[string]interface{}{
		"name": "任务D", "task_type": "task", "status": "open",
		"start_date": "2026-08-03", "end_date": "2026-08-10", "duration_days": 5,
	})
	r := chi.NewRouter()
	r.Post("/api/projects/{id}/tasks", h.CreateTask)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/projects/%d/tasks", pid), bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("状态码 = %d, body=%s", w.Code, w.Body.String())
	}
	var n int
	conn.QueryRow(`SELECT COUNT(*) FROM task_assignees WHERE task_id IN (SELECT id FROM tasks WHERE project_id=? AND name='任务D')`, pid).Scan(&n)
	if n != 2 {
		t.Errorf("默认项目 owner 后关联行数 = %d, want 2", n)
	}
}
```

(测试头部需补 import:`followitup/internal/models`)

- [ ] **Step 2: 跑测试确认失败**

Run: `cd /f/projects/followITup/backend && go test ./internal/api/ -run 'TestCreateTaskWithAssigneeIDs|TestUpdateTaskPreservesOrOverwritesAssignees|TestUpdateTaskLegacyAssigneeText|TestCreateTaskDefaultsToProjectOwners' -v`
Expected: FAIL(旧逻辑不写关联表,断言行数 0)

- [ ] **Step 3: CreateTask 改造**

`backend/internal/api/tasks.go` CreateTask 中,替换「防呆:未指定负责人时默认取项目 owner」块(现 269-274 行)为:

```go
	// 解析负责人:旧 assignee 文本(分号/逗号分隔)逐名解析;未指定时默认取项目全部 owner
	var ids []int64
	var missing []string
	if strings.TrimSpace(t.Assignee) != "" {
		ids, missing = resolveUserIDs(h.db, splitOwnerNames(t.Assignee))
		if len(missing) > 0 {
			writeError(w, http.StatusBadRequest, "INVALID_OWNER", "负责人["+strings.Join(missing, ",")+"]不是系统用户,请从现有用户中选择")
			return
		}
	} else {
		rows, err := h.db.Query(`SELECT po.user_id FROM project_owners po JOIN projects p ON p.id = po.project_id WHERE p.id = ? AND p.deleted_at IS NULL`, projectID)
		if err == nil {
			for rows.Next() {
				var uid int64
				if rows.Scan(&uid) == nil {
					ids = append(ids, uid)
				}
			}
			rows.Close()
		}
	}
	// 快照列同步(写入后回填显示名);id 数组塞回模型
	assigneeSnapshot := strings.Join(ownerNamesOf(h.db, ids), "; ")
```

其中 `ownerNamesOf` 是本任务新增的小工具(也可直接用 `loadTaskAssignees` 思想,但此时未插入关联表,先拼 ids 对应名字):

```go
// ownerNamesOf 按 user_id 数组返回显示名(与传入顺序一致,缺失跳过)
func ownerNamesOf(db *sql.DB, ids []int64) []string {
	if len(ids) == 0 {
		return nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := db.Query(`SELECT id, display_name FROM users WHERE id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	nameByID := map[int64]string{}
	for rows.Next() {
		var id int64
		var name string
		if rows.Scan(&id, &name) == nil {
			nameByID[id] = name
		}
	}
	var out []string
	for _, id := range ids {
		if n, ok := nameByID[id]; ok {
			out = append(out, n)
		}
	}
	return out
}
```

INSERT 语句的 `assignee` 参数改为 `assigneeSnapshot`(现 284-285 行 `t.Assignee` 实参处)。INSERT 成功后、`t.ID = id` 之后加:

```go
	// 写关联表(权威)并回填响应模型
	saveTaskAssignees(h.db, id, ids)
	t.AssigneeIDs = ids
	t.Assignee = assigneeSnapshot
```

- [ ] **Step 4: UpdateTask 改造**

`backend/internal/api/tasks.go` UpdateTask 中,在 `fieldCheck` 解析(现 625-632 行)之后追加:

```go
	// 负责人:assignee_ids/assignee 都未携带 → 保留 DB 旧关联(与 actual_* 同策略)
	var assigneeIDs []int64
	if _, ok := fieldCheck["assignee_ids"]; ok {
		json.Unmarshal(fieldCheck["assignee_ids"], &assigneeIDs)
	} else if _, ok := fieldCheck["assignee"]; ok {
		assigneeIDs, _ = resolveUserIDs(h.db, splitOwnerNames(t.Assignee))
		if len(splitOwnerNames(t.Assignee)) > len(assigneeIDs) {
			writeError(w, http.StatusBadRequest, "INVALID_OWNER", "负责人含非系统用户,请从现有用户中选择")
			return
		}
	} else {
		assigneeIDs, _ = loadTaskAssignees(h.db, taskID)
	}
	t.AssigneeIDs = assigneeIDs
```

在 UPDATE 执行前把 `t.Assignee` 替换为快照(现 676 行实参处):

```go
	// 负责人快照列:以关联表为准(等写关联表后回填;这里先按当前 ids 拼,写关联表用同一批)
	t.Assignee = strings.Join(ownerNamesOf(h.db, assigneeIDs), "; ")
```

UPDATE 成功后(broadcastChange 之后、writeJSON 之前)加:

```go
	// 写关联表(权威);响应模型回填
	if _, ok := fieldCheck["assignee_ids"]; ok || ok2 := fieldCheck["assignee"]; false {
		_ = ok2
	}
	saveTaskAssignees(h.db, taskID, assigneeIDs)
```

注意:上一步的表达式写法不对,改为清晰实现——直接在写关联表前判断是否有提供:

```go
	_, providedIDs := fieldCheck["assignee_ids"]
	_, providedText := fieldCheck["assignee"]
	if providedIDs || providedText {
		saveTaskAssignees(h.db, taskID, assigneeIDs)
	}
```

(已保留旧值时 loadTaskAssignees 已含旧 id,不必重写关联表;若重写也无副作用,但避免多余写。)

- [ ] **Step 5: 跑测试确认通过**

Run: `cd /f/projects/followITup/backend && go test ./internal/api/ -run 'TestCreateTaskWithAssigneeIDs|TestUpdateTaskPreservesOrOverwritesAssignees|TestUpdateTaskLegacyAssigneeText|TestCreateTaskDefaultsToProjectOwners|TestUpdateTaskPartialUpdateKeepsActual|TestUpdateTaskInvalidActual' -v`
Expected: PASS(含既有任务测试回归)

- [ ] **Step 6: 提交**

```bash
git add backend/internal/api/tasks.go backend/internal/api/multi_owner_test.go
git commit -m "feat:任务创建/更新接入多负责人(assignee_ids+文本兼容+项目owner兜底)"
```

---

### Task 4: 任务读路径返回 assignee_ids(ListTasks/GetTask/ListDeletedTasks)

**Files:**
- Modify: `backend/internal/api/tasks.go`(ListTasks 57-118、GetTask 198-230、ListDeletedTasks 120-167)
- Test: `backend/internal/api/multi_owner_test.go`(追加)

**Interfaces:**
- Consumes: Task 2 的 `loadTaskAssignees`
- Produces: 任务对象 `assignee`(分号文本,权威拼装)+ `assignee_ids` 数组

- [ ] **Step 1: 写失败测试**

```go
// ListTasks 返回 assignee_ids 与分号分隔 assignee
func TestListTasksReturnsAssigneeIDs(t *testing.T) {
	conn, h := testTaskHandler(t)
	conn.Exec(`INSERT INTO users (login, email, display_name, password_hash, auth_source, is_active) VALUES ('a','a@x.com','张三','x','local',1), ('b','b@x.com','李四','x','local',1)`)
	pid := setupProject(t, conn)
	tid := setupTask(t, conn, pid, "任务E", "", "")
	ids, _ := resolveUserIDs(conn, []string{"张三", "李四"})
	saveTaskAssignees(conn, tid, ids)

	r := chi.NewRouter()
	r.Get("/api/projects/{id}/tasks", h.ListTasks)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/projects/%d/tasks", pid), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d", w.Code)
	}
	var resp struct {
		Data struct {
			Tasks []models.Task `json:"tasks"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Data.Tasks) != 1 {
		t.Fatalf("任务数 = %d", len(resp.Data.Tasks))
	}
	task := resp.Data.Tasks[0]
	if len(task.AssigneeIDs) != 2 {
		t.Errorf("assignee_ids = %v, want 2 个", task.AssigneeIDs)
	}
	if task.Assignee != "张三; 李四" {
		t.Errorf("assignee = %q, want %q", task.Assignee, "张三; 李四")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd /f/projects/followITup/backend && go test ./internal/api/ -run TestListTasksReturnsAssigneeIDs -v`
Expected: FAIL(assignee_ids 为空)

- [ ] **Step 3: ListTasks 改造**

`tasks.go` ListTasks 中:SELECT 保留原列,scan 后对每个任务回填(循环内调用 loadTaskAssignees 为 N+1,单项目任务量级可接受):

```go
	for rows.Next() {
		// ...现有 scan 不变...
		t.ManualScheduled = manualSched != 0
		// 多负责人:权威在关联表,覆盖快照列并回填 ids
		t.AssigneeIDs, t.Assignee = loadTaskAssignees(h.db, t.ID)
		tasks = append(tasks, t)
	}
```

- [ ] **Step 4: GetTask 改造**

`tasks.go` GetTask 中,scan 成功后(现 227 行 `t.ManualScheduled = ...` 之后)加:

```go
	t.AssigneeIDs, t.Assignee = loadTaskAssignees(h.db, t.ID)
```

- [ ] **Step 5: ListDeletedTasks 改造(回收站)**

`tasks.go` ListDeletedTasks 的 DeletedTask struct 加字段并回填:

```go
		AssigneeIDs []int64 `json:"assignee_ids"`
```

scan 后(现 162 行 if parentID.Valid 之后)加:

```go
		tasks[i].AssigneeIDs, _ = loadTaskAssignees(h.db, t.ID)
```

注意循环内用索引赋值(当前代码用 `tasks = append`,需改收集方式:先把 `t` append,循环结束后再统一回填,或改 `tasks = append(tasks, t)` 后取最后元素):

```go
		tasks = append(tasks, t)
		last := &tasks[len(tasks)-1]
		last.AssigneeIDs, _ = loadTaskAssignees(h.db, last.ID)
```

- [ ] **Step 6: 跑测试确认通过**

Run: `cd /f/projects/followITup/backend && go test ./internal/api/ -v`
Expected: PASS(全部任务相关测试)

- [ ] **Step 7: 提交**

```bash
git add backend/internal/api/tasks.go backend/internal/api/multi_owner_test.go
git commit -m "feat:任务读接口返回 assignee_ids 与分号文本(权威关联表)"
```

---

### Task 5: GetMyTasks 双视角(view=task|project)

**Files:**
- Modify: `backend/internal/api/tasks.go`(GetMyTasks 526-587 行)
- Test: `backend/internal/api/multi_owner_test.go`(追加)

**Interfaces:**
- Consumes: Task 1 关联表
- Produces: `GET /api/tasks/mine?view=task|project&days=N` — task 视角:我名下的进行中+即将开始;project 视角:我名下项目的进行中+即将开始

- [ ] **Step 1: 写失败测试**

```go
// GetMyTasks 双视角:task=我名下任务;project=我名下项目的全部任务
func TestGetMyTasksViews(t *testing.T) {
	conn, h := testTaskHandler(t)
	conn.Exec(`INSERT INTO users (login, email, display_name, password_hash, auth_source, is_active) VALUES ('me@x.com','me@x.com','Me User','x','local',1), ('a@x.com','a@x.com','Alpha','x','local',1), ('b@x.com','b@x.com','Beta','x','local',1)`)
	var me, alpha int64
	conn.QueryRow(`SELECT id FROM users WHERE email='me@x.com'`).Scan(&me)
	conn.QueryRow(`SELECT id FROM users WHERE email='a@x.com'`).Scan(&alpha)

	// 项目1:owner = Me User;项目2:owner = Alpha
	r1, _ := conn.Exec(`INSERT INTO projects (name, start_date, end_date, status, owner) VALUES ('我的项目','2026-08-01','2026-08-31','active','Me User')`)
	pid1, _ := r1.LastInsertId()
	r2, _ := conn.Exec(`INSERT INTO projects (name, start_date, end_date, status, owner) VALUES ('他人项目','2026-08-01','2026-08-31','active','Alpha')`)
	pid2, _ := r2.LastInsertId()

	// 我的项目里:任务1 进行中、assignee=Beta(不是我);任务2 open、明天开始
	conn.Exec(`INSERT INTO tasks (project_id, name, task_type, status, start_date, end_date, duration_days, progress_pct, sort_order) VALUES (?, '项目任务', 'task', 'in_progress', '2026-08-05', '2026-08-15', 5, 30, 0)`, pid1)
	var t1 int64
	conn.QueryRow(`SELECT id FROM tasks WHERE project_id=? AND name='项目任务'`, pid1).Scan(&t1)
	var beta int64
	conn.QueryRow(`SELECT id FROM users WHERE email='b@x.com'`).Scan(&beta)
	saveTaskAssignees(conn, t1, []int64{beta})
	conn.Exec(`INSERT INTO tasks (project_id, name, task_type, status, start_date, end_date, duration_days, progress_pct, sort_order) VALUES (?, '项目任务2', 'task', 'open', '2026-08-12', '2026-08-16', 5, 0, 1)`, pid1)
	var t2 int64
	conn.QueryRow(`SELECT id FROM tasks WHERE project_id=? AND name='项目任务2'`, pid1).Scan(&t2)
	saveTaskAssignees(conn, t2, []int64{me}) // 我名下

	// 他人项目里:任务3 进行中、assignee=我
	conn.Exec(`INSERT INTO tasks (project_id, name, task_type, status, start_date, end_date, duration_days, progress_pct, sort_order) VALUES (?, '他人项目任务', 'task', 'in_progress', '2026-08-05', '2026-08-15', 5, 30, 0)`, pid2)
	var t3 int64
	conn.QueryRow(`SELECT id FROM tasks WHERE project_id=? AND name='他人项目任务'`, pid2).Scan(&t3)
	saveTaskAssignees(conn, t3, []int64{me})

	do := func(view string) ([]string, []string) {
		r := chi.NewRouter()
		r.Get("/api/tasks/mine", h.GetMyTasks)
		req := httptest.NewRequest(http.MethodGet, "/api/tasks/mine?view="+view, nil)
		ctx := context.WithValue(req.Context(), auth.UserIDKey, me)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("view=%s 状态码 = %d, body=%s", view, w.Code, w.Body.String())
		}
		var resp struct {
			Data struct {
				Mine     []MyTaskItem `json:"mine"`
				Starting []MyTaskItem `json:"starting"`
			} `json:"data"`
		}
		json.Unmarshal(w.Body.Bytes(), &resp)
		var m, s []string
		for _, x := range resp.Data.Mine {
			m = append(m, x.Name)
		}
		for _, x := range resp.Data.Starting {
			s = append(s, x.Name)
		}
		return m, s
	}

	// task 视角:mine 只含我名下的(项目任务2 in_progress?不——任务2 是 open,任务3 是 in_progress 且我名下)
	mineT, startingT := do("task")
	// 我名下 in_progress = 他人项目任务;我名下 starting(12 天内)= 项目任务2
	if len(mineT) != 1 || mineT[0] != "他人项目任务" {
		t.Errorf("task视角 mine = %v, want [他人项目任务]", mineT)
	}
	if len(startingT) != 1 || startingT[0] != "项目任务2" {
		t.Errorf("task视角 starting = %v, want [项目任务2]", startingT)
	}
	// project 视角:mine = 我名下项目全部 in_progress(项目任务);starting = 我名下项目 open 即将开始(项目任务2)
	mineP, startingP := do("project")
	if len(mineP) != 1 || mineP[0] != "项目任务" {
		t.Errorf("project视角 mine = %v, want [项目任务]", mineP)
	}
	if len(startingP) != 1 || startingP[0] != "项目任务2" {
		t.Errorf("project视角 starting = %v, want [项目任务2]", startingP)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd /f/projects/followITup/backend && go test ./internal/api/ -run TestGetMyTasksViews -v`
Expected: FAIL(旧实现按 assignee 文本匹配,task 视角含/不含不对)

- [ ] **Step 3: 实现双视角查询**

`tasks.go` GetMyTasks 整体改造(替换 526-587 行):

```go
// GetMyTasks 我的待办：view=task(默认)我名下的任务;view=project 我名下项目的任务。
// 每个视角两个分区:mine(进行中)+ starting(未来 N 天开始,未完成)。
func (h *TaskHandler) GetMyTasks(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录")
		return
	}
	view := r.URL.Query().Get("view")
	if view != "project" {
		view = "task"
	}

	days := 7
	if d, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil && d >= 1 && d <= 30 {
		days = d
	}
	today := time.Now().Format("2006-01-02")
	windowEnd := time.Now().AddDate(0, 0, days).Format("2006-01-02")

	// 待办任务限定条件:task 视角=我负责的任务;project 视角=我负责的项目的任务
	scope := `EXISTS (SELECT 1 FROM task_assignees ta WHERE ta.task_id = t.id AND ta.user_id = ?)`
	scopeArg := userID
	if view == "project" {
		scope = `EXISTS (SELECT 1 FROM project_owners po JOIN projects pj ON pj.id = po.project_id WHERE po.project_id = t.project_id AND po.user_id = ? AND pj.deleted_at IS NULL)`
	}

	var mine []MyTaskItem
	rows, err := h.db.Query(`
		SELECT t.id, t.name, p.name, t.status, COALESCE(t.start_date, ''), COALESCE(t.end_date, ''), t.progress_pct
		FROM tasks t
		JOIN projects p ON p.id = t.project_id AND p.deleted_at IS NULL
		WHERE t.deleted_at IS NULL AND t.status = 'in_progress' AND `+scope+`
		ORDER BY CASE WHEN t.end_date = '' THEN 1 ELSE 0 END, t.end_date`, scopeArg)
	if err == nil {
		for rows.Next() {
			var it MyTaskItem
			if rows.Scan(&it.ID, &it.Name, &it.ProjectName, &it.Status, &it.StartDate, &it.EndDate, &it.ProgressPct) == nil {
				it.Assignee, _ = loadTaskAssignees(h.db, it.ID)
				mine = append(mine, it)
			}
		}
		rows.Close()
	}

	var starting []MyTaskItem
	rows2, err := h.db.Query(`
		SELECT t.id, t.name, p.name, t.status, COALESCE(t.start_date, ''), COALESCE(t.end_date, ''), t.progress_pct
		FROM tasks t
		JOIN projects p ON p.id = t.project_id AND p.deleted_at IS NULL
		WHERE t.deleted_at IS NULL AND t.status != 'completed' AND `+scope+`
		  AND t.start_date >= ? AND t.start_date <= ?
		ORDER BY t.start_date`, scopeArg, today, windowEnd)
	if err == nil {
		for rows2.Next() {
			var it MyTaskItem
			if rows2.Scan(&it.ID, &it.Name, &it.ProjectName, &it.Status, &it.StartDate, &it.EndDate, &it.ProgressPct) == nil {
				it.Assignee, _ = loadTaskAssignees(h.db, it.ID)
				starting = append(starting, it)
			}
		}
		rows2.Close()
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"mine":     mine,
		"starting": starting,
	})
}
```

- [ ] **Step 4: 跑测试确认通过(含旧测试回归)**

Run: `cd /f/projects/followITup/backend && go test ./internal/api/ -run 'TestGetMyTasks|TestGetMyTasksViews' -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add backend/internal/api/tasks.go backend/internal/api/multi_owner_test.go
git commit -m "feat:我的待办双视角(view=task|project)"
```

---

### Task 6: CSV 导入负责人分号多值

**Files:**
- Modify: `backend/internal/api/tasks.go`(ImportTasks 316-512 行)
- Test: `backend/internal/api/multi_owner_test.go`(追加)

**Interfaces:**
- Consumes: Task 2 辅助函数
- Produces: 负责人列 `张三;李四` 逐个解析;失败名提示「负责人『XX』不是系统用户,已归未分配」;任务不跳过

- [ ] **Step 1: 写失败测试**

```go
// CSV 导入:负责人列分号多值,解析失败归未分配+提示
func TestImportTasksMultiAssignee(t *testing.T) {
	conn, h := testTaskHandler(t)
	conn.Exec(`INSERT INTO users (login, email, display_name, password_hash, auth_source, is_active) VALUES ('a@x.com','a@x.com','张三','x','local',1), ('b@x.com','b@x.com','李四','x','local',1)`)
	pid := setupProject(t, conn)

	csv := strings.Join([]string{
		"任务名,WBS,工期,开始日期,负责人,进度,状态",
		"需求,1,3,2026-08-03,张三;李四,50%,进行中",   // 双负责人
		"编码,2,3,2026-08-10,张三;王五,0%,",          // 王五不存在 → 只留张三 + 提示
	}, "\n")
	body, _ := json.Marshal(map[string]string{"csv": csv})
	r := chi.NewRouter()
	r.Post("/api/projects/{id}/tasks/import", h.ImportTasks)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/projects/%d/tasks/import", pid), bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Imported int      `json:"imported"`
			Errors   []string `json:"errors"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.Imported != 2 {
		t.Fatalf("imported = %d, want 2", resp.Data.Imported)
	}
	// 需求任务:2 个负责人
	var tid1 int64
	conn.QueryRow(`SELECT id FROM tasks WHERE project_id=? AND name='需求'`, pid).Scan(&tid1)
	var n int
	conn.QueryRow(`SELECT COUNT(*) FROM task_assignees WHERE task_id=?`, tid1).Scan(&n)
	if n != 2 {
		t.Errorf("需求负责人数 = %d, want 2", n)
	}
	// 编码任务:1 个负责人(王五被跳过)
	var tid2 int64
	conn.QueryRow(`SELECT id FROM tasks WHERE project_id=? AND name='编码'`, pid).Scan(&tid2)
	conn.QueryRow(`SELECT COUNT(*) FROM task_assignees WHERE task_id=?`, tid2).Scan(&n)
	if n != 1 {
		t.Errorf("编码负责人数 = %d, want 1", n)
	}
	// 提示含王五
	found := false
	for _, e := range resp.Data.Errors {
		if strings.Contains(e, "王五") {
			found = true
		}
	}
	if !found {
		t.Errorf("errors = %v, want 含王五提示", resp.Data.Errors)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd /f/projects/followITup/backend && go test ./internal/api/ -run TestImportTasksMultiAssignee -v`
Expected: FAIL(关联表行数 0)

- [ ] **Step 3: ImportTasks 改造**

`tasks.go` ImportTasks 中替换「防呆:负责人空 → 项目 owner」块(现 466-472 行)为:

```go
		// 防呆:负责人空 → 项目全部 owner;非空 → 分号拆分为多值,逐个解析
		assigneeIDs, missing := resolveUserIDs(h.db, splitOwnerNames(row.assignee))
		if len(assigneeIDs) == 0 {
			rows, err := h.db.Query(`SELECT po.user_id FROM project_owners po JOIN projects p ON p.id = po.project_id WHERE p.id = ? AND p.deleted_at IS NULL`, projectID)
			if err == nil {
				for rows.Next() {
					var uid int64
					if rows.Scan(&uid) == nil {
						assigneeIDs = append(assigneeIDs, uid)
					}
				}
				rows.Close()
			}
		}
		for _, m := range missing {
			skipReasons = append(skipReasons, fmt.Sprintf("行[%s %s]:负责人[%s]不是系统用户,已归未分配", row.wbs, row.name, m))
		}
		assigneeSnapshot := strings.Join(ownerNamesOf(h.db, assigneeIDs), "; ")
```

INSERT 语句的 `assignee` 参数改为 `assigneeSnapshot`(现 482 行实参处)。INSERT 成功后(现 492 行 `wbsToID[row.wbs] = id` 处)加:

```go
		saveTaskAssignees(h.db, id, assigneeIDs)
```

- [ ] **Step 4: 跑测试确认通过(含既有导入测试回归)**

Run: `cd /f/projects/followITup/backend && go test ./internal/api/ -run 'TestImportTasks|TestImportTasksMultiAssignee' -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add backend/internal/api/tasks.go backend/internal/api/multi_owner_test.go
git commit -m "feat:CSV导入负责人列支持分号多值+解析失败提示"
```

---

### Task 7: 项目多负责人(CreateProject/UpdateProject/ProjectList/GetProject/CopyProject)

**Files:**
- Modify: `backend/internal/api/projects.go`
- Test: `backend/internal/api/multi_owner_test.go`(追加)

**Interfaces:**
- Consumes: Task 2 的 `splitOwnerNames`/`resolveUserIDs`/`saveProjectOwners`/`loadProjectOwners`
- Produces: 项目对象 `owner_ids` + 分号文本;owner 校验逐项;owner 变更未开始任务改派;CopyProject 复制两套关联表;project_members 同步

- [ ] **Step 1: 写失败测试**

```go
// 创建项目:owner_ids 数组 + 文本兼容 + 校验失败 400
func TestCreateProjectWithOwnerIDs(t *testing.T) {
	conn, h := testTaskHandler(t)
	conn.Exec(`INSERT INTO users (login, email, display_name, password_hash, auth_source, is_active) VALUES ('a@x.com','a@x.com','张三','x','local',1), ('b@x.com','b@x.com','李四','x','local',1)`)
	var uid1 int64
	conn.QueryRow(`SELECT id FROM users WHERE email='a@x.com'`).Scan(&uid1)

	r := chi.NewRouter()
	r.Post("/api/projects", h.CreateProject)

	// 正常:owner_ids 双值
	body, _ := json.Marshal(map[string]interface{}{
		"name": "项目A", "start_date": "2026-08-01", "end_date": "2026-08-31",
		"owner_ids": []int64{uid1, 999}, // 999 不存在 → 400
	})
	req := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("无效 owner_ids 状态码 = %d, want 400", w.Code)
	}
	// 文本兼容
	body2, _ := json.Marshal(map[string]interface{}{
		"name": "项目A", "start_date": "2026-08-01", "end_date": "2026-08-31",
		"owner": "张三;李四",
	})
	req2 := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(body2))
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusCreated {
		t.Fatalf("文本创建状态码 = %d, body=%s", w2.Code, w2.Body.String())
	}
	var pid int64
	conn.QueryRow(`SELECT id FROM projects WHERE name='项目A'`).Scan(&pid)
	var n int
	conn.QueryRow(`SELECT COUNT(*) FROM project_owners WHERE project_id=?`, pid).Scan(&n)
	if n != 2 {
		t.Errorf("project_owners 行数 = %d, want 2", n)
	}
}

// 项目列表返回 owner_ids;CopyProject 复制两套关联表
func TestProjectListAndCopyWithOwners(t *testing.T) {
	conn, h := testTaskHandler(t)
	conn.Exec(`INSERT INTO users (login, email, display_name, password_hash, auth_source, is_active) VALUES ('a@x.com','a@x.com','张三','x','local',1), ('b@x.com','b@x.com','李四','x','local',1)`)
	ids, _ := resolveUserIDs(conn, []string{"张三", "李四"})
	r1, _ := conn.Exec(`INSERT INTO projects (name, start_date, end_date, status, owner) VALUES ('源项目','2026-08-01','2026-08-31','active','张三; 李四')`)
	pid, _ := r1.LastInsertId()
	saveProjectOwners(conn, pid, ids)
	// 任务+负责人
	r2, _ := conn.Exec(`INSERT INTO tasks (project_id, name, task_type, status, start_date, end_date, duration_days, progress_pct, sort_order) VALUES (?, '任务A','task','open','2026-08-03','2026-08-10',5,0,0)`, pid)
	tid, _ := r2.LastInsertId()
	saveTaskAssignees(conn, tid, ids)

	// 项目列表含 owner_ids
	r := chi.NewRouter()
	r.Get("/api/projects", h.ProjectList)
	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("列表状态码 = %d", w.Code)
	}
	var list []struct {
		ID       int64   `json:"id"`
		Owner    string  `json:"owner"`
		OwnerIDs []int64 `json:"owner_ids"`
	}
	json.Unmarshal(w.Body.Bytes(), &list)
	found := false
	for _, p := range list {
		if p.ID == pid {
			found = true
			if len(p.OwnerIDs) != 2 || p.Owner != "张三; 李四" {
				t.Errorf("项目 owner = (%q, %v)", p.Owner, p.OwnerIDs)
			}
		}
	}
	if !found {
		t.Fatal("列表中找不到源项目")
	}

	// 复制项目
	r2r := chi.NewRouter()
	r2r.Post("/api/projects/{id}/copy", h.CopyProject)
	req2 := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/projects/%d/copy", pid), nil)
	ctx := context.WithValue(req2.Context(), auth.UserIDKey, int64(1))
	req2 = req2.WithContext(ctx)
	w2 := httptest.NewRecorder()
	r2r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusCreated {
		t.Fatalf("复制状态码 = %d, body=%s", w2.Code, w2.Body.String())
	}
	var newID int64
	conn.QueryRow(`SELECT id FROM projects WHERE name='源项目(副本)'`).Scan(&newID)
	conn.QueryRow(`SELECT COUNT(*) FROM project_owners WHERE project_id=?`, newID).Scan(&n)
	if n != 2 {
		t.Errorf("副本 project_owners = %d, want 2", n)
	}
	conn.QueryRow(`SELECT COUNT(*) FROM task_assignees ta JOIN tasks t ON t.id = ta.task_id WHERE t.project_id=?`, newID).Scan(&n)
	if n != 2 {
		t.Errorf("副本 task_assignees = %d, want 2", n)
	}
}
```

(测试头部补 import:`followitup/internal/models` 已在 Task 3 加过;本测试用到 `auth` 已导入)

- [ ] **Step 2: 跑测试确认失败**

Run: `cd /f/projects/followITup/backend && go test ./internal/api/ -run 'TestCreateProjectWithOwnerIDs|TestProjectListAndCopyWithOwners' -v`
Expected: FAIL(owner_ids 未实现)

- [ ] **Step 3: CreateProject 改造**

`projects.go` CreateProject 中,替换 owner 校验块(现 221-225 行)为:

```go
	// 负责人解析:owner_ids 优先,其次 owner 文本(分号/逗号分隔);每项必须是活跃系统用户
	var ownerIDs []int64
	if len(p.OwnerIDs) > 0 {
		ownerIDs = p.OwnerIDs
		missing := []string{}
		valid, _ := resolveUserIDs(h.db, ownerNamesOfPlaceholder())
		_ = valid
		_ = missing
	} else if strings.TrimSpace(p.Owner) != "" {
		ownerIDs, _ = resolveUserIDs(h.db, splitOwnerNames(p.Owner))
	}
```

注意 `ownerNamesOfPlaceholder` 不是真实函数——**正确实现**:ids 数组也要校验存在且活跃。用 `loadProjectOwners` 不行(还没写)。新增校验方式:对 owner_ids 数组逐 id 查 users:

```go
	// 负责人解析与校验:owner_ids 优先,其次 owner 文本(分号/逗号分隔);每项必须是活跃系统用户
	var ownerIDs []int64
	if len(p.OwnerIDs) > 0 {
		for _, uid := range p.OwnerIDs {
			var n int
			h.db.QueryRow(`SELECT COUNT(*) FROM users WHERE id=? AND is_active=1`, uid).Scan(&n)
			if n > 0 {
				ownerIDs = append(ownerIDs, uid)
			} else {
				writeError(w, http.StatusBadRequest, "INVALID_OWNER", "项目所有者必须是现有活跃用户(含无效ID)")
				return
			}
		}
	} else if strings.TrimSpace(p.Owner) != "" {
		ownerIDs, _ = resolveUserIDs(h.db, splitOwnerNames(p.Owner))
		if len(splitOwnerNames(p.Owner)) != len(ownerIDs) {
			writeError(w, http.StatusBadRequest, "INVALID_OWNER", "项目所有者必须从现有用户中选择")
			return
		}
	}
	ownerSnapshot := strings.Join(ownerNamesOf(h.db, ownerIDs), "; ")
```

INSERT 语句的 `owner` 参数改为 `ownerSnapshot`(现 239 行实参处)。INSERT 成功后(现 247 行 `p.Status = "active"` 之后)加:

```go
	p.Owner = ownerSnapshot
	p.OwnerIDs = ownerIDs
	if len(ownerIDs) > 0 {
		saveProjectOwners(h.db, id, ownerIDs)
	}
```

创建者加入 project_members 块(现 250-254 行)之后追加:每个 owner 也加入:

```go
	// 项目负责人自动加入成员(role=owner),为将来权限留口子
	for _, uid := range ownerIDs {
		h.db.Exec("INSERT OR IGNORE INTO project_members (project_id, user_id, role) VALUES (?, ?, 'owner')", id, uid)
	}
```

- [ ] **Step 4: ProjectList / GetProject 改造**

`projects.go` ProjectList 的 SELECT(现 116-120 行)把 owner 列替换为子查询拼装:

```go
	query := `SELECT p.id, p.name, p.description, p.start_date, p.end_date, p.status, p.is_public,
		COALESCE(p.baseline_created_at, '') as baseline_created_at,
		COALESCE(p.baseline_created_by, '') as baseline_created_by,
			p.schedule_direction, COALESCE(p.owner, '') as owner,
		(SELECT GROUP_CONCAT(po.user_id, ',') FROM project_owners po WHERE po.project_id = p.id) as owner_ids
		FROM projects p WHERE p.deleted_at IS NULL ORDER BY p.created_at ASC, p.id ASC`
```

scan 增加 `ownerIDsStr`(现 145-148 行)并解析:

```go
		var ownerIDsStr string
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.StartDate, &p.EndDate,
			&p.Status, &isPublic, &p.BaselineCreatedAt, &p.BaselineCreatedBy, &p.ScheduleDirection, &p.Owner, &ownerIDsStr); err != nil {
			continue
		}
		for _, s := range splitOwnerNames(ownerIDsStr) {
			if v, err := strconv.ParseInt(s, 10, 64); err == nil {
				p.OwnerIDs = append(p.OwnerIDs, v)
			}
		}
```

`GetProject`(projects.go 约 111-146 行前部的查询)同样处理:找到其 SELECT 与 scan,加 `(SELECT GROUP_CONCAT(po.user_id, ',') FROM project_owners po WHERE po.project_id = p.id)` 列并解析。GetProject 现按 id 查,若查询无 `p.` 别名则保持原样并追加列。

- [ ] **Step 5: UpdateProject 改造**

`projects.go` UpdateProject:请求体解析 owner_ids/owner(现 442-449 行 owner 保留逻辑替换):

```go
	// 负责人:owner_ids/owner 未携带 → 保留旧关联;携带 → 校验并覆盖
	var ownerIDs []int64
	ownerChanged := false
	if len(p.OwnerIDs) > 0 {
		ownerChanged = true
		for _, uid := range p.OwnerIDs {
			var n int
			h.db.QueryRow(`SELECT COUNT(*) FROM users WHERE id=? AND is_active=1`, uid).Scan(&n)
			if n == 0 {
				writeError(w, http.StatusBadRequest, "INVALID_OWNER", "项目所有者必须是现有活跃用户")
				return
			}
		}
		ownerIDs = p.OwnerIDs
	} else if strings.TrimSpace(p.Owner) != "" && p.Owner != oldOwner {
		ownerChanged = true
		ownerIDs, _ = resolveUserIDs(h.db, splitOwnerNames(p.Owner))
		if len(splitOwnerNames(p.Owner)) != len(ownerIDs) {
			writeError(w, http.StatusBadRequest, "INVALID_OWNER", "项目所有者必须从现有用户中选择")
			return
		}
	} else {
		ownerIDs, _ = loadProjectOwners(h.db, id) // 未携带:保留旧值
	}
	ownerSnapshot := strings.Join(ownerNamesOf(h.db, ownerIDs), "; ")
```

UPDATE 语句(现 450-456 行)的 owner 实参改为 `ownerSnapshot`。UPDATE 成功后、owner 改派块(现 463-474 行)替换为:

```go
	// 写关联表(权威)
	saveProjectOwners(h.db, id, ownerIDs)
	// 项目负责人变更 → 未开始(待开始/已延期)任务自动改派给新全部负责人;已完成/进行中保持不变
	if ownerChanged {
		rows, err := h.db.Query(`SELECT id FROM tasks WHERE project_id=? AND deleted_at IS NULL AND status IN ('open', 'delayed')`, id)
		if err == nil {
			var taskIDs []int64
			for rows.Next() {
				var tid int64
				if rows.Scan(&tid) == nil {
					taskIDs = append(taskIDs, tid)
				}
			}
			rows.Close()
			for _, tid := range taskIDs {
				saveTaskAssignees(h.db, tid, ownerIDs)
			}
		}
		// 新负责人加入项目成员(role=owner)
		for _, uid := range ownerIDs {
			h.db.Exec("INSERT OR IGNORE INTO project_members (project_id, user_id, role) VALUES (?, ?, 'owner')", id, uid)
		}
	}
```

- [ ] **Step 6: CopyProject 改造**

`projects.go` CopyProject:新项目插入后(现 283 行 `newID, _ := res.LastInsertId()` 之后)加:

```go
	// 复制项目负责人(关联表)
	srcOwnerIDs, _ := loadProjectOwners(h.db, srcID)
	saveProjectOwners(h.db, newID, srcOwnerIDs)
```

任务复制循环内(现 312-327 行)INSERT 后加:复制该任务关联表(旧 id 是 st.t.ID,新 id 是 nid):

```go
		if taIDs, _ := loadTaskAssignees(h.db, st.t.ID); len(taIDs) > 0 {
			saveTaskAssignees(h.db, nid, taIDs)
		}
```

- [ ] **Step 7: 跑测试确认通过(含既有项目测试回归)**

Run: `cd /f/projects/followITup/backend && go test ./internal/api/ -v`
Expected: PASS

- [ ] **Step 8: 提交**

```bash
git add backend/internal/api/projects.go backend/internal/api/multi_owner_test.go
git commit -m "feat:项目多负责人(owner_ids+文本兼容+改派+复制+成员同步)"
```

---

### Task 8: 到期提醒改 JOIN 关联表

**Files:**
- Modify: `backend/internal/mail/reminder.go`(RunDueReminder 59-72 行)

**Interfaces:**
- Consumes: Task 1 关联表
- Produces: 提醒按 task_assignees 匹配活跃用户,每人一封

- [ ] **Step 1: 替换查询**

`reminder.go` RunDueReminder 的查询(现 59-72 行)替换为:

```go
	rows, err := db.Query(`
		SELECT p.name, t.name, t.end_date, u.email
		FROM tasks t
		JOIN projects p ON p.id = t.project_id
		JOIN task_assignees ta ON ta.task_id = t.id
		JOIN users u ON u.id = ta.user_id AND u.is_active = 1
		WHERE t.deleted_at IS NULL AND p.deleted_at IS NULL
		  AND t.status != 'completed'
		  AND t.end_date != ''
		  AND t.end_date >= ?
		  AND t.end_date <= ?
		ORDER BY u.email, t.end_date`, today, deadline)
```

- [ ] **Step 2: 编译验证**

Run: `cd /f/projects/followITup/backend && go build ./... && go test ./...`
Expected: 全部通过

- [ ] **Step 3: 提交**

```bash
git add backend/internal/mail/reminder.go
git commit -m "feat:到期提醒按关联表匹配多负责人"
```

---

### Task 9: MultiUserSelect 组件 + 任务详情弹窗接入

**Files:**
- Create: `frontend/src/components/MultiUserSelect.tsx`
- Modify: `frontend/src/components/TaskDetailModal.tsx`
- Modify: `frontend/src/i18n/locales/zh.ts`、`frontend/src/i18n/locales/en.ts`
- Modify: `frontend/src/styles/components.css`(组件样式)

**Interfaces:**
- Produces:
  - `<MultiUserSelect users={[{id, display_name}]} selectedIds={number[]} onChange={(ids: number[]) => void} placeholder={string} />`
  - 任务弹窗保存提交 `assignee_ids: number[]`(不再传 `assignee`)
  - i18n 键:`common.remove`(移除)、`taskDetail.assigneePlaceholder`(选择负责人…)

- [ ] **Step 1: 创建 MultiUserSelect 组件**

`frontend/src/components/MultiUserSelect.tsx`:

```tsx
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

interface Props {
  users: { id: number; display_name: string }[];
  selectedIds: number[];
  onChange: (ids: number[]) => void;
  placeholder?: string;
}

/** 多选负责人:已选用户标签(可点 × 移除)+ 下拉勾选列表(点击 toggle,去重) */
export default function MultiUserSelect({ users, selectedIds, onChange, placeholder }: Props) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  // 点击组件外关闭下拉
  useEffect(() => {
    const onClick = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onClick);
    return () => document.removeEventListener("mousedown", onClick);
  }, []);

  const selected = users.filter((u) => selectedIds.includes(u.id));
  const toggle = (id: number) => {
    if (selectedIds.includes(id)) onChange(selectedIds.filter((x) => x !== id));
    else onChange([...selectedIds, id]);
  };

  return (
    <div className="multi-user-select" ref={rootRef}>
      <div className="mus-tags" onClick={() => setOpen((v) => !v)}>
        {selected.length === 0 && (
          <span className="mus-placeholder">{placeholder || t("taskDetail.assigneePlaceholder")}</span>
        )}
        {selected.map((u) => (
          <span key={u.id} className="mus-tag">
            {u.display_name}
            <button
              type="button"
              className="mus-tag-x"
              title={t("common.remove")}
              onClick={(e) => {
                e.stopPropagation();
                toggle(u.id);
              }}
            >
              ×
            </button>
          </span>
        ))}
      </div>
      {open && (
        <div className="mus-dropdown">
          {users.length === 0 ? (
            <div className="mus-empty">{t("common.noData")}</div>
          ) : (
            users.map((u) => (
              <label key={u.id} className="mus-option">
                <input
                  type="checkbox"
                  checked={selectedIds.includes(u.id)}
                  onChange={() => toggle(u.id)}
                />
                <span>{u.display_name}</span>
              </label>
            ))
          )}
        </div>
      )}
    </div>
  );
}
```

`frontend/src/styles/components.css` 追加:

```css
/* 多选负责人(下拉+标签) */
.multi-user-select { position: relative; min-height: 36px; }
.mus-tags {
  display: flex; flex-wrap: wrap; gap: 4px; align-items: center;
  min-height: 36px; padding: 4px 8px; border: 1px solid var(--border);
  border-radius: 6px; cursor: pointer; background: #fff;
}
.mus-tags:hover { border-color: var(--accent); }
.mus-placeholder { color: var(--text-muted); font-size: 13px; }
.mus-tag {
  display: inline-flex; align-items: center; gap: 4px;
  background: var(--accent-soft, rgba(64, 128, 128, 0.12)); color: var(--accent);
  border-radius: 4px; padding: 2px 6px; font-size: 12px;
}
.mus-tag-x { border: none; background: none; cursor: pointer; color: inherit; font-size: 13px; line-height: 1; padding: 0 2px; }
.mus-tag-x:hover { color: var(--danger); }
.mus-dropdown {
  position: absolute; top: calc(100% + 4px); left: 0; right: 0; z-index: 20;
  background: #fff; border: 1px solid var(--border); border-radius: 6px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08); max-height: 220px; overflow-y: auto;
}
.mus-option { display: flex; align-items: center; gap: 8px; padding: 8px 10px; cursor: pointer; font-size: 13px; }
.mus-option:hover { background: var(--bg-soft, #f5f5f4); }
.mus-option input { accent-color: var(--accent); }
.mus-empty { padding: 10px; color: var(--text-muted); font-size: 13px; }
```

- [ ] **Step 2: TaskDetailModal 接入**

`TaskDetailModal.tsx`:
1. 引入组件:`import MultiUserSelect from "./MultiUserSelect";`
2. state 替换:现 `const [assignee, setAssignee] = useState("")`(80 行)→ `const [assigneeIds, setAssigneeIds] = useState<number[]>([])`
3. 回显(132 行 `setAssignee(task.assignee || "")`)→ `setAssigneeIds(task.assignee_ids || [])`
4. 接口 Task 类型(约 10-30 行)加 `assignee_ids?: number[]`
5. payload(257 行 `assignee: assignee.trim(),`)→ `assignee_ids: assigneeIds,`
6. 渲染(504-515 行 select)替换为:

```tsx
        <div className="form-group">
          <label>{t("taskDetail.assignee")}</label>
          <MultiUserSelect
            users={users}
            selectedIds={assigneeIds}
            onChange={setAssigneeIds}
          />
        </div>
```

`users` state 现为 `{ id, display_name }[]`(76 行),接口匹配。

- [ ] **Step 3: i18n 键**

`frontend/src/i18n/locales/zh.ts` 追加:

```ts
  common: { ..., remove: "移除" },
  taskDetail: { ..., assigneePlaceholder: "选择负责人…" },
```

`frontend/src/i18n/locales/en.ts` 对应:

```ts
  common: { ..., remove: "Remove" },
  taskDetail: { ..., assigneePlaceholder: "Select assignee(s)…" },
```

(若 `common.remove` 已存在则跳过,以实际文件为准——追加前 grep 确认键不重复)

- [ ] **Step 4: 类型检查 + 构建**

Run: `cd /f/projects/followITup/frontend && npx tsc --noEmit; echo "TSC_EXIT=$?"`
Expected: `TSC_EXIT=0`

- [ ] **Step 5: 提交**

```bash
git add frontend/src/components/MultiUserSelect.tsx frontend/src/components/TaskDetailModal.tsx frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts frontend/src/styles/components.css
git commit -m "feat:MultiUserSelect多选组件+任务弹窗接入"
```

---

### Task 10: 看板——创建弹窗多选 + 卡片三行布局 + 我的待办双视角

**Files:**
- Modify: `frontend/src/pages/Dashboard.tsx`
- Modify: `frontend/src/styles/components.css`(卡片布局)
- Modify: `frontend/src/i18n/locales/zh.ts`、`en.ts`

**Interfaces:**
- Consumes: Task 9 的 MultiUserSelect
- Produces: 创建项目提交 `owner_ids`;卡片第二行 owner;我的待办顶部视角切换(「我负责的任务/我负责的项目」)重新拉取 `?view=`
- i18n 键:`dashboard.todoViewTask`、`dashboard.todoViewProject`

- [ ] **Step 1: 创建弹窗多选**

`Dashboard.tsx`:
1. 引入 `MultiUserSelect`
2. createForm state(26 行)owner 字段改为 ownerIds:

```tsx
  const [createForm, setCreateForm] = useState({ name: "", owner_ids: [] as number[], start_date: "", end_date: "", schedule_direction: "forward", description: "" });
```

3. 提交(181-195 行):`if (!createForm.owner.trim())` 校验改为 `if (createForm.owner_ids.length === 0)`;payload `owner: createForm.owner.trim()` 改为 `owner_ids: createForm.owner_ids`
4. 重置(170 行)同步 `owner_ids: []`
5. 渲染(635-650 行 select)替换为:

```tsx
              <div className="form-group">
                <label htmlFor="project-owner">{t("dashboard.ownerRequired")}</label>
                <MultiUserSelect
                  users={userOptions}
                  selectedIds={createForm.owner_ids}
                  onChange={(ids) => setCreateForm({ ...createForm, owner_ids: ids })}
                  placeholder={userOptions.length === 0 ? t("dashboard.ownerEmpty") : t("dashboard.ownerSelect")}
                />
                <span className="form-hint">{t("dashboard.ownerHint")}</span>
              </div>
```

`userOptions` 现为 `{ display_name; email }[]`(约 630 行前定义)——MultiUserSelect 需要 `{ id; display_name }[]`,需确认现有 userOptions 结构;若为 `{ display_name, email }` 则改拉取处为 `{ id, display_name }`(Dashboard 约 120 行 `api.get("/api/users")` 的 setUserOptions)。**注意**:Dashboard 中 userOptions 实际定义需 grep 确认后按实际结构调整为 `{ id: number; display_name: string }[]`。

- [ ] **Step 2: 卡片三行布局**

`Dashboard.tsx` 卡片(363-421 行):把 owner 从 header(381-384 行)移到 body 上方独立行:

```tsx
                  <div className="project-card-body">
                    {/* 第二行:负责人(多值分号分隔,超长省略+title) */}
                    <span className="project-owner" title={p.owner || t("dashboard.ownerTitle")}>
                      <span className="owner-icon">👤</span>
                      <span className="owner-name">{p.owner || "—"}</span>
                    </span>
                    <div className="project-card-progress">
                      {/* 原有进度条块不变 */}
                    </div>
                    {/* 截止/预计不变 */}
                  </div>
```

header 中删除原 owner span(381-384 行)。`components.css` 调整:

```css
/* 原 .project-owner 固定宽规则删除,改为弹性 */
.project-owner {
  display: flex; align-items: center; gap: 4px;
  font-size: 12px; color: var(--text-secondary); margin-bottom: 6px;
}
.project-owner .owner-name {
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 220px;
}
```

(检查现有 `.project-owner`/`.owner-name` 规则实际内容后整块替换;项目名行 `project-name` 不受影响)

- [ ] **Step 3: 我的待办双视角**

`Dashboard.tsx`:
1. state 加:`const [todoView, setTodoView] = useState<"task" | "project">("task");`
2. 拉取(136 行)改为 `api.get(`/api/tasks/mine?days=${todoDays}&view=${todoView}`)`;effect 依赖数组加 `todoView`
3. 我的待办区块标题(540 行)加视角切换:

```tsx
          <h3 className="section-title">{t("dashboard.sectionTodo")}</h3>
          {isLoggedIn && (
            <div className="todo-view-switch">
              <button
                className={`todo-view-btn${todoView === "task" ? " active" : ""}`}
                onClick={() => setTodoView("task")}
              >
                {t("dashboard.todoViewTask")}
              </button>
              <button
                className={`todo-view-btn${todoView === "project" ? " active" : ""}`}
                onClick={() => setTodoView("project")}
              >
                {t("dashboard.todoViewProject")}
              </button>
            </div>
          )}
```

`components.css` 追加:

```css
.todo-view-switch { display: inline-flex; gap: 2px; margin-left: 12px; vertical-align: middle; }
.todo-view-btn {
  border: 1px solid var(--border); background: #fff; color: var(--text-secondary);
  font-size: 12px; padding: 3px 10px; border-radius: 4px; cursor: pointer;
}
.todo-view-btn.active { background: var(--accent); border-color: var(--accent); color: #fff; }
```

- [ ] **Step 4: i18n 键**

zh.ts:`dashboard.todoViewTask: "我负责的任务"`,`dashboard.todoViewProject: "我负责的项目"`;en.ts:`"My tasks"` / `"My projects"`。追加前 grep 防重复。

- [ ] **Step 5: 类型检查 + 构建**

Run: `cd /f/projects/followITup/frontend && npx tsc --noEmit; echo "TSC_EXIT=$?"`
Expected: `TSC_EXIT=0`

- [ ] **Step 6: 提交**

```bash
git add frontend/src/pages/Dashboard.tsx frontend/src/styles/components.css frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts
git commit -m "feat:看板创建弹窗多选owner+卡片三行布局+待办双视角切换"
```

---

### Task 11: 项目详情页 owner 多选 + 资源视图多负责人分组

**Files:**
- Modify: `frontend/src/pages/ProjectDetail.tsx`
- Modify: `frontend/src/pages/Resources.tsx`
- Modify: `frontend/src/i18n/locales/zh.ts`、`en.ts`

**Interfaces:**
- Consumes: Task 9 MultiUserSelect
- Produces: 项目详情提交 `owner_ids`;资源视图多 owner 任务在每个 owner 分组重复出现

- [ ] **Step 1: ProjectDetail 接入多选**

`ProjectDetail.tsx`:
1. Project interface 加 `owner_ids?: number[]`
2. userOptions(24 行)→ `{ id: number; display_name: string }[]`;拉取处(34 行)已返回 id/display_name/email,调整映射
3. state 加 `const [ownerIds, setOwnerIds] = useState<number[]>([]);`;拉取后初始化(26-35 行 effect 内 setProject 后加 `setOwnerIds(p.owner_ids || [])`)
4. 保存函数(112-132 行 select)替换:

```tsx
        <label className="direction-date direction-owner">
          {t("projectDetail.owner")}
          <MultiUserSelect
            users={userOptions}
            selectedIds={ownerIds}
            onChange={setOwnerIds}
          />
        </label>
```

保存触发:MultiUserSelect onChange 每次变更即提交(与现状 select 即改即存一致)——在 onChange 回调里:

```tsx
        <label className="direction-date direction-owner">
          {t("projectDetail.owner")}
          <MultiUserSelect
            users={userOptions}
            selectedIds={ownerIds}
            onChange={async (ids) => {
              setOwnerIds(ids);
              try {
                await api.put(`/api/projects/${id}`, { ...project, owner_ids: ids });
                setProject({ ...project, owner_ids: ids });
              } catch (err: any) {
                alert(getErrorMessage(err, "common.unknownError"));
                setOwnerIds(project.owner_ids || []);
              }
            }}
          />
        </label>
```

5. 标签文字:现 113 行「项目所有者」为硬编码中文,顺带改为 `{t("projectDetail.owner")}`(i18n 键,zh 已有「项目所有者」对应翻译或新增;en 为 "Project owner(s)")。需 grep 确认 `projectDetail.owner` 键是否已存在,不存在则追加。

- [ ] **Step 2: Resources 多负责人分组**

`Resources.tsx`(40-47 行分组逻辑)替换:

```tsx
  // 分组:负责人(分号拆分,多值在每个分组重复)→ 叶子任务列表
  const groups = new Map<string, TaskRow[]>();
  for (const t of tasks) {
    if (!isLeaf(t)) continue; // 父任务不重复计入
    const owners = t.assignee ? t.assignee.split(";").map((s) => s.trim()).filter(Boolean) : [];
    if (owners.length === 0) owners.push(i18n.t("resources.unassigned"));
    for (const o of owners) {
      if (!groups.has(o)) groups.set(o, []);
      groups.get(o)!.push(t);
    }
  }
```

- [ ] **Step 3: i18n 键**

`projectDetail.owner`:zh 若无则加 `"项目负责人"`;en `"Project owner(s)"`。grep 确认后追加。

- [ ] **Step 4: 类型检查**

Run: `cd /f/projects/followITup/frontend && npx tsc --noEmit; echo "TSC_EXIT=$?"`
Expected: `TSC_EXIT=0`

- [ ] **Step 5: 提交**

```bash
git add frontend/src/pages/ProjectDetail.tsx frontend/src/pages/Resources.tsx frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts
git commit -m "feat:项目详情owner多选+资源视图多负责人分组"
```

---

### Task 12: 任务列表行内只读 + 甘特图列/过滤/批量条 + CSV 模板提示

**Files:**
- Modify: `frontend/src/pages/TaskListView.tsx`
- Modify: `frontend/src/pages/ProjectGantt.tsx`
- Modify: `frontend/src/components/ImportModal.tsx`
- Modify: `frontend/src/i18n/locales/zh.ts`、`en.ts`

**Interfaces:**
- Produces: assignee 单元格只读(分号文本);甘特图过滤条按任一 owner 匹配;批量条移除负责人项;CSV 模板负责人列示例含分号多值 + 提示

- [ ] **Step 1: TaskListView 行内只读**

`TaskListView.tsx`:
1. saveEdit 的 switch 删除 `case "assignee"`(123-125 行)
2. 单元格(312-331 行)去掉 onClick/class/cell-editable 与 editingCell 分支,只留:

```tsx
                <td title={t.assignee}>{t.assignee || "—"}</td>
```

(注释说明:多负责人后行内编辑体验差,编辑统一走详情弹窗)

- [ ] **Step 2: ProjectGantt 过滤与批量条**

`ProjectGantt.tsx`:
1. ownerOptions(237-241 行)改为拆分并集:

```tsx
  /** 负责人下拉选项(全部任务按分号拆分去重) */
  const ownerOptions = useMemo(
    () => Array.from(new Set(allTasks.flatMap((t) => (t.assignee || "").split(";").map((s) => s.trim()).filter(Boolean)))),
    [allTasks]
  );
```

2. 过滤判定(256 行)改为任一匹配:

```tsx
      if (ownerFilter !== "all") {
        const owners = (task.assignee || "").split(";").map((s) => s.trim());
        if (!owners.includes(ownerFilter)) return false;
      }
```

3. 批量条(1244-1253 行 assignee select)整块删除

4. assignee 列模板(537-540 行)加 title 防超长:

```tsx
      { name: "assignee_col", label: t("gantt.colAssignee"), width: 90, align: "center",
        template: (task: any) => task.assignee
          ? `<span title="${(task.assignee || "").replace(/"/g, "&quot;")}" style="font-size:11px;">${task.assignee}</span>`
          : '<span style="color:var(--text-muted);">—</span>' },
```

- [ ] **Step 3: ImportModal 模板与提示**

`ImportModal.tsx`(44-63 行模板区):负责人列示例值改分号多值,并在弹窗说明里加提示(找到表头渲染处加一行说明或改模板示例):

```tsx
      "任务名,WBS编号,工期(天),开始日期,负责人,进度(%),状态",
      "需求分析,1,3,2026-08-03,张三;李四,50%,进行中",   // 负责人支持多人,分号分隔
```

i18n 键:`importModal.assigneeHint: "负责人支持多人,分号分隔(如 张三;李四),需为系统用户"`(zh)/en 对应。找到弹窗中现有说明文案区域插入。

- [ ] **Step 4: i18n 键**(grep 防重复后追加)

zh.ts:`importModal.assigneeHint`;en.ts:`"Assignee supports multiple people, separated by ; (e.g. Zhang San; Li Si). Must be system users."`

- [ ] **Step 5: 类型检查 + 构建**

Run: `cd /f/projects/followITup/frontend && npx tsc --noEmit; echo "TSC_EXIT=$?"`
Expected: `TSC_EXIT=0`

- [ ] **Step 6: 提交**

```bash
git add frontend/src/pages/TaskListView.tsx frontend/src/pages/ProjectGantt.tsx frontend/src/components/ImportModal.tsx frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts
git commit -m "feat:列表/甘特图多负责人展示与过滤+CSV模板提示"
```

---

### Task 13: 全量构建 + 后端测试 + 浏览器实测

**Files:**
- Modify: 无(验证)

**Interfaces:**
- 验证全部任务交付物

- [ ] **Step 1: 后端全部测试**

Run: `cd /f/projects/followITup/backend && go test ./...`
Expected: 全部 PASS

- [ ] **Step 2: 前端类型 + 构建**

Run: `cd /f/projects/followITup/frontend && npx tsc --noEmit; echo "TSC_EXIT=$?"`
Expected: `TSC_EXIT=0`

Run: `cd /f/projects/followITup/frontend && npm run build`
Expected: 构建成功

- [ ] **Step 3: 复制产物 + 重建 exe + 重启**

```bash
rm -rf backend/cmd/server/frontend-dist && cp -r frontend/dist backend/cmd/server/frontend-dist
taskkill //IM followitup.exe //F 2>/dev/null || true
cd /f/projects/followITup/backend && go build -o followitup.exe ./cmd/server/
```

启动:`cd /f/projects/followITup/backend && ./followitup.exe config.yaml`(后台)

- [ ] **Step 4: 浏览器实测(中英双语)**

用浏览器验证:
1. 任务详情弹窗:负责人多选下拉+标签,选两人,保存后弹窗回显两人;重复点击同一用户标签不重复
2. 看板卡片:三行布局,第二行 owner 分号显示
3. 创建项目弹窗:owner 多选;创建后卡片显示多人
4. 我的待办:「我负责的任务」视角(默认)与「我负责的项目」视角切换,mine/starting 分区随视角变化
5. 资源视图:多负责人任务出现在每个负责人分组
6. 甘特图:assignee 列分号显示;过滤条选负责人,多负责人任务在任一 owner 过滤下都显示;批量条无负责人项
7. 任务列表:assignee 单元格只读
8. CSV 导入:负责人列 `张三;李四` 导入成功,不存在用户提示
9. 项目详情:owner 多选保存生效
10. 切 EN 语言重复 1/2/3 快速验证

- [ ] **Step 5: 提交收尾(如有遗留)与元数据更新**

```bash
# 若无遗留则跳过
git add . && git commit -m "chore:多负责人功能收尾" 2>/dev/null || true
```

更新 `.wolf/anatomy.md`(新文件:MultiUserSelect.tsx、multi_owner_test.go、migration_test.go、计划文档)、`.wolf/memory.md` 追加条目。

---

## Self-Review 记录

- **Spec 覆盖**:① 关联表+迁移→Task 1;② API(ids+文本兼容+校验)→Task 2/3/4/7;③ GetMyTasks 双视角→Task 5;④ CSV 分号多值+提示→Task 6;⑤ 到期提醒→Task 8;⑥ MultiUserSelect+任务弹窗→Task 9;⑦ 看板三行+创建弹窗+待办切换→Task 10;⑧ 项目详情+资源视图→Task 11;⑨ 甘特图/列表/导入→Task 12;⑩ 防呆(去重/迁移容错/悬空根治)→Task 1/2 内;⑪ 测试→各任务内;⑫ 软删不清理关联表→Task 7 说明(未删关联表)
- **已知偏差(实现时留意)**:Dashboard 的 userOptions 结构需按实际确认;`projectDetail.owner`/`common.remove`/`common.noData` 等 i18n 键 grep 确认防重复;Task 3 Step 4 中"正确实现"段已给出修正写法,以修正段为准
