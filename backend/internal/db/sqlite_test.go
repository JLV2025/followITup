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

// 迁移 v5 后 projects 表应包含排程方向列
func TestMigrationV5ScheduleDirection(t *testing.T) {
	d, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	cols := map[string]bool{}
	rows, err := d.Conn.Query(`PRAGMA table_info(projects)`)
	if err != nil {
		t.Fatalf("PRAGMA projects: %v", err)
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
	if !cols["schedule_direction"] {
		t.Error("projects 表缺少 schedule_direction 列")
	}
}
