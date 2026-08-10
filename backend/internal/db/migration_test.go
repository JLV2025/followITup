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
