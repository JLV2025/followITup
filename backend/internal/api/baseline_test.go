package api

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"followitup/internal/db"
)

// testBaselineDB 创建测试用临时数据库（迁移自动执行）
func testBaselineDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d.Conn
}

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

// 创建基线：所有任务 baseline_* = 当前值，projects 元数据写入
func TestCreateBaselineSnapshot(t *testing.T) {
	conn := testBaselineDB(t)
	now := time.Now().Format("2006-01-02")

	var pid int64
	conn.Exec(`INSERT INTO projects (name, start_date, end_date, status) VALUES ('测试项目', ?, ?, 'active')`, now, now)
	if err := conn.QueryRow(`SELECT id FROM projects ORDER BY id DESC LIMIT 1`).Scan(&pid); err != nil {
		t.Fatalf("获取项目ID: %v", err)
	}

	conn.Exec(`INSERT INTO tasks (project_id, name, start_date, end_date, duration_days, progress_pct, status) VALUES (?, 'A', ?, ?, 5, 40, 'in_progress')`, pid, now, now)
	conn.Exec(`INSERT INTO tasks (project_id, name, start_date, end_date, duration_days, progress_pct, status) VALUES (?, 'B', ?, ?, 3, 100, 'completed')`, pid, now, now)

	if err := createBaselineTx(conn, pid, "admin"); err != nil {
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

	if err := createBaselineTx(conn, pid, "admin"); err != nil {
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
	end := time.Now().AddDate(0, 0, 3).Format("2006-01-02")
	conn.Exec(`UPDATE tasks SET end_date=? WHERE id=?`, end, aID)
	var delay int
	// MAX 聚合不可用于 WHERE 子句，用 HAVING 替代
	conn.QueryRow(`SELECT CAST(julianday(MAX(end_date)) - julianday(MAX(baseline_end_date)) AS INTEGER)
		FROM tasks WHERE project_id=? AND deleted_at IS NULL HAVING MAX(baseline_end_date) IS NOT NULL`, pid).Scan(&delay)
	if delay != 3 {
		t.Errorf("偏差天数 = %d, want 3", delay)
	}
}

// 清除基线：baseline_* 全 NULL
func TestClearBaseline(t *testing.T) {
	conn := testBaselineDB(t)
	now := time.Now().Format("2006-01-02")
	var pid int64
	conn.Exec(`INSERT INTO projects (name, start_date, end_date, status) VALUES ('测试项目', ?, ?, 'active')`, now, now)
	if err := conn.QueryRow(`SELECT id FROM projects ORDER BY id DESC LIMIT 1`).Scan(&pid); err != nil {
		t.Fatalf("获取项目ID: %v", err)
	}
	conn.Exec(`INSERT INTO tasks (project_id, name, start_date, end_date, duration_days, progress_pct) VALUES (?, 'A', ?, ?, 5, 40)`, pid, now, now)

	if err := createBaselineTx(conn, pid, "admin"); err != nil {
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
