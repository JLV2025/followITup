package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

// DB 数据库连接封装
type DB struct {
	Conn *sql.DB
}

// Open 打开 SQLite 数据库
func Open(dataDir string) (*DB, error) {
	dbPath := dataDir + "/followitup.db"
	conn, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	// 连接池配置
	// WAL 模式需要至少 2 个连接（1 读 + 1 写）才能正常工作
	conn.SetMaxOpenConns(4)
	conn.SetMaxIdleConns(2)
	conn.SetConnMaxLifetime(time.Hour)

	d := &DB{Conn: conn}
	if err := d.migrate(); err != nil {
		return nil, fmt.Errorf("数据库迁移失败: %w", err)
	}

	return d, nil
}

// Close 关闭数据库连接
func (d *DB) Close() error {
	return d.Conn.Close()
}

// migration 定义一次迁移
type migration struct {
	version int
	sql     string
}

// migrations 所有迁移，按版本号递增
var migrations = []migration{
	{1, `
	CREATE TABLE IF NOT EXISTS schema_version (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	);

	-- 用户表
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		login TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL DEFAULT '',
		email TEXT NOT NULL UNIQUE,
		display_name TEXT NOT NULL DEFAULT '',
		auth_source TEXT NOT NULL DEFAULT 'local',
		is_admin INTEGER NOT NULL DEFAULT 0,
		is_active INTEGER NOT NULL DEFAULT 1,
		failed_attempts INTEGER NOT NULL DEFAULT 0,
		locked_until TEXT,
		must_change_password INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	);

	-- 项目表
	CREATE TABLE IF NOT EXISTS projects (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		start_date TEXT NOT NULL DEFAULT '',
		end_date TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'active',
		is_public INTEGER NOT NULL DEFAULT 1,
		deleted_at TEXT,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	);

	-- 任务表
	CREATE TABLE IF NOT EXISTS tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id INTEGER NOT NULL,
		parent_id INTEGER,
		name TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		task_type TEXT NOT NULL DEFAULT 'task',
		status TEXT NOT NULL DEFAULT 'open',
		priority TEXT NOT NULL DEFAULT 'medium',
		assignee TEXT NOT NULL DEFAULT '',
		start_date TEXT NOT NULL DEFAULT '',
		end_date TEXT NOT NULL DEFAULT '',
		duration_days INTEGER NOT NULL DEFAULT 1,
		progress_pct REAL NOT NULL DEFAULT 0.0,
		actual_start TEXT NOT NULL DEFAULT '',
		actual_end TEXT NOT NULL DEFAULT '',
		manual_scheduled INTEGER NOT NULL DEFAULT 0,
		sort_order INTEGER NOT NULL DEFAULT 0,
		version INTEGER NOT NULL DEFAULT 0,
		deleted_at TEXT,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now')),
		FOREIGN KEY (project_id) REFERENCES projects(id),
		FOREIGN KEY (parent_id) REFERENCES tasks(id)
	);

	-- 任务依赖关系表
	CREATE TABLE IF NOT EXISTS dependencies (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		predecessor_id INTEGER NOT NULL,
		successor_id INTEGER NOT NULL,
		dep_type TEXT NOT NULL DEFAULT 'FS',
		lag_days INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		FOREIGN KEY (predecessor_id) REFERENCES tasks(id),
		FOREIGN KEY (successor_id) REFERENCES tasks(id),
		UNIQUE(predecessor_id, successor_id)
	);

	-- 项目成员表
	CREATE TABLE IF NOT EXISTS project_members (
		project_id INTEGER NOT NULL,
		user_id INTEGER NOT NULL,
		role TEXT NOT NULL DEFAULT 'editor',
		PRIMARY KEY (project_id, user_id),
		FOREIGN KEY (project_id) REFERENCES projects(id),
		FOREIGN KEY (user_id) REFERENCES users(id)
	);

	-- 操作日志表
	CREATE TABLE IF NOT EXISTS activity_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id INTEGER NOT NULL,
		task_id INTEGER,
		user_id INTEGER NOT NULL,
		action TEXT NOT NULL,
		details_json TEXT NOT NULL DEFAULT '{}',
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		FOREIGN KEY (project_id) REFERENCES projects(id),
		FOREIGN KEY (task_id) REFERENCES tasks(id),
		FOREIGN KEY (user_id) REFERENCES users(id)
	);

	-- 索引
	CREATE INDEX IF NOT EXISTS idx_tasks_project ON tasks(project_id, deleted_at);
	CREATE INDEX IF NOT EXISTS idx_tasks_parent ON tasks(parent_id);
	CREATE INDEX IF NOT EXISTS idx_dependencies_predecessor ON dependencies(predecessor_id);
	CREATE INDEX IF NOT EXISTS idx_dependencies_successor ON dependencies(successor_id);
	CREATE INDEX IF NOT EXISTS idx_activity_project ON activity_log(project_id, created_at);
	`},
	{2, `
	-- 任务约束（v2）
	ALTER TABLE tasks ADD COLUMN constraint_type TEXT NOT NULL DEFAULT '';
	ALTER TABLE tasks ADD COLUMN constraint_date TEXT NOT NULL DEFAULT '';
	`},
       {3, `
		-- 工作日历（v3）
		CREATE TABLE IF NOT EXISTS calendar (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			date TEXT NOT NULL UNIQUE,
			type TEXT NOT NULL DEFAULT 'holiday',
			label TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
		CREATE INDEX IF NOT EXISTS idx_calendar_date ON calendar(date);
		`},
	{4, `
	-- 基线对比（v4）
	ALTER TABLE tasks ADD COLUMN baseline_start_date TEXT;
	ALTER TABLE tasks ADD COLUMN baseline_end_date TEXT;
	ALTER TABLE tasks ADD COLUMN baseline_duration_days INTEGER;
	ALTER TABLE tasks ADD COLUMN baseline_progress_pct REAL;
	ALTER TABLE projects ADD COLUMN baseline_created_at TEXT;
	ALTER TABLE projects ADD COLUMN baseline_created_by TEXT;
	`},
	{5, `
	-- 项目排程方向（v5）：forward=正推(基于开始日期) backward=倒推(基于完成日期)
	ALTER TABLE projects ADD COLUMN schedule_direction TEXT NOT NULL DEFAULT 'forward';
	`},
	{6, `
	-- 系统配置表（v6）：key-value 存储（SMTP/财年/密码策略等）
	CREATE TABLE IF NOT EXISTS settings (
		key        TEXT PRIMARY KEY,
		value      TEXT NOT NULL,
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	);
	`},
	{7, `
	-- end_date 改为独占式（v7）：结束日 = 开始 + 工期（1 天任务 8/5~8/6）
	-- 既有 inclusive 数据 +1 天迁移；里程碑（点）保持 end = start；格式非法（date() 为 NULL）跳过
	UPDATE tasks SET end_date = date(end_date, '+1 day')
	 WHERE end_date != '' AND task_type != 'milestone' AND date(end_date) IS NOT NULL;
	`},
	{8, `
	-- 项目所有者（v8）：创建项目必填，未开始任务默认取项目 owner
	ALTER TABLE projects ADD COLUMN owner TEXT NOT NULL DEFAULT '';
	`},
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
	-- 项目 owner 支持分号分隔多值(如 '张三;李四'),递归 CTE 拆分后逐个匹配;
	-- 每个(项目, 名字)取最小 id,同项目多值保留多行
	WITH RECURSIVE split(project_id, rest, item) AS (
		SELECT id, owner, '' FROM projects WHERE owner != ''
		UNION ALL
		SELECT project_id,
			CASE WHEN instr(rest, ';') > 0 THEN substr(rest, instr(rest, ';') + 1) ELSE '' END,
			CASE WHEN instr(rest, ';') > 0 THEN substr(rest, 1, instr(rest, ';') - 1) ELSE rest END
		FROM split WHERE rest != ''
	)
	INSERT OR IGNORE INTO project_owners (project_id, user_id)
	SELECT s.project_id, MIN(u.id)
	FROM split s
	JOIN users u ON (u.email = trim(s.item) OR u.display_name = trim(s.item)) AND u.is_active = 1
	WHERE trim(s.item) != ''
	GROUP BY s.project_id, s.item;

	-- 快照列回填最新显示名(分号+空格分隔)
	UPDATE tasks SET assignee = (
		SELECT GROUP_CONCAT(u.display_name, '; ' ORDER BY u.id) FROM task_assignees ta JOIN users u ON u.id = ta.user_id
		WHERE ta.task_id = tasks.id)
	WHERE EXISTS (SELECT 1 FROM task_assignees ta WHERE ta.task_id = tasks.id);
	UPDATE projects SET owner = (
		SELECT GROUP_CONCAT(u.display_name, '; ' ORDER BY u.id) FROM project_owners po JOIN users u ON u.id = po.user_id
		WHERE po.project_id = projects.id)
	WHERE EXISTS (SELECT 1 FROM project_owners po WHERE po.project_id = projects.id);
	`},
	{10, `
	-- 实际工期（v10）：实际开始/结束填入后由后端算好落库，供汇总报告对比计划 vs 实际
	ALTER TABLE tasks ADD COLUMN actual_duration_days INTEGER NOT NULL DEFAULT 0;
	`},
}

// migrate 执行所有未应用的迁移
func (d *DB) migrate() error {
	// 确保 schema_version 表首先存在
	_, err := d.Conn.Exec(`CREATE TABLE IF NOT EXISTS schema_version (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`)
	if err != nil {
		return fmt.Errorf("创建版本表失败: %w", err)
	}

	// 获取当前版本
	var currentVersion int
	err = d.Conn.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&currentVersion)
	if err != nil {
		currentVersion = 0
	}

	// 按顺序执行迁移
	for _, m := range migrations {
		if m.version <= currentVersion {
			continue
		}
		log.Printf("[DB] 执行迁移 v%d...", m.version)

		tx, err := d.Conn.Begin()
		if err != nil {
			return fmt.Errorf("开启事务失败 (v%d): %w", m.version, err)
		}

		if _, err := tx.Exec(m.sql); err != nil {
			tx.Rollback()
			return fmt.Errorf("迁移 v%d 执行失败: %w", m.version, err)
		}

		if _, err := tx.Exec("INSERT OR REPLACE INTO schema_version (version) VALUES (?)", m.version); err != nil {
			tx.Rollback()
			return fmt.Errorf("记录版本 v%d 失败: %w", m.version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("提交迁移 v%d 失败: %w", m.version, err)
		}

		log.Printf("[DB] 迁移 v%d 完成", m.version)
	}

	return nil
}
