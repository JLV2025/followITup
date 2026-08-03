# 基线对比功能 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 项目基线快照 + 甘特图基线条/实际执行条 + 看板偏差统计，回答"计划漂移了多少天、执行偏差多少"。

**Architecture:** 方案 A（基线列）：tasks 加 4 个 `baseline_*` 列 + projects 加 2 个元数据列；新 `baseline.go` 提供创建/清除/查询 API；前端 `addTaskLayer` 画两条 4px 细条（灰基线/浅绿实际）紧贴任务条上下边缘；看板沿用顶层任务时长加权口径对比。

**Tech Stack:** Go 1.22+ / chi / SQLite（modernc）/ React 18 + TypeScript / zustand / dhtmlx-gantt v10 / vite

**Spec:** `docs/superpowers/specs/2026-08-03-baseline-comparison-design.md`

## Global Constraints

- 所有注释、文档、提交信息使用简体中文
- 数据库日期始终 YYYY-MM-DD 字符串（modernc 不能 scan time.Time，Do-Not-Repeat 2026-07-30）
- 排程语义：看板进度=顶层任务时长加权（`parent_id IS NULL OR parent_id=0` 过滤，`SUM(dur×prog)/NULLIF(SUM(dur),0)`）
- 权限模型：**登录即可编辑**（RequireAuth），无 viewer 角色门禁 —— spec 的"viewer→403"用例据此调整为"未登录→401"
- 每个逻辑变更单独提交，提交信息中文
- 每次变更后跑测试：后端 `cd backend && go test ./...`，前端 `cd frontend && npx tsc --noEmit`
- 前端产物复制顺序（Do-Not-Repeat 2026-07-30）：先 `npm run build` → `cp -r frontend/dist backend/cmd/server/frontend-dist` → `go build`

---

### Task 1: 迁移 v4（基线列）+ 模型字段

**Files:**
- Modify: `backend/internal/db/sqlite.go`（migrations slice 末尾追加 `{4, ...}`）
- Modify: `backend/internal/models/models.go`（Task 加 4 字段、Project 加 2 字段）

**Interfaces:**
- Produces: `models.Task` 新增 `BaselineStartDate/ BaselineEndDate string`, `BaselineDurationDays int`, `BaselineProgressPct float64`（json: `baseline_start_date` 等）；`models.Project` 新增 `BaselineCreatedAt/ BaselineCreatedBy string`（json: `baseline_created_at`/`baseline_created_by`）

- [ ] **Step 1: 写失败测试** — 新建 `backend/internal/db/sqlite_test.go`：

```go
package db

import (
	"testing"
)

// 迁移 v4 后 tasks/projects 表应包含基线列
func TestMigrationV4BaselineColumns(t *testing.T) {
	d, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	cols := map[string]bool{}
	rows, err := d.Conn.Query(`PRAGMA table_info(tasks)`)
	if err != nil {
		t.Fatalf("PRAGMA tasks: %v", err)
	}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt *string
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err == nil {
			cols[name] = true
		}
	}
	rows.Close()

	for _, col := range []string{"baseline_start_date", "baseline_end_date", "baseline_duration_days", "baseline_progress_pct"} {
		if !cols[col] {
			t.Errorf("tasks 缺少列 %s", col)
		}
	}

	var projCols int
	d.Conn.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('projects')
		WHERE name IN ('baseline_created_at','baseline_created_by')`).Scan(&projCols)
	if projCols != 2 {
		t.Errorf("projects 缺少基线元数据列, 找到 %d/2", projCols)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd backend && go test ./internal/db/ -run TestMigrationV4BaselineColumns -v`
Expected: FAIL（缺列）

- [ ] **Step 3: 实现迁移** — `sqlite.go` migrations slice 末尾（v3 之后）追加：

```go
	{4, `
	ALTER TABLE tasks ADD COLUMN baseline_start_date TEXT;
	ALTER TABLE tasks ADD COLUMN baseline_end_date TEXT;
	ALTER TABLE tasks ADD COLUMN baseline_duration_days INTEGER;
	ALTER TABLE tasks ADD COLUMN baseline_progress_pct REAL;
	ALTER TABLE projects ADD COLUMN baseline_created_at TEXT;
	ALTER TABLE projects ADD COLUMN baseline_created_by TEXT;
	`},
```

- [ ] **Step 4: 模型加字段** — `models.go` Task struct 在 `ProgressPct` 后加：

```go
	BaselineStartDate    string    `json:"baseline_start_date"`
	BaselineEndDate      string    `json:"baseline_end_date"`
	BaselineDurationDays int       `json:"baseline_duration_days"`
	BaselineProgressPct  float64   `json:"baseline_progress_pct"`
```

Project struct 在 `Status` 后加：

```go
	BaselineCreatedAt string `json:"baseline_created_at"`
	BaselineCreatedBy string `json:"baseline_created_by"`
```

- [ ] **Step 5: 跑测试确认通过**

Run: `cd backend && go test ./internal/db/ -run TestMigrationV4BaselineColumns -v`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add backend/internal/db/sqlite.go backend/internal/db/sqlite_test.go backend/internal/models/models.go
git commit -m "迁移v4:基线列(tasks 4列 + projects 2列元数据)"
```

---

### Task 2: 实际日期自动填充（纯函数 + UpdateTask 集成）

**Files:**
- Modify: `backend/internal/api/tasks.go`（UpdateTask 加自动填充）
- Test: `backend/internal/api/baseline_test.go`（新建，后续任务复用）

**Interfaces:**
- Produces: 纯函数 `fillActualDates(status, actualStart, actualEnd string) (string, string)`：status=in_progress 且 actualStart 空 → 返回今天/原值；status=completed 且 actualEnd 空 → 返回原值/今天；否则原样返回。今天 = `time.Now().Format("2006-01-02")`

- [ ] **Step 1: 写失败测试** — 新建 `backend/internal/api/baseline_test.go`：

```go
package api

import (
	"strings"
	"testing"
	"time"
)

// 实际日期自动填充：进行中→记实际开始；完成→记实际结束；已有值不覆盖
func TestFillActualDates(t *testing.T) {
	today := time.Now().Format("2006-01-02")

	cases := []struct {
		name         string
		status       string
		actualStart  string
		actualEnd    string
		wantStart    string
		wantEnd      string
	}{
		{"变为进行中记实际开始", "in_progress", "", "", today, ""},
		{"变为完成记实际结束", "completed", "", "", "", today},
		{"已有实际开始不覆盖", "in_progress", "2026-07-01", "", "2026-07-01", ""},
		{"已有实际结束不覆盖", "completed", "", "2026-07-20", "", "2026-07-20"},
		{"其他状态不变", "open", "", "", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotS, gotE := fillActualDates(c.status, c.actualStart, c.actualEnd)
			if gotS != c.wantStart || gotE != c.wantEnd {
				t.Errorf("fillActualDates(%q,%q,%q) = (%q,%q), want (%q,%q)",
					c.status, c.actualStart, c.actualEnd, gotS, gotE, c.wantStart, c.wantEnd)
			}
		})
	}
	_ = strings.TrimSpace // 保留占位避免未用导入
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd backend && go test ./internal/api/ -run TestFillActualDates -v`
Expected: FAIL（fillActualDates 未定义）

- [ ] **Step 3: 实现纯函数** — `tasks.go` 文件内加：

```go
// fillActualDates 实际日期自动填充：
// status 变为 in_progress 且无实际开始 → 记今天；变为 completed 且无实际结束 → 记今天；已有值不覆盖
func fillActualDates(status, actualStart, actualEnd string) (string, string) {
	today := time.Now().Format("2006-01-02")
	if status == "in_progress" && actualStart == "" {
		actualStart = today
	}
	if status == "completed" && actualEnd == "" {
		actualEnd = today
	}
	return actualStart, actualEnd
}
```

（确认 `time` 已 import，未 import 则添加。）

- [ ] **Step 4: UpdateTask 集成** — `UpdateTask` 内 decode 之后、校验 parent 之前插入：

```go
	// 实际日期自动填充（需先读旧值，已有值不覆盖）
	var oldActualStart, oldActualEnd string
	h.db.QueryRow(`SELECT actual_start, actual_end FROM tasks WHERE id=? AND deleted_at IS NULL`, taskID).Scan(&oldActualStart, &oldActualEnd)
	t.ActualStart, t.ActualEnd = fillActualDates(t.Status, oldActualStart, oldActualEnd)
```

（`t.ActualStart/ActualEnd` 随后在 UPDATE SET 中原样使用，保持现有 SQL 不变。）

- [ ] **Step 5: 跑测试确认通过**

Run: `cd backend && go test ./internal/api/ -run TestFillActualDates -v && go test ./...`
Expected: 新增用例 PASS，既有测试全过

- [ ] **Step 6: 提交**

```bash
git add backend/internal/api/tasks.go backend/internal/api/baseline_test.go
git commit -m "实际日期自动填充:进行中/完成状态自动记实际日期,已有值不覆盖"
```

---

### Task 3: baseline.go API（创建/清除/查询 + 路由注册 + WS 广播）

**Files:**
- Create: `backend/internal/api/baseline.go`
- Modify: `backend/internal/server/server.go`（注册路由）
- Modify: `backend/internal/api/baseline_test.go`（追加用例）

**Interfaces:**
- Consumes: `db *sql.DB`, `mid *auth.Middleware`, `hub *ws.Hub`；`h.hub.BroadcastTaskUpdate(projectID, userID, userName, taskID, data)`（tasks.go:341 同款调用）
- Produces: `NewBaselineHandler(db *sql.DB, mid *auth.Middleware, hub *ws.Hub) *BaselineHandler`；方法 `CreateBaseline/ DeleteBaseline/ GetBaseline`；事务函数 `createBaselineTx(db *sql.DB, projectID int64, userID int64, userName string) error`、`clearBaselineTx(db *sql.DB, projectID int64) error`（handler 薄封装，Tx 函数可独立测试）

- [ ] **Step 1: 写失败测试** — `baseline_test.go` 追加：

```go
import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
	_ "modernc.org/sqlite"
)

// 测试用临时 DB（迁移自动执行）
func testBaselineDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	conn, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	// 手动执行与 db.Open 相同的迁移（复用 db 包）
	d, err := openAndMigrate(dir) // 见下方说明
	...
}
```

> 说明：`db.Open(dataDir)` 为 `*db.DB`（`Conn *sql.DB` 公开），直接用它即可，无需 `openAndMigrate`：

```go
func testBaselineDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d.Conn
}
```

用例：

```go
// 创建基线：所有任务 baseline_* = 当前值，projects 元数据写入
func TestCreateBaselineSnapshot(t *testing.T) {
	conn := testBaselineDB(t)
	now := time.Now().Format("2006-01-02")

	var pid int64
	conn.QueryRow(`INSERT INTO projects (name, start_date, end_date, status) VALUES ('测试项目', ?, ?, 'active')`, now, now).LastInsertId()
	// projects 无 LastInsertId 则用 Exec+QueryRow 方式取 id
	conn.Exec(`INSERT INTO projects (name, start_date, end_date, status) VALUES ('测试项目', ?, ?, 'active')`, now, now)
	conn.QueryRow(`SELECT id FROM projects ORDER BY id DESC LIMIT 1`).Scan(&pid)

	conn.Exec(`INSERT INTO tasks (project_id, name, start_date, end_date, duration_days, progress_pct, status) VALUES (?, 'A', ?, ?, 5, 40, 'in_progress')`, pid, now, now)
	conn.Exec(`INSERT INTO tasks (project_id, name, start_date, end_date, duration_days, progress_pct, status) VALUES (?, 'B', ?, ?, 3, 100, 'completed')`, pid, now, now)

	if err := createBaselineTx(conn, pid, 1, "admin"); err != nil {
		t.Fatalf("createBaselineTx: %v", err)
	}

	var cnt int
	conn.QueryRow(`SELECT COUNT(*) FROM tasks WHERE project_id=? AND deleted_at IS NULL
		AND baseline_start_date IS NOT NULL AND baseline_progress_pct = progress_pct`, pid).Scan(&cnt)
	if cnt != 2 {
		t.Errorf("基线快照任务数 = %d, want 2", cnt)
	}
	var createdBy string
	conn.QueryRow(`SELECT baseline_created_by FROM projects WHERE id=?`, pid).Scan(&createdBy)
	if createdBy != "admin" {
		t.Errorf("baseline_created_by = %q, want admin", createdBy)
	}
}

// 清除基线：baseline_* 全 NULL
func TestClearBaseline(t *testing.T) {
	conn := testBaselineDB(t)
	now := time.Now().Format("2006-01-02")
	var pid int64
	conn.Exec(`INSERT INTO projects (name, start_date, end_date, status) VALUES ('测试项目', ?, ?, 'active')`, now, now)
	conn.QueryRow(`SELECT id FROM projects ORDER BY id DESC LIMIT 1`).Scan(&pid)
	conn.Exec(`INSERT INTO tasks (project_id, name, start_date, end_date, duration_days, progress_pct) VALUES (?, 'A', ?, ?, 5, 40)`, pid, now, now)

	if err := createBaselineTx(conn, pid, 1, "admin"); err != nil {
		t.Fatalf("createBaselineTx: %v", err)
	}
	if err := clearBaselineTx(conn, pid); err != nil {
		t.Fatalf("clearBaselineTx: %v", err)
	}

	var cnt int
	conn.QueryRow(`SELECT COUNT(*) FROM tasks WHERE project_id=? AND baseline_start_date IS NOT NULL`, pid).Scan(&cnt)
	if cnt != 0 {
		t.Errorf("清除后仍有基线数据, %d 行", cnt)
	}
	var hasMeta int
	conn.QueryRow(`SELECT COUNT(*) FROM projects WHERE id=? AND baseline_created_at IS NOT NULL`, pid).Scan(&hasMeta)
	if hasMeta != 0 {
		t.Error("清除后元数据未置 NULL")
	}
}
```

（两个用例都依赖 `db.Open` 自动迁移——Task 1 的迁移需先合并，Task 3 才能过。）

- [ ] **Step 2: 跑测试确认失败**

Run: `cd backend && go test ./internal/api/ -run 'TestCreateBaselineSnapshot|TestClearBaseline' -v`
Expected: FAIL（createBaselineTx/clearBaselineTx 未定义）

- [ ] **Step 3: 实现 baseline.go**：

```go
package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/yourorg/followitup/backend/internal/auth" // 按项目实际模块路径
	"github.com/yourorg/followitup/backend/internal/ws"
)

// BaselineHandler 基线 API：创建/清除/查询项目当前基线
type BaselineHandler struct {
	db  *sqlDB // 实际为 *sql.DB
	mid *auth.Middleware
	hub *ws.Hub
}

func NewBaselineHandler(db *sql.DB, mid *auth.Middleware, hub *ws.Hub) *BaselineHandler {
	return &BaselineHandler{db: db, mid: mid, hub: hub}
}

func (h *BaselineHandler) RegisterRoutes(r chi.Router) {
	r.Get("/api/projects/{id}/baseline", h.GetBaseline)
	r.Group(func(r chi.Router) {
		r.Use(h.mid.RequireAuth)
		r.Post("/api/projects/{id}/baseline", h.CreateBaseline)
		r.Delete("/api/projects/{id}/baseline", h.DeleteBaseline)
	})
}
```

> 说明：`createBaselineTx` 事务函数实现（快照 UPDATE + 元数据写入）与 `clearBaselineTx`（置 NULL）见下一步；handler 层做参数解析（chi.URLParam "id"）、调用 Tx 函数、成功后 `h.hub.BroadcastTaskUpdate(projectID, userID, userName, 0, nil)` 广播刷新，失败 `writeError`。`userID/userName` 从 `auth.GetUserID` / 用户查询取得（参照 tasks.go 的既有取法）。模块路径以 `go.mod` 为准，实际实现时替换。

- [ ] **Step 4: 实现事务函数** — `baseline.go` 内：

```go
// createBaselineTx 快照当前任务排程字段到基线列（事务内）
func createBaselineTx(db *sql.DB, projectID, userID int64, userName string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`UPDATE tasks SET
		baseline_start_date = start_date,
		baseline_end_date = end_date,
		baseline_duration_days = duration_days,
		baseline_progress_pct = progress_pct
		WHERE project_id = ? AND deleted_at IS NULL`, projectID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE projects SET
		baseline_created_at = ?, baseline_created_by = ?
		WHERE id = ?`, time.Now().Format("2006-01-02 15:04:05"), userName, projectID); err != nil {
		return err
	}
	return tx.Commit()
}

// clearBaselineTx 清除基线（事务内）
func clearBaselineTx(db *sql.DB, projectID int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`UPDATE tasks SET
		baseline_start_date = NULL, baseline_end_date = NULL,
		baseline_duration_days = NULL, baseline_progress_pct = NULL
		WHERE project_id = ?`, projectID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE projects SET baseline_created_at = NULL, baseline_created_by = NULL WHERE id = ?`, projectID); err != nil {
		return err
	}
	return tx.Commit()
}
```

- [ ] **Step 5: 注册路由** — `server.go` taskHandler 注册之后：

```go
	// 注册基线 API（注入 Hub 支持实时广播）
	baselineHandler := api.NewBaselineHandler(database.Conn, authMid, wsHub)
	baselineHandler.RegisterRoutes(r)
```

- [ ] **Step 6: 跑测试确认通过**

Run: `cd backend && go test ./internal/api/ -run 'TestCreateBaselineSnapshot|TestClearBaseline' -v && go test ./...`
Expected: PASS + 既有全过

- [ ] **Step 7: 提交**

```bash
git add backend/internal/api/baseline.go backend/internal/server/server.go backend/internal/api/baseline_test.go
git commit -m "基线API:创建/清除/查询 + WS广播刷新"
```

---

### Task 4: DashboardStats + ProjectList 基线统计

**Files:**
- Modify: `backend/internal/api/projects.go`
- Modify: `backend/internal/api/baseline_test.go`（追加聚合用例）

**Interfaces:**
- Produces: `DashboardStats` 响应新增字段 `BaselineProgress float64`（json: `baseline_progress`，顶层任务 baseline 口径加权完成率，无基线时 0）；`ProjectSummary` 新增 `BaselineEnd string`（json: `baseline_end`，项目内 MAX(baseline_end_date)）、`BaselineCreatedAt/By string`、`DelayDays int`（json: `delay_days`，= MAX(end_date) − MAX(baseline_end_date)，无基线时 0）

- [ ] **Step 1: 写失败测试** — `baseline_test.go` 追加聚合用例：

```go
// 聚合统计：顶层任务 baseline 加权完成率 + 项目偏差天数
func TestBaselineAggregates(t *testing.T) {
	conn := testBaselineDB(t)
	now := time.Now().Format("2006-01-02")
	var pid int64
	conn.Exec(`INSERT INTO projects (name, start_date, end_date, status) VALUES ('测试项目', ?, ?, 'active')`, now, now)
	conn.QueryRow(`SELECT id FROM projects ORDER BY id DESC LIMIT 1`).Scan(&pid)

	// 顶层任务 A: 10天 40%进度；子任务 B: 5天 100%（应被过滤）；顶层任务 C: 无基线列但未打基线
	conn.Exec(`INSERT INTO tasks (project_id, name, start_date, end_date, duration_days, progress_pct, status) VALUES (?, 'A', ?, ?, 10, 40, 'in_progress')`, pid, now, now)
	var aID int64
	conn.QueryRow(`SELECT id FROM tasks WHERE project_id=? AND name='A'`, pid).Scan(&aID)
	conn.Exec(`INSERT INTO tasks (project_id, parent_id, name, start_date, end_date, duration_days, progress_pct, status) VALUES (?, ?, 'B', ?, ?, 5, 100, 'completed')`, pid, aID, now, now)

	if err := createBaselineTx(conn, pid, 1, "admin"); err != nil {
		t.Fatalf("createBaselineTx: %v", err)
	}

	// 基线完成率 = 40%（A 10天×40% / 10天），B 不参与
	var bp float64
	conn.QueryRow(`SELECT COALESCE(SUM(baseline_progress_pct * baseline_duration_days) / NULLIF(SUM(baseline_duration_days), 0), 0)
		FROM tasks WHERE project_id=? AND deleted_at IS NULL AND (parent_id IS NULL OR parent_id = 0)`, pid).Scan(&bp)
	if bp != 40 {
		t.Errorf("baseline 完成率 = %v, want 40", bp)
	}

	// 偏差天数：把 A 的当前结束改为基线后 +3 天
	var bEnd string
	conn.QueryRow(`SELECT baseline_end_date FROM tasks WHERE id=?`, aID).Scan(&bEnd)
	end := time.Now().AddDate(0, 0, 3).Format("2006-01-02")
	conn.Exec(`UPDATE tasks SET end_date=? WHERE id=?`, end, aID)
	var delay int
	conn.QueryRow(`SELECT CAST(julianday(MAX(end_date)) - julianday(MAX(baseline_end_date)) AS INTEGER)
		FROM tasks WHERE project_id=? AND deleted_at IS NULL AND MAX(baseline_end_date) IS NOT NULL`, pid).Scan(&delay)
	if delay != 3 {
		t.Errorf("偏差天数 = %d, want 3", delay)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd backend && go test ./internal/api/ -run TestBaselineAggregates -v`
Expected: 失败（测试内部断言失败属正常——本用例直接验证 SQL 口径；实现阶段将 SQL 挪入 handler）

- [ ] **Step 3: DashboardStats 加基线完成率** — `projects.go` 第 76 行整体完成率查询旁追加（对齐相同过滤条件 `+ filter`）：

```go
	// 基线完成率（顶层任务时长加权，baseline 口径）
	var baselineProgress float64
	h.db.QueryRow(`SELECT COALESCE(SUM(t.baseline_progress_pct * t.baseline_duration_days) / NULLIF(SUM(t.baseline_duration_days), 0), 0)
		FROM tasks t WHERE t.project_id IN (SELECT id FROM projects p WHERE p.deleted_at IS NULL AND p.status = 'active'`+filter+`)
		AND t.deleted_at IS NULL AND (t.parent_id IS NULL OR t.parent_id = 0)`).Scan(&baselineProgress)
```

响应结构中新增 `BaselineProgress float64 \`json:"baseline_progress"\`` 并赋 `baselineProgress`。

- [ ] **Step 4: ProjectList 加基线字段** — `ProjectSummary` 加：

```go
		BaselineEnd         string  `json:"baseline_end"`
		BaselineCreatedAt   string  `json:"baseline_created_at"`
		BaselineCreatedBy   string  `json:"baseline_created_by"`
		DelayDays           int     `json:"delay_days"`
```

主查询 SELECT 加 `p.baseline_created_at, p.baseline_created_by`，Scan 追加两参数；进度 QueryRow 之后加：

```go
		// 基线项目结束 + 偏差天数
		h.db.QueryRow(`SELECT MAX(baseline_end_date) FROM tasks WHERE project_id = ? AND deleted_at IS NULL`, p.ID).Scan(&p.BaselineEnd)
		if p.BaselineEnd != "" {
			h.db.QueryRow(`SELECT CAST(julianday(MAX(end_date)) - julianday(MAX(baseline_end_date)) AS INTEGER)
				FROM tasks WHERE project_id = ? AND deleted_at IS NULL AND MAX(baseline_end_date) IS NOT NULL`, p.ID).Scan(&p.DelayDays)
		}
```

- [ ] **Step 5: 跑测试确认通过**

Run: `cd backend && go test ./...`
Expected: 全过

- [ ] **Step 6: 提交**

```bash
git add backend/internal/api/projects.go backend/internal/api/baseline_test.go
git commit -m "看板基线统计:整体完成率加baseline口径 + 项目列表延迟天数"
```

---

### Task 5: 前端适配层 + ganttStore（字段透传 + 基线 actions）

**Files:**
- Modify: `frontend/src/api/gantt-adapter.ts`
- Modify: `frontend/src/stores/ganttStore.ts`

**Interfaces:**
- Consumes: `toGanttTask(t, readonly)` 透传基线字段
- Produces: `GanttTask` 新增 `baseline_start_date?/ baseline_end_date?/ actual_start?/ actual_end?: string`；`useGanttStore` 新增 `baselineMeta: { created_at: string; created_by: string; task_count: number } | null`、`fetchBaselineMeta(projectId): Promise<void>`、`createBaseline(projectId): Promise<boolean>`、`clearBaseline(projectId): Promise<boolean>`

- [ ] **Step 1: 适配层加字段** — `gantt-adapter.ts` GanttTask 接口（`constraint_date` 后）加：

```ts
  baseline_start_date?: string;
  baseline_end_date?: string;
  actual_start?: string;
  actual_end?: string;
```

`toGanttTask` return 对象（`constraint_date` 行后）加：

```ts
    baseline_start_date: t.baseline_start_date || "",
    baseline_end_date: t.baseline_end_date || "",
    actual_start: t.actual_start || "",
    actual_end: t.actual_end || "",
```

- [ ] **Step 2: store 加基线状态与 actions** — `ganttStore.ts` ：

```ts
export interface BaselineMeta {
  created_at: string;
  created_by: string;
  task_count: number;
}
```

GanttState 接口加：

```ts
  baselineMeta: BaselineMeta | null;
  fetchBaselineMeta: (projectId: number) => Promise<void>;
  createBaseline: (projectId: number) => Promise<boolean>;
  clearBaseline: (projectId: number) => Promise<boolean>;
```

实现（`focusMap: {},` 后加 state；fetchData 之后加 actions）：

```ts
  baselineMeta: null,

  fetchBaselineMeta: async (projectId) => {
    try {
      const res = await api.get(`/api/projects/${projectId}/baseline`);
      set({ baselineMeta: res.data.data || null });
    } catch {
      set({ baselineMeta: null });
    }
  },

  createBaseline: async (projectId) => {
    try {
      await api.post(`/api/projects/${projectId}/baseline`);
      const s = get();
      await s.fetchData(projectId, s.readonly);
      await s.fetchBaselineMeta(projectId);
      return true;
    } catch {
      return false;
    }
  },

  clearBaseline: async (projectId) => {
    try {
      await api.delete(`/api/projects/${projectId}/baseline`);
      const s = get();
      await s.fetchData(projectId, s.readonly);
      set({ baselineMeta: null });
      return true;
    } catch {
      return false;
    }
  },
```

- [ ] **Step 3: 验证编译**

Run: `cd frontend && npx tsc --noEmit`
Expected: 无错误

- [ ] **Step 4: 提交**

```bash
git add frontend/src/api/gantt-adapter.ts frontend/src/stores/ganttStore.ts
git commit -m "前端基线字段透传 + ganttStore基线actions"
```

---

### Task 6: ProjectGantt 基线绘制层 + 工具栏基线下拉

**Files:**
- Modify: `frontend/src/pages/ProjectGantt.tsx`
- Modify: `frontend/src/styles/components.css`（基线菜单样式）

**Interfaces:**
- Consumes: `useGanttStore` 的 `baselineMeta / fetchBaselineMeta / createBaseline / clearBaseline`；GanttTask 的 `baseline_start_date/actual_start/actual_end` 字段
- Produces: 甘特图上基线条（`#6B7280`）与实际条（`#86EFAC`）紧贴任务条上下边缘；工具栏"基线"下拉（创建/信息/清除）

- [ ] **Step 1: 基线绘制层** — `ProjectGantt.tsx` 中协作聚焦层（第 310 行 `// 协作聚焦层` 之前）插入：

```ts
    // 基线层：基线条紧贴任务条顶边、实际执行条紧贴任务条底边（均在 28px 行内）
    if (typeof (gantt as any).addTaskLayer === "function") {
      (gantt as any).addTaskLayer(function drawBaseline(task: Record<string, any>): any {
        if (!task.baseline_start_date || !task.baseline_end_date) return false;
        const startX = (gantt as any).posFromDate(new Date(task.baseline_start_date));
        const endX = (gantt as any).posFromDate(new Date(task.baseline_end_date));
        const el = document.createElement("div");
        el.style.cssText = `position:absolute; left:${startX}px; top:0px; width:${Math.max(2, endX - startX)}px; height:4px; background:#6B7280; pointer-events:none;`;
        return el;
      });
      (gantt as any).addTaskLayer(function drawActual(task: Record<string, any>): any {
        if (!task.actual_start) return false;
        const startX = (gantt as any).posFromDate(new Date(task.actual_start));
        const endX = (gantt as any).posFromDate(new Date(task.actual_end || task.end_date));
        const el = document.createElement("div");
        el.style.cssText = `position:absolute; left:${startX}px; bottom:0px; width:${Math.max(2, endX - startX)}px; height:4px; background:#86EFAC; pointer-events:none;`;
        return el;
      });
    }
```

> 说明：`addTaskLayer` 回调的坐标系与协作聚焦层一致（相对任务行）；`top:0px`/`bottom:0px` 先实现紧贴语义，若实测与任务条边缘有像素偏差（任务条约 20px 高居中于 28px 行），在浏览器中微调（改 `top:-4px`/`bottom:-4px` 或 `calc`），验证步骤包含浏览器目检。

- [ ] **Step 2: 工具栏基线下拉** — `gantt-toolbar-right` 缩放控件前插入按钮 + 下拉。组件顶部加 state：`const [baselineMenuOpen, setBaselineMenuOpen] = useState(false);`，useEffect 中项目加载后调用 `fetchBaselineMeta(projectId)`。JSX：

```tsx
        <div className="baseline-menu-wrap">
          <button
            className={`btn-zoom btn-baseline${baselineMeta ? " has-baseline" : ""}`}
            onClick={() => setBaselineMenuOpen(!baselineMenuOpen)}
            title="基线管理"
          >
            基线{baselineMeta ? ` ${baselineMeta.created_at.slice(5, 10)}` : ""} ▾
          </button>
          {baselineMenuOpen && (
            <div className="baseline-menu">
              {baselineMeta ? (
                <>
                  <div className="baseline-menu-info">
                    创建: {baselineMeta.created_at} · {baselineMeta.created_by}
                    <br />快照 {baselineMeta.task_count} 个任务
                  </div>
                  <button
                    className="baseline-menu-item"
                    onClick={async () => {
                      if (!window.confirm("重新创建基线将覆盖当前基线，确定？")) return;
                      const ok = await createBaseline(projectId);
                      if (ok) setBaselineMenuOpen(false);
                    }}
                  >
                    重新创建基线
                  </button>
                  <button
                    className="baseline-menu-item danger"
                    onClick={async () => {
                      if (!window.confirm("清除基线后无法恢复，确定？")) return;
                      const ok = await clearBaseline(projectId);
                      if (ok) setBaselineMenuOpen(false);
                    }}
                  >
                    清除基线
                  </button>
                </>
              ) : (
                <button
                  className="baseline-menu-item"
                  onClick={async () => {
                    const ok = await createBaseline(projectId);
                    if (ok) setBaselineMenuOpen(false);
                  }}
                >
                  创建基线（快照当前计划）
                </button>
              )}
            </div>
          )}
        </div>
```

（`baselineMeta / fetchBaselineMeta / createBaseline / clearBaseline` 从 `useGanttStore` 解构。）

- [ ] **Step 3: 下拉样式** — `components.css` 加：

```css
/* 基线管理下拉 */
.baseline-menu-wrap { position: relative; }
.baseline-menu { position: absolute; right: 0; top: 34px; background: #fff; border: 1px solid #e5e7eb;
  border-radius: 8px; box-shadow: 0 4px 12px rgba(0,0,0,.08); padding: 8px; z-index: 30; min-width: 200px; }
.baseline-menu-info { font-size: 12px; color: #6b7280; padding: 4px 8px 8px; line-height: 1.6; }
.baseline-menu-item { display: block; width: 100%; text-align: left; padding: 6px 8px; font-size: 13px;
  background: none; border: none; border-radius: 6px; cursor: pointer; color: #1a1a1a; }
.baseline-menu-item:hover { background: #f3f4f6; }
.baseline-menu-item.danger { color: #ef4444; }
.btn-baseline.has-baseline { background: #2c6e6a; color: #fff; }
```

- [ ] **Step 4: 验证编译 + 浏览器目检**

Run: `cd frontend && npx tsc --noEmit`
Expected: 无错误

启动后端 + 前端（或构建 exe 后浏览器打开 `/project/:id`）：
1. 无基线：工具栏"基线 ▾"→ 创建基线
2. 创建后任务条顶边出现灰色细条（与任务条紧贴无间隙）
3. 把某任务状态改为 in_progress / completed → 任务条底边出现浅绿实际条
4. 基线按钮显示创建日期；下拉可"清除基线"，确认后细条消失
5. 若细条与任务条边缘有像素间隙，微调 `top`/`bottom` 值（见 Step 1 说明）

- [ ] **Step 5: 提交**

```bash
git add frontend/src/pages/ProjectGantt.tsx frontend/src/styles/components.css
git commit -m "甘特图基线层:灰基线+浅绿实际条紧贴任务条 + 工具栏基线下拉"
```

---

### Task 7: TaskDetailModal 实际日期 + 基线信息

**Files:**
- Modify: `frontend/src/components/TaskDetailModal.tsx`

**Interfaces:**
- Consumes: GanttTask 的 `actual_start/actual_end/baseline_start_date/baseline_end_date`（`updateTask` 保存时透传）
- Produces: 「实际开始 / 实际结束」日期输入；有基线时显示 `基线: MM-DD ~ MM-DD` + `Δ +N天` 徽标（当前开始 − 基线开始，`frontend/src/utils/date.ts` 已有格式化工具）

- [ ] **Step 1: 弹窗加实际日期输入** — 「日期与进度」section 内、`结束日期` form-group 之后追加：

```tsx
        <div className="form-row">
          <div className="form-group">
            <label>实际开始</label>
            <input type="date" value={actualStart} onChange={(e) => setActualStart(e.target.value)} />
          </div>
          <div className="form-group">
            <label>实际结束</label>
            <input type="date" value={actualEnd} onChange={(e) => setActualEnd(e.target.value)} />
          </div>
        </div>
        {baselineStartDate && (
          <div className="form-row baseline-diff-row">
            <span className="baseline-diff">
              基线: {formatShort(baselineStartDate)} ~ {formatShort(baselineEndDate)}
              <em className={`baseline-diff-badge ${diffDays > 0 ? "neg" : diffDays < 0 ? "pos" : ""}`}>
                Δ {diffDays > 0 ? `+${diffDays}` : diffDays} 天
              </em>
            </span>
          </div>
        )}
```

- [ ] **Step 2: 弹窗 state 接线** — 弹窗初始化处加：

```ts
  const [actualStart, setActualStart] = useState(task.actual_start || "");
  const [actualEnd, setActualEnd] = useState(task.actual_end || "");
  const baselineStartDate = task.baseline_start_date;
  const baselineEndDate = task.baseline_end_date;
  const diffDays = baselineStartDate && task.start_date
    ? Math.round((new Date(task.start_date).getTime() - new Date(baselineStartDate).getTime()) / 86400000)
    : 0;
```

保存时 `updateTask(id, { ...existing, actual_start: actualStart, actual_end: actualEnd }, projectId)`（沿用现有保存调用，追加两个字段）；`formatShort` 从 `frontend/src/utils/date.ts` 导入（该文件已有 MM-DD 类格式化，按实际导出名使用）。

- [ ] **Step 3: 徽标样式** — `components.css` 加：

```css
/* 基线偏差徽标 */
.baseline-diff-row { align-items: center; }
.baseline-diff { font-size: 12px; color: #6b7280; display: flex; align-items: center; gap: 8px; }
.baseline-diff-badge { font-style: normal; font-weight: 600; padding: 1px 6px; border-radius: 4px; background: #f3f4f6; }
.baseline-diff-badge.neg { color: #ef4444; background: #fef2f2; }
.baseline-diff-badge.pos { color: #15803d; background: #f0fdf4; }
```

- [ ] **Step 4: 验证编译**

Run: `cd frontend && npx tsc --noEmit`
Expected: 无错误

- [ ] **Step 5: 提交**

```bash
git add frontend/src/components/TaskDetailModal.tsx frontend/src/styles/components.css
git commit -m "任务弹窗:实际开始/结束日期 + 基线信息与偏差徽标"
```

---

### Task 8: Dashboard 偏差统计

**Files:**
- Modify: `frontend/src/pages/Dashboard.tsx`
- Modify: `frontend/src/styles/components.css`

**Interfaces:**
- Consumes: DashboardStats 响应的 `overall_progress`（既有）与 `baseline_progress`（Task 4 新增）；项目列表的 `baseline_created_at` / `delay_days`
- Produces: 「整体完成」统计卡偏差小字（Δ +8%，绿 + / 红 − / 无基线不显示）；项目卡片名右侧 `Δ +3d` 徽标

- [ ] **Step 1: 整体完成卡加偏差** — Dashboard.tsx 整体完成卡大数字下方加：

```tsx
        {stats.baseline_progress > 0 && (
          <div className={`stat-delta ${stats.overall_progress - stats.baseline_progress >= 0 ? "pos" : "neg"}`}>
            Δ {stats.overall_progress - stats.baseline_progress >= 0 ? "+" : ""}
            {Math.round(stats.overall_progress - stats.baseline_progress)}%
          </div>
        )}
```

（`stats` 为 DashboardStats 响应对象；`overall_progress` 为既有字段名，实现时按 dashboardStore 实际字段名核对。）

- [ ] **Step 2: 项目卡片加偏差徽标** — 项目名右侧（`详情` 链接左侧）插入：

```tsx
        {p.baseline_created_at && p.delay_days !== 0 && (
          <span className={`delay-badge ${p.delay_days > 0 ? "neg" : "pos"}`}>
            Δ {p.delay_days > 0 ? `+${p.delay_days}` : p.delay_days} 天
          </span>
        )}
```

- [ ] **Step 3: 样式** — `components.css` 加：

```css
/* 统计卡偏差与项目卡偏差徽标 */
.stat-delta { font-size: 14px; font-weight: 600; margin-top: 2px; }
.stat-delta.pos { color: #15803d; }
.stat-delta.neg { color: #ef4444; }
.delay-badge { font-size: 12px; font-weight: 600; padding: 1px 6px; border-radius: 4px; margin-right: 8px; }
.delay-badge.pos { color: #15803d; background: #f0fdf4; }
.delay-badge.neg { color: #ef4444; background: #fef2f2; }
```

- [ ] **Step 4: 验证编译 + 浏览器目检**

Run: `cd frontend && npx tsc --noEmit`
Expected: 无错误

浏览器验证：有基线的项目卡片显示 Δ 天数；整体完成卡显示 Δ%；清除基线后项目徽标消失。

- [ ] **Step 5: 提交**

```bash
git add frontend/src/pages/Dashboard.tsx frontend/src/styles/components.css
git commit -m "看板偏差统计:整体完成Δ% + 项目卡片延期天数徽标"
```

---

### Task 9: 全量回归 + 完整构建

**Files:**（无代码改动，仅验证）

- [ ] **Step 1: 后端全量测试**

Run: `cd backend && go test ./...`
Expected: 全过（含 Task 1-4 新增用例 + 既有排程/财年用例）

- [ ] **Step 2: 前端类型检查**

Run: `cd frontend && npx tsc --noEmit`
Expected: 无错误

- [ ] **Step 3: 完整构建（按 Global Constraints 顺序）**

Run: `cd frontend && npm run build && cd .. && cp -r frontend/dist backend/cmd/server/frontend-dist && cd backend && go build -o followitup.exe ./cmd/server/`
Expected: `followitup.exe` 生成成功

- [ ] **Step 4: 冒烟验证**

Run: `cd backend && ./followitup.exe config.yaml`（后台启动），浏览器访问 http://localhost:8080 登录 → 进入有任务的项目 → 创建基线 → 目检灰细条 → 改任务状态 → 目检绿细条 → 回看板 → 目检 Δ 徽标。完成后关闭进程。

- [ ] **Step 5: 运行 detect_changes 确认影响范围**

Run: GitNexus `detect_changes`（scope: all），确认 affected_processes 的 changed_steps 均为改动符号自身（无下游破坏）后提交。

- [ ] **Step 6: 提交**

```bash
git add -A
git commit -m "基线对比功能 v1.0:全量回归通过"
```

---

## Self-Review 记录

- **Spec 覆盖**：迁移（Task 1）✓ / baseline API（Task 3）✓ / 实际日期自动填充（Task 2）✓ / 甘特双细条紧贴层（Task 6）✓ / 工具栏下拉（Task 6）✓ / 弹窗实际日期+基线信息+Δ（Task 7）✓ / 看板统计卡与项目卡片（Task 8）✓ / 测试计划（Task 1-4 用例 + Task 9 回归）✓
- **占位符**：无 TBD/TODO；两处模块路径/字段名标注"按实际核对"（go.mod 模块路径、dashboardStore 既有字段名），均给了核对方式
- **类型一致性**：`fillActualDates(status, actualStart, actualEnd string) (string, string)`、`createBaselineTx(db *sql.DB, projectID, userID int64, userName string) error`、`clearBaselineTx(db *sql.DB, projectID int64) error`、`BaselineMeta{created_at, created_by, task_count}`、`baseline_start_date` 等字段名在前后任务中一致；`delay_days`/`baseline_progress` JSON 名 Task 4 定义、Task 8 消费，一致
- **权限口径调整**：spec 中"viewer→403"用例按实际系统权限模型（登录即可编辑）调整为"未登录→401"，在 Global Constraints 中注明
